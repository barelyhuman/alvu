package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"text/template"

	"github.com/barelyhuman/go/env"
	ghttp "github.com/cjoudrey/gluahttp"

	cp "github.com/otiai10/copy"

	stringsLib "github.com/vadv/gopher-lua-libs/strings"

	yamlLib "github.com/vadv/gopher-lua-libs/yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"

	luaAlvu "codeberg.org/reaper/alvu/lua/alvu"
	luajson "layeh.com/gopher-json"
)

var mdProcessor goldmark.Markdown
var baseurl string
var basePath string
var hookCollection HookCollection
var wg = &sync.WaitGroup{}

type SiteMeta struct {
	BaseURL string
}

func main() {
	onDebug(func() {
		debugInfo("Before Exec")
		memuse()
	})

	basePathFlag := flag.String("path", ".", "`DIR` to search for the needed folders in")
	outPathFlag := flag.String("out", "./dist", "`DIR` to output the compiled files to")
	baseurlFlag := flag.String("baseurl", "/", "`URL` to be used as the root of the project")
	hooksPathFlag := flag.String("hooks", "./hooks", "`DIR` that contains hooks for the content")

	flag.Parse()

	baseurl = *baseurlFlag
	basePath = path.Join(*basePathFlag)
	pagesPath := path.Join(*basePathFlag, "pages")
	publicPath := path.Join(*basePathFlag, "public")
	headFilePath := path.Join(pagesPath, "_head.html")
	tailFilePath := path.Join(pagesPath, "_tail.html")
	outPath := path.Join(*outPathFlag)
	hooksPath := path.Join(*basePathFlag, *hooksPathFlag)

	onDebug(func() {
		debugInfo("Opening _head")
		memuse()
	})
	headFileFd, err := os.Open(headFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _head.html found,skipping")
		}
	}

	onDebug(func() {
		debugInfo("Opening _tail")
		memuse()
	})
	tailFileFd, err := os.Open(tailFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _tail.html found, skipping")
		}
	}

	onDebug(func() {
		debugInfo("Before copying files")
		memuse()
	})
	// copy public to out
	_, err = os.Stat(publicPath)
	if err == nil {
		err = cp.Copy(publicPath, outPath)
		if err != nil {
			bail(err)
		}
	}
	onDebug(func() {
		debugInfo("After copying files")
		memuse()
	})

	onDebug(func() {
		debugInfo("Reading hook and to process files")
		memuse()
	})
	CollectHooks(basePath, hooksPath)
	toProcess := CollectFilesToProcess(pagesPath)

	initMDProcessor()

	onDebug(func() {
		debugInfo("Running all OnStart hooks")
		memuse()
	})
	hookCollection.RunAll("OnStart")

	prefixSlashPath := regexp.MustCompile(`^\/`)

	onDebug(func() {
		debugInfo("Processing Files")
		memuse()
	})
	for _, toProcessItem := range toProcess {
		fileName := strings.Replace(toProcessItem, pagesPath, "", 1)
		fileName = prefixSlashPath.ReplaceAllString(fileName, "")
		destFilePath := strings.Replace(toProcessItem, pagesPath, outPath, 1)

		alvuFile := &AlvuFile{
			lock:       &sync.Mutex{},
			sourcePath: toProcessItem,
			destPath:   destFilePath,
			name:       fileName,
			headFile:   headFileFd,
			tailFile:   tailFileFd,
			data:       map[string]interface{}{},
			extras:     map[string]interface{}{},
		}

		bail(alvuFile.ReadFile())
		bail(alvuFile.ParseMeta())

		for _, hook := range hookCollection {

			isForSpecificFile := hook.state.GetGlobal("ForFile")

			if isForSpecificFile != lua.LNil && alvuFile.name != isForSpecificFile.String() {
				continue
			}

			bail(alvuFile.ProcessFile(hook.state))
		}
		alvuFile.FlushFile()
	}
	onDebug(func() {
		debugInfo("Run all OnFinish Hooks")
		memuse()
	})
	// right before completion run all hooks again but for the onFinish
	hookCollection.RunAll("OnFinish")
	hookCollection.Shutdown()

	onDebug(func() {
		runtime.GC()
		debugInfo("On Completions")
		memuse()
	})
}

func CollectFilesToProcess(basepath string) []string {
	files := []string{}

	pathstoprocess, err := os.ReadDir(basepath)
	if err != nil {
		panic(err)
	}

	for _, pathInfo := range pathstoprocess {
		_path := path.Join(basepath, pathInfo.Name())

		if pathInfo.Name() == "_head.html" || pathInfo.Name() == "_tail.html" {
			continue
		}

		if pathInfo.IsDir() {
			files = append(files, CollectFilesToProcess(_path)...)
		} else {
			files = append(files, _path)
		}
	}

	return files
}

func CollectHooks(basePath, hooksBasePath string) {
	if _, err := os.Stat(hooksBasePath); err != nil {
		return
	}
	pathsToProcess, err := os.ReadDir(hooksBasePath)
	if err != nil {
		panic(err)
	}

	for _, pathInfo := range pathsToProcess {
		if !strings.HasSuffix(pathInfo.Name(), ".lua") {
			continue
		}
		hook := NewHook()
		hookPath := path.Join(hooksBasePath, pathInfo.Name())
		if err := hook.DoFile(hookPath); err != nil {
			panic(err)
		}
		hookCollection = append(hookCollection, &Hook{
			path:  hookPath,
			state: hook,
		})
	}

}

func initMDProcessor() {
	mdProcessor = goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
}

type Hook struct {
	path  string
	state *lua.LState
}

type HookCollection []*Hook

func (hc HookCollection) Shutdown() {
	for _, hook := range hc {
		hook.state.Close()
	}
}

func (hc HookCollection) RunAll(funcName string) {
	for _, hook := range hc {
		hookFunc := hook.state.GetGlobal(funcName)

		if hookFunc == lua.LNil {
			continue
		}

		if err := hook.state.CallByParam(lua.P{
			Fn:      hookFunc,
			NRet:    0,
			Protect: true,
		}); err != nil {
			bail(err)
		}
	}
}

type AlvuFile struct {
	lock             *sync.Mutex
	name             string
	sourcePath       string
	destPath         string
	meta             map[string]interface{}
	content          []byte
	writeableContent []byte
	headFile         *os.File
	tailFile         *os.File
	targetName       []byte
	data             map[string]interface{}
	extras           map[string]interface{}
}

func (a *AlvuFile) ReadFile() error {
	filecontent, err := os.ReadFile(a.sourcePath)
	if err != nil {
		return fmt.Errorf("error reading file, error: %v", err)
	}
	a.content = filecontent
	return nil
}

func (a *AlvuFile) ParseMeta() error {
	sep := []byte("---")
	if !bytes.HasPrefix(a.content, sep) {
		a.writeableContent = a.content
		return nil
	}

	metaParts := bytes.SplitN(a.content, sep, 3)

	var meta map[string]interface{}
	err := yaml.Unmarshal([]byte(metaParts[1]), &meta)
	if err != nil {
		return err
	}

	a.meta = meta
	a.writeableContent = []byte(metaParts[2])

	return nil
}

func (a *AlvuFile) ProcessFile(hook *lua.LState) error {
	// pre process hook => should return back json with `content` and `data`
	a.lock.Lock()
	defer a.lock.Unlock()

	a.targetName = regexp.MustCompile(`\.md$`).ReplaceAll([]byte(a.name), []byte(".html"))
	buf := bytes.NewBuffer([]byte(""))
	mdToHTML := ""

	if filepath.Ext(a.name) == ".md" {
		newName := strings.Replace(a.name, filepath.Ext(a.name), ".html", 1)
		a.targetName = []byte(newName)
		mdProcessor.Convert(a.writeableContent, buf)
		mdToHTML = buf.String()
	}

	if hook == nil {
		return nil
	}

	hookInput := struct {
		Name             string                 `json:"name"`
		SourcePath       string                 `json:"source_path"`
		DestPath         string                 `json:"dest_path"`
		Meta             map[string]interface{} `json:"meta"`
		WriteableContent string                 `json:"content"`
		HTMLContent      string                 `json:"html"`
	}{
		Name:             string(a.targetName),
		SourcePath:       a.sourcePath,
		DestPath:         a.destPath,
		Meta:             a.meta,
		WriteableContent: string(a.writeableContent),
		HTMLContent:      mdToHTML,
	}

	hookJsonInput, err := json.Marshal(hookInput)
	bail(err)

	if err := hook.CallByParam(lua.P{
		Fn:      hook.GetGlobal("Writer"),
		NRet:    1,
		Protect: true,
	}, lua.LString(hookJsonInput)); err != nil {
		panic(err)
	}

	ret := hook.Get(-1)

	var fromPlug map[string]interface{}

	err = json.Unmarshal([]byte(ret.String()), &fromPlug)
	bail(err)

	if fromPlug["content"] != nil {
		stringVal := fmt.Sprintf("%s", fromPlug["content"])
		a.writeableContent = []byte(stringVal)
	}

	if fromPlug["name"] != nil {
		a.targetName = []byte(fmt.Sprintf("%v", fromPlug["name"]))
	}

	if fromPlug["data"] != nil {
		a.data = mergeMapWithCheck(a.data, fromPlug["data"])
	}

	if fromPlug["extras"] != nil {
		a.extras = mergeMapWithCheck(a.extras, fromPlug["extras"])
	}

	hook.Pop(1)
	return nil
}

func (a *AlvuFile) FlushFile() {
	destFolder := filepath.Dir(a.destPath)
	os.MkdirAll(destFolder, os.ModePerm)

	targetFile := strings.Replace(path.Join(a.destPath), a.name, string(a.targetName), 1)
	f, err := os.Create(targetFile)
	bail(err)
	defer f.Sync()

	writeHeadTail := false

	if filepath.Ext(a.sourcePath) == ".md" || filepath.Ext(a.sourcePath) == "html" {
		writeHeadTail = true
	}

	var toHtml bytes.Buffer
	if writeHeadTail {
		mdProcessor.Convert(a.writeableContent, &toHtml)
	} else {
		toHtml.Write(a.writeableContent)
	}

	t := template.New(path.Join(a.sourcePath))
	t.Parse(toHtml.String())

	if writeHeadTail {
		a.headFile.Seek(0, 0)
		_, err = io.Copy(f, a.headFile)
		bail(err)
	}

	err = t.Execute(f, struct {
		Meta   SiteMeta
		Data   map[string]interface{}
		Extras map[string]interface{}
	}{
		Meta: SiteMeta{
			BaseURL: baseurl,
		},
		Data:   a.data,
		Extras: a.extras,
	})

	bail(err)

	if writeHeadTail {
		a.tailFile.Seek(0, 0)
		_, err = io.Copy(f, a.tailFile)
		bail(err)
	}
}

func NewHook() *lua.LState {
	lState := lua.NewState()
	luaAlvu.Preload(lState)
	luajson.Preload(lState)
	yamlLib.Preload(lState)
	stringsLib.Preload(lState)
	lState.PreloadModule("http", ghttp.NewHttpModule(&http.Client{}).Loader)
	if basePath == "." {
		lState.SetGlobal("workingdir", lua.LString(""))
	} else {
		lState.SetGlobal("workingdir", lua.LString(basePath))
	}
	return lState
}

// UTILS

func memuse() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("heap: %v MiB\n", bytesToMB(m.HeapAlloc))
}

func bytesToMB(inBytes uint64) uint64 {
	return inBytes / 1024 / 1024
}

func bail(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[alvu]: "+err.Error())
	panic("")
}

func debugInfo(msg string, a ...any) {
	Color := "\u001b[1m\033[36m"
	Reset := "\033[0m"

	prefix := "[alvu]"
	prefix = Color + prefix
	prefix += Reset + " "

	fmt.Fprintf(os.Stdout, prefix+msg+" \n", a...)
}

func showDebug() bool {
	showInfo := env.Get("DEBUG_ALVU", "")
	return len(showInfo) != 0
}

func onDebug(fn func()) {
	if !showDebug() {
		return
	}

	fn()
}

func mergeMapWithCheck(maps ...any) (source map[string]interface{}) {
	source = map[string]interface{}{}
	for _, toCheck := range maps {
		if pairs, ok := toCheck.(map[string]interface{}); ok {
			for k, v := range pairs {
				source[k] = v
			}
		}
	}
	return source
}

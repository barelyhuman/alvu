package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

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

type SiteMeta struct {
	BaseURL string
}

func bail(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[alvu]: "+err.Error())
	panic("")
}
func main() {
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

	headContent, err := os.ReadFile(headFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _head.html found,skipping")
		}
	}

	tailContent, err := os.ReadFile(tailFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _tail.html found, skipping")
		}
	}

	// copy public to out
	_, err = os.Stat(publicPath)
	if err == nil {
		err = cp.Copy(publicPath, outPath)
		if err != nil {
			bail(err)
		}
	}

	CollectHooks(basePath, hooksPath)
	toProcess := CollectFilesToProcess(pagesPath)

	initMDProcessor()

	hookCollection.RunAll("OnStart")

	prefixSlashPath := regexp.MustCompile(`^\/`)

	alvuFiles := []*AlvuFile{}

	for _, toProcessItem := range toProcess {
		fileName := strings.Replace(toProcessItem, pagesPath, "", 1)
		fileName = prefixSlashPath.ReplaceAllString(fileName, "")
		destFilePath := strings.Replace(toProcessItem, pagesPath, outPath, 1)

		alvuFile := &AlvuFile{
			lock:        &sync.Mutex{},
			sourcePath:  toProcessItem,
			destPath:    destFilePath,
			name:        fileName,
			headContent: headContent,
			tailContent: tailContent,
			data:        map[string]interface{}{},
			extras:      map[string]interface{}{},
		}

		bail(alvuFile.ReadFile())
		bail(alvuFile.ParseMeta())

		alvuFiles = append(alvuFiles, alvuFile)
	}

	for _, hook := range hookCollection {
		for _, alvuFile := range alvuFiles {
			isForSpecificFile := hook.state.GetGlobal("ForFile")

			if isForSpecificFile != lua.LNil && alvuFile.name != isForSpecificFile.String() {
				continue
			}

			bail(alvuFile.ProcessFile(hook.state))
			alvuFile.FlushFile()
		}
	}

	// right before completion run all hooks again but for the onFinish
	hookCollection.RunAll("OnFinish")
	hookCollection.Shutdown()
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
	log.Println(len(hc))
	for i, hook := range hc {
		print("executing", i)
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
	headContent      []byte
	tailContent      []byte
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
		// merge resulting map with overall for this file
		if pairs, ok := fromPlug["data"].(map[string]interface{}); ok {
			for k, v := range pairs {
				a.data[k] = v
			}
		}
	}

	if fromPlug["extras"] != nil {
		if pairs, ok := fromPlug["extras"].(map[string]interface{}); ok {
			for k, v := range pairs {
				a.extras[k] = v
			}
		}
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
		f.Write(a.headContent)
	}

	t.Execute(f, struct {
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

	if writeHeadTail {
		f.Write(a.tailContent)
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

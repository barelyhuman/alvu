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

	luajson "layeh.com/gopher-json"
)

var mdProcessor goldmark.Markdown
var baseurl string
var basePath string
var hookCollection []*lua.LState

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

func StartAlvuFileWorker(in <-chan *AlvuFile, out chan *AlvuFile, hookfiles []string) {
	for file := range in {
		err := file.ReadFile()
		bail(err)
		err = file.ParseMeta()
		bail(err)

		if len(hookfiles) > 0 {
			for _, hookpath := range hookfiles {
				hook := NewHook()
				err = hook.DoFile(hookpath)
				bail(err)
				err = file.ProcessFile(hook)
				bail(err)
			}
		} else {
			err = file.ProcessFile(nil)
			bail(err)
		}
		out <- file
	}
}

func main() {
	// wait group for the files being sent to process
	senderGroup := &sync.WaitGroup{}

	// FIXME: replace with a proper cli parser solution
	// also add in config file parsing for this
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

	// read the head content into memory
	// FIXME: change to a bufio instead
	headContent, err := os.ReadFile(headFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _head.html found,skipping")
		}
	}

	// read the tail content into memory
	// FIXME: change to a bufio instead
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

	// collect the files paths needed to be processed
	// also includes the hooks they are to be run against
	hooksToUse := CollectHooks(basePath, hooksPath)
	toProcess := CollectFilesToProcess(pagesPath)

	// setup the initial markdown processor
	// FIXME: if none of the files above have a `.md` file, ignore
	// this step
	initMDProcessor()

	prefixSlashPath := regexp.MustCompile(`^\/`)

	fileQIn := make(chan *AlvuFile, len(toProcess)*len(hooksToUse))
	fileQOut := make(chan *AlvuFile, len(toProcess)*len(hooksToUse))

	// right before completion run all hooks again but for the onFinish
	RunAllHooks(hooksToUse, "OnStart")

	// start up 5 process threads
	for i := 0; i < 5; i++ {
		go StartAlvuFileWorker(fileQIn, fileQOut, hooksToUse)
	}

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

		senderGroup.Add(1)
		go func() {
			defer senderGroup.Done()
			fileQIn <- alvuFile
			outFile := <-fileQOut
			outFile.FlushFile()
		}()
	}

	senderGroup.Wait()
	close(fileQIn)
	close(fileQOut)

	// right before completion run all hooks again but for the onFinish
	RunAllHooks(hooksToUse, "OnFinish")

	// cleanup hooks
	for _, h := range hookCollection {
		h.Close()
	}
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

func CollectHooks(basePath, hooksBasePath string) (hooks []string) {
	if _, err := os.Stat(hooksBasePath); err != nil {
		return
	}
	pathstoprocess, err := os.ReadDir(hooksBasePath)
	if err != nil {
		panic(err)
	}

	for _, pathInfo := range pathstoprocess {
		if !strings.HasSuffix(pathInfo.Name(), ".lua") {
			continue
		}
		hookPath := path.Join(hooksBasePath, pathInfo.Name())
		hooks = append(hooks, hookPath)
	}
	return
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

// ReadFile read the file data for the particualr alvu file
// FIXME: add in a bufio reader and use that for the processing
// later
func (a *AlvuFile) ReadFile() error {
	filecontent, err := os.ReadFile(a.sourcePath)
	if err != nil {
		return fmt.Errorf("error reading file, error: %v", err)
	}
	a.content = filecontent
	return nil
}

// ReadFile read the file data for the particualr alvu file
// FIXME: should use the bufio reader once `ReadFile` is able to
// use it
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
	luajson.Preload(lState)
	yamlLib.Preload(lState)
	stringsLib.Preload(lState)
	lState.PreloadModule("http", ghttp.NewHttpModule(&http.Client{}).Loader)
	if basePath == "." {
		lState.SetGlobal("workingdir", lua.LString(""))
	} else {
		lState.SetGlobal("workingdir", lua.LString(basePath))
	}
	hookCollection = append(hookCollection, lState)
	return lState
}

func RunAllHooks(hooksToUse []string, funcToCall string) {
	for _, hookPath := range hooksToUse {
		hook := NewHook()
		if err := hook.DoFile(hookPath); err != nil {
			panic(err)
		}

		hookFunc := hook.GetGlobal(funcToCall)

		if hookFunc != lua.LNil {
			if err := hook.CallByParam(lua.P{
				Fn:      hook.GetGlobal(funcToCall),
				NRet:    0,
				Protect: true,
			}); err != nil {
				panic(err)
			}
		}
	}
}

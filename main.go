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

func main() {
	basePathFlag := flag.String("path", ".", "`DIR` to search for the needed folders in")
	outPathFlag := flag.String("out", "./dist", "`DIR` to output the compiled files to")
	hooksPathFlag := flag.String("hooks", "./hooks", "`DIR` that contains hooks for the content")

	flag.Parse()

	alvuFiles := []AlvuFile{}
	basePath := path.Join(*basePathFlag)
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
	if err != nil && err != fs.ErrNotExist {
		log.Println(err)
	}

	err = cp.Copy(publicPath, outPath)
	if err != nil {
		log.Println(err)
	}

	// load all hooks
	hooksToUse := CollectHooks(basePath, hooksPath)
	toProcess := CollectFilesToProcess(pagesPath)
	initMDProcessor()

	prefixSlashPath := regexp.MustCompile(`^\/`)

	for _, toProcessItem := range toProcess {
		fileName := strings.Replace(toProcessItem, pagesPath, "", 1)
		fileName = prefixSlashPath.ReplaceAllString(fileName, "")
		destFilePath := strings.Replace(toProcessItem, pagesPath, outPath, 1)

		alvuFiles = append(alvuFiles, AlvuFile{
			sourcePath:  toProcessItem,
			destPath:    destFilePath,
			name:        fileName,
			headContent: headContent,
			tailContent: tailContent,
			hooks:       hooksToUse,
		})
	}

	// var wg sync.WaitGroup
	for _, alvuFile := range alvuFiles {
		// wg.Add(1)
		// alvuFile := alvuFile
		// go func() {
		// defer wg.Done()
		alvuFile.ReadFile()
		alvuFile.ParseMeta()
		alvuFile.ProcessFile()
		// }()
	}

	// wg.Wait()

	for _, hook := range hooksToUse {
		hook.Close()
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

		if pathInfo.Name() == "_head.html" {
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

func CollectHooks(basePath, hooksBasePath string) (hooks []*lua.LState) {
	pathstoprocess, err := os.ReadDir(hooksBasePath)
	if err != nil {
		panic(err)
	}

	for _, pathInfo := range pathstoprocess {
		if !strings.HasSuffix(pathInfo.Name(), ".lua") {
			continue
		}
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

		hookPath := path.Join(hooksBasePath, pathInfo.Name())

		if err := lState.DoFile(hookPath); err != nil {
			panic(err)
		}
		hooks = append(hooks, lState)
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
	name             string
	sourcePath       string
	destPath         string
	meta             map[string]interface{}
	content          []byte
	writeableContent []byte
	headContent      []byte
	tailContent      []byte
	hooks            []*lua.LState
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

func (a *AlvuFile) ProcessFile() error {
	// pre process hook => should return back json with `content` and `data`
	content := a.writeableContent
	name := regexp.MustCompile(`\.md$`).ReplaceAll([]byte(a.name), []byte(".html"))
	_ = a.destPath

	hookInput := struct {
		Name             string                 `json:"name"`
		SourcePath       string                 `json:"source_path"`
		DestPath         string                 `json:"dest_path"`
		Meta             map[string]interface{} `json:"meta"`
		WriteableContent string                 `json:"content"`
	}{
		Name:             string(name),
		SourcePath:       a.sourcePath,
		DestPath:         a.destPath,
		Meta:             a.meta,
		WriteableContent: string(a.writeableContent),
	}

	hookJsonInput, _ := json.Marshal(hookInput)

	extras := map[string]interface{}{}
	data := map[string]interface{}{}

	for _, hook := range a.hooks {
		if err := hook.CallByParam(lua.P{
			Fn:      hook.GetGlobal("Writer"),
			NRet:    1,
			Protect: true,
		}, lua.LString(hookJsonInput)); err != nil {
			panic(err)
		}

		ret := hook.Get(-1)

		var fromPlug map[string]interface{}

		err := json.Unmarshal([]byte(ret.String()), &fromPlug)
		if err != nil {
			panic(err)
		}

		if fromPlug["content"] != nil {
			stringVal := fmt.Sprintf("%s", fromPlug["content"])
			content = []byte(stringVal)
		}

		if fromPlug["name"] != nil {
			name = []byte(fmt.Sprintf("%v", fromPlug["name"]))
		}

		if fromPlug["data"] != nil {
			// merge resulting map with overall for this file
			pairs, _ := fromPlug["data"].(map[string]interface{})
			for k, v := range pairs {
				data[k] = v
			}
		}

		if fromPlug["extras"] != nil {
			pairs, _ := fromPlug["extras"].(map[string]interface{})
			for k, v := range pairs {
				extras[k] = v
			}
		}

		hook.Pop(1)
	}

	destFolder := filepath.Dir(a.destPath)
	os.MkdirAll(destFolder, os.ModePerm)

	targetFile := strings.Replace(path.Join(a.destPath), a.name, string(name), 1)
	f, _ := os.Create(targetFile)
	defer f.Sync()

	var toHtml bytes.Buffer
	mdProcessor.Convert(content, &toHtml)

	t := template.New(path.Join(a.sourcePath))
	t.Parse(toHtml.String())
	f.Write(a.headContent)

	t.Execute(f, struct {
		Data   map[string]interface{}
		Extras map[string]interface{}
	}{
		Data:   data,
		Extras: extras,
	})

	f.Write(a.headContent)

	return nil
}

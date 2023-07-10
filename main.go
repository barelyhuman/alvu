package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	textTmpl "text/template"

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

	"github.com/barelyhuman/go/env"
	ghttp "github.com/cjoudrey/gluahttp"
	"github.com/fsnotify/fsnotify"

	"github.com/barelyhuman/go/color"
	cp "github.com/otiai10/copy"

	stringsLib "github.com/vadv/gopher-lua-libs/strings"

	yamlLib "github.com/vadv/gopher-lua-libs/yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"

	highlighting "github.com/yuin/goldmark-highlighting"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"

	luaAlvu "github.com/barelyhuman/alvu/lua/alvu"
	"golang.org/x/net/websocket"
	luajson "layeh.com/gopher-json"
)

const logPrefix = "[alvu] "

var mdProcessor goldmark.Markdown
var baseurl string
var basePath string
var outPath string
var hardWraps bool
var hookCollection HookCollection
var reloadCh = []chan bool{}
var serveFlag *bool

var reservedFiles []string = []string{"_head.html", "_tail.html", "_layout.html"}

type SiteMeta struct {
	BaseURL string
}

type PageRenderData struct {
	Meta   SiteMeta
	Data   map[string]interface{}
	Extras map[string]interface{}
}

type LayoutRenderData struct {
	PageRenderData
	Content template.HTML
}

// TODO: move stuff into the alvu struct type
// on each newly added feature or during improving
// older features.
type Alvu struct {
	publicPath string
	files      []*AlvuFile
	filesIndex []string
}

func (al *Alvu) AddFile(file *AlvuFile) {
	al.files = append(al.files, file)
	al.filesIndex = append(al.filesIndex, file.sourcePath)
}

func (al *Alvu) IsAlvuFile(filePath string) bool {
	for _, af := range al.filesIndex {
		if af == filePath {
			return true
		}
	}
	return false
}

func (al *Alvu) Build() {
	for ind := range al.files {
		alvuFile := al.files[ind]
		alvuFile.Build()
	}

	onDebug(func() {
		debugInfo("Run all OnFinish Hooks")
		memuse()
	})

	// right before completion run all hooks again but for the onFinish
	hookCollection.RunAll("OnFinish")
}

func (al *Alvu) CopyPublic() {
	onDebug(func() {
		debugInfo("Before copying files")
		memuse()
	})
	// copy public to out
	_, err := os.Stat(al.publicPath)
	if err == nil {
		err = cp.Copy(al.publicPath, outPath)
		if err != nil {
			bail(err)
		}
	}
	onDebug(func() {
		debugInfo("After copying files")
		memuse()
	})
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
	enableHighlightingFlag := flag.Bool("highlight", false, "enable highlighting for markdown files")
	highlightThemeFlag := flag.String("highlight-theme", "bw", "`THEME` to use for highlighting (supports most themes from pygments)")
	serveFlag = flag.Bool("serve", false, "start a local server")
	hardWrapsFlag := flag.Bool("hard-wrap", true, "enable hard wrapping of elements with `<br>`")
	portFlag := flag.String("port", "3000", "`PORT` to start the server on")

	flag.Parse()

	baseurl = *baseurlFlag
	basePath = path.Join(*basePathFlag)
	pagesPath := path.Join(*basePathFlag, "pages")
	publicPath := path.Join(*basePathFlag, "public")
	headFilePath := path.Join(pagesPath, "_head.html")
	baseFilePath := path.Join(pagesPath, "_layout.html")
	tailFilePath := path.Join(pagesPath, "_tail.html")
	outPath = path.Join(*outPathFlag)
	hooksPath := path.Join(*basePathFlag, *hooksPathFlag)
	hardWraps = *hardWrapsFlag

	headTailDeprecationWarning := color.ColorString{}
	headTailDeprecationWarning.Yellow(logPrefix).Yellow("[WARN] use of _tail.html and _head.html is deprecated, please use _layout.html instead")

	alvuApp := &Alvu{
		publicPath: publicPath,
	}

	watcher := NewWatcher(alvuApp)

	if *serveFlag {
		watcher.AddDir(pagesPath)
		watcher.AddDir(publicPath)
	}

	onDebug(func() {
		debugInfo("Opening _head")
		memuse()
	})
	headFileFd, err := os.Open(headFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _head.html found,skipping")
		}
	} else {
		fmt.Println(headTailDeprecationWarning.String())
	}

	onDebug(func() {
		debugInfo("Opening _layout")
		memuse()
	})
	baseFileFd, err := os.Open(baseFilePath)
	if err != nil {
		if err == fs.ErrNotExist {
			log.Println("no _layout.html found,skipping")
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
	} else {
		fmt.Println(headTailDeprecationWarning.String())
	}

	alvuApp.CopyPublic()

	onDebug(func() {
		debugInfo("Reading hook and to process files")
		memuse()
	})
	CollectHooks(basePath, hooksPath)
	toProcess := CollectFilesToProcess(pagesPath)
	onDebug(func() {
		log.Println("printing files to process")
		log.Println(toProcess)
	})

	initMDProcessor(*enableHighlightingFlag, *highlightThemeFlag)

	onDebug(func() {
		debugInfo("Running all OnStart hooks")
		memuse()
	})

	hookCollection.RunAll("OnStart")

	prefixSlashPath := regexp.MustCompile(`^\/`)

	onDebug(func() {
		debugInfo("Creating Alvu Files")
		memuse()
	})
	for _, toProcessItem := range toProcess {
		fileName := strings.Replace(toProcessItem, pagesPath, "", 1)
		fileName = prefixSlashPath.ReplaceAllString(fileName, "")
		destFilePath := strings.Replace(toProcessItem, pagesPath, outPath, 1)

		alvuFile := &AlvuFile{
			lock:         &sync.Mutex{},
			sourcePath:   toProcessItem,
			hooks:        hookCollection,
			destPath:     destFilePath,
			name:         fileName,
			headFile:     headFileFd,
			tailFile:     tailFileFd,
			baseTemplate: baseFileFd,
			data:         map[string]interface{}{},
			extras:       map[string]interface{}{},
		}

		alvuApp.AddFile(alvuFile)

		// If serving, also add the nested path into it
		if *serveFlag {
			watcher.AddDir(path.Dir(alvuFile.sourcePath))
		}
	}

	alvuApp.Build()

	onDebug(func() {
		runtime.GC()
		debugInfo("On Completions")
		memuse()
	})

	cs := &color.ColorString{}
	fmt.Println(cs.Blue(logPrefix).Green("Compiled ").Cyan("\"" + basePath + "\"").Green(" to ").Cyan("\"" + outPath + "\"").String())

	if *serveFlag {
		go watcher.StartWatching()
		runServer(*portFlag)
	}

	hookCollection.Shutdown()
}

func runServer(port string) {
	normalizedPort := port

	if !strings.HasPrefix(normalizedPort, ":") {
		normalizedPort = ":" + normalizedPort
	}

	cs := &color.ColorString{}
	cs.Blue(logPrefix).Green("Serving on").Reset(" ").Cyan(normalizedPort)
	fmt.Println(cs.String())

	http.Handle("/", http.HandlerFunc(ServeHandler))
	AddWebsocketHandler()

	err := http.ListenAndServe(normalizedPort, nil)

	if strings.Contains(err.Error(), "address already in use") {
		bail(errors.New("port already in use, use another port with the `-port` flag instead"))
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

		if Contains(reservedFiles, pathInfo.Name()) {
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

func initMDProcessor(highlight bool, theme string) {

	rendererOptions := []renderer.Option{
		html.WithXHTML(),
		html.WithUnsafe(),
	}

	if hardWraps {
		rendererOptions = append(rendererOptions, html.WithHardWraps())
	}
	gmPlugins := []goldmark.Option{
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			rendererOptions...,
		),
	}

	if highlight {
		gmPlugins = append(gmPlugins, goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle(theme),
			),
		))
	}

	mdProcessor = goldmark.New(gmPlugins...)
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
	hooks            HookCollection
	name             string
	sourcePath       string
	destPath         string
	meta             map[string]interface{}
	content          []byte
	writeableContent []byte
	headFile         *os.File
	tailFile         *os.File
	baseTemplate     *os.File
	targetName       []byte
	data             map[string]interface{}
	extras           map[string]interface{}
}

func (alvuFile *AlvuFile) Build() {
	bail(alvuFile.ReadFile())
	bail(alvuFile.ParseMeta())

	if len(alvuFile.hooks) == 0 {
		alvuFile.ProcessFile(nil)
	}

	for _, hook := range hookCollection {

		isForSpecificFile := hook.state.GetGlobal("ForFile")

		if isForSpecificFile != lua.LNil {
			if alvuFile.name == isForSpecificFile.String() {
				alvuFile.ProcessFile(hook.state)
			} else {
				bail(alvuFile.ProcessFile(nil))
			}
		} else {
			bail(alvuFile.ProcessFile(hook.state))
		}
	}

	alvuFile.FlushFile()
}

func (af *AlvuFile) ReadFile() error {
	filecontent, err := os.ReadFile(af.sourcePath)
	if err != nil {
		return fmt.Errorf("error reading file, error: %v", err)
	}
	af.content = filecontent
	return nil
}

func (af *AlvuFile) ParseMeta() error {
	sep := []byte("---")
	if !bytes.HasPrefix(af.content, sep) {
		af.writeableContent = af.content
		return nil
	}

	metaParts := bytes.SplitN(af.content, sep, 3)

	var meta map[string]interface{}
	err := yaml.Unmarshal([]byte(metaParts[1]), &meta)
	if err != nil {
		return err
	}

	af.meta = meta
	af.writeableContent = []byte(metaParts[2])

	return nil
}

func (af *AlvuFile) ProcessFile(hook *lua.LState) error {
	// pre process hook => should return back json with `content` and `data`
	af.lock.Lock()
	defer af.lock.Unlock()

	af.targetName = regexp.MustCompile(`\.md$`).ReplaceAll([]byte(af.name), []byte(".html"))
	onDebug(func() {
		debugInfo(af.name + " will be changed to " + string(af.targetName))
	})

	buf := bytes.NewBuffer([]byte(""))
	mdToHTML := ""

	if filepath.Ext(af.name) == ".md" {
		newName := strings.Replace(af.name, filepath.Ext(af.name), ".html", 1)
		af.targetName = []byte(newName)
		mdProcessor.Convert(af.writeableContent, buf)
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
		Name:             string(af.targetName),
		SourcePath:       af.sourcePath,
		DestPath:         af.destPath,
		Meta:             af.meta,
		WriteableContent: string(af.writeableContent),
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
		af.writeableContent = []byte(stringVal)
	}

	if fromPlug["name"] != nil {
		af.targetName = []byte(fmt.Sprintf("%v", fromPlug["name"]))
	}

	if fromPlug["data"] != nil {
		af.data = mergeMapWithCheck(af.data, fromPlug["data"])
	}

	if fromPlug["extras"] != nil {
		af.extras = mergeMapWithCheck(af.extras, fromPlug["extras"])
	}

	hook.Pop(1)
	return nil
}

func (af *AlvuFile) FlushFile() {
	destFolder := filepath.Dir(af.destPath)
	os.MkdirAll(destFolder, os.ModePerm)

	targetFile := strings.Replace(path.Join(af.destPath), af.name, string(af.targetName), 1)
	onDebug(func() {
		debugInfo("flushing for file: " + af.name + string(af.targetName))
		debugInfo("flusing file: " + targetFile)
	})

	f, err := os.Create(targetFile)
	bail(err)
	defer f.Sync()

	writeHeadTail := false

	if af.baseTemplate == nil && (filepath.Ext(af.sourcePath) == ".md" || filepath.Ext(af.sourcePath) == "html") {
		writeHeadTail = true
	}

	if writeHeadTail && af.headFile != nil {
		shouldCopyContentsWithReset(af.headFile, f)
	}

	renderData := PageRenderData{
		Meta: SiteMeta{
			BaseURL: baseurl,
		},
		Data:   af.data,
		Extras: af.extras,
	}

	// Run the Markdown file through the conversion
	// process to be able to use template variables in
	// the markdown instead of writing them in
	// raw HTML
	var preConvertHTML bytes.Buffer
	preConvertTmpl := textTmpl.New("temporary_pre_template")
	preConvertTmpl.Parse(string(af.writeableContent))
	err = preConvertTmpl.Execute(&preConvertHTML, renderData)
	bail(err)

	var toHtml bytes.Buffer
	err = mdProcessor.Convert(preConvertHTML.Bytes(), &toHtml)
	bail(err)

	layoutData := LayoutRenderData{
		PageRenderData: renderData,
		Content:        template.HTML(toHtml.Bytes()),
	}

	// If a layout file was found
	// write the converted html content into the
	// layout template file
	if af.baseTemplate != nil {
		layout := template.New("layout")
		layoutTemplateData := string(readFileToBytes(af.baseTemplate))
		layoutTemplateData = _injectLiveReload(&layoutTemplateData)
		toHtml.Reset()
		layout.Parse(layoutTemplateData)
		layout.Execute(&toHtml, layoutData)
	}

	io.Copy(
		f, &toHtml,
	)

	if writeHeadTail && af.tailFile != nil && af.baseTemplate == nil {
		shouldCopyContentsWithReset(af.tailFile, f)
	}

	data, err := os.ReadFile(targetFile)
	bail(err)

	onDebug(func() {
		debugInfo("template path: %v", af.sourcePath)
	})

	t := template.New(path.Join(af.sourcePath))
	t.Parse(string(data))

	f.Seek(0, 0)

	err = t.Execute(f, renderData)
	bail(err)
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
	cs := &color.ColorString{}
	fmt.Fprintln(os.Stderr, cs.Red(logPrefix).Red(": "+err.Error()).String())
	panic("")
}

func debugInfo(msg string, a ...any) {
	cs := &color.ColorString{}
	prefix := logPrefix
	baseMessage := cs.Reset("").Yellow(prefix).Reset(" ").Gray(msg).String()
	fmt.Fprintf(os.Stdout, baseMessage+" \n", a...)
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

func readFileToBytes(fd *os.File) []byte {
	buf := &bytes.Buffer{}
	fd.Seek(0, 0)
	_, err := io.Copy(buf, fd)
	bail(err)
	return buf.Bytes()
}

func shouldCopyContentsWithReset(src *os.File, target *os.File) {
	src.Seek(0, 0)
	_, err := io.Copy(target, src)
	bail(err)
}

func ServeHandler(rw http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	if path == "/" {
		path = filepath.Join(outPath, "index.html")
		http.ServeFile(rw, req, path)
		return
	}

	// check if the requested file already exists
	file := filepath.Join(outPath, path)
	info, err := os.Stat(file)

	// if not, check if it's a directory
	// and if it's a directory, we look for
	// a index.html inside the directory to return instead
	if err == nil {
		if info.Mode().IsDir() {
			file = filepath.Join(outPath, path, "index.html")
			_, err := os.Stat(file)
			if err != nil {
				notFoundHandler(rw, req)
				return
			}
		}

		http.ServeFile(rw, req, file)
		return
	}

	// if neither a directory or file was found
	// try a secondary case where the file might be missing
	// a `.html` extension for cleaner url so append a .html
	// to look for the file.
	if err != nil {
		file := filepath.Join(outPath, normalizeFilePath(path))
		_, err := os.Stat(file)

		if err != nil {
			notFoundHandler(rw, req)
			return
		}

		http.ServeFile(rw, req, file)
		return
	}

	notFoundHandler(rw, req)
}

// _webSocketHandler Internal function to setup a listener loop
// for the live reload setup
func _webSocketHandler(ws *websocket.Conn) {
	reloadCh = append(reloadCh, make(chan bool, 1))
	currIndex := len(reloadCh) - 1

	defer ws.Close()

	for range reloadCh[currIndex] {
		err := websocket.Message.Send(ws, "reload")
		if err != nil {
			// For debug only
			// log.Printf("Error sending message: %s", err.Error())
			break
		}
		onDebug(func() {
			debugInfo("Reload message sent")
		})
	}

}

func AddWebsocketHandler() {
	wsHandler := websocket.Handler(_webSocketHandler)

	// Use a custom HTTP handler function to upgrade the HTTP request to WebSocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Check the request's 'Upgrade' header to see if it's a WebSocket request
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "Not a WebSocket handshake request", http.StatusBadRequest)
			return
		}

		// Upgrade the HTTP connection to a WebSocket connection
		wsHandler.ServeHTTP(w, r)
	})

}

// _clientNotifyReload Internal function to
// report changes to all possible reload channels
func _clientNotifyReload() {
	for ind := range reloadCh {
		reloadCh[ind] <- true
	}
}

func normalizeFilePath(path string) string {
	if strings.HasSuffix(path, ".html") {
		return path
	}
	return path + ".html"
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, "404, Page not found....")
}

func Contains(collection []string, item string) bool {
	for _, x := range collection {
		if item == x {
			return true
		}
	}
	return false
}

// Watcher , create an interface over the fsnotify watcher
// to be able to run alvu compile processes again
// FIXME: redundant compile process for the files
type Watcher struct {
	alvu   *Alvu
	notify *fsnotify.Watcher
	dirs   []string
}

func NewWatcher(alvu *Alvu) *Watcher {
	watcher := &Watcher{
		alvu: alvu,
	}
	notifier, err := fsnotify.NewWatcher()
	bail(err)
	watcher.notify = notifier
	return watcher
}

func (w *Watcher) AddDir(dirPath string) {

	for _, pth := range w.dirs {
		if pth == dirPath {
			return
		}
	}

	w.dirs = append(w.dirs, dirPath)
	err := w.notify.Add(dirPath)
	bail(err)
}

func (w *Watcher) RebuildAlvu() {
	w.alvu.CopyPublic()
	w.alvu.Build()
}

func (w *Watcher) RebuildFile(filePath string) {
	for i, af := range w.alvu.files {
		if af.sourcePath != filePath {
			continue
		}

		w.alvu.files[i].Build()
	}
}

func (w *Watcher) StartWatching() {

	go func() {
		for {
			select {
			case event, ok := <-w.notify.Events:
				if !ok {
					return
				}

				onDebug(func() {
					debugInfo("Events registered")
				})

				if event.Op&fsnotify.Write == fsnotify.Write {
					onDebug(func() {
						debugInfo("File Changed")
					})

					recompiledText := &color.ColorString{}
					recompiledText.Blue(logPrefix).Green("Recompiled!").Reset(" ")

					_, err := os.Stat(event.Name)
					// Do nothing if the file doesn't exit, just continue
					if err != nil {
						continue
					}

					// If alvu file then just build the file, else
					// just rebuilt the whole folder since it could
					// be a file from the public folder or the _layout file
					if w.alvu.IsAlvuFile(event.Name) {
						recompilingText := &color.ColorString{}
						recompilingText.Blue(logPrefix).Cyan("Recompiling: ").Gray(event.Name).Reset(" ")
						fmt.Println(recompilingText.String())
						w.RebuildFile(event.Name)
					} else {
						recompilingText := &color.ColorString{}
						recompilingText.Blue(logPrefix).Cyan("Recompiling: ").Gray("All").Reset(" ")
						fmt.Println(recompilingText.String())
						w.RebuildAlvu()
					}

					_clientNotifyReload()
					fmt.Println(recompiledText.String())
					continue
				}
			case err, ok := <-w.notify.Errors:
				if !ok {
					return
				}
				fmt.Println("Error happened 😢", err)
			}
		}
	}()
}

func _injectLiveReload(layoutHTML *string) string {
	if !*serveFlag {
		return *layoutHTML
	}
	return *layoutHTML + `<script>
				  const socket = new WebSocket("ws://localhost:3000/ws");
			
				  // Connection opened
				  socket.addEventListener("open", (event) => {
					socket.send("Hello Server!");
				  });
			
				  // Listen for messages
				  socket.addEventListener("message", (event) => {
					if (event.data == "reload") {
					  socket.close();
					  window.location.reload();
					}
				  });
			</script>`
}

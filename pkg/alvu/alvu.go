package alvu

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	templateHTML "html/template"

	"github.com/barelyhuman/alvu/transformers"
	"github.com/barelyhuman/alvu/transformers/markdown"
	"golang.org/x/net/websocket"

	htmlT "github.com/barelyhuman/alvu/transformers/html"
)

// Constants
const slotStartTag = "<slot>"
const slotEndTag = "</slot>"
const contentTag = "{{.Content}}"

type SiteMeta struct {
	BaseURL string
}

type PageRenderData struct {
	Meta   SiteMeta
	Data   map[string]interface{}
	Extras map[string]interface{}
}

type AlvuConfig struct {
	HookDir  string
	OutDir   string
	RootPath string

	BaseURL            string
	EnableHardWrap     bool
	EnableHighlighting bool
	HighlightingTheme  string

	Serve        bool
	PollDuration int
	PortNumber   string

	Transformers map[string][]transformers.Transfomer

	// Internals
	logger      Logger
	hookHandler *Hooks
	watcher     *Watcher
	rebuildChan chan bool
}

func (ac *AlvuConfig) Rebuild(path string) {
	ac.logger.Info(fmt.Sprintf("Changed: %v, Recompiling.", path))
	err := ac.Build()
	if err != nil {
		ac.logger.Error(err.Error())
	}
	ac.rebuildChan <- true
}

func (ac *AlvuConfig) Run() error {
	ac.rebuildChan = make(chan bool, 1)
	ac.logger = Logger{
		logPrefix: "[alvu]",
	}

	if ac.Serve {
		ac.watcher = NewWatcher()
		ac.watcher.logger = ac.logger
		go func(ac *AlvuConfig) {
			for path := range ac.watcher.recompile {
				ac.Rebuild(path)
			}
		}(ac)
	}

	err := ac.Build()
	if err != nil {
		return err
	}

	if ac.Serve {
		ac.watcher.Start()
	}

	return ac.StartServer()
}

func (ac *AlvuConfig) Build() error {
	hooksHandler := Hooks{
		ac: *ac,
	}
	ac.hookHandler = &hooksHandler

	ac.Transformers = map[string][]transformers.Transfomer{
		".html": {
			&htmlT.HTMLTransformer{},
		},
		".md": {
			&markdown.MarkdownTransformer{
				EnableHardWrap:     ac.EnableHardWrap,
				EnableHighlighting: ac.EnableHighlighting,
				HighlightingTheme:  ac.HighlightingTheme,
				BaseURL:            ac.BaseURL,
			},
		},
	}

	pageDir := filepath.Join(ac.RootPath, "pages")
	publicDir := filepath.Join(ac.RootPath, "public")

	filesToProcess, err := ac.ReadDir(pageDir)
	if err != nil {
		return err
	}

	ac.logger.Debug(fmt.Sprintf("filesToProcess: %v", filesToProcess))

	publicFiles, err := ac.ReadDir(publicDir)
	if err != nil {
		return err
	}

	ac.watcher.AddDir(pageDir)
	ac.watcher.AddDir(publicDir)

	normalizedFiles, err := runTransfomers(filesToProcess, ac)
	if err != nil {
		return err
	}

	var processedFiles []HookedFile

	ac.hookHandler.Load()

	ac.hookHandler.runLifeCycleHooks("OnStart")

	for _, tf := range normalizedFiles {
		processedFiles = append(processedFiles, hooksHandler.ProcessFile(tf))
	}

	ac.HandlePublicFiles(publicFiles)
	return ac.FlushFiles(processedFiles)
}

func (ac *AlvuConfig) ReadLayout() string {
	layoutFilePath := filepath.Join(ac.RootPath, "pages", "_layout.html")
	fileInfo, err := os.Stat(layoutFilePath)
	defaultLayout := ""

	if ac.Serve {
		defaultLayout = injectWebsocketConnection(defaultLayout, ac.PortNumber)
	}

	if os.IsNotExist(err) {
		return defaultLayout
	}
	if fileInfo.IsDir() {
		return defaultLayout
	}
	data, _ := os.ReadFile(
		layoutFilePath,
	)

	if ac.Serve {
		return injectWebsocketConnection(string(data), ac.PortNumber)
	}

	return string(data)
}

func (ac *AlvuConfig) HandlePublicFiles(files []string) (err error) {
	var wg sync.WaitGroup
	for _, v := range files {
		wg.Add(1)
		file := v
		go func() {
			destFile := filepath.Clean(file)
			destFile = strings.TrimPrefix(destFile, filepath.Join(ac.RootPath, "public"))
			destFile = filepath.Join(ac.OutDir, destFile)
			os.MkdirAll(filepath.Dir(destFile), os.ModePerm)

			fileToCreate, _ := os.Create(destFile)
			reader, _ := os.OpenFile(file, os.O_RDONLY, os.ModePerm)
			io.Copy(fileToCreate, reader)
			wg.Done()
		}()
	}
	wg.Wait()
	return
}

func (ac *AlvuConfig) createTransformedFile(filePath string, content string) (tranformedFile transformers.TransformedFile, err error) {
	fileExt := filepath.Ext(filePath)
	fileWriter, err := os.CreateTemp("", "alvu-")
	if err != nil {
		return
	}
	defer fileWriter.Close()

	_, err = fileWriter.WriteString(content)
	if err != nil {
		return
	}
	tranformedFile.TransformedFile = fileWriter.Name()
	tranformedFile.SourcePath = filePath
	tranformedFile.Extension = fileExt
	return
}

func (ac *AlvuConfig) FlushFiles(files []HookedFile) error {
	if err := os.MkdirAll(ac.OutDir, os.ModePerm); err != nil {
		return err
	}

	if hasLegacySlot(ac.ReadLayout()) {
		ac.logger.Warning("Please use `<slot></slot>` instead of `{{.Content}}` in _layout.html")
	}

	for i := range files {
		hookedFile := files[i]
		originalDir, baseFile := filepath.Split(hookedFile.SourcePath)
		newDir := strings.TrimPrefix(originalDir, filepath.Join(ac.RootPath, "pages"))
		fileWithNewExtension := strings.TrimSuffix(baseFile, hookedFile.Extension) + ".html"
		destFile := filepath.Join(
			ac.OutDir,
			newDir,
			fileWithNewExtension,
		)

		ac.logger.Debug(fmt.Sprintf("originalFile:%v, desFile: %v", hookedFile.SourcePath, destFile))

		err := os.MkdirAll(filepath.Dir(destFile), os.ModePerm)
		if err != nil {
			return err
		}

		destWriter, err := os.Create(destFile)
		if err != nil {
			return err
		}
		defer destWriter.Close()

		if len(hookedFile.transform) > 1 {
			for _, t := range ac.Transformers[hookedFile.transform] {
				afterTransform, err := t.TransformContent(hookedFile.content)
				if err != nil {
					return err
				}
				hookedFile.content = afterTransform
			}
		}

		replaced, err := ac.injectInSlot(
			ac.ReadLayout(),
			string(hookedFile.content),
		)

		if err != nil {
			return err
		}

		template := templateHTML.New("temporaryTemplate")
		template = template.Funcs(templateHTML.FuncMap{
			"transform": func(extension string, content string) templateHTML.HTML {
				var transformed []byte = []byte(content)
				for _, t := range ac.Transformers[extension] {
					transformed, _ = t.TransformContent(transformed)
				}
				return templateHTML.HTML(transformed)
			},
		})
		template, err = template.Parse(replaced)
		if err != nil {
			ac.logger.Error(fmt.Sprintf("Failed to write to dist file with error: %v", err))
			panic("")
		}

		renderData := PageRenderData{
			Meta: SiteMeta{
				BaseURL: ac.BaseURL,
			},
			Data:   hookedFile.data,
			Extras: hookedFile.extras,
		}

		err = template.Execute(destWriter, renderData)
		if err != nil {
			return err
		}
	}

	ac.logger.Info("Output in: " + ac.OutDir)
	ac.logger.Success("Done")
	ac.hookHandler.runLifeCycleHooks("OnFinish")

	return nil
}

func runTransfomers(filesToProcess []string, ac *AlvuConfig) ([]transformers.TransformedFile, error) {
	normalizedFiles := []transformers.TransformedFile{}

	for _, fileToNormalize := range filesToProcess {
		extension := filepath.Ext(fileToNormalize)

		if len(ac.Transformers[extension]) < 1 {
			continue
		}

		originalContent, err := os.ReadFile(fileToNormalize)
		mutableContent := originalContent
		if err != nil {
			return nil, fmt.Errorf("failed to read file %v with error %v", fileToNormalize, err)
		}

		var meta map[string]interface{}
		for _, transformer := range ac.Transformers[extension] {
			nextContent, err := transformer.TransformContent(mutableContent)
			if err != nil {
				return nil, fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
			}
			newMeta, _, _ := transformer.ExtractMeta(originalContent)
			if err != nil {
				return nil, fmt.Errorf("failed to extract meta from file: %v, with error: %v", fileToNormalize, err)
			}
			if hasKeys(newMeta) {
				meta = newMeta
			}
			mutableContent = nextContent
		}

		transformedFile, err := ac.createTransformedFile(fileToNormalize, string(mutableContent))
		if err != nil {
			return nil, fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
		}

		transformedFile.Meta = meta
		normalizedFiles = append(normalizedFiles, transformedFile)
	}
	return normalizedFiles, nil
}

func (ac *AlvuConfig) ReadDir(dir string) (filepaths []string, err error) {
	readFilepaths, err := recursiveRead(dir)
	if err != nil {
		return
	}
	sanitizedCollection := []string{}
	for _, v := range readFilepaths {
		if filepath.Base(v) == "_layout.html" {
			continue
		}
		sanitizedCollection = append(sanitizedCollection, v)
	}
	return sanitizedCollection, nil
}

func (ac *AlvuConfig) injectInSlot(htmlString string, replacement string) (string, error) {
	if hasLegacySlot(htmlString) {
		return injectInLegacySlot(htmlString, replacement), nil
	}
	slotStartPos := strings.Index(htmlString, slotStartTag)
	slotEndPos := strings.Index(htmlString, slotEndTag)
	if slotStartPos == -1 && slotEndPos == -1 {
		return htmlString, nil
	}
	baseString := strings.Replace(htmlString, slotEndTag, "", slotEndPos)
	return strings.Replace(baseString, slotStartTag, replacement, slotStartPos), nil
}

func (ac *AlvuConfig) NormalisePort(port string) string {
	normalisedPort := port

	if !strings.HasPrefix(normalisedPort, ":") {
		normalisedPort = ":" + normalisedPort
	}

	return normalisedPort
}

func (ac *AlvuConfig) StartServer() error {
	if !ac.Serve {
		return nil
	}

	wsHandler := websocket.Handler(ac._webSocketHandler)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Check the request's 'Upgrade' header to see if it's a WebSocket request
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "Not a WebSocket handshake request", http.StatusBadRequest)
			return
		}

		// Upgrade the HTTP connection to a WebSocket connection
		wsHandler.ServeHTTP(w, r)
	})

	normalisedPort := ac.NormalisePort(ac.PortNumber)

	http.Handle("/", http.HandlerFunc(ac.ServeHandler))
	ac.logger.Info(fmt.Sprintf("Starting Server - %v:%v", "http://localhost", ac.PortNumber))

	err := http.ListenAndServe(normalisedPort, nil)
	if strings.Contains(err.Error(), "address already in use") {
		ac.logger.Error("port already in use, use another port with the `-port` flag instead")
		return err
	}

	return nil
}

// _webSocketHandler Internal function to setup a listener loop
// for the live reload setup
func (ac *AlvuConfig) _webSocketHandler(ws *websocket.Conn) {
	defer ws.Close()
	for range ac.rebuildChan {
		err := websocket.Message.Send(ws, "reload")
		if err != nil {
			break
		}
	}
}

func (ac *AlvuConfig) ServeHandler(rw http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	if path == "/" {
		path = filepath.Join(ac.OutDir, "index.html")
		http.ServeFile(rw, req, path)
		return
	}

	// check if the requested file already exists
	file := filepath.Join(ac.OutDir, path)
	info, err := os.Stat(file)

	// if not, check if it's a directory
	// and if it's a directory, we look for
	// a index.html inside the directory to return instead
	if err == nil {
		if info.Mode().IsDir() {
			file = filepath.Join(ac.OutDir, path, "index.html")
			_, err := os.Stat(file)
			if err != nil {
				ac.notFoundHandler(rw, req)
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
		file := filepath.Join(ac.OutDir, normalizeStaticLookupPath(path))
		_, err := os.Stat(file)

		if err != nil {
			ac.notFoundHandler(rw, req)
			return
		}

		http.ServeFile(rw, req, file)
		return
	}

	ac.notFoundHandler(rw, req)
}

func (ac *AlvuConfig) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	var notFoundPageExists bool
	filePointer, err := os.Stat(filepath.Join(ac.OutDir, "404.html"))
	if err != nil {
		if os.IsNotExist(err) {
			notFoundPageExists = false
		}
	}

	if filePointer.Size() > 0 {
		notFoundPageExists = true
	}

	if notFoundPageExists {
		compiledNotFoundFile := filepath.Join(ac.OutDir, "404.html")
		notFoundFile, err := os.ReadFile(compiledNotFoundFile)
		if err != nil {
			http.Error(w, "404, Page not found....", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(notFoundFile)
		return
	}

	http.Error(w, "404, Page not found....", http.StatusNotFound)
}

func normalizeStaticLookupPath(path string) string {
	if strings.HasSuffix(path, ".html") {
		return path
	}
	return path + ".html"
}

func recursiveRead(dir string) (filepaths []string, err error) {
	dirEntry, err := os.ReadDir(
		dir,
	)

	if err != nil {
		return
	}

	for _, de := range dirEntry {
		if de.IsDir() {
			subDirs, err := recursiveRead(filepath.Join(dir, de.Name()))
			if err != nil {
				return filepaths, err
			}
			filepaths = append(filepaths, subDirs...)
		} else {
			filepaths = append(filepaths, filepath.Join(dir, de.Name()))
		}
	}

	return
}

func hasLegacySlot(htmlString string) bool {
	return strings.Contains(htmlString, contentTag)
}

func injectInLegacySlot(htmlString string, replacement string) string {
	contentTagPos := strings.Index(htmlString, contentTag)
	if contentTagPos == -1 {
		return htmlString
	}
	return strings.Replace(htmlString, contentTag, replacement, contentTagPos)
}

func injectWebsocketConnection(htmlString string, port string) string {
	return htmlString + fmt.Sprintf(`<script>
	const socket = new WebSocket("ws://localhost:%v/ws");

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
</script>`, port)
}

func hasKeys(i map[string]interface{}) bool {
	keys := make([]string, 0, len(i))
	for k := range i {
		keys = append(keys, k)
	}
	return len(keys) > 0
}

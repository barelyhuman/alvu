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
}

func (ac *AlvuConfig) Run() error {
	ac.logger = Logger{
		logPrefix: "[alvu]",
	}

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

	filesToProcess, err := ac.ReadDir(filepath.Join(ac.RootPath, "pages"))
	if err != nil {
		return err
	}

	ac.logger.Debug(fmt.Sprintf("filesToProcess: %v", filesToProcess))

	publicFiles, err := ac.ReadDir(filepath.Join(ac.RootPath, "public"))
	if err != nil {
		return err
	}

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
	err = ac.FlushFiles(processedFiles)
	if err != nil {
		return err
	}

	return ac.StartServer()
}

func (ac *AlvuConfig) ReadLayout() string {
	layoutFilePath := filepath.Join(ac.RootPath, "pages", "_layout.html")
	fileInfo, err := os.Stat(layoutFilePath)
	if os.IsNotExist(err) {
		return ""
	}
	if fileInfo.IsDir() {
		return ""
	}
	data, _ := os.ReadFile(
		layoutFilePath,
	)
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
		if err != nil {
			return nil, fmt.Errorf("failed to read file %v with error %v", fileToNormalize, err)
		}

		for _, transformer := range ac.Transformers[extension] {
			nextContent, err := transformer.TransformContent(originalContent)
			if err != nil {
				return nil, fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
			}
			originalContent = nextContent
		}

		transformedFile, err := ac.createTransformedFile(fileToNormalize, string(originalContent))
		if err != nil {
			return nil, fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
		}

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

func (ac *AlvuConfig) StartServer() error {
	if !ac.Serve {
		return nil
	}

	normalizedPort := string(ac.PortNumber)

	if !strings.HasPrefix(normalizedPort, ":") {
		normalizedPort = ":" + normalizedPort
	}

	http.Handle("/", http.HandlerFunc(ac.ServeHandler))
	ac.logger.Info(fmt.Sprintf("Starting Server - %v:%v", "http://localhost", ac.PortNumber))

	err := http.ListenAndServe(normalizedPort, nil)
	if strings.Contains(err.Error(), "address already in use") {
		ac.logger.Error("port already in use, use another port with the `-port` flag instead")
		return err
	}

	return nil
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

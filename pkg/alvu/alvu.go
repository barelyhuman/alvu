package alvu

import (
	"fmt"
	"io"
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
	return ac.FlushFiles(processedFiles)
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
			destFile = strings.TrimPrefix(destFile, "public")
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
		newDir := strings.TrimPrefix(originalDir, "pages")
		fileWithNewExtension := strings.TrimSuffix(baseFile, hookedFile.Extension) + ".html"
		destFile := filepath.Join(
			ac.OutDir,
			newDir,
			fileWithNewExtension,
		)

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

		replaced, _ := ac.injectInSlot(
			ac.ReadLayout(),
			string(hookedFile.content),
		)

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

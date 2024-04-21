package alvu

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/barelyhuman/alvu/transformers"
	"github.com/barelyhuman/alvu/transformers/markdown"
	"golang.org/x/net/html"

	htmlT "github.com/barelyhuman/alvu/transformers/html"
)

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
	_layoutBuffer bytes.Buffer
}

func (ac *AlvuConfig) Run() error {
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

	publicFiles, err := ac.ReadDir(filepath.Join(ac.RootPath, "public"))
	if err != nil {
		return err
	}

	normalizedFiles, err := runTransfomers(filesToProcess, ac)
	if err != nil {
		return err
	}

	processedFiles := normalizedFiles

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

func (ac *AlvuConfig) FlushFiles(files []transformers.TransformedFile) error {
	if err := os.MkdirAll(ac.OutDir, os.ModePerm); err != nil {
		return err
	}

	for _, tf := range files {
		originalDir, baseFile := filepath.Split(tf.SourcePath)
		newDir := strings.TrimPrefix(originalDir, "pages")
		fileWithNewExtension := strings.TrimSuffix(baseFile, tf.Extension) + ".html"
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

		sourceFileData, err := os.ReadFile(tf.TransformedFile)
		if err != nil {
			return err
		}

		replaced, _ := replaceBodyTag(
			ac.ReadLayout(),
			string(sourceFileData),
		)

		_, err = io.Copy(destWriter, bytes.NewBuffer([]byte(replaced)))

		if err != nil {
			return err
		}
	}

	return nil
}

func runTransfomers(filesToProcess []string, ac *AlvuConfig) ([]transformers.TransformedFile, error) {
	normalizedFiles := []transformers.TransformedFile{}

	for _, fileToNormalize := range filesToProcess {
		extension := filepath.Ext(fileToNormalize)

		if len(ac.Transformers[extension]) < 1 {
			continue
		}

		for _, transformer := range ac.Transformers[extension] {
			transformedFile, err := transformer.Transform(fileToNormalize)
			if err != nil {
				return nil, fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
			}
			normalizedFiles = append(normalizedFiles, transformedFile)
		}
	}
	return normalizedFiles, nil
}

func (ac *AlvuConfig) ReadDir(dir string) (dirs []string, err error) {
	return recursiveRead(dir)
}

func recursiveRead(dir string) (dirs []string, err error) {
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
				return dirs, err
			}
			dirs = append(dirs, subDirs...)
		} else {
			dirs = append(dirs, filepath.Join(dir, de.Name()))
		}
	}

	return
}

func replaceBodyTag(htmlString string, replacement string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlString))
	if err != nil {
		return "", err
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			replaceNodes, _ := html.ParseFragment(strings.NewReader(replacement), n)
			for _, childNodes := range replaceNodes {
				n.AppendChild(childNodes)
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		return "", err
	}

	return buf.String(), nil
}

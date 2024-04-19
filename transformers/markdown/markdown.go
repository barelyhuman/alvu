package markdown

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/barelyhuman/alvu/transformers"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
)

type MarkdownTransformer struct {
	processor          goldmark.Markdown
	EnableHardWrap     bool
	EnableHighlighting bool
	HighlightingTheme  string
}

func (mt *MarkdownTransformer) Transform(filePath string) (transformedFile transformers.TransformedFile, err error) {
	if mt.processor == nil {
		mt.Init()
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	tmpDir := os.TempDir()
	filename := filepath.Base(filePath)
	fileExt := filepath.Ext(filename)
	filename = strings.TrimSuffix(filename, fileExt)
	filename += ".html"
	tmpFile := filepath.Join(tmpDir, filename)

	fileWriter, err := os.Create(tmpFile)
	if err != nil {
		return
	}

	err = mt.processor.Convert(fileBytes, fileWriter)
	if err != nil {
		return
	}

	return transformers.TransformedFile{
		SourcePath:      filePath,
		TransformedFile: tmpFile,
		Extension:       fileExt,
	}, nil
}

func (mt *MarkdownTransformer) Init() {
	rendererOptions := []renderer.Option{
		html.WithXHTML(),
		html.WithUnsafe(),
	}

	if mt.EnableHardWrap {
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

	if mt.EnableHighlighting {
		gmPlugins = append(gmPlugins, goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle(mt.HighlightingTheme),
			),
		))
	}

	mt.processor = goldmark.New(gmPlugins...)
}

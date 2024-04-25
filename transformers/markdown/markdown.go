package markdown

import (
	"bytes"

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

func (mt *MarkdownTransformer) TransformContent(input []byte) (result []byte, err error) {
	mt.EnsureProcessor()

	var buffer bytes.Buffer
	err = mt.processor.Convert(input, &buffer)
	if err != nil {
		return
	}

	result = buffer.Bytes()
	return
}

func (mt *MarkdownTransformer) EnsureProcessor() {
	if mt.processor != nil {
		return
	}

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

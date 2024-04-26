package markdown

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/barelyhuman/go/color"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type MarkdownTransformer struct {
	processor          goldmark.Markdown
	EnableHardWrap     bool
	EnableHighlighting bool
	HighlightingTheme  string
	BaseURL            string
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

	linkRewriter := &relativeLinkRewriter{
		baseURL: mt.BaseURL,
	}

	gmPlugins := []goldmark.Option{
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.Prioritized(linkRewriter, 100)),
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

type relativeLinkRewriter struct {
	baseURL string
}

func (rlr *relativeLinkRewriter) Transform(doc *ast.Document, reader text.Reader, pctx parser.Context) {
	ast.Walk(doc, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}

		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		validURL, _ := url.Parse(string(link.Destination))

		if validURL.Scheme == "http" || validURL.Scheme == "https" || validURL.Scheme == "mailto" {
			return ast.WalkContinue, nil
		}

		if strings.HasPrefix(validURL.Path, "{{.Meta.BaseURL}}") {
			newDestination, _ := url.JoinPath(
				rlr.baseURL,
				strings.TrimPrefix(validURL.Path, "{{.Meta.BaseURL}}"),
			)
			link.Destination = []byte(newDestination)
			printMetaLinkWarning()
		} else if strings.HasPrefix(validURL.Path, "/") {
			// from root
			newDestination, _ := url.JoinPath(
				rlr.baseURL,
				validURL.Path,
			)
			link.Destination = []byte(newDestination)
		}

		return ast.WalkSkipChildren, nil
	})
}

// TODO: remove in v0.3
func printMetaLinkWarning() {
	warning := "{{.Meta.BaseURL}} is no more needed in markdown files, links will be rewritten automatically.\n Use root first links, eg: pages/docs/some-topic.md would be linked as /docs/some-topic"
	cs := color.ColorString{}
	cs.Reset(" ").Yellow(warning)
	fmt.Println(cs.String())
}

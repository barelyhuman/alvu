package alvu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barelyhuman/alvu/transformers"
	"github.com/barelyhuman/alvu/transformers/markdown"
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
}

func (ac *AlvuConfig) Run() error {
	ac.Transformers = map[string][]transformers.Transfomer{
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

	normalizedFiles := []transformers.TransformedFile{}

	// transform phase
	for _, fileToNormalize := range filesToProcess {
		extension := filepath.Ext(fileToNormalize)

		if len(ac.Transformers[extension]) < 1 {
			continue
		}
		for _, transformer := range ac.Transformers[extension] {
			transformedFile, err := transformer.Transform(fileToNormalize)
			if err != nil {
				return fmt.Errorf("failed to transform file: %v, with error: %v", fileToNormalize, err)
			}
			normalizedFiles = append(normalizedFiles, transformedFile)
		}
	}

	fmt.Printf("normalizedFiles: %v\n", normalizedFiles)

	return nil
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

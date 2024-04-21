package html

import (
	"path/filepath"

	"github.com/barelyhuman/alvu/transformers"
)

type HTMLTransformer struct {
}

func (mt *HTMLTransformer) Transform(filePath string) (transformedFile transformers.TransformedFile, err error) {
	return transformers.TransformedFile{
		SourcePath:      filePath,
		TransformedFile: filePath,
		Extension:       filepath.Ext(filePath),
	}, nil
}

func (mt *HTMLTransformer) Init() {

}

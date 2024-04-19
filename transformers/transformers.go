package transformers

type TransformedFile struct {
	SourcePath      string
	TransformedFile string
	Extension       string
}

type Transfomer interface {
	Transform(filePath string) (TransformedFile, error)
}

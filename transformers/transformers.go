package transformers

type TransformedFile struct {
	SourcePath      string
	TransformedFile string
	Extension       string
}

type Transfomer interface {
	TransformContent(data []byte) ([]byte, error)
}

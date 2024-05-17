package transformers

type TransformedFile struct {
	SourcePath      string
	TransformedFile string
	Extension       string
	Meta            map[string]interface{}
}

type Transfomer interface {
	TransformContent(data []byte) ([]byte, error)
	ExtractMeta(data []byte) (map[string]interface{}, []byte, error)
}

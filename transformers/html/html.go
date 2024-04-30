package html

type HTMLTransformer struct{}

func (mt *HTMLTransformer) TransformContent(input []byte) (result []byte, err error) {
	result = input
	return
}

func (mt *HTMLTransformer) ExtractMeta(input []byte) (result map[string]interface{}, content []byte, err error) {
	result = map[string]interface{}{}
	return
}

func (mt *HTMLTransformer) Init() {}

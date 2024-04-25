package html

type HTMLTransformer struct{}

func (mt *HTMLTransformer) TransformContent(input []byte) (result []byte, err error) {
	result = input
	return
}

func (mt *HTMLTransformer) Init() {}

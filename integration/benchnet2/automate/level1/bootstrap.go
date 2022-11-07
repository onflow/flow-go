package level1

type Bootstrap struct {
}

func NewBootstrap(bootstrapPath string) Bootstrap {
	return Bootstrap{}
}

func (b *Bootstrap) GenTemplateData(outputToFile bool) string {
	return "{}"
}

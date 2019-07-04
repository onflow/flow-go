package runtime

// Runtime is a runtime capable of executing the Bamboo programming language.
type Runtime interface {
	ExecuteScript(script []byte, readRegister func(string) []byte, writeRegister func(string, []byte)) bool
}

// NewMockRuntime returns a mocked version of the Bamboo runtime.
func NewMockRuntime() Runtime {
	return &mock{}
}

type mock struct {
}

func (m *mock) ExecuteScript(script []byte, readRegister func(string) []byte, writeRegister func(string, []byte)) bool {
	return true
}

package runtime

import (
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

type testRuntimeInterface struct {
	getValue func(controller []byte, owner []byte, key []byte) (value []byte, err error)
	setValue func(controller []byte, owner []byte, key []byte, value []byte) (err error)
}

func (i *testRuntimeInterface) GetValue(controller []byte, owner []byte, key []byte) (value []byte, err error) {
	return i.getValue(controller, owner, key)
}

func (i *testRuntimeInterface) SetValue(controller []byte, owner []byte, key []byte, value []byte) (err error) {
	return i.setValue(controller, owner, key, value)
}

func TestNewInterpreterRuntime(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()
	script := []byte(`
        fun main() {
            const controller = [1]
            const owner = [2]
            const key = [3]
            const value = getValue(controller, owner, key)
            setValue(controller, owner, key, value + 2)
		}
    `)

	state := big.NewInt(3)

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller []byte, owner []byte, key []byte) (value []byte, err error) {
			// ignore controller, owner, and key
			return state.Bytes(), nil
		},
		setValue: func(controller []byte, owner []byte, key []byte, value []byte) (err error) {
			// ignore controller, owner, and key
			state.SetBytes(value)
			return nil
		},
	}

	errs := runtime.ExecuteScript(script, runtimeInterface)
	Expect(errs).To(BeEmpty())
	Expect(state.Int64()).To(Equal(int64(5)))
}

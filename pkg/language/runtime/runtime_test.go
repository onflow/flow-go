package runtime

import (
	"math/big"
	"testing"

	. "github.com/onsi/gomega"
)

type testRuntimeInterface struct {
	getValue          func(controller, owner, key []byte) (value []byte, err error)
	setValue          func(controller, owner, key, value []byte) (err error)
	createAccount     func(publicKey, code []byte) (accountID []byte, err error)
	updateAccountCode func(accountID, code []byte) (err error)
}

func (i *testRuntimeInterface) GetValue(controller, owner, key []byte) (value []byte, err error) {
	return i.getValue(controller, owner, key)
}

func (i *testRuntimeInterface) SetValue(controller, owner, key, value []byte) (err error) {
	return i.setValue(controller, owner, key, value)
}

func (i *testRuntimeInterface) CreateAccount(publicKey, code []byte) (accountID []byte, err error) {
	return i.createAccount(publicKey, code)
}

func (i *testRuntimeInterface) UpdateAccountCode(accountID, code []byte) (err error) {
	return i.updateAccountCode(accountID, code)
}

func TestNewInterpreterRuntime(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()
	script := []byte(`
        fun main() {
            let controller = [1]
            let owner = [2]
            let key = [3]
            let value = getValue(controller, owner, key)
            setValue(controller, owner, key, value + 2)
		}
	`)

	state := big.NewInt(3)

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			// ignore controller, owner, and key
			return state.Bytes(), nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			// ignore controller, owner, and key
			state.SetBytes(value)
			return nil
		},
		createAccount: func(key, code []byte) (accountID []byte, err error) {
			return nil, nil
		},
		updateAccountCode: func(accountID, code []byte) (err error) {
			return nil
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(state.Int64()).
		To(Equal(int64(5)))
}

package runtime

import (
	"fmt"
	"math/big"
	"testing"

	. "github.com/onsi/gomega"
)

type testRuntimeInterface struct {
	resolveImport     func(ImportLocation) ([]byte, error)
	getValue          func(controller, owner, key []byte) (value []byte, err error)
	setValue          func(controller, owner, key, value []byte) (err error)
	createAccount     func(publicKey, code []byte) (accountID []byte, err error)
	updateAccountCode func(accountID, code []byte) (err error)
}

func (i *testRuntimeInterface) ResolveImport(location ImportLocation) ([]byte, error) {
	return i.resolveImport(location)
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

func TestRuntimeGetAndSetValue(t *testing.T) {
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

func TestRuntimeImport(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()

	importedScript := []byte(`
       fun answer(): Int {
           return 42
		}
	`)

	script := []byte(`
       import "imported"

       fun main(): Int {
           let answer =  answer()
           if answer != 42 {
               panic("?!")
           }
           return answer
		}
	`)

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location ImportLocation) (bytes []byte, e error) {
			switch location {
			case StringImportLocation("imported"):
				return importedScript, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
	}

	value, err := runtime.ExecuteScript(script, runtimeInterface)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(value).To(Equal(42))
}

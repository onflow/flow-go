package runtime

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/types"
)

type testRuntimeInterface struct {
	resolveImport      func(ImportLocation) ([]byte, error)
	getValue           func(controller, owner, key []byte) (value []byte, err error)
	setValue           func(controller, owner, key, value []byte) (err error)
	createAccount      func(publicKeys [][]byte, keyWeights []int, code []byte) (accountID []byte, err error)
	addAccountKey      func(address types.Address, publicKey []byte, keyWeight int) error
	removeAccountKey   func(address types.Address, index int) error
	updateAccountCode  func(address types.Address, code []byte) (err error)
	getSigningAccounts func() []types.Address
	log                func(string)
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

func (i *testRuntimeInterface) CreateAccount(publicKeys [][]byte, keyWeights []int, code []byte) (accountID []byte, err error) {
	return i.createAccount(publicKeys, keyWeights, code)
}

func (i *testRuntimeInterface) AddAccountKey(address types.Address, publicKey []byte, keyWeight int) error {
	return i.addAccountKey(address, publicKey, keyWeight)
}

func (i *testRuntimeInterface) RemoveAccountKey(address types.Address, index int) error {
	return i.removeAccountKey(address, index)
}

func (i *testRuntimeInterface) UpdateAccountCode(address types.Address, code []byte) (err error) {
	return i.updateAccountCode(address, code)
}

func (i *testRuntimeInterface) GetSigningAccounts() []types.Address {
	if i.getSigningAccounts == nil {
		return nil
	}
	return i.getSigningAccounts()
}

func (i *testRuntimeInterface) Log(message string) {
	i.log(message)
}

func TestRuntimeGetAndSetValue(t *testing.T) {

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
		createAccount: func(publicKeys [][]byte, keyWeights []int, code []byte) (accountID []byte, err error) {
			return nil, nil
		},
		updateAccountCode: func(address types.Address, code []byte) (err error) {
			return nil
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface)

	assert.Nil(t, err)

	assert.Equal(t, int64(5), state.Int64())
}

func TestRuntimeImport(t *testing.T) {

	runtime := NewInterpreterRuntime()

	importedScript := []byte(`
       fun answer(): Int {
           return 42
		}
	`)

	script := []byte(`
       import "imported"

       fun main(): Int {
           let answer = answer()
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
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(42), value)
}

func TestRuntimeInvalidMainMissingAccount(t *testing.T) {

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(): Int {
           return 42
		}
	`)

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface)
	assert.Error(t, err)
}

func TestRuntimeMainWithAccount(t *testing.T) {

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(account: Account): Int {
           log(account.address)
           return 42
		}
	`)

	var loggedMessage string

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			return nil, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			return nil
		},
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
		log: func(message string) {
			loggedMessage = message
		},
	}

	value, err := runtime.ExecuteScript(script, runtimeInterface)

	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(42), value)
	assert.Equal(t, `"2a00000000000000000000000000000000000000"`, loggedMessage)
}

func TestRuntimeStorage(t *testing.T) {

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(account: Account) {
           log(account.storage[Int])

           account.storage[Int] = 42
           log(account.storage[Int])

           account.storage[[Int]] = [1, 2, 3]
           log(account.storage[[Int]])

           account.storage[String] = "xyz"
           log(account.storage[String])
       }
	`)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			return nil, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			return nil
		},
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface)
	require.Nil(t, err)
	assert.Equal(t, []string{"nil", "42", "[1, 2, 3]", `"xyz"`}, loggedMessages)
}

func TestRuntimeStorageMultipleTransactions(t *testing.T) {

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(account: Account) {
           log(account.storage[[String]])
           account.storage[[String]] = ["A", "B"]
       }
	`)

	var loggedMessages []string
	var storedValue []byte

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			return storedValue, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			storedValue = value
			return nil
		},
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface)
	assert.Nil(t, err)

	_, err = runtime.ExecuteScript(script, runtimeInterface)
	assert.Nil(t, err)

	assert.Equal(t, []string{"nil", `["A", "B"]`}, loggedMessages)
}

// test function call of stored structure declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsStructures(t *testing.T) {

	runtime := NewInterpreterRuntime()

	deepThought := []byte(`
       struct DeepThought {
           fun answer(): Int {
               return 42
           }
       }
	`)

	script1 := []byte(`
	   import "deep-thought"

       fun main(account: Account) {
           account.storage[DeepThought] = DeepThought()

           log(account.storage[DeepThought])
       }
	`)

	script2 := []byte(`
	   import "deep-thought"

       fun main(account: Account): Int {
           log(account.storage[DeepThought])

           let computer = account.storage[DeepThought]
               ?? panic("missing computer")

           return computer.answer()
       }
	`)

	var loggedMessages []string
	var storedValue []byte

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location ImportLocation) (bytes []byte, e error) {
			switch location {
			case StringImportLocation("deep-thought"):
				return deepThought, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			return storedValue, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			storedValue = value
			return nil
		},
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	_, err := runtime.ExecuteScript(script1, runtimeInterface)
	assert.Nil(t, err)

	answer, err := runtime.ExecuteScript(script2, runtimeInterface)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(42), answer)
}

func TestRuntimeStorageMultipleTransactionsInt(t *testing.T) {

	runtime := NewInterpreterRuntime()

	script1 := []byte(`
	  fun main(account: Account) {
	      account.storage[Int] = 42
	  }
	`)

	script2 := []byte(`
	  fun main(account: Account): Int {
	      return account.storage[Int] ?? panic("stored value is nil")
	  }
	`)

	var loggedMessages []string
	var storedValue []byte

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			return storedValue, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			storedValue = value
			return nil
		},
		getSigningAccounts: func() []types.Address {
			return []types.Address{[20]byte{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	_, err := runtime.ExecuteScript(script1, runtimeInterface)
	assert.Nil(t, err)

	result, err := runtime.ExecuteScript(script2, runtimeInterface)
	assert.Equal(t, big.NewInt(42), result)
	assert.Nil(t, err)
}

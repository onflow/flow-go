package runtime

import (
	"fmt"
	"math/big"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/types"
)

type testRuntimeInterface struct {
	resolveImport      func(ImportLocation) ([]byte, error)
	getValue           func(controller, owner, key []byte) (value []byte, err error)
	setValue           func(controller, owner, key, value []byte) (err error)
	createAccount      func(publicKeys [][]byte, keyWeights []int, code []byte) (accountID []byte, err error)
	updateAccountCode  func(accountID, code []byte) (err error)
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

func (i *testRuntimeInterface) UpdateAccountCode(accountID, code []byte) (err error) {
	return i.updateAccountCode(accountID, code)
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
		createAccount: func(publicKeys [][]byte, keyWeights []int, code []byte) (accountID []byte, err error) {
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

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(value).To(Equal(big.NewInt(42)))
}

func TestRuntimeInvalidMainMissingAccount(t *testing.T) {
	RegisterTestingT(t)

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

	Expect(err).
		To(HaveOccurred())
}

func TestRuntimeMainWithAccount(t *testing.T) {
	RegisterTestingT(t)

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

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(value).To(Equal(big.NewInt(42)))

	Expect(loggedMessage).
		To(Equal(`"2a00000000000000000000000000000000000000"`))
}

func TestRuntimeStorage(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(account: Account) {
           log(account.storage["nothing"])

           account.storage["answer"] = 42
           log(account.storage["answer"])

           account.storage["123"] = [1, 2, 3]
           log(account.storage["123"])

           account.storage["xyz"] = "xyz"
           log(account.storage["xyz"])
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

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(loggedMessages).
		To(Equal([]string{"nil", "42", "[1, 2, 3]", `"xyz"`}))
}

func TestRuntimeStorageMultipleTransactions(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()

	script := []byte(`
       fun main(account: Account) {
           log(account.storage["x"])
           account.storage["x"] = ["A", "B"]
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

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = runtime.ExecuteScript(script, runtimeInterface)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(loggedMessages).
		To(Equal([]string{"nil", `["A", "B"]`}))
}

// test function call of stored structure declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsStructures(t *testing.T) {
	RegisterTestingT(t)

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
           account.storage["x"] = DeepThought()

           log(account.storage["x"])
       }
	`)

	script2 := []byte(`
	   import "deep-thought"

       fun main(account: Account): Int {
           log(account.storage["x"])

           let stored = account.storage["x"]
               ?? panic("missing computer")

           let computer = (stored as? DeepThought)
               ?? panic("not a computer")

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

	Expect(err).
		To(Not(HaveOccurred()))

	answer, err := runtime.ExecuteScript(script2, runtimeInterface)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(answer).
		To(Equal(big.NewInt(42)))
}

func TestRuntimeStorageMultipleTransactionsInt(t *testing.T) {
	RegisterTestingT(t)

	runtime := NewInterpreterRuntime()

	script1 := []byte(`
	  fun main(account: Account) {
	      account.storage["count"] = 42
	  }
	`)

	script2 := []byte(`
	  fun main(account: Account): Int {
	      let count = account.storage["count"] ?? panic("stored value is nil")
	      return (count as? Int) ?? panic("not an Int")
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

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(runtime.ExecuteScript(script2, runtimeInterface)).
		To(Equal(big.NewInt(42)))
}

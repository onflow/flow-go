package runtime

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type testRuntimeInterface struct {
	resolveImport      func(Location) (values.Bytes, error)
	getValue           func(controller, owner, key values.Bytes) (value values.Bytes, err error)
	setValue           func(controller, owner, key, value values.Bytes) (err error)
	createAccount      func(publicKeys []values.Bytes, code values.Bytes) (address values.Address, err error)
	addAccountKey      func(address values.Address, publicKey values.Bytes) error
	removeAccountKey   func(address values.Address, index values.Int) (publicKey values.Bytes, err error)
	updateAccountCode  func(address values.Address, code values.Bytes) (err error)
	getSigningAccounts func() []values.Address
	log                func(string)
	emitEvent          func(values.Event)
}

func (i *testRuntimeInterface) ResolveImport(location Location) (values.Bytes, error) {
	return i.resolveImport(location)
}

func (i *testRuntimeInterface) GetValue(controller, owner, key values.Bytes) (value values.Bytes, err error) {
	return i.getValue(controller, owner, key)
}

func (i *testRuntimeInterface) SetValue(controller, owner, key, value values.Bytes) (err error) {
	return i.setValue(controller, owner, key, value)
}

func (i *testRuntimeInterface) CreateAccount(publicKeys []values.Bytes, code values.Bytes) (address values.Address, err error) {
	return i.createAccount(publicKeys, code)
}

func (i *testRuntimeInterface) AddAccountKey(address values.Address, publicKey values.Bytes) error {
	return i.addAccountKey(address, publicKey)
}

func (i *testRuntimeInterface) RemoveAccountKey(address values.Address, index values.Int) (publicKey values.Bytes, err error) {
	return i.removeAccountKey(address, index)
}

func (i *testRuntimeInterface) UpdateAccountCode(address values.Address, code values.Bytes) (err error) {
	return i.updateAccountCode(address, code)
}

func (i *testRuntimeInterface) GetSigningAccounts() []values.Address {
	if i.getSigningAccounts == nil {
		return nil
	}
	return i.getSigningAccounts()
}

func (i *testRuntimeInterface) Log(message string) {
	i.log(message)
}

func (i *testRuntimeInterface) EmitEvent(event values.Event) {
	i.emitEvent(event)
}

func TestRuntimeImport(t *testing.T) {

	runtime := NewInterpreterRuntime()

	importedScript := []byte(`
      pub fun answer(): Int {
        return 42
      }
    `)

	script := []byte(`
      import "imported"

      pub fun main(): Int {
          let answer = answer()
          if answer != 42 {
            panic("?!")
          }
          return answer
        }
    `)

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return importedScript, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
	}

	value, err := runtime.ExecuteScript(script, runtimeInterface, nil)
	assert.NoError(t, err)
	assert.Equal(t, values.NewInt(42), value)
}

func TestRuntimeInvalidTransactionArgumentAccount(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {}
        execute {}
      }
    `)

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
	}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)
	assert.Error(t, err)
}

func TestRuntimeTransactionWithAccount(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare(signer: Account) {
          log(signer.address)
        }
        execute {}
      }
    `)

	var loggedMessage string

	runtimeInterface := &testRuntimeInterface{
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return nil, nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessage = message
		},
	}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)

	assert.NoError(t, err)
	assert.Equal(t, "2a00000000000000000000000000000000000000", loggedMessage)
}

func TestRuntimeProgramWithNoTransaction(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      pub fun main() {}
    `)

	runtimeInterface := &testRuntimeInterface{}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)

	if assert.IsType(t, Error{}, err) {
		err := err.(Error)
		assert.IsType(t, InvalidTransactionCountError{}, err.Unwrap())
	}
}

func TestRuntimeProgramWithMultipleTransaction(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        execute {}
      }
      transaction {
        execute {}
      }
    `)

	runtimeInterface := &testRuntimeInterface{}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)

	if assert.IsType(t, Error{}, err) {
		err := err.(Error)
		assert.IsType(t, InvalidTransactionCountError{}, err.Unwrap())
	}
}

func TestRuntimeStorage(t *testing.T) {

	tests := map[string]string{
		"resource": `
          let r <- signer.storage[R] <- createR()
          log(r == nil)
          destroy r

          let r2 <- signer.storage[R] <- nil
          log(r2 != nil)
          destroy r2
        `,
		"reference": `
          log(signer.storage[&R] == nil)

          let oldR <- signer.storage[R] <- createR()
          destroy oldR

          signer.storage[&R] = &signer.storage[R] as R
          log(signer.storage[&R] != nil)
        `,
		"resource array": `
          let rs <- signer.storage[[R]] <- [<-createR()]
          log(rs == nil)
          destroy rs

          let rs2 <- signer.storage[[R]] <- nil
          log(rs2 != nil)
          destroy rs2
        `,
		"resource dictionary": `
          let rs <- signer.storage[{String: R}] <- {"r": <-createR()}
          log(rs == nil)
          destroy rs

          let rs2 <- signer.storage[{String: R}] <- nil
          log(rs2 != nil)
          destroy rs2
        `,
	}

	for name, code := range tests {
		t.Run(name, func(t *testing.T) {
			runtime := NewInterpreterRuntime()

			imported := []byte(`
              pub resource R {}

              pub fun createR(): <-R {
                return <-create R()
              }
            `)

			script := []byte(fmt.Sprintf(`
                  import "imported"

                  transaction {
                    prepare(signer: Account) {
                      %s
                    }
                    execute {}
                  }
                `,
				code,
			))

			storedValues := map[string][]byte{}

			var loggedMessages []string

			runtimeInterface := &testRuntimeInterface{
				resolveImport: func(location Location) (bytes values.Bytes, err error) {
					switch location {
					case StringLocation("imported"):
						return imported, nil
					default:
						return nil, fmt.Errorf("unknown import location: %s", location)
					}
				},
				getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
					return storedValues[string(key)], nil
				},
				setValue: func(controller, owner, key, value values.Bytes) (err error) {
					storedValues[string(key)] = value
					return nil
				},
				getSigningAccounts: func() []values.Address {
					return []values.Address{{42}}
				},
				log: func(message string) {
					loggedMessages = append(loggedMessages, message)
				},
			}

			err := runtime.ExecuteTransaction(script, runtimeInterface, nil)
			require.NoError(t, err)

			assert.Equal(t, []string{"true", "true"}, loggedMessages)
		})
	}
}

func TestRuntimeStorageMultipleTransactionsResourceWithArray(t *testing.T) {
	runtime := NewInterpreterRuntime()

	container := []byte(`
      pub resource Container {
        pub let values: [Int]

        init() {
          self.values = []
        }
      }

      pub fun createContainer(): <-Container {
        return <-create Container()
      }
    `)

	script1 := []byte(`
      import "container"

      transaction {

        prepare(signer: Account) {
          var container: <-Container? <- createContainer()
          signer.storage[Container] <-> container
          destroy container
          let ref = &signer.storage[Container] as Container
          signer.storage[&Container] = ref
        }

        execute {}
      }
    `)

	script2 := []byte(`
      import "container"

      transaction {
        prepare(signer: Account) {
          let ref = signer.storage[&Container] ?? panic("no container")
          let length = ref.values.length
          ref.values.append(1)
          let length2 = ref.values.length
        }
        execute {}
      }
    `)

	script3 := []byte(`
      import "container"

      transaction {
        prepare(signer: Account) {
          let ref = signer.storage[&Container] ?? panic("no container")
          let length = ref.values.length
          ref.values.append(2)
          let length2 = ref.values.length
        }
        execute {}
      }
    `)

	var loggedMessages []string
	storedValues := map[string]values.Bytes{}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("container"):
				return container, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script3, runtimeInterface, nil)
	require.NoError(t, err)
}

// TestRuntimeStorageMultipleTransactionsResourceFunction tests a function call
// of a stored resource declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsResourceFunction(t *testing.T) {
	runtime := NewInterpreterRuntime()

	deepThought := []byte(`
      pub resource DeepThought {

        pub fun answer(): Int {
          return 42
        }
      }

      pub fun createDeepThought(): <-DeepThought {
        return <-create DeepThought()
      }
    `)

	script1 := []byte(`
      import "deep-thought"

      transaction {

        prepare(signer: Account) {
          let existing <- signer.storage[DeepThought] <- createDeepThought()
          if existing != nil {
             panic("already initialized")
          }
          destroy existing
        }

        execute {}
      }
    `)

	script2 := []byte(`
      import "deep-thought"

      transaction {
        prepare(signer: Account) {
          let answer = signer.storage[DeepThought]?.answer()
          log(answer ?? 0)
        }
        execute {}
      }
    `)

	var loggedMessages []string
	storedValues := map[string]values.Bytes{}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("deep-thought"):
				return deepThought, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)

	assert.Contains(t, loggedMessages, "42")
}

// TestRuntimeStorageMultipleTransactionsResourceField tests reading a field
// of a stored resource declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsResourceField(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource Number {
        pub(set) var n: Int
        init(_ n: Int) {
          self.n = n
        }
      }

      pub fun createNumber(_ n: Int): <-Number {
        return <-create Number(n)
      }
    `)

	script1 := []byte(`
      import "imported"

      transaction {
        prepare(signer: Account) {
          let oldNumber <- signer.storage[Number] <- createNumber(42)
          if oldNumber != nil {
             panic("already initialized")
          }
          destroy oldNumber

        }
        execute {}
      }
    `)

	script2 := []byte(`
      import "imported"

      transaction {
        prepare(signer: Account) {
          if let number <- signer.storage[Number] <- nil {
            log(number.n)
            destroy number
          }
        }
        execute {}
      }
    `)

	var loggedMessages []string
	storedValues := map[string]values.Bytes{}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	require.NoError(t, err)

	assert.Contains(t, loggedMessages, "42")
}

// TestRuntimeCompositeFunctionInvocationFromImportingProgram checks
// that member functions of imported composites can be invoked from an importing program.
// See https://github.com/dapperlabs/flow-go/issues/838
//
func TestRuntimeCompositeFunctionInvocationFromImportingProgram(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      // function must have arguments
      pub fun x(x: Int) {}

      // invocation must be in composite
      pub resource Y {
        pub fun x() {
          x(x: 1)
        }
      }

      pub fun createY(): <-Y {
        return <-create Y()
      }
    `)

	script1 := []byte(`
      import Y, createY from "imported"

      transaction {
        prepare(signer: Account) {
          let oldY <- signer.storage[Y] <- createY()
          destroy oldY
        }
        execute {}
      }
    `)

	script2 := []byte(`
      import Y from "imported"

      transaction {
        prepare(signer: Account) {
          let y <- signer.storage[Y] <- nil
          y?.x()
          destroy y
        }
        execute {}
      }
    `)

	storedValues := map[string]values.Bytes{}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)
}

func TestRuntimeResourceContractUseThroughReference(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun x() {
          log("x!")
        }
      }

      pub fun createR(): <-R {
        return <- create R()
      }
    `)

	script1 := []byte(`
      import R, createR from "imported"

      transaction {

        prepare(signer: Account) {
          let r <- signer.storage[R] <- createR()
          if r != nil {
             panic("already initialized")
          }
          destroy r
        }

        execute {}
      }
    `)

	script2 := []byte(`
      import R from "imported"

      transaction {

        prepare(signer: Account) {
          let ref = &signer.storage[R] as R
          ref.x()
        }

        execute {}
      }
    `)

	storedValues := map[string][]byte{}

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestRuntimeResourceContractUseThroughStoredReference(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun x() {
          log("x!")
        }
      }

      pub fun createR(): <-R {
          return <- create R()
      }
    `)

	script1 := []byte(`
      import R, createR from "imported"

      transaction {

        prepare(signer: Account) {
          let r <- signer.storage[R] <- createR()
          if r != nil {
             panic("already initialized")
          }
          destroy r

          signer.storage[&R] = &signer.storage[R] as R
        }

        execute {}
      }
    `)

	script2 := []byte(`
      import R from "imported"

      transaction {
        prepare(signer: Account) {
          let ref = signer.storage[&R] ?? panic("no R ref")
          ref.x()
        }
        execute {}
      }
    `)

	storedValues := map[string][]byte{}

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestRuntimeResourceContractWithInterface(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported1 := []byte(`
      pub resource interface RI {
        pub fun x()
      }
    `)

	imported2 := []byte(`
      import RI from "imported1"

      pub resource R: RI {
        pub fun x() {
          log("x!")
        }
      }

      pub fun createR(): <-R {
        return <- create R()
      }
    `)

	script1 := []byte(`
      import RI from "imported1"
      import R, createR from "imported2"

      transaction {
        prepare(signer: Account) {
          var r: <-R? <- createR()
          signer.storage[R] <-> r
          if r != nil {
            panic("already initialized")
          }
          destroy r

          signer.storage[&RI] = &signer.storage[R] as RI
        }
        execute {}
      }
    `)

	// TODO: Get rid of the requirement that the underlying type must be imported.
	//   This requires properly initializing Interpreter.CompositeFunctions.
	//   Also initialize Interpreter.DestructorFunctions

	script2 := []byte(`
      import RI from "imported1"
      import R from "imported2"

      transaction {
        prepare(signer: Account) {
          let ref = signer.storage[&RI] ?? panic("no RI ref")
          ref.x()
        }
        execute {}
      }
    `)

	storedValues := map[string][]byte{}

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported1"):
				return imported1, nil
			case StringLocation("imported2"):
				return imported2, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestParseAndCheckProgram(t *testing.T) {
	t.Run("ValidProgram", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte("pub fun test(): Int { return 42 }")
		runtimeInterface := &testRuntimeInterface{}

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nil)
		assert.NoError(t, err)
	})

	t.Run("InvalidSyntax", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte("invalid syntax")
		runtimeInterface := &testRuntimeInterface{}

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nil)
		assert.NotNil(t, err)
	})

	t.Run("InvalidSemantics", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte(`pub let a: Int = "b"`)
		runtimeInterface := &testRuntimeInterface{}

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nil)
		assert.NotNil(t, err)
	})
}

func TestRuntimeSyntaxError(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      pub fun main(): String {
          return "Hello World!
      }
    `)

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
	}

	_, err := runtime.ExecuteScript(script, runtimeInterface, nil)
	assert.Error(t, err)
}

func TestRuntimeStorageChanges(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource X {
        pub(set) var x: Int
    
        init() {
          self.x = 0
        }
      }

      pub fun createX(): <-X {
          return <-create X()
      }
    `)

	script1 := []byte(`
      import X, createX from "imported"

      transaction {
        prepare(signer: Account) {
          var x: <-X? <- createX()
          signer.storage[X] <-> x
          destroy x

          let ref = &signer.storage[X] as X
          ref.x = 1
        }
        execute {}
      }
    `)

	script2 := []byte(`
      import X from "imported"

      transaction {
        prepare(signer: Account) {
          let ref = &signer.storage[X] as X
          log(ref.x)
        }
        execute {}
      }
    `)

	storedValues := map[string][]byte{}

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	assert.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	assert.NoError(t, err)

	assert.Equal(t, []string{"1"}, loggedMessages)
}

func TestRuntimeAccountAddress(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare(signer: Account) {
          log(signer.address)
        }
        execute {}
      }
    `)

	var loggedMessages []string

	address := interpreter.AddressValue{42}

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []values.Address {
			return []values.Address{address.Export().(values.Address)}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{fmt.Sprint(address)}, loggedMessages)
}

func TestRuntimePublicAccountAddress(t *testing.T) {
	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {

        prepare() {
          log(getAccount(0x42).address)
        }

        execute {}
      }
    `)

	var loggedMessages []string

	address := interpreter.NewAddressValueFromBytes([]byte{0x42})

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []values.Address {
			return nil
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{fmt.Sprint(address)}, loggedMessages)
}

func TestRuntimeAccountPublishAndAccess(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun test(): Int {
          return 42
        }
      }

      pub fun createR(): <-R {
        return <-create R()
      }
    `)

	script1 := []byte(`
      import "imported"

      transaction {
        prepare(signer: Account) {
          let existing <- signer.storage[R] <- createR()
          destroy existing
          signer.published[&R] = &signer.storage[R] as R 
        }
        execute {}
      }
    `)

	address := interpreter.AddressValue{42}

	script2 := []byte(
		fmt.Sprintf(
			`
              import "imported"
    
              transaction {
    
                prepare(signer: Account) {
                  log(getAccount(0x%s).published[&R]?.test() ?? 0)
                }
    
                execute {}
              }
            `,
			address,
		),
	)

	var loggedMessages []string

	storedValues := map[string]values.Bytes{}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getValue: func(controller, owner, key values.Bytes) (value values.Bytes, err error) {
			return storedValues[string(key)], nil
		},
		setValue: func(controller, owner, key, value values.Bytes) (err error) {
			storedValues[string(key)] = value
			return nil
		},
		getSigningAccounts: func() []values.Address {
			return []values.Address{address.Export().(values.Address)}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	err := runtime.ExecuteTransaction(script1, runtimeInterface, nil)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, runtimeInterface, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"42"}, loggedMessages)
}

func TestRuntimeCyclicImport(t *testing.T) {
	runtime := NewInterpreterRuntime()

	imported := []byte(`
      import "imported"
    `)

	script := []byte(
		`
		  import "imported"

		  transaction {
			execute {}
		  }
		`,
	)

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes values.Bytes, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getSigningAccounts: func() []values.Address {
			return nil
		},
	}

	err := runtime.ExecuteTransaction(script, runtimeInterface, nil)

	require.Error(t, err)
	require.IsType(t, Error{}, err)
	assert.IsType(t, ast.CyclicImportsError{}, err.(Error).Unwrap())
}

/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runtime

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/cadence/runtime/tests/checker"
	"github.com/onflow/cadence/runtime/tests/utils"
)

type testRuntimeInterfaceStorage struct {
	storedValues map[string][]byte
	valueExists  func(controller, owner, key []byte) (exists bool, err error)
	getValue     func(controller, owner, key []byte) (value []byte, err error)
	setValue     func(controller, owner, key, value []byte) (err error)
}

func newTestStorage(
	onRead func(controller, owner, key, value []byte),
	onWrite func(controller, owner, key, value []byte),
) testRuntimeInterfaceStorage {

	storageKey := func(owner, controller, key string) string {
		return strings.Join([]string{owner, controller, key}, "|")
	}

	storedValues := map[string][]byte{}

	storage := testRuntimeInterfaceStorage{
		storedValues: storedValues,
		valueExists: func(controller, owner, key []byte) (bool, error) {
			_, ok := storedValues[storageKey(string(controller), string(owner), string(key))]
			return ok, nil
		},
		getValue: func(controller, owner, key []byte) (value []byte, err error) {
			value = storedValues[storageKey(string(controller), string(owner), string(key))]
			if onRead != nil {
				onRead(controller, owner, key, value)
			}
			return value, nil
		},
		setValue: func(controller, owner, key, value []byte) (err error) {
			storedValues[storageKey(string(controller), string(owner), string(key))] = value
			if onWrite != nil {
				onWrite(controller, owner, key, value)
			}
			return nil
		},
	}

	return storage
}

type testRuntimeInterface struct {
	resolveImport      func(Location) ([]byte, error)
	getCachedProgram   func(Location) (*ast.Program, error)
	cacheProgram       func(Location, *ast.Program) error
	storage            testRuntimeInterfaceStorage
	createAccount      func(payer Address) (address Address, err error)
	addAccountKey      func(address Address, publicKey []byte) error
	removeAccountKey   func(address Address, index int) (publicKey []byte, err error)
	updateAccountCode  func(address Address, code []byte) (err error)
	getSigningAccounts func() []Address
	log                func(string)
	emitEvent          func(cadence.Event)
	generateUUID       func() uint64
	computationLimit   uint64
	decodeArgument     func(b []byte, t cadence.Type) (cadence.Value, error)
	programParsed      func(location ast.Location, duration time.Duration)
	programChecked     func(location ast.Location, duration time.Duration)
	programInterpreted func(location ast.Location, duration time.Duration)
	valueEncoded       func(duration time.Duration)
	valueDecoded       func(duration time.Duration)
	unsafeRandom       func() uint64
}

var _ Interface = &testRuntimeInterface{}

func (i *testRuntimeInterface) ResolveImport(location Location) ([]byte, error) {
	return i.resolveImport(location)
}

func (i *testRuntimeInterface) GetCachedProgram(location Location) (*ast.Program, error) {
	if i.getCachedProgram == nil {
		return nil, nil
	}
	return i.getCachedProgram(location)
}

func (i *testRuntimeInterface) CacheProgram(location Location, program *ast.Program) error {
	if i.cacheProgram == nil {
		return nil
	}
	return i.cacheProgram(location, program)
}

func (i *testRuntimeInterface) ValueExists(controller, owner, key []byte) (exists bool, err error) {
	return i.storage.valueExists(controller, owner, key)
}

func (i *testRuntimeInterface) GetValue(controller, owner, key []byte) (value []byte, err error) {
	return i.storage.getValue(controller, owner, key)
}

func (i *testRuntimeInterface) SetValue(controller, owner, key, value []byte) (err error) {
	return i.storage.setValue(controller, owner, key, value)
}

func (i *testRuntimeInterface) CreateAccount(payer Address) (address Address, err error) {
	return i.createAccount(payer)
}

func (i *testRuntimeInterface) AddAccountKey(address Address, publicKey []byte) error {
	return i.addAccountKey(address, publicKey)
}

func (i *testRuntimeInterface) RemoveAccountKey(address Address, index int) (publicKey []byte, err error) {
	return i.removeAccountKey(address, index)
}

func (i *testRuntimeInterface) UpdateAccountCode(address Address, code []byte) (err error) {
	return i.updateAccountCode(address, code)
}

func (i *testRuntimeInterface) GetSigningAccounts() []Address {
	if i.getSigningAccounts == nil {
		return nil
	}
	return i.getSigningAccounts()
}

func (i *testRuntimeInterface) Log(message string) {
	i.log(message)
}

func (i *testRuntimeInterface) EmitEvent(event cadence.Event) {
	i.emitEvent(event)
}

func (i *testRuntimeInterface) GenerateUUID() uint64 {
	if i.generateUUID == nil {
		return 0
	}
	return i.generateUUID()
}

func (i *testRuntimeInterface) GetComputationLimit() uint64 {
	return i.computationLimit
}

func (i *testRuntimeInterface) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	return i.decodeArgument(b, t)
}

func (i *testRuntimeInterface) ProgramParsed(location ast.Location, duration time.Duration) {
	if i.programParsed == nil {
		return
	}
	i.programParsed(location, duration)
}

func (i *testRuntimeInterface) ProgramChecked(location ast.Location, duration time.Duration) {
	if i.programChecked == nil {
		return
	}
	i.programChecked(location, duration)
}

func (i *testRuntimeInterface) ProgramInterpreted(location ast.Location, duration time.Duration) {
	if i.programInterpreted == nil {
		return
	}
	i.programInterpreted(location, duration)
}

func (i *testRuntimeInterface) ValueEncoded(duration time.Duration) {
	if i.valueEncoded == nil {
		return
	}
	i.valueEncoded(duration)
}

func (i *testRuntimeInterface) ValueDecoded(duration time.Duration) {
	if i.valueDecoded == nil {
		return
	}
	i.valueDecoded(duration)
}

func (i *testRuntimeInterface) GetCurrentBlockHeight() uint64 {
	return 1
}

func (i *testRuntimeInterface) GetBlockAtHeight(height uint64) (hash BlockHash, timestamp int64, exists bool,
	err error) {
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, height)
	if err != nil {
		panic(err)
	}

	encoded := buf.Bytes()
	copy(hash[stdlib.BlockIDSize-len(encoded):], encoded)

	return hash, time.Unix(int64(height), 0).UnixNano(), true, nil
}

func (i *testRuntimeInterface) UnsafeRandom() uint64 {
	if i.unsafeRandom == nil {
		return 0
	}
	return i.unsafeRandom()
}

func TestRuntimeImport(t *testing.T) {

	t.Parallel()

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
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return importedScript, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	value, err := runtime.ExecuteScript(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, cadence.NewInt(42), value)
}

func TestRuntimeProgramCache(t *testing.T) {

	t.Parallel()

	programCache := map[ast.LocationID]*ast.Program{}
	cacheHits := make(map[ast.LocationID]bool)

	importedScript := []byte(`
	transaction {
		prepare() {}
		execute {}
	}
	`)
	importedScriptLocation := ast.StringLocation("imported")

	runtime := NewInterpreterRuntime()
	runtimeInterface := &testRuntimeInterface{
		getCachedProgram: func(location ast.Location) (*ast.Program, error) {
			program, found := programCache[location.ID()]
			cacheHits[location.ID()] = found
			if !found {
				return nil, nil
			}
			return program, nil
		},
		cacheProgram: func(location ast.Location, program *ast.Program) error {
			programCache[location.ID()] = program
			return nil
		},
		resolveImport: func(location Location) ([]byte, error) {
			switch location {
			case importedScriptLocation:
				return importedScript, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
	}

	t.Run("empty cache, cache miss", func(t *testing.T) {

		script := []byte(`
		import "imported"

		transaction {
			prepare() {}
			execute {}
		}
		`)
		scriptLocation := ast.StringLocation("placeholder")

		// Initial call, should parse script, store result in cache.
		err := runtime.ParseAndCheckProgram(script, runtimeInterface, scriptLocation)
		assert.NoError(t, err)

		// Program was added to cache.
		cachedProgram, exists := programCache[scriptLocation.ID()]
		assert.True(t, exists)
		assert.NotNil(t, cachedProgram)

		// Script was not in cache.
		assert.False(t, cacheHits[scriptLocation.ID()])
	})

	t.Run("program previously parsed, cache hit", func(t *testing.T) {

		script := []byte(`
		import "imported"

		transaction {
			prepare() {}
			execute {}
		}
		`)
		scriptLocation := ast.StringLocation("placeholder")

		// Call a second time to hit the cache
		err := runtime.ParseAndCheckProgram(script, runtimeInterface, scriptLocation)
		assert.NoError(t, err)

		// Script was in cache.
		assert.True(t, cacheHits[scriptLocation.ID()])
	})

	t.Run("imported program previously parsed, cache hit", func(t *testing.T) {

		script := []byte(`
		import "imported"

		transaction {
			prepare() {}
			execute {}
		}
		`)
		scriptLocation := ast.StringLocation("placeholder")

		// Call a second time to hit the cache
		err := runtime.ParseAndCheckProgram(script, runtimeInterface, scriptLocation)
		assert.NoError(t, err)

		// Script was in cache.
		assert.True(t, cacheHits[scriptLocation.ID()])
		// Import was in cache.
		assert.True(t, cacheHits[importedScriptLocation.ID()])
	})
}

func newTransactionLocationGenerator() func() TransactionLocation {
	var transactionCount uint8
	return func() TransactionLocation {
		defer func() { transactionCount++ }()
		return TransactionLocation{transactionCount}
	}
}

func TestRuntimeInvalidTransactionArgumentAccount(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {}
        execute {}
      }
    `)

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	assert.Error(t, err)
}

func TestRuntimeTransactionWithAccount(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare(signer: AuthAccount) {
          log(signer.address)
        }
      }
    `)

	var loggedMessage string

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return []Address{
				common.BytesToAddress([]byte{42}),
			}
		},
		log: func(message string) {
			loggedMessage = message
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, "0x2a", loggedMessage)
}

func TestRuntimeTransactionWithArguments(t *testing.T) {

	t.Parallel()

	var tests = []struct {
		label        string
		script       string
		args         [][]byte
		authorizers  []Address
		expectedLogs []string
		check        func(t *testing.T, err error)
	}{
		{
			label: "Single argument",
			script: `
			  transaction(x: Int) {
				execute {
				  log(x)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewInt(42)),
			},
			expectedLogs: []string{"42"},
		},
		{
			label: "Single argument with authorizer",
			script: `
			  transaction(x: Int) {
				prepare(signer: AuthAccount) {
				  log(signer.address)
				}

				execute {
				  log(x)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewInt(42)),
			},
			authorizers:  []Address{common.BytesToAddress([]byte{42})},
			expectedLogs: []string{"0x2a", "42"},
		},
		{
			label: "Multiple arguments",
			script: `
			  transaction(x: Int, y: String) {
				execute {
				  log(x)
				  log(y)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewInt(42)),
				jsoncdc.MustEncode(cadence.NewString("foo")),
			},
			expectedLogs: []string{"42", `"foo"`},
		},
		{
			label: "Invalid bytes",
			script: `
			  transaction(x: Int) { execute {} }
			`,
			args: [][]byte{
				{1, 2, 3, 4}, // not valid JSON-CDC
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
			},
		},
		{
			label: "Type mismatch",
			script: `
			  transaction(x: Int) {
				execute {
				  log(x)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewString("foo")),
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
				assert.IsType(t, &InvalidTypeAssignmentError{}, errors.Unwrap(errors.Unwrap(err)))
			},
		},
		{
			label: "Address",
			script: `
			  transaction(x: Address) {
				execute {
				  let acct = getAccount(x)
				  log(acct.address)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.BytesToAddress(
						[]byte{
							0x0, 0x0, 0x0, 0x0,
							0x0, 0x0, 0x0, 0x1,
						},
					),
				),
			},
			expectedLogs: []string{"0x1"},
		},
		{
			label: "Array",
			script: `
			  transaction(x: [Int]) {
				execute {
				  log(x)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewArray(
						[]cadence.Value{
							cadence.NewInt(1),
							cadence.NewInt(2),
							cadence.NewInt(3),
						},
					),
				),
			},
			expectedLogs: []string{"[1, 2, 3]"},
		},
		{
			label: "Dictionary",
			script: `
			  transaction(x: {String:Int}) {
				execute {
				  log(x["y"])
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewDictionary(
						[]cadence.KeyValuePair{
							{
								Key:   cadence.NewString("y"),
								Value: cadence.NewInt(42),
							},
						},
					),
				),
			},
			expectedLogs: []string{"42"},
		},
		{
			label: "Invalid dictionary",
			script: `
			  transaction(x: {String:String}) {
				execute {
				  log(x["y"])
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewDictionary(
						[]cadence.KeyValuePair{
							{
								Key:   cadence.NewString("y"),
								Value: cadence.NewInt(42),
							},
						},
					),
				),
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
				assert.IsType(t, &InvalidTypeAssignmentError{}, errors.Unwrap(errors.Unwrap(err)))
			},
		},
		{
			label: "Struct",
			script: `
			  pub struct Foo {
				pub var y: String

				init() {
				  self.y = "initial string"
				}
 			  }

			  transaction(x: Foo) {
				execute {
				  log(x.y)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.
						NewStruct([]cadence.Value{cadence.NewString("bar")}).
						WithType(cadence.StructType{
							TypeID:     "test.Foo",
							Identifier: "Foo",
							Fields: []cadence.Field{
								{
									Identifier: "y",
									Type:       cadence.StringType{},
								},
							},
						}),
				),
			},
			expectedLogs: []string{`"bar"`},
		},
		{
			label: "Struct in array",
			script: `
			  pub struct Foo {
				pub var y: String

				init() {
				  self.y = "initial string"
				}
 			  }

			  transaction(f: [Foo]) {
				execute {
				  let x = f[0]
				  log(x.y)
				}
			  }
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewArray([]cadence.Value{
						cadence.
							NewStruct([]cadence.Value{cadence.NewString("bar")}).
							WithType(cadence.StructType{
								TypeID:     "test.Foo",
								Identifier: "Foo",
								Fields: []cadence.Field{
									{
										Identifier: "y",
										Type:       cadence.StringType{},
									},
								},
							}),
					}),
				),
			},
			expectedLogs: []string{`"bar"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			rt := NewInterpreterRuntime()

			var loggedMessages []string

			runtimeInterface := &testRuntimeInterface{
				getSigningAccounts: func() []Address { return tt.authorizers },
				decodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
					return jsoncdc.Decode(b)
				},
				log: func(message string) {
					loggedMessages = append(loggedMessages, message)
				},
			}

			err := rt.ExecuteTransaction(
				[]byte(tt.script),
				tt.args,
				runtimeInterface,
				utils.TestLocation,
			)

			if tt.check != nil {
				tt.check(t, err)
			} else {
				if !assert.NoError(t, err) {
					for err := err; err != nil; err = errors.Unwrap(err) {
						t.Log(err)
					}
				}
				assert.ElementsMatch(t, tt.expectedLogs, loggedMessages)
			}
		})
	}
}

func TestRuntimeScriptArguments(t *testing.T) {

	t.Parallel()

	var tests = []struct {
		label        string
		script       string
		args         [][]byte
		expectedLogs []string
		check        func(t *testing.T, err error)
	}{
		{
			label: "No arguments",
			script: `
				pub fun main() {
					log("t")
				}
			`,
			args:         nil,
			expectedLogs: []string{`"t"`},
		},
		{
			label: "Single argument",
			script: `
				pub fun main(x: Int) {
					log(x)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewInt(42)),
			},
			expectedLogs: []string{"42"},
		},
		{
			label: "Multiple arguments",
			script: `
				pub fun main(x: Int, y: String) {
					log(x)
					log(y)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewInt(42)),
				jsoncdc.MustEncode(cadence.NewString("foo")),
			},
			expectedLogs: []string{"42", `"foo"`},
		},
		{
			label: "Invalid bytes",
			script: `
				pub fun main(x: Int) { }
			`,
			args: [][]byte{
				{1, 2, 3, 4}, // not valid JSON-CDC
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
			},
		},
		{
			label: "Type mismatch",
			script: `
				pub fun main(x: Int) {
					log(x)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(cadence.NewString("foo")),
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
				assert.IsType(t, &InvalidTypeAssignmentError{}, errors.Unwrap(errors.Unwrap(err)))
			},
		},
		{
			label: "Address",
			script: `
				pub fun main(x: Address) {
					log(x)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.BytesToAddress(
						[]byte{
							0x0, 0x0, 0x0, 0x0,
							0x0, 0x0, 0x0, 0x1,
						},
					),
				),
			},
			expectedLogs: []string{"0x1"},
		},
		{
			label: "Array",
			script: `
				pub fun main(x: [Int]) {
					log(x)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewArray(
						[]cadence.Value{
							cadence.NewInt(1),
							cadence.NewInt(2),
							cadence.NewInt(3),
						},
					),
				),
			},
			expectedLogs: []string{"[1, 2, 3]"},
		},
		{
			label: "Dictionary",
			script: `
				pub fun main(x: {String:Int}) {
					log(x["y"])
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewDictionary(
						[]cadence.KeyValuePair{
							{
								Key:   cadence.NewString("y"),
								Value: cadence.NewInt(42),
							},
						},
					),
				),
			},
			expectedLogs: []string{"42"},
		},
		{
			label: "Invalid dictionary",
			script: `
				pub fun main(x: {String:String}) {
					log(x["y"])
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewDictionary(
						[]cadence.KeyValuePair{
							{
								Key:   cadence.NewString("y"),
								Value: cadence.NewInt(42),
							},
						},
					),
				),
			},
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.IsType(t, &InvalidEntryPointArgumentError{}, errors.Unwrap(err))
				assert.IsType(t, &InvalidTypeAssignmentError{}, errors.Unwrap(errors.Unwrap(err)))
			},
		},
		{
			label: "Struct",
			script: `
				pub struct Foo {
					pub var y: String

					init() {
						self.y = "initial string"
					}
				}

				pub fun main(x: Foo) {
					log(x.y)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.
						NewStruct([]cadence.Value{cadence.NewString("bar")}).
						WithType(cadence.StructType{
							TypeID:     "test.Foo",
							Identifier: "Foo",
							Fields: []cadence.Field{
								{
									Identifier: "y",
									Type:       cadence.StringType{},
								},
							},
						}),
				),
			},
			expectedLogs: []string{`"bar"`},
		},
		{
			label: "Struct in array",
			script: `
				pub struct Foo {
					pub var y: String

					init() {
						self.y = "initial string"
					}
				}

				pub fun main(f: [Foo]) {
					let x = f[0]
					log(x.y)
				}
			`,
			args: [][]byte{
				jsoncdc.MustEncode(
					cadence.NewArray([]cadence.Value{
						cadence.
							NewStruct([]cadence.Value{cadence.NewString("bar")}).
							WithType(cadence.StructType{
								TypeID:     "test.Foo",
								Identifier: "Foo",
								Fields: []cadence.Field{
									{
										Identifier: "y",
										Type:       cadence.StringType{},
									},
								},
							}),
					}),
				),
			},
			expectedLogs: []string{`"bar"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			rt := NewInterpreterRuntime()

			var loggedMessages []string

			runtimeInterface := &testRuntimeInterface{
				decodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
					return jsoncdc.Decode(b)
				},
				log: func(message string) {
					loggedMessages = append(loggedMessages, message)
				},
			}

			_, err := rt.ExecuteScript(
				[]byte(tt.script),
				tt.args,
				runtimeInterface,
				utils.TestLocation,
			)

			if tt.check != nil {
				tt.check(t, err)
			} else {
				if !assert.NoError(t, err) {
					for err := err; err != nil; err = errors.Unwrap(err) {
						t.Log(err)
					}
				}
				assert.ElementsMatch(t, tt.expectedLogs, loggedMessages)
			}
		})
	}
}

func TestRuntimeProgramWithNoTransaction(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      pub fun main() {}
    `)

	runtimeInterface := &testRuntimeInterface{}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())

	require.IsType(t, Error{}, err)
	err = err.(Error).Unwrap()
	assert.IsType(t, InvalidTransactionCountError{}, err)
}

func TestRuntimeProgramWithMultipleTransaction(t *testing.T) {

	t.Parallel()

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

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())

	require.IsType(t, Error{}, err)
	err = err.(Error).Unwrap()
	assert.IsType(t, InvalidTransactionCountError{}, err)
}

func TestRuntimeStorage(t *testing.T) {

	t.Parallel()

	tests := map[string]string{
		"resource": `
          let r <- signer.load<@R>(from: /storage/r)
          log(r == nil)
          destroy r

          signer.save(<-createR(), to: /storage/r)
          let r2 <- signer.load<@R>(from: /storage/r)
          log(r2 != nil)
          destroy r2
        `,
		"struct": `
          let s = signer.load<S>(from: /storage/s)
          log(s == nil)

          signer.save(S(), to: /storage/s)
          let s2 = signer.load<S>(from: /storage/s)
          log(s2 != nil)
        `,
		"resource array": `
		  let rs <- signer.load<@[R]>(from: /storage/rs)
		  log(rs == nil)
		  destroy rs

		  signer.save(<-[<-createR()], to: /storage/rs)
		  let rs2 <- signer.load<@[R]>(from: /storage/rs)
		  log(rs2 != nil)
		  destroy rs2
		`,
		"struct array": `
		  let s = signer.load<[S]>(from: /storage/s)
		  log(s == nil)

		  signer.save([S()], to: /storage/s)
		  let s2 = signer.load<[S]>(from: /storage/s)
		  log(s2 != nil)
		`,
		"resource dictionary": `
		  let rs <- signer.load<@{String: R}>(from: /storage/rs)
		  log(rs == nil)
		  destroy rs

          signer.save(<-{"r": <-createR()}, to: /storage/rs)
		  let rs2 <- signer.load<@{String: R}>(from: /storage/rs)
		  log(rs2 != nil)
		  destroy rs2
		`,
		"struct dictionary": `
		  let s = signer.load<{String: S}>(from: /storage/s)
		  log(s == nil)

          signer.save({"s": S()}, to: /storage/s)
		  let rs2 = signer.load<{String: S}>(from: /storage/s)
		  log(rs2 != nil)
		`,
	}

	for name, code := range tests {
		t.Run(name, func(t *testing.T) {
			runtime := NewInterpreterRuntime()

			imported := []byte(`
              pub resource R {}

              pub fun createR(): @R {
                return <-create R()
              }

              pub struct S {}
            `)

			script := []byte(fmt.Sprintf(`
                  import "imported"

                  transaction {
                    prepare(signer: AuthAccount) {
                      %s
                    }
                  }
                `,
				code,
			))

			var loggedMessages []string

			runtimeInterface := &testRuntimeInterface{
				resolveImport: func(location Location) ([]byte, error) {
					switch location {
					case StringLocation("imported"):
						return imported, nil
					default:
						return nil, fmt.Errorf("unknown import location: %s", location)
					}
				},
				storage: newTestStorage(nil, nil),
				getSigningAccounts: func() []Address {
					return []Address{{42}}
				},
				log: func(message string) {
					loggedMessages = append(loggedMessages, message)
				},
			}

			nextTransactionLocation := newTransactionLocationGenerator()

			err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
			require.NoError(t, err)

			assert.Equal(t, []string{"true", "true"}, loggedMessages)
		})
	}
}

func TestRuntimeStorageMultipleTransactionsResourceWithArray(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	container := []byte(`
      pub resource Container {
        pub let values: [Int]

        init() {
          self.values = []
        }
      }

      pub fun createContainer(): @Container {
        return <-create Container()
      }
    `)

	script1 := []byte(`
      import "container"

      transaction {

        prepare(signer: AuthAccount) {
          signer.save(<-createContainer(), to: /storage/container)
          signer.link<&Container>(/public/container, target: /storage/container)
        }
      }
    `)

	script2 := []byte(`
      import "container"

      transaction {
        prepare(signer: AuthAccount) {
          let publicAccount = getAccount(signer.address)
          let ref = publicAccount.getCapability(/public/container)!.borrow<&Container>()!

          let length = ref.values.length
          ref.values.append(1)
          let length2 = ref.values.length
        }
      }
    `)

	script3 := []byte(`
      import "container"

      transaction {
        prepare(signer: AuthAccount) {
          let publicAccount = getAccount(signer.address)
          let ref = publicAccount.getCapability(/public/container)!.borrow<&Container>()!

          let length = ref.values.length
          ref.values.append(2)
          let length2 = ref.values.length
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("container"):
				return container, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script3, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

// TestRuntimeStorageMultipleTransactionsResourceFunction tests a function call
// of a stored resource declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsResourceFunction(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	deepThought := []byte(`
      pub resource DeepThought {

        pub fun answer(): Int {
          return 42
        }
      }

      pub fun createDeepThought(): @DeepThought {
        return <-create DeepThought()
      }
    `)

	script1 := []byte(`
      import "deep-thought"

      transaction {

        prepare(signer: AuthAccount) {
          signer.save(<-createDeepThought(), to: /storage/deepThought)
        }
      }
    `)

	script2 := []byte(`
      import "deep-thought"

      transaction {
        prepare(signer: AuthAccount) {
          let answer = signer.borrow<&DeepThought>(from: /storage/deepThought)?.answer()
          log(answer ?? 0)
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("deep-thought"):
				return deepThought, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Contains(t, loggedMessages, "42")
}

// TestRuntimeStorageMultipleTransactionsResourceField tests reading a field
// of a stored resource declared in an imported program
//
func TestRuntimeStorageMultipleTransactionsResourceField(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource Number {
        pub(set) var n: Int
        init(_ n: Int) {
          self.n = n
        }
      }

      pub fun createNumber(_ n: Int): @Number {
        return <-create Number(n)
      }
    `)

	script1 := []byte(`
      import "imported"

      transaction {
        prepare(signer: AuthAccount) {
          signer.save(<-createNumber(42), to: /storage/number)
        }
      }
    `)

	script2 := []byte(`
      import "imported"

      transaction {
        prepare(signer: AuthAccount) {
          if let number <- signer.load<@Number>(from: /storage/number) {
            log(number.n)
            destroy number
          }
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Contains(t, loggedMessages, "42")
}

// TestRuntimeCompositeFunctionInvocationFromImportingProgram checks
// that member functions of imported composites can be invoked from an importing program.
// See https://github.com/dapperlabs/flow-go/issues/838
//
func TestRuntimeCompositeFunctionInvocationFromImportingProgram(t *testing.T) {

	t.Parallel()

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

      pub fun createY(): @Y {
        return <-create Y()
      }
    `)

	script1 := []byte(`
      import Y, createY from "imported"

      transaction {
        prepare(signer: AuthAccount) {
          signer.save(<-createY(), to: /storage/y)
        }
      }
    `)

	script2 := []byte(`
      import Y from "imported"

      transaction {
        prepare(signer: AuthAccount) {
          let y <- signer.load<@Y>(from: /storage/y)
          y?.x()
          destroy y
        }
      }
    `)

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

func TestRuntimeResourceContractUseThroughReference(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun x() {
          log("x!")
        }
      }

      pub fun createR(): @R {
        return <- create R()
      }
    `)

	script1 := []byte(`
      import R, createR from "imported"

      transaction {

        prepare(signer: AuthAccount) {
          signer.save(<-createR(), to: /storage/r)
        }
      }
    `)

	script2 := []byte(`
      import R from "imported"

      transaction {

        prepare(signer: AuthAccount) {
          let ref = signer.borrow<&R>(from: /storage/r)!
          ref.x()
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestRuntimeResourceContractUseThroughLink(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun x() {
          log("x!")
        }
      }

      pub fun createR(): @R {
          return <- create R()
      }
    `)

	script1 := []byte(`
      import R, createR from "imported"

      transaction {

        prepare(signer: AuthAccount) {
          signer.save(<-createR(), to: /storage/r)
          signer.link<&R>(/public/r, target: /storage/r)
        }
      }
    `)

	script2 := []byte(`
      import R from "imported"

      transaction {
        prepare(signer: AuthAccount) {
          let publicAccount = getAccount(signer.address)
          let ref = publicAccount.getCapability(/public/r)!.borrow<&R>()!
          ref.x()
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestRuntimeResourceContractWithInterface(t *testing.T) {

	t.Parallel()

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

      pub fun createR(): @R {
        return <- create R()
      }
    `)

	script1 := []byte(`
      import RI from "imported1"
      import R, createR from "imported2"

      transaction {
        prepare(signer: AuthAccount) {
          signer.save(<-createR(), to: /storage/r)
          signer.link<&AnyResource{RI}>(/public/r, target: /storage/r)
        }
      }
    `)

	// TODO: Get rid of the requirement that the underlying type must be imported.
	//   This requires properly initializing Interpreter.CompositeFunctions.
	//   Also initialize Interpreter.DestructorFunctions

	script2 := []byte(`
      import RI from "imported1"
      import R from "imported2"

      transaction {
        prepare(signer: AuthAccount) {
          let ref = signer.getCapability(/public/r)!.borrow<&AnyResource{RI}>()!
          ref.x()
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported1"):
				return imported1, nil
			case StringLocation("imported2"):
				return imported2, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"\"x!\""}, loggedMessages)
}

func TestParseAndCheckProgram(t *testing.T) {

	t.Parallel()

	t.Run("ValidProgram", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte("pub fun test(): Int { return 42 }")
		runtimeInterface := &testRuntimeInterface{}

		nextTransactionLocation := newTransactionLocationGenerator()

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nextTransactionLocation())
		assert.NoError(t, err)
	})

	t.Run("InvalidSyntax", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte("invalid syntax")
		runtimeInterface := &testRuntimeInterface{}

		nextTransactionLocation := newTransactionLocationGenerator()

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nextTransactionLocation())
		assert.NotNil(t, err)
	})

	t.Run("InvalidSemantics", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte(`pub let a: Int = "b"`)
		runtimeInterface := &testRuntimeInterface{}

		nextTransactionLocation := newTransactionLocationGenerator()

		err := runtime.ParseAndCheckProgram(script, runtimeInterface, nextTransactionLocation())
		assert.NotNil(t, err)
	})
}

func TestRuntimeSyntaxError(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      pub fun main(): String {
          return "Hello World!
      }
    `)

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	_, err := runtime.ExecuteScript(script, nil, runtimeInterface, nextTransactionLocation())
	assert.Error(t, err)
}

func TestRuntimeStorageChanges(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource X {
        pub(set) var x: Int

        init() {
          self.x = 0
        }
      }

      pub fun createX(): @X {
          return <-create X()
      }
    `)

	script1 := []byte(`
      import X, createX from "imported"

      transaction {
        prepare(signer: AuthAccount) {
          signer.save(<-createX(), to: /storage/x)

          let ref = signer.borrow<&X>(from: /storage/x)!
          ref.x = 1
        }
      }
    `)

	script2 := []byte(`
      import X from "imported"

      transaction {
        prepare(signer: AuthAccount) {
          let ref = signer.borrow<&X>(from: /storage/x)!
          log(ref.x)
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"1"}, loggedMessages)
}

func TestRuntimeAccountAddress(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare(signer: AuthAccount) {
          log(signer.address)
        }
      }
    `)

	var loggedMessages []string

	address := common.BytesToAddress([]byte{42})

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return []Address{address}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"0x2a"}, loggedMessages)
}

func TestRuntimePublicAccountAddress(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {
          log(getAccount(0x42).address)
        }
      }
    `)

	var loggedMessages []string

	address := interpreter.NewAddressValueFromBytes([]byte{0x42})

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return nil
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{fmt.Sprint(address)}, loggedMessages)
}

func TestRuntimeAccountPublishAndAccess(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported := []byte(`
      pub resource R {
        pub fun test(): Int {
          return 42
        }
      }

      pub fun createR(): @R {
        return <-create R()
      }
    `)

	script1 := []byte(`
      import "imported"

      transaction {
        prepare(signer: AuthAccount) {
          signer.save(<-createR(), to: /storage/r)
          signer.link<&R>(/public/r, target: /storage/r)
        }
      }
    `)

	address := common.BytesToAddress([]byte{42})

	script2 := []byte(
		fmt.Sprintf(
			`
              import "imported"

              transaction {

                prepare(signer: AuthAccount) {
                  log(getAccount(0x%s).getCapability(/public/r)!.borrow<&R>()!.test())
                }
              }
            `,
			address,
		),
	)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) ([]byte, error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{address}
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, []string{"42"}, loggedMessages)
}

func TestRuntimeTransaction_UpdateAccountCodeEmpty(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {

          prepare(signer: AuthAccount) {
              signer.setCode([])
          }
      }
    `)

	var accountCode []byte
	var events []cadence.Event

	runtimeInterface := &testRuntimeInterface{
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())

	require.NoError(t, err)

	assert.NotNil(t, accountCode)
	assert.Len(t, events, 1)
}

func TestRuntimeTransaction_CreateAccountEmpty(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare(signer: AuthAccount) {
          AuthAccount(payer: signer)
        }
      }
    `)

	var events []cadence.Event

	runtimeInterface := &testRuntimeInterface{
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{{42}}
		},
		createAccount: func(payer Address) (address Address, err error) {
			return Address{42}, nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, events, 1)
}

func TestRuntimeTransaction_AddPublicKey(t *testing.T) {
	runtime := NewInterpreterRuntime()

	keyA := cadence.NewArray([]cadence.Value{
		cadence.NewInt(1),
		cadence.NewInt(2),
		cadence.NewInt(3),
	})

	keyB := cadence.NewArray([]cadence.Value{
		cadence.NewInt(3),
		cadence.NewInt(4),
		cadence.NewInt(5),
	})

	keys := cadence.NewArray([]cadence.Value{
		keyA,
		keyB,
	})

	var tests = []struct {
		name     string
		code     string
		keyCount int
		args     []cadence.Value
	}{
		{
			name: "Single key",
			code: `
			  transaction(keyA: [Int]) {
				prepare(signer: AuthAccount) {
				  let acct = AuthAccount(payer: signer)
				  acct.addPublicKey(keyA)
				}
			  }
			`,
			keyCount: 1,
			args:     []cadence.Value{keyA},
		},
		{
			name: "Multiple keys",
			code: `
			  transaction(keys: [[Int]]) {
				prepare(signer: AuthAccount) {
				  let acct = AuthAccount(payer: signer)
				  for key in keys {
					acct.addPublicKey(key)
				  }
				}
			  }
			`,
			keyCount: 2,
			args:     []cadence.Value{keys},
		},
	}

	for _, tt := range tests {

		var events []cadence.Event
		var keys [][]byte

		runtimeInterface := &testRuntimeInterface{
			storage: newTestStorage(nil, nil),
			getSigningAccounts: func() []Address {
				return []Address{{42}}
			},
			createAccount: func(payer Address) (address Address, err error) {
				return Address{42}, nil
			},
			addAccountKey: func(address Address, publicKey []byte) error {
				keys = append(keys, publicKey)
				return nil
			},
			emitEvent: func(event cadence.Event) {
				events = append(events, event)
			},
			decodeArgument: func(b []byte, t cadence.Type) (value cadence.Value, err error) {
				return jsoncdc.Decode(b)
			},
		}

		t.Run(tt.name, func(t *testing.T) {
			args := make([][]byte, len(tt.args))
			for i, arg := range tt.args {
				args[i], _ = jsoncdc.Encode(arg)
			}

			err := runtime.ExecuteTransaction([]byte(tt.code), args, runtimeInterface, utils.TestLocation)
			require.NoError(t, err)
			assert.Len(t, events, tt.keyCount+1)
			assert.Len(t, keys, tt.keyCount)

			assert.EqualValues(t, stdlib.AccountCreatedEventType.ID(), events[0].Type().ID())

			for _, event := range events[1:] {
				assert.EqualValues(t, stdlib.AccountKeyAddedEventType.ID(), event.Type().ID())
			}
		})
	}
}

func TestRuntimeCyclicImport(t *testing.T) {

	t.Parallel()

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
		resolveImport: func(location Location) (bytes []byte, err error) {
			switch location {
			case StringLocation("imported"):
				return imported, nil
			default:
				return nil, fmt.Errorf("unknown import location: %s", location)
			}
		},
		getSigningAccounts: func() []Address {
			return nil
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())

	require.Error(t, err)
	require.IsType(t, Error{}, err)
	assert.IsType(t, ast.CyclicImportsError{}, err.(Error).Unwrap())
}

func ArrayValueFromBytes(bytes []byte) *interpreter.ArrayValue {
	byteValues := make([]interpreter.Value, len(bytes))

	for i, b := range bytes {
		byteValues[i] = interpreter.UInt8Value(b)
	}

	return interpreter.NewArrayValueUnownedNonCopying(byteValues...)
}

func TestRuntimeTransactionWithContractDeployment(t *testing.T) {

	t.Parallel()

	expectSuccess := func(t *testing.T, err error, accountCode []byte, events []cadence.Event, expectedEventType cadence.Type) {
		require.NoError(t, err)

		assert.NotNil(t, accountCode)

		require.Len(t, events, 1)

		event := events[0]

		require.Equal(t, event.Type(), expectedEventType)

		expectedEventCompositeType := expectedEventType.(cadence.EventType)

		codeHashParameterIndex := -1

		for i, field := range expectedEventCompositeType.Fields {
			if field.Identifier != stdlib.AccountEventCodeHashParameter.Identifier {
				continue
			}
			codeHashParameterIndex = i
		}

		if codeHashParameterIndex < 0 {
			t.Error("couldn't find code hash parameter in event type")
		}

		expectedCodeHash := sha3.Sum256(accountCode)

		codeHashValue := event.Fields[codeHashParameterIndex]

		actualCodeHash, err := interpreter.ByteArrayValueToByteSlice(importValue(codeHashValue))
		require.NoError(t, err)

		require.Equal(t, expectedCodeHash[:], actualCodeHash)
	}

	expectFailure := func(t *testing.T, err error, accountCode []byte, events []cadence.Event, _ cadence.Type) {
		require.Error(t, err)

		assert.Nil(t, accountCode)
		assert.Len(t, events, 0)
	}

	type argument interface {
		fmt.Stringer
		interpreter.Value
	}

	type test struct {
		name      string
		contract  string
		arguments []argument
		check     func(t *testing.T, err error, accountCode []byte, events []cadence.Event, expectedEventType cadence.Type)
	}

	tests := []test{
		{
			name: "no arguments",
			contract: `
              pub contract Test {}
            `,
			arguments: []argument{},
			check:     expectSuccess,
		},
		{
			name: "with argument",
			contract: `
              pub contract Test {
                  init(_ x: Int) {}
              }
            `,
			arguments: []argument{
				interpreter.NewIntValueFromInt64(1),
			},
			check: expectSuccess,
		},
		{
			name: "with incorrect argument",
			contract: `
              pub contract Test {
                  init(_ x: Int) {}
              }
            `,
			arguments: []argument{
				interpreter.BoolValue(true),
			},
			check: expectFailure,
		},
		{
			name: "additional argument",
			contract: `
              pub contract Test {}
            `,
			arguments: []argument{
				interpreter.NewIntValueFromInt64(1),
			},
			check: expectFailure,
		},
		{
			name: "additional code which is invalid at top-level",
			contract: `
              pub contract Test {}

              fun test() {}
            `,
			arguments: []argument{},
			check:     expectFailure,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			contractArrayCode := ArrayValueFromBytes([]byte(test.contract)).String()

			argumentCodes := make([]string, len(test.arguments))

			for i, argument := range test.arguments {
				argumentCodes[i] = argument.String()
			}

			argumentCode := strings.Join(argumentCodes, ", ")
			if len(test.arguments) > 0 {
				argumentCode = ", " + argumentCode
			}

			script := []byte(fmt.Sprintf(
				`
                      transaction {

                          prepare(signer: AuthAccount) {
                              signer.setCode(%s%s)
                          }
                      }
                    `,
				contractArrayCode,
				argumentCode,
			))

			runtime := NewInterpreterRuntime()

			var accountCode []byte
			var events []cadence.Event

			runtimeInterface := &testRuntimeInterface{
				storage: newTestStorage(nil, nil),
				getSigningAccounts: func() []Address {
					return []Address{{42}}
				},
				updateAccountCode: func(address Address, code []byte) (err error) {
					accountCode = code
					return nil
				},
				emitEvent: func(event cadence.Event) {
					events = append(events, event)
				},
			}

			nextTransactionLocation := newTransactionLocationGenerator()

			err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())

			test.check(t, err, accountCode, events, exportType(stdlib.AccountCodeUpdatedEventType))
		})
	}
}

func TestRuntimeContractAccount(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	addressValue := cadence.BytesToAddress([]byte{0xCA, 0xDE})

	contract := []byte(`
      pub contract Test {
          pub let address: Address

          init() {
              // field 'account' can be used, as it is considered initialized
              self.address = self.account.address
          }

          // test that both functions are linked back into restored composite values,
          // and also injected fields are injected back into restored composite values
          //
          pub fun test(): Address {
              return self.account.address
          }
      }
    `)

	script1 := []byte(`
      import Test from 0xCADE

      pub fun main(): Address {
          return Test.address
      }
    `)

	script2 := []byte(`
      import Test from 0xCADE

      pub fun main(): Address {
          return Test.test()
      }
    `)

	deploy := []byte(fmt.Sprintf(
		`
          transaction {

              prepare(signer: AuthAccount) {
                  signer.setCode(%s)
              }
          }
        `,
		ArrayValueFromBytes(contract).String(),
	))

	var accountCode []byte
	var events []cadence.Event

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(_ Location) (bytes []byte, err error) {
			return accountCode, nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{common.BytesToAddress(addressValue.Bytes())}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.NotNil(t, accountCode)

	t.Run("", func(t *testing.T) {
		value, err := runtime.ExecuteScript(script1, nil, runtimeInterface, nextTransactionLocation())
		require.NoError(t, err)

		assert.Equal(t, addressValue, value)
	})

	t.Run("", func(t *testing.T) {
		value, err := runtime.ExecuteScript(script2, nil, runtimeInterface, nextTransactionLocation())
		require.NoError(t, err)

		assert.Equal(t, addressValue, value)
	})
}

func TestRuntimeContractNestedResource(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	addressValue := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	contract := []byte(`
        pub contract Test {
            pub resource R {
                // test that the hello function is linked back into the nested resource
                // after being loaded from storage
                pub fun hello(): String {
                    return "Hello World!"
                }
            }

            init() {
                // store nested resource in account on deployment
                self.account.save(<-create R(), to: /storage/r)
            }
        }
    `)

	tx := []byte(`
		import Test from 0x01

		transaction {

			prepare(acct: AuthAccount) {
				log(acct.borrow<&Test.R>(from: /storage/r)?.hello())
			}
		}
	`)

	deploy := []byte(fmt.Sprintf(
		`
        transaction {

            prepare(signer: AuthAccount) {
                signer.setCode(%s)
            }
        }
        `,
		ArrayValueFromBytes(contract).String(),
	))

	var accountCode []byte
	var loggedMessage string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(_ Location) (bytes []byte, err error) {
			return accountCode, nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{addressValue}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {},
		log: func(message string) {
			loggedMessage = message
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.NotNil(t, accountCode)

	err = runtime.ExecuteTransaction(tx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t, `"Hello World!"`, loggedMessage)
}

const fungibleTokenContract = `
pub contract FungibleToken {

    pub resource interface Provider {
        pub fun withdraw(amount: Int): @Vault {
            pre {
                amount > 0:
                    "Withdrawal amount must be positive"
            }
            post {
                result.balance == amount:
                    "Incorrect amount returned"
            }
        }
    }

    pub resource interface Receiver {
        pub balance: Int

        init(balance: Int) {
            pre {
                balance >= 0:
                    "Initial balance must be non-negative"
            }
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }

        pub fun deposit(from: @AnyResource{Receiver}) {
            pre {
                from.balance > 0:
                    "Deposit balance needs to be positive!"
            }
            post {
                self.balance == before(self.balance) + before(from.balance):
                    "Incorrect amount removed"
            }
        }
    }

    pub resource Vault: Provider, Receiver {

        pub var balance: Int

        init(balance: Int) {
            self.balance = balance
        }

        pub fun withdraw(amount: Int): @Vault {
            self.balance = self.balance - amount
            return <-create Vault(balance: amount)
        }

        // transfer combines withdraw and deposit into one function call
        pub fun transfer(to: &AnyResource{Receiver}, amount: Int) {
            pre {
                amount <= self.balance:
                    "Insufficient funds"
            }
            post {
                self.balance == before(self.balance) - amount:
                    "Incorrect amount removed"
            }
            to.deposit(from: <-self.withdraw(amount: amount))
        }

        pub fun deposit(from: @AnyResource{Receiver}) {
            self.balance = self.balance + from.balance
            destroy from
        }

        pub fun createEmptyVault(): @Vault {
            return <-create Vault(balance: 0)
        }
    }

    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balance: 0)
    }

    pub resource VaultMinter {
        pub fun mintTokens(amount: Int, recipient: &AnyResource{Receiver}) {
            recipient.deposit(from: <-create Vault(balance: amount))
        }
    }

    init() {
        self.account.save(<-create Vault(balance: 30), to: /storage/vault)
        self.account.save(<-create VaultMinter(), to: /storage/minter)
    }
}
`

func TestRuntimeFungibleTokenUpdateAccountCode(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	address1Value := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	address2Value := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2,
	}

	deploy := []byte(fmt.Sprintf(
		`
          transaction {

              prepare(signer: AuthAccount) {
                  signer.setCode(%s)
              }
          }
        `,
		ArrayValueFromBytes([]byte(fungibleTokenContract)).String(),
	))

	setup1Transaction := []byte(`
      import FungibleToken from 0x01

      transaction {

          prepare(acct: AuthAccount) {

              acct.link<&AnyResource{FungibleToken.Receiver}>(
                  /public/receiver,
                  target: /storage/vault
              )

              acct.link<&FungibleToken.Vault>(
                  /private/vault,
                  target: /storage/vault
              )
          }
      }
    `)

	setup2Transaction := []byte(`
      // NOTE: import location not the same as in setup1Transaction
      import FungibleToken from 0x01

      transaction {

          prepare(acct: AuthAccount) {
              let vault <- FungibleToken.createEmptyVault()

              acct.save(<-vault, to: /storage/vault)

              acct.link<&AnyResource{FungibleToken.Receiver}>(
                  /public/receiver,
                  target: /storage/vault
              )

              acct.link<&FungibleToken.Vault>(
                  /private/vault,
                  target: /storage/vault
              )
          }
      }
    `)

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	signerAccount := address1Value

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{signerAccount}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(setup1Transaction, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	signerAccount = address2Value

	err = runtime.ExecuteTransaction(setup2Transaction, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

func TestRuntimeFungibleTokenCreateAccount(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	address1Value := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	address2Value := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2,
	}

	deploy := []byte(fmt.Sprintf(
		`
          transaction {
            prepare(signer: AuthAccount) {
                let acct = AuthAccount(payer: signer)
                acct.setCode(%s)
            }
          }
        `,
		ArrayValueFromBytes([]byte(fungibleTokenContract)).String(),
	))

	setup1Transaction := []byte(`
      import FungibleToken from 0x2

      transaction {

          prepare(acct: AuthAccount) {
              acct.link<&AnyResource{FungibleToken.Receiver}>(
                  /public/receiver,
                  target: /storage/vault
              )

              acct.link<&FungibleToken.Vault>(
                  /private/vault,
                  target: /storage/vault
              )
          }
      }
    `)

	setup2Transaction := []byte(`
      // NOTE: import location not the same as in setup1Transaction
      import FungibleToken from 0x02

      transaction {

          prepare(acct: AuthAccount) {
              let vault <- FungibleToken.createEmptyVault()

              acct.save(<-vault, to: /storage/vault)

              acct.link<&AnyResource{FungibleToken.Receiver}>(
                  /public/receiver,
                  target: /storage/vault
              )

              acct.link<&FungibleToken.Vault>(
                  /private/vault,
                  target: /storage/vault
              )
          }
      }
    `)

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	signerAccount := address1Value

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		createAccount: func(payer Address) (address Address, err error) {
			return address2Value, nil
		},
		getSigningAccounts: func() []Address {
			return []Address{signerAccount}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(setup1Transaction, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(setup2Transaction, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

func TestRuntimeInvokeStoredInterfaceFunction(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	makeDeployTransaction := func(code string) []byte {
		return []byte(fmt.Sprintf(
			`
              transaction {
                prepare(signer: AuthAccount) {
                  let acct = AuthAccount(payer: signer)
                  acct.setCode(%s)
                }
              }
            `,
			ArrayValueFromBytes([]byte(code)).String(),
		))
	}

	contractInterfaceCode := `
      pub contract interface TestContractInterface {

          pub resource interface RInterface {

              pub fun check(a: Int, b: Int) {
                  pre { a > 1 }
                  post { b > 1 }
              }
          }
      }
	`

	contractCode := `
	  import TestContractInterface from 0x2

	  pub contract TestContract: TestContractInterface {

	      pub resource R: TestContractInterface.RInterface {

	          pub fun check(a: Int, b: Int) {
	              pre { a < 3 }
                  post { b < 3 }
	          }
	      }

	      pub fun createR(): @R {
	          return <-create R()
	      }
	   }
	`

	setupCode := []byte(`
	  import TestContractInterface from 0x2
	  import TestContract from 0x3

	  transaction {
	      prepare(signer: AuthAccount) {
	          signer.save(<-TestContract.createR(), to: /storage/r)
	      }
	  }
	`)

	makeUseCode := func(a int, b int) []byte {
		return []byte(
			fmt.Sprintf(
				`
	              import TestContractInterface from 0x2

                  // NOTE: *not* importing concrete implementation.
                  //   Should be imported automatically when loading the value from storage

	              // import TestContract from 0x3

	              transaction {
	                  prepare(signer: AuthAccount) {
	                      signer.borrow<&AnyResource{TestContractInterface.RInterface}>(from: /storage/r)?.check(a: %d, b: %d)
	                  }
	              }
	            `,
				a,
				b,
			),
		)
	}

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	var nextAccount byte = 0x2

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		createAccount: func(payer Address) (address Address, err error) {
			result := interpreter.NewAddressValueFromBytes([]byte{nextAccount})
			nextAccount++
			return result.ToAddress(), nil
		},
		getSigningAccounts: func() []Address {
			return []Address{{0x1}}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(
		makeDeployTransaction(contractInterfaceCode),
		nil,
		runtimeInterface,
		nextTransactionLocation(),
	)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(
		makeDeployTransaction(contractCode),
		nil,
		runtimeInterface,
		nextTransactionLocation(),
	)
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(
		setupCode,
		nil,
		runtimeInterface,
		nextTransactionLocation(),
	)
	require.NoError(t, err)

	for a := 1; a <= 3; a++ {
		for b := 1; b <= 3; b++ {

			t.Run(fmt.Sprintf("%d/%d", a, b), func(t *testing.T) {

				err = runtime.ExecuteTransaction(
					makeUseCode(a, b),
					nil,
					runtimeInterface,
					nextTransactionLocation(),
				)

				if a == 2 && b == 2 {
					assert.NoError(t, err)
				} else {
					require.Error(t, err)
					require.IsType(t, Error{}, err)
					assert.IsType(t, &interpreter.ConditionError{}, err.(Error).Err)
				}
			})
		}
	}
}

func TestRuntimeBlock(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {
          let block = getCurrentBlock()
          log(block)
          log(block.height)
          log(block.id)
          log(block.timestamp)

          let nextBlock = getBlock(at: block.height + UInt64(1))
          log(nextBlock)
          log(nextBlock?.height)
          log(nextBlock?.id)
          log(nextBlock?.timestamp)
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return nil
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"Block(height: 1, id: 0x0000000000000000000000000000000000000000000000000000000000000001, timestamp: 1.00000000)",
			"1",
			"[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1]",
			"1.00000000",
			"Block(height: 2, id: 0x0000000000000000000000000000000000000000000000000000000000000002, timestamp: 2.00000000)",
			"2",
			"[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2]",
			"2.00000000",
		},
		loggedMessages,
	)
}

func TestUnsafeRandom(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {
          let rand = unsafeRandom()
          log(rand)
        }
      }
    `)

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		unsafeRandom: func() uint64 {
			return 7558174677681708339
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"7558174677681708339",
		},
		loggedMessages,
	)
}

func TestRuntimeTransactionTopLevelDeclarations(t *testing.T) {

	t.Parallel()

	t.Run("transaction with function", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte(`
          pub fun test() {}

          transaction {}
        `)

		runtimeInterface := &testRuntimeInterface{
			getSigningAccounts: func() []Address {
				return nil
			},
		}

		nextTransactionLocation := newTransactionLocationGenerator()

		err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
		require.NoError(t, err)
	})

	t.Run("transaction with resource", func(t *testing.T) {
		runtime := NewInterpreterRuntime()

		script := []byte(`
          pub resource R {}

          transaction {}
        `)

		runtimeInterface := &testRuntimeInterface{
			getSigningAccounts: func() []Address {
				return nil
			},
		}

		nextTransactionLocation := newTransactionLocationGenerator()

		err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
		require.Error(t, err)

		require.IsType(t, Error{}, err)
		err = err.(Error).Unwrap()

		errs := checker.ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidTopLevelDeclarationError{}, errs[0])
	})
}

func TestRuntimeStoreIntegerTypes(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	addressValue := interpreter.AddressValue{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xCA, 0xDE,
	}

	for _, integerType := range sema.AllIntegerTypes {

		typeName := integerType.String()

		t.Run(typeName, func(t *testing.T) {

			contract := []byte(
				fmt.Sprintf(
					`
                      pub contract Test {

                          pub let n: %s

                          init() {
                              self.n = 42
                          }
                      }
                    `,
					typeName,
				),
			)

			deploy := []byte(
				fmt.Sprintf(
					`
                      transaction {

                          prepare(signer: AuthAccount) {
                              signer.setCode(%s)
                          }
                      }
                    `,
					ArrayValueFromBytes(contract).String(),
				),
			)

			var accountCode []byte
			var events []cadence.Event

			runtimeInterface := &testRuntimeInterface{
				resolveImport: func(_ Location) (bytes []byte, err error) {
					return accountCode, nil
				},
				storage: newTestStorage(nil, nil),
				getSigningAccounts: func() []Address {
					return []Address{addressValue.ToAddress()}
				},
				updateAccountCode: func(address Address, code []byte) (err error) {
					accountCode = code
					return nil
				},
				emitEvent: func(event cadence.Event) {
					events = append(events, event)
				},
			}

			nextTransactionLocation := newTransactionLocationGenerator()

			err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
			require.NoError(t, err)

			assert.NotNil(t, accountCode)
		})
	}
}

func TestInterpretResourceOwnerFieldUseComposite(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	address := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	contract := []byte(`
      pub contract Test {

          pub resource R {

              pub fun logOwnerAddress() {
                log(self.owner?.address)
              }
          }

          pub fun createR(): @R {
              return <-create R()
          }
      }
    `)

	deploy := []byte(
		fmt.Sprintf(
			`
              transaction {

                  prepare(signer: AuthAccount) {
                      signer.setCode(%s)
                  }
              }
            `,
			ArrayValueFromBytes(contract).String(),
		),
	)

	tx := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {

              let r <- Test.createR()
              log(r.owner?.address)
              r.logOwnerAddress()

              signer.save(<-r, to: /storage/r)
              signer.link<&Test.R>(/public/r, target: /storage/r)

              let ref1 = signer.borrow<&Test.R>(from: /storage/r)!
              log(ref1.owner?.address)
              ref1.logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/r)!.borrow<&Test.R>()!
              log(ref2.owner?.address)
              ref2.logOwnerAddress()
          }
      }
    `)

	tx2 := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {
              let ref1 = signer.borrow<&Test.R>(from: /storage/r)!
              log(ref1.owner?.address)
              ref1.logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/r)!.borrow<&Test.R>()!
              log(ref2.owner?.address)
              ref2.logOwnerAddress()
          }
      }
    `)

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{address}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(tx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"nil", "nil",
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)

	loggedMessages = nil
	err = runtime.ExecuteTransaction(tx2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)
}

func TestInterpretResourceOwnerFieldUseArray(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	address := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	contract := []byte(`
      pub contract Test {

          pub resource R {

              pub fun logOwnerAddress() {
                log(self.owner?.address)
              }
          }

          pub fun createR(): @R {
              return <-create R()
          }
      }
    `)

	deploy := []byte(
		fmt.Sprintf(
			`
              transaction {

                  prepare(signer: AuthAccount) {
                      signer.setCode(%s)
                  }
              }
            `,
			ArrayValueFromBytes(contract).String(),
		),
	)

	tx := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {

              let rs <- [
                  <-Test.createR(),
                  <-Test.createR()
              ]
              log(rs[0].owner?.address)
              log(rs[1].owner?.address)
              rs[0].logOwnerAddress()
              rs[1].logOwnerAddress()

              signer.save(<-rs, to: /storage/rs)
              signer.link<&[Test.R]>(/public/rs, target: /storage/rs)

              let ref1 = signer.borrow<&[Test.R]>(from: /storage/rs)!
              log(ref1[0].owner?.address)
              log(ref1[1].owner?.address)
              ref1[0].logOwnerAddress()
              ref1[1].logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/rs)!.borrow<&[Test.R]>()!
              log(ref2[0].owner?.address)
              log(ref2[1].owner?.address)
              ref2[0].logOwnerAddress()
              ref2[1].logOwnerAddress()
          }
      }
    `)

	tx2 := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {
              let ref1 = signer.borrow<&[Test.R]>(from: /storage/rs)!
              log(ref1[0].owner?.address)
              log(ref1[1].owner?.address)
              ref1[0].logOwnerAddress()
              ref1[1].logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/rs)!.borrow<&[Test.R]>()!
              log(ref2[0].owner?.address)
              log(ref2[1].owner?.address)
              ref2[0].logOwnerAddress()
              ref2[1].logOwnerAddress()
          }
      }
    `)

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{address}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(tx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"nil", "nil",
			"nil", "nil",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)

	loggedMessages = nil
	err = runtime.ExecuteTransaction(tx2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)
}

func TestInterpretResourceOwnerFieldUseDictionary(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	address := Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	contract := []byte(`
      pub contract Test {

          pub resource R {

              pub fun logOwnerAddress() {
                log(self.owner?.address)
              }
          }

          pub fun createR(): @R {
              return <-create R()
          }
      }
    `)

	deploy := []byte(
		fmt.Sprintf(
			`
              transaction {

                  prepare(signer: AuthAccount) {
                      signer.setCode(%s)
                  }
              }
            `,
			ArrayValueFromBytes(contract).String(),
		),
	)

	tx := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {

              let rs <- {
                  "a": <-Test.createR(),
                  "b": <-Test.createR()
              }
              log(rs["a"]?.owner?.address)
              log(rs["b"]?.owner?.address)
              rs["a"]?.logOwnerAddress()
              rs["b"]?.logOwnerAddress()

              signer.save(<-rs, to: /storage/rs)
              signer.link<&{String: Test.R}>(/public/rs, target: /storage/rs)

              let ref1 = signer.borrow<&{String: Test.R}>(from: /storage/rs)!
              log(ref1["a"]?.owner?.address)
              log(ref1["b"]?.owner?.address)
              ref1["a"]?.logOwnerAddress()
              ref1["b"]?.logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/rs)!.borrow<&{String: Test.R}>()!
              log(ref2["a"]?.owner?.address)
              log(ref2["b"]?.owner?.address)
              ref2["a"]?.logOwnerAddress()
              ref2["b"]?.logOwnerAddress()
          }
      }
    `)

	tx2 := []byte(`
      import Test from 0x1

      transaction {

          prepare(signer: AuthAccount) {
              let ref1 = signer.borrow<&{String: Test.R}>(from: /storage/rs)!
              log(ref1["a"]?.owner?.address)
              log(ref1["b"]?.owner?.address)
              ref1["a"]?.logOwnerAddress()
              ref1["b"]?.logOwnerAddress()

              let publicAccount = getAccount(0x01)
              let ref2 = publicAccount.getCapability(/public/rs)!.borrow<&{String: Test.R}>()!
              log(ref2["a"]?.owner?.address)
              log(ref2["b"]?.owner?.address)
              ref2["a"]?.logOwnerAddress()
              ref2["b"]?.logOwnerAddress()
          }
      }
    `)

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	var loggedMessages []string

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{address}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	err = runtime.ExecuteTransaction(tx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"nil", "nil",
			"nil", "nil",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)

	loggedMessages = nil
	err = runtime.ExecuteTransaction(tx2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
			"0x1", "0x1",
		},
		loggedMessages,
	)
}

func TestRuntimeComputationLimit(t *testing.T) {

	t.Parallel()

	const computationLimit = 5

	type test struct {
		name string
		code string
		ok   bool
	}

	tests := []test{
		{
			name: "Infinite while loop",
			code: `
              while true {}
            `,
			ok: false,
		},
		{
			name: "Limited while loop",
			code: `
              var i = 0
              while i < 5 {
                  i = i + 1
              }
            `,
			ok: false,
		},
		{
			name: "Too many for-in loop iterations",
			code: `
              for i in [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] {}
            `,
			ok: false,
		},
		{
			name: "Some for-in loop iterations",
			code: `
              for i in [1, 2, 3, 4] {}
            `,
			ok: true,
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			script := []byte(
				fmt.Sprintf(
					`
                      transaction {
                          prepare() {
                              %s
                          }
                      }
                    `,
					test.code,
				),
			)

			runtime := NewInterpreterRuntime()

			runtimeInterface := &testRuntimeInterface{
				getSigningAccounts: func() []Address {
					return nil
				},
				computationLimit: computationLimit,
			}

			nextTransactionLocation := newTransactionLocationGenerator()

			err := runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
			if test.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				require.IsType(t, Error{}, err)
				err = err.(Error).Unwrap()

				assert.Equal(t,
					ComputationLimitExceededError{
						Limit: computationLimit,
					},
					err,
				)
			}
		})
	}
}

func TestRuntimeMetrics(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	imported1Location := StringLocation("imported1")

	importedScript1 := []byte(`
      pub fun generate(): [Int] {
        return [1, 2, 3]
      }
    `)

	imported2Location := StringLocation("imported2")

	importedScript2 := []byte(`
      pub fun getPath(): Path {
        return /storage/foo
      }
    `)

	script1 := []byte(`
      import "imported1"

      transaction {
          prepare(signer: AuthAccount) {
              signer.save(generate(), to: /storage/foo)
          }
          execute {}
      }
    `)

	script2 := []byte(`
      import "imported2"

      transaction {
          prepare(signer: AuthAccount) {
              signer.load<[Int]>(from: getPath())
          }
          execute {}
      }
    `)

	storage := newTestStorage(nil, nil)

	type reports struct {
		programParsed      map[ast.LocationID]int
		programChecked     map[ast.LocationID]int
		programInterpreted map[ast.LocationID]int
		valueEncoded       int
		valueDecoded       int
	}

	newRuntimeInterface := func() (runtimeInterface Interface, r *reports) {

		r = &reports{
			programParsed:      map[ast.LocationID]int{},
			programChecked:     map[ast.LocationID]int{},
			programInterpreted: map[ast.LocationID]int{},
		}

		runtimeInterface = &testRuntimeInterface{
			storage: storage,
			getSigningAccounts: func() []Address {
				return []Address{{42}}
			},
			resolveImport: func(location Location) (bytes []byte, err error) {
				switch location {
				case imported1Location:
					return importedScript1, nil
				case imported2Location:
					return importedScript2, nil
				default:
					return nil, fmt.Errorf("unknown import location: %s", location)
				}
			},
			programParsed: func(location ast.Location, duration time.Duration) {
				r.programParsed[location.ID()]++
			},
			programChecked: func(location ast.Location, duration time.Duration) {
				r.programChecked[location.ID()]++
			},
			programInterpreted: func(location ast.Location, duration time.Duration) {
				r.programInterpreted[location.ID()]++
			},
			valueEncoded: func(duration time.Duration) {
				r.valueEncoded++
			},
			valueDecoded: func(duration time.Duration) {
				r.valueDecoded++
			},
		}

		return
	}

	i1, r1 := newRuntimeInterface()

	nextTransactionLocation := newTransactionLocationGenerator()

	transactionLocation := nextTransactionLocation()
	err := runtime.ExecuteTransaction(script1, nil, i1, transactionLocation)
	require.NoError(t, err)

	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
			imported1Location.ID():   1,
		},
		r1.programParsed,
	)
	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
			imported1Location.ID():   1,
		},
		r1.programChecked,
	)
	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
		},
		r1.programInterpreted,
	)
	assert.Equal(t, 1, r1.valueEncoded)
	assert.Equal(t, 0, r1.valueDecoded)

	i2, r2 := newRuntimeInterface()

	transactionLocation = nextTransactionLocation()

	err = runtime.ExecuteTransaction(script2, nil, i2, transactionLocation)
	require.NoError(t, err)

	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
			imported2Location.ID():   1,
		},
		r2.programParsed,
	)
	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
			imported2Location.ID():   1,
		},
		r2.programChecked,
	)
	assert.Equal(t,
		map[ast.LocationID]int{
			transactionLocation.ID(): 1,
		},
		r2.programInterpreted,
	)
	assert.Equal(t, 0, r2.valueEncoded)
	assert.Equal(t, 1, r2.valueDecoded)
}

type testRead struct {
	controller, owner, key []byte
}

func (r testRead) String() string {
	return fmt.Sprintf("%x %s", r.controller, r.key)
}

type testWrite struct {
	controller, owner, key, value []byte
}

func (w testWrite) String() string {
	return fmt.Sprintf("%x %s", w.controller, w.key)
}

func TestRuntimeContractWriteback(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	addressValue := cadence.BytesToAddress([]byte{0xCA, 0xDE})

	contract := []byte(`
      pub contract Test {

          pub(set) var test: Int

          init() {
              self.test = 1
          }
      }
    `)

	deploy := []byte(fmt.Sprintf(
		`
          transaction {

              prepare(signer: AuthAccount) {
                  signer.setCode(%s)
              }
          }
        `,
		ArrayValueFromBytes(contract).String(),
	))

	readTx := []byte(`
      import Test from 0xCADE

       transaction {

          prepare(signer: AuthAccount) {
              log(Test.test)
          }
       }
    `)

	writeTx := []byte(`
      import Test from 0xCADE

       transaction {

          prepare(signer: AuthAccount) {
              Test.test = 2
          }
       }
    `)

	var accountCode []byte
	var events []cadence.Event
	var loggedMessages []string
	var writes []testWrite

	onWrite := func(controller, owner, key, value []byte) {
		writes = append(writes, testWrite{
			controller,
			owner,
			key,
			value,
		})
	}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(_ Location) (bytes []byte, err error) {
			return accountCode, nil
		},
		storage: newTestStorage(nil, onWrite),
		getSigningAccounts: func() []Address {
			return []Address{common.BytesToAddress(addressValue.Bytes())}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.NotNil(t, accountCode)

	assert.Len(t, writes, 1)

	err = runtime.ExecuteTransaction(readTx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, writes, 1)

	err = runtime.ExecuteTransaction(writeTx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, writes, 2)
}

func TestRuntimeStorageWriteback(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	addressValue := cadence.BytesToAddress([]byte{0xCA, 0xDE})

	contract := []byte(`
      pub contract Test {

          pub resource R {

              pub(set) var test: Int

              init() {
                  self.test = 1
              }
          }


          pub fun createR(): @R {
              return <-create R()
          }
      }
    `)

	deploy := []byte(fmt.Sprintf(
		`
          transaction {

              prepare(signer: AuthAccount) {
                  signer.setCode(%s)
              }
          }
        `,
		ArrayValueFromBytes(contract).String(),
	))

	setupTx := []byte(`
      import Test from 0xCADE

       transaction {

          prepare(signer: AuthAccount) {
              signer.save(<-Test.createR(), to: /storage/r)
          }
       }
    `)

	var accountCode []byte
	var events []cadence.Event
	var loggedMessages []string
	var writes []testWrite

	onWrite := func(controller, owner, key, value []byte) {
		writes = append(writes, testWrite{
			controller,
			owner,
			key,
			value,
		})
	}

	runtimeInterface := &testRuntimeInterface{
		resolveImport: func(_ Location) (bytes []byte, err error) {
			return accountCode, nil
		},
		storage: newTestStorage(nil, onWrite),
		getSigningAccounts: func() []Address {
			return []Address{common.BytesToAddress(addressValue.Bytes())}
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
		log: func(message string) {
			loggedMessages = append(loggedMessages, message)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	err := runtime.ExecuteTransaction(deploy, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.NotNil(t, accountCode)

	assert.Len(t, writes, 1)

	err = runtime.ExecuteTransaction(setupTx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, writes, 2)

	readTx := []byte(`
     import Test from 0xCADE

      transaction {

         prepare(signer: AuthAccount) {
             log(signer.borrow<&Test.R>(from: /storage/r)!.test)
         }
      }
    `)

	err = runtime.ExecuteTransaction(readTx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, writes, 2)

	writeTx := []byte(`
     import Test from 0xCADE

      transaction {

         prepare(signer: AuthAccount) {
             let r = signer.borrow<&Test.R>(from: /storage/r)!
             r.test = 2
         }
      }
    `)

	err = runtime.ExecuteTransaction(writeTx, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	assert.Len(t, writes, 3)
}

func TestRuntimeExternalError(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	script := []byte(`
      transaction {
        prepare() {
          log("ok")
        }
      }
    `)

	type logPanic struct{}

	runtimeInterface := &testRuntimeInterface{
		getSigningAccounts: func() []Address {
			return nil
		},
		log: func(message string) {
			panic(logPanic{})
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	assert.PanicsWithValue(t,
		interpreter.ExternalError{
			Recovered: logPanic{},
		},
		func() {
			_ = runtime.ExecuteTransaction(script, nil, runtimeInterface, nextTransactionLocation())
		},
	)
}

func TestRuntimeUpdateCodeCaching(t *testing.T) {

	t.Parallel()

	const helloWorldContract = `
      pub contract HelloWorld {

          pub let greeting: String

          init() {
              self.greeting = "Hello, World!"
          }

          pub fun hello(): String {
              return self.greeting
          }
      }
    `

	const callHelloTxTemplate = `
        import HelloWorld from 0x%s

        transaction {
            prepare(signer: AuthAccount) {
                assert(HelloWorld.hello() == "Hello, World!")
            }
        }
    `

	createAccountScript := []byte(`
        transaction {
            prepare(signer: AuthAccount) {
                AuthAccount(payer: signer)
            }
        }
    `)

	updateCodeScript := []byte(fmt.Sprintf(
		`
		  transaction {
			  prepare(signer: AuthAccount) {
				  signer.setCode(%s)
			  }
		  }
		`,
		ArrayValueFromBytes([]byte(helloWorldContract)).String(),
	))

	runtime := NewInterpreterRuntime()

	accountCodes := map[string][]byte{}
	var events []cadence.Event

	cachedPrograms := map[LocationID]*ast.Program{}

	var accountCounter uint8 = 0

	var signerAddresses []Address

	runtimeInterface := &testRuntimeInterface{
		createAccount: func(payer Address) (address Address, err error) {
			accountCounter++
			return Address{accountCounter}, nil
		},
		resolveImport: func(location Location) (bytes []byte, err error) {
			key := string(location.(AddressLocation).ID())
			return accountCodes[key], nil
		},
		cacheProgram: func(location Location, program *ast.Program) error {
			cachedPrograms[location.ID()] = program
			return nil
		},
		getCachedProgram: func(location Location) (*ast.Program, error) {
			return cachedPrograms[location.ID()], nil
		},
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return signerAddresses
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			key := string(AddressLocation(address[:]).ID())
			accountCodes[key] = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	signerAddresses = []Address{{accountCounter}}

	err := runtime.ExecuteTransaction(createAccountScript, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	signerAddresses = []Address{{accountCounter}}

	err = runtime.ExecuteTransaction(updateCodeScript, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	callScript := []byte(fmt.Sprintf(callHelloTxTemplate, Address{accountCounter}))

	err = runtime.ExecuteTransaction(callScript, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

func TestRuntimeTransaction_UpdateAccountCodeUnsafeNotInitializing(t *testing.T) {

	t.Parallel()

	runtime := NewInterpreterRuntime()

	const contract1 = `
      pub contract Test {

          pub resource R {

              pub let name: String

              init(name: String) {
                  self.name = name
              }

              pub fun hello(): Int {
                  return 1
              }
          }

          pub var rs: @{String: R}

          pub fun hello(): Int {
              return 1
          }

          init() {
              self.rs <- {}
              self.rs["r1"] <-! create R(name: "1")
          }
      }
    `

	const contract2 = `
      pub contract Test {

          pub resource R {

              pub let name: String

              init(name: String) {
                  self.name = name
              }

              pub fun hello(): Int {
                  return 2
              }
          }

          pub var rs: @{String: R}

          pub fun hello(): Int {
              return 2
          }

          init() {
              self.rs <- {}
              panic("should never be executed")
          }
      }
    `

	newDeployTransaction := func(code, function string) []byte {
		return []byte(fmt.Sprintf(
			`
              transaction {

                  prepare(signer: AuthAccount) {
                      signer.%s(%s)
                  }
              }
            `,
			function,
			ArrayValueFromBytes([]byte(code)).String(),
		))
	}

	var accountCode []byte
	var events []cadence.Event

	runtimeInterface := &testRuntimeInterface{
		storage: newTestStorage(nil, nil),
		getSigningAccounts: func() []Address {
			return []Address{common.BytesToAddress([]byte{0x42})}
		},
		resolveImport: func(_ Location) (bytes []byte, err error) {
			return accountCode, nil
		},
		updateAccountCode: func(address Address, code []byte) (err error) {
			accountCode = code
			return nil
		},
		emitEvent: func(event cadence.Event) {
			events = append(events, event)
		},
	}

	nextTransactionLocation := newTransactionLocationGenerator()

	deployTx1 := newDeployTransaction(contract1, "setCode")

	err := runtime.ExecuteTransaction(deployTx1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	script1 := []byte(`
      import 0x42

      pub fun main() {
          // Check stored data

          assert(Test.rs.length == 1)
          assert(Test.rs["r1"]?.name == "1")

          // Check functions

          assert(Test.rs["r1"]?.hello() == 1)
          assert(Test.hello() == 1)
      }
    `)

	_, err = runtime.ExecuteScript(script1, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	deployTx2 := newDeployTransaction(contract2, "unsafeNotInitializingSetCode")

	err = runtime.ExecuteTransaction(deployTx2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)

	script2 := []byte(`
      import 0x42

      pub fun main() {
          // Existing data is still available and the same as before

          assert(Test.rs.length == 1)
          assert(Test.rs["r1"]?.name == "1")

          // New function code is executed.
          // Compare with script1 above, which checked 1.

          assert(Test.rs["r1"]?.hello() == 2)
          assert(Test.hello() == 2)
      }
    `)

	_, err = runtime.ExecuteScript(script2, nil, runtimeInterface, nextTransactionLocation())
	require.NoError(t, err)
}

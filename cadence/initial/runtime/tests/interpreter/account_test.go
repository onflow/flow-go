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

package interpreter_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/cadence/runtime/trampoline"
)

func testAccount(t *testing.T, auth bool, code string) (*interpreter.Interpreter, map[string]interpreter.OptionalValue) {

	address := interpreter.NewAddressValueFromBytes([]byte{42})

	valueDeclarations := map[string]sema.ValueDeclaration{}
	values := map[string]interpreter.Value{}

	panicFunction := interpreter.NewHostFunctionValue(func(invocation interpreter.Invocation) trampoline.Trampoline {
		panic(errors.NewUnreachableError())
	})

	// `authAccount`

	valueDeclarations["authAccount"] = stdlib.StandardLibraryValue{
		Name:       "authAccount",
		Type:       &sema.AuthAccountType{},
		Kind:       common.DeclarationKindConstant,
		IsConstant: true,
	}

	values["authAccount"] = interpreter.NewAuthAccountValue(
		address,
		panicFunction,
		panicFunction,
		panicFunction,
		panicFunction,
	)

	// `pubAccount`

	valueDeclarations["pubAccount"] = stdlib.StandardLibraryValue{
		Name:       "pubAccount",
		Type:       &sema.PublicAccountType{},
		Kind:       common.DeclarationKindConstant,
		IsConstant: true,
	}

	values["pubAccount"] = interpreter.NewPublicAccountValue(address)

	// `account`

	if auth {
		valueDeclarations["account"] = valueDeclarations["authAccount"]
		values["account"] = values["authAccount"]
	} else {
		valueDeclarations["account"] = valueDeclarations["pubAccount"]
		values["account"] = values["pubAccount"]
	}

	storedValues := map[string]interpreter.OptionalValue{}

	// NOTE: checker, getter and setter are very naive for testing purposes and don't remove nil values
	//

	storageChecker := func(_ *interpreter.Interpreter, _ common.Address, key string) bool {
		_, ok := storedValues[key]
		return ok
	}

	storageSetter := func(_ *interpreter.Interpreter, _ common.Address, key string, value interpreter.OptionalValue) {
		if _, ok := value.(interpreter.NilValue); ok {
			delete(storedValues, key)
		} else {
			storedValues[key] = value
		}
	}

	storageGetter := func(_ *interpreter.Interpreter, _ common.Address, key string, deferred bool) interpreter.OptionalValue {
		value := storedValues[key]
		if value == nil {
			return interpreter.NilValue{}
		}
		return value
	}

	inter := parseCheckAndInterpretWithOptions(t,
		code,
		ParseCheckAndInterpretOptions{
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(values),
				interpreter.WithStorageExistenceHandler(storageChecker),
				interpreter.WithStorageReadHandler(storageGetter),
				interpreter.WithStorageWriteHandler(storageSetter),
			},
		},
	)

	return inter, storedValues
}

func TestInterpretAuthAccountSave(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  resource R {}

                  fun test() {
                      let r <- create R()
                      account.save(<-r, to: /storage/r)
                  }
                `,
			)

			// Save first value

			t.Run("initial save", func(t *testing.T) {

				_, err := inter.Invoke("test")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)
				for _, value := range storedValues {

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, &interpreter.CompositeValue{}, innerValue)
				}

			})

			// Attempt to save again, overwriting should fail

			t.Run("second save", func(t *testing.T) {

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.OverwriteError{}, err)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
                          resource R {}

                          fun test() {
                              let r <- create R()
                              account.save(<-r, to: /%s/r)
                          }
                        `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})

		}
	})

	t.Run("struct", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  struct S {}

                  fun test() {
                      let s = S()
                      account.save(s, to: /storage/s)
                  }
                `,
			)

			// Save first value

			t.Run("initial save", func(t *testing.T) {

				_, err := inter.Invoke("test")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)
				for _, value := range storedValues {

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, &interpreter.CompositeValue{}, innerValue)
				}

			})

			// Attempt to save again, overwriting should fail

			t.Run("second save", func(t *testing.T) {

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.OverwriteError{}, err)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
                          struct S {}

                          fun test() {
                              let s = S()
                              account.save(s, to: /%s/r)
                          }
                        `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})

		}
	})
}

func TestInterpretAuthAccountLoad(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  resource R {}

                  resource R2 {}

                  fun save() {
                      let r <- create R()
                      account.save(<-r, to: /storage/r)
                  }

                  fun loadR(): @R? {
                      return <-account.load<@R>(from: /storage/r)
                  }

                  fun loadR2(): @R2? {
                      return <-account.load<@R2>(from: /storage/r)
                  }
                `,
			)

			t.Run("save R and load R ", func(t *testing.T) {

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				// first load

				value, err := inter.Invoke("loadR")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue := value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.CompositeValue{}, innerValue)

				// NOTE: check loaded value was removed from storage
				require.Len(t, storedValues, 0)

				// second load

				value, err = inter.Invoke("loadR")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)
			})

			t.Run("save R and load R2", func(t *testing.T) {

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				// load

				value, err := inter.Invoke("loadR2")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                       resource R {}

	                       fun test(): @R? {
	                           return <-account.load<@R>(from: /%s/r)
	                       }
	                     `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})

	t.Run("struct", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  struct S {}

                  struct S2 {}

                  fun save() {
                      let s = S()
                      account.save(s, to: /storage/s)
                  }

                  fun loadS(): S? {
                      return account.load<S>(from: /storage/s)
                  }

                  fun loadS2(): S2? {
                      return account.load<S2>(from: /storage/s)
                  }
                `,
			)

			t.Run("save S and load S", func(t *testing.T) {

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				// first load

				value, err := inter.Invoke("loadS")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue := value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.CompositeValue{}, innerValue)

				// NOTE: check loaded value was removed from storage
				require.Len(t, storedValues, 0)

				// second load

				value, err = inter.Invoke("loadS")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)
			})

			t.Run("save S and load S2", func(t *testing.T) {

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				// load

				value, err := inter.Invoke("loadS2")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                       struct S {}

	                       fun test(): S? {
	                           return account.load<S>(from: /%s/s)
	                       }
	                     `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})
}

func TestInterpretAuthAccountCopy(t *testing.T) {

	t.Parallel()

	t.Run("valid", func(t *testing.T) {

		const code = `
          struct S {}

          struct S2 {}

          fun save() {
              let s = S()
              account.save(s, to: /storage/s)
          }

          fun copyS(): S? {
              return account.copy<S>(from: /storage/s)
          }

          fun copyS2(): S2? {
              return account.copy<S2>(from: /storage/s)
          }
        `

		t.Run("save S and copy S ", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				code,
			)

			// save

			_, err := inter.Invoke("save")
			require.NoError(t, err)

			require.Len(t, storedValues, 1)

			testCopyS := func() {

				value, err := inter.Invoke("copyS")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue := value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.CompositeValue{}, innerValue)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			}

			testCopyS()

			testCopyS()
		})

		t.Run("save S and copy S2", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				code,
			)

			// save

			_, err := inter.Invoke("save")
			require.NoError(t, err)

			require.Len(t, storedValues, 1)

			// load

			value, err := inter.Invoke("copyS2")
			require.NoError(t, err)

			require.IsType(t, interpreter.NilValue{}, value)

			// NOTE: check loaded value was *not* removed from storage
			require.Len(t, storedValues, 1)
		})
	})

	for _, domain := range common.AllPathDomainsByIdentifier {

		if domain == common.PathDomainStorage {
			continue
		}

		t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

			inter, _ := testAccount(
				t,
				true,
				fmt.Sprintf(
					`
	                  struct S {}

	                  fun test(): S? {
	                      return account.load<S>(from: /%s/s)
	                  }
	                `,
					domain.Identifier(),
				),
			)

			_, err := inter.Invoke("test")

			require.Error(t, err)

			require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
		})
	}

}

func TestInterpretAuthAccountBorrow(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  resource R {
                      let foo: Int

                      init() {
                          self.foo = 42
                      }
                  }

                  resource R2 {}

                  fun save() {
                      let r <- create R()
                      account.save(<-r, to: /storage/r)
                  }

                  fun borrowR(): &R? {
                      return account.borrow<&R>(from: /storage/r)
                  }

                  fun foo(): Int {
                      return account.borrow<&R>(from: /storage/r)!.foo
                  }

                  fun borrowR2(): &R2? {
                      return account.borrow<&R2>(from: /storage/r)
                  }
                `,
			)

			// save

			_, err := inter.Invoke("save")
			require.NoError(t, err)

			require.Len(t, storedValues, 1)

			t.Run("borrow R ", func(t *testing.T) {

				// first borrow

				value, err := inter.Invoke("borrowR")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue := value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.StorageReferenceValue{}, innerValue)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)

				// foo

				value, err = inter.Invoke("foo")
				require.NoError(t, err)

				require.Equal(t, interpreter.NewIntValueFromInt64(42), value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)

				// TODO: should fail, i.e. return nil

				// second borrow

				value, err = inter.Invoke("borrowR")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue = value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.StorageReferenceValue{}, innerValue)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})

			t.Run("borrow R2", func(t *testing.T) {

				value, err := inter.Invoke("borrowR2")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      resource R {}

	                      fun test(): &R? {
	                          return account.borrow<&R>(from: /%s/r)
	                      }
	                    `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})

	t.Run("struct", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {

			inter, storedValues := testAccount(
				t,
				true,
				`
                  struct S {
                      let foo: Int

                      init() {
                          self.foo = 42
                      }
                  }

                  struct S2 {}

                  fun save() {
                      let s = S()
                      account.save(s, to: /storage/s)
                  }

                  fun borrowS(): &S? {
                      return account.borrow<&S>(from: /storage/s)
                  }

                  fun foo(): Int {
                      return account.borrow<&S>(from: /storage/s)!.foo
                  }

                  fun borrowS2(): &S2? {
                      return account.borrow<&S2>(from: /storage/s)
                  }
                `,
			)

			// save

			_, err := inter.Invoke("save")
			require.NoError(t, err)

			require.Len(t, storedValues, 1)

			t.Run("borrow S", func(t *testing.T) {

				// first borrow

				value, err := inter.Invoke("borrowS")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue := value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.StorageReferenceValue{}, innerValue)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)

				// foo

				value, err = inter.Invoke("foo")
				require.NoError(t, err)

				require.Equal(t, interpreter.NewIntValueFromInt64(42), value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)

				// TODO: should fail, i.e. return nil

				// second borrow

				value, err = inter.Invoke("borrowS")
				require.NoError(t, err)

				require.IsType(t, &interpreter.SomeValue{}, value)

				innerValue = value.(*interpreter.SomeValue).Value

				assert.IsType(t, &interpreter.StorageReferenceValue{}, innerValue)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})

			t.Run("borrow S2", func(t *testing.T) {

				value, err := inter.Invoke("borrowS2")
				require.NoError(t, err)

				require.IsType(t, interpreter.NilValue{}, value)

				// NOTE: check loaded value was *not* removed from storage
				require.Len(t, storedValues, 1)
			})
		})

		for _, domain := range common.AllPathDomainsByIdentifier {

			if domain == common.PathDomainStorage {
				continue
			}

			t.Run(fmt.Sprintf("invalid: %s domain", domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      struct S {}

	                      fun test(): &S? {
	                          return account.borrow<&S>(from: /%s/s)
	                      }
	                    `,
						domain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})
}

func TestInterpretAuthAccountLink(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		for _, capabilityDomain := range []common.PathDomain{
			common.PathDomainPrivate,
			common.PathDomainPublic,
		} {

			t.Run(capabilityDomain.Name(), func(t *testing.T) {

				inter, storedValues := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      resource R {}

	                      resource R2 {}

	                      fun save() {
	                          let r <- create R()
	                          account.save(<-r, to: /storage/r)
	                      }

	                      fun linkR(): Capability? {
	                          return account.link<&R>(/%[1]s/r, target: /storage/r)
	                      }

	                      fun linkR2(): Capability? {
	                          return account.link<&R2>(/%[1]s/r2, target: /storage/r)
	                      }
	                    `,
						capabilityDomain.Identifier(),
					),
				)

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				t.Run("link R", func(t *testing.T) {

					// first link

					value, err := inter.Invoke("linkR")
					require.NoError(t, err)

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, interpreter.CapabilityValue{}, innerValue)

					// stored value + link
					require.Len(t, storedValues, 2)

					// second link

					value, err = inter.Invoke("linkR")
					require.NoError(t, err)

					require.IsType(t, interpreter.NilValue{}, value)

					// NOTE: check loaded value was *not* removed from storage
					require.Len(t, storedValues, 2)
				})

				t.Run("link R2", func(t *testing.T) {

					// first link

					value, err := inter.Invoke("linkR2")
					require.NoError(t, err)

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, interpreter.CapabilityValue{}, innerValue)

					// stored value + link
					require.Len(t, storedValues, 3)

					// second link

					value, err = inter.Invoke("linkR2")
					require.NoError(t, err)

					require.IsType(t, interpreter.NilValue{}, value)

					// NOTE: check loaded value was *not* removed from storage
					require.Len(t, storedValues, 3)
				})
			})
		}

		for _, targetDomain := range common.AllPathDomainsByIdentifier {

			testName := fmt.Sprintf(
				"invalid: new capability path: storage domain, target path: %s domain",
				targetDomain,
			)

			t.Run(testName, func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      resource R {}

	                      fun test() {
	                          account.link<&R>(/storage/r, target: /%s/r)
	                      }
	                    `,
						targetDomain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})

	t.Run("struct", func(t *testing.T) {

		for _, capabilityDomain := range []common.PathDomain{
			common.PathDomainPrivate,
			common.PathDomainPublic,
		} {

			t.Run(capabilityDomain.Name(), func(t *testing.T) {

				inter, storedValues := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      struct S {}

	                      struct S2 {}

	                      fun save() {
	                          let s = S()
	                          account.save(s, to: /storage/s)
	                      }

	                      fun linkS(): Capability? {
	                          return account.link<&S>(/%[1]s/s, target: /storage/s)
	                      }

	                      fun linkS2(): Capability? {
	                          return account.link<&S2>(/%[1]s/s2, target: /storage/s)
	                      }
	                    `,
						capabilityDomain.Identifier(),
					),
				)

				// save

				_, err := inter.Invoke("save")
				require.NoError(t, err)

				require.Len(t, storedValues, 1)

				t.Run("link S", func(t *testing.T) {

					// first link

					value, err := inter.Invoke("linkS")
					require.NoError(t, err)

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, interpreter.CapabilityValue{}, innerValue)

					// stored value + link
					require.Len(t, storedValues, 2)

					// second link

					value, err = inter.Invoke("linkS")
					require.NoError(t, err)

					require.IsType(t, interpreter.NilValue{}, value)

					// NOTE: check loaded value was *not* removed from storage
					require.Len(t, storedValues, 2)
				})

				t.Run("link S2", func(t *testing.T) {

					// first link

					value, err := inter.Invoke("linkS2")
					require.NoError(t, err)

					require.IsType(t, &interpreter.SomeValue{}, value)

					innerValue := value.(*interpreter.SomeValue).Value

					assert.IsType(t, interpreter.CapabilityValue{}, innerValue)

					// stored value + link
					require.Len(t, storedValues, 3)

					// second link

					value, err = inter.Invoke("linkS2")
					require.NoError(t, err)

					require.IsType(t, interpreter.NilValue{}, value)

					// NOTE: check loaded value was *not* removed from storage
					require.Len(t, storedValues, 3)
				})
			})
		}

		for _, targetDomain := range common.AllPathDomainsByIdentifier {

			testName := fmt.Sprintf(
				"invalid: new capability path: storage domain, target path: %s domain",
				targetDomain,
			)

			t.Run(testName, func(t *testing.T) {

				inter, _ := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      struct S {}

	                      fun test() {
	                          account.link<&S>(/storage/s, target: /%s/s)
	                      }
	                    `,
						targetDomain.Identifier(),
					),
				)

				_, err := inter.Invoke("test")

				require.Error(t, err)

				require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
			})
		}
	})

}

func TestInterpretAuthAccountUnlink(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		for _, capabilityDomain := range []common.PathDomain{
			common.PathDomainPrivate,
			common.PathDomainPublic,
		} {

			t.Run(capabilityDomain.Name(), func(t *testing.T) {

				inter, storedValues := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      resource R {}

	                      resource R2 {}

	                      fun saveAndLinkR() {
	                          let r <- create R()
	                          account.save(<-r, to: /storage/r)
	                          account.link<&R>(/%[1]s/r, target: /storage/r)
	                      }

	                      fun unlinkR() {
	                          account.unlink(/%[1]s/r)
	                      }

                          fun unlinkR2() {
	                          account.unlink(/%[1]s/r2)
	                      }
	                    `,
						capabilityDomain.Identifier(),
					),
				)

				// save and link

				_, err := inter.Invoke("saveAndLinkR")
				require.NoError(t, err)

				require.Len(t, storedValues, 2)

				t.Run("unlink R", func(t *testing.T) {
					_, err := inter.Invoke("unlinkR")
					require.NoError(t, err)

					require.Len(t, storedValues, 1)
				})

				t.Run("unlink R2", func(t *testing.T) {

					_, err := inter.Invoke("unlinkR2")
					require.NoError(t, err)

					require.Len(t, storedValues, 1)
				})
			})
		}

		t.Run("storage", func(t *testing.T) {

			inter, _ := testAccount(
				t,
				true,
				`
	              resource R {}

	              fun test() {
	                  account.unlink(/storage/r)
	              }
	            `,
			)

			_, err := inter.Invoke("test")

			require.Error(t, err)

			require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
		})
	})

	t.Run("struct", func(t *testing.T) {

		for _, capabilityDomain := range []common.PathDomain{
			common.PathDomainPrivate,
			common.PathDomainPublic,
		} {

			t.Run(capabilityDomain.Name(), func(t *testing.T) {

				inter, storedValues := testAccount(
					t,
					true,
					fmt.Sprintf(
						`
	                      struct S {}

	                      struct S2 {}

	                      fun saveAndLinkS() {
	                          let s = S()
	                          account.save(s, to: /storage/s)
	                          account.link<&S>(/%[1]s/s, target: /storage/s)
	                      }

	                      fun unlinkS() {
	                          account.unlink(/%[1]s/s)
	                      }

                          fun unlinkS2() {
	                          account.unlink(/%[1]s/s2)
	                      }
	                    `,
						capabilityDomain.Identifier(),
					),
				)

				// save and link

				_, err := inter.Invoke("saveAndLinkS")
				require.NoError(t, err)

				require.Len(t, storedValues, 2)

				t.Run("unlink S", func(t *testing.T) {
					_, err := inter.Invoke("unlinkS")
					require.NoError(t, err)

					require.Len(t, storedValues, 1)
				})

				t.Run("unlink S2", func(t *testing.T) {

					_, err := inter.Invoke("unlinkS2")
					require.NoError(t, err)

					require.Len(t, storedValues, 1)
				})
			})
		}

		t.Run("storage", func(t *testing.T) {

			inter, _ := testAccount(
				t,
				true,
				`
	              struct S {}

	              fun test() {
	                  account.unlink(/storage/s)
	              }
	            `,
			)

			_, err := inter.Invoke("test")

			require.Error(t, err)

			require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
		})
	})
}

func TestInterpretAccountGetLinkTarget(t *testing.T) {

	t.Parallel()

	for _, auth := range []bool{true, false} {

		t.Run(fmt.Sprintf("auth: %v", auth), func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				for _, capabilityDomain := range []common.PathDomain{
					common.PathDomainPrivate,
					common.PathDomainPublic,
				} {

					t.Run(capabilityDomain.Name(), func(t *testing.T) {

						inter, storedValues := testAccount(
							t,
							auth,
							fmt.Sprintf(
								`
	                              resource R {}

	                              fun link() {
	                                  authAccount.link<&R>(/%[1]s/r, target: /storage/r)
	                              }

	                              fun existing(): Path? {
	                                  return account.getLinkTarget(/%[1]s/r)
	                              }

                                  fun nonExisting(): Path? {
	                                  return account.getLinkTarget(/%[1]s/r2)
	                              }
	                            `,
								capabilityDomain.Identifier(),
							),
						)

						// link

						_, err := inter.Invoke("link")
						require.NoError(t, err)

						require.Len(t, storedValues, 1)

						t.Run("existing", func(t *testing.T) {
							value, err := inter.Invoke("existing")
							require.NoError(t, err)

							require.IsType(t, &interpreter.SomeValue{}, value)

							innerValue := value.(*interpreter.SomeValue).Value

							assert.Equal(t,
								interpreter.PathValue{
									Domain:     common.PathDomainStorage,
									Identifier: "r",
								},
								innerValue,
							)

							require.Len(t, storedValues, 1)
						})

						t.Run("nonExisting", func(t *testing.T) {

							value, err := inter.Invoke("nonExisting")
							require.NoError(t, err)

							require.Equal(t, interpreter.NilValue{}, value)

							require.Len(t, storedValues, 1)
						})
					})
				}

				t.Run("storage", func(t *testing.T) {

					inter, _ := testAccount(
						t,
						auth,
						`
	                      resource R {}

	                      fun test() {
	                          account.getLinkTarget(/storage/r)
	                      }
	                    `,
					)

					_, err := inter.Invoke("test")

					require.Error(t, err)

					require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
				})
			})

			t.Run("struct", func(t *testing.T) {

				for _, capabilityDomain := range []common.PathDomain{
					common.PathDomainPrivate,
					common.PathDomainPublic,
				} {

					t.Run(capabilityDomain.Name(), func(t *testing.T) {

						inter, storedValues := testAccount(
							t,
							auth,
							fmt.Sprintf(
								`
	                              struct S {}

	                              fun link() {
	                                  authAccount.link<&S>(/%[1]s/s, target: /storage/s)
	                              }

	                              fun existing(): Path? {
	                                  return account.getLinkTarget(/%[1]s/s)
	                              }

                                  fun nonExisting(): Path? {
	                                  return account.getLinkTarget(/%[1]s/s2)
	                              }
	                            `,
								capabilityDomain.Identifier(),
							),
						)

						// link

						_, err := inter.Invoke("link")
						require.NoError(t, err)

						require.Len(t, storedValues, 1)

						t.Run("existing", func(t *testing.T) {
							value, err := inter.Invoke("existing")
							require.NoError(t, err)

							require.IsType(t, &interpreter.SomeValue{}, value)

							innerValue := value.(*interpreter.SomeValue).Value

							assert.Equal(t,
								interpreter.PathValue{
									Domain:     common.PathDomainStorage,
									Identifier: "s",
								},
								innerValue,
							)

							require.Len(t, storedValues, 1)
						})

						t.Run("nonExisting", func(t *testing.T) {

							value, err := inter.Invoke("nonExisting")
							require.NoError(t, err)

							require.Equal(t, interpreter.NilValue{}, value)

							require.Len(t, storedValues, 1)
						})
					})
				}

				t.Run("storage", func(t *testing.T) {

					inter, _ := testAccount(
						t,
						auth,
						`
	                      struct S {}

	                      fun test() {
	                          account.getLinkTarget(/storage/s)
	                      }
	                    `,
					)

					_, err := inter.Invoke("test")

					require.Error(t, err)

					require.IsType(t, &interpreter.InvalidPathDomainError{}, err)
				})
			})
		})
	}
}

func TestInterpretAccountGetCapability(t *testing.T) {

	t.Parallel()

	tests := map[bool][]common.PathDomain{
		true: {
			common.PathDomainPublic,
			common.PathDomainPrivate,
		},
		false: {
			common.PathDomainPublic,
		},
	}

	for auth, validDomains := range tests {

		for _, domain := range common.AllPathDomainsByIdentifier {

			t.Run(fmt.Sprintf("auth: %v, domain: %s", auth, domain), func(t *testing.T) {

				inter, _ := testAccount(
					t,
					auth,
					fmt.Sprintf(
						`
	                      fun test(): Capability? {
	                          return account.getCapability(/%[1]s/r)
	                      }
	                    `,
						domain.Identifier(),
					),
				)

				value, err := inter.Invoke("test")

				require.NoError(t, err)

				isValid := false

				for _, validDomain := range validDomains {

					if domain == validDomain {

						isValid = true

						require.IsType(t, &interpreter.SomeValue{}, value)

						break
					}
				}

				if !isValid {
					require.IsType(t, interpreter.NilValue{}, value)
				}
			})
		}
	}
}

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

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

// TODO: test multiple initializers once overloading is supported

func TestCheckInvalidFieldInitializationEmptyInitializer(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var foo: Int
          var bar: Int

          init(foo: Int) {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.FieldUninitializedError{}, errs[0])
	assert.IsType(t, &sema.FieldUninitializedError{}, errs[1])
}

func TestCheckFieldInitializationFromArgument(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       struct Test {
           var foo: Int

           init(foo: Int) {
               self.foo = foo
           }
       }
   `)

	require.NoError(t, err)
}

func TestCheckFieldInitializationWithFunctionCallAfterAllFieldsInitialized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var foo: Int

          init() {
              self.foo = 1
              self.bar()
          }

          fun bar() {}
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidFieldInitializationWithFunctionCallBeforeAllFieldsInitialized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var foo: Int

          init() {
              self.bar()
              self.foo = 1
          }

          fun bar() {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UninitializedUseError{}, errs[0])
}

func TestCheckInvalidFieldInitializationWithUseBeforeAllFieldsInitialized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var foo: Int
          var bar: Int

          init() {
              self.foo = 1
              bar(self)
              self.bar = 2
          }
      }

      fun bar(_ test: Test) {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UninitializedUseError{}, errs[0])
}

func TestCheckConstantFieldInitialization(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          let foo: Int

          init() {
              self.foo = 1
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidRepeatedConstantFieldInitialization(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          let foo: Int

          init() {
              self.foo = 1
              self.foo = 1
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckFieldInitializationInIfStatement(t *testing.T) {

	t.Parallel()

	t.Run("ValidIfStatement", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init() {
                   if 1 > 2 {
                       self.foo = 1
                   } else {
                       self.foo = 2
                   }
               }
           }
       `)

		require.NoError(t, err)
	})

	t.Run("InvalidIfStatement", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init() {
                   if 1 > 2 {
                       self.foo = 1
                   }
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.FieldUninitializedError{}, errs[0])
	})
}

func TestCheckFieldInitializationInWhileStatement(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        struct Test {
            var foo: Int

            init() {
                while 1 < 2 {
                    self.foo = 1
                }
            }
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.FieldUninitializedError{}, errs[0])
}

func TestCheckFieldInitializationFromField(t *testing.T) {

	t.Parallel()

	t.Run("FromInitializedField", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int
               var bar: Int

               init() {
                   self.foo = 1
                   self.bar = self.foo + 1
               }
           }
       `)

		require.NoError(t, err)
	})

	t.Run("FromUninitializedField", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int
               var bar: Int

               init() {
                   self.bar = self.foo + 1
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
		assert.IsType(t, &sema.FieldUninitializedError{}, errs[1])
	})
}

func TestCheckFieldInitializationUsages(t *testing.T) {

	t.Parallel()

	t.Run("InitializedUsage", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           fun myFunc(_ x: Int) {}

           struct Test {
               var foo: Int

               init() {
                   self.foo = 1
                   myFunc(self.foo)
               }
           }
       `)

		require.NoError(t, err)

	})

	t.Run("UninitializedUsage", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           fun myFunc(_ x: Int) {}

           struct Test {
               var foo: Int

               init() {
                   myFunc(self.foo)
                   self.foo = 1
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
	})

	t.Run("IfStatementUsage", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init() {
                   if self.foo > 0 {

                   }

                   self.foo = 1
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
	})

	t.Run("ArrayLiteralUsage", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init() {
                   var x = [self.foo]

                   self.foo = 1
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
	})

	t.Run("BinaryOperationUsage", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init() {
                   var x = 4 + self.foo

                   self.foo = 1
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
	})

	t.Run("ComplexUsages", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var a: Int
               var b: Int
               var c: Int

               init(x: Int) {
                   self.a = x

                   if self.a < 4 {
                       self.b = self.a + 2
                   } else {
                       self.b = 0
                   }

                   if self.a + self.b < 5 {
                       self.c = self.a
                   } else {
                       self.c = self.b
                   }
               }
           }
       `)

		require.NoError(t, err)
	})
}

func TestCheckFieldInitializationWithReturn(t *testing.T) {

	t.Parallel()

	t.Run("Direct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init(foo: Int) {
                   return
                   self.foo = foo
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
		assert.IsType(t, &sema.FieldUninitializedError{}, errs[1])
	})

	t.Run("InsideIf", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init(foo: Int) {
                   if false {
                       return
                   }
                   self.foo = foo
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.FieldUninitializedError{}, errs[0])
	})

	t.Run("InsideWhile", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
           struct Test {
               var foo: Int

               init(foo: Int) {
                   while false {
                       return
                   }
                   self.foo = foo
               }
           }
       `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.FieldUninitializedError{}, errs[0])
	})
}

func TestCheckFieldInitializationWithPotentialNeverCallInElse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(
		t,
		`
          struct Test {
              let foo: Int

              init(foo: Int?) {
                  if let foo = foo {
                      self.foo = foo
                  } else {
                      panic("no x")
                  }
              }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckFieldInitializationWithPotentialNeverCallInNilCoalescing(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(t,
		`
          struct Test {
              let foo: Int

              init(foo: Int?) {
                  self.foo = foo ?? panic("no x")
              }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidFieldInitializationWithUseOfUninitializedInPrecondition(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var on: Bool

          init() {
              pre { self.on }
              self.on = true
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UninitializedFieldAccessError{}, errs[0])
}

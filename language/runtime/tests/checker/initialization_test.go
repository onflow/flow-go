package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

// TODO: test multiple initializers once overloading is supported

func TestCheckInvalidFieldInitializationEmptyInitializer(t *testing.T) {

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

	_, err := ParseAndCheck(t, `
       struct Test {
           var foo: Int

           init(foo: Int) {
               self.foo = foo
           }
       }
   `)

	assert.Nil(t, err)
}

func TestCheckFieldInitializationWithFunctionCallAfterAllFieldsInitialized(t *testing.T) {

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

	assert.Nil(t, err)
}

func TestCheckInvalidFieldInitializationWithFunctionCallBeforeAllFieldsInitialized(t *testing.T) {

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

	_, err := ParseAndCheck(t, `
      struct Test {
          let foo: Int

          init() {
              self.foo = 1
          }
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidRepeatedConstantFieldInitialization(t *testing.T) {

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

		assert.Nil(t, err)
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

		assert.Nil(t, err)
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

		assert.Nil(t, err)

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

		assert.Nil(t, err)
	})
}

func TestCheckFieldInitializationWithReturn(t *testing.T) {

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

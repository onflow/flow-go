package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckReferenceInFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          test
      }
	`)

	assert.Nil(t, err)
}

func TestCheckParameterNameWithFunctionName(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(test: Int) {
          test
      }
	`)

	assert.Nil(t, err)
}

func TestCheckMutuallyRecursiveFunctions(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun isEven(_ n: Int): Bool {
          if n == 0 {
              return true
          }
          return isOdd(n - 1)
      }

      fun isOdd(_ n: Int): Bool {
          if n == 0 {
              return false
          }
          return isEven(n - 1)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          fun foo() {}
          fun foo() {}
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckFunctionRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun foo() {
          fun foo() {}
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
       pub fun test() {}
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
       pub(set) fun test() {}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
}

func TestCheckReturnWithoutExpression(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun returnNothing() {
           return
       }
	`)

	assert.Nil(t, err)
}

func TestCheckAnyReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun foo(): Any {
          return foo
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidParameterTypes(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: X, y: Y) {}
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

	assert.IsType(t, &sema.NotDeclaredError{}, errs[1])

}

func TestCheckInvalidParameterNameRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(a: Int, a: Int) {}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckParameterRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(a: Int) {
          let a = 1
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidParameterAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(a: Int) {
          a = 1
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckInvalidArgumentLabelRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x a: Int, x b: Int) {}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckArgumentLabelRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(_ a: Int, _ b: Int) {}
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionDeclarationReturnValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return true
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidResourceCapturingThroughVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Kitty {}

      fun makeKittyCloner(): ((): <-Kitty) {
          let kitty <- create Kitty()
          return fun (): <-Kitty {
              return <-kitty
          }
      }

      let test = makeKittyCloner()
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceCapturingError{}, errs[1])
}

func TestCheckInvalidResourceCapturingThroughParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Kitty {}

      fun makeKittyCloner(kitty: <-Kitty): ((): <-Kitty) {
          return fun (): <-Kitty {
              return <-kitty
          }
      }

      let test = makeKittyCloner(kitty: <-create Kitty())
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceCapturingError{}, errs[1])
}

func TestCheckInvalidSelfResourceCapturing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Kitty {
          fun makeCloner(): ((): <-Kitty) {
              return fun (): <-Kitty {
                  return <-self
              }
          }
      }

      let kitty <- create Kitty()
      let test = kitty.makeCloner()
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceCapturingError{}, errs[0])

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
}

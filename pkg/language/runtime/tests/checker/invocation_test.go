package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionCallWithoutArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionCallWithNotRequiredArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[0])
}

func TestCheckIndirectFunctionCallWithoutArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          let g = f
          return g(1)
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionCallMissingArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
}

func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(y: 1)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return true()
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotCallableError{}, errs[0])
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return 2()
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotCallableError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: true)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithWrongTypeAndMissingArgumentLabel(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
}

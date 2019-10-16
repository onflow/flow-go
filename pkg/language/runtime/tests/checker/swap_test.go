package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidUnknownDeclarationSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          x <-> y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidLeftConstantSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = 2
          var y = 1
          x <-> y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckInvalidRightConstantSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          let y = 1
          x <-> y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          var y = 3
          x <-> y
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidTypesSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          var y = "1"
          x <-> y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidTypesSwap2(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = "2"
          var y = 1
          x <-> y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressionLeft(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          f() <-> x
      }

      fun f(): Int {
          return 2
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressionRight(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          x <-> f()
      }

      fun f(): Int {
          return 2
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressions(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          f() <-> f()
      }

      fun f(): Int {
          return 2
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[1])
}

func TestCheckSwapOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x: Int? = 2
          var y: Int? = nil
          x <-> y
      }
	`)

	assert.Nil(t, err)
}

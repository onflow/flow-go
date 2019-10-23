package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidUnknownDeclarationAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          x = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidConstantAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = 2
          x = 3
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          x = 3
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidGlobalConstantAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = 2

      fun test() {
          x = 3
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckGlobalVariableAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      var x = 2

      fun test(): Int {
          x = 3
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidAssignmentToParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int8) {
           x = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

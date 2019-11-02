package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckArrayIndexingWithInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [0, 3]
          z[0]
      }
    `)

	assert.Nil(t, err)
}

func TestCheckNestedArrayIndexingWithInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [[0, 1], [2, 3]]
          z[0][1]
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayIndexingWithBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [0, 3]
          z[true]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexingTypeError{}, errs[0])
}

func TestCheckInvalidArrayIndexingIntoBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return true[0]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
}

func TestCheckInvalidArrayIndexingIntoInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return 2[0]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
}

func TestCheckInvalidArrayIndexingAssignmentWithBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [0, 3]
          z[true] = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexingTypeError{}, errs[0])
}

func TestCheckArrayIndexingAssignmentWithInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [0, 3]
          z[0] = 2
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayIndexingAssignmentWithWrongType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = [0, 3]
          z[0] = true
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidStringIndexingWithBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = "abc"
          z[true]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexingTypeError{}, errs[0])
}

func TestCheckInvalidUnknownDeclarationIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          x[0]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          x[0] = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

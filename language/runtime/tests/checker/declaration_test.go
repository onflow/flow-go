package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckConstantAndVariableDeclarations(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x = 1
        var y = 1
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.IntType{},
		checker.GlobalValues["x"].Type,
	)

	assert.Equal(t,
		&sema.IntType{},
		checker.GlobalValues["y"].Type,
	)
}

func TestCheckInvalidGlobalConstantRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun x() {}

        let y = true
        let y = false
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckInvalidGlobalFunctionRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
        let x = true

        fun y() {}
        fun y() {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckInvalidLocalRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test() {
            let x = true
            let x = false
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckInvalidLocalFunctionRedeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test() {
            let x = true

            fun y() {}
            fun y() {}
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckInvalidUnknownDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun test() {
           return x
       }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

	assert.IsType(t, &sema.InvalidReturnValueError{}, errs[1])
}

func TestCheckInvalidUnknownDeclarationInGlobal(t *testing.T) {

	_, err := ParseAndCheck(t, `
       let x = y
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidUnknownDeclarationInGlobalAndUnknownType(t *testing.T) {

	_, err := ParseAndCheck(t, `
       let x: X = y
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.Equal(t,
		"y",
		errs[0].(*sema.NotDeclaredError).Name,
	)
	assert.Equal(t,
		common.DeclarationKindVariable,
		errs[0].(*sema.NotDeclaredError).ExpectedKind,
	)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	assert.Equal(t,
		"X",
		errs[1].(*sema.NotDeclaredError).Name,
	)
	assert.Equal(t,
		common.DeclarationKindType,
		errs[1].(*sema.NotDeclaredError).ExpectedKind,
	)
}

func TestCheckInvalidUnknownDeclarationCallInGlobal(t *testing.T) {

	_, err := ParseAndCheck(t, `
       let x = y()
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidRedeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(a: Int, a: Int) {
        let x = 1
        let x = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])

	assert.IsType(t, &sema.RedeclarationError{}, errs[1])
}

func TestCheckInvalidConstantValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Bool = 1
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidReference(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          testX
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x = 1
        var y = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.IntType{}))

	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.IntType{}))
}

func TestCheckInvalidGlobalConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun x() {}

        let y = true
        let y = false
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidGlobalFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        let x = true

        fun y() {}
        fun y() {}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidLocalRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test() {
            let x = true
            let x = false
        }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidLocalFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test() {
            let x = true

            fun y() {}
            fun y() {}
        }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun test() {
           return x
       }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidReturnValueError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobal(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       let x = y
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobalAndUnknownType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       let x: X = y
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("y"))
	Expect(errs[0].(*sema.NotDeclaredError).ExpectedKind).
		To(Equal(common.DeclarationKindValue))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[1].(*sema.NotDeclaredError).Name).
		To(Equal("X"))
	Expect(errs[1].(*sema.NotDeclaredError).ExpectedKind).
		To(Equal(common.DeclarationKindType))
}

func TestCheckInvalidUnknownDeclarationCallInGlobal(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       let x = y()
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidRedeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(a: Int, a: Int) {
        let x = 1
        let x = 2
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidConstantValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Bool = 1
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          testX
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

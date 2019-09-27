package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          x = 2
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let x = 2
          x = 3
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          var x = 2
          x = 3
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x = 2

      fun test() {
          x = 3
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      var x = 2

      fun test(): Int {
          x = 3
          return x
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int8) {
           x = 2
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

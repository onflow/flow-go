package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [[0, 1], [2, 3]]
          z[0][1]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true]
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          return true[0]
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          return 2[0]
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true] = 2
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckArrayIndexingAssignmentWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0] = 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingAssignmentWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0] = true
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidStringIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = "abc"
          z[true]
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          x[0]
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          x[0] = 2
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

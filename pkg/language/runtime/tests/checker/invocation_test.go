package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionCallWithNotRequiredArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckIndirectFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          let g = f
          return g(1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionCallMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(y: 1)
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          return true()
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          return 2()
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: true)
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionCallWithWrongTypeAndMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

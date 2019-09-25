package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckParameterNameWithFunctionName(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(test: Int) {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckMutuallyRecursiveFunctions(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
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

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          fun foo() {}
          fun foo() {}
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun foo() {
          fun foo() {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       pub fun test() {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       pub(set) fun test() {}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidAccessModifierError{}))
}

func TestCheckReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun returnNothing() {
           return
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAnyReturnType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun foo(): Any {
          return foo
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidParameterTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: X, y: Y) {}
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

}

func TestCheckInvalidParameterNameRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(a: Int, a: Int) {}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckParameterRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(a: Int) {
          let a = 1
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x a: Int, x b: Int) {}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(_ a: Int, _ b: Int) {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionDeclarationReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          return true
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckFunctionConditions(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          pre {
              x != 0
          }
          post {
              x == 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPreConditionReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          pre {
              y == 0
          }
          post {
              z == 0
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("y"))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[1].(*sema.NotDeclaredError).Name).
		To(Equal("z"))
}

func TestCheckInvalidFunctionNonBoolCondition(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          pre {
              1
          }
          post {
              2
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          post {
              before(x) != 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithBeforeAndNoArgument(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          post {
              before() != 0
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))

}

func TestCheckInvalidFunctionPreConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          pre {
              before(x) != 0
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("before"))
}

func TestCheckInvalidFunctionWithBeforeVariableAndPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          post {
              before(x) == 0
          }
          let before = 0
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionWithBeforeVariable(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int) {
          let before = 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostCondition(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPreConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          pre {
              result == 0
          }
          return 0
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckInvalidFunctionPostConditionWithResultWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          post {
              result == true
          }
          return 0
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))
}

func TestCheckFunctionPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          post {
              result == 0
          }
          return 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          post {
              result == 0
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckFunctionWithoutReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionWithoutReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(result: Int) {
          post {
              result == 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionWithReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {
          post {
              result == 2
          }
          let result = 1
          return result * 2
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

// TODO: should this be invalid?
func TestCheckFunctionWithReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(result: Int): Int {
          post {
              result == 2
          }
          return result * 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          post {
              (fun (): Int { return 2 })() == 2
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.FunctionExpressionInConditionError{}))
}

func TestCheckFunctionPostConditionWithMessageUsingStringLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          post {
             1 == 2: "nope"
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithMessageUsingBooleanLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          post {
             1 == 2: true
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithMessageUsingResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): String {
          post {
             1 == 2: result
          }
          return ""
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostConditionWithMessageUsingBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: String) {
          post {
             1 == 2: before(x)
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostConditionWithMessageUsingParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: String) {
          post {
             1 == 2: x
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

package runtime

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

func parseAndCheck(code string) (*sema.Checker, error) {
	program, errors := parser.ParseProgram(code)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()
	return checker, err
}

func TestCheckConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = 1
        var y = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.IntType{}))

	Expect(checker.Globals["y"].Type).
		To(Equal(&sema.IntType{}))
}

func TestCheckBoolean(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.BoolType{}))
}

func TestCheckString(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = "x"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.StringType{}))
}

func expectCheckerErrors(err error, len int) []error {
	if len <= 0 {
		return nil
	}

	Expect(err).To(HaveOccurred())

	Expect(err).
		To(BeAssignableToTypeOf(&sema.CheckerError{}))

	errs := err.(*sema.CheckerError).Errors

	Expect(errs).To(HaveLen(len))

	return errs
}

func TestCheckInvalidGlobalConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun x() {}

        let y = true
        let y = false
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidGlobalFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        let x = true

        fun y() {}
        fun y() {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidLocalRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test() {
            let x = true
            let x = false
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           return x
       }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidReturnValueError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobal(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x = y
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobalAndUnknownType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x: X = y
	`)

	errs := expectCheckerErrors(err, 2)

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

	_, err := parseAndCheck(`
       let x = y()
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = 2
          x = 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      let x = 2

      fun test() {
          x = 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(x: Int8) {
           x = 2
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayElements(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, true]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNestedArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return 2[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true] = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckArrayIndexingAssignmentWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0] = true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x[0] = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidParameterTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: X, y: Y) {}
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

}

func TestCheckInvalidParameterNameRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int, a: Int) {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckParameterRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int) {
          let a = 1
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidRedeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int, a: Int) {
        let x = 1
        let x = 2
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x a: Int, x b: Int) {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(_ a: Int, _ b: Int) {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConstantValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Bool = 1
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionDeclarationReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionExpressionReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let test = fun (): Int {
          return true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          testX
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckParameterNameWithFunctionName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(test: Int) {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if 1 {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if true {} else {
              x
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 1 : 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = 1 ? 2 : 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidConditionalExpressionElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 2 : y
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidConditionalExpressionTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 2 : false
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while 1 {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidWhileBlock(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while true { x }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckIndirectFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(y: 1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return 2()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: true)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionCallWithWrongTypeAndMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = !1
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = !true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = -true
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = -1
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

type operationTest struct {
	ty          sema.Type
	left, right string
	matchers    []types.GomegaMatcher
}

type operationTests struct {
	operations []ast.Operation
	tests      []operationTest
}

func TestCheckIntegerBinaryOperations(t *testing.T) {
	RegisterTestingT(t)

	allOperationTests := []operationTests{
		{
			operations: []ast.Operation{
				ast.OperationPlus, ast.OperationMinus, ast.OperationMod, ast.OperationMul, ast.OperationDiv,
			},
			tests: []operationTest{
				{&sema.IntType{}, "1", "2", nil},
				{&sema.IntType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
					BeAssignableToTypeOf(&sema.TypeMismatchError{}),
				}},
				{&sema.IntType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.IntType{}, "true", "false", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
					BeAssignableToTypeOf(&sema.TypeMismatchError{}),
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationLess, ast.OperationLessEqual, ast.OperationGreater, ast.OperationGreaterEqual,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "1", "2", nil},
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "true", "false", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationOr, ast.OperationAnd,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "true", "false", nil},
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
				}},
				{&sema.BoolType{}, "1", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationEqual, ast.OperationUnequal,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "true", "false", nil},
				{&sema.BoolType{}, "1", "2", nil},
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
			},
		},
	}

	for _, operationTests := range allOperationTests {
		for _, operation := range operationTests.operations {
			for _, test := range operationTests.tests {
				_, err := parseAndCheck(
					fmt.Sprintf(
						`fun test(): %s { return %s %s %s }`,
						test.ty, test.left, operation.Symbol(), test.right,
					),
				)

				errs := expectCheckerErrors(err, len(test.matchers))

				for i, matcher := range test.matchers {
					Expect(errs[i]).
						To(matcher)
				}
			}
		}
	}
}

func TestCheckFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun returnNothing() {
           return
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAnyReturnType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo(): Any {
          return foo
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a: Any = 1
      let b: Any = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               break
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               fun () {
                   break
               }
           }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}

func TestCheckContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               continue
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               fun () {
                   continue
               }
           }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}

func TestCheckInvalidFunctionDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          fun foo() {}
          fun foo() {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo() {
          fun foo() {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       pub fun test() {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       pub(set) fun test() {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidAccessModifierError{}))
}

func TestCheckInvalidStructureRedeclaringType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Int {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckStructure(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Test {
            pub(set) var foo: Int

            init(foo: Int) {
                self.foo = foo
            }

            pub fun getFoo(): Int {
                return self.foo
            }
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInitializerName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Test {
            init() {}
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidInitializerName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Test {
            initializer() {}
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidInitializerNameError{}))
}

func TestCheckInvalidStructureFieldName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Test {
            let init: Int
        }
	`)

	errs := expectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidNameError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))
}

func TestCheckInvalidStructureFunctionName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        struct Test {
            fun init() {}
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidNameError{}))
}

func TestCheckInvalidStructureRedeclaringFields(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           let x: Int
           let x: Int
       }
	`)

	errs := expectCheckerErrors(err, 4)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

	Expect(errs[3]).
		To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))
}

func TestCheckInvalidStructureRedeclaringFunctions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           fun x() {}
           fun x() {}
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidStructureRedeclaringFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           let x: Int
           fun x() {}
       }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))
}

func TestCheckStructureFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           let x: Int

           init() {
               self.x = 1
           }

           fun y() {}
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureFieldType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           let x: X
       }
	`)

	errs := expectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))
}

func TestCheckInvalidStructureInitializerParameterType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           init(x: X) {}
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidStructureInitializerParameters(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           init(x: Int, x: Int) {}
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidStructureInitializer(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           init() { X }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidStructureFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       struct Test {
           fun test() { X }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckStructureInitializerSelfReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          init() { self }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStructureFunctionSelfReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          fun test() { self }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidLocalStructure(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
	  fun test() {
          struct Test {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidDeclarationError{}))
}

func TestCheckInvalidStructureMissingInitializer(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          let foo: Int
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))
}

func TestCheckStructureFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          let foo: Int

          init() {
              self.foo = 1
          }

          fun test() {
              self.foo
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          init() {
              self.foo
          }

          fun test() {
              self.bar
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
	Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
		To(Equal("foo"))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
	Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
		To(Equal("bar"))
}

func TestCheckStructureFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          var foo: Int

          init() {
              self.foo = 1
              let alsoSelf = self
              alsoSelf.foo = 2
          }

          fun test() {
              self.foo = 3
              let alsoSelf = self
              alsoSelf.foo = 4
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureSelfAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          init() {
              self = Test()
          }

          fun test() {
              self = Test()
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckInvalidStructureFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          init() {
              self.foo = 1
          }

          fun test() {
              self.bar = 2
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
	Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
		To(Equal("foo"))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
	Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
		To(Equal("bar"))
}

func TestCheckInvalidStructureFieldAssignmentWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          var foo: Int

          init() {
              self.foo = true
          }

          fun test() {
              self.foo = false
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidStructureFieldConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          let foo: Int

          init() {
              // initialization is fine
              self.foo = 1
          }

          fun test() {
              // assignment is invalid
              self.foo = 2
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))
}

func TestCheckStructureFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          fun foo() {}

          fun bar() {
              self.foo()
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          fun foo() {}

          fun bar() {
              self.baz()
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
}

func TestCheckInvalidStructureFunctionAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          fun foo() {}

          fun bar() {
              self.foo = 2
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))
	Expect(errs[0].(*sema.AssignmentToConstantMemberError).Name).
		To(Equal("foo"))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckStructureInstantiation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {

          init(x: Int) {
              let test: Test = Test(x: 1)
          }

          fun test() {
              let test: Test = Test(x: 2)
          }
      }

      let test: Test = Test(x: 3)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = 1
      struct Foo {}
      struct Foo {}
	`)

	errs := expectCheckerErrors(err, 2)

	// NOTE: two errors: one because type is redeclared,
	// the other because the global is redeclared

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

}

func TestCheckInvalidIncompatibleStructureTypes(t *testing.T) {
	// tests that structure typing is nominal, not structural

	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Foo {
          init() {}
      }

      struct Bar {
          init() {}
      }

      let foo: Foo = Bar()
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidStructureFunctionWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Foo {
          fun test(self: Int) {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidStructureInitializerWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Foo {
          init(self: Int) {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckStructureInitializesConstant(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {
          let foo: Int

          init() {
              self.foo = 42
          }
      }

	  let test = Test()
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStructureInitializerWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {

          init(x: Int) {}
      }

	  let test = Test(x: 1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureInitializerCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {

          init(x: Int) {}
      }

	  let test = Test(1)
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckStructureFunctionWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct Test {

          fun test(x: Int) {}
      }

	  let test = Test().test(x: 1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStructureFunctionCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     struct Test {

         fun test(x: Int) {}
     }

	  let test = Test().test(1)
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckStructureConstructorReferenceInInitializerAndFunction(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`

      struct Test {

          init() {
              Test
          }

          fun test(): Test {
              return Test()
          }
      }

      fun test(): Test {
         return Test()
      }

      fun test2(): Test {
         return Test().test()
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	testType := checker.FindType("Test")

	Expect(testType).
		To(BeAssignableToTypeOf(&sema.StructureType{}))

	structureType := testType.(*sema.StructureType)

	Expect(structureType.Identifier).
		To(Equal("Test"))

	testFunctionMember := structureType.Members["test"]

	Expect(testFunctionMember.Type).
		To(BeAssignableToTypeOf(&sema.FunctionType{}))

	testFunctionType := testFunctionMember.Type.(*sema.FunctionType)

	Expect(testFunctionType.ReturnType).
		To(BeIdenticalTo(structureType))
}

func TestCheckFunctionConditions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              y == 0
          }
          post {
              z == 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

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

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              1
          }
          post {
              2
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(x: Int) {
          post {
              before() != 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))

}

func TestCheckInvalidFunctionPreConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              before(x) != 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("before"))
}

func TestCheckInvalidFunctionWithBeforeVariableAndPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          post {
              before(x) == 0
          }
          let before = 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionWithBeforeVariable(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          let before = 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostCondition(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(): Int {
          pre {
              result == 0
          }
          return 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckInvalidFunctionPostConditionWithResultWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              result == true
          }
          return 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))
}

func TestCheckFunctionPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test() {
          post {
              result == 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckFunctionWithoutReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              result == 2
          }
          let result = 1
          return result * 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

// TODO: should this be invalid?
func TestCheckFunctionWithReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              (fun (): Int { return 2 })() == 2
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.FunctionExpressionInConditionError{}))
}

func TestCheckFunctionPostConditionWithStringMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
             1 == 2: "nope"
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithNonStringMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
             1 == 2: true
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithMessageWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

func TestCheckFunctionPostConditionWithMessageWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
      fun test(x: String) {
          post {
             1 == 2: x
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

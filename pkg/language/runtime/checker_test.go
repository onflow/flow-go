package runtime

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"testing"
)

func TestCheckConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = 1
        var y = 1
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.IntType{}))

	Expect(checker.Globals["y"].Type).
		To(Equal(&sema.IntType{}))
}

func TestCheckBoolean(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = true
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.BoolType{}))
}

func TestCheckInvalidGlobalRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = true
        let x = false
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidVariableRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        fun test() {
            let x = true
            let x = false
        }
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          x = 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let x = 2
          x = 3
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          var x = 2
          x = 3
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let x = 2

      fun test() {
          x = 3
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      var x = 2

      fun test(): Int {
          x = 3
          return x
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(x: Int8) {
           x = 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [0, 3]
          z[0]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [[0, 1], [2, 3]]
          z[0][1]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [0, 3]
          z[true]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(): Int64 {
          return true[0]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(): Int64 {
          return 2[0]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [0, 3]
          z[true] = 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckArrayIndexingAssignmentWithInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [0, 3]
          z[0] = 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingAssignmentWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let z = [0, 3]
          z[0] = true
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          x[0]
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          x[0] = 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidParameterNameRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(a: Int, a: Int) {}
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(x a: Int, x b: Int) {}
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(_ a: Int, _ b: Int) {}
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConstantValue(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let x: Bool = 1
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionDeclarationReturnValue(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(): Int {
          return true
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionExpressionReturnValue(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let test = fun (): Int {
          return true
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidReference(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          testX
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          test
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckParameterNameWithFunctionName(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(test: Int) {
          test
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          if true {}
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          if 1 {}
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementElse(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          if true {} else {
              x
          }
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let x = true ? 1 : 2
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let x = 1 ? 2 : 3
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidConditionalExpressionElse(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let x = true ? 2 : y
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidConditionalExpressionTypes(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          let x = true ? 2 : false
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidWhileTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          while 1 {}
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckWhileTest(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          while true {}
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidWhileBlock(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test() {
          while true { x }
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

// TODO:
//func TestCheckFunctionCallMissingArgumentLabel(t *testing.T) {
//	RegisterTestingT(t)
//
//	program, errors := parser.Parse(`
//      fun f(x: Int): Int {
//          return x
//      }
//
//      fun test(): Int {
//          return f(1)
//      }
//	`)
//
//	Expect(errors).
//		To(BeEmpty())
//
//	checker := sema.NewChecker(program)
//	err := checker.Check()
//
//	Expect(err).
//		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
//}
//
//func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {
//	RegisterTestingT(t)
//
//	program, errors := parser.Parse(`
//      fun f(x: Int): Int {
//          return x
//      }
//
//      fun test(): Int {
//          return f(y: 1)
//      }
//	`)
//
//	Expect(errors).
//		To(BeEmpty())
//
//	checker := sema.NewChecker(program)
//	err := checker.Check()
//
//	Expect(err).
//		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
//}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(): Int {
          return true()
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun test(): Int32 {
          return 2()
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = !1
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = !true
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = -true
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = -1
	`)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))
}

type operationTest struct {
	left, right string
	matcher     types.GomegaMatcher
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
				ast.OperationLess, ast.OperationLessEqual, ast.OperationGreater, ast.OperationGreaterEqual,
			},
			tests: []operationTest{
				{"1", "2", Not(HaveOccurred())},
				{"true", "2", BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{})},
				{"1", "true", BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{})},
				{"true", "false", BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{})},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationOr, ast.OperationAnd,
			},
			tests: []operationTest{
				{"true", "false", Not(HaveOccurred())},
				{"true", "2", BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{})},
				{"1", "true", BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{})},
				{"1", "2", BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{})},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationEqual, ast.OperationUnequal,
			},
			tests: []operationTest{
				{"true", "false", Not(HaveOccurred())},
				{"1", "2", Not(HaveOccurred())},
				{"true", "2", BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{})},
				{"1", "true", BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{})},
			},
		},
	}

	for _, operationTests := range allOperationTests {
		for _, operation := range operationTests.operations {
			for _, test := range operationTests.tests {
				code := fmt.Sprintf(`let a = %s %s %s`, test.left, operation.Symbol(), test.right)
				program, errors := parser.Parse(code)

				Expect(errors).
					To(BeEmpty())

				checker := sema.NewChecker(program)
				err := checker.Check()

				Expect(err).
					To(test.matcher)
			}
		}
	}
}

package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"testing"
)

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let a = !1
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let a = !true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let a = -true
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
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
				{&sema.BoolType{}, `"test"`, `"test"`, nil},
			},
		},
	}

	for _, operationTests := range allOperationTests {
		for _, operation := range operationTests.operations {
			for _, test := range operationTests.tests {
				t.Run("", func(t *testing.T) {
					RegisterTestingT(t)

					_, err := ParseAndCheck(
						fmt.Sprintf(
							`fun test(): %s { return %s %s %s }`,
							test.ty, test.left, operation.Symbol(), test.right,
						),
					)

					errs := ExpectCheckerErrors(err, len(test.matchers))

					for i, matcher := range test.matchers {
						Expect(errs[i]).
							To(matcher)
					}
				})
			}
		}
	}
}

func TestCheckConcatenatingExpression(t *testing.T) {
	tests := []operationTest{
		{&sema.StringType{}, `"abc"`, `"def"`, nil},
		{&sema.StringType{}, `""`, `"def"`, nil},
		{&sema.StringType{}, `"abc"`, `""`, nil},
		{&sema.StringType{}, `""`, `""`, nil},
		{&sema.StringType{}, "1", `"def"`, []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},
		{&sema.StringType{}, `"abc"`, "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
		}},
		{&sema.StringType{}, "1", "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},

		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[3, 4]", nil},
		// TODO: support empty arrays
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[3, 4]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[]", nil},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "1", "[3, 4]", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
		}},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(
				fmt.Sprintf(
					`fun test(): %s { return %s %s %s }`,
					test.ty, test.left, ast.OperationConcat.Symbol(), test.right,
				),
			)

			errs := ExpectCheckerErrors(err, len(test.matchers))

			for i, matcher := range test.matchers {
				Expect(errs[i]).To(matcher)
			}
		})
	}
}

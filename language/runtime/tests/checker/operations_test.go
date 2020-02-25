package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a = !1
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
}

func TestCheckUnaryBooleanNegation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a = !true
	`)

	require.NoError(t, err)
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a = -true
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
}

func TestCheckUnaryIntegerNegation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a = -1
	`)

	require.NoError(t, err)
}

type operationTest struct {
	ty             sema.Type
	left, right    string
	expectedErrors []error
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
				{&sema.Fix64Type{}, "1.2", "3.4", nil},
				{&sema.Fix64Type{}, "1.2", "3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "1", "2.3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.Int64Type{}, "true", "1.2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.IntType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.Fix64Type{}, "1.2", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "true", "false", []error{
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationLess, ast.OperationLessEqual, ast.OperationGreater, ast.OperationGreaterEqual,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "1", "2", nil},
				{&sema.BoolType{}, "1.2", "3.4", nil},
				{&sema.BoolType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1.2", "3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1", "2.3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "true", "1.2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1.2", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "true", "false", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationOr, ast.OperationAnd,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "true", "false", nil},
				{&sema.BoolType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
				}},
				{&sema.BoolType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandError{},
				}},
				{&sema.BoolType{}, "1", "2", []error{
					&sema.InvalidBinaryOperandsError{},
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
				{&sema.BoolType{}, "1.2", "3.4", nil},
				{&sema.BoolType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1.2", "3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1", "2.3", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, `"test"`, `"test"`, nil},
			},
		},
	}

	for _, operationTests := range allOperationTests {
		for _, operation := range operationTests.operations {
			for _, test := range operationTests.tests {

				testName := fmt.Sprintf(
					"%s / %s %s %s",
					test.ty, test.left, operation.Symbol(), test.right,
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`fun test(): %s { return %s %s %s }`,
							test.ty, test.left, operation.Symbol(), test.right,
						),
					)

					errs := ExpectCheckerErrors(t, err, len(test.expectedErrors))

					for i, expectedErr := range test.expectedErrors {
						assert.IsType(t, expectedErr, errs[i])
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
		{&sema.StringType{}, "1", `"def"`, []error{
			&sema.InvalidBinaryOperandError{},
			&sema.InvalidBinaryOperandsError{},
			&sema.TypeMismatchError{},
		}},
		{&sema.StringType{}, `"abc"`, "2", []error{
			&sema.InvalidBinaryOperandError{},
			&sema.InvalidBinaryOperandsError{},
		}},
		{&sema.StringType{}, "1", "2", []error{
			&sema.InvalidBinaryOperandsError{},
			&sema.TypeMismatchError{},
		}},

		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[3, 4]", nil},
		// TODO: support empty arrays
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[3, 4]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[]", nil},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "1", "[3, 4]", []error{
			&sema.InvalidBinaryOperandError{},
			&sema.InvalidBinaryOperandsError{},
			&sema.TypeMismatchError{},
		}},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "2", []error{
			&sema.InvalidBinaryOperandError{},
			&sema.InvalidBinaryOperandsError{},
		}},
	}

	for _, test := range tests {

		testName := fmt.Sprintf(
			"%s / %s %s %s",
			test.ty, test.left, ast.OperationConcat.Symbol(), test.right,
		)

		t.Run(testName, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`fun test(): %s { return %s %s %s }`,
					test.ty, test.left, ast.OperationConcat.Symbol(), test.right,
				),
			)

			errs := ExpectCheckerErrors(t, err, len(test.expectedErrors))

			for i, expectedErr := range test.expectedErrors {
				assert.IsType(t, expectedErr, errs[i])
			}
		})
	}
}

func TestCheckInvalidCompositeEquality(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindEvent {
			continue
		}

		t.Run(compositeKind.Name(), func(t *testing.T) {

			var preparationCode, firstIdentifier, secondIdentifier string
			if compositeKind == common.CompositeKindContract {
				firstIdentifier = "X"
				secondIdentifier = "X"
			} else {
				preparationCode = fmt.Sprintf(`
		              let x1: %[1]sX %[2]s %[3]s X%[4]s
                      let x2: %[1]sX %[2]s %[3]s X%[4]s
		            `,
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				)
				firstIdentifier = "x1"
				secondIdentifier = "x2"
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s X {}

                      %[2]s

                      let a = %[3]s == %[4]s
                    `,
					compositeKind.Keyword(),
					preparationCode,
					firstIdentifier,
					secondIdentifier,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
		})
	}
}

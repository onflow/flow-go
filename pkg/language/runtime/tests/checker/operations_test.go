package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
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

	assert.Nil(t, err)
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

	assert.Nil(t, err)
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
				{&sema.IntType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.IntType{}, "1", "true", []error{
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
				{&sema.BoolType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.BoolType{}, "1", "true", []error{
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
				{&sema.BoolType{}, "true", "2", []error{
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
				t.Run("", func(t *testing.T) {

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
		t.Run("", func(t *testing.T) {

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

/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let a = !1
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
}

func TestCheckUnaryBooleanNegation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let a = !true
	`)

	require.NoError(t, err)
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let a = -true
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
}

func TestCheckUnaryIntegerNegation(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	allOperationTests := []operationTests{
		{
			operations: []ast.Operation{
				ast.OperationPlus,
				ast.OperationMinus,
				ast.OperationMod,
				ast.OperationMul,
				ast.OperationDiv,
			},
			tests: []operationTest{
				{&sema.IntType{}, "1", "2", nil},
				{&sema.UFix64Type{}, "1.2", "3.4", nil},
				{&sema.Fix64Type{}, "-1.2", "-3.4", nil},
				{&sema.UFix64Type{}, "1.2", "3", []error{
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
				{&sema.Fix64Type{}, "true", "1.2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.IntType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.UFix64Type{}, "1.2", "true", []error{
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
				ast.OperationLess,
				ast.OperationLessEqual,
				ast.OperationGreater,
				ast.OperationGreaterEqual,
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
				ast.OperationOr,
				ast.OperationAnd,
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
				ast.OperationEqual,
				ast.OperationNotEqual,
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
		{
			operations: []ast.Operation{
				ast.OperationBitwiseOr,
				ast.OperationBitwiseXor,
				ast.OperationBitwiseAnd,
				ast.OperationBitwiseLeftShift,
				ast.OperationBitwiseRightShift,
			},
			tests: []operationTest{
				{&sema.IntType{}, "1", "2", nil},
				{&sema.UFix64Type{}, "1.2", "3.4", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.UFix64Type{}, "1.2", "3", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "1", "2.3", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "true", "2", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.UFix64Type{}, "true", "1.2", []error{
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
				{&sema.IntType{}, "1", "true", []error{
					&sema.InvalidBinaryOperandError{},
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.UFix64Type{}, "1.2", "true", []error{
					&sema.InvalidBinaryOperandsError{},
				}},
				{&sema.IntType{}, "true", "false", []error{
					&sema.InvalidBinaryOperandsError{},
					&sema.TypeMismatchError{},
				}},
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

func TestCheckInvalidCompositeEquality(t *testing.T) {

	t.Parallel()

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

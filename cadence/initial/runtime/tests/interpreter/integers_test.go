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

package interpreter_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

var testIntegerTypesAndValues = map[string]interpreter.Value{
	// Int*
	"Int":    interpreter.NewIntValueFromInt64(50),
	"Int8":   interpreter.Int8Value(50),
	"Int16":  interpreter.Int16Value(50),
	"Int32":  interpreter.Int32Value(50),
	"Int64":  interpreter.Int64Value(50),
	"Int128": interpreter.NewInt128ValueFromInt64(50),
	"Int256": interpreter.NewInt256ValueFromInt64(50),
	// UInt*
	"UInt":    interpreter.NewUIntValueFromUint64(50),
	"UInt8":   interpreter.UInt8Value(50),
	"UInt16":  interpreter.UInt16Value(50),
	"UInt32":  interpreter.UInt32Value(50),
	"UInt64":  interpreter.UInt64Value(50),
	"UInt128": interpreter.NewUInt128ValueFromUint64(50),
	"UInt256": interpreter.NewUInt256ValueFromUint64(50),
	// Word*
	"Word8":  interpreter.Word8Value(50),
	"Word16": interpreter.Word16Value(50),
	"Word32": interpreter.Word32Value(50),
	"Word64": interpreter.Word64Value(50),
}

func init() {
	for _, integerType := range sema.AllIntegerTypes {
		if _, ok := testIntegerTypesAndValues[integerType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", integerType))
		}
	}
}

func TestInterpretIntegerConversions(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x: %[1]s = 50
                      let y = %[1]s(40) + %[1]s(10)
                      let z = y == x
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["x"].Value,
			)

			assert.Equal(t,
				value,
				inter.Globals["y"].Value,
			)

			assert.Equal(t,
				interpreter.BoolValue(true),
				inter.Globals["z"].Value,
			)

		})
	}
}

func TestInterpretAddressConversion(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: Address = 0x1
      let y = Address(0x2)
    `)

	assert.Equal(t,
		interpreter.AddressValue{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
		},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.AddressValue{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2,
		},
		inter.Globals["y"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x: %s = 50
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["x"].Value,
			)

		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                        let x: %s? = 50
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(value),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInAssignment(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      var x: %s = 50
                      fun test() {
                          x = x + x
                      }
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["x"].Value,
			)

			_, err := inter.Invoke("test")
			require.NoError(t, err)

			numberValue := value.(interpreter.NumberValue)
			assert.Equal(t,
				numberValue.Plus(numberValue),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInAssignmentOptional(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      var x: %s? = 50
                      fun test() {
                          x = 100
                      }
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(value),
				inter.Globals["x"].Value,
			)

			_, err := inter.Invoke("test")
			require.NoError(t, err)

			numberValue := value.(interpreter.NumberValue)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(
					numberValue.Plus(numberValue),
				),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      fun test(_ x: %[1]s): %[1]s {
                          return x
                      }
                      let x = test(50)
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                        fun test(_ x: %[1]s?): %[1]s? {
                            return x
                        }
                        let x = test(50)
                    `,
					integerType,
				),
			)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(value),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInReturn(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      fun test(): %s {
                          return 50
                      }
                    `,
					integerType,
				),
			)

			result, err := inter.Invoke("test")
			require.NoError(t, err)

			assert.Equal(t,
				value,
				result,
			)
		})
	}
}

func TestInterpretIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {

	t.Parallel()

	for integerType, value := range testIntegerTypesAndValues {

		t.Run(integerType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      fun test(): %s? {
                          return 50
                      }
                    `,
					integerType,
				),
			)

			result, err := inter.Invoke("test")
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(value),
				result,
			)
		})
	}
}

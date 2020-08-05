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
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

func TestInterpretNegativeZeroFixedPoint(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = -0.42
    `)

	assert.Equal(t,
		interpreter.Fix64Value(-42000000),
		inter.Globals["x"].Value,
	)
}

func TestInterpretFixedPointConversionAndAddition(t *testing.T) {

	t.Parallel()

	tests := map[string]interpreter.Value{
		// Fix*
		"Fix64": interpreter.Fix64Value(123000000),
		// UFix*
		"UFix64": interpreter.UFix64Value(123000000),
	}

	for _, fixedPointType := range sema.AllFixedPointTypes {
		if _, ok := tests[fixedPointType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", fixedPointType))
		}
	}

	for fixedPointType, value := range tests {

		t.Run(fixedPointType, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x: %[1]s = 1.23
                      let y = %[1]s(0.42) + %[1]s(0.81)
                      let z = y == x
                    `,
					fixedPointType,
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

var testFixedPointValues = map[string]interpreter.Value{
	"Fix64":  interpreter.Fix64Value(50 * sema.Fix64Factor),
	"UFix64": interpreter.UFix64Value(50 * sema.Fix64Factor),
}

func init() {
	for _, fixedPointType := range sema.AllFixedPointTypes {
		if _, ok := testFixedPointValues[fixedPointType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing fixed-point type: %s", fixedPointType))
		}
	}
}

func TestInterpretFixedPointConversions(t *testing.T) {

	t.Parallel()

	// check conversion to integer types

	for fixedPointType, fixedPointValue := range testFixedPointValues {

		for integerType, integerValue := range testIntegerTypesAndValues {

			testName := fmt.Sprintf("valid %s to %s", fixedPointType, integerType)

			t.Run(testName, func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
                          let x: %[1]s = 50.0
                          let y = %[2]s(x)
                        `,
						fixedPointType,
						integerType,
					),
				)

				assert.Equal(t,
					fixedPointValue,
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					integerValue,
					inter.Globals["y"].Value,
				)
			})
		}
	}

	t.Run("valid UFix64 to UFix64", func(t *testing.T) {

		for _, value := range []uint64{
			50,
			sema.UFix64TypeMinInt,
			sema.UFix64TypeMaxInt,
		} {

			t.Run(fmt.Sprint(value), func(t *testing.T) {

				code := fmt.Sprintf(
					`
                          let x: UFix64 = %d.0
                          let y = UFix64(x)
                        `,
					value,
				)

				inter := parseCheckAndInterpret(t, code)

				expected := interpreter.UFix64Value(value * sema.Fix64Factor)

				assert.Equal(t,
					expected,
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					expected,
					inter.Globals["y"].Value,
				)
			})
		}
	})

	t.Run("valid Fix64 to Fix64", func(t *testing.T) {

		for _, value := range []int64{
			-50,
			50,
			sema.Fix64TypeMinInt,
			sema.Fix64TypeMaxInt,
		} {

			t.Run(fmt.Sprint(value), func(t *testing.T) {

				code := fmt.Sprintf(
					`
                          let x: Fix64 = %d.0
                          let y = Fix64(x)
                        `,
					value,
				)

				inter := parseCheckAndInterpret(t, code)

				expected := interpreter.Fix64Value(value * sema.Fix64Factor)

				assert.Equal(t,
					expected,
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					expected,
					inter.Globals["y"].Value,
				)
			})
		}
	})

	t.Run("valid Fix64 to UFix64", func(t *testing.T) {

		for _, value := range []int64{0, 50, sema.Fix64TypeMaxInt} {

			t.Run(fmt.Sprint(value), func(t *testing.T) {

				code := fmt.Sprintf(
					`
                          let x: Fix64 = %d.0
                          let y = UFix64(x)
                        `,
					value,
				)

				inter := parseCheckAndInterpret(t, code)

				assert.Equal(t,
					interpreter.Fix64Value(value*sema.Fix64Factor),
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.UFix64Value(value*sema.Fix64Factor),
					inter.Globals["y"].Value,
				)
			})
		}
	})

	t.Run("valid UFix64 to Fix64", func(t *testing.T) {

		for _, value := range []int64{0, 50, sema.Fix64TypeMaxInt} {

			t.Run(fmt.Sprint(value), func(t *testing.T) {

				code := fmt.Sprintf(
					`
                          let x: UFix64 = %d.0
                          let y = Fix64(x)
                        `,
					value,
				)

				inter := parseCheckAndInterpret(t, code)

				assert.Equal(t,
					interpreter.UFix64Value(value*sema.Fix64Factor),
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.Fix64Value(value*sema.Fix64Factor),
					inter.Globals["y"].Value,
				)
			})
		}
	})

	t.Run("invalid negative Fix64 to UFix64", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
		  fun test(): UFix64 {
		      let x: Fix64 = -1.0
		      return UFix64(x)
		  }
		`)

		_, err := inter.Invoke("test")
		require.Error(t, err)

		assert.IsType(t, interpreter.UnderflowError{}, err)
	})

	t.Run("invalid UFix64 > max Fix64 int to Fix64", func(t *testing.T) {

		inter := parseCheckAndInterpret(t,
			fmt.Sprintf(
				`
		          fun test(): Fix64 {
		              let x: UFix64 = %d.0
		              return Fix64(x)
		          }
		        `,
				sema.Fix64TypeMaxInt+1,
			),
		)

		_, err := inter.Invoke("test")
		require.Error(t, err)

		assert.IsType(t, interpreter.OverflowError{}, err)
	})

	t.Run("invalid negative integer to UFix64", func(t *testing.T) {

		for _, integerType := range sema.AllSignedIntegerTypes {

			t.Run(integerType.String(), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
	                     fun test(): UFix64 {
	                         let x: %s = -1
	                         return UFix64(x)
	                     }
	                   `,
						integerType,
					),
				)

				_, err := inter.Invoke("test")
				require.Error(t, err)

				assert.IsType(t, interpreter.UnderflowError{}, err)
			})
		}
	})

	t.Run("invalid big integer (>uint64) to UFix64", func(t *testing.T) {

		bigIntegerTypes := []sema.Type{
			&sema.Word64Type{},
			&sema.UInt64Type{},
			&sema.UInt128Type{},
			&sema.UInt256Type{},
			&sema.Int256Type{},
			&sema.Int128Type{},
		}

		for _, integerType := range bigIntegerTypes {

			t.Run(integerType.String(), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
	                     fun test(): UFix64 {
	                         let x: %s = %d
	                         return UFix64(x)
	                     }
	                   `,
						integerType,
						sema.UFix64TypeMaxInt+1,
					),
				)

				_, err := inter.Invoke("test")
				require.Error(t, err)

				assert.IsType(t, interpreter.OverflowError{}, err)
			})
		}
	})

	t.Run("invalid integer > UFix64 max int to UFix64", func(t *testing.T) {

		const testedValue = sema.UFix64TypeMaxInt + 1
		testValueBig := big.NewInt(0).SetUint64(testedValue)

		for _, integerType := range sema.AllIntegerTypes {

			// Only test for integer types that can hold testedValue

			maxInt := integerType.(sema.IntegerRangedType).MaxInt()
			if maxInt != nil && maxInt.Cmp(testValueBig) < 0 {
				continue
			}

			t.Run(integerType.String(), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
	                     fun test(): UFix64 {
	                         let x: %s = %d
	                         return UFix64(x)
	                     }
	                   `,
						integerType,
						testedValue,
					),
				)

				_, err := inter.Invoke("test")
				require.Error(t, err)

				assert.IsType(t, interpreter.OverflowError{}, err)
			})
		}
	})

	t.Run("invalid integer > Fix64 max int to Fix64", func(t *testing.T) {

		const testedValue = sema.Fix64TypeMaxInt + 1
		testValueBig := big.NewInt(0).SetUint64(testedValue)

		for _, integerType := range sema.AllIntegerTypes {

			// Only test for integer types that can hold testedValue

			maxInt := integerType.(sema.IntegerRangedType).MaxInt()
			if maxInt != nil && maxInt.Cmp(testValueBig) < 0 {
				continue
			}

			t.Run(integerType.String(), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
	                     fun test(): Fix64 {
	                         let x: %s = %d
	                         return Fix64(x)
	                     }
	                   `,
						integerType,
						testedValue,
					),
				)

				_, err := inter.Invoke("test")
				require.Error(t, err)

				assert.IsType(t, interpreter.OverflowError{}, err)
			})
		}
	})

	t.Run("invalid integer < Fix64 min int to Fix64", func(t *testing.T) {

		const testedValue = sema.Fix64TypeMinInt - 1
		testValueBig := big.NewInt(testedValue)

		for _, integerType := range sema.AllSignedIntegerTypes {

			// Only test for integer types that can hold testedValue

			minInt := integerType.(sema.IntegerRangedType).MinInt()
			if minInt != nil && minInt.Cmp(testValueBig) > 0 {
				continue
			}

			t.Run(integerType.String(), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
	                     fun test(): Fix64 {
	                         let x: %s = %d
	                         return Fix64(x)
	                     }
	                   `,
						integerType,
						testedValue,
					),
				)

				_, err := inter.Invoke("test")
				require.Error(t, err)

				assert.IsType(t, interpreter.UnderflowError{}, err)
			})
		}
	})
}

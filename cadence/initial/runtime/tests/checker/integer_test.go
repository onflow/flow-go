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
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

var allIntegerTypesAndAddressType = append(
	sema.AllIntegerTypes,
	&sema.AddressType{},
)

func TestCheckIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				checker, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let x: %s = 0x1
                        `,
						ty,
					),
				)

				require.NoError(t, err)

				assert.Equal(t,
					ty,
					checker.GlobalValues["x"].Type,
				)
			})
		}
	}
}

func TestCheckIntegerLiteralTypeConversionInAssignment(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				checker, err := ParseAndCheck(t,
					fmt.Sprintf(`
                          var x: %s = 0x1
                          fun test() {
                              x = 0x2
                          }
                        `,
						ty,
					),
				)

				require.NoError(t, err)

				assert.Equal(t,
					ty,
					checker.GlobalValues["x"].Type,
				)
			})
		}
	}
}

func TestCheckIntegerLiteralRanges(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {
		t.Run(ty.String(), func(t *testing.T) {

			min := ty.(sema.IntegerRangedType).MinInt()
			var minPlusOne *big.Int
			if min != nil {
				minPlusOne = new(big.Int).Add(min, big.NewInt(1))
			}

			max := ty.(sema.IntegerRangedType).MaxInt()
			var maxMinusOne *big.Int
			if max != nil {
				maxMinusOne = new(big.Int).Sub(max, big.NewInt(1))
			}

			var minString string
			var minPlusOneString string
			var maxString string
			var maxMinusOneString string

			// addresses are only valid as hexadecimal literals
			if _, isAddressType := ty.(*sema.AddressType); isAddressType {
				formatAddress := func(i *big.Int) string {
					return fmt.Sprintf("0x%s", i.Text(16))
				}

				minString = formatAddress(min)
				minPlusOneString = formatAddress(minPlusOne)
				maxString = formatAddress(max)
				maxMinusOneString = formatAddress(maxMinusOne)
			} else {
				if min != nil {
					minString = min.String()
					minPlusOneString = minPlusOne.String()
				}
				if max != nil {
					maxString = max.String()
					maxMinusOneString = maxMinusOne.String()
				}
			}

			// check min, if any

			if minString != "" {
				t.Run("min", func(t *testing.T) {
					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let min: %[1]s = %[2]s
                              let a = %[1]s(%[2]s)
                            `,
							ty.String(),
							minString,
						),
					)
					assert.NoError(t, err)
				})

				t.Run("min + 1", func(t *testing.T) {
					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let minPlusOne: %[1]s = %[2]s
                              let a = %[1]s(%[2]s)
                            `,
							ty.String(),
							minPlusOneString,
						),
					)
					assert.NoError(t, err)
				})
			}

			// check max, if any

			if maxString != "" {
				t.Run("max", func(t *testing.T) {
					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let max: %[1]s = %[2]s
			                  let b = %[1]s(%[2]s)
                            `,
							ty.String(),
							maxString,
						),
					)
					assert.NoError(t, err)
				})

				t.Run("max - 1", func(t *testing.T) {
					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let maxMinusOne: %[1]s = %[2]s
			                  let b = %[1]s(%[2]s)
                            `,
							ty.String(),
							maxMinusOneString,
						),
					)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func TestCheckInvalidIntegerLiteralValues(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {

		min := ty.(sema.IntegerRangedType).MinInt()
		if min != nil {
			t.Run(fmt.Sprintf("%s - 1", ty.String()), func(t *testing.T) {
				var minMinusOneString string

				// addresses are only valid as hexadecimal literals
				if _, isAddressType := ty.(*sema.AddressType); isAddressType {
					minMinusOneString = "-0x1"
				} else {
					minMinusOne := new(big.Int).Sub(min, big.NewInt(1))
					minMinusOneString = minMinusOne.String()
				}

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let minMinusOne: %[1]s = %[2]s
                          let minMinusOne2 = %[1]s(%[2]s)
                        `,
						ty.String(),
						minMinusOneString,
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				if _, isAddressType := ty.(*sema.AddressType); isAddressType {
					assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[0])
					assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[1])
				} else {
					assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[0])
					assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[1])
				}
			})
		}

		max := ty.(sema.IntegerRangedType).MaxInt()
		if max != nil {
			t.Run(fmt.Sprintf("%s + 1", ty.String()), func(t *testing.T) {

				maxPlusOne := new(big.Int).Add(max, big.NewInt(1))
				var maxPlusOneString string

				// addresses are only valid as hexadecimal literals
				if _, isAddressType := ty.(*sema.AddressType); isAddressType {
					maxPlusOneString = fmt.Sprintf("0x%s", maxPlusOne.Text(16))
				} else {
					maxPlusOneString = maxPlusOne.String()
				}

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let maxPlusOne: %[1]s = %[2]s
                          let maxPlusOne2 = %[1]s(%[2]s)
                        `,
						ty.String(),
						maxPlusOneString,
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				if _, isAddressType := ty.(*sema.AddressType); isAddressType {
					assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[0])
					assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[1])
				} else {
					assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[0])
					assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[1])
				}
			})
		}
	}
}

// Test fix for crasher, see https://github.com/dapperlabs/flow-go/pull/675
// Integer literal value fits range can't be checked when target is Never
//
func TestCheckInvalidIntegerLiteralWithNeverReturnType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        fun test(): Never {
            return 1
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                          fun test(_ x: %s) {}
                          let x = test(0x1)
                        `,
						ty,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckIntegerLiteralTypeConversionInReturn(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {
			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                          fun test(): %s {
                              return 0x1
                          }
                        `,
						ty,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckInvalidAddressDecimal(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        let a: Address = 1
        let b = Address(2)
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[0])
	assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[1])
}

func TestCheckSignedIntegerNegate(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllSignedIntegerTypes {
		name := ty.String()
		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      let x: %s = 1
                      let y = -x
                    `,
					name,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidUnsignedIntegerNegate(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllUnsignedIntegerTypes {
		name := ty.String()
		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      let x: %s = 1
                      let y = -x
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
		})
	}
}

func TestCheckInvalidIntegerConversionFunctionWithoutArgs(t *testing.T) {

	t.Parallel()

	for _, ty := range allIntegerTypesAndAddressType {

		t.Run(ty.String(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let e = %s()
                    `,
					ty,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ArgumentCountError{}, errs[0])

		})
	}
}

func TestCheckFixedPointToIntegerConversion(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllIntegerTypes {

		t.Run(ty.String(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let e = %s(0.0)
                    `,
					ty,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckIntegerLiteralArguments(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllIntegerTypes {

		t.Run(ty.String(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      fun add(_ a: %[1]s, _ b: %[1]s): %[1]s {
                          return a + b
                      }

                      let res = add(1, 2)
                    `,
					ty,
				),
			)

			require.NoError(t, err)
		})
	}
}

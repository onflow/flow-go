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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckFixedPointLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllFixedPointTypes {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				checker, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let x: %s = 1.2
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

func TestCheckFixedPointLiteralTypeConversionInAssignment(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllFixedPointTypes {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				checker, err := ParseAndCheck(t,
					fmt.Sprintf(`
                         var x: %s = 1.2
                         fun test() {
                             x = 3.4
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

func TestCheckFixedPointLiteralRanges(t *testing.T) {

	t.Parallel()

	inferredType := func(t *testing.T, literal string) sema.Type {

		// NOTE: not checking error, because the inferred type
		// might be used for an invalid literal

		checker, _ := ParseAndCheck(t,
			fmt.Sprintf(
				`
				  let x = %s
				`,
				literal,
			),
		)

		return checker.GlobalValues["x"].Type
	}

	for _, ty := range sema.AllFixedPointTypes {
		t.Run(ty.String(), func(t *testing.T) {

			ranged := ty.(sema.FractionalRangedType)

			// min

			minInt := ranged.MinInt()
			minIntMinusOne := new(big.Int).Sub(minInt, big.NewInt(1))
			minIntPlusOne := new(big.Int).Add(minInt, big.NewInt(1))
			minFractional := ranged.MinFractional()
			minFractionalPlusOne := new(big.Int).Add(minFractional, big.NewInt(1))

			formatLiteral := func(integer, fractional *big.Int) string {
				var builder strings.Builder
				builder.WriteString(integer.String())
				builder.WriteRune('.')
				builder.WriteString(interpreter.PadLeft(fractional.String(), '0', ranged.Scale()))
				return builder.String()
			}

			t.Run("min int + 1, min fractional", func(t *testing.T) {

				literal := formatLiteral(minIntPlusOne, minFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					assert.NoError(t, err)
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						assert.NoError(t, err)
					})
				}
			})

			t.Run("min int, min fractional", func(t *testing.T) {

				literal := formatLiteral(minInt, minFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					assert.NoError(t, err)
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						assert.NoError(t, err)
					})
				}
			})

			if minInt.Sign() < 0 {

				t.Run("min int, min fractional - 1", func(t *testing.T) {

					literal := formatLiteral(minInt, minFractionalPlusOne)

					t.Run("variable declaration", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: %s = %s
                                `,
								ty,
								literal,
							),
						)

						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
					})

					if inferredType(t, literal).Equal(ty) {

						t.Run("expression statement", func(t *testing.T) {

							_, err := ParseAndCheck(t,
								fmt.Sprintf(
									`
					                  fun test() { %s }
					                `,
									literal,
								),
							)

							errs := ExpectCheckerErrors(t, err, 1)

							assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
						})
					}
				})
			}

			t.Run("min int - 1, min fractional", func(t *testing.T) {

				literal := formatLiteral(minIntMinusOne, minFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
					})
				}
			})

			// max

			maxInt := ranged.MaxInt()
			maxIntMinusOne := new(big.Int).Sub(maxInt, big.NewInt(1))
			maxIntPlusOne := new(big.Int).Add(maxInt, big.NewInt(1))
			maxFractional := ranged.MaxFractional()
			maxFractionalPlusOne := new(big.Int).Add(maxFractional, big.NewInt(1))

			t.Run("max int - 1, max fractional", func(t *testing.T) {

				literal := formatLiteral(maxIntMinusOne, maxFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					assert.NoError(t, err)
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						assert.NoError(t, err)
					})
				}
			})

			t.Run("max int, max fractional", func(t *testing.T) {

				literal := formatLiteral(maxInt, maxFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					assert.NoError(t, err)
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						assert.NoError(t, err)
					})
				}
			})

			t.Run("max int, max fractional + 1", func(t *testing.T) {

				literal := formatLiteral(maxInt, maxFractionalPlusOne)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              let x: %s = %s
                            `,
							ty,
							literal,
						),
					)

					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
				                  fun test() { %s }
				                `,
								literal,
							),
						)

						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
					})
				}
			})

			t.Run("max int + 1, max fractional", func(t *testing.T) {

				literal := formatLiteral(maxIntPlusOne, maxFractional)

				t.Run("variable declaration", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                             let x: %s = %s
                           `,
							ty,
							literal,
						),
					)

					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
				})

				if inferredType(t, literal).Equal(ty) {

					t.Run("expression statement", func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  fun test() { %s }
                                `,
								literal,
							),
						)

						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
					})
				}
			})
		})
	}
}

// Fixed-point literal value fits range can't be checked when target is Never
//
func TestCheckInvalidFixedPointLiteralWithNeverReturnType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        fun test(): Never {
            return 1.2
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckFixedPointLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllFixedPointTypes {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                          fun test(_ x: %s) {}
                          let x = test(1.2)
                        `,
						ty,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFixedPointLiteralTypeConversionInReturn(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllFixedPointTypes {
		// Test non-optional and optional type

		for _, ty := range []sema.Type{
			ty,
			&sema.OptionalType{Type: ty},
		} {
			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                         fun test(): %s {
                             return 1.2
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

func TestCheckSignedFixedPointNegate(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllSignedFixedPointTypes {
		name := ty.String()

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      let x: %s = 1.2
                      let y = -x
                    `,
					name,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidUnsignedFixedPointNegate(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllUnsignedFixedPointTypes {

		t.Run(ty.String(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      let x: %s = 1.2
                      let y = -x
                    `,
					ty,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
		})
	}
}

func TestCheckInvalidNegativeZeroUnsignedFixedPoint(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllUnsignedFixedPointTypes {

		t.Run(ty.String(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      let x: %s = -0.42
                    `,
					ty,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
		})
	}
}

func TestCheckFixedPointLiteralScales(t *testing.T) {

	t.Parallel()

	for _, ty := range sema.AllFixedPointTypes {
		t.Run(ty.String(), func(t *testing.T) {

			scale := ty.(sema.FractionalRangedType).Scale()

			generateFraction := func(scale uint) string {
				var builder strings.Builder
				var i uint
				for ; i < scale; i++ {
					builder.WriteRune('0' + rune(i%10))
				}
				return builder.String()
			}

			var i uint = 1
			for ; i < scale*2; i++ {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let withType: %[1]s = 1.%[2]s
                          let withoutType = 1.%[2]s
                        `,
						ty,
						generateFraction(i),
					),
				)

				if i <= scale {
					assert.NoError(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidFixedPointLiteralScaleError{}, errs[0])
					assert.IsType(t, &sema.InvalidFixedPointLiteralScaleError{}, errs[1])
				}
			}
		})
	}
}

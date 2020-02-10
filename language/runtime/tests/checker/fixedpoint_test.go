package checker

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckFixedPointLiteralTypeConversionInVariableDeclaration(t *testing.T) {

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

	for _, ty := range sema.AllFixedPointTypes {
		t.Run(ty.String(), func(t *testing.T) {

			ranged := ty.(sema.FractionalRangedType)

			// min

			minInt := ranged.MinInt()
			minIntMinusOne := big.NewInt(0).Sub(minInt, big.NewInt(1))
			minIntPlusOne := big.NewInt(0).Add(minInt, big.NewInt(1))
			minFractional := ranged.MinFractional()
			minFractionalPlusOne := big.NewInt(0).Add(minFractional, big.NewInt(1))

			t.Run("min int + 1, min fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let x: %s = %s.%s
                        `,
						ty,
						minIntPlusOne,
						minFractional,
					),
				)

				assert.NoError(t, err)
			})

			t.Run("min int, min fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let x: %s = %s.%s
                        `,
						ty,
						minInt,
						minFractional,
					),
				)

				assert.NoError(t, err)
			})

			t.Run("min int, min fractional - 1", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
			             let x: %s = %s.%s
			           `,
						ty,
						minInt,
						minFractionalPlusOne,
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
			})

			t.Run("min int - 1, min fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         let x: %s = %s.%s
                       `,
						ty,
						minIntMinusOne,
						minFractional,
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
			})

			// max

			maxInt := ranged.MaxInt()
			maxIntMinusOne := big.NewInt(0).Sub(maxInt, big.NewInt(1))
			maxIntPlusOne := big.NewInt(0).Add(maxInt, big.NewInt(1))
			maxFractional := ranged.MaxFractional()
			maxFractionalPlusOne := big.NewInt(0).Add(maxFractional, big.NewInt(1))

			t.Run("max int - 1, max fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
			             let x: %s = %s.%s
			           `,
						ty,
						maxIntMinusOne,
						maxFractional,
					),
				)

				assert.NoError(t, err)
			})

			t.Run("max int, max fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
			             let x: %s = %s.%s
			           `,
						ty,
						maxInt,
						maxFractional,
					),
				)

				assert.NoError(t, err)
			})

			t.Run("max int, max fractional + 1", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
			             let x: %[1]s = %s.%s
			           `,
						ty,
						maxInt,
						maxFractionalPlusOne,
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
			})

			t.Run("max int + 1, max fractional", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
			             let x: %s = %s.%s
			           `,
						ty,
						maxIntPlusOne,
						maxFractional,
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidFixedPointLiteralRangeError{}, errs[0])
			})
		})
	}
}

// Fixed-point literal value fits range can't be checked when target is Never
//
func TestCheckInvalidFixedPointLiteralWithNeverReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Never {
            return 1.2
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckFixedPointLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

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

	for _, ty := range sema.AllUnsignedFixedPointTypes {
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

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
		})
	}
}

func TestCheckFixedPointLiteralScales(t *testing.T) {

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
			              let val: %[1]s = 1.%s
			            `,
						ty,
						generateFraction(i),
					),
				)

				if i <= scale {
					assert.NoError(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidFixedPointLiteralScaleError{}, errs[0])
				}
			}
		})
	}
}

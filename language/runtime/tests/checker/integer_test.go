package checker

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Int8 = 1
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.Int8Type{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Int8? = 1
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.Int8Type{}},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckIntegerLiteralTypeConversionInAssignment(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        var x: Int8 = 1
        fun test() {
            x = 2
        }
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.Int8Type{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckIntegerLiteralTypeConversionInAssignmentOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
        var x: Int8? = 1
        fun test() {
            x = 2
        }
    `)

	require.NoError(t, err)
}

func TestCheckIntegerLiteralRanges(t *testing.T) {

	for _, ty := range []sema.Type{
		&sema.Int8Type{},
		&sema.Int16Type{},
		&sema.Int32Type{},
		&sema.Int64Type{},
		&sema.UInt8Type{},
		&sema.UInt16Type{},
		&sema.UInt32Type{},
		&sema.UInt64Type{},
		&sema.AddressType{},
	} {
		t.Run(ty.String(), func(t *testing.T) {

			min := ty.(sema.Ranged).Min()
			max := ty.(sema.Ranged).Max()

			var minString string
			var maxString string

			// addresses are only valid as hexadecimal literals
			if _, isAddressType := ty.(*sema.AddressType); isAddressType {
				minString = fmt.Sprintf("0x%s", min.Text(16))
				maxString = fmt.Sprintf("0x%s", max.Text(16))
			} else {
				minString = min.String()
				maxString = max.String()
			}

			code := fmt.Sprintf(
				`
                    let min: %[1]s = %[2]s
                    let max: %[1]s = %[3]s
                    let a = %[1]s(%[2]s)
                    let b = %[1]s(%[3]s)
                `,
				ty.String(),
				minString,
				maxString,
			)

			_, err := ParseAndCheck(t, code)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidIntegerLiteralValues(t *testing.T) {

	for _, ty := range []sema.Type{
		&sema.Int8Type{},
		&sema.Int16Type{},
		&sema.Int32Type{},
		&sema.Int64Type{},
		&sema.UInt8Type{},
		&sema.UInt16Type{},
		&sema.UInt32Type{},
		&sema.UInt64Type{},
		&sema.AddressType{},
	} {

		t.Run(fmt.Sprintf("%s_minMinusOne", ty.String()), func(t *testing.T) {

			var minMinusOneString string

			// addresses are only valid as hexadecimal literals
			if _, isAddressType := ty.(*sema.AddressType); isAddressType {
				minMinusOneString = "-0x1"
			} else {
				minMinusOne := big.NewInt(0).Sub(ty.(sema.Ranged).Min(), big.NewInt(1))
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

		t.Run(fmt.Sprintf("%s_maxPlusOne", ty.String()), func(t *testing.T) {

			maxPlusOne := big.NewInt(0).Add(ty.(sema.Ranged).Max(), big.NewInt(1))
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

// Test fix for crasher, see https://github.com/dapperlabs/flow-go/pull/675
// Integer literal value fits range can't be checked when target is Never
//
func TestCheckInvalidIntegerLiteralWithNeverReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Never {
            return 1
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(_ x: Int8) {}
        let x = test(1)
    `)

	require.NoError(t, err)
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(_ x: Int8?) {}
        let x = test(1)
    `)

	require.NoError(t, err)
}

func TestCheckIntegerLiteralTypeConversionInReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Int8 {
            return 1
        }
    `)

	require.NoError(t, err)
}

func TestCheckIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Int8? {
            return 1
        }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidAddressDecimal(t *testing.T) {

	_, err := ParseAndCheck(t, `
        let a: Address = 1
        let b = Address(2)
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[0])
	assert.IsType(t, &sema.InvalidAddressLiteralError{}, errs[1])
}

func TestCheckSignedIntegerNegate(t *testing.T) {

	for _, ty := range []sema.Type{
		&sema.Int8Type{},
		&sema.Int16Type{},
		&sema.Int32Type{},
		&sema.Int64Type{},
	} {
		name := ty.String()
		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                        let x = -%s(1)
                    `,
					name,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidUnsignedIntegerNegate(t *testing.T) {

	for _, ty := range []sema.Type{
		&sema.UInt8Type{},
		&sema.UInt16Type{},
		&sema.UInt32Type{},
		&sema.UInt64Type{},
	} {
		name := ty.String()
		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                        let x = -%s(1)
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidUnaryOperandError{}, errs[0])
		})
	}
}

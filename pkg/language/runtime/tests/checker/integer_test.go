package checker

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Int8 = 1
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.Int8Type{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Int8? = 1
    `)

	assert.Nil(t, err)

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

	assert.Nil(t, err)

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

	assert.Nil(t, err)
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
	} {
		t.Run(ty.String(), func(t *testing.T) {

			code := fmt.Sprintf(`
                let min: %s = %s
                let max: %s = %s
            `,
				ty.String(),
				ty.(sema.Ranged).Min(),
				ty.String(),
				ty.(sema.Ranged).Max(),
			)

			_, err := ParseAndCheck(t, code)

			assert.Nil(t, err)
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
	} {
		t.Run(fmt.Sprintf("%s_minMinusOne", ty.String()), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                let minMinusOne: %s = %s
            `,
				ty.String(),
				big.NewInt(0).Sub(ty.(sema.Ranged).Min(), big.NewInt(1)),
			))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[0])
		})

		t.Run(fmt.Sprintf("%s_maxPlusOne", ty.String()), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                let maxPlusOne: %s = %s
            `,
				ty.String(),
				big.NewInt(0).Add(ty.(sema.Ranged).Max(), big.NewInt(1)),
			))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidIntegerLiteralRangeError{}, errs[0])
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

	assert.Nil(t, err)
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(_ x: Int8?) {}
        let x = test(1)
    `)

	assert.Nil(t, err)
}

func TestCheckIntegerLiteralTypeConversionInReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Int8 {
            return 1
        }
    `)

	assert.Nil(t, err)
}

func TestCheckIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
        fun test(): Int8? {
            return 1
        }
    `)

	assert.Nil(t, err)
}

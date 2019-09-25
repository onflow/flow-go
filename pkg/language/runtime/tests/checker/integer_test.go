package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

func TestCheckIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x: Int8 = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.Int8Type{}))
}

func TestCheckIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x: Int8? = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.Int8Type{}}))
}

func TestCheckIntegerLiteralTypeConversionInAssignment(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        var x: Int8 = 1
        fun test() {
            x = 2
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.Int8Type{}))
}

func TestCheckIntegerLiteralTypeConversionInAssignmentOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        var x: Int8? = 1
        fun test() {
            x = 2
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralRanges(t *testing.T) {
	RegisterTestingT(t)

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
			RegisterTestingT(t)

			code := fmt.Sprintf(`
                let min: %s = %s
                let max: %s = %s 
	        `,
				ty.String(),
				ty.(sema.Ranged).Min(),
				ty.String(),
				ty.(sema.Ranged).Max(),
			)

			_, err := ParseAndCheck(code)

			Expect(err).
				To(Not(HaveOccurred()))
		})
	}
}

func TestCheckInvalidIntegerLiteralValues(t *testing.T) {
	RegisterTestingT(t)

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
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
                let minMinusOne: %s = %s
	        `,
				ty.String(),
				big.NewInt(0).Sub(ty.(sema.Ranged).Min(), big.NewInt(1)),
			))

			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidIntegerLiteralRangeError{}))
		})

		t.Run(fmt.Sprintf("%s_maxPlusOne", ty.String()), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
                let maxPlusOne: %s = %s
	        `,
				ty.String(),
				big.NewInt(0).Add(ty.(sema.Ranged).Max(), big.NewInt(1)),
			))

			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidIntegerLiteralRangeError{}))
		})
	}
}

// Test fix for crasher, see https://github.com/dapperlabs/flow-go/pull/675
// Integer literal value fits range can't be checked when target is Never
//
func TestCheckInvalidIntegerLiteralWithNeverReturnType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test(): Never {
            return 1
        }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test(_ x: Int8) {}
        let x = test(1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test(_ x: Int8?) {}
        let x = test(1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test(): Int8 {
            return 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
        fun test(): Int8? {
            return 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

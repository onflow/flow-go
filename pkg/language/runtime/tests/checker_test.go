package tests

import (
	"fmt"
	"math/big"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
)

func parseAndCheck(code string) (*sema.Checker, error) {
	return parseAndCheckWithExtra(code, nil, nil, nil)
}

func parseAndCheckWithExtra(
	code string,
	values map[string]sema.ValueDeclaration,
	types map[string]sema.TypeDeclaration,
	resolver ast.ImportResolver,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	Expect(err).
		To(Not(HaveOccurred()))

	if resolver != nil {
		err := program.ResolveImports(resolver)
		if err != nil {
			return nil, err
		}
	}

	checker, err := sema.NewChecker(program, values, types)
	if err != nil {
		return checker, err
	}

	err = checker.Check()
	return checker, err
}

func TestCheckConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = 1
        var y = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.IntType{}))

	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.IntType{}))
}

func TestCheckIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x: Int8 = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.Int8Type{}))
}

func TestCheckIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x: Int8? = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.Int8Type{}}))
}

func TestCheckIntegerLiteralTypeConversionInAssignment(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
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

	_, err := parseAndCheck(`
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

			_, err := parseAndCheck(code)

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

			_, err := parseAndCheck(fmt.Sprintf(`
                let minMinusOne: %s = %s
	        `,
				ty.String(),
				big.NewInt(0).Sub(ty.(sema.Ranged).Min(), big.NewInt(1)),
			))

			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidIntegerLiteralRangeError{}))
		})

		t.Run(fmt.Sprintf("%s_maxPlusOne", ty.String()), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
                let maxPlusOne: %s = %s
	        `,
				ty.String(),
				big.NewInt(0).Add(ty.(sema.Ranged).Max(), big.NewInt(1)),
			))

			errs := expectCheckerErrors(err, 1)

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

	_, err := parseAndCheck(`
        fun test(): Never {
            return 1
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test(_ x: Int8) {}
        let x = test(1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test(_ x: Int8?) {}
        let x = test(1)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test(): Int8 {
            return 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test(): Int8? {
            return 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckBoolean(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.BoolType{}))
}

func TestCheckCharacter(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x: Character = "x"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.CharacterType{}))
}

func TestCheckCharacterUnicodeScalar(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x: Character = "\u{1F1FA}\u{1F1F8}"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.CharacterType{}))
}

// TODO: prevent invalid character literals
// func TestCheckInvalidCharacterLiteral(t *testing.T) {
// 	RegisterTestingT(t)

// 	_, err := parseAndCheck(`
//         let x: Character = "abc"
// 	`)

// 	errs := expectCheckerErrors(err, 1)

// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

func TestCheckString(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = "x"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.StringType{}))
}

func TestCheckStringConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
	  fun test(): String {
	 	  let a = "abc"
		  let b = "def"
		  let c = a.concat(b)
		  return c
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStringConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): String {
		  let a = "abc"
		  let b = [1, 2]
		  let c = a.concat(b)
		  return c
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckStringConcatBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): String {
		  let a = "abc"
		  let b = "def"
		  let c = a.concat
		  return c(b)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStringSlice(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
	  fun test(): String {
	 	  let a = "abcdef"
		  return a.slice(from: 0, upTo: 1)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStringSlice(t *testing.T) {
	t.Run("MissingBothArgumentLabels", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := parseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(0, 1)
		`)

		errs := expectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
	})

	t.Run("MissingOneArgumentLabel", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := parseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(from: 0, 1)
		`)

		errs := expectCheckerErrors(err, 1)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
	})

	t.Run("InvalidArgumentType", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := parseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(from: "a", upTo: "b")
		`)

		errs := expectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
	})
}

func TestCheckStringSliceBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): String {
		  let a = "abcdef"
		  let c = a.slice
		  return c(from: 0, upTo: 1)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func expectCheckerErrors(err error, len int) []error {
	if len <= 0 && err == nil {
		return nil
	}

	Expect(err).To(HaveOccurred())

	Expect(err).
		To(BeAssignableToTypeOf(&sema.CheckerError{}))

	errs := err.(*sema.CheckerError).Errors

	Expect(errs).To(HaveLen(len))

	return errs
}

func TestCheckInvalidGlobalConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun x() {}

        let y = true
        let y = false
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidGlobalFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        let x = true

        fun y() {}
        fun y() {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidLocalRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test() {
            let x = true
            let x = false
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidLocalFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
        fun test() {
            let x = true

            fun y() {}
            fun y() {}
        }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           return x
       }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidReturnValueError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobal(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x = y
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationInGlobalAndUnknownType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x: X = y
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("y"))
	Expect(errs[0].(*sema.NotDeclaredError).ExpectedKind).
		To(Equal(common.DeclarationKindValue))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[1].(*sema.NotDeclaredError).Name).
		To(Equal("X"))
	Expect(errs[1].(*sema.NotDeclaredError).ExpectedKind).
		To(Equal(common.DeclarationKindType))
}

func TestCheckInvalidUnknownDeclarationCallInGlobal(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x = y()
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = 2
          x = 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          var x = 2
          x = 3
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = 2

      fun test() {
          x = 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      var x = 2

      fun test(): Int {
          x = 3
          return x
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int8) {
           x = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))
}

func TestCheckInvalidArrayElements(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z = [0, true]
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedArrayIndexingWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [[0, 1], [2, 3]]
          z[0][1]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return 2[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexableTypeError{}))
}

func TestCheckInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[true] = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckArrayIndexingAssignmentWithInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0] = 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayIndexingAssignmentWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = [0, 3]
          z[0] = true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckStringIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = "abc"
          let y: Character = z[0]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStringIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = "abc"
          z[true]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckStringIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
		  let z = "abc"
		  let y: Character = "d"
          z[0] = y
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let z = "abc"
          z[0] = "d"
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

// TODO: prevent assignment with invalid character literal
// func TestCheckStringIndexingAssignmentWithInvalidCharacterLiteral(t *testing.T) {
// 	RegisterTestingT(t)

// 	_, err := parseAndCheck(`
//       fun test() {
//           let z = "abc"
//           z[0] = "def"
//       }
// 	`)

// 	errs := expectCheckerErrors(err, 1)

// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

func TestCheckInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x[0]
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          x[0] = 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidParameterTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: X, y: Y) {}
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

}

func TestCheckInvalidParameterNameRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int, a: Int) {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckParameterRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int) {
          let a = 1
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidRedeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(a: Int, a: Int) {
        let x = 1
        let x = 2
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckInvalidArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x a: Int, x b: Int) {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckArgumentLabelRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(_ a: Int, _ b: Int) {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConstantValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Bool = 1
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionDeclarationReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionExpressionReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let test = fun (): Int {
          return true
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          testX
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckParameterNameWithFunctionName(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(test: Int) {
          test
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if 1 {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          if true {} else {
              x
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckIfStatementTestWithDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int?): Int {
          if var y = x {
              return y
		  }
		  
		  return 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTestWithDeclarationReferenceInElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int?) {
          if var y = x {
              // ...
          } else {
              y
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckIfStatementTestWithDeclarationNestedOptionals(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     fun test(x: Int??): Int? {
         if var y = x {
             return y
		 }
		 
		 return nil
     }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIfStatementTestWithDeclarationNestedOptionalsExplicitAnnotation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     fun test(x: Int??): Int? {
         if var y: Int? = x {
             return y
		 }
		 
		 return nil
     }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTestWithDeclarationNonOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     fun test(x: Int) {
         if var y = x {
             // ...
		 }
		 
		 return
     }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementTestWithDeclarationSameType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int?): Int? {
          if var y: Int? = x {
             return y
		  }
		  
		  return nil
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 1 : 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidConditionalExpressionTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = 1 ? 2 : 3
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidConditionalExpressionElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 2 : y
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidConditionalExpressionTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = true ? 2 : false
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while 1 {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidWhileBlock(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          while true { x }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionCallWithNotRequiredArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckIndirectFunctionCallWithoutArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          let g = f
          return g(1)
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionCallMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(y: 1)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return true()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          return 2()
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotCallableError{}))
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: true)
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidFunctionCallWithWrongTypeAndMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
}

func TestCheckInvalidUnaryBooleanNegationOfInteger(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = !1
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = !true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidUnaryIntegerNegationOfBoolean(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = -true
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidUnaryOperandError{}))
}

func TestCheckUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a = -1
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

type operationTest struct {
	ty          sema.Type
	left, right string
	matchers    []types.GomegaMatcher
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
				{&sema.IntType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
					BeAssignableToTypeOf(&sema.TypeMismatchError{}),
				}},
				{&sema.IntType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.IntType{}, "true", "false", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
					BeAssignableToTypeOf(&sema.TypeMismatchError{}),
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationLess, ast.OperationLessEqual, ast.OperationGreater, ast.OperationGreaterEqual,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "1", "2", nil},
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "true", "false", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
			},
		},
		{
			operations: []ast.Operation{
				ast.OperationOr, ast.OperationAnd,
			},
			tests: []operationTest{
				{&sema.BoolType{}, "true", "false", nil},
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
				}},
				{&sema.BoolType{}, "1", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
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
				{&sema.BoolType{}, "true", "2", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, "1", "true", []types.GomegaMatcher{
					BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
				}},
				{&sema.BoolType{}, `"test"`, `"test"`, nil},
			},
		},
	}

	for _, operationTests := range allOperationTests {
		for _, operation := range operationTests.operations {
			for _, test := range operationTests.tests {
				t.Run("", func(t *testing.T) {
					RegisterTestingT(t)

					_, err := parseAndCheck(
						fmt.Sprintf(
							`fun test(): %s { return %s %s %s }`,
							test.ty, test.left, operation.Symbol(), test.right,
						),
					)

					errs := expectCheckerErrors(err, len(test.matchers))

					for i, matcher := range test.matchers {
						Expect(errs[i]).
							To(matcher)
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
		{&sema.StringType{}, "1", `"def"`, []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},
		{&sema.StringType{}, `"abc"`, "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
		}},
		{&sema.StringType{}, "1", "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},

		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[3, 4]", nil},
		// TODO: support empty arrays
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "[]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[3, 4]", nil},
		// {&sema.VariableSizedType{Type: &sema.IntType{}}, "[]", "[]", nil},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "1", "[3, 4]", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
			BeAssignableToTypeOf(&sema.TypeMismatchError{}),
		}},
		{&sema.VariableSizedType{Type: &sema.IntType{}}, "[1, 2]", "2", []types.GomegaMatcher{
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}),
			BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}),
		}},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(
				fmt.Sprintf(
					`fun test(): %s { return %s %s %s }`,
					test.ty, test.left, ast.OperationConcat.Symbol(), test.right,
				),
			)

			errs := expectCheckerErrors(err, len(test.matchers))

			for i, matcher := range test.matchers {
				Expect(errs[i]).To(matcher)
			}
		})
	}
}

func TestCheckFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun returnNothing() {
           return
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAnyReturnType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo(): Any {
          return foo
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let a: Any = 1
      let b: Any = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               break
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               fun () {
                   break
               }
           }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}

func TestCheckContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               continue
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       fun test() {
           while true {
               fun () {
                   continue
               }
           }
       }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}

func TestCheckInvalidFunctionDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          fun foo() {}
          fun foo() {}
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo() {
          fun foo() {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       pub fun test() {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionAccess(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       pub(set) fun test() {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidAccessModifierError{}))
}

func TestCheckInvalidCompositeRedeclaringType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Int {}
        `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckComposite(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              pub(set) var foo: Int

              init(foo: Int) {
                  self.foo = foo
              }

              pub fun getFoo(): Int {
                  return self.foo
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInitializerName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInitializerName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              initializer() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidInitializerNameError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let init: Int
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 3
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidNameError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun init() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidNameError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFields(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: Int
              let x: Int
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 4
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		Expect(errs[3]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[4]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun x() {}
              fun x() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: Int
              fun x() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: Int

              init() {
                  self.x = 1
              }

              fun y() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: X
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 3
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerParameterType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init(x: X) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerParameters(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init(x: Int, x: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init() { X }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun test() { X }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializerSelfReference(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init() { self }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunctionSelfReference(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun test() { self }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidLocalComposite(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          fun test() {
              %s Test {}
          }
        `, kind.Keyword()))

		errs := expectCheckerErrors(err, 1)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidDeclarationError{}))
	}
}

func TestCheckInvalidCompositeMissingInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
           %s Test {
               let foo: Int
           }
        `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let foo: Int

              init() {
                  self.foo = 1
              }

              fun test() {
                  self.foo
              }
          }
        `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init() {
                  self.foo
              }

              fun test() {
                  self.bar
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
			To(Equal("bar"))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {
              var foo: Int

              init() {
                  self.foo = 1
                  let alsoSelf %[2]s self
                  alsoSelf.foo = 2
              }

              fun test() {
                  self.foo = 3
                  let alsoSelf %[2]s self
                  alsoSelf.foo = 4
              }
          }
	    `,
			kind.Keyword(),
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeSelfAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {
              init() {
                  self %[2]s Test()
              }

              fun test() {
                  self %[2]s Test()
              }
          }
	    `,
			kind.Keyword(),
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              init() {
                  self.foo = 1
              }

              fun test() {
                  self.bar = 2
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
			To(Equal("bar"))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAssignmentWrongType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              var foo: Int

              init() {
                  self.foo = true
              }

              fun test() {
                  self.foo = false
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              let foo: Int

              init() {
                  // initialization is fine
                  self.foo = 1
              }

              fun test() {
                  // assignment is invalid
                  self.foo = 2
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun foo() {}

              fun bar() {
                  self.foo()
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun foo() {}

              fun bar() {
                  self.baz()
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {
              fun foo() {}

              fun bar() {
                  self.foo = 2
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))
		Expect(errs[0].(*sema.AssignmentToConstantMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInstantiation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {

              init(x: Int) {
                  let test: %[2]sTest %[3]s Test(x: 1)
              }

              fun test() {
                  let test: %[2]sTest %[3]s Test(x: 2)
              }
          }

          let test: %[2]sTest %[3]s Test(x: 3)
    	`,
			kind.Keyword(),
			annotation,
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidSameCompositeRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          let x = 1
          %[1]s Foo {}
          %[1]s Foo {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 2
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		// NOTE: two errors: one because type is redeclared,
		// the other because the global is redeclared

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidDifferentCompositeRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// only check different kinds
			if firstKind == secondKind {
				continue
			}

			_, err := parseAndCheck(fmt.Sprintf(`
              let x = 1
              %s Foo {}
              %s Foo {}
	        `, firstKind.Keyword(), secondKind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if firstKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			if secondKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := expectCheckerErrors(err, expectedErrorCount)

			// NOTE: two errors: one because type is redeclared,
			// the other because the global is redeclared

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

			if firstKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindStructure {

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			} else if firstKind != common.CompositeKindStructure ||
				secondKind != common.CompositeKindStructure {

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		}
	}
}

func TestCheckInvalidForwardReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = y
      let y = x
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidIncompatibleSameCompositeTypes(t *testing.T) {
	// tests that composite typing is nominal, not structural,
	// and composite kind is considered

	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			annotation := getCompositeKindAnnotation(firstKind)
			transferOperator := getCompositeKindTransferOperator(firstKind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s Foo {
                  init() {}
              }

              %[2]s Bar {
                  init() {}
              }

              let foo: %[3]sFoo %[4]s Bar()
    	    `,
				firstKind.Keyword(),
				secondKind.Keyword(),
				annotation,
				transferOperator,
			))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if firstKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			if secondKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := expectCheckerErrors(err, expectedErrorCount)

			if firstKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindStructure {

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			} else if firstKind != common.CompositeKindStructure ||
				secondKind != common.CompositeKindStructure {

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}

			Expect(errs[expectedErrorCount-1]).
				To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		}
	}
}

func TestCheckInvalidCompositeFunctionWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Foo {
              fun test(self: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Foo {
              init(self: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializesConstant(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {
              let foo: Int

              init() {
                  self.foo = 42
              }
          }

	      let test %[2]s Test()
	    `, kind.Keyword(), transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializerWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {

              init(x: Int) {}
          }

	      let test %[2]s Test(x: 1)
	    `, kind.Keyword(), transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s Test {

              init(x: Int) {}
          }

	      let test %[2]s Test(1)

	    `, kind.Keyword(), transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		}
	}
}

func TestCheckCompositeFunctionWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {

              fun test(x: Int) {}
          }

	      let test = Test().test(x: 1)
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s Test {

              fun test(x: Int) {}
          }

	      let test = Test().test(1)
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		}
	}
}

func TestCheckCompositeConstructorReferenceInInitializerAndFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)

		checker, err := parseAndCheck(fmt.Sprintf(`
          %s Test {

              init() {
                  Test
              }

              fun test(): %[2]sTest {
                  return Test()
              }
          }

          fun test(): %[2]sTest {
              return Test()
          }

          fun test2(): %[2]sTest {
              return Test().test()
          }
        `, kind.Keyword(), annotation))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))

			testType := checker.FindType("Test")

			Expect(testType).
				To(BeAssignableToTypeOf(&sema.CompositeType{}))

			structureType := testType.(*sema.CompositeType)

			Expect(structureType.Identifier).
				To(Equal("Test"))

			testFunctionMember := structureType.Members["test"]

			Expect(testFunctionMember.Type).
				To(BeAssignableToTypeOf(&sema.FunctionType{}))

			testFunctionType := testFunctionMember.Type.(*sema.FunctionType)

			Expect(testFunctionType.ReturnTypeAnnotation.Type).
				To(BeIdenticalTo(structureType))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func getCompositeKindAnnotation(kind common.CompositeKind) string {
	if kind != common.CompositeKindResource {
		return ""
	}
	return "<-"
}

func getCompositeKindTransferOperator(kind common.CompositeKind) string {
	if kind != common.CompositeKindResource {
		return "="
	}
	return "<-"
}

func TestCheckInvalidCompositeFieldMissingVariableKind(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s X {
              x: Int

              init(x: Int) {
                  self.x = x
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidVariableKindError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
            %[1]s X {
                fun foo(): ((): %[2]sX) {
                    return self.bar
                }

                fun bar(): %[2]sX {
                    return self
                }
            }
	    `, kind.Keyword(), annotation))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckFunctionConditions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              x != 0
          }
          post {
              x == 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPreConditionReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              y == 0
          }
          post {
              z == 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("y"))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[1].(*sema.NotDeclaredError).Name).
		To(Equal("z"))
}

func TestCheckInvalidFunctionNonBoolCondition(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              1
          }
          post {
              2
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          post {
              before(x) != 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithBeforeAndNoArgument(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          post {
              before() != 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))

}

func TestCheckInvalidFunctionPreConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          pre {
              before(x) != 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("before"))
}

func TestCheckInvalidFunctionWithBeforeVariableAndPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          post {
              before(x) == 0
          }
          let before = 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

func TestCheckFunctionWithBeforeVariable(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int) {
          let before = 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostCondition(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPreConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          pre {
              result == 0
          }
          return 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckInvalidFunctionPostConditionWithResultWrongType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              result == true
          }
          return 0
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))
}

func TestCheckFunctionPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              result == 0
          }
          return 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          post {
              result == 0
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
	Expect(errs[0].(*sema.NotDeclaredError).Name).
		To(Equal("result"))
}

func TestCheckFunctionWithoutReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionWithoutReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(result: Int) {
          post {
              result == 0
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionWithReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {
          post {
              result == 2
          }
          let result = 1
          return result * 2
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RedeclarationError{}))
}

// TODO: should this be invalid?
func TestCheckFunctionWithReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(result: Int): Int {
          post {
              result == 2
          }
          return result * 2
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          post {
              (fun (): Int { return 2 })() == 2
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.FunctionExpressionInConditionError{}))
}

func TestCheckFunctionPostConditionWithMessageUsingStringLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          post {
             1 == 2: "nope"
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidFunctionPostConditionWithMessageUsingBooleanLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          post {
             1 == 2: true
          }
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionPostConditionWithMessageUsingResult(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): String {
          post {
             1 == 2: result
          }
          return ""
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostConditionWithMessageUsingBefore(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: String) {
          post {
             1 == 2: before(x)
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckFunctionPostConditionWithMessageUsingParameter(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: String) {
          post {
             1 == 2: x
          }
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckMutuallyRecursiveFunctions(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun isEven(_ n: Int): Bool {
          if n == 0 {
              return true
          }
          return isOdd(n - 1)
      }

      fun isOdd(_ n: Int): Bool {
          if n == 0 {
              return false
          }
          return isEven(n - 1)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckCompositeReferenceBeforeDeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          var tests = 0

          fun test(): %sTest {
              return Test()
          }

          %s Test {
             init() {
                 tests = tests + 1
             }
          }
        `, annotation, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckNever(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheckWithExtra(
		`
            fun test(): Int {
                return panic("XXX")
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int? = 1
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int? = false
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckOptionalNesting(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int?? = 1
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int? = nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNestingNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int?? = nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     fun test(): Int?? {
         return nil
     }
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNonOptionalNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int = nil
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilCoalescingNilIntToOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let one = 1
      let none: Int? = nil
      let x: Int? = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilIntToOptionals(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let one = 1
      let none: Int?? = nil
      let x: Int? = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilIntToOptionalNilLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let one = 1
      let x: Int? = nil ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingMismatch(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int? = nil ?? false
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilCoalescingRightSubtype(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int? = nil ?? nil
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let one = 1
      let none: Int? = nil
      let x: Int = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingOptionalsInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let one = 1
      let none: Int?? = nil
      let x: Int = none ?? one
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilCoalescingNilLiteralInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let one = 1
     let x: Int = nil ?? one
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingMismatchNonOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int = nil ?? false
   `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidNilCoalescingRightSubtype(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int = nil ?? nil
   `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidNilCoalescingNonMatchingTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int? = 1
      let y = x ?? false
   `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}))
}

func TestCheckNilCoalescingAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Any? = 1
     let y = x ?? false
  `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingOptionalRightHandSide(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
     let x: Int? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["z"].Type).
		To(BeAssignableToTypeOf(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckNilCoalescingBothOptional(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
     let x: Int?? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["z"].Type).
		To(BeAssignableToTypeOf(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckNilsComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x = nil == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int? = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNonOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNonOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int?? = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int? = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int?? = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingWithNever(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheckWithExtra(
		`
          let x: Int? = nil
          let y = x ?? panic("nope")
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int? = nil
     let y: Int?? = nil
     let z = x == y
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNestedOptionalComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
     let x: Int? = nil
     let y: Bool?? = nil
     let z = x == y
   `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))
}

func TestCheckInvalidNonOptionalReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(x: Int?): Int {
          return x
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidLocalInterface(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          fun test() {
              %s interface Test {}
          }
        `, kind.Keyword()))

		errs := expectCheckerErrors(err, 1)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidDeclarationError{}))
	}
}

func TestCheckInterfaceWithFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              fun test()
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceWithFunctionImplementationAndConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              fun test(x: Int) {
                  pre {
                    x == 0
                  }
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              fun test(): Int {
                 return 1
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementationNoConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              fun test() {
                // ...
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceWithInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              init()
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceWithInitializerImplementation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              init() {
                // ...
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}
		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceWithInitializerImplementationAndConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {
              init(x: Int) {
                  pre {
                    x == 0
                  }
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConstructorCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s interface Test {}

          let test = Test()
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.NotCallableError{}))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.NotCallableError{}))
		}
	}
}

func TestCheckInterfaceUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheckWithExtra(
			fmt.Sprintf(`
              %[1]s interface Test {}

              let test: %[2]sTest %[3]s panic("")
            `,
				kind.Keyword(),
				annotation,
				transferOperator,
			),
			stdlib.StandardLibraryFunctions{
				stdlib.PanicFunction,
			}.ToValueDeclarations(),
			nil,
			nil,
		)

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceConformanceNoRequirements(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {}

          %[1]s TestImpl: Test {}

          let test: %[2]sTest %[3]s TestImpl()
	    `,
			kind.Keyword(),
			annotation,
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// only test incompatible combinations
			if firstKind == secondKind {
				continue
			}

			annotation := getCompositeKindAnnotation(firstKind)
			transferOperator := getCompositeKindTransferOperator(firstKind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {}

              %[2]s TestImpl: Test {}

              let test: %[3]sTest %[4]s TestImpl()
	        `,
				firstKind.Keyword(),
				secondKind.Keyword(),
				annotation,
				transferOperator,
			))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if firstKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			if secondKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			_ = expectCheckerErrors(err, expectedErrorCount)
			//
			//	Expect(errs[0]).
			//		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			//
			//	Expect(errs[1]).
			//		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {}

          // NOTE: not declaring conformance
          %[1]s TestImpl {}

          let test: %[2]sTest %[3]s TestImpl()
	    `,
			kind.Keyword(),
			annotation,
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
		}
	}
}

func TestCheckInvalidCompositeInterfaceConformanceNonInterface(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s TestImpl: Int {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidConformanceError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceFieldUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              x: Int
          }

          %[1]s TestImpl: Test {
              var x: Int

              init(x: Int) {
                  self.x = x
              }
          }

          let test: %[2]sTest %[3]s TestImpl(x: 1)

          let x = test.x

        `, kind.Keyword(), annotation, transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {}

          %[1]s TestImpl: Test {
              var x: Int

              init(x: Int) {
                  self.x = x
              }
          }

          let test: %[2]sTest %[3]s TestImpl(x: 1)

          let x = test.x
    	`,
			kind.Keyword(),
			annotation,
			transferOperator,
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		}
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              fun test(): Int
          }

          %[1]s TestImpl: Test {
              fun test(): Int {
                  return 2
              }
          }

          let test: %[2]sTest %[3]s TestImpl()

          let val = test.test()
	    `, kind.Keyword(), annotation, transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {}

          %[1]s TestImpl: Test {
              fun test(): Int {
                  return 2
              }
          }

          let test: %[2]sTest %[3]s TestImpl()

          let val = test.test()
	    `, kind.Keyword(), annotation, transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceInitializerExplicitMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              init(x: Int)
          }

          %[1]s TestImpl: Test {
              init(x: Bool) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceInitializerImplicitMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              init(x: Int)
          }

          %[1]s TestImpl: Test {
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceMissingFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              fun test(): Int
          }

          %[1]s TestImpl: Test {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceFunctionMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              fun test(): Int
          }

          %[1]s TestImpl: Test {
              fun test(): Bool {
                  return true
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {

			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceMissingField(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
               x: Int
          }

          %[1]s TestImpl: Test {}

	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceFieldTypeMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              x: Int
          }

          %[1]s TestImpl: Test {
              var x: Bool
              init(x: Bool) {
                 self.x = x
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {

			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceKindFieldFunctionMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              x: Bool
          }

          %[1]s TestImpl: Test {
              fun x(): Bool {
                  return true
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {

			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceKindFunctionFieldMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              fun x(): Bool
          }

          %[1]s TestImpl: Test {
              var x: Bool

              init(x: Bool) {
                 self.x = x
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {

			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindLetVarMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              let x: Bool
          }

          %[1]s TestImpl: Test {
              var x: Bool

              init(x: Bool) {
                 self.x = x
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindVarLetMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface Test {
              var x: Bool
          }

          %[1]s TestImpl: Test {
              let x: Bool

              init(x: Bool) {
                 self.x = x
              }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))
		} else {

			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.ConformanceError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidInterfaceConformanceRepetition(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface X {}

          %[1]s interface Y {}

          %[1]s TestImpl: X, Y, X {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 3
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.DuplicateConformanceError{}))

		if kind != common.CompositeKindStructure {

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.DuplicateConformanceError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceTypeAsValue(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		checker, err := parseAndCheck(fmt.Sprintf(`
          %s interface X {}

          let x = X
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))

			Expect(checker.GlobalValues["x"].Type).
				To(BeAssignableToTypeOf(&sema.InterfaceMetaType{}))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {
			annotation := getCompositeKindAnnotation(firstKind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s S {}

              %s interface I {
                  s: %sS
              }
	        `, firstKind.Keyword(), secondKind.Keyword(), annotation))

			expectedErrorCount := 0
			if firstKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			if secondKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			if expectedErrorCount == 0 {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := expectCheckerErrors(err, expectedErrorCount)

				for i := 0; i < expectedErrorCount; i += 1 {
					Expect(errs[i]).
						To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
				}
			}
		}
	}
}

func TestCheckInterfaceWithFunctionHavingStructType(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {
			annotation := getCompositeKindAnnotation(firstKind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s S {}

              %s interface I {
                  fun s(): %sS
              }
	        `, firstKind.Keyword(), secondKind.Keyword(), annotation))

			expectedErrorCount := 0
			if firstKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			if secondKind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			if expectedErrorCount == 0 {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := expectCheckerErrors(err, expectedErrorCount)

				for i := 0; i < expectedErrorCount; i += 1 {
					Expect(errs[i]).
						To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
				}
			}
		}
	}
}

func TestCheckOccurrencesVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
        let x = 1
        var y = x
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	occurrences := checker.Occurrences.All()

	matchers := []*occurrenceMatcher{
		{
			startPos:        sema.Position{Line: 2, Column: 12},
			endPos:          sema.Position{Line: 2, Column: 12},
			originStartPos:  &sema.Position{Line: 2, Column: 12},
			originEndPos:    &sema.Position{Line: 2, Column: 12},
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 3, Column: 12},
			endPos:          sema.Position{Line: 3, Column: 12},
			originStartPos:  &sema.Position{Line: 3, Column: 12},
			originEndPos:    &sema.Position{Line: 3, Column: 12},
			declarationKind: common.DeclarationKindVariable,
		},
		{
			startPos:        sema.Position{Line: 3, Column: 16},
			endPos:          sema.Position{Line: 3, Column: 16},
			originStartPos:  &sema.Position{Line: 2, Column: 12},
			originEndPos:    &sema.Position{Line: 2, Column: 12},
			declarationKind: common.DeclarationKindConstant,
		},
	}

	ms := make([]interface{}, len(matchers))
	for i := range matchers {
		ms[i] = matchers[i]
	}

	Expect(occurrences).
		To(ConsistOf(ms...))

	for _, matcher := range matchers {
		Expect(checker.Occurrences.Find(matcher.startPos)).To(Not(BeNil()))
		Expect(checker.Occurrences.Find(matcher.endPos)).To(Not(BeNil()))
	}
}

func TestCheckOccurrencesFunction(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
		fun f1(paramX: Int, paramY: Bool) {
		   let x = 1
		   var y: Int? = x
		   fun f2() {
		       if let y = y {
		       }
		   }
           f1(paramX: 1, paramY: true)
		}

        fun f3() {
            f1(paramX: 2, paramY: false)
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	occurrences := checker.Occurrences.All()

	matchers := []*occurrenceMatcher{
		{
			startPos:        sema.Position{Line: 2, Column: 6},
			endPos:          sema.Position{Line: 2, Column: 7},
			originStartPos:  &sema.Position{Line: 2, Column: 6},
			originEndPos:    &sema.Position{Line: 2, Column: 6},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 2, Column: 9},
			endPos:          sema.Position{Line: 2, Column: 14},
			originStartPos:  &sema.Position{Line: 2, Column: 9},
			originEndPos:    &sema.Position{Line: 2, Column: 9},
			declarationKind: common.DeclarationKindParameter,
		},
		{
			startPos:        sema.Position{Line: 2, Column: 22},
			endPos:          sema.Position{Line: 2, Column: 27},
			originStartPos:  &sema.Position{Line: 2, Column: 22},
			originEndPos:    &sema.Position{Line: 2, Column: 22},
			declarationKind: common.DeclarationKindParameter,
		},
		{
			startPos:        sema.Position{Line: 3, Column: 9},
			endPos:          sema.Position{Line: 3, Column: 9},
			originStartPos:  &sema.Position{Line: 3, Column: 9},
			originEndPos:    &sema.Position{Line: 3, Column: 9},
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 4, Column: 9},
			endPos:          sema.Position{Line: 4, Column: 9},
			originStartPos:  &sema.Position{Line: 4, Column: 9},
			originEndPos:    &sema.Position{Line: 4, Column: 9},
			declarationKind: common.DeclarationKindVariable,
		},
		{
			startPos:        sema.Position{Line: 4, Column: 19},
			endPos:          sema.Position{Line: 4, Column: 19},
			originStartPos:  &sema.Position{Line: 3, Column: 9},
			originEndPos:    &sema.Position{Line: 3, Column: 9},
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 5, Column: 9},
			endPos:          sema.Position{Line: 5, Column: 10},
			originStartPos:  &sema.Position{Line: 5, Column: 9},
			originEndPos:    &sema.Position{Line: 5, Column: 9},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 6, Column: 16},
			endPos:          sema.Position{Line: 6, Column: 16},
			originStartPos:  &sema.Position{Line: 6, Column: 16},
			originEndPos:    &sema.Position{Line: 6, Column: 16},
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 6, Column: 20},
			endPos:          sema.Position{Line: 6, Column: 20},
			originStartPos:  &sema.Position{Line: 4, Column: 9},
			originEndPos:    &sema.Position{Line: 4, Column: 9},
			declarationKind: common.DeclarationKindVariable,
		},
		{
			startPos:        sema.Position{Line: 9, Column: 11},
			endPos:          sema.Position{Line: 9, Column: 12},
			originStartPos:  &sema.Position{Line: 2, Column: 6},
			originEndPos:    &sema.Position{Line: 2, Column: 6},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 12, Column: 12},
			endPos:          sema.Position{Line: 12, Column: 13},
			originStartPos:  &sema.Position{Line: 12, Column: 12},
			originEndPos:    &sema.Position{Line: 12, Column: 12},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 13, Column: 12},
			endPos:          sema.Position{Line: 13, Column: 13},
			originStartPos:  &sema.Position{Line: 2, Column: 6},
			originEndPos:    &sema.Position{Line: 2, Column: 6},
			declarationKind: common.DeclarationKindFunction,
		},
	}

	ms := make([]interface{}, len(matchers))
	for i := range matchers {
		ms[i] = matchers[i]
	}

	Expect(occurrences).
		To(ConsistOf(ms...))

	for _, matcher := range matchers {
		Expect(checker.Occurrences.Find(matcher.startPos)).To(Not(BeNil()))
		Expect(checker.Occurrences.Find(matcher.endPos)).To(Not(BeNil()))
	}
}

// TODO: implement occurrences for type references
//  (e.g. conformances, conditional casting expression)

func TestCheckOccurrencesStructAndInterface(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
		struct interface I1 {}

	    struct S1: I1 {
	       let x: Int
	       init() {
	          self.x = 1
	          self.test()
	       }
	       fun test() {}
	    }

	    fun f(): S1 {
	       return S1()
	    }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	occurrences := checker.Occurrences.All()

	matchers := []*occurrenceMatcher{
		{
			startPos:        sema.Position{Line: 2, Column: 19},
			endPos:          sema.Position{Line: 2, Column: 20},
			originStartPos:  &sema.Position{Line: 2, Column: 19},
			originEndPos:    &sema.Position{Line: 2, Column: 19},
			declarationKind: common.DeclarationKindStructureInterface,
		},
		{
			startPos:        sema.Position{Line: 4, Column: 12},
			endPos:          sema.Position{Line: 4, Column: 13},
			originStartPos:  &sema.Position{Line: 4, Column: 12},
			originEndPos:    &sema.Position{Line: 4, Column: 12},
			declarationKind: common.DeclarationKindStructure,
		},
		{
			startPos:        sema.Position{Line: 5, Column: 8},
			endPos:          sema.Position{Line: 5, Column: 17},
			originStartPos:  &sema.Position{Line: 5, Column: 12},
			originEndPos:    &sema.Position{Line: 5, Column: 12},
			declarationKind: common.DeclarationKindField,
		},
		// self
		{
			startPos:        sema.Position{Line: 7, Column: 11},
			endPos:          sema.Position{Line: 7, Column: 14},
			originStartPos:  nil,
			originEndPos:    nil,
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 7, Column: 16},
			endPos:          sema.Position{Line: 7, Column: 16},
			originStartPos:  &sema.Position{Line: 5, Column: 12},
			originEndPos:    &sema.Position{Line: 5, Column: 12},
			declarationKind: common.DeclarationKindField,
		},
		// self
		{
			startPos:        sema.Position{Line: 8, Column: 11},
			endPos:          sema.Position{Line: 8, Column: 14},
			originStartPos:  nil,
			originEndPos:    nil,
			declarationKind: common.DeclarationKindConstant,
		},
		{
			startPos:        sema.Position{Line: 8, Column: 16},
			endPos:          sema.Position{Line: 8, Column: 19},
			originStartPos:  &sema.Position{Line: 10, Column: 12},
			originEndPos:    &sema.Position{Line: 10, Column: 15},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 10, Column: 12},
			endPos:          sema.Position{Line: 10, Column: 15},
			originStartPos:  &sema.Position{Line: 10, Column: 12},
			originEndPos:    &sema.Position{Line: 10, Column: 12},
			declarationKind: common.DeclarationKindFunction,
		},
		// TODO: why the duplicate?
		{
			startPos:        sema.Position{Line: 10, Column: 12},
			endPos:          sema.Position{Line: 10, Column: 15},
			originStartPos:  &sema.Position{Line: 10, Column: 12},
			originEndPos:    &sema.Position{Line: 10, Column: 15},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 13, Column: 9},
			endPos:          sema.Position{Line: 13, Column: 9},
			originStartPos:  &sema.Position{Line: 13, Column: 9},
			originEndPos:    &sema.Position{Line: 13, Column: 9},
			declarationKind: common.DeclarationKindFunction,
		},
		{
			startPos:        sema.Position{Line: 14, Column: 15},
			endPos:          sema.Position{Line: 14, Column: 16},
			originStartPos:  &sema.Position{Line: 4, Column: 12},
			originEndPos:    &sema.Position{Line: 4, Column: 12},
			declarationKind: common.DeclarationKindFunction,
		},
	}

	ms := make([]interface{}, len(matchers))
	for i := range matchers {
		ms[i] = matchers[i]
	}

	Expect(occurrences).
		To(ConsistOf(ms...))

	for _, matcher := range matchers {
		Expect(checker.Occurrences.Find(matcher.startPos)).To(Not(BeNil()))
		Expect(checker.Occurrences.Find(matcher.endPos)).To(Not(BeNil()))
	}
}

func TestCheckInvalidImport(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
       import "unknown"
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnresolvedImportError{}))
}

func TestCheckInvalidRepeatedImport(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheckWithExtra(
		`
           import "unknown"
           import "unknown"
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return &ast.Program{}, nil
		},
	)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RepeatedImportError{}))
}

func TestCheckImportAll(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
	   fun answer(): Int {
	       return 42
		}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = parseAndCheckWithExtra(
		`
           import "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidImportUnexported(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
       let x = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = parseAndCheckWithExtra(
		`
           import answer from "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotExportedError{}))
}

func TestCheckImportSome(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
	   fun answer(): Int {
	       return 42
       }

       let x = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = parseAndCheckWithExtra(
		`
           import answer from "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidImportedError(t *testing.T) {
	RegisterTestingT(t)

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       let x: Bool = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = parseAndCheckWithExtra(
		`
           import x from "imported"
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return imported, nil
		},
	)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ImportedProgramError{}))
}

func TestCheckImportTypes(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		checker, err := parseAndCheck(fmt.Sprintf(`
	       %s Test {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}

		_, err = parseAndCheckWithExtra(
			`
               import "imported"

               let x: Test = Test()
            `,
			nil,
			nil,
			func(location ast.ImportLocation) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		)

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ImportedProgramError{}))
		}

	}
}

func TestCheckDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z = {"a": 1, "b": 2}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckDictionaryType(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z: {String: Int} = {"a": 1, "b": 2}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryTypeKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z: {Int: Int} = {"a": 1, "b": 2}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryTypeValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z: {String: String} = {"a": 1, "b": 2}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryTypeSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z: {Int: String} = {"a": 1, "b": 2}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryKeys(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z = {"a": 1, true: 2}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryValues(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let z = {"a": 1, "b": true}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionaryIndexingString(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
      let x = {"abc": 1, "def": 2}
      let y = x["abc"]
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckDictionaryIndexingBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = {true: 1, false: 2}
      let y = x[true]
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = {"abc": 1, "def": 2}
      let y = x[true]
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckDictionaryIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = 3
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = true
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionaryRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x.remove(key: "abc")
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x.remove(key: true)
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFailableDowncastingAny(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
      let x: Any = 1
      let y: Int? = x as? Int
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.FailableDowncastingTypes).
		To(Not(BeEmpty()))
}

func TestCheckInvalidFailableDowncastingAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Any = 1
      let y: Bool? = x as? Int
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

// TODO: add support for statically known casts
func TestCheckInvalidFailableDowncastingStaticallyKnown(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Int = 1
      let y: Int? = x as? Int
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for interfaces
// TODO: add test this is *INVALID* for resources
func TestCheckInvalidFailableDowncastingInterface(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct interface I {}

      struct S: I {}

      let x: I = S()
      let y: S? = x as? S
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableDowncastingOptionalAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Any? = 1
      let y: Int?? = x as? Int?
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableDowncastingArrayAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: [Any] = [1]
      let y: [Int]? = x as? [Int]
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

func TestCheckOptionalAnyFailableDowncastingNil(t *testing.T) {
	RegisterTestingT(t)

	checker, err := parseAndCheck(`
      let x: Any? = nil
      let y = x ?? 23
      let z = y as? Int
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.AnyType{}}))

	// TODO: record result type of conditional and box to any in interpreter
	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.AnyType{}))

	Expect(checker.GlobalValues["z"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.IntType{}}))
}

// TODO: return common super type for conditional
func TestCheckInvalidAnyConditional(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x: Any = true
      let y = true ? 1 : x
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckLength(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayAppend(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayAppend(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append("4")
          return x
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayAppendBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
	  fun test(): [Int] {
	 	  let a = [1, 2]
		  let b = [3, 4]
          let c = a.concat(b)
          return c
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = ["a", "b"]
          let c = a.concat(b)
          return c
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayConcatBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = [3, 4]
		  let c = a.concat
		  return c(b)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayInsert(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayInsert(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, "4")
          return x
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.remove(at: 1)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.remove(at: "1")
          return x
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayRemoveFirst(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeFirst()
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayRemoveFirst(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeFirst(1)
          return x
      }
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckArrayRemoveLast(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeLast()
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayContains(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains(2)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayContains(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains("abc")
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidArrayContainsNotEquatable(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Bool {
          let z = [[1], [2], [3]]
          return z.contains([1, 2])
      }
    `)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotEquatableTypeError{}))
}

func TestCheckEmptyArray(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let xs: [Int] = []
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyArrayCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo(xs: [Int]) {
          foo(xs: [])
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let xs: {String: Int} = {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyDictionaryCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun foo(xs: {String: Int}) {
          foo(xs: {})
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArraySubtyping(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface I {}
          %[1]s S: I {}

          let xs: %[2]s[S] %[3]s []
          let ys: %[2]s[I] %[3]s xs
	    `, kind.Keyword(), annotation, transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidArraySubtyping(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let xs: [Bool] = []
      let ys: [Int] = xs
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionarySubtyping(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		annotation := getCompositeKindAnnotation(kind)
		transferOperator := getCompositeKindTransferOperator(kind)

		_, err := parseAndCheck(fmt.Sprintf(`
          %[1]s interface I {}
          %[1]s S: I {}

          let xs: %[2]s{String: S} %[3]s {}
          let ys: %[2]s{String: I} %[3]s xs
	    `, kind.Keyword(), annotation, transferOperator))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := expectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidDictionarySubtyping(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      let xs: {String: Bool} = {}
      let ys: {String: Int} = xs
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckUnaryMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      resource X {}

      fun foo(x: <-X): <-X {
          return x
      }

      var x <- foo(x: <-X())

      fun bar() {
          x <- X()
      }
	`)

	// TODO: add support for resources

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

}

func TestCheckImmediateDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      resource X {}

      fun test() {
          destroy X()
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckIndirectDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      resource X {}

      fun test() {
          let x <- X()
          destroy x
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      struct X {}

      fun test() {
          destroy X()
      }
	`)

	// TODO: add support for resources

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidDestructionError{}))
}

func TestCheckUnaryCreateAndDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      resource X {}

      fun test() {
          var x = create X()
          destroy x
      }
	`)

	// TODO: add support for resources
	// TODO: add support for create expressions
	// TODO: add support for destroy expressions
	// TODO: use moving transfer operator once create expression is supported

	errs := expectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedExpressionError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.InvalidDestructionError{}))
}

func TestCheckInvalidCompositeInitializerOverloading(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := parseAndCheck(fmt.Sprintf(`
          %s X {
              init() {}
              init(y: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := expectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.UnsupportedOverloadingError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckAssertWithoutMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheckWithExtra(
		`
            fun test() {
                assert(1 == 2)
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAssertWithMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheckWithExtra(
		`
            fun test() {
                assert(1 == 2, message: "test message")
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

// TODO: add support for nested composite declarations

func TestCheckInvalidNestedCompositeDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      contract TestContract {
          resource TestResource {}
      }
    `)

	errs := expectCheckerErrors(err, 2)

	// TODO: add support for contracts

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	// TODO: add support for nested composite declarations

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

}

func TestCheckInvalidNestedInterfaceDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      contract interface TestContract {
          resource TestResource {}
      }
    `)

	errs := expectCheckerErrors(err, 2)

	// TODO: add support for contracts

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	// TODO: add support for nested composite declarations

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckMissingReturnStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      fun test(): Int {}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingReturnStatementError{}))
}

func TestCheckMissingReturnStatementInterfaceFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
      	struct interface Test {
			fun test(x: Int): Int {
				pre {
					x != 0
				}
			}
		}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckMissingReturnStatementStructFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseAndCheck(`
		struct Test {
			pub(set) var foo: Int

			init(foo: Int) {
				self.foo = foo
			}

			pub fun getFoo(): Int {
				if 2 > 1 {
					return 0
				}
			}
		}
	`)

	errs := expectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingReturnStatementError{}))
}

func TestCheckFunctionDeclarationParameterWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(): <-T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(): T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckVariableDeclarationWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: <-T %[2]s T()
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckVariableDeclarationWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: T %[2]s T()
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFieldDeclarationWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: <-T
                  init(t: <-T) {
                      self.t %[2]s t
                  }
              }
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 4)

				// NOTE: one invalid move annotation error for field, one for parameter

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 2)

				// NOTE: one invalid move annotation error for field, one for parameter

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFieldDeclarationWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: T
                  init(t: T) {
                      self.t %[2]s t
                  }
              }
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				// NOTE: one missing move annotation error for field, one for parameter

				errs := expectCheckerErrors(err, 4)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (): <-T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (): T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((<-T): Void) = fun (r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((T): Void) = fun (r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((): <-T) = fun (): <-T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := parseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((): T) = fun (): T {
                  return T()
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFailableDowncastingWithMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test %[2]s T() as? <-T
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindStructure:

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
			}
		})
	}
}

func TestCheckFailableDowncastingWithoutMoveAnnotation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			transferOperator := getCompositeKindTransferOperator(kind)

			_, err := parseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test %[2]s T() as? T
	        `,
				kind.Keyword(),
				transferOperator,
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := expectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := expectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindStructure:

				// TODO: add support for non-Any types in failable downcasting

				errs := expectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
			}
		})
	}
}


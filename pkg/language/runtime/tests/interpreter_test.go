package tests

import (
	"fmt"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/trampoline"
)

func parseCheckAndInterpret(t *testing.T, code string) *interpreter.Interpreter {
	return parseCheckAndInterpretWithExtra(t, code, nil, nil, nil)
}

func parseCheckAndInterpretWithExtra(
	t *testing.T,
	code string,
	predefinedValueTypes map[string]sema.ValueDeclaration,
	predefinedValues map[string]interpreter.Value,
	handleCheckerError func(error),
) *interpreter.Interpreter {

	checker, err := ParseAndCheckWithExtra(t, code, predefinedValueTypes, nil, nil)

	if handleCheckerError != nil {
		handleCheckerError(err)
	} else {
		require.Nil(t, err)
	}

	inter, err := interpreter.NewInterpreter(checker, predefinedValues)

	require.Nil(t, err)

	err = inter.Interpret()

	require.Nil(t, err)

	return inter
}

func TestInterpretConstantAndVariableDeclarations(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        let x = 1
        let y = true
        let z = 1 + 2
        var a = 3 == 3
        var b = [1, 2]
        let s = "123"
    `)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(3),
		inter.Globals["z"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
		),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NewStringValue("123"),
		inter.Globals["s"].Value,
	)
}

func TestInterpretDeclarations(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun test(): Int {
            return 42
        }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(42),
		value,
	)
}

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {

	inter := parseCheckAndInterpret(t, ``)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.NotDeclaredError{}, err)
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let test = 1
   `)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.NotInvokableError{}, err)
}

func TestInterpretLexicalScope(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 10

       fun f(): Int {
          // check resolution
          return x
       }

       fun g(): Int {
          // check scope is lexical, not dynamic
          let x = 20
          return f()
       }
	`)

	assert.Equal(t,
		interpreter.NewIntValue(10),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("f")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(10),
		value,
	)

	value, err = inter.Invoke("g")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(10),
		value,
	)
}

func TestInterpretFunctionSideEffects(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       var value = 0

       fun test(_ newValue: Int) {
           value = newValue
       }
	`)

	newValue := big.NewInt(42)

	value, err := inter.Invoke("test", newValue)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.IntValue{Int: newValue},
		inter.Globals["value"].Value,
	)
}

func TestInterpretNoHoisting(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 2

       fun test(): Int {
          if x == 0 {
              let x = 3
              return x
          }
          return x
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
	`)

	assert.Equal(t,
		interpreter.NewIntValue(10),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(42),
		inter.Globals["y"].Value,
	)
}

func TestInterpretVariableAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 2
           x = 3
           return x
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       var x = 2

       fun test(): Int {
           x = 3
           return x
       }
	`)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(3),
		inter.Globals["x"].Value,
	)
}

func TestInterpretConstantRedeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 2

       fun test(): Int {
           let x = 3
           return x
       }
	`)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

func TestInterpretParameters(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun returnA(a: Int, b: Int): Int {
           return a
       }

       fun returnB(a: Int, b: Int): Int {
           return b
       }
	`)

	value, err := inter.Invoke("returnA", big.NewInt(24), big.NewInt(42))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(24),
		value,
	)

	value, err = inter.Invoke("returnB", big.NewInt(24), big.NewInt(42))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(42),
		value,
	)
}

func TestInterpretArrayIndexing(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           let z = [0, 3]
           return z[1]
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           let z = [0, 3]
           z[1] = 2
           return z[1]
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)
}

func TestInterpretStringIndexing(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
	  let a = "abc"
	  let x = a[0]
	  let y = a[1]
	  let z = a[2]
	`)

	assert.Equal(t,
		interpreter.NewStringValue("a"),
		inter.Globals["x"].Value,
	)
	assert.Equal(t,
		interpreter.NewStringValue("b"),
		inter.Globals["y"].Value,
	)
	assert.Equal(t,
		interpreter.NewStringValue("c"),
		inter.Globals["z"].Value,
	)
}

func TestInterpretStringIndexingUnicode(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
	  fun testUnicodeA(): Character {
		  let a = "caf\u{E9}"
		  return a[3]
	  }

	  fun testUnicodeB(): Character {
		let b = "cafe\u{301}"
		return b[3]
	  }
	`)

	value, err := inter.Invoke("testUnicodeA")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("\u00e9"),
		value,
	)

	value, err = inter.Invoke("testUnicodeB")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("e\u0301"),
		value,
	)
}

func TestInterpretStringIndexingAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
		  let z = "abc"
		  let y: Character = "d"
		  z[0] = y
		  return z
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		value,
		interpreter.NewStringValue("dbc"),
	)
}

func TestInterpretStringIndexingAssignmentUnicode(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
		  let z = "cafe chair"
		  let y: Character = "e\u{301}"
		  z[3] = y
		  return z
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("cafe\u0301 chair"),
		value,
	)
}

func TestInterpretStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let z = "abc"
		  z[0] = "d"
		  z[1] = "e"
		  z[2] = "f"
		  return z
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("def"),
		value,
	)
}

type stringSliceTest struct {
	str           string
	from          int
	to            int
	result        string
	expectedError error
}

func TestInterpretStringSlicing(t *testing.T) {
	tests := []stringSliceTest{
		{"abcdef", 0, 6, "abcdef", nil},
		{"abcdef", 0, 0, "", nil},
		{"abcdef", 0, 1, "a", nil},
		{"abcdef", 0, 2, "ab", nil},
		{"abcdef", 1, 2, "b", nil},
		{"abcdef", 2, 3, "c", nil},
		{"abcdef", 5, 6, "f", nil},
		// TODO: check invalid arguments
		// {"abcdef", -1, 0, "", &InvalidIndexError}
		// },
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(`
				fun test(): String {
				  let s = "%s"
				  return s.slice(from: %d, upTo: %d)
				}
			`, test.str, test.from, test.to))

			value, err := inter.Invoke("test")
			assert.IsType(t, test.expectedError, err)
			assert.Equal(t,
				interpreter.NewStringValue(test.result),
				value,
			)
		})
	}
}

func TestInterpretReturnWithoutExpression(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun returnNothing() {
           return
       }
	`)

	value, err := inter.Invoke("returnNothing")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretReturns(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun returnEarly(): Int {
           return 2
           return 1
       }
	`)

	value, err := inter.Invoke("returnEarly")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)
}

// TODO: perform each operator test for each integer type

func TestInterpretPlusOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 2 + 4
	`)

	assert.Equal(t,
		interpreter.NewIntValue(6),
		inter.Globals["x"].Value,
	)
}

func TestInterpretMinusOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 2 - 4
	`)

	assert.Equal(t,
		interpreter.NewIntValue(-2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretMulOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 2 * 4
	`)

	assert.Equal(t,
		interpreter.NewIntValue(8),
		inter.Globals["x"].Value,
	)
}

func TestInterpretDivOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 7 / 3
	`)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretModOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       let x = 5 % 3
	`)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretConcatOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
		let a = "abc" & "def"
		let b = "" & "def"
		let c = "abc" & ""
		let d = "" & ""

		let e = [1, 2] & [3, 4]
		// TODO: support empty arrays
		// let f = [1, 2] & []
		// let g = [] & [3, 4]
		// let h = [] & []
	`)

	assert.Equal(t,
		interpreter.NewStringValue("abcdef"),
		inter.Globals["a"].Value,
	)
	assert.Equal(t,
		interpreter.NewStringValue("def"),
		inter.Globals["b"].Value,
	)
	assert.Equal(t,
		interpreter.NewStringValue("abc"),
		inter.Globals["c"].Value,
	)
	assert.Equal(t,
		interpreter.NewStringValue(""),
		inter.Globals["d"].Value,
	)

	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(4),
		),
		inter.Globals["e"].Value,
	)

	// TODO: support empty arrays
	// Expect(inter.Globals["f"].Value).
	// 	To(Equal(interpreter.ArrayValue{
	// 		Values: &[]interpreter.Value{
	// 			interpreter.NewIntValue(1),
	// 			interpreter.NewIntValue(2),
	// 		},
	// 	}))
	// Expect(inter.Globals["g"].Value).
	// 	To(Equal(interpreter.ArrayValue{
	// 		Values: &[]interpreter.Value{
	// 			interpreter.NewIntValue(3),
	// 			interpreter.NewIntValue(4),
	// 		},
	// 	}))
	// Expect(inter.Globals["h"].Value).
	// 	To(Equal(interpreter.ArrayValue{
	// 		Values: &[]interpreter.Value{},
	// 	}))
}

func TestInterpretEqualOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersUnequal(): Bool {
          return 5 == 3
      }

      fun testIntegersEqual(): Bool {
          return 3 == 3
      }

      fun testTrueAndTrue(): Bool {
          return true == true
      }

      fun testTrueAndFalse(): Bool {
          return true == false
      }

      fun testFalseAndTrue(): Bool {
          return false == true
      }

      fun testFalseAndFalse(): Bool {
          return false == false
      }

      fun testEqualStrings(): Bool {
          return "123" == "123"
      }

      fun testUnequalStrings(): Bool {
          return "123" == "abc"
      }

      fun testUnicodeStrings(): Bool {
          return "caf\u{E9}" == "cafe\u{301}"
      }
	`)

	value, err := inter.Invoke("testIntegersUnequal")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testTrueAndTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testTrueAndFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testFalseAndTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testFalseAndFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testEqualStrings")
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testUnequalStrings")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testUnicodeStrings")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)
}

func TestInterpretUnequalOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersUnequal(): Bool {
          return 5 != 3
      }

      fun testIntegersEqual(): Bool {
          return 3 != 3
      }

      fun testTrueAndTrue(): Bool {
          return true != true
      }

      fun testTrueAndFalse(): Bool {
          return true != false
      }

      fun testFalseAndTrue(): Bool {
          return false != true
      }

      fun testFalseAndFalse(): Bool {
          return false != false
      }
	`)

	value, err := inter.Invoke("testIntegersUnequal")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testTrueAndTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testTrueAndFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testFalseAndTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testFalseAndFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretLessOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersGreater(): Bool {
          return 5 < 3
      }

      fun testIntegersEqual(): Bool {
          return 3 < 3
      }

      fun testIntegersLess(): Bool {
          return 3 < 5
      }
    `)

	value, err := inter.Invoke("testIntegersGreater")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testIntegersLess")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)
}

func TestInterpretLessEqualOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersGreater(): Bool {
          return 5 <= 3
      }

      fun testIntegersEqual(): Bool {
          return 3 <= 3
      }

      fun testIntegersLess(): Bool {
          return 3 <= 5
      }
	`)

	value, err := inter.Invoke("testIntegersGreater")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testIntegersLess")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)
}

func TestInterpretGreaterOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersGreater(): Bool {
          return 5 > 3
      }

      fun testIntegersEqual(): Bool {
          return 3 > 3
      }

      fun testIntegersLess(): Bool {
          return 3 > 5
      }
	`)

	value, err := inter.Invoke("testIntegersGreater")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testIntegersLess")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretGreaterEqualOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testIntegersGreater(): Bool {
          return 5 >= 3
      }

      fun testIntegersEqual(): Bool {
          return 3 >= 3
      }

      fun testIntegersLess(): Bool {
          return 3 >= 5
      }
	`)

	value, err := inter.Invoke("testIntegersGreater")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testIntegersEqual")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testIntegersLess")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretOrOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testTrueTrue(): Bool {
          return true || true
      }

      fun testTrueFalse(): Bool {
          return true || false
      }

      fun testFalseTrue(): Bool {
          return false || true
      }

      fun testFalseFalse(): Bool {
          return false || false
      }
	`)

	value, err := inter.Invoke("testTrueTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testTrueFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testFalseTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testFalseFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretOrOperatorShortCircuitLeftSuccess(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Bool {
          x = true
          return true
      }

      fun changeY(): Bool {
          y = true
          return true
      }

      let test = changeX() || changeY()
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretOrOperatorShortCircuitLeftFailure(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Bool {
          x = true
          return false
      }

      fun changeY(): Bool {
          y = true
          return true
      }

      let test = changeX() || changeY()
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)
}

func TestInterpretAndOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testTrueTrue(): Bool {
          return true && true
      }

      fun testTrueFalse(): Bool {
          return true && false
      }

      fun testFalseTrue(): Bool {
          return false && true
      }

      fun testFalseFalse(): Bool {
          return false && false
      }
	`)

	value, err := inter.Invoke("testTrueTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("testTrueFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testFalseTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)

	value, err = inter.Invoke("testFalseFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretAndOperatorShortCircuitLeftSuccess(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Bool {
          x = true
          return true
      }

      fun changeY(): Bool {
          y = true
          return true
      }

      let test = changeX() && changeY()
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)
}

func TestInterpretAndOperatorShortCircuitLeftFailure(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Bool {
          x = true
          return false
      }

      fun changeY(): Bool {
          y = true
          return true
      }

      let test = changeX() && changeY()
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretIfStatement(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun testTrue(): Int {
           if true {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testFalse(): Int {
           if false {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testNoElse(): Int {
           if true {
               return 2
           }
           return 3
       }

       fun testElseIf(): Int {
           if false {
               return 2
           } else if true {
               return 3
           }
           return 4
       }
	`)

	value, err := inter.Invoke("testTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("testFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)

	value, err = inter.Invoke("testNoElse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("testElseIf")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

func TestInterpretWhileStatement(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 5 {
               x = x + 2
           }
           return x
       }

	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(6),
		value,
	)
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 2
               if x > 5 {
                   return x
               }
           }
           return x
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(6),
		value,
	)
}

func TestInterpretWhileStatementWithContinue(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var i = 0
           var x = 0
           while i < 10 {
               i = i + 1
               if i < 5 {
                   continue
               }
               x = x + 1
           }
           return x
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(6),
		value,
	)
}

func TestInterpretWhileStatementWithBreak(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 1
               if x == 5 {
                   break
               }
           }
           return x
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(5),
		value,
	)
}

func TestInterpretExpressionStatement(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       var x = 0

       fun incX() {
           x = x + 2
       }

       fun test(): Int {
           incX()
           return x
       }
	`)

	assert.Equal(t,
		interpreter.NewIntValue(0),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretConditionalOperator(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun testTrue(): Int {
           return true ? 2 : 3
       }

       fun testFalse(): Int {
			return false ? 2 : 3
       }
	`)

	value, err := inter.Invoke("testTrue")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("testFalse")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

func TestInterpretFunctionBindingInFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun foo(): Any {
          return foo
      }
  `)

	_, err := inter.Invoke("foo")
	assert.Nil(t, err)
}

func TestInterpretRecursionFib(t *testing.T) {
	// mainly tests that the function declaration identifier is bound
	// to the function inside the function and that the arguments
	// of the function calls are evaluated in the call-site scope

	inter := parseCheckAndInterpret(t, `
       fun fib(_ n: Int): Int {
           if n < 2 {
              return n
           }
           return fib(n - 1) + fib(n - 2)
       }
   `)

	value, err := inter.Invoke("fib", big.NewInt(14))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(377),
		value,
	)
}

func TestInterpretRecursionFactorial(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun factorial(_ n: Int): Int {
            if n < 1 {
               return 1
            }

            return n * factorial(n - 1)
        }
   `)

	value, err := inter.Invoke("factorial", big.NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(120),
		value,
	)
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = -2
      let y = -(-2)
	`)

	assert.Equal(t,
		interpreter.NewIntValue(-2),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["y"].Value,
	)
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let a = !true
      let b = !(!true)
      let c = !false
      let d = !(!false)
	`)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["c"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["d"].Value,
	)
}

func TestInterpretHostFunction(t *testing.T) {

	program, _, err := parser.ParseProgram(`
      let a = test(1, 2)
	`)

	assert.Nil(t, err)

	testFunction := stdlib.NewStandardLibraryFunction(
		"test",
		&sema.FunctionType{
			ParameterTypeAnnotations: sema.NewTypeAnnotations(
				&sema.IntType{},
				&sema.IntType{},
			),
			ReturnTypeAnnotation: sema.NewTypeAnnotation(
				&sema.IntType{},
			),
		},
		func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
			a := arguments[0].(interpreter.IntValue).Int
			b := arguments[1].(interpreter.IntValue).Int
			value := big.NewInt(0).Add(a, b)
			result := interpreter.IntValue{Int: value}
			return trampoline.Done{Result: result}
		},
		nil,
	)

	checker, err := sema.NewChecker(
		program,
		stdlib.StandardLibraryFunctions{
			testFunction,
		}.ToValueDeclarations(),
		nil,
	)
	assert.Nil(t, err)

	err = checker.Check()
	assert.Nil(t, err)

	inter, err := interpreter.NewInterpreter(
		checker,
		map[string]interpreter.Value{
			testFunction.Name: testFunction.Function,
		},
	)

	assert.Nil(t, err)

	err = inter.Interpret()
	assert.Nil(t, err)

	assert.Equal(t,
		interpreter.NewIntValue(3),
		inter.Globals["a"].Value,
	)
}

func TestInterpretStructureDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       struct Test {}

       fun test(): Test {
           return Test()
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretStructureDeclarationWithInitializer(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       var value = 0

       struct Test {
           init(_ newValue: Int) {
               value = newValue
           }
       }

       fun test(newValue: Int): Test {
           return Test(newValue)
       }
	`)

	newValue := big.NewInt(42)

	value, err := inter.Invoke("test", newValue)
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)

	assert.Equal(t,
		interpreter.IntValue{Int: newValue},
		inter.Globals["value"].Value,
	)
}

func TestInterpretStructureSelfReferenceInInitializer(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

      struct Test {

          init() {
              self
          }
      }

      fun test() {
          Test()
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureConstructorReferenceInInitializerAndFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

      struct Test {

          init() {
              Test
          }

          fun test(): Test {
              return Test()
          }
      }

      fun test(): Test {
          return Test()
      }

      fun test2(): Test {
          return Test().test()
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)

	value, err = inter.Invoke("test2")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretStructureSelfReferenceInFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

    struct Test {

        fun test() {
            self
        }
    }

    fun test() {
        Test().test()
    }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureConstructorReferenceInFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

    struct Test {

        fun test() {
            Test
        }
    }

    fun test() {
        Test().test()
    }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureDeclarationWithField(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

      struct Test {
          var test: Int

          init(_ test: Int) {
              self.test = test
          }
      }

      fun test(test: Int): Int {
          let test = Test(test)
          return test.test
      }
	`)

	newValue := big.NewInt(42)

	value, err := inter.Invoke("test", newValue)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.IntValue{Int: newValue},
		value,
	)
}

func TestInterpretStructureDeclarationWithFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var value = 0

      struct Test {
          fun test(_ newValue: Int) {
              value = newValue
          }
      }

      fun test(newValue: Int) {
          let test = Test()
          test.test(newValue)
      }
	`)

	newValue := big.NewInt(42)

	value, err := inter.Invoke("test", newValue)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.IntValue{Int: newValue},
		inter.Globals["value"].Value,
	)
}

func TestInterpretStructureFunctionCall(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Test {
          fun foo(): Int {
              return 42
          }

          fun bar(): Int {
              return self.foo()
          }
      }

      let value = Test().bar()
	`)

	assert.Equal(t,
		interpreter.NewIntValue(42),
		inter.Globals["value"].Value,
	)
}

func TestInterpretStructureFieldAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Test {
          var foo: Int

          init() {
              self.foo = 1
              let alsoSelf = self
              alsoSelf.foo = 2
          }

          fun test() {
              self.foo = 3
              let alsoSelf = self
              alsoSelf.foo = 4
          }
      }

	  let test = Test()

      fun callTest() {
          test.test()
      }
	`)

	actual := inter.Globals["test"].Value.(interpreter.CompositeValue).GetMember(inter, "foo")
	assert.Equal(t,
		interpreter.NewIntValue(1),
		actual,
	)

	value, err := inter.Invoke("callTest")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	actual = inter.Globals["test"].Value.(interpreter.CompositeValue).GetMember(inter, "foo")
	assert.Equal(t,
		interpreter.NewIntValue(3),
		actual,
	)
}

func TestInterpretStructureInitializesConstant(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Test {
          let foo: Int

          init() {
              self.foo = 42
          }
      }

	  let test = Test()
	`)

	actual := inter.Globals["test"].Value.(interpreter.CompositeValue).GetMember(inter, "foo")
	assert.Equal(t,
		interpreter.NewIntValue(42),
		actual,
	)
}

func TestInterpretStructureFunctionMutatesSelf(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Test {
          var foo: Int

          init() {
              self.foo = 0
          }

          fun inc() {
              self.foo = self.foo + 1
          }
      }

      fun test(): Int {
          let test = Test()
          test.inc()
          test.inc()
          return test.foo
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)
}

func TestInterpretFunctionPreCondition(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          pre {
              x == 0
          }
          return x
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := big.NewInt(0)
	value, err := inter.Invoke("test", zero)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.IntValue{Int: zero},
		value,
	)
}

func TestInterpretFunctionPostCondition(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := big.NewInt(0)
	value, err := inter.Invoke("test", zero)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.IntValue{Int: zero},
		value,
	)
}

func TestInterpretFunctionWithResultAndPostConditionWithResult(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              result == 0
          }
          return x
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := big.NewInt(0)
	value, err := inter.Invoke("test", zero)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.IntValue{Int: zero},
		value,
	)
}

func TestInterpretFunctionWithoutResultAndPostConditionWithResult(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretFunctionPostConditionWithBefore(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = 0

      fun test() {
          pre {
              x == 0
          }
          post {
              x == before(x) + 1
          }
          x = x + 1
      }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretFunctionPostConditionWithBeforeFailingPreCondition(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = 0

      fun test() {
          pre {
              x == 1
          }
          post {
              x == before(x) + 1
          }
          x = x + 1
      }
	`)

	_, err := inter.Invoke("test")

	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		ast.ConditionKindPre,
		err.(*interpreter.ConditionError).ConditionKind,
	)
}

func TestInterpretFunctionPostConditionWithBeforeFailingPostCondition(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = 0

      fun test() {
          pre {
              x == 0
          }
          post {
              x == before(x) + 2
          }
          x = x + 1
      }
	`)

	_, err := inter.Invoke("test")

	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		ast.ConditionKindPost,
		err.(*interpreter.ConditionError).ConditionKind,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingStringLiteral(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              y == 0: "y should be zero"
          }
          let y = x
          return y
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"y should be zero",
		err.(*interpreter.ConditionError).Message,
	)

	zero := big.NewInt(0)
	value, err := inter.Invoke("test", zero)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.IntValue{Int: zero},
		value,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingResult(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): String {
          post {
              y == 0: result
          }
          let y = x
          return "return value"
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"return value",
		err.(*interpreter.ConditionError).Message,
	)

	zero := big.NewInt(0)
	value, err := inter.Invoke("test", zero)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("return value"),
		value,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingBefore(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: String): String {
          post {
              1 == 2: before(x)
          }
          return "return value"
      }
	`)

	_, err := inter.Invoke("test", "parameter value")
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"parameter value",
		err.(*interpreter.ConditionError).Message,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingParameter(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: String): String {
          post {
              1 == 2: x
          }
          return "return value"
      }
	`)

	_, err := inter.Invoke("test", "parameter value")
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"parameter value",
		err.(*interpreter.ConditionError).Message,
	)
}

func TestInterpretStructCopyOnDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): [Bool] {
          let cat = Cat()
          let kitty = cat
          kitty.wasFed = true
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnDeclarationModifiedWithStructFunction(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }

          fun feed() {
              self.wasFed = true
          }
      }

      fun test(): [Bool] {
          let cat = Cat()
          let kitty = cat
          kitty.feed()
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnIdentifierAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): [Bool] {
          var cat = Cat()
          let kitty = Cat()
          cat = kitty
          kitty.wasFed = true
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnIndexingAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): [Bool] {
          let cats = [Cat()]
          let kitty = Cat()
          cats[0] = kitty
          kitty.wasFed = true
          return [cats[0].wasFed, kitty.wasFed]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnMemberAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      struct Carrier {
          var cat: Cat
          init(cat: Cat) {
              self.cat = cat
          }
      }

      fun test(): [Bool] {
          let carrier = Carrier(cat: Cat())
          let kitty = Cat()
          carrier.cat = kitty
          kitty.wasFed = true
          return [carrier.cat.wasFed, kitty.wasFed]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnPassing(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun feed(cat: Cat) {
          cat.wasFed = true
      }

      fun test(): Bool {
          let kitty = Cat()
          feed(cat: kitty)
          return kitty.wasFed
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretArrayCopy(t *testing.T) {

	inter := parseCheckAndInterpret(t, `

      fun change(_ numbers: [Int]): [Int] {
          numbers[0] = 1
          return numbers
      }

      fun test(): [Int] {
          let numbers = [0]
          let numbers2 = change(numbers)
          return [
              numbers[0],
              numbers2[0]
          ]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(0),
			interpreter.NewIntValue(1),
		),
		value,
	)
}

func TestInterpretStructCopyInArray(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      struct Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun test(): [Int] {
        let foo = Foo(bar: 1)
        let foos = [foo, foo]
        foo.bar = 2
        foos[0].bar = 3
		return [foo.bar, foos[0].bar, foos[1].bar]
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(1),
		),
		value,
	)
}

func TestInterpretMutuallyRecursiveFunctions(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
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

	four := big.NewInt(4)

	value, err := inter.Invoke("isEven", four)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("isOdd", four)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretReferenceBeforeDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var tests = 0

      fun test(): Test {
          return Test()
      }

      struct Test {
         init() {
             tests = tests + 1
         }
      }
    `)

	assert.Equal(t,
		interpreter.NewIntValue(0),
		inter.Globals["tests"].Value,
	)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["tests"].Value,
	)

	value, err = inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["tests"].Value,
	)
}

func TestInterpretOptionalVariableDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int?? = 2
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.SomeValue{
				Value: interpreter.NewIntValue(2),
			},
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalParameterInvokedExternal(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int??): Int?? {
          return x
      }
    `)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.SomeValue{
				Value: interpreter.NewIntValue(2),
			},
		},
		value,
	)
}

func TestInterpretOptionalParameterInvokedInternal(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun testActual(x: Int??): Int?? {
          return x
      }

      fun test(): Int?? {
          return testActual(x: 2)
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.SomeValue{
				Value: interpreter.NewIntValue(2),
			},
		},
		value,
	)
}

func TestInterpretOptionalReturn(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int?? {
          return x
      }
    `)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.SomeValue{
				Value: interpreter.NewIntValue(2),
			},
		},
		value,
	)
}

func TestInterpretOptionalAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x: Int?? = 1

      fun test() {
          x = 2
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.SomeValue{
				Value: interpreter.NewIntValue(2),
			},
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNil(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     let x: Int? = nil
   `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalNestingNil(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = nil
   `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilReturnValue(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     fun test(): Int?? {
         return nil
     }
   `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NilValue{},
		value,
	)
}

func TestInterpretSomeReturnValue(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     fun test(): Int? {
         let x: Int? = 1
         return x
     }
   `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		value,
	)
}

func TestInterpretSomeReturnValueFromDictionary(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     fun test(): Int? {
         let foo: {String: Int} = {"a": 1}
         return foo["a"]
     }
   `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		value,
	)
}

func TestInterpretNilCoalescingNilIntToOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int? = nil
      let x: Int? = none ?? one
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilIntToOptionals(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int?? = nil
      let x: Int? = none ?? one
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilIntToOptionalNilLiteral(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let x: Int? = nil ?? one
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingRightSubtype(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = nil ?? nil
    `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilInt(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int? = nil
      let x: Int = none ?? one
    `)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilLiteralInt(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let x: Int = nil ?? one
    `)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingShortCircuitLeftSuccess(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Int? {
          x = true
          return 1
      }

      fun changeY(): Int {
          y = true
          return 2
      }

      let test = changeX() ?? changeY()
    `)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingShortCircuitLeftFailure(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var x = false
      var y = false

      fun changeX(): Int? {
          x = true
          return nil
      }

      fun changeY(): Int {
          y = true
          return 2
      }

      let test = changeX() ?? changeY()
    `)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["test"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingOptionalAnyNil(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = nil
      let y = x ?? true
    `)

	assert.Equal(t,
		interpreter.AnyValue{
			Type:  &sema.BoolType{},
			Value: interpreter.BoolValue(true),
		},
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingOptionalAnySome(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = 2
      let y = x ?? true
    `)

	assert.Equal(t,
		interpreter.AnyValue{
			Type:  &sema.IntType{},
			Value: interpreter.NewIntValue(2),
		},
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingOptionalRightHandSide(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = 1
      let y: Int? = 2
      let z = x ?? y
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilCoalescingBothOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = 1
     let y: Int? = 2
     let z = x ?? y
   `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilCoalescingBothOptionalLeftNil(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = nil
     let y: Int? = 2
     let z = x ?? y
   `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilsComparison(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = nil == nil
   `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNonOptionalNilComparison(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int = 1
      let y = x == nil
   `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNonOptionalNilComparisonSwapped(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int = 1
      let y = nil == x
   `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalNilComparison(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
     let x: Int? = 1
     let y = x == nil
   `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNestedOptionalNilComparison(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int?? = 1
      let y = x == nil
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalNilComparisonSwapped(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = 1
      let y = nil == x
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNestedOptionalNilComparisonSwapped(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int?? = 1
      let y = nil == x
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNestedOptionalComparisonNils(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = nil
      let y: Int?? = nil
      let z = x == y
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["z"].Value,
	)
}

func TestInterpretNestedOptionalComparisonValues(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = 2
      let y: Int?? = 2
      let z = x == y
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["z"].Value,
	)
}

func TestInterpretNestedOptionalComparisonMixed(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Int? = 2
      let y: Int?? = nil
      let z = x == y
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["z"].Value,
	)
}

func TestInterpretIfStatementTestWithDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int?): Int {
          if var y = x {
              return y
          } else {
              return 0
          }
      }
	`)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("test", nil)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(0),
		value,
	)
}

func TestInterpretIfStatementTestWithDeclarationAndElse(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int?): Int {
          if var y = x {
              return y
          }
          return 0
      }
	`)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("test", nil)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(0),
		value,
	)
}

func TestInterpretIfStatementTestWithDeclarationNestedOptionals(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int??): Int? {
          if var y = x {
              return y
          } else {
              return 0
          }
      }
	`)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		value,
	)

	value, err = inter.Invoke("test", nil)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(0),
		},
		value,
	)
}

func TestInterpretIfStatementTestWithDeclarationNestedOptionalsExplicitAnnotation(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int??): Int? {
          if var y: Int? = x {
              return y
          } else {
              return 0
          }
      }
	`)

	value, err := inter.Invoke("test", big.NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		value,
	)

	value, err = inter.Invoke("test", nil)
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(0),
		},
		value,
	)
}

func TestInterpretInterfaceConformanceNoRequirements(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		// TODO: add support for non-structure / non-resource composites

		if kind != common.CompositeKindStructure &&
			kind != common.CompositeKindResource {

			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(`
              %[1]s interface Test {}

              %[1]s TestImpl: Test {}

              let test: %[2]sTest %[3]s %[4]s TestImpl()
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			assert.IsType(t,
				interpreter.CompositeValue{},
				inter.Globals["test"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFieldUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		// TODO: add support for non-structure / non-resource composites

		if kind != common.CompositeKindStructure &&
			kind != common.CompositeKindResource {

			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(`
              %[1]s interface Test {
                  x: Int
              }

              %[1]s TestImpl: Test {
                  var x: Int

                  init(x: Int) {
                      self.x = x
                  }
              }

              let test: %[2]sTest %[3]s %[4]s TestImpl(x: 1)

              let x = test.x
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			assert.Equal(t,
				interpreter.NewIntValue(1),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFunctionUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		// TODO: add support for non-structure / non-resource composites

		if kind != common.CompositeKindStructure &&
			kind != common.CompositeKindResource {

			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(
				`
                    %[1]s interface Test {
                        fun test(): Int
                    }

                    %[1]s TestImpl: Test {
                        fun test(): Int {
                            return 2
                        }
                    }

                    let test: %[2]s Test %[3]s %[4]s TestImpl()

                    let val = test.test()
	            `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			assert.Equal(t,
				interpreter.NewIntValue(2),
				inter.Globals["val"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFunctionUseWithPreCondition(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		// TODO: add support for non-structure / non-resource composites

		if kind != common.CompositeKindStructure &&
			kind != common.CompositeKindResource {

			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(`
              %[1]s interface Test {
                  fun test(x: Int): Int {
                      pre {
                          x > 0: "x must be positive"
                      }
                  }
              }

              %[1]s TestImpl: Test {
                  fun test(x: Int): Int {
                      pre {
                          x < 2: "x must be smaller than 2"
                      }
                      return x
                  }
              }

              fun callTest(x: Int): Int {
                  let test: %[2]s Test %[3]s %[4]s TestImpl()
                  let res = test.test(x: x)
                  %[5]s test
                  return res
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
				kind.DestructionKeyword(),
			))

			_, err := inter.Invoke("callTest", big.NewInt(0))
			assert.IsType(t, &interpreter.ConditionError{}, err)

			value, err := inter.Invoke("callTest", big.NewInt(1))
			assert.Nil(t, err)
			assert.Equal(t,
				interpreter.NewIntValue(1),
				value,
			)

			_, err = inter.Invoke("callTest", big.NewInt(2))
			assert.IsType(t,
				&interpreter.ConditionError{},
				err,
			)
		})
	}
}

func TestInterpretInitializerWithInterfacePreCondition(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		// TODO: add support for non-structure / non-resource composites

		if kind != common.CompositeKindStructure &&
			kind != common.CompositeKindResource {

			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t, fmt.Sprintf(`
              %[1]s interface Test {
                  init(x: Int) {
                      pre {
                          x > 0: "x must be positive"
                      }
                  }
              }

              %[1]s TestImpl: Test {
                  init(x: Int) {
                      pre {
                          x < 2: "x must be smaller than 2"
                      }
                  }
              }

              fun test(x: Int): %[2]sTest {
                  return %[2]s %[3]s TestImpl(x: x)
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			_, err := inter.Invoke("test", big.NewInt(0))
			assert.IsType(t,
				&interpreter.ConditionError{},
				err,
			)

			value, err := inter.Invoke("test", big.NewInt(1))
			assert.Nil(t, err)
			assert.IsType(t,
				interpreter.CompositeValue{},
				value,
			)

			_, err = inter.Invoke("test", big.NewInt(2))
			assert.IsType(t,
				&interpreter.ConditionError{},
				err,
			)
		})
	}
}

func TestInterpretImport(t *testing.T) {

	checkerImported, err := ParseAndCheck(t, `
      fun answer(): Int {
          return 42
      }
	`)
	require.Nil(t, err)

	checkerImporting, err := ParseAndCheckWithExtra(t,
		`
          import answer from "imported"

          fun test(): Int {
              return answer()
          }
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			assert.Equal(t,
				ast.StringImportLocation("imported"),
				location,
			)
			return checkerImported.Program, nil
		},
	)
	require.Nil(t, err)

	inter, err := interpreter.NewInterpreter(checkerImporting, nil)
	require.Nil(t, err)

	err = inter.Interpret()
	require.Nil(t, err)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(42),
		value,
	)
}

func TestInterpretImportError(t *testing.T) {

	valueDeclarations :=
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations()

	checkerImported, err := ParseAndCheckWithExtra(t,
		`
          fun answer(): Int {
              return panic("?!")
          }
	    `,
		valueDeclarations,
		nil,
		nil,
	)
	require.Nil(t, err)

	checkerImporting, err := ParseAndCheckWithExtra(t,
		`
          import answer from "imported"

          fun test(): Int {
              return answer()
          }
        `,
		valueDeclarations,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			assert.Equal(t,
				ast.StringImportLocation("imported"),
				location,
			)
			return checkerImported.Program, nil
		},
	)
	require.Nil(t, err)

	values := stdlib.StandardLibraryFunctions{
		stdlib.PanicFunction,
	}.ToValues()

	inter, err := interpreter.NewInterpreter(checkerImporting, values)
	require.Nil(t, err)

	err = inter.Interpret()
	require.Nil(t, err)

	_, err = inter.Invoke("test")

	assert.IsType(t, stdlib.PanicError{}, err)
	assert.Equal(t,
		"?!",
		err.(stdlib.PanicError).Message,
	)
}

func TestInterpretDictionary(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {"a": 1, "b": 2}
	`)

	assert.Equal(t,
		interpreter.DictionaryValue{
			"a": interpreter.NewIntValue(1),
			"b": interpreter.NewIntValue(2),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretDictionaryNonLexicalOrder(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {"c": 3, "b": 2, "a": 1}
	`)

	assert.Equal(t,
		interpreter.DictionaryValue{
			"c": interpreter.NewIntValue(3),
			"b": interpreter.NewIntValue(2),
			"a": interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretDictionaryIndexingString(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {"abc": 1, "def": 2}
      let a = x["abc"]
      let b = x["def"]
      let c = x["ghi"]
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["c"].Value,
	)
}

func TestInterpretDictionaryIndexingBool(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {true: 1, false: 2}
      let a = x[true]
      let b = x[false]
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		inter.Globals["b"].Value,
	)
}

func TestInterpretDictionaryIndexingInt(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {23: "a", 42: "b"}
      let a = x[23]
      let b = x[42]
      let c = x[100]
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewStringValue("a"),
		},
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewStringValue("b"),
		},
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["c"].Value,
	)
}

func TestInterpretDictionaryIndexingAssignmentExisting(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = {"abc": 42}
      fun test() {
          x["abc"] = 23
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.SomeValue{Value: interpreter.NewIntValue(23)},
		inter.Globals["x"].Value.(interpreter.DictionaryValue).Get(interpreter.NewStringValue("abc")),
	)
}

func TestInterpretFailableDowncastingAnySuccess(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any = 42
      let y: Int? = x as? Int
    `)

	assert.Equal(t,
		interpreter.AnyValue{
			Type:  &sema.IntType{},
			Value: interpreter.NewIntValue(42),
		},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(42),
		},
		inter.Globals["y"].Value,
	)
}

func TestInterpretFailableDowncastingAnyFailure(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any = 42
      let y: Bool? = x as? Bool
    `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalAny(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = 42
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.AnyValue{
				Type:  &sema.IntType{},
				Value: interpreter.NewIntValue(42),
			},
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalAnyFailableDowncasting(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = 42
      let y = (x ?? 23) as? Int
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.AnyValue{
				Type:  &sema.IntType{},
				Value: interpreter.NewIntValue(42),
			},
		},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(42),
		},
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalAnyFailableDowncastingInt(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = 23
      let y = x ?? 42
      let z = y as? Int
    `)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.AnyValue{
				Type:  &sema.IntType{},
				Value: interpreter.NewIntValue(23),
			},
		},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.AnyValue{
			Type:  &sema.IntType{},
			Value: interpreter.NewIntValue(23),
		},
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(23),
		},
		inter.Globals["z"].Value,
	)
}

func TestInterpretOptionalAnyFailableDowncastingNil(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x: Any? = nil
      let y = x ?? 42
      let z = y as? Int
    `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.AnyValue{
			Type:  &sema.IntType{},
			Value: interpreter.NewIntValue(42),
		},
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(42),
		},
		inter.Globals["z"].Value,
	)
}

func TestInterpretLength(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	assert.Equal(t,
		interpreter.NewIntValue(4),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(3),
		inter.Globals["y"].Value,
	)
}

func TestInterpretStructureFunctionBindingInside(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        struct X {
            fun foo(): ((): X) {
                return self.bar
            }

            fun bar(): X {
                return self
            }
        }

        fun test(): X {
            let x = X()
            let bar = x.foo()
            return bar()
        }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretStructureFunctionBindingOutside(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        struct X {
            fun foo(): X {
                return self
            }
        }

        fun test(): X {
            let x = X()
            let bar = x.foo
            return bar()
        }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.IsType(t,
		interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretArrayAppend(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(4),
		),
		value,
	)
}

func TestInterpretArrayAppendBound(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(4),
		),
		value,
	)
}

func TestInterpretArrayConcat(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let a = [1, 2]
          return a.concat([3, 4])
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(4),
		),
		value,
	)
}

func TestInterpretArrayConcatBound(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let a = [1, 2]
          let b = a.concat
          return b([3, 4])
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(4),
		),
		value,
	)
}

func TestInterpretArrayInsert(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(4),
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
		),
		value,
	)
}

func TestInterpretArrayRemove(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
          let x = [1, 2, 3]
          let y = x.remove(at: 1)
    `)

	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(3),
		),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayRemoveFirst(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
          let x = [1, 2, 3]
          let y = x.removeFirst()
    `)

	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(3),
		),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayRemoveLast(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
          let x = [1, 2, 3]
          let y = x.removeLast()
    `)

	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(1),
			interpreter.NewIntValue(2),
		),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValue(3),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayContains(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun doesContain(): Bool {
		  let a = [1, 2]
		  return a.contains(1)
	  }

	  fun doesNotContain(): Bool {
		  let a = [1, 2]
		  return a.contains(3)
	  }
    `)

	value, err := inter.Invoke("doesContain")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("doesNotContain")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretStringConcat(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let a = "abc"
          return a.concat("def")
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("abcdef"),
		value,
	)
}

func TestInterpretStringConcatBound(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let a = "abc"
          let b = a.concat
          return b("def")
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewStringValue("abcdef"),
		value,
	)
}

func TestInterpretDictionaryRemove(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      var removed: Int? = nil

      fun test(): {String: Int} {
          let x = {"abc": 1, "def": 2}
          removed = x.remove(key: "abc")
          return x
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.DictionaryValue{
			"def": interpreter.NewIntValue(2),
		},
		value,
	)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["removed"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInVariableDeclaration(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        let x: Int8 = 1
	`)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInVariableDeclarationOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        let x: Int8? = 1
	`)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInAssignment(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        var x: Int8 = 1
        fun test() {
            x = 2
        }
	`)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)

	_, err := inter.Invoke("test")
	assert.Nil(t, err)

	assert.Equal(t,
		interpreter.NewIntValue(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInAssignmentOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        var x: Int8? = 1
        fun test() {
            x = 2
        }
	`)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)

	_, err := inter.Invoke("test")
	assert.Nil(t, err)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(2),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInFunctionCallArgument(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun test(_ x: Int8): Int8 {
            return x
        }
        let x = test(1)
	`)

	assert.Equal(t,
		interpreter.NewIntValue(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInFunctionCallArgumentOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun test(_ x: Int8?): Int8? {
            return x
        }
        let x = test(1)
	`)

	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInReturn(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun test(): Int8 {
            return 1
        }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(1),
		value,
	)
}

func TestInterpretIntegerLiteralTypeConversionInReturnOptional(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
        fun test(): Int8? {
            return 1
        }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(1),
		},
		value,
	)
}

func TestInterpretIndirectDestroy(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretUnaryMove(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun foo(x: <-X): <-X {
          return <-x
      }

      fun bar() {
          let x <- foo(x: <-create X())
          destroy x
      }
    `)

	value, err := inter.Invoke("bar")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretResourceMoveInArray(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
      resource Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun test(): Int {
        let foo <- create Foo(bar: 1)
        let foos <- [<-foo]
        let bar = foos[0].bar
        destroy foos
        return bar
      }
    `)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(1),
		value,
	)
}

func TestInterpretClosure(t *testing.T) {
	// Create a closure that increments and returns
	// a variable each time it is invoked.

	inter := parseCheckAndInterpret(t, `
        fun makeCounter(): ((): Int) {
            var count = 0
            return fun (): Int {
                count = count + 1
                return count
            }
        }

        let test = makeCounter()
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(1),
		value,
	)

	value, err = inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(2),
		value,
	)

	value, err = inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewIntValue(3),
		value,
	)
}

// TestInterpretCompositeFunctionInvocationFromImportingProgram checks
// that member functions of imported composites can be invoked from an importing program.
// See https://github.com/dapperlabs/flow-go/issues/838
//
func TestInterpretCompositeFunctionInvocationFromImportingProgram(t *testing.T) {

	checkerImported, err := ParseAndCheck(t, `
      // function must have arguments
      fun x(x: Int) {}

      // invocation must be in composite
      struct Y {
        fun x() {
          x(x: 1)
        }
      }
    `)
	require.Nil(t, err)

	checkerImporting, err := ParseAndCheckWithExtra(t,
		`
          import Y from "imported"

          fun test() {
              // get member must bind using imported interpreter
              Y().x()
          }
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			assert.Equal(t,
				ast.StringImportLocation("imported"),
				location,
			)
			return checkerImported.Program, nil
		},
	)
	require.Nil(t, err)

	inter, err := interpreter.NewInterpreter(checkerImporting, nil)
	require.Nil(t, err)

	err = inter.Interpret()
	require.Nil(t, err)

	_, err = inter.Invoke("test")
	assert.Nil(t, err)
}

var storageValueDeclaration = map[string]sema.ValueDeclaration{
	"storage": stdlib.StandardLibraryValue{
		Name:       "storage",
		Type:       &sema.StorageType{},
		Kind:       common.DeclarationKindConstant,
		IsConstant: true,
	},
}

func TestInterpretStorage(t *testing.T) {

	storedValues := map[sema.Type]interpreter.Value{}

	inter := parseCheckAndInterpretWithExtra(t,
		`
          fun test(): Int? {
              storage[Int] = 42
              return storage[Int]
          }
	    `,
		storageValueDeclaration,
		map[string]interpreter.Value{
			"storage": interpreter.StorageValue{
				Getter: func(key sema.Type) interpreter.Value {
					return storedValues[key]
				},
				Setter: func(key sema.Type, value interpreter.Value) {
					storedValues[key] = value
				},
			},
		},
		nil,
	)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.SomeValue{
			Value: interpreter.NewIntValue(42),
		},
		value,
	)
}

func TestInterpretSwapVariables(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       fun test(): [Int] {
           var x = 2
           var y = 3
           x <-> y
           return [x, y]
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(3),
			interpreter.NewIntValue(2),
		),
		value,
	)
}

func TestInterpretSwapArrayAndField(t *testing.T) {

	inter := parseCheckAndInterpret(t, `
       struct Foo {
           var bar: Int

           init(bar: Int) {
               self.bar = bar
           }
       }

       fun test(): [Int] {
           let foo = Foo(bar: 1)
           let nums = [2]
           foo.bar <-> nums[0]
           return [foo.bar, nums[0]]
       }
	`)

	value, err := inter.Invoke("test")
	assert.Nil(t, err)
	assert.Equal(t,
		interpreter.NewArrayValue(
			interpreter.NewIntValue(2),
			interpreter.NewIntValue(1),
		),
		value,
	)
}

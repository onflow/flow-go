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
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/cadence/runtime/tests/checker"
	"github.com/onflow/cadence/runtime/tests/examples"
	. "github.com/onflow/cadence/runtime/tests/utils"
	"github.com/onflow/cadence/runtime/trampoline"
)

type ParseCheckAndInterpretOptions struct {
	Options            []interpreter.Option
	CheckerOptions     []sema.Option
	HandleCheckerError func(error)
}

func parseCheckAndInterpret(t *testing.T, code string) *interpreter.Interpreter {
	return parseCheckAndInterpretWithOptions(t, code, ParseCheckAndInterpretOptions{})
}

func parseCheckAndInterpretWithOptions(
	t *testing.T,
	code string,
	options ParseCheckAndInterpretOptions,
) *interpreter.Interpreter {

	checker, err := checker.ParseAndCheckWithOptions(t,
		code,
		checker.ParseAndCheckOptions{
			Options: options.CheckerOptions,
		},
	)

	if options.HandleCheckerError != nil {
		options.HandleCheckerError(err)
	} else {
		if !assert.NoError(t, err) {
			assert.FailNow(t, errors.UnrollChildErrors(err))
			return nil
		}
	}

	var uuid uint64 = 0

	interpreterOptions := append(
		[]interpreter.Option{
			interpreter.WithUUIDHandler(func() uint64 {
				uuid++
				return uuid
			}),
		},
		options.Options...,
	)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreterOptions...,
	)

	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	return inter
}

func constructorArguments(compositeKind common.CompositeKind, arguments string) string {
	if compositeKind == common.CompositeKindContract {
		return ""
	}
	return fmt.Sprintf("(%s)", arguments)
}

// makeContractValueHandler creates an interpreter option which
// sets the ContractValueHandler.
// The handler immediately invokes the constructor with the given arguments.
//
func makeContractValueHandler(
	arguments []interpreter.Value,
	argumentTypes []sema.Type,
	parameterTypes []sema.Type,
) interpreter.Option {
	return interpreter.WithContractValueHandler(
		func(
			inter *interpreter.Interpreter,
			compositeType *sema.CompositeType,
			constructor interpreter.FunctionValue,
		) *interpreter.CompositeValue {
			value, err := inter.InvokeFunctionValue(
				constructor,
				arguments,
				argumentTypes,
				parameterTypes,
				ast.Range{},
			)
			if err != nil {
				panic(err)
			}

			return value.(*interpreter.CompositeValue)
		},
	)
}

func TestInterpretConstantAndVariableDeclarations(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
        let x = 1
        let y = true
        let z = 1 + 2
        var a = 3 == 3
        var b = [1, 2]
        let s = "123"
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		inter.Globals["z"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
		),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NewStringValue("123"),
		inter.Globals["s"].Value,
	)
}

func TestInterpretDeclarations(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
        fun test(): Int {
            return 42
        }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		value,
	)
}

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, ``)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.NotDeclaredError{}, err)
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       let test = 1
   `)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.NotInvokableError{}, err)
}

func TestInterpretLexicalScope(t *testing.T) {

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(10),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("f")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(10),
		value,
	)

	value, err = inter.Invoke("g")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(10),
		value,
	)
}

func TestInterpretFunctionSideEffects(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       var value = 0

       fun test(_ newValue: Int) {
           value = newValue
       }
    `)

	newValue := interpreter.NewIntValueFromInt64(42)

	value, err := inter.Invoke("test", newValue)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		newValue,
		inter.Globals["value"].Value,
	)
}

func TestInterpretNoHoisting(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(10),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		inter.Globals["y"].Value,
	)
}

func TestInterpretVariableAssignment(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 2
           x = 3
           return x
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       var x = 2

       fun test(): Int {
           x = 3
           return x
       }
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		inter.Globals["x"].Value,
	)
}

func TestInterpretConstantRedeclaration(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       let x = 2

       fun test(): Int {
           let x = 3
           return x
       }
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)
}

func TestInterpretParameters(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun returnA(a: Int, b: Int): Int {
           return a
       }

       fun returnB(a: Int, b: Int): Int {
           return b
       }
    `)

	a := interpreter.NewIntValueFromInt64(24)
	b := interpreter.NewIntValueFromInt64(42)

	value, err := inter.Invoke("returnA", a, b)
	require.NoError(t, err)

	assert.Equal(t, a, value)

	value, err = inter.Invoke("returnB", a, b)
	require.NoError(t, err)

	assert.Equal(t, b, value)
}

func TestInterpretArrayIndexing(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           let z = [0, 3]
           return z[1]
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       let z = [0, 3]

       fun test() {
           z[1] = 2
       }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	actualArray := inter.Globals["z"].Value

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(0),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.SetIndex(1, interpreter.NewIntValueFromInt64(2))

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(0),
			interpreter.NewIntValueFromInt64(2),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)
}

func TestInterpretStringIndexing(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("\u00e9"),
		value,
	)

	value, err = inter.Invoke("testUnicodeB")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("e\u0301"),
		value,
	)
}

func TestInterpretStringIndexingAssignment(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let z = "abc"
      let y: Character = "d"

      fun test() {
          z[0] = y
      }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	actualString := inter.Globals["z"].Value

	expectedString := interpreter.NewStringValue("abc").
		Copy().(*interpreter.StringValue)
	expectedString.SetIndex(0, interpreter.NewStringValue("d"))

	require.Equal(t,
		expectedString,
		actualString,
	)

	assert.True(t, actualString.IsModified())

	assert.Equal(t,
		"dbc",
		actualString.(*interpreter.StringValue).Str,
	)
}

func TestInterpretStringIndexingAssignmentUnicode(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let z = "cafe chair"
          let y: Character = "e\u{301}"
          z[3] = y
          return z
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("cafe\u0301 chair"),
		value,
	)
}

func TestInterpretStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

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

	t.Parallel()

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

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      fun test(): String {
                        let s = "%s"
                        return s.slice(from: %d, upTo: %d)
                      }
                    `,
					test.str,
					test.from,
					test.to,
				),
			)

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

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun returnNothing() {
           return
       }
    `)

	value, err := inter.Invoke("returnNothing")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretReturns(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
           pub fun returnEarly(): Int {
               return 2
               return 1
           }
        `,
		ParseCheckAndInterpretOptions{
			HandleCheckerError: func(err error) {
				errs := checker.ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
			},
		},
	)

	value, err := inter.Invoke("returnEarly")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)
}

func TestInterpretEqualOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersUnequal": false,
		"testIntegersEqual":   true,
		"testTrueAndTrue":     true,
		"testTrueAndFalse":    false,
		"testFalseAndTrue":    false,
		"testFalseAndFalse":   true,
		"testEqualStrings":    true,
		"testUnequalStrings":  false,
		"testUnicodeStrings":  true,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretUnequalOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersUnequal": true,
		"testIntegersEqual":   false,
		"testTrueAndTrue":     false,
		"testTrueAndFalse":    true,
		"testFalseAndTrue":    true,
		"testFalseAndFalse":   false,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretLessOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersGreater": false,
		"testIntegersEqual":   false,
		"testIntegersLess":    true,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretLessEqualOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersGreater": false,
		"testIntegersEqual":   true,
		"testIntegersLess":    true,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretGreaterOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersGreater": true,
		"testIntegersEqual":   false,
		"testIntegersLess":    false,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretGreaterEqualOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testIntegersGreater": true,
		"testIntegersEqual":   true,
		"testIntegersLess":    false,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretOrOperator(t *testing.T) {

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testTrueTrue":   true,
		"testTrueFalse":  true,
		"testFalseTrue":  true,
		"testFalseFalse": false,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretOrOperatorShortCircuitLeftSuccess(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	for name, expected := range map[string]bool{
		"testTrueTrue":   true,
		"testTrueFalse":  false,
		"testFalseTrue":  false,
		"testFalseFalse": false,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.BoolValue(expected),
				value,
			)
		})
	}
}

func TestInterpretAndOperatorShortCircuitLeftSuccess(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
           pub fun testTrue(): Int {
               if true {
                   return 2
               } else {
                   return 3
               }
               return 4
           }

           pub fun testFalse(): Int {
               if false {
                   return 2
               } else {
                   return 3
               }
               return 4
           }

           pub fun testNoElse(): Int {
               if true {
                   return 2
               }
               return 3
           }

           pub fun testElseIf(): Int {
               if false {
                   return 2
               } else if true {
                   return 3
               }
               return 4
           }
        `,
		ParseCheckAndInterpretOptions{
			HandleCheckerError: func(err error) {
				errs := checker.ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
				assert.IsType(t, &sema.UnreachableStatementError{}, errs[1])
			},
		},
	)

	for name, expected := range map[string]int64{
		"testTrue":   2,
		"testFalse":  3,
		"testNoElse": 2,
		"testElseIf": 3,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := inter.Invoke(name)
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.NewIntValueFromInt64(expected),
				value,
			)
		})
	}
}

func TestInterpretExpressionStatement(t *testing.T) {

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["x"].Value,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["x"].Value,
	)
}

func TestInterpretConditionalOperator(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun testTrue(): Int {
           return true ? 2 : 3
       }

       fun testFalse(): Int {
            return false ? 2 : 3
       }
    `)

	value, err := inter.Invoke("testTrue")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)

	value, err = inter.Invoke("testFalse")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)
}

func TestInterpretFunctionBindingInFunction(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun foo(): AnyStruct {
          return foo
      }
  `)

	_, err := inter.Invoke("foo")
	require.NoError(t, err)
}

func TestInterpretRecursionFib(t *testing.T) {

	t.Parallel()

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

	value, err := inter.Invoke(
		"fib",
		interpreter.NewIntValueFromInt64(14),
	)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(377),
		value,
	)
}

func TestInterpretRecursionFactorial(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
        fun factorial(_ n: Int): Int {
            if n < 1 {
               return 1
            }

            return n * factorial(n - 1)
        }
   `)

	value, err := inter.Invoke(
		"factorial",
		interpreter.NewIntValueFromInt64(5),
	)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(120),
		value,
	)
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = -2
      let y = -(-2)
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(-2),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["y"].Value,
	)
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	program, _, err := parser.ParseProgram(`
      pub let a = test(1, 2)
    `)

	require.NoError(t, err)

	testFunction := stdlib.NewStandardLibraryFunction(
		"test",
		&sema.FunctionType{
			Parameters: []*sema.Parameter{
				{
					Label:          sema.ArgumentLabelNotRequired,
					Identifier:     "a",
					TypeAnnotation: sema.NewTypeAnnotation(&sema.IntType{}),
				},
				{
					Label:          sema.ArgumentLabelNotRequired,
					Identifier:     "b",
					TypeAnnotation: sema.NewTypeAnnotation(&sema.IntType{}),
				},
			},
			ReturnTypeAnnotation: sema.NewTypeAnnotation(
				&sema.IntType{},
			),
		},
		func(invocation interpreter.Invocation) trampoline.Trampoline {
			a := invocation.Arguments[0].(interpreter.IntValue).ToBigInt()
			b := invocation.Arguments[1].(interpreter.IntValue).ToBigInt()
			value := new(big.Int).Add(a, b)
			result := interpreter.NewIntValueFromBigInt(value)
			return trampoline.Done{Result: result}
		},
		nil,
	)

	checker, err := sema.NewChecker(
		program,
		TestLocation,
		sema.WithPredeclaredValues(
			stdlib.StandardLibraryFunctions{
				testFunction,
			}.ToValueDeclarations(),
		),
	)
	require.NoError(t, err)

	err = checker.Check()
	require.NoError(t, err)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(
			map[string]interpreter.Value{
				testFunction.Name: testFunction.Function,
			},
		),
	)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		inter.Globals["a"].Value,
	)
}

func TestInterpretHostFunctionWithVariableArguments(t *testing.T) {

	t.Parallel()

	program, _, err := parser.ParseProgram(`
      pub let nothing = test(1, true, "test")
    `)

	require.NoError(t, err)

	called := false

	testFunction := stdlib.NewStandardLibraryFunction(
		"test",
		&sema.FunctionType{
			Parameters: []*sema.Parameter{
				{
					Label:          sema.ArgumentLabelNotRequired,
					Identifier:     "value",
					TypeAnnotation: sema.NewTypeAnnotation(&sema.IntType{}),
				},
			},
			ReturnTypeAnnotation: sema.NewTypeAnnotation(
				&sema.IntType{},
			),
			RequiredArgumentCount: (func() *int {
				var count = 1
				return &count
			})(),
		},
		func(invocation interpreter.Invocation) trampoline.Trampoline {
			called = true

			require.Len(t, invocation.ArgumentTypes, 3)
			assert.IsType(t, &sema.IntType{}, invocation.ArgumentTypes[0])
			assert.IsType(t, &sema.BoolType{}, invocation.ArgumentTypes[1])
			assert.IsType(t, &sema.StringType{}, invocation.ArgumentTypes[2])

			require.Len(t, invocation.Arguments, 3)
			assert.Equal(t, interpreter.NewIntValueFromInt64(1), invocation.Arguments[0])
			assert.Equal(t, interpreter.BoolValue(true), invocation.Arguments[1])
			assert.Equal(t, interpreter.NewStringValue("test"), invocation.Arguments[2])

			return trampoline.Done{Result: interpreter.VoidValue{}}
		},
		nil,
	)

	checker, err := sema.NewChecker(
		program,
		TestLocation,
		sema.WithPredeclaredValues(
			stdlib.StandardLibraryFunctions{
				testFunction,
			}.ToValueDeclarations(),
		),
	)
	require.NoError(t, err)

	err = checker.Check()
	require.NoError(t, err)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(
			map[string]interpreter.Value{
				testFunction.Name: testFunction.Function,
			},
		),
	)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	assert.True(t, called)
}

func TestInterpretCompositeDeclaration(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		switch compositeKind {
		case common.CompositeKindContract,
			common.CompositeKindEvent:

			continue
		}

		t.Run(compositeKind.Name(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                       pub %[1]s Test {}

                       pub fun test(): %[2]sTest {
                           return %[3]s %[4]s Test%[5]s
                       }
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind, ""),
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(nil, nil, nil),
					},
				},
			)

			value, err := inter.Invoke("test")
			require.NoError(t, err)

			assert.IsType(t,
				&interpreter.CompositeValue{},
				value,
			)
		})
	}
}

func TestInterpretStructureSelfUseInInitializer(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureConstructorUseInInitializerAndFunction(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)

	value, err = inter.Invoke("test2")
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretStructureSelfUseInFunction(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureConstructorUseInFunction(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretStructureDeclarationWithField(t *testing.T) {

	t.Parallel()

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

	newValue := interpreter.NewIntValueFromInt64(42)

	value, err := inter.Invoke("test", newValue)
	require.NoError(t, err)

	assert.Equal(t, newValue, value)
}

func TestInterpretStructureDeclarationWithFunction(t *testing.T) {

	t.Parallel()

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

	newValue := interpreter.NewIntValueFromInt64(42)

	value, err := inter.Invoke("test", newValue)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t, newValue, inter.Globals["value"].Value)
}

func TestInterpretStructureFunctionCall(t *testing.T) {

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(42),
		inter.Globals["value"].Value,
	)
}

func TestInterpretStructureFieldAssignment(t *testing.T) {

	t.Parallel()

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

	test := inter.Globals["test"].Value.(*interpreter.CompositeValue)

	assert.True(t, test.IsModified())

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		test.GetField("foo"),
	)

	value, err := inter.Invoke("callTest")
	require.NoError(t, err)

	assert.True(t, test.IsModified())

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		test.GetField("foo"),
	)
}

func TestInterpretStructureInitializesConstant(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      struct Test {
          let foo: Int

          init() {
              self.foo = 42
          }
      }

      let test = Test()
    `)

	actual := inter.Globals["test"].Value.(*interpreter.CompositeValue).
		GetMember(inter, interpreter.LocationRange{}, "foo")
	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		actual,
	)
}

func TestInterpretStructureFunctionMutatesSelf(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)
}

func TestInterpretFunctionPreCondition(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          pre {
              x == 0
          }
          return x
      }
    `)

	_, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(42),
	)
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := interpreter.NewIntValueFromInt64(0)
	value, err := inter.Invoke("test", zero)
	require.NoError(t, err)

	assert.Equal(t, zero, value)
}

func TestInterpretFunctionPostCondition(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
    `)

	_, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(42),
	)
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := interpreter.NewIntValueFromInt64(0)
	value, err := inter.Invoke("test", zero)
	require.NoError(t, err)

	assert.Equal(t, zero, value)
}

func TestInterpretFunctionWithResultAndPostConditionWithResult(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              result == 0
          }
          return x
      }
    `)

	_, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(42),
	)
	assert.IsType(t, &interpreter.ConditionError{}, err)

	zero := interpreter.NewIntValueFromInt64(0)
	value, err := inter.Invoke("test", zero)
	require.NoError(t, err)

	assert.Equal(t, zero, value)
}

func TestInterpretFunctionWithoutResultAndPostConditionWithResult(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretFunctionPostConditionWithBefore(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretFunctionPostConditionWithBeforeFailingPreCondition(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int {
          post {
              y == 0: "y should be zero"
          }
          let y = x
          return y
      }
    `)

	_, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(42),
	)
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"y should be zero",
		err.(*interpreter.ConditionError).Message,
	)

	zero := interpreter.NewIntValueFromInt64(0)
	value, err := inter.Invoke("test", zero)
	require.NoError(t, err)

	assert.Equal(t, zero, value)
}

func TestInterpretFunctionPostConditionWithMessageUsingResult(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): String {
          post {
              y == 0: result
          }
          let y = x
          return "return value"
      }
    `)

	_, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(42),
	)
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"return value",
		err.(*interpreter.ConditionError).Message,
	)

	zero := interpreter.NewIntValueFromInt64(0)
	value, err := inter.Invoke("test", zero)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("return value"),
		value,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingBefore(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: String): String {
          post {
              1 == 2: before(x)
          }
          return "return value"
      }
    `)

	_, err := inter.Invoke("test", interpreter.NewStringValue("parameter value"))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"parameter value",
		err.(*interpreter.ConditionError).Message,
	)
}

func TestInterpretFunctionPostConditionWithMessageUsingParameter(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: String): String {
          post {
              1 == 2: x
          }
          return "return value"
      }
    `)

	_, err := inter.Invoke("test", interpreter.NewStringValue("parameter value"))
	assert.IsType(t, &interpreter.ConditionError{}, err)

	assert.Equal(t,
		"parameter value",
		err.(*interpreter.ConditionError).Message,
	)
}

func TestInterpretStructCopyOnDeclaration(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnDeclarationModifiedWithStructFunction(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnIdentifierAssignment(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnIndexingAssignment(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnMemberAssignment(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		),
		value,
	)
}

func TestInterpretStructCopyOnPassing(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretArrayCopy(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(0),
			interpreter.NewIntValueFromInt64(1),
		),
		value,
	)
}

func TestInterpretStructCopyInArray(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(1),
		),
		value,
	)
}

func TestInterpretMutuallyRecursiveFunctions(t *testing.T) {

	t.Parallel()

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

	four := interpreter.NewIntValueFromInt64(4)

	value, err := inter.Invoke("isEven", four)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("isOdd", four)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretUseBeforeDeclaration(t *testing.T) {

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["tests"].Value,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["tests"].Value,
	)

	value, err = inter.Invoke("test")
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["tests"].Value,
	)
}

func TestInterpretOptionalVariableDeclaration(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: Int?? = 2
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalParameterInvokedExternal(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int??): Int?? {
          return x
      }
    `)

	value, err := inter.Invoke(
		"test",
		interpreter.NewIntValueFromInt64(2),
	)
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
		),
		value,
	)
}

func TestInterpretOptionalParameterInvokedInternal(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun testActual(x: Int??): Int?? {
          return x
      }

      fun test(): Int?? {
          return testActual(x: 2)
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
		),
		value,
	)
}

func TestInterpretOptionalReturn(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(x: Int): Int?? {
          return x
      }
    `)

	value, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(2))
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
		),
		value,
	)
}

func TestInterpretOptionalAssignment(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var x: Int?? = 1

      fun test() {
          x = 2
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int? = nil
   `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalNestingNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = nil
   `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilReturnValue(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     fun test(): Int?? {
         return nil
     }
   `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NilValue{},
		value,
	)
}

func TestInterpretSomeReturnValue(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     fun test(): Int? {
         let x: Int? = 1
         return x
     }
   `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		value,
	)
}

func TestInterpretSomeReturnValueFromDictionary(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     fun test(): Int? {
         let foo: {String: Int} = {"a": 1}
         return foo["a"]
     }
   `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		value,
	)
}

func TestInterpretNilCoalescingNilIntToOptional(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int? = nil
      let x: Int? = none ?? one
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilIntToOptionals(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int?? = nil
      let x: Int? = none ?? one
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilIntToOptionalNilLiteral(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let x: Int? = nil ?? one
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingRightSubtype(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: Int? = nil ?? nil
    `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilInt(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let none: Int? = nil
      let x: Int = none ?? one
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingNilLiteralInt(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let one = 1
      let x: Int = nil ?? one
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNilCoalescingShortCircuitLeftSuccess(t *testing.T) {

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(1),
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

	t.Parallel()

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
		interpreter.NewIntValueFromInt64(2),
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

func TestInterpretNilCoalescingOptionalAnyStructNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = nil
      let y = x ?? true
    `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingOptionalAnyStructSome(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = 2
      let y = x ?? true
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["y"].Value,
	)
}

func TestInterpretNilCoalescingOptionalRightHandSide(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: Int? = 1
      let y: Int? = 2
      let z = x ?? y
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilCoalescingBothOptional(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = 1
     let y: Int? = 2
     let z = x ?? y
   `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilCoalescingBothOptionalLeftNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int?? = nil
     let y: Int? = 2
     let z = x ?? y
   `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(2),
		),
		inter.Globals["z"].Value,
	)
}

func TestInterpretNilsComparison(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = nil == nil
   `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["x"].Value,
	)
}

func TestInterpretNonOptionalNilComparison(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: Int = 1
      let y = x == nil
      let z = nil == x
   `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["z"].Value,
	)
}

func TestInterpretOptionalNilComparison(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

func TestInterpretOptionalSomeValueComparison(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int? = 1
     let y = x == 1
   `)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalNilValueComparison(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
     let x: Int? = nil
     let y = x == 1
   `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["y"].Value,
	)
}

func TestInterpretCompositeNilEquality(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindEvent {
			continue
		}

		var setupCode, identifier string
		if compositeKind == common.CompositeKindContract {
			identifier = "X"
		} else {
			setupCode = fmt.Sprintf(
				`pub let x: %[1]sX? %[2]s %[3]s X%[4]s`,
				compositeKind.Annotation(),
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind, ""),
			)
			identifier = "x"
		}

		t.Run(compositeKind.Name(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s X {}

                      %[2]s

                      pub let y = %[3]s == nil
                      pub let z = nil == %[3]s
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(nil, nil, nil),
					},
				},
			)

			assert.Equal(t,
				interpreter.BoolValue(false),
				inter.Globals["y"].Value,
			)

			assert.Equal(t,
				interpreter.BoolValue(false),
				inter.Globals["z"].Value,
			)
		})
	}
}

func TestInterpretIfStatementTestWithDeclaration(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var branch = 0

      fun test(x: Int?): Int {
          if var y = x {
              branch = 1
              return y
          } else {
              branch = 2
              return 0
          }
      }
    `)

	t.Run("2", func(t *testing.T) {
		value, err := inter.Invoke(
			"test",
			interpreter.NewIntValueFromInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(1),
			inter.Globals["branch"].Value,
		)
	})

	t.Run("nil", func(t *testing.T) {
		value, err := inter.Invoke("test", interpreter.NilValue{})
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(0),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			inter.Globals["branch"].Value,
		)
	})
}

func TestInterpretIfStatementTestWithDeclarationAndElse(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var branch = 0

      fun test(x: Int?): Int {
          if var y = x {
              branch = 1
              return y
          }
          branch = 2
          return 0
      }
    `)

	t.Run("2", func(t *testing.T) {
		value, err := inter.Invoke(
			"test",
			interpreter.NewIntValueFromInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(1),
			inter.Globals["branch"].Value,
		)
	})

	t.Run("nil", func(t *testing.T) {
		value, err := inter.Invoke("test", interpreter.NilValue{})
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(0),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			inter.Globals["branch"].Value,
		)

	})
}

func TestInterpretIfStatementTestWithDeclarationNestedOptionals(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var branch = 0

      fun test(x: Int??): Int? {
          if var y = x {
              branch = 1
              return y
          } else {
              branch = 2
              return 0
          }
      }
    `)

	t.Run("2", func(t *testing.T) {
		value, err := inter.Invoke(
			"test",
			interpreter.NewIntValueFromInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(1),
			inter.Globals["branch"].Value,
		)
	})

	t.Run("nil", func(t *testing.T) {
		value, err := inter.Invoke("test", interpreter.NilValue{})
		require.NoError(t, err)

		assert.Equal(t,
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(0),
			),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			inter.Globals["branch"].Value,
		)
	})
}

func TestInterpretIfStatementTestWithDeclarationNestedOptionalsExplicitAnnotation(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var branch = 0

      fun test(x: Int??): Int? {
          if var y: Int? = x {
              branch = 1
              return y
          } else {
              branch = 2
              return 0
          }
      }
    `)

	t.Run("2", func(t *testing.T) {
		value, err := inter.Invoke(
			"test",
			interpreter.NewIntValueFromInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(2),
			),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(1),
			inter.Globals["branch"].Value,
		)

	})

	t.Run("nil", func(t *testing.T) {
		value, err := inter.Invoke("test", interpreter.NilValue{})
		require.NoError(t, err)
		assert.Equal(t,
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(0),
			),
			value,
		)
		assert.Equal(t,
			interpreter.NewIntValueFromInt64(2),
			inter.Globals["branch"].Value,
		)
	})
}

func TestInterpretInterfaceConformanceNoRequirements(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		interfaceType := AsInterfaceType("Test", compositeKind)

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test {}

                      pub %[1]s TestImpl: Test {}

                      pub let test: %[2]s%[3]s %[4]s %[5]s TestImpl%[6]s
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					interfaceType,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind, ""),
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(nil, nil, nil),
					},
				},
			)

			assert.IsType(t,
				&interpreter.CompositeValue{},
				inter.Globals["test"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFieldUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		var setupCode, identifier string
		if compositeKind == common.CompositeKindContract {
			identifier = "TestImpl"
		} else {
			interfaceType := AsInterfaceType("Test", compositeKind)

			setupCode = fmt.Sprintf(
				`pub let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				interfaceType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind, "x: 1"),
			)
			identifier = "test"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test {
                          pub x: Int
                      }

                      pub %[1]s TestImpl: Test {
                          pub var x: Int

                          init(x: Int) {
                              self.x = x
                          }
                      }

                      %[2]s

                      pub let x = %[3]s.x
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(
							[]interpreter.Value{
								interpreter.NewIntValueFromInt64(1),
							},
							[]sema.Type{
								&sema.IntType{},
							},
							[]sema.Type{
								&sema.IntType{},
							},
						),
					},
				},
			)

			assert.Equal(t,
				interpreter.NewIntValueFromInt64(1),
				inter.Globals["x"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFunctionUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		var setupCode, identifier string
		if compositeKind == common.CompositeKindContract {
			identifier = "TestImpl"
		} else {
			interfaceType := AsInterfaceType("Test", compositeKind)

			setupCode = fmt.Sprintf(
				`pub let test: %[1]s %[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				interfaceType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind, ""),
			)
			identifier = "test"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test {
                          pub fun test(): Int
                      }

                      pub %[1]s TestImpl: Test {
                          pub fun test(): Int {
                              return 2
                          }
                      }

                      %[2]s

                      pub let val = %[3]s.test()
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(nil, nil, nil),
					},
				},
			)

			assert.Equal(t,
				interpreter.NewIntValueFromInt64(2),
				inter.Globals["val"].Value,
			)
		})
	}
}

func TestInterpretInterfaceFunctionUseWithPreCondition(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		var setupCode, tearDownCode, identifier string

		if compositeKind == common.CompositeKindContract {
			identifier = "TestImpl"
		} else {
			interfaceType := AsInterfaceType("Test", compositeKind)

			setupCode = fmt.Sprintf(
				`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				interfaceType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind, ""),
			)
			identifier = "test"
		}

		if compositeKind == common.CompositeKindResource {
			tearDownCode = `destroy test`
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			inter := parseCheckAndInterpretWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test {
                          pub fun test(x: Int): Int {
                              pre {
                                  x > 0: "x must be positive"
                              }
                          }
                      }

                      pub %[1]s TestImpl: Test {
                          pub fun test(x: Int): Int {
                              pre {
                                  x < 2: "x must be smaller than 2"
                              }
                              return x
                          }
                      }

                      pub fun callTest(x: Int): Int {
                          %[2]s
                          let res = %[3]s.test(x: x)
                          %[4]s
                          return res
                      }
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
					tearDownCode,
				),
				ParseCheckAndInterpretOptions{
					Options: []interpreter.Option{
						makeContractValueHandler(nil, nil, nil),
					},
				},
			)

			_, err := inter.Invoke("callTest", interpreter.NewIntValueFromInt64(0))
			assert.IsType(t, &interpreter.ConditionError{}, err)

			value, err := inter.Invoke("callTest", interpreter.NewIntValueFromInt64(1))
			require.NoError(t, err)

			assert.Equal(t,
				interpreter.NewIntValueFromInt64(1),
				value,
			)

			_, err = inter.Invoke("callTest", interpreter.NewIntValueFromInt64(2))
			assert.IsType(t,
				&interpreter.ConditionError{},
				err,
			)
		})
	}
}

func TestInterpretInitializerWithInterfacePreCondition(t *testing.T) {

	t.Parallel()

	tests := map[int64]error{
		0: &interpreter.ConditionError{},
		1: nil,
		2: &interpreter.ConditionError{},
	}

	for _, compositeKind := range common.CompositeKindsWithBody {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			for value, expectedError := range tests {

				t.Run(fmt.Sprint(value), func(t *testing.T) {

					var testFunction string
					if compositeKind != common.CompositeKindContract {

						interfaceType := AsInterfaceType("Test", compositeKind)

						testFunction =
							fmt.Sprintf(
								`
					               pub fun test(x: Int): %[1]s%[2]s {
					                   return %[3]s %[4]s TestImpl%[5]s
					               }
                                `,
								compositeKind.Annotation(),
								interfaceType,
								compositeKind.MoveOperator(),
								compositeKind.ConstructionKeyword(),
								constructorArguments(compositeKind, "x: x"),
							)
					}

					checker, err := checker.ParseAndCheck(t,
						fmt.Sprintf(
							`
					             pub %[1]s interface Test {
					                 init(x: Int) {
					                     pre {
					                         x > 0: "x must be positive"
					                     }
					                 }
					             }

					             pub %[1]s TestImpl: Test {
					                 init(x: Int) {
					                     pre {
					                         x < 2: "x must be smaller than 2"
					                     }
					                 }
					             }

					             %[2]s
					           `,
							compositeKind.Keyword(),
							testFunction,
						),
					)
					require.NoError(t, err)

					check := func(err error) {
						if expectedError == nil {
							require.NoError(t, err)
						} else {
							assert.IsType(t, expectedError, err)
						}
					}

					uuidHandler := interpreter.WithUUIDHandler(func() uint64 {
						return 0
					})

					if compositeKind == common.CompositeKindContract {

						inter, err := interpreter.NewInterpreter(
							checker,
							makeContractValueHandler(
								[]interpreter.Value{
									interpreter.NewIntValueFromInt64(value),
								},
								[]sema.Type{
									&sema.IntType{},
								},
								[]sema.Type{
									&sema.IntType{},
								},
							),
							uuidHandler,
						)
						require.NoError(t, err)

						err = inter.Interpret()
						check(err)
					} else {
						inter, err := interpreter.NewInterpreter(checker, uuidHandler)
						require.NoError(t, err)

						err = inter.Interpret()
						require.NoError(t, err)

						_, err = inter.Invoke("test", interpreter.NewIntValueFromInt64(value))
						check(err)
					}
				})
			}
		})
	}
}

func TestInterpretTypeRequirementWithPreCondition(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`

          pub struct interface Also {
             pub fun test(x: Int) {
                 pre {
                     x >= 0: "x >= 0"
                 }
             }
          }

          pub contract interface Test {

              pub struct Nested {
                  pub fun test(x: Int) {
                      pre {
                          x >= 1: "x >= 1"
                      }
                  }
              }
          }

          pub contract TestImpl: Test {

              pub struct Nested: Also {
                  pub fun test(x: Int) {
                      pre {
                          x < 2: "x < 2"
                      }
                  }
              }
          }

          pub fun test(x: Int) {
              TestImpl.Nested().test(x: x)
          }
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	t.Run("-1", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(-1))
		require.IsType(t,
			&interpreter.ConditionError{},
			err,
		)

		// NOTE: The type requirement condition (`Test.Nested`) is evaluated first,
		//  before the type's conformances (`Also`)

		assert.Equal(t, err.(*interpreter.ConditionError).Message, "x >= 1")
	})

	t.Run("0", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(0))
		assert.IsType(t,
			&interpreter.ConditionError{},
			err,
		)
		assert.Equal(t, "x >= 1", err.(*interpreter.ConditionError).Message)
	})

	t.Run("1", func(t *testing.T) {
		value, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(1))
		require.NoError(t, err)

		assert.IsType(t,
			interpreter.VoidValue{},
			value,
		)
	})

	t.Run("2", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(2))
		assert.IsType(t,
			&interpreter.ConditionError{},
			err,
		)
	})
}

func TestInterpretImport(t *testing.T) {

	t.Parallel()

	checkerImported, err := checker.ParseAndCheck(t, `
      pub fun answer(): Int {
          return 42
      }
    `)
	require.NoError(t, err)

	checkerImporting, err := checker.ParseAndCheckWithOptions(t,
		`
          import answer from "imported"

          pub fun test(): Int {
              return answer()
          }
        `,
		checker.ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				assert.Equal(t,
					ImportedLocation,
					location,
				)
				return checkerImported.Program, nil
			},
		},
	)
	require.NoError(t, err)

	inter, err := interpreter.NewInterpreter(checkerImporting)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		value,
	)
}

func TestInterpretImportError(t *testing.T) {

	t.Parallel()

	valueDeclarations :=
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations()

	checkerImported, err := checker.ParseAndCheckWithOptions(t,
		`
          pub fun answer(): Int {
              return panic("?!")
          }
        `,
		checker.ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
		},
	)
	require.NoError(t, err)

	checkerImporting, err := checker.ParseAndCheckWithOptions(t,
		`
          import answer from "imported"

          pub fun test(): Int {
              return answer()
          }
        `,
		checker.ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				assert.Equal(t,
					ImportedLocation,
					location,
				)
				return checkerImported.Program, nil
			},
		},
	)
	require.NoError(t, err)

	values := stdlib.StandardLibraryFunctions{
		stdlib.PanicFunction,
	}.ToValues()

	inter, err := interpreter.NewInterpreter(
		checkerImporting,
		interpreter.WithPredefinedValues(values),
	)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	_, err = inter.Invoke("test")

	assert.IsType(t, stdlib.PanicError{}, err)
	assert.Equal(t,
		"?!",
		err.(stdlib.PanicError).Message,
	)
}

func TestInterpretDictionary(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"a": 1, "b": 2}
    `)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("a"), interpreter.NewIntValueFromInt64(1),
		interpreter.NewStringValue("b"), interpreter.NewIntValueFromInt64(2),
	).Copy()

	actualDict := inter.Globals["x"].Value

	assert.Equal(t,
		expectedDict,
		actualDict,
	)

	assert.True(t, actualDict.IsModified())
}

func TestInterpretDictionaryInsertionOrder(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"c": 3, "a": 1, "b": 2}
    `)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("c"), interpreter.NewIntValueFromInt64(3),
		interpreter.NewStringValue("a"), interpreter.NewIntValueFromInt64(1),
		interpreter.NewStringValue("b"), interpreter.NewIntValueFromInt64(2),
	).Copy()

	actualDict := inter.Globals["x"].Value

	assert.Equal(t,
		expectedDict,
		actualDict,
	)

	assert.True(t, actualDict.IsModified())
}

func TestInterpretDictionaryIndexingString(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"abc": 1, "def": 2}
      let a = x["abc"]
      let b = x["def"]
      let c = x["ghi"]
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(2),
		),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["c"].Value,
	)
}

func TestInterpretDictionaryIndexingBool(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {true: 1, false: 2}
      let a = x[true]
      let b = x[false]
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(2),
		),
		inter.Globals["b"].Value,
	)
}

func TestInterpretDictionaryIndexingInt(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {23: "a", 42: "b"}
      let a = x[23]
      let b = x[42]
      let c = x[100]
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewStringValue("a"),
		),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewStringValue("b"),
		),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["c"].Value,
	)
}

func TestInterpretDictionaryIndexingAssignmentExisting(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"abc": 42}
      fun test() {
          x["abc"] = 23
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("abc"), interpreter.NewIntValueFromInt64(42),
	).Copy().(*interpreter.DictionaryValue)
	expectedDict.Set(
		inter,
		interpreter.LocationRange{},
		interpreter.NewStringValue("abc"),
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(23),
		),
	)

	actualDict := inter.Globals["x"].Value.(*interpreter.DictionaryValue)

	require.Equal(t,
		expectedDict,
		actualDict,
	)

	newValue := actualDict.
		Get(inter, interpreter.LocationRange{}, interpreter.NewStringValue("abc"))

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(interpreter.NewIntValueFromInt64(23)),
		newValue,
	)

	assert.Equal(t,
		map[string]interpreter.Value{
			"abc": interpreter.NewIntValueFromInt64(23),
		},
		actualDict.Entries,
	)

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewStringValue("abc"),
		},
		actualDict.Keys.Values,
	)

	assert.True(t, actualDict.IsModified())
}

func TestInterpretDictionaryIndexingAssignmentNew(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"def": 42}
      fun test() {
          x["abc"] = 23
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("def"), interpreter.NewIntValueFromInt64(42),
	).Copy().(*interpreter.DictionaryValue)
	expectedDict.Set(
		inter,
		interpreter.LocationRange{},
		interpreter.NewStringValue("abc"),
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(23),
		),
	)

	actualDict := inter.Globals["x"].Value.(*interpreter.DictionaryValue)

	require.Equal(t,
		expectedDict,
		actualDict,
	)

	newValue := actualDict.
		Get(inter, interpreter.LocationRange{}, interpreter.NewStringValue("abc"))

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(interpreter.NewIntValueFromInt64(23)),
		newValue,
	)

	assert.Equal(t,
		map[string]interpreter.Value{
			"def": interpreter.NewIntValueFromInt64(42),
			"abc": interpreter.NewIntValueFromInt64(23),
		},
		actualDict.Entries,
	)

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewStringValue("def"),
			interpreter.NewStringValue("abc"),
		},
		actualDict.Keys.Values,
	)

	assert.True(t, actualDict.IsModified())
}

func TestInterpretDictionaryIndexingAssignmentNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = {"def": 42, "abc": 23}
      fun test() {
          x["def"] = nil
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("def"), interpreter.NewIntValueFromInt64(42),
		interpreter.NewStringValue("abc"), interpreter.NewIntValueFromInt64(23),
	).Copy().(*interpreter.DictionaryValue)
	expectedDict.Set(
		inter,
		interpreter.LocationRange{},
		interpreter.NewStringValue("def"),
		interpreter.NilValue{},
	)

	actualDict := inter.Globals["x"].Value.(*interpreter.DictionaryValue)

	require.Equal(t,
		expectedDict,
		actualDict,
	)

	newValue := actualDict.
		Get(inter, interpreter.LocationRange{}, interpreter.NewStringValue("def"))

	assert.Equal(t,
		interpreter.NilValue{},
		newValue,
	)

	assert.Equal(t,
		map[string]interpreter.Value{
			"abc": interpreter.NewIntValueFromInt64(23),
		},
		actualDict.Entries,
	)

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewStringValue("abc"),
		},
		actualDict.Keys.Values,
	)

	assert.True(t, actualDict.IsModified())
}

func TestInterpretOptionalAnyStruct(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = 42
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["x"].Value,
	)
}

func TestInterpretOptionalAnyStructFailableCasting(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = 42
      let y = (x ?? 23) as? Int
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["y"].Value,
	)
}

func TestInterpretOptionalAnyStructFailableCastingInt(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = 23
      let y = x ?? 42
      let z = y as? Int
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(23),
		),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(23),
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(23),
		),
		inter.Globals["z"].Value,
	)
}

func TestInterpretOptionalAnyStructFailableCastingNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x: AnyStruct? = nil
      let y = x ?? 42
      let z = y as? Int
    `)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		inter.Globals["y"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["z"].Value,
	)
}

func TestInterpretLength(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(4),
		inter.Globals["x"].Value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		inter.Globals["y"].Value,
	)
}

func TestInterpretStructureFunctionBindingInside(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretStructureFunctionBindingOutside(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretArrayAppend(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let xs = [1, 2, 3]

      fun test() {
          xs.append(4)
      }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(1),
		interpreter.NewIntValueFromInt64(2),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.Append(interpreter.NewIntValueFromInt64(4))

	actualArray := inter.Globals["xs"].Value

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(4),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)
}

func TestInterpretArrayAppendBound(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(4),
		),
		value,
	)
}

func TestInterpretArrayConcat(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let a = [1, 2]
          return a.concat([3, 4])
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(4),
		),
		value,
	)
}

func TestInterpretArrayConcatBound(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let a = [1, 2]
          let b = a.concat
          return b([3, 4])
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(4),
		),
		value,
	)
}

func TestInterpretArrayInsert(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = [1, 2, 3]

      fun test() {
          x.insert(at: 1, 4)
      }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(1),
		interpreter.NewIntValueFromInt64(2),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.Insert(1, interpreter.NewIntValueFromInt64(4))

	actualArray := inter.Globals["x"].Value

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(4),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)
}

func TestInterpretArrayRemove(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = [1, 2, 3]
      let y = x.remove(at: 1)
    `)

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(1),
		interpreter.NewIntValueFromInt64(2),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.Remove(1)

	actualArray := inter.Globals["x"].Value

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(3),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayRemoveFirst(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = [1, 2, 3]
      let y = x.removeFirst()
    `)

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(1),
		interpreter.NewIntValueFromInt64(2),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.RemoveFirst()

	actualArray := inter.Globals["x"].Value

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(3),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayRemoveLast(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
          let x = [1, 2, 3]
          let y = x.removeLast()
    `)

	expectedArray := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(1),
		interpreter.NewIntValueFromInt64(2),
		interpreter.NewIntValueFromInt64(3),
	).Copy().(*interpreter.ArrayValue)
	expectedArray.RemoveLast()

	actualArray := inter.Globals["x"].Value

	require.Equal(t,
		expectedArray,
		actualArray,
	)

	assert.True(t, actualArray.IsModified())

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
		},
		actualArray.(*interpreter.ArrayValue).Values,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		inter.Globals["y"].Value,
	)
}

func TestInterpretArrayContains(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(true),
		value,
	)

	value, err = inter.Invoke("doesNotContain")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

func TestInterpretStringConcat(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let a = "abc"
          return a.concat("def")
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("abcdef"),
		value,
	)
}

func TestInterpretStringConcatBound(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): String {
          let a = "abc"
          let b = a.concat
          return b("def")
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewStringValue("abcdef"),
		value,
	)
}

func TestInterpretDictionaryRemove(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let xs = {"abc": 1, "def": 2}
      let removed = xs.remove(key: "abc")
    `)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("abc"), interpreter.NewIntValueFromInt64(1),
		interpreter.NewStringValue("def"), interpreter.NewIntValueFromInt64(2),
	).Copy().(*interpreter.DictionaryValue)
	expectedDict.Remove(nil, interpreter.LocationRange{}, interpreter.NewStringValue("abc"))

	actualDict := inter.Globals["xs"].Value.(*interpreter.DictionaryValue)

	assert.Equal(t,
		expectedDict,
		actualDict,
	)

	assert.Equal(t,
		map[string]interpreter.Value{
			"def": interpreter.NewIntValueFromInt64(2),
		},
		actualDict.Entries,
	)

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewStringValue("def"),
		},
		actualDict.Keys.Values,
	)

	assert.True(t, actualDict.IsModified())

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["removed"].Value,
	)
}

func TestInterpretDictionaryInsert(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let xs = {"abc": 1, "def": 2}
      let inserted = xs.insert(key: "abc", 3)
    `)

	expectedDict := interpreter.NewDictionaryValueUnownedNonCopying(
		interpreter.NewStringValue("abc"), interpreter.NewIntValueFromInt64(1),
		interpreter.NewStringValue("def"), interpreter.NewIntValueFromInt64(2),
	).Copy().(*interpreter.DictionaryValue)
	expectedDict.Insert(
		nil,
		interpreter.LocationRange{},
		interpreter.NewStringValue("abc"),
		interpreter.NewIntValueFromInt64(3),
	)

	actualDict := inter.Globals["xs"].Value.(*interpreter.DictionaryValue)

	require.Equal(t,
		expectedDict,
		actualDict,
	)

	assert.Equal(t,
		map[string]interpreter.Value{
			"abc": interpreter.NewIntValueFromInt64(3),
			"def": interpreter.NewIntValueFromInt64(2),
		},
		actualDict.Entries,
	)

	assert.Equal(t,
		[]interpreter.Value{
			interpreter.NewStringValue("abc"),
			interpreter.NewStringValue("def"),
		},
		actualDict.Keys.Values,
	)

	assert.True(t, actualDict.IsModified())

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(1),
		),
		inter.Globals["inserted"].Value,
	)
}

func TestInterpretDictionaryKeys(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): [String] {
          let dict = {"def": 2, "abc": 1}
          dict.insert(key: "a", 3)
          return dict.keys
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewStringValue("def"),
			interpreter.NewStringValue("abc"),
			interpreter.NewStringValue("a"),
		),
		value,
	)
}

func TestInterpretDictionaryValues(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      fun test(): [Int] {
          let dict = {"def": 2, "abc": 1}
          dict.insert(key: "a", 3)
          return dict.values
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(3),
		),
		value,
	)
}

func TestInterpretDictionaryKeyTypes(t *testing.T) {

	t.Parallel()

	tests := map[string]string{
		"String":    `"abc"`,
		"Character": `"X"`,
		"Address":   `0x1`,
		"Bool":      `true`,
	}

	for _, integerType := range sema.AllIntegerTypes {
		tests[integerType.String()] = `42`
	}

	for _, fixedPointType := range sema.AllFixedPointTypes {
		tests[fixedPointType.String()] = `1.23`
	}

	for ty, code := range tests {
		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let k: %s = %s
                      let xs = {k: "test"}
                      let v = xs[k]
                    `,
					ty,
					code,
				),
			)

			assert.Equal(t,
				interpreter.NewSomeValueOwningNonCopying(
					interpreter.NewStringValue("test"),
				),
				inter.Globals["v"].Value,
			)
		})
	}
}

func TestInterpretIndirectDestroy(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretUnaryMove(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun foo(x: @X): @X {
          return <-x
      }

      fun bar() {
          let x <- foo(x: <-create X())
          destroy x
      }
    `)

	value, err := inter.Invoke("bar")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)
}

func TestInterpretResourceMoveInArrayAndDestroy(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destroys = 0

      resource Foo {
          var bar: Int

          init(bar: Int) {
              self.bar = bar
          }

          destroy() {
              destroys = destroys + 1
          }
      }

      fun test(): Int {
          let foo1 <- create Foo(bar: 1)
          let foo2 <- create Foo(bar: 2)
          let foos <- [<-foo1, <-foo2]
          let bar = foos[1].bar
          destroy foos
          return bar
      }
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destroys"].Value,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["destroys"].Value,
	)
}

func TestInterpretResourceMoveInDictionaryAndDestroy(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destroys = 0

      resource Foo {
          var bar: Int

          init(bar: Int) {
              self.bar = bar
          }

          destroy() {
              destroys = destroys + 1
          }
      }

      fun test() {
          let foo1 <- create Foo(bar: 1)
          let foo2 <- create Foo(bar: 2)
          let foos <- {"foo1": <-foo1, "foo2": <-foo2}
          destroy foos
      }
    `)

	require.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destroys"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["destroys"].Value,
	)
}

func TestInterpretClosure(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		value,
	)

	value, err = inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		value,
	)

	value, err = inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(3),
		value,
	)
}

// TestInterpretCompositeFunctionInvocationFromImportingProgram checks
// that member functions of imported composites can be invoked from an importing program.
// See https://github.com/dapperlabs/flow-go/issues/838
//
func TestInterpretCompositeFunctionInvocationFromImportingProgram(t *testing.T) {

	t.Parallel()

	checkerImported, err := checker.ParseAndCheck(t, `
      // function must have arguments
      pub fun x(x: Int) {}

      // invocation must be in composite
      pub struct Y {

          pub fun x() {
              x(x: 1)
          }
      }
    `)
	require.NoError(t, err)

	checkerImporting, err := checker.ParseAndCheckWithOptions(t,
		`
          import Y from "imported"

          pub fun test() {
              // get member must bind using imported interpreter
              Y().x()
          }
        `,
		checker.ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				assert.Equal(t,
					ImportedLocation,
					location,
				)
				return checkerImported.Program, nil
			},
		},
	)
	require.NoError(t, err)

	inter, err := interpreter.NewInterpreter(checkerImporting)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	_, err = inter.Invoke("test")
	require.NoError(t, err)
}

func TestInterpretSwapVariables(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): [Int] {
           var x = 2
           var y = 3
           x <-> y
           return [x, y]
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(3),
			interpreter.NewIntValueFromInt64(2),
		),
		value,
	)
}

func TestInterpretSwapArrayAndField(t *testing.T) {

	t.Parallel()

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
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(1),
		),
		value,
	)
}

func TestInterpretResourceDestroyExpressionNoDestructor(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       resource R {}

       fun test() {
           let r <- create R()
           destroy r
       }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)
}

func TestInterpretResourceDestroyExpressionDestructor(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       var ranDestructor = false

       resource R {
           destroy() {
               ranDestructor = true
           }
       }

       fun test() {
           let r <- create R()
           destroy r
       }
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["ranDestructor"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["ranDestructor"].Value,
	)
}

func TestInterpretResourceDestroyExpressionNestedResources(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var ranDestructorA = false
      var ranDestructorB = false

      resource B {
          destroy() {
              ranDestructorB = true
          }
      }

      resource A {
          let b: @B

          init(b: @B) {
              self.b <- b
          }

          destroy() {
              ranDestructorA = true
              destroy self.b
          }
      }

      fun test() {
          let b <- create B()
          let a <- create A(b: <-b)
          destroy a
      }
    `)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["ranDestructorA"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(false),
		inter.Globals["ranDestructorB"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["ranDestructorA"].Value,
	)

	assert.Equal(t,
		interpreter.BoolValue(true),
		inter.Globals["ranDestructorB"].Value,
	)
}

func TestInterpretResourceDestroyArray(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destructionCount = 0

      resource R {
          destroy() {
              destructionCount = destructionCount + 1
          }
      }

      fun test() {
          let rs <- [<-create R(), <-create R()]
          destroy rs
      }
    `)

	require.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destructionCount"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["destructionCount"].Value,
	)
}

func TestInterpretResourceDestroyDictionary(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destructionCount = 0

      resource R {
          destroy() {
              destructionCount = destructionCount + 1
          }
      }

      fun test() {
          let rs <- {"r1": <-create R(), "r2": <-create R()}
          destroy rs
      }
    `)

	require.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destructionCount"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(2),
		inter.Globals["destructionCount"].Value,
	)
}

func TestInterpretResourceDestroyOptionalSome(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destructionCount = 0

      resource R {
          destroy() {
              destructionCount = destructionCount + 1
          }
      }

      fun test() {
          let maybeR: @R? <- create R()
          destroy maybeR
      }
    `)

	require.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destructionCount"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(1),
		inter.Globals["destructionCount"].Value,
	)
}

func TestInterpretResourceDestroyOptionalNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      var destructionCount = 0

      resource R {
          destroy() {
              destructionCount = destructionCount + 1
          }
      }

      fun test() {
          let maybeR: @R? <- nil
          destroy maybeR
      }
    `)

	require.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destructionCount"].Value,
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(0),
		inter.Globals["destructionCount"].Value,
	)
}

// TestInterpretResourceDestroyExpressionResourceInterfaceCondition tests that
// the resource interface's destructor is called, even if the conforming resource
// does not have an destructor
//
func TestInterpretResourceDestroyExpressionResourceInterfaceCondition(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource interface I {
          destroy() {
              pre { false }
          }
      }

      resource R: I {}

      fun test() {
          let r <- create R()
          destroy r
      }
    `)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.ConditionError{}, err)
}

// TestInterpretInterfaceInitializer tests that the interface's initializer
// is called, even if the conforming composite does not have an initializer
//
func TestInterpretInterfaceInitializer(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      struct interface I {
          init(a a1: Bool) {
              pre { a1 }
          }
      }

      struct S: I {
          init(a a2: Bool) {}
      }

      fun test() {
          S(a: false)
      }
    `)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.ConditionError{}, err)
}

func TestInterpretEmitEvent(t *testing.T) {

	t.Parallel()

	var actualEvents []*interpreter.CompositeValue

	inter := parseCheckAndInterpret(t,
		`
          event Transfer(to: Int, from: Int)
          event TransferAmount(to: Int, from: Int, amount: Int)

          fun test() {
              emit Transfer(to: 1, from: 2)
              emit Transfer(to: 3, from: 4)
              emit TransferAmount(to: 1, from: 2, amount: 100)
          }
        `,
	)

	inter.SetOnEventEmittedHandler(
		func(_ *interpreter.Interpreter, event *interpreter.CompositeValue, eventType *sema.CompositeType) {
			actualEvents = append(actualEvents, event)
		},
	)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	transferEventType := inter.Checker.GlobalTypes["Transfer"].Type
	transferAmountEventType := inter.Checker.GlobalTypes["TransferAmount"].Type

	expectedEvents := []*interpreter.CompositeValue{
		interpreter.NewCompositeValue(
			TestLocation,
			transferEventType.ID(),
			common.CompositeKindEvent,
			map[string]interpreter.Value{
				"to":   interpreter.NewIntValueFromInt64(1),
				"from": interpreter.NewIntValueFromInt64(2),
			},
			nil,
		),
		interpreter.NewCompositeValue(
			TestLocation,
			transferEventType.ID(),
			common.CompositeKindEvent,
			map[string]interpreter.Value{
				"to":   interpreter.NewIntValueFromInt64(3),
				"from": interpreter.NewIntValueFromInt64(4),
			},
			nil,
		),
		interpreter.NewCompositeValue(
			TestLocation,
			transferAmountEventType.ID(),
			common.CompositeKindEvent,
			map[string]interpreter.Value{
				"to":     interpreter.NewIntValueFromInt64(1),
				"from":   interpreter.NewIntValueFromInt64(2),
				"amount": interpreter.NewIntValueFromInt64(100),
			},
			nil,
		),
	}

	for _, event := range expectedEvents {
		event.InitializeFunctions(inter)
	}

	assert.Equal(t, expectedEvents, actualEvents)
}

type testValue struct {
	value   interpreter.Value
	literal string
}

func (v testValue) String() string {
	if v.literal == "" {
		return fmt.Sprint(v.value)
	}
	return v.literal
}

func TestInterpretEmitEventParameterTypes(t *testing.T) {

	t.Parallel()

	validTypes := map[string]testValue{
		"String":    {value: interpreter.NewStringValue("test")},
		"Character": {value: interpreter.NewStringValue("X")},
		"Bool":      {value: interpreter.BoolValue(true)},
		"Address": {
			value:   interpreter.NewAddressValueFromBytes([]byte{0x1}),
			literal: `0x1`,
		},
		// Int*
		"Int":    {value: interpreter.NewIntValueFromInt64(42)},
		"Int8":   {value: interpreter.Int8Value(42)},
		"Int16":  {value: interpreter.Int16Value(42)},
		"Int32":  {value: interpreter.Int32Value(42)},
		"Int64":  {value: interpreter.Int64Value(42)},
		"Int128": {value: interpreter.NewInt128ValueFromInt64(42)},
		"Int256": {value: interpreter.NewInt256ValueFromInt64(42)},
		// UInt*
		"UInt":    {value: interpreter.NewUIntValueFromUint64(42)},
		"UInt8":   {value: interpreter.UInt8Value(42)},
		"UInt16":  {value: interpreter.UInt16Value(42)},
		"UInt32":  {value: interpreter.UInt32Value(42)},
		"UInt64":  {value: interpreter.UInt64Value(42)},
		"UInt128": {value: interpreter.NewUInt128ValueFromUint64(42)},
		"UInt256": {value: interpreter.NewUInt256ValueFromUint64(42)},
		// Word*
		"Word8":  {value: interpreter.Word8Value(42)},
		"Word16": {value: interpreter.Word16Value(42)},
		"Word32": {value: interpreter.Word32Value(42)},
		"Word64": {value: interpreter.Word64Value(42)},
		// Fix*
		"Fix64": {value: interpreter.Fix64Value(123000000)},
		// UFix*
		"UFix64": {value: interpreter.UFix64Value(123000000)},
	}

	for _, integerType := range sema.AllIntegerTypes {
		if _, ok := validTypes[integerType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", integerType))
		}
	}

	for _, fixedPointType := range sema.AllFixedPointTypes {
		if _, ok := validTypes[fixedPointType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", fixedPointType))
		}
	}

	tests := map[string]testValue{}

	for validType, value := range validTypes {
		tests[validType] = value

		tests[fmt.Sprintf("%s?", validType)] =
			testValue{
				value:   interpreter.NewSomeValueOwningNonCopying(value.value),
				literal: value.literal,
			}

		tests[fmt.Sprintf("[%s]", validType)] =
			testValue{
				value:   interpreter.NewArrayValueUnownedNonCopying(value.value),
				literal: fmt.Sprintf("[%s as %s]", value, validType),
			}

		tests[fmt.Sprintf("[%s; 1]", validType)] =
			testValue{
				value:   interpreter.NewArrayValueUnownedNonCopying(value.value),
				literal: fmt.Sprintf("[%s as %s]", value, validType),
			}

		tests[fmt.Sprintf("{%[1]s: %[1]s}", validType)] =
			testValue{
				value:   interpreter.NewDictionaryValueUnownedNonCopying(value.value, value.value).Copy(),
				literal: fmt.Sprintf("{%[1]s as %[2]s: %[1]s as %[2]s}", value, validType),
			}
	}

	for ty, value := range tests {

		t.Run(ty, func(t *testing.T) {

			code := fmt.Sprintf(
				`
                  event Test(_ value: %[1]s)

                  fun test() {
                      emit Test(%[2]s as %[1]s)
                  }
                `,
				ty,
				value,
			)

			inter := parseCheckAndInterpret(t, code)

			var actualEvents []*interpreter.CompositeValue

			inter.SetOnEventEmittedHandler(
				func(_ *interpreter.Interpreter, event *interpreter.CompositeValue, eventType *sema.CompositeType) {
					actualEvents = append(actualEvents, event)
				},
			)

			_, err := inter.Invoke("test")
			require.NoError(t, err)

			expectedEvents := []*interpreter.CompositeValue{
				interpreter.NewCompositeValue(
					TestLocation,
					inter.Checker.GlobalTypes["Test"].Type.ID(),
					common.CompositeKindEvent,
					map[string]interpreter.Value{
						"value": value.value,
					},
					nil,
				),
			}

			for _, event := range expectedEvents {
				event.InitializeFunctions(inter)
			}

			assert.Equal(t,
				expectedEvents,
				actualEvents,
			)

		})
	}
}

func TestInterpretSwapResourceDictionaryElementReturnSwapped(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun test(): @X? {
          let xs: @{String: X} <- {}
          var x: @X? <- create X()
          xs["foo"] <-> x
          destroy xs
          return <-x
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NilValue{},
		value,
	)
}

func TestInterpretSwapResourceDictionaryElementReturnDictionary(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun test(): @{String: X} {
          let xs: @{String: X} <- {}
          var x: @X? <- create X()
          xs["foo"] <-> x
          destroy x
          return <-xs
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	require.IsType(t,
		&interpreter.DictionaryValue{},
		value,
	)

	foo := value.(*interpreter.DictionaryValue).
		Get(inter, interpreter.LocationRange{}, interpreter.NewStringValue("foo"))

	require.IsType(t,
		&interpreter.SomeValue{},
		foo,
	)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		foo.(*interpreter.SomeValue).Value,
	)
}

func TestInterpretSwapResourceDictionaryElementRemoveUsingNil(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource X {}

      fun test(): @X? {
          let xs: @{String: X} <- {"foo": <-create X()}
          var x: @X? <- nil
          xs["foo"] <-> x
          destroy xs
          return <-x
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	require.IsType(t,
		&interpreter.SomeValue{},
		value,
	)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value.(*interpreter.SomeValue).Value,
	)
}

func TestInterpretReferenceExpression(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      pub resource R {}

      pub fun test(): &R {
          let r <- create R()
          let ref = &r as &R
          destroy r
          return ref
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	require.IsType(t,
		&interpreter.EphemeralReferenceValue{},
		value,
	)
}

func TestInterpretReferenceUse(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      pub resource R {
          pub(set) var x: Int

          init() {
              self.x = 0
          }

          pub fun setX(_ newX: Int) {
              self.x = newX
          }
      }

      pub fun test(): [Int] {
          let r <- create R()

          let ref1 = &r as &R
          let ref2 = &r as &R

          ref1.x = 1
          let x1 = ref1.x
          ref1.setX(2)
          let x2 = ref1.x

          let x3 = ref2.x
          let res = [x1, x2, x3]
          destroy r
          return res
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
			interpreter.NewIntValueFromInt64(2),
		),
		value,
	)
}

func TestInterpretReferenceUseAccess(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      pub resource R {
          pub(set) var x: Int

          init() {
              self.x = 0
          }

          pub fun setX(_ newX: Int) {
              self.x = newX
          }
      }

      pub fun test(): [Int] {
          let rs <- [<-create R()]
          let ref = &rs as &[R]
          let x0 = ref[0].x
          ref[0].x = 1
          let x1 = ref[0].x
          ref[0].setX(2)
          let x2 = ref[0].x
          let res = [x0, x1, x2]
          destroy rs
          return res
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(0),
			interpreter.NewIntValueFromInt64(1),
			interpreter.NewIntValueFromInt64(2),
		),
		value,
	)
}

func TestInterpretReferenceDereferenceFailure(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      pub resource R {
          pub fun foo() {}
      }

      pub fun test() {
          let r <- create R()
          let ref = &r as &R
          destroy r
          ref.foo()
      }
    `)

	_, err := inter.Invoke("test")
	assert.IsType(t, &interpreter.DestroyedCompositeError{}, err)
}

func TestInterpretInvalidForwardReferenceCall(t *testing.T) {

	t.Parallel()

	// TODO: improve:
	//   - call to `g` should succeed, but access to `y` should fail with error
	//   - maybe make this a static error

	assert.Panics(t, func() {
		_ = parseCheckAndInterpret(t, `
          fun f(): Int {
             return g()
          }

          let x = f()
          let y = 0

          fun g(): Int {
              return y
          }
        `)
	})
}

func TestInterpretVariableDeclarationSecondValue(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource R {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test(): @[R?] {
          let x <- create R(id: 1)
          var ys <- {"r": <-create R(id: 2)}
          // NOTE: nested move is valid here
          let z <- ys["r"] <- x

          // NOTE: nested move is invalid here
          let r <- ys.remove(key: "r")

          destroy ys

          return <-[<-z, <-r]
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	require.IsType(t,
		&interpreter.ArrayValue{},
		value,
	)

	values := value.(*interpreter.ArrayValue).Values

	require.IsType(t,
		&interpreter.SomeValue{},
		values[0],
	)

	firstValue := values[0].(*interpreter.SomeValue).Value

	require.IsType(t,
		&interpreter.CompositeValue{},
		firstValue,
	)

	firstResource := firstValue.(*interpreter.CompositeValue)

	assert.Equal(t,
		firstResource.GetField("id"),
		interpreter.NewIntValueFromInt64(2),
	)

	require.IsType(t,
		&interpreter.SomeValue{},
		values[1],
	)

	secondValue := values[1].(*interpreter.SomeValue).Value

	require.IsType(t,
		&interpreter.CompositeValue{},
		secondValue,
	)

	secondResource := secondValue.(*interpreter.CompositeValue)

	assert.Equal(t,
		secondResource.GetField("id"),
		interpreter.NewIntValueFromInt64(1),
	)
}

func TestInterpretCastingIntLiteralToInt8(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = 42 as Int8
    `)

	assert.Equal(t,
		interpreter.Int8Value(42),
		inter.Globals["x"].Value,
	)
}

func TestInterpretCastingIntLiteralToAnyStruct(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = 42 as AnyStruct
    `)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(42),
		inter.Globals["x"].Value,
	)
}

func TestInterpretCastingIntLiteralToOptional(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      let x = 42 as Int?
    `)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(interpreter.NewIntValueFromInt64(42)),
		inter.Globals["x"].Value,
	)
}

func TestInterpretCastingResourceToAnyResource(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      resource R {}

      fun test(): @AnyResource {
          let r <- create R()
          let x <- r as @AnyResource
          return <-x
      }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.IsType(t,
		&interpreter.CompositeValue{},
		value,
	)
}

func TestInterpretOptionalChainingFieldRead(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          struct Test {
              let x: Int

              init(x: Int) {
                  self.x = x
              }
          }

          let test1: Test? = nil
          let x1 = test1?.x

          let test2: Test? = Test(x: 42)
          let x2 = test2?.x
        `,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x1"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["x2"].Value,
	)
}

func TestInterpretOptionalChainingFunctionRead(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          struct Test {
              fun x(): Int {
                  return 42
              }
          }

          let test1: Test? = nil
          let x1 = test1?.x

          let test2: Test? = Test()
          let x2 = test2?.x
        `,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x1"].Value,
	)

	require.IsType(t,
		&interpreter.SomeValue{},
		inter.Globals["x2"].Value,
	)

	assert.IsType(t,
		interpreter.BoundFunctionValue{},
		inter.Globals["x2"].Value.(*interpreter.SomeValue).Value,
	)
}

func TestInterpretOptionalChainingFunctionCall(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
         struct Test {
             fun x(): Int {
                 return 42
             }
         }

         let test1: Test? = nil
         let x1 = test1?.x()

         let test2: Test? = Test()
         let x2 = test2?.x()
       `,
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["x1"].Value,
	)

	assert.Equal(t,
		interpreter.NewSomeValueOwningNonCopying(
			interpreter.NewIntValueFromInt64(42),
		),
		inter.Globals["x2"].Value,
	)
}

func TestInterpretOptionalChainingFieldReadAndNilCoalescing(t *testing.T) {

	t.Parallel()

	standardLibraryFunctions :=
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}

	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	predefinedValues := standardLibraryFunctions.ToValues()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          struct Test {
              let x: Int

              init(x: Int) {
                  self.x = x
              }
          }

          let test: Test? = Test(x: 42)
          let x = test?.x ?? panic("nil")
        `,
		ParseCheckAndInterpretOptions{
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(predefinedValues),
			},
		},
	)

	assert.Equal(t,
		inter.Globals["x"].Value,
		interpreter.NewIntValueFromInt64(42),
	)
}

func TestInterpretOptionalChainingFunctionCallAndNilCoalescing(t *testing.T) {

	t.Parallel()

	standardLibraryFunctions :=
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}

	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	predefinedValues := standardLibraryFunctions.ToValues()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          struct Test {
              fun x(): Int {
                  return 42
              }
          }

          let test: Test? = Test()
          let x = test?.x() ?? panic("nil")
        `,
		ParseCheckAndInterpretOptions{
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(predefinedValues),
			},
		},
	)

	assert.Equal(t,
		inter.Globals["x"].Value,
		interpreter.NewIntValueFromInt64(42),
	)
}

func TestInterpretCompositeDeclarationNestedTypeScopingOuterInner(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          pub contract Test {

              pub struct X {

                  pub fun test(): X {
                     return Test.x()
                  }
              }

              pub fun x(): X {
                 return X()
              }
          }

          pub let x1 = Test.x()
          pub let x2 = x1.test()
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	x1 := inter.Globals["x1"].Value
	x2 := inter.Globals["x2"].Value

	require.IsType(t,
		&interpreter.CompositeValue{},
		x1,
	)

	assert.Equal(t,
		sema.TypeID("test.Test.X"),
		x1.(*interpreter.CompositeValue).TypeID,
	)

	require.IsType(t,
		&interpreter.CompositeValue{},
		x2,
	)

	assert.Equal(t,
		sema.TypeID("test.Test.X"),
		x2.(*interpreter.CompositeValue).TypeID,
	)
}

func TestInterpretCompositeDeclarationNestedConstructor(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          pub contract Test {

              pub struct X {}
          }

          pub let x = Test.X()
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	x := inter.Globals["x"].Value

	require.IsType(t,
		&interpreter.CompositeValue{},
		x,
	)

	assert.Equal(t,
		sema.TypeID("test.Test.X"),
		x.(*interpreter.CompositeValue).TypeID,
	)
}

func TestInterpretFungibleTokenContract(t *testing.T) {

	t.Parallel()

	code := strings.Join(
		[]string{
			examples.FungibleTokenContractInterface,
			examples.ExampleFungibleTokenContract,
			`
              pub fun test(): [Int; 2] {

                  let publisher <- ExampleToken.sprout(balance: 100)
                  let receiver <- ExampleToken.sprout(balance: 0)

                  let withdrawn <- publisher.withdraw(amount: 60)
                  receiver.deposit(vault: <-withdrawn)

                  let publisherBalance = publisher.balance
                  let receiverBalance = receiver.balance

                  destroy publisher
                  destroy receiver

                  return [publisherBalance, receiverBalance]
              }
            `,
		},
		"\n",
	)

	standardLibraryFunctions :=
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}

	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	predefinedValues := standardLibraryFunctions.ToValues()

	inter := parseCheckAndInterpretWithOptions(t,
		code,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(predefinedValues),
				makeContractValueHandler(nil, nil, nil),
			},
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
		},
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(40),
			interpreter.NewIntValueFromInt64(60),
		),
		value,
	)
}

func TestInterpretContractAccountFieldUse(t *testing.T) {

	t.Parallel()

	code := `
      pub contract Test {
          pub let address: Address

          init() {
              // field 'account' can be used, as it is considered initialized
              self.address = self.account.address
          }

          pub fun test(): Address {
              return self.account.address
          }
      }

      pub let address1 = Test.address
      pub let address2 = Test.test()
    `

	addressValue := interpreter.AddressValue{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	inter := parseCheckAndInterpretWithOptions(t, code,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
				interpreter.WithInjectedCompositeFieldsHandler(
					func(
						_ *interpreter.Interpreter,
						_ ast.Location,
						_ sema.TypeID,
						_ common.CompositeKind,
					) map[string]interpreter.Value {
						panicFunction := interpreter.NewHostFunctionValue(func(invocation interpreter.Invocation) trampoline.Trampoline {
							panic(errors.NewUnreachableError())
						})
						return map[string]interpreter.Value{
							"account": interpreter.NewAuthAccountValue(
								addressValue,
								panicFunction,
								panicFunction,
								panicFunction,
								panicFunction,
							),
						}
					},
				),
			},
		},
	)

	assert.Equal(t,
		addressValue,
		inter.Globals["address1"].Value,
	)

	assert.Equal(t,
		addressValue,
		inter.Globals["address2"].Value,
	)
}

func TestInterpretConformToImportedInterface(t *testing.T) {

	t.Parallel()

	checkerImported, err := checker.ParseAndCheck(t, `
      struct interface Foo {
          fun check(answer: Int) {
              pre {
                  answer == 42
              }
          }
      }
	`)
	require.NoError(t, err)

	checkerImporting, err := checker.ParseAndCheckWithOptions(t,
		`
          import Foo from "imported"

          struct Bar: Foo {
              fun check(answer: Int) {}
          }

          fun test() {
              let bar = Bar()
              bar.check(answer: 1)
          }
        `,
		checker.ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				assert.Equal(t,
					ast.StringLocation("imported"),
					location,
				)
				return checkerImported.Program, nil
			},
		},
	)
	require.NoError(t, err)

	inter, err := interpreter.NewInterpreter(checkerImporting)
	require.NoError(t, err)

	err = inter.Interpret()
	require.NoError(t, err)

	_, err = inter.Invoke("test")
	assert.IsType(t, &interpreter.ConditionError{}, err)
}

func TestInterpretFunctionPostConditionInInterface(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      struct interface SI {
          on: Bool

          fun turnOn() {
              post {
                  self.on
              }
          }
      }

      struct S: SI {
          var on: Bool

          init() {
              self.on = false
          }

          fun turnOn() {
              self.on = true
          }
      }

      struct S2: SI {
          var on: Bool

          init() {
              self.on = false
          }

          fun turnOn() {
              // incorrect
          }
      }

      fun test() {
          S().turnOn()
      }

      fun test2() {
          S2().turnOn()
      }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	_, err = inter.Invoke("test2")
	assert.IsType(t, &interpreter.ConditionError{}, err)
}

func TestInterpretFunctionPostConditionWithBeforeInInterface(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      struct interface SI {
          on: Bool

          fun toggle() {
              post {
                  self.on != before(self.on)
              }
          }
      }

      struct S: SI {
          var on: Bool

          init() {
              self.on = false
          }

          fun toggle() {
              self.on = !self.on
          }
      }

      struct S2: SI {
          var on: Bool

          init() {
              self.on = false
          }

          fun toggle() {
              // incorrect
          }
      }

      fun test() {
          S().toggle()
      }

      fun test2() {
          S2().toggle()
      }
    `)

	_, err := inter.Invoke("test")
	require.NoError(t, err)

	_, err = inter.Invoke("test2")
	assert.IsType(t, &interpreter.ConditionError{}, err)
}

func TestInterpretContractUseInNestedDeclaration(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t, `
          pub contract C {

              pub var i: Int

              pub struct S {

                  init() {
                      C.i = C.i + 1
                  }
              }

              init () {
                  self.i = 0
                  S()
                  S()
              }
          }
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	i := inter.Globals["C"].Value.(interpreter.MemberAccessibleValue).
		GetMember(inter, interpreter.LocationRange{}, "i")

	require.IsType(t,
		interpreter.NewIntValueFromInt64(2),
		i,
	)
}

func TestInterpretResourceInterfaceInitializerAndDestructorPreConditions(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `

      resource interface RI {

          x: Int

          init(_ x: Int) {
              pre { x > 1: "invalid init" }
          }

          destroy() {
              pre { self.x < 3: "invalid destroy" }
          }
      }

      resource R: RI {

          let x: Int

          init(_ x: Int) {
              self.x = x
          }
      }

      fun test(_ x: Int) {
          let r <- create R(x)
          destroy r
      }
    `)

	t.Run("1", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(1))
		require.Error(t, err)

		require.IsType(t, &interpreter.ConditionError{}, err)
		assert.Equal(t, "invalid init", err.(*interpreter.ConditionError).Message)
	})

	t.Run("2", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(2))
		require.NoError(t, err)
	})

	t.Run("3", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(3))
		require.Error(t, err)

		require.IsType(t, &interpreter.ConditionError{}, err)
		assert.Equal(t, "invalid destroy", err.(*interpreter.ConditionError).Message)
	})
}

func TestInterpretResourceTypeRequirementInitializerAndDestructorPreConditions(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          pub contract interface CI {

              pub resource R {

                  pub x: Int

                  init(_ x: Int) {
                      pre { x > 1: "invalid init" }
                  }

                  destroy() {
                      pre { self.x < 3: "invalid destroy" }
                  }
              }
          }

          pub contract C: CI {

              pub resource R {

                  pub let x: Int

                  init(_ x: Int) {
                      self.x = x
                  }
              }

              pub fun test(_ x: Int) {
                  let r <- create C.R(x)
                  destroy r
              }
          }

          fun test(_ x: Int) {
              C.test(x)
          }
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	t.Run("1", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(1))
		require.Error(t, err)

		require.IsType(t, &interpreter.ConditionError{}, err)
		assert.Equal(t, "invalid init", err.(*interpreter.ConditionError).Message)
	})

	t.Run("2", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(2))
		require.NoError(t, err)
	})

	t.Run("3", func(t *testing.T) {
		_, err := inter.Invoke("test", interpreter.NewIntValueFromInt64(3))
		require.Error(t, err)

		require.IsType(t, &interpreter.ConditionError{}, err)
		assert.Equal(t, "invalid destroy", err.(*interpreter.ConditionError).Message)
	})
}

func TestInterpretNonStorageReference(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          resource NFT {
              var id: Int

              init(id: Int) {
                  self.id = id
              }
          }

          fun test(): Int {
              let resources <- [
                  <-create NFT(id: 1),
                  <-create NFT(id: 2)
              ]

              let nftRef = &resources[1] as &NFT
              let nftRef2 = nftRef
              nftRef2.id = 3

              let nft <- resources.remove(at: 1)
              destroy resources
              let newID = nft.id
              destroy nft

              return newID
          }
        `,
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t, interpreter.NewIntValueFromInt64(3), value)
}

func TestInterpretNonStorageReferenceAfterDestruction(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          resource NFT {
              var id: Int

              init(id: Int) {
                  self.id = id
              }
          }

          fun test(): Int {
              let nft <- create NFT(id: 1)
              let nftRef = &nft as &NFT
              destroy nft
              return nftRef.id
          }
        `,
	)

	_, err := inter.Invoke("test")
	require.Error(t, err)

	assert.IsType(t, &interpreter.DestroyedCompositeError{}, err)
}

func TestInterpretNonStorageReferenceToOptional(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          resource Foo {
              let name: String

              init(name: String) {
                  self.name = name
              }
          }


          fun testSome(): String {
              let xs: @{String: Foo} <- {"yes": <-create Foo(name: "YES")}
              let ref = &xs["yes"] as &Foo
              let name = ref.name
              destroy xs
              return name
          }

          fun testNil(): String {
              let xs: @{String: Foo} <- {}
              let ref = &xs["no"] as &Foo
              let name = ref.name
              destroy xs
              return name
          }
        `,
	)

	t.Run("some", func(t *testing.T) {
		value, err := inter.Invoke("testSome")
		require.NoError(t, err)

		assert.Equal(t, interpreter.NewStringValue("YES"), value)
	})

	t.Run("nil", func(t *testing.T) {
		_, err := inter.Invoke("testNil")
		require.Error(t, err)

		assert.IsType(t, &interpreter.DereferenceError{}, err)
	})
}

func TestInterpretFix64(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          let a = 789.00123010
          let b = 1234.056
          let c = -12345.006789
        `,
	)

	assert.Equal(t,
		interpreter.UFix64Value(78_900_123_010),
		inter.Globals["a"].Value,
	)

	assert.Equal(t,
		interpreter.UFix64Value(123_405_600_000),
		inter.Globals["b"].Value,
	)

	assert.Equal(t,
		interpreter.Fix64Value(-1_234_500_678_900),
		inter.Globals["c"].Value,
	)
}

func TestInterpretFix64Mul(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t,
		`
          let a = Fix64(1.1) * -1.1
        `,
	)

	assert.Equal(t,
		interpreter.Fix64Value(-121000000),
		inter.Globals["a"].Value,
	)
}

func TestInterpretHexDecode(t *testing.T) {

	t.Parallel()

	expected := interpreter.NewArrayValueUnownedNonCopying(
		interpreter.NewIntValueFromInt64(71),
		interpreter.NewIntValueFromInt64(111),
		interpreter.NewIntValueFromInt64(32),
		interpreter.NewIntValueFromInt64(87),
		interpreter.NewIntValueFromInt64(105),
		interpreter.NewIntValueFromInt64(116),
		interpreter.NewIntValueFromInt64(104),
		interpreter.NewIntValueFromInt64(32),
		interpreter.NewIntValueFromInt64(116),
		interpreter.NewIntValueFromInt64(104),
		interpreter.NewIntValueFromInt64(101),
		interpreter.NewIntValueFromInt64(32),
		interpreter.NewIntValueFromInt64(70),
		interpreter.NewIntValueFromInt64(108),
		interpreter.NewIntValueFromInt64(111),
		interpreter.NewIntValueFromInt64(119),
	)

	t.Run("in Cadence", func(t *testing.T) {

		standardLibraryFunctions :=
			stdlib.StandardLibraryFunctions{
				stdlib.PanicFunction,
			}

		valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
		predefinedValues := standardLibraryFunctions.ToValues()

		inter := parseCheckAndInterpretWithOptions(t,
			`
              fun hexDecode(_ s: String): [Int] {
                  if s.length % 2 != 0 {
                      panic("Input must have even number of characters")
                  }
                  let table = {
                          "0" : 0,
                          "1" : 1,
                          "2" : 2,
                          "3" : 3,
                          "4" : 4,
                          "5" : 5,
                          "6" : 6,
                          "7" : 7,
                          "8" : 8,
                          "9" : 9,
                          "a" : 10,
                          "A" : 10,
                          "b" : 11,
                          "B" : 11,
                          "c" : 12,
                          "C" : 12,
                          "d" : 13,
                          "D" : 13,
                          "e" : 14,
                          "E" : 14,
                          "f" : 15,
                          "F" : 15
                      }
                  let length = s.length / 2
                  var i = 0
                  var res: [Int] = []
                  while i < length {
                      let c = s.slice(from: i*2, upTo: i*2+1)
                      let in = table[c] ?? panic("Invalid character ".concat(c))
                      let c2 = s.slice(from: i*2+1, upTo: i*2+2)
                      let in2 = table[c2] ?? panic("Invalid character ".concat(c2))
                      res.append(16 * in + in2)
                      i = i+1
                  }
                  return res
              }

              fun test(): [Int] {
                  return hexDecode("476F20576974682074686520466C6F77")
              }
            `,
			ParseCheckAndInterpretOptions{
				CheckerOptions: []sema.Option{
					sema.WithPredeclaredValues(valueDeclarations),
				},
				Options: []interpreter.Option{
					interpreter.WithPredefinedValues(predefinedValues),
				},
			},
		)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		assert.Equal(t, expected, result)
	})

	t.Run("native", func(t *testing.T) {

		inter := parseCheckAndInterpret(t,
			`
              fun test(): [Int] {
                  return "476F20576974682074686520466C6F77".decodeHex()
              }
            `,
		)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		assert.Equal(t, expected, result)
	})

}

func TestInterpretOptionalChainingOptionalFieldRead(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
      struct Test {
          let x: Int?

          init(x: Int?) {
              self.x = x
          }
      }

      let test: Test? = Test(x: 1)
      let x = test?.x
    `)

	assert.Equal(t,
		&interpreter.SomeValue{
			Value: interpreter.NewIntValueFromInt64(1),
		},
		inter.Globals["x"].Value,
	)
}

func TestInterpretResourceOwnerFieldUse(t *testing.T) {

	t.Parallel()

	storedValues := map[string]interpreter.OptionalValue{}

	// NOTE: Getter and Setter are very naive for testing purposes and don't remove nil values
	//

	checker := func(_ *interpreter.Interpreter, _ common.Address, key string) bool {
		_, ok := storedValues[key]
		return ok
	}

	getter := func(_ *interpreter.Interpreter, _ common.Address, key string, deferred bool) interpreter.OptionalValue {
		value, ok := storedValues[key]
		if !ok {
			return interpreter.NilValue{}
		}
		return value
	}

	setter := func(_ *interpreter.Interpreter, _ common.Address, key string, value interpreter.OptionalValue) {
		storedValues[key] = value
	}

	address := common.Address{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
	}

	addressValue := interpreter.AddressValue(address)

	code := `
      pub resource R {}

      pub fun test(): [Address?] {
          let addresses: [Address?] = []

          let r <- create R()
          addresses.append(r.owner?.address)

          account.save(<-r, to: /storage/r)

          let ref = account.borrow<&R>(from: /storage/r)
          addresses.append(ref?.owner?.address)

          return addresses
      }
    `

	panicFunction := interpreter.NewHostFunctionValue(func(invocation interpreter.Invocation) trampoline.Trampoline {
		panic(errors.NewUnreachableError())
	})

	// `authAccount`

	valueDeclarations := map[string]sema.ValueDeclaration{
		"account": stdlib.StandardLibraryValue{
			Name:       "account",
			Type:       &sema.AuthAccountType{},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}

	values := map[string]interpreter.Value{
		"account": interpreter.NewAuthAccountValue(
			addressValue,
			panicFunction,
			panicFunction,
			panicFunction,
			panicFunction,
		),
	}

	inter := parseCheckAndInterpretWithOptions(t,
		code,
		ParseCheckAndInterpretOptions{
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(values),
				interpreter.WithStorageExistenceHandler(checker),
				interpreter.WithStorageReadHandler(getter),
				interpreter.WithStorageWriteHandler(setter),
				interpreter.WithStorageKeyHandler(
					func(_ *interpreter.Interpreter, _ common.Address, indexingType sema.Type) string {
						return string(indexingType.ID())
					},
				),
			},
		},
	)

	result, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NilValue{},
			interpreter.NewSomeValueOwningNonCopying(interpreter.AddressValue(address)),
		),
		result,
	)
}

func TestInterpretResourceAssignmentForceTransfer(t *testing.T) {

	t.Parallel()

	t.Run("new to nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource X {}

          fun test() {
              var x: @X? <- nil
              x <-! create X()
              destroy x
          }
        `)

		_, err := inter.Invoke("test")
		require.NoError(t, err)
	})

	t.Run("new to non-nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	     resource X {}

	     fun test() {
	         var x: @X? <- create X()
	         x <-! create X()
	         destroy x
	     }
	   `)

		_, err := inter.Invoke("test")
		require.Error(t, err)

		assert.IsType(t, &interpreter.ForceAssignmentToNonNilResourceError{}, err)
	})

	t.Run("existing to nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	     resource X {}

	     fun test() {
	         let x <- create X()
	         var x2: @X? <- nil
	         x2 <-! x
	         destroy x2
	     }
	   `)

		_, err := inter.Invoke("test")
		require.NoError(t, err)
	})

	t.Run("existing to non-nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	     resource X {}

	     fun test() {
	         let x <- create X()
	         var x2: @X? <- create X()
	         x2 <-! x
	         destroy x2
	     }
	   `)

		_, err := inter.Invoke("test")
		require.Error(t, err)

		assert.IsType(t, &interpreter.ForceAssignmentToNonNilResourceError{}, err)
	})
}

func TestInterpretForce(t *testing.T) {

	t.Parallel()

	t.Run("non-nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          let x: Int? = 1
          let y = x!
        `)

		assert.Equal(t,
			interpreter.NewSomeValueOwningNonCopying(
				interpreter.NewIntValueFromInt64(1),
			),
			inter.Globals["x"].Value,
		)

		assert.Equal(t,
			interpreter.NewIntValueFromInt64(1),
			inter.Globals["y"].Value,
		)
	})

	t.Run("nil", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          let x: Int? = nil

          fun test(): Int {
              return x!
          }
        `)

		_, err := inter.Invoke("test")
		require.Error(t, err)

		assert.IsType(t, &interpreter.ForceNilError{}, err)
	})
}

func permutations(xs []string) (res [][]string) {
	var f func([]string, int)
	f = func(a []string, k int) {
		if k == len(a) {
			res = append(res, append([]string{}, a...))
		} else {
			for i := k; i < len(xs); i++ {
				a[k], a[i] = a[i], a[k]
				f(a, k+1)
				a[k], a[i] = a[i], a[k]
			}
		}
	}

	f(xs, 0)

	return res
}

func TestInterpretCompositeValueFieldEncodingOrder(t *testing.T) {

	t.Parallel()

	fieldValues := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	initializations := make([]string, 0, len(fieldValues))

	for name, value := range fieldValues {
		initialization := fmt.Sprintf("self.%s = %d", name, value)
		initializations = append(initializations, initialization)
	}

	allInitializations := permutations(initializations)

	encodings := make([][]byte, len(allInitializations))

	for i, initialization := range allInitializations {

		inter := parseCheckAndInterpret(t,
			fmt.Sprintf(
				`
                  struct Test {
                      let a: Int
                      let b: Int
                      let c: Int

                      init() {
                          %s
                      }
                  }

                  let test = Test()
                `,
				strings.Join(initialization, "\n"),
			),
		)

		test := inter.Globals["test"].Value.(*interpreter.CompositeValue)

		test.SetOwner(&common.Address{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
		})

		encoded, _, err := interpreter.EncodeValue(test, nil, false)
		require.NoError(t, err)

		encodings[i] = encoded
	}

	expected := encodings[0]

	for _, actual := range encodings[1:] {
		require.Equal(t, expected, actual)
	}
}

func TestInterpretDictionaryValueEncodingOrder(t *testing.T) {

	t.Parallel()

	fieldValues := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	initializations := make([]string, 0, len(fieldValues))

	for name, value := range fieldValues {
		initialization := fmt.Sprintf(`xs["%s"] = %d`, name, value)
		initializations = append(initializations, initialization)
	}

	for _, initialization := range permutations(initializations) {

		inter := parseCheckAndInterpret(t,
			fmt.Sprintf(
				`
                  fun construct(): {String: Int} {
                      let xs: {String: Int} = {}
                      %s
                      return xs
                  }

                  let test = construct()
                `,
				strings.Join(initialization, "\n"),
			),
		)

		test := inter.Globals["test"].Value.(*interpreter.DictionaryValue)

		owner := &common.Address{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
		}

		test.SetOwner(owner)

		var path []string = nil
		encoded, _, err := interpreter.EncodeValue(test, path, false)
		require.NoError(t, err)

		decoder, err := interpreter.NewDecoder(bytes.NewReader(encoded), owner)
		require.NoError(t, err)

		decoded, err := decoder.Decode(path)
		require.NoError(t, err)

		test.SetModified(false)
		test.Keys.SetModified(false)
		for _, key := range test.Keys.Values {
			stringKey := key.(*interpreter.StringValue)
			stringKey.SetModified(false)
		}

		require.Equal(t, test, decoded)
	}
}

func TestInterpretEphemeralReferenceToOptional(t *testing.T) {

	t.Parallel()

	_ = parseCheckAndInterpretWithOptions(t,
		`
          contract C {

              var rs: @{Int: R}

              resource R {
                  pub let id: Int

                  init(id: Int) {
                      self.id = id
                  }
              }

              fun borrow(id: Int): &R {
                  return &C.rs[id] as &R
              }

              init() {
                  self.rs <- {}
                  self.rs[1] <-! create R(id: 1)
                  let ref = self.borrow(id: 1)
                  ref.id
              }
          }
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)
}

func TestInterpretNestedDeclarationOrder(t *testing.T) {

	t.Parallel()

	t.Run("A, B", func(t *testing.T) {
		_ = parseCheckAndInterpretWithOptions(t,
			`
              pub contract Test {

                  pub resource A {

                      pub fun b(): @B {
                          return <-create B()
                      }
                  }

                  pub resource B {}

                  init() {
                      let a <- create A()
                      let b <- a.b()
                      destroy a
                      destroy b
                  }
              }
            `,
			ParseCheckAndInterpretOptions{
				Options: []interpreter.Option{
					makeContractValueHandler(nil, nil, nil),
				},
			},
		)
	})

	t.Run("B, A", func(t *testing.T) {

		_ = parseCheckAndInterpretWithOptions(t,
			`
              pub contract Test {

                  pub resource B {}

                  pub resource A {

                      pub fun b(): @B {
                          return <-create B()
                      }
                  }

                  init() {
                      let a <- create A()
                      let b <- a.b()
                      destroy a
                      destroy b
                  }
              }
            `,
			ParseCheckAndInterpretOptions{
				Options: []interpreter.Option{
					makeContractValueHandler(nil, nil, nil),
				},
			},
		)
	})
}

func TestInterpretCountDigits256(t *testing.T) {

	t.Parallel()

	type test struct {
		Type    sema.Type
		Literal string
		Count   int
	}

	for _, test := range []test{
		{
			&sema.Int256Type{},
			"676983016644359394637212096269997871684197836659065544033845082275068334",
			72,
		},
		{
			&sema.UInt256Type{},
			"676983016644359394637212096269997871684197836659065544033845082275068334",
			72,
		},
		{
			&sema.Int128Type{},
			"676983016644359394637212096269997871",
			36,
		},
		{
			&sema.UInt128Type{},
			"676983016644359394637212096269997871",
			36,
		},
	} {

		t.Run(test.Type.String(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      fun countDigits(_ x: %[2]s): UInt8 {
                          var count: UInt8 = UInt8(0)
                          var input = x
                          while input != %[2]s(0) {
                              count = count + UInt8(1)
                              input = input / %[2]s(10)
                          }
                          return count
                      }

                      let number: %[2]s = %[1]s
                      let result1 = countDigits(%[1]s)
                      let result2 = countDigits(%[2]s(%[1]s))
                      let result3 = countDigits(number)
                    `,
					test.Literal,
					test.Type,
				),
			)

			bigInt, ok := new(big.Int).SetString(test.Literal, 10)
			require.True(t, ok)

			assert.Equal(t,
				bigInt,
				inter.Globals["number"].Value.(interpreter.BigNumberValue).ToBigInt(),
			)

			expected := interpreter.UInt8Value(test.Count)

			for i := 1; i <= 3; i++ {
				variableName := fmt.Sprintf("result%d", i)
				assert.Equal(t,
					expected,
					inter.Globals[variableName].Value,
				)
			}
		})
	}
}

func TestInterpretFailableCastingCompositeTypeConfusion(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          contract A {
              struct S {}
          }

          contract B {
              struct S {}
          }

          let s = A.S() as? B.S
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				makeContractValueHandler(nil, nil, nil),
			},
		},
	)

	assert.Equal(t,
		interpreter.NilValue{},
		inter.Globals["s"].Value,
	)
}

func TestInterpretNestedDestroy(t *testing.T) {

	var logs []string

	logFunction := stdlib.NewStandardLibraryFunction(
		"log",
		&sema.FunctionType{
			Parameters: []*sema.Parameter{
				{
					Label:          sema.ArgumentLabelNotRequired,
					Identifier:     "value",
					TypeAnnotation: sema.NewTypeAnnotation(&sema.AnyStructType{}),
				},
			},
			ReturnTypeAnnotation: sema.NewTypeAnnotation(
				&sema.VoidType{},
			),
		},
		func(invocation interpreter.Invocation) trampoline.Trampoline {
			message := fmt.Sprintf("%v", invocation.Arguments[0])
			logs = append(logs, message)
			result := interpreter.VoidValue{}
			return trampoline.Done{Result: result}
		},
		nil,
	)

	valueDeclarations :=
		stdlib.StandardLibraryFunctions{
			logFunction,
		}.ToValueDeclarations()

	values := stdlib.StandardLibraryFunctions{
		logFunction,
	}.ToValues()

	inter := parseCheckAndInterpretWithOptions(t,
		`
          resource B {
              let id: Int

              init(_ id: Int){
                  self.id = id
              }

              destroy(){
                  log("destroying B with id:")
                  log(self.id)
              }
          }

          resource A {
              let id: Int
              let bs: @[B]

              init(_ id: Int){
                  self.id = id
                  self.bs <- []
              }

              fun add(_ b: @B){
                  self.bs.append(<-b)
              }

              destroy() {
                  log("destroying A with id:")
                  log(self.id)
                  destroy self.bs
              }
          }

          fun test() {
              let a <- create A(1)
              a.add(<- create B(2))
              a.add(<- create B(3))
              a.add(<- create B(4))

              destroy a
          }
        `,
		ParseCheckAndInterpretOptions{
			Options: []interpreter.Option{
				interpreter.WithPredefinedValues(values),
			},
			CheckerOptions: []sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
			},
			HandleCheckerError: nil,
		},
	)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.VoidValue{},
		value,
	)

	assert.Equal(t,
		[]string{
			`"destroying A with id:"`,
			"1",
			`"destroying B with id:"`,
			"2",
			`"destroying B with id:"`,
			"3",
			`"destroying B with id:"`,
			"4",
		},
		logs,
	)
}

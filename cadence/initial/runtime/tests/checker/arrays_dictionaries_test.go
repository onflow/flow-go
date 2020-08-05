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
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/cmd"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

func TestCheckDictionary(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, "b": 2}
	`)

	assert.NoError(t, err)
}

func TestCheckDictionaryType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z: {String: Int} = {"a": 1, "b": 2}
	`)

	assert.NoError(t, err)
}

func TestCheckInvalidDictionaryTypeKey(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z: {Int: Int} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryTypeValue(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z: {String: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryTypeSwapped(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z: {Int: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryKeys(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, true: 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryValues(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, "b": true}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryIndexingString(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      let x = {"abc": 1, "def": 2}
      let y = x["abc"]
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["y"].Type,
	)
}

func TestCheckDictionaryIndexingBool(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x = {true: 1, false: 2}
      let y = x[true]
	`)

	assert.NoError(t, err)
}

func TestCheckInvalidDictionaryIndexing(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x = {"abc": 1, "def": 2}
      let y = x[true]
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexingTypeError{}, errs[0])
}

func TestCheckDictionaryIndexingAssignment(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = 3
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidDictionaryIndexingAssignment(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = true
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryRemove(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.remove(key: "abc")
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidDictionaryRemove(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.remove(key: true)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryInsert(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.insert(key: "abc", 3)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidDictionaryInsert(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.insert(key: true, 3)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryKeys(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let keys = {"abc": 1, "def": 2}.keys
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.VariableSizedType{Type: &sema.StringType{}},
		checker.GlobalValues["keys"].Type,
	)
}

func TestCheckDictionaryValues(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let values = {"abc": 1, "def": 2}.values
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.VariableSizedType{Type: &sema.IntType{}},
		checker.GlobalValues["values"].Type,
	)
}

func TestCheckLength(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	require.NoError(t, err)
}

func TestCheckArrayAppend(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayAppend(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append("4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckArrayAppendBound(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayAppendToConstantSize(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int; 3] {
          let x: [Int; 3] = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayConcat(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
	  fun test(): [Int] {
	 	  let a = [1, 2]
		  let b = [3, 4]
          let c = a.concat(b)
          return c
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayConcat(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = ["a", "b"]
          let c = a.concat(b)
          return c
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayConcatOfConstantSized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
	  fun test(): [Int] {
	 	  let a: [Int; 2] = [1, 2]
		  let b: [Int; 2] = [3, 4]
          let c = a.concat(b)
          return c
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayConcatBound(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = [3, 4]
		  let c = a.concat
		  return c(b)
      }
    `)

	require.NoError(t, err)
}

func TestCheckArrayInsert(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayInsert(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, "4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayInsertIntoConstantSized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int; 3] {
          let x: [Int; 3] = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayRemove(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.remove(at: 1)
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayRemove(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.remove(at: "1")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayRemoveFromConstantSized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int; 3] {
          let x: [Int; 3] = [1, 2, 3]
          let old: Int? = x.remove(at: 1)
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayRemoveFirst(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeFirst()
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayRemoveFirst(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeFirst(1)
          return x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])
}

func TestCheckInvalidArrayRemoveFirstFromConstantSized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int; 3] {
          let x: [Int; 3] = [1, 2, 3]
          let old: Int? = x.removeFirst()
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayRemoveLast(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeLast()
          return x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayRemoveLastFromConstantSized(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): [Int; 3] {
          let x: [Int; 3] = [1, 2, 3]
          let old: Int? = x.removeLast()
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckArrayContains(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains(2)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidArrayContains(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains("abc")
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayContainsNotEquatable(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let z = [[1], [2], [3]]
          return z.contains([1, 2])
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotEquatableTypeError{}, errs[0])
}

func TestCheckEmptyArray(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let xs: [Int] = []
	`)

	require.NoError(t, err)
}

func TestCheckEmptyArrayCall(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun foo(xs: [Int]) {
          foo(xs: [])
      }
	`)

	require.NoError(t, err)
}

func TestCheckEmptyDictionary(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let xs: {String: Int} = {}
	`)

	require.NoError(t, err)
}

func TestCheckEmptyDictionaryCall(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun foo(xs: {String: Int}) {
          foo(xs: {})
      }
	`)

	require.NoError(t, err)
}

func TestCheckArraySubtyping(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			body := "{}"
			if kind == common.CompositeKindEvent {
				body = "()"
			}

			interfaceType := AsInterfaceType("I", kind)

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface I %[2]s
                      %[1]s S: I %[2]s

                      let xs: %[3]s[S] %[4]s []
                      let ys: %[3]s[%[5]s] %[4]s xs
	                `,
					kind.Keyword(),
					body,
					kind.Annotation(),
					kind.TransferOperator(),
					interfaceType,
				),
			)

			if !assert.NoError(t, err) {
				cmd.PrettyPrintError(err, "", map[string]string{"": ""})
			}
		})
	}
}

func TestCheckInvalidArraySubtyping(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let xs: [Bool] = []
      let ys: [Int] = xs
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionarySubtyping(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			body := "{}"
			if kind == common.CompositeKindEvent {
				body = "()"
			}

			interfaceType := AsInterfaceType("I", kind)

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface I %[2]s
                      %[1]s S: I %[2]s

                      let xs: %[3]s{String: S} %[4]s {}
                      let ys: %[3]s{String: %[5]s} %[4]s xs
	                `,
					kind.Keyword(),
					body,
					kind.Annotation(),
					kind.TransferOperator(),
					interfaceType,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidDictionarySubtyping(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let xs: {String: Bool} = {}
      let ys: {String: Int} = xs
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayElements(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let z = [0, true]
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckConstantSizedArrayDeclaration(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: [Int; 3] = [1, 2, 3]
    `)

	require.NoError(t, err)
}

func TestCheckInvalidConstantSizedArrayDeclarationCountMismatchTooMany(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: [Int; 2] = [1, 2, 3]
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ConstantSizedArrayLiteralSizeError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidConstantSizedArrayDeclarationOutOfRangeSize(t *testing.T) {

	t.Parallel()

	t.Run("negative", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          let x: [Int; -1] = []
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidConstantSizedTypeSizeError{}, errs[0])
	})

	t.Run("too large", func(t *testing.T) {

		tooLarge := new(big.Int).SetUint64(math.MaxUint64)
		tooLarge.Add(tooLarge, big.NewInt(1))

		_, err := ParseAndCheck(t,
			fmt.Sprintf(
				`
                  let x: [Int; %s] = []
			    `,
				tooLarge,
			),
		)

		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.InvalidConstantSizedTypeSizeError{}, errs[0])
		assert.IsType(t, &sema.ConstantSizedArrayLiteralSizeError{}, errs[1])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
	})
}

func TestCheckInvalidConstantSizedArrayDeclarationBase(t *testing.T) {

	t.Parallel()

	for _, size := range []string{"0x42", "0b1010", "0o10"} {

		t.Run(size, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let x: [Int; %s] = []
                    `,
					size,
				),
			)

			errs := ExpectCheckerErrors(t, err, 3)

			assert.IsType(t, &sema.InvalidConstantSizedTypeBaseError{}, errs[0])
			assert.IsType(t, &sema.ConstantSizedArrayLiteralSizeError{}, errs[1])
			assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
		})
	}
}

func TestCheckDictionaryKeyTypesExpressions(t *testing.T) {

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
		t.Run(fmt.Sprintf("valid: %s", ty), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let k: %s = %s
                      let xs = {k: "x"}
                    `,
					ty,
					code,
				),
			)

			require.NoError(t, err)
		})
	}

	for name, code := range map[string]string{
		"struct": `
           struct X {}
           let k = X()
        `,
		"array":      `let k = [1]`,
		"dictionary": `let k = {"a": 1}`,
	} {
		t.Run(fmt.Sprintf("invalid: %s", name), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s
                      let xs = {k: "x"}
                    `,
					code,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDictionaryKeyTypeError{}, errs[0])
		})
	}
}

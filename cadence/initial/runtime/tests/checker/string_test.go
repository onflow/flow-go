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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckCharacter(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let x: Character = "x"
	`)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.CharacterType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckCharacterUnicodeScalar(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let x: Character = "\u{1F1FA}\u{1F1F8}"
	`)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.CharacterType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckString(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let x = "x"
	`)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.StringType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckStringConcat(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
	  fun test(): String {
	 	  let a = "abc"
		  let b = "def"
		  let c = a.concat(b)
		  return c
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidStringConcat(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abc"
		  let b = [1, 2]
		  let c = a.concat(b)
		  return c
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckStringConcatBound(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abc"
		  let b = "def"
		  let c = a.concat
		  return c(b)
      }
    `)

	require.NoError(t, err)
}

func TestCheckStringSlice(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
	  fun test(): String {
	 	  let a = "abcdef"
		  return a.slice(from: 0, upTo: 1)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidStringSlice(t *testing.T) {

	t.Parallel()

	t.Run("MissingBothArgumentLabels", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(0, 1)
		`)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
	})

	t.Run("MissingOneArgumentLabel", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(from: 0, 1)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
	})

	t.Run("InvalidArgumentType", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(from: "a", upTo: "b")
		`)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})
}

func TestCheckStringSliceBound(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abcdef"
		  let c = a.slice
		  return c(from: 0, upTo: 1)
      }
    `)

	require.NoError(t, err)
}

// TODO: prevent invalid character literals
// func TestCheckInvalidCharacterLiteral(t *testing.T) {
// 	//
// 	_, err := ParseAndCheck(t, `
//         let x: Character = "abc"
// 	`)
//
// 	errs := ExpectCheckerErrors(t, err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

// TODO: prevent assignment with invalid character literal
// func TestCheckStringIndexingAssignmentWithInvalidCharacterLiteral(t *testing.T) {
// 	//
// 	_, err := ParseAndCheck(t, `
//       fun test() {
//           let z = "abc"
//           z[0] = "def"
//       }
// 	`)
//
// 	errs := ExpectCheckerErrors(t, err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

func TestCheckStringIndexing(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = "abc"
          let y: Character = z[0]
      }
	`)

	require.NoError(t, err)
}

func TestCheckStringIndexingAssignment(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
		  let z = "abc"
		  let y: Character = "d"
          z[0] = y
      }
	`)

	require.NoError(t, err)
}

func TestCheckStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = "abc"
          z[0] = "d"
      }
	`)

	require.NoError(t, err)
}

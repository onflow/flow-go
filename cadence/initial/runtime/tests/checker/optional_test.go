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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckOptional(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int? = 1
    `)

	require.NoError(t, err)
}

func TestCheckInvalidOptional(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int? = false
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckOptionalNesting(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int?? = 1
    `)

	require.NoError(t, err)
}

func TestCheckNil(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
   `)

	require.NoError(t, err)
}

func TestCheckOptionalNestingNil(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int?? = nil
   `)

	require.NoError(t, err)
}

func TestCheckNilReturnValue(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     fun test(): Int?? {
         return nil
     }
   `)

	require.NoError(t, err)
}

func TestCheckInvalidNonOptionalNil(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int = nil
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckNilsComparison(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x = nil == nil
   `)

	require.NoError(t, err)
}

func TestCheckOptionalNilComparison(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckNonOptionalNilComparison(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckNonOptionalNilComparisonSwapped(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = nil == x
     let z = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckOptionalNilComparisonSwapped(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = nil == x
   `)

	require.NoError(t, err)
}

func TestCheckNestedOptionalNilComparisonSwapped(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int?? = 1
     let y = nil == x
   `)

	require.NoError(t, err)
}

func TestCheckNestedOptionalComparison(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
     let y: Int?? = nil
     let z = x == y
   `)

	require.NoError(t, err)
}

func TestCheckInvalidNestedOptionalComparison(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
     let y: Bool?? = nil
     let z = x == y
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
}

func TestCheckCompositeNilEquality(t *testing.T) {

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
				`let x: %[1]sX? %[2]s %[3]s X%[4]s`,
				compositeKind.Annotation(),
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
			identifier = "x"
		}

		t.Run(compositeKind.Name(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s X {}

                      %[2]s

                      let a = %[3]s == nil
                      let b = nil == %[3]s
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeNilEquality(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindEvent {
			continue
		}

		var setupCode, firstIdentifier, secondIdentifier string
		if compositeKind == common.CompositeKindContract {
			firstIdentifier = "X"
			secondIdentifier = "X"
		} else {
			setupCode = fmt.Sprintf(`
                  let x: %[1]sX? %[2]s %[3]s X%[4]s
                  let y: %[1]sX? %[2]s nil
                `,
				compositeKind.Annotation(),
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
			firstIdentifier = "x"
			secondIdentifier = "y"
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Name(), func(t *testing.T) {
			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s X %[2]s

                      %[3]s

                      let a = %[4]s == %[5]s
                    `,
					compositeKind.Keyword(),
					body,
					setupCode,
					firstIdentifier,
					secondIdentifier,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
		})
	}
}

func TestCheckInvalidNonOptionalReturn(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(x: Int?): Int {
          return x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidOptionalIntegerConversion(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int8? = 1
      let y: Int16? = x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

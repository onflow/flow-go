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

func TestCheckOptionalChainingNonOptionalFieldRead(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      struct Test {
          let x: Int

          init(x: Int) {
              self.x = x
          }
      }

      let test: Test? = Test(x: 1)
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckOptionalChainingOptionalFieldRead(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      struct Test {
          let x: Int?

          init(x: Int?) {
              self.x = x
          }
      }

      let test: Test? = Test(x: 1)
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckOptionalChainingFunctionRead(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      struct Test {
          fun x(): Int {
              return 42
          }
      }

      let test: Test? = Test()
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.True(t,
		checker.GlobalValues["x"].Type.Equal(
			&sema.OptionalType{
				Type: &sema.FunctionType{
					ReturnTypeAnnotation: &sema.TypeAnnotation{
						Type: &sema.IntType{},
					},
				},
			},
		),
	)
}

func TestCheckOptionalChainingFunctionCall(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      struct Test {
          fun x(): Int {
              return 42
          }
      }

      let test: Test? = Test()
      let x = test?.x()
    `)

	require.NoError(t, err)

	assert.True(t,
		checker.GlobalValues["x"].Type.Equal(
			&sema.OptionalType{Type: &sema.IntType{}},
		),
	)
}

func TestCheckInvalidOptionalChainingNonOptional(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          let x: Int

          init(x: Int) {
              self.x = x
          }
      }

      let test = Test(x: 1)
      let x = test?.x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidOptionalChainingError{}, errs[0])
}

func TestCheckInvalidOptionalChainingFieldAssignment(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Test {
          var x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          let test: Test? = Test(x: 1)
          test?.x = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedOptionalChainingAssignmentError{}, errs[0])
}

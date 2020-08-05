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
	"github.com/onflow/cadence/runtime/stdlib"
)

func TestCheckInvalidFunctionCallWithTooFewArguments(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])
}

func TestCheckFunctionCallWithArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
    `)

	require.NoError(t, err)
}

func TestCheckFunctionCallWithoutArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidFunctionCallWithNotRequiredArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(_ x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: 1)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[0])
}

func TestCheckIndirectFunctionCallWithoutArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          let g = f
          return g(1)
      }
    `)

	require.NoError(t, err)
}

func TestCheckFunctionCallMissingArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(1)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
}

func TestCheckFunctionCallIncorrectArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(y: 1)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithTooManyArguments(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(2, 3)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
}

func TestCheckInvalidFunctionCallOfBool(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return true()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotCallableError{}, errs[0])
}

func TestCheckInvalidFunctionCallOfInteger(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          return 2()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotCallableError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithWrongType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(x: true)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidFunctionCallWithWrongTypeAndMissingArgumentLabel(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int): Int {
          return x
      }

      fun test(): Int {
          return f(true)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

	assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
}

func TestCheckInvocationOfFunctionFromStructFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun f(x: Int) {}

      struct Y {
        fun x() {
          f(x: 1)
        }
      }
    `)
	require.NoError(t, err)
}

func TestCheckInvalidStructFunctionInvocation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `

      struct Y {
        fun x() {
          x()
        }
      }
    `)
	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvocationOfFunctionFromStructFunctionWithSameName(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun x(y: Int) {}

      struct Y {
        // struct function and global function have same name
        fun x() {
          x(y: 1)
        }
      }
    `)
	require.NoError(t, err)
}

func TestCheckIntricateIntegerBinaryExpression(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int8 = 100
      let y = (Int8(90) + Int8(10)) == x
    `)
	require.NoError(t, err)
}

func TestCheckInvocationWithOnlyVarargs(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithOptions(t,
		`
            pub fun test() {
                foo(1)
            }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(
					stdlib.StandardLibraryFunctions{
						stdlib.StandardLibraryFunction{
							Name: "foo",
							Type: &sema.FunctionType{
								ReturnTypeAnnotation: &sema.TypeAnnotation{
									Type: &sema.VoidType{},
								},
								RequiredArgumentCount: func() *int {
									// NOTE: important to check *all* arguments are optional
									var count = 0
									return &count
								}(),
							},
						},
					}.ToValueDeclarations(),
				),
			},
		},
	)

	require.NoError(t, err)
}

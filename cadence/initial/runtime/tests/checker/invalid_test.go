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

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckSpuriousIdentifierAssignmentInvalidValueTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test() {
              var x = 1
              x = y
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIdentifierAssignmentInvalidTargetTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test() {
              var x: X = 1
              x = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIndexAssignmentInvalidValueTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test() {
              let values: {String: Int} = {}
              values["x"] = x
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIndexAssignmentInvalidElementTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test() {
              let values: {String: X} = {}
              values["x"] = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousMemberAssignmentInvalidValueTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          struct X {
              var x: Int
              init() {
                  self.x = y
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousMemberAssignmentInvalidMemberTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
         struct X {
              var y: Y
              init() {
                  self.y = 0
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousReturnWithInvalidValueTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test(): Int {
              return x
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousReturnWithInvalidReturnTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          fun test(): X {
              return 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousCastWithInvalidTargetTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          let y = 1 as X
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousCastWithInvalidValueTypeMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          let y = x as Int
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

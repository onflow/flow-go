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

func TestCheckForce(t *testing.T) {

	t.Parallel()

	t.Run("valid", func(t *testing.T) {

		checker, err := ParseAndCheck(t, `
          let x: Int? = 1
          let y = x!
        `)

		require.NoError(t, err)

		assert.Equal(t,
			&sema.OptionalType{Type: &sema.IntType{}},
			checker.GlobalValues["x"].Type,
		)

		assert.Equal(t,
			&sema.IntType{},
			checker.GlobalValues["y"].Type,
		)

	})

	t.Run("invalid: non-optional", func(t *testing.T) {

		checker, err := ParseAndCheck(t, `
          let x: Int = 1
          let y = x!
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NonOptionalForceError{}, errs[0])

		assert.Equal(t,
			&sema.IntType{},
			checker.GlobalValues["x"].Type,
		)

		assert.Equal(t,
			&sema.IntType{},
			checker.GlobalValues["y"].Type,
		)
	})

	t.Run("invalid: force resource multiple times", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          let x: @R? <- create R()
          let x2 <- x!
          let x3 <- x!
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	})
}

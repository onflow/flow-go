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

func TestCheckMetaType(t *testing.T) {

	t.Parallel()

	t.Run("constructor", func(t *testing.T) {
		t.Parallel()

		checker, err := ParseAndCheck(t, `
          let type: Type = Type<[Int]>()
        `)

		require.NoError(t, err)

		assert.Equal(t,
			&sema.MetaType{},
			checker.GlobalValues["type"].Type,
		)
	})

	t.Run("identifier", func(t *testing.T) {
		t.Parallel()

		checker, err := ParseAndCheck(t, `
          let type = Type<[Int]>()
          let identifier = type.identifier
        `)

		require.NoError(t, err)

		assert.Equal(t,
			&sema.MetaType{},
			checker.GlobalValues["type"].Type,
		)
	})
}

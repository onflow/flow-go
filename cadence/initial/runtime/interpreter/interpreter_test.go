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

package interpreter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-initial/runtime/sema"
	"github.com/onflow/cadence-initial/runtime/tests/utils"
)

func TestInterpreterOptionalBoxing(t *testing.T) {

	t.Parallel()

	checker, err := sema.NewChecker(nil, utils.TestLocation)
	require.NoError(t, err)

	inter, err := NewInterpreter(checker)
	require.NoError(t, err)

	t.Run("Bool to Bool?", func(t *testing.T) {
		value := inter.boxOptional(
			BoolValue(true),
			&sema.BoolType{},
			&sema.OptionalType{Type: &sema.BoolType{}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(BoolValue(true)),
			value,
		)
	})

	t.Run("Bool? to Bool?", func(t *testing.T) {
		value := inter.boxOptional(
			NewSomeValueOwningNonCopying(BoolValue(true)),
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.BoolType{}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(BoolValue(true)),
			value,
		)
	})

	t.Run("Bool? to Bool??", func(t *testing.T) {
		value := inter.boxOptional(
			NewSomeValueOwningNonCopying(BoolValue(true)),
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(
				NewSomeValueOwningNonCopying(BoolValue(true)),
			),
			value,
		)
	})

	t.Run("nil (Never?) to Bool??", func(t *testing.T) {
		// NOTE:
		value := inter.boxOptional(
			NilValue{},
			&sema.OptionalType{Type: &sema.NeverType{}},
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
		)
		assert.Equal(t,
			NilValue{},
			value,
		)
	})

	t.Run("nil (Some(nil): Never??) to Bool??", func(t *testing.T) {
		// NOTE:
		value := inter.boxOptional(
			NewSomeValueOwningNonCopying(NilValue{}),
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.NeverType{}}},
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
		)
		assert.Equal(t,
			NilValue{},
			value,
		)
	})
}

func TestInterpreterBoxing(t *testing.T) {

	t.Parallel()

	checker, err := sema.NewChecker(nil, utils.TestLocation)
	require.NoError(t, err)

	inter, err := NewInterpreter(checker)
	require.NoError(t, err)

	for _, anyType := range []sema.Type{
		&sema.AnyStructType{},
		&sema.AnyResourceType{},
	} {

		t.Run(anyType.String(), func(t *testing.T) {

			t.Run(fmt.Sprintf("Bool to %s?", anyType), func(t *testing.T) {

				assert.Equal(t,
					NewSomeValueOwningNonCopying(
						BoolValue(true),
					),
					inter.convertAndBox(
						BoolValue(true),
						&sema.BoolType{},
						&sema.OptionalType{Type: anyType},
					),
				)

			})

			t.Run(fmt.Sprintf("Bool? to %s?", anyType), func(t *testing.T) {

				assert.Equal(t,
					NewSomeValueOwningNonCopying(
						BoolValue(true),
					),
					inter.convertAndBox(
						NewSomeValueOwningNonCopying(BoolValue(true)),
						&sema.OptionalType{Type: &sema.BoolType{}},
						&sema.OptionalType{Type: anyType},
					),
				)
			})
		})
	}
}

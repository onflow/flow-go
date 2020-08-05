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
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence-initial/runtime/sema"
)

func TestNegate(t *testing.T) {

	t.Parallel()

	t.Run("Int8", func(t *testing.T) {
		assert.Panics(t, func() {
			Int8Value(math.MinInt8).Negate()
		})
	})

	t.Run("Int16", func(t *testing.T) {
		assert.Panics(t, func() {
			Int16Value(math.MinInt16).Negate()
		})
	})

	t.Run("Int32", func(t *testing.T) {
		assert.Panics(t, func() {
			Int32Value(math.MinInt32).Negate()
		})
	})

	t.Run("Int64", func(t *testing.T) {
		assert.Panics(t, func() {
			Int64Value(math.MinInt64).Negate()
		})
	})

	t.Run("Int128", func(t *testing.T) {
		assert.Panics(t, func() {
			Int128Value{new(big.Int).Set(sema.Int128TypeMinIntBig)}.Negate()
		})
	})

	t.Run("Int256", func(t *testing.T) {
		assert.Panics(t, func() {
			Int256Value{new(big.Int).Set(sema.Int256TypeMinIntBig)}.Negate()
		})
	})
}

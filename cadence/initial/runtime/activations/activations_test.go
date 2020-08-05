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

package activations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActivations(t *testing.T) {

	t.Parallel()

	activations := &Activations{}

	activations.Set("a", 1)

	assert.Equal(t, activations.Find("a"), 1)
	assert.Nil(t, activations.Find("b"))

	activations.PushCurrent()

	activations.Set("a", 2)
	activations.Set("b", 3)

	assert.Equal(t, activations.Find("a"), 2)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Nil(t, activations.Find("c"))

	activations.PushCurrent()

	activations.Set("a", 5)
	activations.Set("c", 4)

	assert.Equal(t, activations.Find("a"), 5)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Equal(t, activations.Find("c"), 4)

	activations.Pop()

	assert.Equal(t, activations.Find("a"), 2)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Nil(t, activations.Find("c"))

	activations.Pop()

	assert.Equal(t, activations.Find("a"), 1)
	assert.Nil(t, activations.Find("b"))
	assert.Nil(t, activations.Find("c"))

	activations.Pop()

	assert.Nil(t, activations.Find("a"))
	assert.Nil(t, activations.Find("b"))
	assert.Nil(t, activations.Find("c"))
}

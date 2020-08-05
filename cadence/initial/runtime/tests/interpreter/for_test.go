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

package interpreter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/interpreter"
)

func TestInterpretForStatement(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var sum = 0
           for y in [1, 2, 3, 4] {
               sum = sum + y
           }
           return sum
       }

    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(10),
		value,
	)
}

func TestInterpretForStatementWithReturn(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           for x in [1, 2, 3, 4, 5] {
               if x > 3 {
                   return x
               }
           }
           return -1
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(4),
		value,
	)
}

func TestInterpretForStatementWithContinue(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): [Int] {
           var xs: [Int] = []
           for x in [1, 2, 3, 4, 5] {
               if x <= 3 {
                   continue
               }
               xs.append(x)
           }
           return xs
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewArrayValueUnownedNonCopying(
			interpreter.NewIntValueFromInt64(4),
			interpreter.NewIntValueFromInt64(5),
		),
		value,
	)
}

func TestInterpretForStatementWithBreak(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var y = 0
           for x in [1, 2, 3, 4] {
               y = x
               if x > 3 {
                   break
               }
           }
           return y
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(4),
		value,
	)
}

func TestInterpretForStatementEmpty(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Bool {
           var x = false
           for y in [] {
               x = true
           }
           return x
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.BoolValue(false),
		value,
	)
}

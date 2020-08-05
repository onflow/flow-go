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

func TestInterpretWhileStatement(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 5 {
               x = x + 2
           }
           return x
       }

    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(6),
		value,
	)
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 2
               if x > 5 {
                   return x
               }
           }
           return x
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(6),
		value,
	)
}

func TestInterpretWhileStatementWithContinue(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var i = 0
           var x = 0
           while i < 10 {
               i = i + 1
               if i < 5 {
                   continue
               }
               x = x + 1
           }
           return x
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(6),
		value,
	)
}

func TestInterpretWhileStatementWithBreak(t *testing.T) {

	t.Parallel()

	inter := parseCheckAndInterpret(t, `
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 1
               if x == 5 {
                   break
               }
           }
           return x
       }
    `)

	value, err := inter.Invoke("test")
	require.NoError(t, err)

	assert.Equal(t,
		interpreter.NewIntValueFromInt64(5),
		value,
	)
}

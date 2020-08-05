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

package stdlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func TestAssert(t *testing.T) {

	t.Parallel()

	program := &ast.Program{}

	checker, err := sema.NewChecker(
		program,
		utils.TestLocation,
		sema.WithPredeclaredValues(BuiltinFunctions.ToValueDeclarations()),
	)
	require.Nil(t, err)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(BuiltinFunctions.ToValues()),
	)
	require.Nil(t, err)

	_, err = inter.Invoke(
		"assert",
		interpreter.BoolValue(false),
		interpreter.NewStringValue("oops"),
	)
	assert.Equal(t,
		AssertionError{
			Message:       "oops",
			LocationRange: interpreter.LocationRange{},
		},
		err,
	)

	_, err = inter.Invoke("assert", interpreter.BoolValue(false))
	assert.Equal(t,
		AssertionError{
			Message:       "",
			LocationRange: interpreter.LocationRange{},
		},
		err)

	_, err = inter.Invoke(
		"assert",
		interpreter.BoolValue(true),
		interpreter.NewStringValue("oops"),
	)
	assert.NoError(t, err)

	_, err = inter.Invoke("assert", interpreter.BoolValue(true))
	assert.NoError(t, err)
}

func TestPanic(t *testing.T) {

	t.Parallel()

	checker, err := sema.NewChecker(
		&ast.Program{},
		utils.TestLocation,
		sema.WithPredeclaredValues(BuiltinFunctions.ToValueDeclarations()),
	)
	require.Nil(t, err)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(BuiltinFunctions.ToValues()),
	)
	require.Nil(t, err)

	_, err = inter.Invoke("panic", interpreter.NewStringValue("oops"))
	assert.Equal(t,
		PanicError{
			Message:       "oops",
			LocationRange: interpreter.LocationRange{},
		},
		err)
}

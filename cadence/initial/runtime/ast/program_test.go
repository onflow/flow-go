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

package ast

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgram_ResolveImports(t *testing.T) {

	t.Parallel()

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := &Program{}

	err := a.ResolveImports(func(location Location) (*Program, error) {
		switch location {
		case StringLocation("b"):
			return b, nil
		case StringLocation("c"):
			return c, nil
		default:
			return nil, fmt.Errorf("tried to resolve unknown import location: %s", location)
		}
	})

	require.NoError(t, err)

	importsA := a.ImportedPrograms()

	actual := importsA[StringLocation("b").ID()]
	if actual != b {
		assert.Fail(t, "not b", actual)
	}

	importsB := b.ImportedPrograms()

	actual = importsB[StringLocation("c").ID()]
	if actual != c {
		assert.Fail(t, "not c", actual)
	}
}

func TestProgram_ResolveImportsCycle(t *testing.T) {

	t.Parallel()

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := makeImportingProgram("a")

	err := a.ResolveImports(func(location Location) (*Program, error) {
		switch location {
		case StringLocation("a"):
			return a, nil
		case StringLocation("b"):
			return b, nil
		case StringLocation("c"):
			return c, nil
		default:
			return nil, fmt.Errorf("tried to resolve unknown import location: %s", location)
		}
	})

	assert.Equal(t,
		err,
		CyclicImportsError{
			Location: StringLocation("b"),
		},
	)
}

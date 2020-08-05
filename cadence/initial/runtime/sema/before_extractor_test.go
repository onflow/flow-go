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

package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/parser"
)

func TestBeforeExtractor(t *testing.T) {

	t.Parallel()

	expression, inputIsComplete, err := parser.ParseExpression(`
        before(x + before(y)) + z
    `)

	require.True(t, inputIsComplete)
	require.NoError(t, err)

	extractor := NewBeforeExtractor(nil)

	identifier1 := ast.Identifier{
		Identifier: extractor.ExpressionExtractor.FormatIdentifier(0),
	}
	identifier2 := ast.Identifier{
		Identifier: extractor.ExpressionExtractor.FormatIdentifier(1),
	}

	result := extractor.ExtractBefore(expression)

	assert.Equal(t,
		result,
		ast.ExpressionExtraction{
			RewrittenExpression: &ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IdentifierExpression{
					Identifier: identifier2,
				},
				Right: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "z",
						Pos:        ast.Position{Offset: 33, Line: 2, Column: 32},
					},
				},
			},
			ExtractedExpressions: []ast.ExtractedExpression{
				{
					Identifier: identifier1,
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "y",
							Pos:        ast.Position{Offset: 27, Line: 2, Column: 26},
						},
					},
				},
				{
					Identifier: identifier2,
					Expression: &ast.BinaryExpression{
						Operation: ast.OperationPlus,
						Left: &ast.IdentifierExpression{
							Identifier: ast.Identifier{
								Identifier: "x",
								Pos:        ast.Position{Offset: 16, Line: 2, Column: 15},
							},
						},
						Right: &ast.IdentifierExpression{
							Identifier: identifier1,
						},
					},
				},
			},
		},
	)
}

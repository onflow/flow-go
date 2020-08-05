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

package parser2

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	oldParser "github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/parser2/lexer"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func TestParseSimpleInfixExpression(t *testing.T) {

	t.Parallel()

	t.Run("no spaces", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1+2*3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Right: &ast.BinaryExpression{
					Operation: ast.OperationMul,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
							EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(3),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
							EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("with spaces", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("  1   +   2  *   3 ")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				Right: &ast.BinaryExpression{
					Operation: ast.OperationMul,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
							EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(3),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 17, Offset: 17},
							EndPos:   ast.Position{Line: 1, Column: 17, Offset: 17},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("repeated infix, same operator, left associative", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 + 2 + 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.BinaryExpression{
					Operation: ast.OperationPlus,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
							EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
							EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
						},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
			result,
		)
	})

	t.Run("repeated infix, same operator, right associative", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 ?? 2 ?? 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationNilCoalesce,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Right: &ast.BinaryExpression{
					Operation: ast.OperationNilCoalesce,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
							EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(3),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
							EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
						},
					},
				},
			},
			result,
		)
	})
}

func TestParseAdvancedExpression(t *testing.T) {

	t.Parallel()

	t.Run("mixed infix and prefix", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 +- 2 -- 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationMinus,
				Left: &ast.BinaryExpression{
					Operation: ast.OperationPlus,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
							EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(-2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
						},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(-3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
					},
				},
			},
			result,
		)
	})

	t.Run("nested expression", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("(1 + 2) * 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationMul,
				Left: &ast.BinaryExpression{
					Operation: ast.OperationPlus,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
							EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
							EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
						},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
						EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
					},
				},
			},
			result,
		)
	})

	t.Run("less and greater", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 < 2 > 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationGreater,
				Left: &ast.BinaryExpression{
					Operation: ast.OperationLess,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
							EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
							EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
						},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
			result,
		)
	})

	t.Run("conditional", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("a ? b : c ? d : e")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ConditionalExpression{
				Test: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Then: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "b",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				Else: &ast.ConditionalExpression{
					Test: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "c",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					Then: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "d",
							Pos:        ast.Position{Line: 1, Column: 12, Offset: 12},
						},
					},
					Else: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "e",
							Pos:        ast.Position{Line: 1, Column: 16, Offset: 16},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("boolean expressions", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("true + false")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.BoolExpression{
					Value: true,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				Right: &ast.BoolExpression{
					Value: false,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
			},
			result,
		)
	})

	t.Run("move operator, nested", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("(<-x)")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.UnaryExpression{
				Operation: ast.OperationMove,
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
			},
			result,
		)
	})

}

func TestParseArrayExpression(t *testing.T) {

	t.Parallel()

	t.Run("array expression", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("[ 1,2 + 3, 4  ,  5 ]")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ArrayExpression{
				Values: []ast.Expression{
					&ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
							EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
						},
					},
					&ast.BinaryExpression{
						Operation: ast.OperationPlus,
						Left: &ast.IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
								EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
							},
						},
						Right: &ast.IntegerExpression{
							Value: big.NewInt(3),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
								EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
							},
						},
					},
					&ast.IntegerExpression{
						Value: big.NewInt(4),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
							EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
						},
					},
					&ast.IntegerExpression{
						Value: big.NewInt(5),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 17, Offset: 17},
							EndPos:   ast.Position{Line: 1, Column: 17, Offset: 17},
						},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 19, Offset: 19},
				},
			},
			result,
		)
	})
}

func TestParseDictionaryExpression(t *testing.T) {

	t.Parallel()

	t.Run("dictionary expression", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("{ 1:2 + 3, 4  :  5 }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.DictionaryExpression{
				Entries: []ast.Entry{
					{
						Key: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
								EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
							},
						},
						Value: &ast.BinaryExpression{
							Operation: ast.OperationPlus,
							Left: &ast.IntegerExpression{
								Value: big.NewInt(2),
								Base:  10,
								Range: ast.Range{
									StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
									EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
								},
							},
							Right: &ast.IntegerExpression{
								Value: big.NewInt(3),
								Base:  10,
								Range: ast.Range{
									StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
									EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
								},
							},
						},
					},
					{
						Key: &ast.IntegerExpression{
							Value: big.NewInt(4),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
								EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
							},
						},
						Value: &ast.IntegerExpression{
							Value: big.NewInt(5),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 17, Offset: 17},
								EndPos:   ast.Position{Line: 1, Column: 17, Offset: 17},
							},
						},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 19, Offset: 19},
				},
			},
			result,
		)
	})
}

func TestParseIndexExpression(t *testing.T) {
	t.Run("index expression", func(t *testing.T) {
		result, errs := ParseExpression("a[0]")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IndexExpression{
				TargetExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				IndexingExpression: &ast.IntegerExpression{
					Value: big.NewInt(0),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
					EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
				},
			},
			result,
		)
	})
	t.Run("index expression with whitespace", func(t *testing.T) {
		result, errs := ParseExpression("a [ 0 ]")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IndexExpression{
				TargetExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				IndexingExpression: &ast.IntegerExpression{
					Value: big.NewInt(0),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
					EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
				},
			},
			result,
		)
	})
	t.Run("index expression with identifier", func(t *testing.T) {
		result, errs := ParseExpression("a [foo]")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IndexExpression{
				TargetExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				IndexingExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
					EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
				},
			},
			result,
		)
	})
}

func TestParseIdentifier(t *testing.T) {

	t.Parallel()

	t.Run("identifier in addition", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("a + 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
			result,
		)
	})
}

func TestParsePath(t *testing.T) {

	t.Parallel()

	result, errs := ParseExpression("/foo/bar")
	require.Empty(t, errs)

	utils.AssertEqualWithDiff(t,
		&ast.PathExpression{
			Domain: ast.Identifier{
				Identifier: "foo",
				Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
			},
			Identifier: ast.Identifier{
				Identifier: "bar",
				Pos:        ast.Position{Line: 1, Column: 5, Offset: 5},
			},
			StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
		},
		result,
	)
}

func TestParseString(t *testing.T) {

	t.Parallel()

	t.Run("valid, empty", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("\"\"")
		assert.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("invalid, empty, missing end at end of file", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("\"")
		assert.Equal(t,
			[]error{
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("invalid, empty, missing end at end of line", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("\"\n")
		assert.Equal(t,
			[]error{
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("invalid, non-empty, missing end at end of file", func(t *testing.T) {

		t.Parallel()
		result, errs := ParseExpression("\"t")
		assert.Equal(t,
			[]error{
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "t",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("invalid, non-empty, missing end at end of line", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("\"t\n")
		assert.Equal(t,
			[]error{
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "t",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("invalid, non-empty, missing escape character", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("\"\\")
		assert.Equal(t,
			[]error{
				errors.New("incomplete escape sequence: missing character after escape character"),
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("valid, with escapes", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\tst\"te\u{1F3CE}\u{FE0F}xt"`)
		assert.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "te\tst\"te\U0001F3CE\uFE0Fxt",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 30, Offset: 30},
				},
			},
			result,
		)
	})

	t.Run("invalid, unknown escape character", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\Xst"`)
		assert.Equal(t,
			[]error{
				errors.New("invalid escape character: 'X'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "test",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("invalid, missing '{' after Unicode escape character", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\u`)
		assert.Equal(t,
			[]error{
				errors.New("incomplete Unicode escape sequence: missing character '{' after escape character"),
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "te",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
				},
			},
			result,
		)
	})

	t.Run("invalid, invalid character after Unicode escape character", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\us`)
		assert.Equal(t,
			[]error{
				errors.New("invalid Unicode escape sequence: expected '{', got 's'"),
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "te",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
				},
			},
			result,
		)
	})

	t.Run("invalid, missing '}' after Unicode escape sequence digits", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\u{`)
		assert.Equal(t,
			[]error{
				errors.New("incomplete Unicode escape sequence: missing character '}' after escape character"),
				errors.New("invalid end of string literal: missing '\"'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "te",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
				},
			},
			result,
		)
	})

	t.Run("valid, empty Unicode escape sequence", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\u{}"`)
		assert.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "te",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("valid, non-empty Unicode escape sequence", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(
			`"te\u{73}t ` +
				`\u{4A}J\u{4a}J ` +
				`\u{4B}K\u{4b}K ` +
				`\u{4C}L\u{4c}L ` +
				`\u{4D}M\u{4d}M ` +
				`\u{4E}N\u{4e}N ` +
				`\u{4F}O\u{4f}O"`,
		)
		assert.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "test JJJJ KKKK LLLL MMMM NNNN OOOO",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 100, Offset: 100},
				},
			},
			result,
		)
	})

	t.Run("invalid, non-empty Unicode escape sequence", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`"te\u{X}st"`)
		assert.Equal(t,
			[]error{
				errors.New("invalid Unicode escape sequence: expected hex digit, got 'X'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.StringExpression{
				Value: "test",
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
				},
			},
			result,
		)
	})
}

func TestInvocation(t *testing.T) {

	t.Parallel()

	t.Run("no arguments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: nil,
				EndPos:    ast.Position{Offset: 2, Line: 1, Column: 2},
			},
			result,
		)
	})

	t.Run("no arguments, with whitespace", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f ()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: nil,
				EndPos:    ast.Position{Offset: 3, Line: 1, Column: 3},
			},
			result,
		)
	})

	t.Run("no arguments, with whitespace within params", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f ( )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: nil,
				EndPos:    ast.Position{Offset: 4, Line: 1, Column: 4},
			},
			result,
		)
	})

	t.Run("with arguments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(1)")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label: "",
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 2, Line: 1, Column: 2},
								EndPos:   ast.Position{Offset: 2, Line: 1, Column: 2},
							},
						},
					},
				},
				EndPos: ast.Position{Offset: 3, Line: 1, Column: 3},
			},
			result,
		)
	})

	t.Run("with labeled arguments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(label:1)")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label:         "label",
						LabelStartPos: &ast.Position{Offset: 2, Line: 1, Column: 2},
						LabelEndPos:   &ast.Position{Offset: 6, Line: 1, Column: 6},
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 8, Line: 1, Column: 8},
								EndPos:   ast.Position{Offset: 8, Line: 1, Column: 8},
							},
						},
					},
				},
				EndPos: ast.Position{Offset: 9, Line: 1, Column: 9},
			},
			result,
		)
	})

	t.Run("with arguments, multiple", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(1,2)")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label: "",
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 2, Line: 1, Column: 2},
								EndPos:   ast.Position{Offset: 2, Line: 1, Column: 2},
							},
						},
					},
					{
						Label: "",
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 4, Line: 1, Column: 4},
								EndPos:   ast.Position{Offset: 4, Line: 1, Column: 4},
							},
						},
					},
				},
				EndPos: ast.Position{Offset: 5, Line: 1, Column: 5},
			},
			result,
		)
	})

	t.Run("with arguments, multiple, labeled", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(a:1,b:2)")
		require.Empty(t, errs)

		assert.Equal(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label:         "a",
						LabelStartPos: &ast.Position{Offset: 2, Line: 1, Column: 2},
						LabelEndPos:   &ast.Position{Offset: 2, Line: 1, Column: 2},
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 4, Line: 1, Column: 4},
								EndPos:   ast.Position{Offset: 4, Line: 1, Column: 4},
							},
						},
					},
					{
						Label:         "b",
						LabelStartPos: &ast.Position{Offset: 6, Line: 1, Column: 6},
						LabelEndPos:   &ast.Position{Offset: 6, Line: 1, Column: 6},
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 8, Line: 1, Column: 8},
								EndPos:   ast.Position{Offset: 8, Line: 1, Column: 8},
							},
						},
					},
				},
				EndPos: ast.Position{Offset: 9, Line: 1, Column: 9},
			},
			result,
		)
	})

	t.Run("invalid: no arguments, multiple commas", func(t *testing.T) {

		t.Parallel()

		_, errs := ParseExpression("f(,,)")
		require.Equal(t,
			[]error{
				fmt.Errorf(
					"expected argument or end of argument list, got %s",
					lexer.TokenComma,
				),
			},
			errs,
		)
	})

	t.Run("invalid: with argument, multiple commas", func(t *testing.T) {

		t.Parallel()

		_, errs := ParseExpression("f(1,,)")
		require.Equal(t,
			[]error{
				fmt.Errorf(
					"expected argument or end of argument list, got %s",
					lexer.TokenComma,
				),
			},
			errs,
		)
	})

	t.Run("invalid: with multiple argument, no commas", func(t *testing.T) {

		t.Parallel()

		_, errs := ParseExpression("f(1 2)")
		require.Equal(t,
			[]error{
				fmt.Errorf(
					"unexpected argument in argument list (expecting delimiter or end of argument list), got %s",
					lexer.TokenDecimalLiteral,
				),
			},
			errs,
		)
	})

	t.Run("with arguments, nested", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(1,g(2))")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label: "",
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 2, Line: 1, Column: 2},
								EndPos:   ast.Position{Offset: 2, Line: 1, Column: 2},
							},
						},
					},
					{
						Label: "",
						Expression: &ast.InvocationExpression{
							InvokedExpression: &ast.IdentifierExpression{
								Identifier: ast.Identifier{
									Identifier: "g",
									Pos:        ast.Position{Offset: 4, Line: 1, Column: 4},
								},
							},
							Arguments: []*ast.Argument{
								{
									Label: "",
									Expression: &ast.IntegerExpression{
										Value: big.NewInt(2),
										Base:  10,
										Range: ast.Range{
											StartPos: ast.Position{Offset: 6, Line: 1, Column: 6},
											EndPos:   ast.Position{Offset: 6, Line: 1, Column: 6},
										},
									},
								},
							},
							EndPos: ast.Position{Offset: 7, Line: 1, Column: 7},
						},
					},
				},
				EndPos: ast.Position{Offset: 8, Line: 1, Column: 8},
			},
			result,
		)
	})

	t.Run("with arguments, nested, string", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f(1,g(\"test\"))")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Arguments: []*ast.Argument{
					{
						Label: "",
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Offset: 2, Line: 1, Column: 2},
								EndPos:   ast.Position{Offset: 2, Line: 1, Column: 2},
							},
						},
					},
					{
						Label: "",
						Expression: &ast.InvocationExpression{
							InvokedExpression: &ast.IdentifierExpression{
								Identifier: ast.Identifier{
									Identifier: "g",
									Pos:        ast.Position{Offset: 4, Line: 1, Column: 4},
								},
							},
							Arguments: []*ast.Argument{
								{
									Label: "",
									Expression: &ast.StringExpression{
										Value: "test",
										Range: ast.Range{
											StartPos: ast.Position{Offset: 6, Line: 1, Column: 6},
											EndPos:   ast.Position{Offset: 11, Line: 1, Column: 11},
										},
									},
								},
							},
							EndPos: ast.Position{Offset: 12, Line: 1, Column: 12},
						},
					},
				},
				EndPos: ast.Position{Offset: 13, Line: 1, Column: 13},
			},
			result,
		)
	})
}

func TestMemberExpression(t *testing.T) {

	t.Parallel()

	t.Run("identifier, no space", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f.n")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.MemberExpression{
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Identifier: ast.Identifier{
					Identifier: "n",
					Pos:        ast.Position{Offset: 2, Line: 1, Column: 2},
				},
			},
			result,
		)
	})

	t.Run("whitespace between", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f .n")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.MemberExpression{
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Identifier: ast.Identifier{
					Identifier: "n",
					Pos:        ast.Position{Offset: 3, Line: 1, Column: 3},
				},
			},
			result,
		)
	})

	t.Run("precedence, left", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f.n * 3")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationMul,
				Left: &ast.MemberExpression{
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "f",
							Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
						},
					},
					Identifier: ast.Identifier{
						Identifier: "n",
						Pos:        ast.Position{Offset: 2, Line: 1, Column: 2},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Offset: 6, Line: 1, Column: 6},
						EndPos:   ast.Position{Offset: 6, Line: 1, Column: 6},
					},
				},
			},
			result,
		)
	})

	t.Run("precedence, right", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("3 * f.n")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationMul,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(3),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Offset: 0, Line: 1, Column: 0},
						EndPos:   ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Right: &ast.MemberExpression{
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "f",
							Pos:        ast.Position{Offset: 4, Line: 1, Column: 4},
						},
					},
					Identifier: ast.Identifier{
						Identifier: "n",
						Pos:        ast.Position{Offset: 6, Line: 1, Column: 6},
					},
				},
			},
			result,
		)
	})

	t.Run("identifier, optional", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("f?.n")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.MemberExpression{
				Optional: true,
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "f",
						Pos:        ast.Position{Offset: 0, Line: 1, Column: 0},
					},
				},
				Identifier: ast.Identifier{
					Identifier: "n",
					Pos:        ast.Position{Offset: 3, Line: 1, Column: 3},
				},
			},
			result,
		)
	})
}

func TestParseBlockComment(t *testing.T) {

	t.Parallel()

	t.Run("nested comment, nothing else", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" /* test  foo/* bar  */ asd*/ true")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BoolExpression{
				Value: true,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 30, Offset: 30},
					EndPos:   ast.Position{Line: 1, Column: 33, Offset: 33},
				},
			},
			result,
		)
	})

	t.Run("two comments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" /*test  foo*/ /* bar  */ true")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BoolExpression{
				Value: true,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 26, Offset: 26},
					EndPos:   ast.Position{Line: 1, Column: 29, Offset: 29},
				},
			},
			result,
		)
	})

	t.Run("in infix", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" 1/*test  foo*/+/* bar  */ 2  ")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(2),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 27, Offset: 27},
						EndPos:   ast.Position{Line: 1, Column: 27, Offset: 27},
					},
				},
			},
			result,
		)
	})
}

func BenchmarkParseInfix(b *testing.B) {

	b.Run("new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ParseExpression("(8 - 1 + 3) * 6 - ((3 + 7) * 2)")
		}
	})

	b.Run("old", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, _ = oldParser.ParseExpression("(8 - 1 + 3) * 6 - ((3 + 7) * 2)")
		}
	})
}

func BenchmarkParseArray(b *testing.B) {

	var builder strings.Builder
	for i := 0; i < 10_000; i++ {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Itoa(rand.Intn(math.MaxUint8)))
	}

	lit := fmt.Sprintf(`[%s]`, builder.String())

	b.ResetTimer()

	b.Run("new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ParseExpression(lit)
		}
	})

	b.Run("old", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, _ = oldParser.ParseExpression(lit)
		}
	})
}

func TestParseReference(t *testing.T) {

	t.Parallel()

	result, errs := ParseExpression("& t as T")
	require.Empty(t, errs)

	utils.AssertEqualWithDiff(t,
		&ast.ReferenceExpression{
			Expression: &ast.IdentifierExpression{
				Identifier: ast.Identifier{
					Identifier: "t",
					Pos:        ast.Position{Line: 1, Column: 2, Offset: 2},
				},
			},
			Type: &ast.NominalType{
				Identifier: ast.Identifier{
					Identifier: "T",
					Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
		},
		result,
	)
}

func TestParseCasts(t *testing.T) {

	t.Parallel()

	t.Run("non-failable", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" t as T")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.CastingExpression{
				Operation: ast.OperationCast,
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				TypeAnnotation: &ast.TypeAnnotation{
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "T",
							Pos:        ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
				},
			},
			result,
		)
	})

	t.Run("failable", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" t as? T")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.CastingExpression{
				Operation: ast.OperationFailableCast,
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				TypeAnnotation: &ast.TypeAnnotation{
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "T",
							Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)

	})

	t.Run("force", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" t as! T")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.CastingExpression{
				Operation: ast.OperationForceCast,
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				TypeAnnotation: &ast.TypeAnnotation{
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "T",
							Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)

	})
}

func TestParseForceExpression(t *testing.T) {

	t.Parallel()

	t.Run("identifier", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("t!")
		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			&ast.ForceExpression{
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				EndPos: ast.Position{Line: 1, Column: 1, Offset: 1},
			},
			result,
		)
	})

	t.Run("with whitespace", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(" t ! ")
		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			&ast.ForceExpression{
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				EndPos: ast.Position{Line: 1, Column: 3, Offset: 3},
			},
			result,
		)
	})

	t.Run("precedence, force unwrap before move", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("<-t!")
		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			&ast.UnaryExpression{
				Operation: ast.OperationMove,
				Expression: &ast.ForceExpression{
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "t",
							Pos:        ast.Position{Line: 1, Column: 2, Offset: 2},
						},
					},
					EndPos: ast.Position{Line: 1, Column: 3, Offset: 3},
				},
				StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
			},
			result,
		)
	})

	t.Run("precedence", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("10 *  t!")
		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationMul,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(10),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				Right: &ast.ForceExpression{
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "t",
							Pos:        ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					EndPos: ast.Position{Line: 1, Column: 7, Offset: 7},
				}},
			result,
		)
	})

	t.Run("newline", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("x\n!y")

		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "x",
							Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
						},
					},
				},
				&ast.ExpressionStatement{
					Expression: &ast.UnaryExpression{
						Operation: ast.OperationNegate,
						Expression: &ast.IdentifierExpression{
							Identifier: ast.Identifier{
								Identifier: "y",
								Pos:        ast.Position{Line: 2, Column: 1, Offset: 3},
							},
						},
						StartPos: ast.Position{Line: 2, Column: 0, Offset: 2},
					},
				},
			},
			result,
		)
	})

	t.Run("member access, newline", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("x\n.y!")

		require.Empty(t, errs)
		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.ForceExpression{
						Expression: &ast.MemberExpression{
							Expression: &ast.IdentifierExpression{
								Identifier: ast.Identifier{
									Identifier: "x",
									Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
								},
							},
							Identifier: ast.Identifier{
								Identifier: "y",
								Pos:        ast.Position{Line: 2, Column: 1, Offset: 3},
							},
						},
						EndPos: ast.Position{Line: 2, Column: 2, Offset: 4},
					},
				},
			},
			result,
		)
	})
}

func TestParseCreate(t *testing.T) {

	t.Parallel()

	t.Run("simple", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("create T()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.CreateExpression{
				InvocationExpression: &ast.InvocationExpression{
					InvokedExpression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "T",
							Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
					EndPos: ast.Position{Line: 1, Column: 9, Offset: 9},
				},
				StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
			},
			result,
		)
	})
}

func TestParseNil(t *testing.T) {

	t.Parallel()

	result, errs := ParseExpression(" nil")
	require.Empty(t, errs)

	utils.AssertEqualWithDiff(t,
		&ast.NilExpression{
			Pos: ast.Position{Line: 1, Column: 1, Offset: 1},
		},
		result,
	)
}

func TestParseDestroy(t *testing.T) {

	t.Parallel()

	t.Run("simple", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("destroy t")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.DestroyExpression{
				Expression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "t",
						Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
				StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
			},
			result,
		)
	})
}

func TestParseLineComment(t *testing.T) {

	t.Parallel()

	result, errs := ParseExpression(" //// // this is a comment\n 1 / 2")
	require.Empty(t, errs)

	utils.AssertEqualWithDiff(t,
		&ast.BinaryExpression{
			Operation: ast.OperationDiv,
			Left: &ast.IntegerExpression{
				Value: big.NewInt(1),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 2, Column: 1, Offset: 28},
					EndPos:   ast.Position{Line: 2, Column: 1, Offset: 28},
				},
			},
			Right: &ast.IntegerExpression{
				Value: big.NewInt(2),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 2, Column: 5, Offset: 32},
					EndPos:   ast.Position{Line: 2, Column: 5, Offset: 32},
				},
			},
		},
		result,
	)
}

func TestParseFunctionExpression(t *testing.T) {

	t.Parallel()

	t.Run("without return type", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("fun () { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FunctionExpression{
				ParameterList: &ast.ParameterList{
					Parameters: nil,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				ReturnTypeAnnotation: &ast.TypeAnnotation{
					IsResource: false,
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "",
							Pos:        ast.Position{Line: 1, Column: 5, Offset: 5},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
				},
				FunctionBlock: &ast.FunctionBlock{
					Block: &ast.Block{
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
				},
				StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
			},
			result,
		)
	})

	t.Run("with return type", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("fun (): X { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FunctionExpression{
				ParameterList: &ast.ParameterList{
					Parameters: nil,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				ReturnTypeAnnotation: &ast.TypeAnnotation{
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "X",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
				},
				FunctionBlock: &ast.FunctionBlock{
					Block: &ast.Block{
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
							EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
						},
					},
				},
				StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
			},
			result,
		)
	})
}

func TestParseIntegerLiterals(t *testing.T) {

	t.Parallel()

	t.Run("binary prefix, missing trailing digits", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0b`)
		require.Equal(t,
			[]error{
				errors.New("missing digits"),
				&InvalidIntegerLiteralError{
					Literal:                   "0b",
					IntegerLiteralKind:        IntegerLiteralKindBinary,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindUnknown,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: nil,
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("binary", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0b101010`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(42),
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("binary with leading zeros", func(t *testing.T) {

		result, errs := ParseExpression(`0b001000`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(8),
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("binary with underscores", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0b101010_101010`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(2730),
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 14, Offset: 14},
				},
			},
			result,
		)
	})

	t.Run("binary with leading underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0b_101010_101010`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "0b_101010_101010",
					IntegerLiteralKind:        IntegerLiteralKindBinary,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindLeadingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(2730),
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
				},
			},
			result,
		)
	})

	t.Run("binary with trailing underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0b101010_101010_`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "0b101010_101010_",
					IntegerLiteralKind:        IntegerLiteralKindBinary,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindTrailingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(2730),
				Base:  2,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
				},
			},
			result,
		)
	})

	t.Run("octal prefix, missing trailing digits", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0o`)
		require.Equal(t,
			[]error{
				errors.New("missing digits"),
				&InvalidIntegerLiteralError{
					Literal:                   `0o`,
					IntegerLiteralKind:        IntegerLiteralKindOctal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindUnknown,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: nil,
				Base:  8,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("octal", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0o32`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(26),
				Base:  8,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
				},
			},
			result,
		)
	})

	t.Run("octal with underscores", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0o32_45`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1701),
				Base:  8,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
				},
			},
			result,
		)
	})

	t.Run("octal with trailing underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0o_32_45`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "0o_32_45",
					IntegerLiteralKind:        IntegerLiteralKindOctal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindLeadingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1701),
				Base:  8,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("octal with leading underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0o32_45_`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "0o32_45_",
					IntegerLiteralKind:        IntegerLiteralKindOctal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindTrailingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1701),
				Base:  8,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("decimal", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`1234567890`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1234567890),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
				},
			},
			result,
		)
	})

	t.Run("decimal with underscores", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`1_234_567_890`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1234567890),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
				},
			},
			result,
		)
	})

	t.Run("decimal with trailing underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`1_234_567_890_`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "1_234_567_890_",
					IntegerLiteralKind:        IntegerLiteralKindDecimal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindTrailingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1234567890),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
				},
			},
			result,
		)
	})

	t.Run("hexadecimal prefix, missing trailing digits", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0x`)
		require.Equal(t,
			[]error{
				errors.New("missing digits"),
				&InvalidIntegerLiteralError{
					Literal:                   `0x`,
					IntegerLiteralKind:        IntegerLiteralKindHexadecimal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindUnknown,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: nil,
				Base:  16,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("hexadecimal", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0xf2`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(242),
				Base:  16,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
				},
			},
			result,
		)
	})

	t.Run("hexadecimal with underscores", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0xf2_09`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(61961),
				Base:  16,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
				},
			},
			result,
		)
	})

	t.Run("hexadecimal with leading underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0x_f2_09`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   "0x_f2_09",
					IntegerLiteralKind:        IntegerLiteralKindHexadecimal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindLeadingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(61961),
				Base:  16,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("hexadecimal with trailing underscore", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0xf2_09_`)
		require.Equal(t,
			[]error{
				&InvalidIntegerLiteralError{
					Literal:                   `0xf2_09_`,
					IntegerLiteralKind:        IntegerLiteralKindHexadecimal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindTrailingUnderscore,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(61961),
				Base:  16,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
				},
			},
			result,
		)
	})

	t.Run("0", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(0),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("01", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`01`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(1),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("09", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`09`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(9),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("leading zeros", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("00123")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: big.NewInt(123),
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
				},
			},
			result,
		)
	})

	t.Run("invalid prefix", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression(`0z123`)
		require.Equal(t,
			[]error{
				errors.New("invalid number literal prefix: 'z'"),
				&InvalidIntegerLiteralError{
					Literal:                   `0z123`,
					IntegerLiteralKind:        IntegerLiteralKindDecimal,
					InvalidIntegerLiteralKind: InvalidNumberLiteralKindUnknown,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.IntegerExpression{
				Value: nil,
				Base:  10,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
				},
			},
			result,
		)
	})
}

func TestParseFixedPoint(t *testing.T) {

	t.Parallel()

	t.Run("with underscores", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1234_5678_90.0009_8765_4321")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FixedPointExpression{
				Negative:        false,
				UnsignedInteger: big.NewInt(1234567890),
				Fractional:      big.NewInt(987654321),
				Scale:           12,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 26, Offset: 26},
				},
			},
			result,
		)
	})

	t.Run("leading zero", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("0.1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FixedPointExpression{
				Negative:        false,
				UnsignedInteger: big.NewInt(0),
				Fractional:      big.NewInt(1),
				Scale:           1,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
				},
			},
			result,
		)
	})

	t.Run("missing fractional digits", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("0.")
		require.Equal(t,
			[]error{
				errors.New("missing fractional digits"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			&ast.FixedPointExpression{
				Negative:        false,
				UnsignedInteger: big.NewInt(0),
				Fractional:      big.NewInt(0),
				Scale:           1,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})
}

func TestParseLessThanOrTypeArguments(t *testing.T) {

	t.Run("binary expression with less operator", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 < 2")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationLess,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(2),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
			result,
		)
	})

	t.Run("invocation, zero type arguments, zero arguments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("a < > ()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				TypeArguments: nil,
				Arguments:     nil,
				EndPos:        ast.Position{Line: 1, Column: 7, Offset: 7},
			},
			result,
		)
	})

	t.Run("invocation, one type argument, one argument", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("a < { K : V } > ( 1 )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				TypeArguments: []*ast.TypeAnnotation{
					{
						IsResource: false,
						Type: &ast.DictionaryType{
							KeyType: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "K",
									Pos:        ast.Position{Line: 1, Column: 6, Offset: 6},
								},
							},
							ValueType: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "V",
									Pos:        ast.Position{Line: 1, Column: 10, Offset: 10},
								},
							},
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
								EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				Arguments: []*ast.Argument{
					{
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 18, Offset: 18},
								EndPos:   ast.Position{Line: 1, Column: 18, Offset: 18},
							},
						},
					},
				},
				EndPos: ast.Position{Line: 1, Column: 20, Offset: 20},
			},
			result,
		)
	})

	t.Run("invocation, three type arguments, two arguments", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("a < { K : V } , @R , [ S ] > ( 1 , 2 )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.InvocationExpression{
				InvokedExpression: &ast.IdentifierExpression{
					Identifier: ast.Identifier{
						Identifier: "a",
						Pos:        ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				TypeArguments: []*ast.TypeAnnotation{
					{
						IsResource: false,
						Type: &ast.DictionaryType{
							KeyType: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "K",
									Pos:        ast.Position{Line: 1, Column: 6, Offset: 6},
								},
							},
							ValueType: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "V",
									Pos:        ast.Position{Line: 1, Column: 10, Offset: 10},
								},
							},
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
								EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					{
						IsResource: true,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "R",
								Pos:        ast.Position{Line: 1, Column: 17, Offset: 17},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 16, Offset: 16},
					},
					{
						IsResource: false,
						Type: &ast.VariableSizedType{
							Type: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "S",
									Pos:        ast.Position{Line: 1, Column: 23, Offset: 23},
								},
							},
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 21, Offset: 21},
								EndPos:   ast.Position{Line: 1, Column: 25, Offset: 25},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 21, Offset: 21},
					},
				},
				Arguments: []*ast.Argument{
					{
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 31, Offset: 31},
								EndPos:   ast.Position{Line: 1, Column: 31, Offset: 31},
							},
						},
					},
					{
						Expression: &ast.IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 35, Offset: 35},
								EndPos:   ast.Position{Line: 1, Column: 35, Offset: 35},
							},
						},
					},
				},
				EndPos: ast.Position{Line: 1, Column: 37, Offset: 37},
			},
			result,
		)
	})

	t.Run("precedence, invocation in binary expression", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("1 + a<>()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				Right: &ast.InvocationExpression{
					InvokedExpression: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "a",
							Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
						},
					},
					TypeArguments: nil,
					Arguments:     nil,
					EndPos:        ast.Position{Line: 1, Column: 8, Offset: 8},
				},
			},
			result,
		)
	})

	t.Run("precedence, binary expressions", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseExpression("0 + 1 < 2")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.BinaryExpression{
				Operation: ast.OperationLess,
				Left: &ast.BinaryExpression{
					Operation: ast.OperationPlus,
					Left: &ast.IntegerExpression{
						Value: big.NewInt(0),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
							EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
						},
					},
					Right: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
							EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
						},
					},
				},
				Right: &ast.IntegerExpression{
					Value: big.NewInt(2),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
			result,
		)
	})

}

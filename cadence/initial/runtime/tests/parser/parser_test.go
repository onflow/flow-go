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

package parser

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/parser"
	"example.com/cadence-initial/runtime/parser2"
	"example.com/cadence-initial/runtime/tests/utils"
)

func TestParseReplInput(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseReplInput(`
        struct X {}; let x = X(); x
    `)

	require.NoError(t, err)
	require.IsType(t, []interface{}{}, actual)

	require.Len(t, actual, 3)
	assert.IsType(t, &CompositeDeclaration{}, actual[0])
	assert.IsType(t, &VariableDeclaration{}, actual[1])
	assert.IsType(t, &ExpressionStatement{}, actual[2])
}

func TestParseInvalidProgramWithRest(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
	    .asd
	`)

	assert.Nil(t, actual)
	assert.IsType(t, parser.Error{}, err)
}

func TestParseInvalidIncompleteConstKeyword(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
	    le
	`)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 6, Line: 2, Column: 5},
		syntaxError.Pos,
	)

	assert.Contains(t, syntaxError.Message, "extraneous input")
}

func TestParseInvalidIncompleteStringLiteral(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
	    let = "Hello, World!
	`)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 3)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 26, Line: 2, Column: 11},
		syntaxError.Pos,
	)

	assert.Contains(t, syntaxError.Message, "token recognition error")
}

func TestParseNames(t *testing.T) {

	t.Parallel()

	names := map[string]bool{
		// Valid: title-case
		//
		"PersonID": true,

		// Valid: with underscore
		//
		"token_name": true,

		// Valid: leading underscore and characters
		//
		"_balance": true,

		// Valid: leading underscore and numbers
		"_8264": true,

		// Valid: characters and number
		//
		"account2": true,

		// Invalid: leading number
		//
		"1something": false,

		// Invalid: invalid character #
		"_#1": false,

		// Invalid: various invalid characters
		//
		"!@#$%^&*": false,
	}

	for name, validExpected := range names {

		code := fmt.Sprintf(`let %s = 1`, name)

		t.Run("old", func(t *testing.T) {

			actual, _, err := parser.ParseProgram(code)

			if validExpected {
				assert.NotNil(t, actual)
				assert.NoError(t, err)

			} else {
				assert.Nil(t, actual)
				assert.IsType(t, parser.Error{}, err)
			}
		})

		t.Run("new", func(t *testing.T) {

			actual, err := parser2.ParseProgram(code)

			if validExpected {
				assert.NotNil(t, actual)
				assert.NoError(t, err)

			} else {
				assert.Nil(t, actual)
				assert.IsType(t, parser2.Error{}, err)
			}
		})
	}
}

func TestParseInvalidIncompleteConstantDeclaration1(t *testing.T) {

	t.Parallel()

	actual, inputIsComplete, err := parser.ParseProgram(`
	    let
	`)

	assert.False(t, inputIsComplete)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError1 := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 11, Line: 3, Column: 1},
		syntaxError1.Pos,
	)

	assert.Contains(t, syntaxError1.Message, "mismatched input")
}

func TestParseInvalidIncompleteConstantDeclaration2(t *testing.T) {

	t.Parallel()

	actual, inputIsComplete, err := parser.ParseProgram(`
	    let =
	`)

	assert.False(t, inputIsComplete)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 2)

	syntaxError1 := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 10, Line: 2, Column: 9},
		syntaxError1.Pos,
	)

	assert.Contains(t, syntaxError1.Message, "missing")

	syntaxError2 := errors[1].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 13, Line: 3, Column: 1},
		syntaxError2.Pos,
	)

	assert.Contains(t, syntaxError2.Message, "mismatched input")
}

func testParse(t *testing.T, code string, expected []Declaration, use func(actual *Program)) {

	expectedProgram := &Program{
		Declarations: expected,
	}

	t.Run("old", func(t *testing.T) {
		actual, _, err := parser.ParseProgram(code)
		require.NoError(t, err)
		if expected != nil {
			utils.AssertEqualWithDiff(t, expectedProgram, actual)
		}

		if use != nil {
			use(actual)
		}
	})

	t.Run("new", func(t *testing.T) {
		actual, err := parser2.ParseProgram(code)
		require.NoError(t, err)
		if expected != nil {
			utils.AssertEqualWithDiff(t, expectedProgram, actual)
		}

		if use != nil {
			use(actual)
		}
	})
}

func TestParseBoolExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = true
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &BoolExpression{
					Value: true,
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseIdentifierExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let b = a
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "b",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "a",
						Pos:        Position{Offset: 14, Line: 2, Column: 13},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseArrayExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = [1, 2]
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "a",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &ArrayExpression{
					Values: []Expression{
						&IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
								EndPos:   Position{Offset: 15, Line: 2, Column: 14},
							},
						},
						&IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 18, Line: 2, Column: 17},
								EndPos:   Position{Offset: 18, Line: 2, Column: 17},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 19, Line: 2, Column: 18},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseDictionaryExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let x = {"a": 1, "b": 2}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "x",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &DictionaryExpression{
					Entries: []Entry{
						{
							Key: &StringExpression{
								Value: "a",
								Range: Range{
									StartPos: Position{Offset: 15, Line: 2, Column: 14},
									EndPos:   Position{Offset: 17, Line: 2, Column: 16},
								},
							},
							Value: &IntegerExpression{
								Value: big.NewInt(1),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 20, Line: 2, Column: 19},
									EndPos:   Position{Offset: 20, Line: 2, Column: 19},
								},
							},
						},
						{
							Key: &StringExpression{
								Value: "b",
								Range: Range{
									StartPos: Position{Offset: 23, Line: 2, Column: 22},
									EndPos:   Position{Offset: 25, Line: 2, Column: 24},
								},
							},
							Value: &IntegerExpression{
								Value: big.NewInt(2),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 28, Line: 2, Column: 27},
									EndPos:   Position{Offset: 28, Line: 2, Column: 27},
								},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 29, Line: 2, Column: 28},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseInvocationExpressionWithoutLabels(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = b(1, 2)
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &InvocationExpression{
					InvokedExpression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "b",
							Pos:        Position{Offset: 14, Line: 2, Column: 13},
						},
					},
					Arguments: []*Argument{
						{
							Label: "",
							Expression: &IntegerExpression{
								Value: big.NewInt(1),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 16, Line: 2, Column: 15},
									EndPos:   Position{Offset: 16, Line: 2, Column: 15},
								},
							},
						},
						{
							Label: "",
							Expression: &IntegerExpression{
								Value: big.NewInt(2),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 19, Line: 2, Column: 18},
									EndPos:   Position{Offset: 19, Line: 2, Column: 18},
								},
							},
						},
					},
					EndPos: Position{Offset: 20, Line: 2, Column: 19},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseInvocationExpressionWithLabels(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = b(x: 1, y: 2)
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &InvocationExpression{
					InvokedExpression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "b",
							Pos:        Position{Offset: 14, Line: 2, Column: 13},
						},
					},
					Arguments: []*Argument{
						{
							Label:         "x",
							LabelStartPos: &Position{Offset: 16, Line: 2, Column: 15},
							LabelEndPos:   &Position{Offset: 16, Line: 2, Column: 15},
							Expression: &IntegerExpression{
								Value: big.NewInt(1),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 19, Line: 2, Column: 18},
									EndPos:   Position{Offset: 19, Line: 2, Column: 18},
								},
							},
						},
						{
							Label:         "y",
							LabelStartPos: &Position{Offset: 22, Line: 2, Column: 21},
							LabelEndPos:   &Position{Offset: 22, Line: 2, Column: 21},
							Expression: &IntegerExpression{
								Value: big.NewInt(2),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 25, Line: 2, Column: 24},
									EndPos:   Position{Offset: 25, Line: 2, Column: 24},
								},
							},
						},
					},
					EndPos: Position{Offset: 26, Line: 2, Column: 25},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseMemberExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = b.c
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &MemberExpression{
					Expression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "b",
							Pos:        Position{Offset: 14, Line: 2, Column: 13},
						},
					},
					Identifier: Identifier{
						Identifier: "c",
						Pos:        Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseOptionalMemberExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = b?.c
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &MemberExpression{
					Optional: true,
					Expression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "b",
							Pos:        Position{Offset: 14, Line: 2, Column: 13},
						},
					},
					Identifier: Identifier{
						Identifier: "c",
						Pos:        Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseIndexExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = b[1]
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &IndexExpression{
					TargetExpression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "b",
							Pos:        Position{Offset: 14, Line: 2, Column: 13},
						},
					},
					IndexingExpression: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseUnaryExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let foo = -boo
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "foo",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 14, Line: 2, Column: 13},
				},
				Value: &UnaryExpression{
					Operation: OperationMinus,
					Expression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "boo",
							Pos:        Position{Offset: 17, Line: 2, Column: 16},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseOrExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = false || true
	`
	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationOr,
					Left: &BoolExpression{
						Value: false,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
					Right: &BoolExpression{
						Value: true,
						Range: Range{
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 29, Line: 2, Column: 28},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseAndExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = false && true
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationAnd,
					Left: &BoolExpression{
						Value: false,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
					Right: &BoolExpression{
						Value: true,
						Range: Range{
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 29, Line: 2, Column: 28},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseEqualityExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = false == true
	`
	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationEqual,
					Left: &BoolExpression{
						Value: false,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
					Right: &BoolExpression{
						Value: true,
						Range: Range{
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 29, Line: 2, Column: 28},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseRelationalExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = 1 < 2
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationLess,
					Left: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 17, Line: 2, Column: 16},
						},
					},
					Right: &IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseAdditiveExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = 1 + 2
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationPlus,
					Left: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 17, Line: 2, Column: 16},
						},
					},
					Right: &IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseMultiplicativeExpression(t *testing.T) {

	t.Parallel()

	const code = `
        let a = 1 * 2
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationMul,
					Left: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 17, Line: 2, Column: 16},
						},
					},
					Right: &IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseFunctionExpressionAndReturn(t *testing.T) {

	t.Parallel()

	const code = `
	    let test = fun (): Int { return 1 }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &FunctionExpression{
					ParameterList: &ParameterList{
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 22, Line: 2, Column: 21},
						},
					},
					ReturnTypeAnnotation: &TypeAnnotation{
						IsResource: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 25, Line: 2, Column: 24},
							},
						},
						StartPos: Position{Offset: 25, Line: 2, Column: 24},
					},
					FunctionBlock: &FunctionBlock{
						Block: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Expression: &IntegerExpression{
										Value: big.NewInt(1),
										Base:  10,
										Range: Range{
											StartPos: Position{Offset: 38, Line: 2, Column: 37},
											EndPos:   Position{Offset: 38, Line: 2, Column: 37},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 31, Line: 2, Column: 30},
										EndPos:   Position{Offset: 38, Line: 2, Column: 37},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 29, Line: 2, Column: 28},
								EndPos:   Position{Offset: 40, Line: 2, Column: 39},
							},
						},
					},
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseFunctionAndBlock(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() { return }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Range: Range{
									StartPos: Position{Offset: 19, Line: 2, Column: 18},
									EndPos:   Position{Offset: 24, Line: 2, Column: 23},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 26, Line: 2, Column: 25},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseFunctionParameterWithoutLabel(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test(x: Int) { }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Label: "",
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 15, Line: 2, Column: 14},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 18, Line: 2, Column: 17},
									},
								},
								StartPos: Position{Offset: 18, Line: 2, Column: 17},
							},
							Range: Range{
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
								EndPos:   Position{Offset: 20, Line: 2, Column: 19},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 21, Line: 2, Column: 20},
						},
					},
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 23, Line: 2, Column: 22},
							EndPos:   Position{Offset: 25, Line: 2, Column: 24},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseFunctionParameterWithLabel(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test(x y: Int) { }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Label: "x",
							Identifier: Identifier{
								Identifier: "y",
								Pos:        Position{Offset: 17, Line: 2, Column: 16},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 20, Line: 2, Column: 19},
									},
								},
								StartPos: Position{Offset: 20, Line: 2, Column: 19},
							},
							Range: Range{
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
								EndPos:   Position{Offset: 22, Line: 2, Column: 21},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 23, Line: 2, Column: 22},
						},
					},
					StartPos: Position{Offset: 23, Line: 2, Column: 22},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 25, Line: 2, Column: 24},
							EndPos:   Position{Offset: 27, Line: 2, Column: 26},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseIfStatement(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            if true {
                return
            } else if false {
                false
                1
            } else {
                2
            }
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&IfStatement{
								Test: &BoolExpression{
									Value: true,
									Range: Range{
										StartPos: Position{Offset: 34, Line: 3, Column: 15},
										EndPos:   Position{Offset: 37, Line: 3, Column: 18},
									},
								},
								Then: &Block{
									Statements: []Statement{
										&ReturnStatement{
											Expression: nil,
											Range: Range{
												StartPos: Position{Offset: 57, Line: 4, Column: 16},
												EndPos:   Position{Offset: 62, Line: 4, Column: 21},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 39, Line: 3, Column: 20},
										EndPos:   Position{Offset: 76, Line: 5, Column: 12},
									},
								},
								Else: &Block{
									Statements: []Statement{
										&IfStatement{
											Test: &BoolExpression{
												Value: false,
												Range: Range{
													StartPos: Position{Offset: 86, Line: 5, Column: 22},
													EndPos:   Position{Offset: 90, Line: 5, Column: 26},
												},
											},
											Then: &Block{
												Statements: []Statement{
													&ExpressionStatement{
														Expression: &BoolExpression{
															Value: false,
															Range: Range{
																StartPos: Position{Offset: 110, Line: 6, Column: 16},
																EndPos:   Position{Offset: 114, Line: 6, Column: 20},
															},
														},
													},
													&ExpressionStatement{
														Expression: &IntegerExpression{
															Value: big.NewInt(1),
															Base:  10,
															Range: Range{
																StartPos: Position{Offset: 132, Line: 7, Column: 16},
																EndPos:   Position{Offset: 132, Line: 7, Column: 16},
															},
														},
													},
												},
												Range: Range{
													StartPos: Position{Offset: 92, Line: 5, Column: 28},
													EndPos:   Position{Offset: 146, Line: 8, Column: 12},
												},
											},
											Else: &Block{
												Statements: []Statement{
													&ExpressionStatement{
														Expression: &IntegerExpression{
															Value: big.NewInt(2),
															Base:  10,
															Range: Range{
																StartPos: Position{Offset: 171, Line: 9, Column: 16},
																EndPos:   Position{Offset: 171, Line: 9, Column: 16},
															},
														},
													},
												},
												Range: Range{
													StartPos: Position{Offset: 153, Line: 8, Column: 19},
													EndPos:   Position{Offset: 185, Line: 10, Column: 12},
												},
											},
											StartPos: Position{Offset: 83, Line: 5, Column: 19},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 83, Line: 5, Column: 19},
										EndPos:   Position{Offset: 185, Line: 10, Column: 12},
									},
								},
								StartPos: Position{Offset: 31, Line: 3, Column: 12},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 195, Line: 11, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseIfStatementWithVariableDeclaration(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            if var y = x {
                1
            } else {
                2
            }
        }
	`

	ifStatement := &IfStatement{
		Then: &Block{
			Statements: []Statement{
				&ExpressionStatement{
					Expression: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 62, Line: 4, Column: 16},
							EndPos:   Position{Offset: 62, Line: 4, Column: 16},
						},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 44, Line: 3, Column: 25},
				EndPos:   Position{Offset: 76, Line: 5, Column: 12},
			},
		},
		Else: &Block{
			Statements: []Statement{
				&ExpressionStatement{
					Expression: &IntegerExpression{
						Value: big.NewInt(2),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 101, Line: 6, Column: 16},
							EndPos:   Position{Offset: 101, Line: 6, Column: 16},
						},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 83, Line: 5, Column: 19},
				EndPos:   Position{Offset: 115, Line: 7, Column: 12},
			},
		},
		StartPos: Position{Offset: 31, Line: 3, Column: 12},
	}

	ifTestVariableDeclaration := &VariableDeclaration{
		IsConstant: false,
		Identifier: Identifier{
			Identifier: "y",
			Pos:        Position{Offset: 38, Line: 3, Column: 19},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 40, Line: 3, Column: 21},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "x",
				Pos:        Position{Offset: 42, Line: 3, Column: 23},
			},
		},
		StartPos:          Position{Offset: 34, Line: 3, Column: 15},
		ParentIfStatement: ifStatement,
	}

	ifStatement.Test = ifTestVariableDeclaration

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							ifStatement,
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 125, Line: 8, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseIfStatementNoElse(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            if true {
                return
            }
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&IfStatement{
								Test: &BoolExpression{
									Value: true,
									Range: Range{
										StartPos: Position{Offset: 34, Line: 3, Column: 15},
										EndPos:   Position{Offset: 37, Line: 3, Column: 18},
									},
								},
								Then: &Block{
									Statements: []Statement{
										&ReturnStatement{
											Expression: nil,
											Range: Range{
												StartPos: Position{Offset: 57, Line: 4, Column: 16},
												EndPos:   Position{Offset: 62, Line: 4, Column: 21},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 39, Line: 3, Column: 20},
										EndPos:   Position{Offset: 76, Line: 5, Column: 12},
									},
								},
								StartPos: Position{Offset: 31, Line: 3, Column: 12},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 86, Line: 6, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseWhileStatement(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            while true {
              return
              break
              continue
            }
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&WhileStatement{
								Test: &BoolExpression{
									Value: true,
									Range: Range{
										StartPos: Position{Offset: 37, Line: 3, Column: 18},
										EndPos:   Position{Offset: 40, Line: 3, Column: 21},
									},
								},
								Block: &Block{
									Statements: []Statement{
										&ReturnStatement{
											Expression: nil,
											Range: Range{
												StartPos: Position{Offset: 58, Line: 4, Column: 14},
												EndPos:   Position{Offset: 63, Line: 4, Column: 19},
											},
										},
										&BreakStatement{
											Range: Range{
												StartPos: Position{Offset: 79, Line: 5, Column: 14},
												EndPos:   Position{Offset: 83, Line: 5, Column: 18},
											},
										},
										&ContinueStatement{
											Range: Range{
												StartPos: Position{Offset: 99, Line: 6, Column: 14},
												EndPos:   Position{Offset: 106, Line: 6, Column: 21},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 42, Line: 3, Column: 23},
										EndPos:   Position{Offset: 120, Line: 7, Column: 12},
									},
								},
								StartPos: Position{Offset: 31, Line: 3, Column: 12},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 130, Line: 8, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseForStatement(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            for x in xs {}
        }
	`
	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ForStatement{
								Identifier: Identifier{
									Identifier: "x",
									Pos:        Position{Offset: 35, Line: 3, Column: 16},
								},
								Value: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "xs",
										Pos:        Position{Offset: 40, Line: 3, Column: 21},
									},
								},
								Block: &Block{
									Range: Range{
										StartPos: Position{Offset: 43, Line: 3, Column: 24},
										EndPos:   Position{Offset: 44, Line: 3, Column: 25},
									},
								},
								StartPos: Position{Offset: 31, Line: 3, Column: 12},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 54, Line: 4, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseAssignment(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            a = 1
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&AssignmentStatement{
								Target: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "a",
										Pos:        Position{Offset: 31, Line: 3, Column: 12},
									},
								},
								Transfer: &Transfer{
									Operation: TransferOperationCopy,
									Pos:       Position{Offset: 33, Line: 3, Column: 14},
								},
								Value: &IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 35, Line: 3, Column: 16},
										EndPos:   Position{Offset: 35, Line: 3, Column: 16},
									},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 45, Line: 4, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseAccessAssignment(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() {
            x.foo.bar[0][1].baz = 1
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&AssignmentStatement{
								Target: &MemberExpression{
									Expression: &IndexExpression{
										TargetExpression: &IndexExpression{
											TargetExpression: &MemberExpression{
												Expression: &MemberExpression{
													Expression: &IdentifierExpression{
														Identifier: Identifier{
															Identifier: "x",
															Pos:        Position{Offset: 31, Line: 3, Column: 12},
														},
													},
													Identifier: Identifier{
														Identifier: "foo",
														Pos:        Position{Offset: 33, Line: 3, Column: 14},
													},
												},
												Identifier: Identifier{
													Identifier: "bar",
													Pos:        Position{Offset: 37, Line: 3, Column: 18},
												},
											},
											IndexingExpression: &IntegerExpression{
												Value: new(big.Int),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 41, Line: 3, Column: 22},
													EndPos:   Position{Offset: 41, Line: 3, Column: 22},
												},
											},
											Range: Range{
												StartPos: Position{Offset: 40, Line: 3, Column: 21},
												EndPos:   Position{Offset: 42, Line: 3, Column: 23},
											},
										},
										IndexingExpression: &IntegerExpression{
											Value: big.NewInt(1),
											Base:  10,
											Range: Range{
												StartPos: Position{Offset: 44, Line: 3, Column: 25},
												EndPos:   Position{Offset: 44, Line: 3, Column: 25},
											},
										},
										Range: Range{
											StartPos: Position{Offset: 43, Line: 3, Column: 24},
											EndPos:   Position{Offset: 45, Line: 3, Column: 26},
										},
									},
									Identifier: Identifier{
										Identifier: "baz",
										Pos:        Position{Offset: 47, Line: 3, Column: 28},
									},
								},
								Transfer: &Transfer{
									Operation: TransferOperationCopy,
									Pos:       Position{Offset: 51, Line: 3, Column: 32},
								},
								Value: &IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 53, Line: 3, Column: 34},
										EndPos:   Position{Offset: 53, Line: 3, Column: 34},
									},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 63, Line: 4, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseExpressionStatementWithAccess(t *testing.T) {

	t.Parallel()

	const code = `
	    fun test() { x.foo.bar[0][1].baz }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ExpressionStatement{
								Expression: &MemberExpression{
									Expression: &IndexExpression{
										TargetExpression: &IndexExpression{
											TargetExpression: &MemberExpression{
												Expression: &MemberExpression{
													Expression: &IdentifierExpression{
														Identifier: Identifier{
															Identifier: "x",
															Pos:        Position{Offset: 19, Line: 2, Column: 18},
														},
													},
													Identifier: Identifier{
														Identifier: "foo",
														Pos:        Position{Offset: 21, Line: 2, Column: 20},
													},
												},
												Identifier: Identifier{
													Identifier: "bar",
													Pos:        Position{Offset: 25, Line: 2, Column: 24},
												},
											},
											IndexingExpression: &IntegerExpression{
												Value: new(big.Int),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 29, Line: 2, Column: 28},
													EndPos:   Position{Offset: 29, Line: 2, Column: 28},
												},
											},
											Range: Range{
												StartPos: Position{Offset: 28, Line: 2, Column: 27},
												EndPos:   Position{Offset: 30, Line: 2, Column: 29},
											},
										},
										IndexingExpression: &IntegerExpression{
											Value: big.NewInt(1),
											Base:  10,
											Range: Range{
												StartPos: Position{Offset: 32, Line: 2, Column: 31},
												EndPos:   Position{Offset: 32, Line: 2, Column: 31},
											},
										},
										Range: Range{
											StartPos: Position{Offset: 31, Line: 2, Column: 30},
											EndPos:   Position{Offset: 33, Line: 2, Column: 32},
										},
									},
									Identifier: Identifier{
										Identifier: "baz",
										Pos:        Position{Offset: 35, Line: 2, Column: 34},
									},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 17, Line: 2, Column: 16},
							EndPos:   Position{Offset: 39, Line: 2, Column: 38},
						},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseParametersAndArrayTypes(t *testing.T) {

	t.Parallel()

	const code = `
		pub fun test(a: Int32, b: [Int32; 2], c: [[Int32; 3]]): [[Int64]] {}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessPublic,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Identifier: Identifier{
								Identifier: "a",
								Pos:        Position{Offset: 16, Line: 2, Column: 15},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int32",
										Pos:        Position{Offset: 19, Line: 2, Column: 18},
									},
								},
								StartPos: Position{Offset: 19, Line: 2, Column: 18},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 23, Line: 2, Column: 22},
							},
						},
						{
							Identifier: Identifier{
								Identifier: "b",
								Pos:        Position{Offset: 26, Line: 2, Column: 25},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &ConstantSizedType{
									Type: &NominalType{
										Identifier: Identifier{
											Identifier: "Int32",
											Pos:        Position{Offset: 30, Line: 2, Column: 29},
										},
									},
									Size: &IntegerExpression{
										Value: big.NewInt(2),
										Base:  10,
										Range: Range{
											StartPos: Position{Offset: 37, Line: 2, Column: 36},
											EndPos:   Position{Offset: 37, Line: 2, Column: 36},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 29, Line: 2, Column: 28},
										EndPos:   Position{Offset: 38, Line: 2, Column: 37},
									},
								},
								StartPos: Position{Offset: 29, Line: 2, Column: 28},
							},
							Range: Range{
								StartPos: Position{Offset: 26, Line: 2, Column: 25},
								EndPos:   Position{Offset: 38, Line: 2, Column: 37},
							},
						},
						{
							Identifier: Identifier{
								Identifier: "c",
								Pos:        Position{Offset: 41, Line: 2, Column: 40},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &VariableSizedType{
									Type: &ConstantSizedType{
										Type: &NominalType{
											Identifier: Identifier{
												Identifier: "Int32",
												Pos:        Position{Offset: 46, Line: 2, Column: 45},
											},
										},
										Size: &IntegerExpression{
											Value: big.NewInt(3),
											Base:  10,
											Range: Range{
												StartPos: Position{Offset: 53, Line: 2, Column: 52},
												EndPos:   Position{Offset: 53, Line: 2, Column: 52},
											},
										},
										Range: Range{
											StartPos: Position{Offset: 45, Line: 2, Column: 44},
											EndPos:   Position{Offset: 54, Line: 2, Column: 53},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 44, Line: 2, Column: 43},
										EndPos:   Position{Offset: 55, Line: 2, Column: 54},
									},
								},
								StartPos: Position{Offset: 44, Line: 2, Column: 43},
							},
							Range: Range{
								StartPos: Position{Offset: 41, Line: 2, Column: 40},
								EndPos:   Position{Offset: 55, Line: 2, Column: 54},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 56, Line: 2, Column: 55},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &VariableSizedType{
						Type: &VariableSizedType{
							Type: &NominalType{
								Identifier: Identifier{Identifier: "Int64",
									Pos: Position{Offset: 61, Line: 2, Column: 60},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 60, Line: 2, Column: 59},
								EndPos:   Position{Offset: 66, Line: 2, Column: 65},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 59, Line: 2, Column: 58},
							EndPos:   Position{Offset: 67, Line: 2, Column: 66},
						},
					},
					StartPos: Position{Offset: 59, Line: 2, Column: 58},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 69, Line: 2, Column: 68},
							EndPos:   Position{Offset: 70, Line: 2, Column: 69},
						},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseDictionaryType(t *testing.T) {

	t.Parallel()

	const code = `
	    let x: {String: Int} = {}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "x",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &DictionaryType{
						KeyType: &NominalType{
							Identifier: Identifier{
								Identifier: "String",
								Pos:        Position{Offset: 14, Line: 2, Column: 13},
							},
						},
						ValueType: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 22, Line: 2, Column: 21},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 25, Line: 2, Column: 24},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 27, Line: 2, Column: 26},
				},
				Value: &DictionaryExpression{
					Range: Range{
						StartPos: Position{Offset: 29, Line: 2, Column: 28},
						EndPos:   Position{Offset: 30, Line: 2, Column: 29},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

// TODO: remove
func TestParseIntegerLiterals(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32
        let hex = 0xf2
        let binary = 0b101010
        let decimal = 1234567890
	`)

	require.NoError(t, err)

	octal := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "octal",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(26),
			Base:  8,
			Range: Range{
				StartPos: Position{Offset: 15, Line: 2, Column: 14},
				EndPos:   Position{Offset: 18, Line: 2, Column: 17},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	hex := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "hex",
			Pos:        Position{Offset: 32, Line: 3, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 36, Line: 3, Column: 16},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(242),
			Base:  16,
			Range: Range{
				StartPos: Position{Offset: 38, Line: 3, Column: 18},
				EndPos:   Position{Offset: 41, Line: 3, Column: 21},
			},
		},
		StartPos: Position{Offset: 28, Line: 3, Column: 8},
	}

	binary := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "binary",
			Pos:        Position{Offset: 55, Line: 4, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 62, Line: 4, Column: 19},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(42),
			Base:  2,
			Range: Range{
				StartPos: Position{Offset: 64, Line: 4, Column: 21},
				EndPos:   Position{Offset: 71, Line: 4, Column: 28},
			},
		},
		StartPos: Position{Offset: 51, Line: 4, Column: 8},
	}

	decimal := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 85, Line: 5, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 93, Line: 5, Column: 20},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(1234567890),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 95, Line: 5, Column: 22},
				EndPos:   Position{Offset: 104, Line: 5, Column: 31},
			},
		},
		StartPos: Position{Offset: 81, Line: 5, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{octal, hex, binary, decimal},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

// TODO: remove
func TestParseIntegerLiteralsWithUnderscores(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32_45
        let hex = 0xf2_09
        let binary = 0b101010_101010
        let decimal = 1_234_567_890
	`)

	require.NoError(t, err)

	octal := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "octal",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(1701),
			Base:  8,
			Range: Range{
				StartPos: Position{Offset: 15, Line: 2, Column: 14},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	hex := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "hex",
			Pos:        Position{Offset: 35, Line: 3, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 39, Line: 3, Column: 16},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(61961),
			Base:  16,
			Range: Range{
				StartPos: Position{Offset: 41, Line: 3, Column: 18},
				EndPos:   Position{Offset: 47, Line: 3, Column: 24},
			},
		},
		StartPos: Position{Offset: 31, Line: 3, Column: 8},
	}

	binary := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "binary",
			Pos:        Position{Offset: 61, Line: 4, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 68, Line: 4, Column: 19},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(2730),
			Base:  2,
			Range: Range{
				StartPos: Position{Offset: 70, Line: 4, Column: 21},
				EndPos:   Position{Offset: 84, Line: 4, Column: 35},
			},
		},
		StartPos: Position{Offset: 57, Line: 4, Column: 8},
	}

	decimal := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 98, Line: 5, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 106, Line: 5, Column: 20},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(1234567890),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 108, Line: 5, Column: 22},
				EndPos:   Position{Offset: 120, Line: 5, Column: 34},
			},
		},
		StartPos: Position{Offset: 94, Line: 5, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{octal, hex, binary, decimal},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

// TODO: remove
func TestParseInvalidIntegerLiteralPrefixWithout(t *testing.T) {

	t.Parallel()

	for _, prefix := range []string{"o", "b", "x"} {

		_, _, err := parser.ParseProgram(fmt.Sprintf(`let x = 0%s`, prefix))

		require.Error(t, err)

		require.IsType(t, parser.Error{}, err)

		errors := err.(parser.Error).Errors
		assert.Len(t, errors, 1)

		syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)
		assert.Equal(t,
			Position{Offset: 8, Line: 1, Column: 8},
			syntaxError.StartPos,
		)
	}
}

// TODO: remove
func TestParseInvalidOctalIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let octal = 0o_32_45
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	require.IsType(t, &parser.InvalidIntegerLiteralError{}, errors[0])

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 15, Line: 2, Column: 14},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 22, Line: 2, Column: 21},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindOctal,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseIntegerLiteralWithLeadingZeros(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
        let decimal = 0123
	`)

	require.NoError(t, err)

	decimal := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 21, Line: 2, Column: 20},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(123),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 23, Line: 2, Column: 22},
				EndPos:   Position{Offset: 26, Line: 2, Column: 25},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{decimal},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

// TODO: remove
func TestParseInvalidOctalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32_45_
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 15, Line: 2, Column: 14},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 22, Line: 2, Column: 21},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindOctal,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseInvalidBinaryIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let binary = 0b_101010_101010
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 16, Line: 2, Column: 15},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 31, Line: 2, Column: 30},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindBinary,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseInvalidBinaryIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let binary = 0b101010_101010_
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 16, Line: 2, Column: 15},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 31, Line: 2, Column: 30},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindBinary,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseInvalidDecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let decimal = 1_234_567_890_
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 17, Line: 2, Column: 16},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 30, Line: 2, Column: 29},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindDecimal,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseInvalidHexadecimalIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let hex = 0x_f2_09
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 13, Line: 2, Column: 12},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 20, Line: 2, Column: 19},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindHexadecimal,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseInvalidHexadecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let hex = 0xf2_09_
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 13, Line: 2, Column: 12},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 20, Line: 2, Column: 19},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindHexadecimal,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)

}

// TODO: remove
func TestParseInvalidIntegerLiteral(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let hex = 0z123
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	assert.Equal(t,
		Position{Offset: 13, Line: 2, Column: 12},
		syntaxError.StartPos,
	)

	assert.Equal(t,
		Position{Offset: 17, Line: 2, Column: 16},
		syntaxError.EndPos,
	)

	assert.Equal(t,
		parser.IntegerLiteralKindUnknown,
		syntaxError.IntegerLiteralKind,
	)

	assert.Equal(t,
		parser.InvalidNumberLiteralKindUnknownPrefix,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

// TODO: remove
func TestParseDecimalIntegerLiteralWithLeadingZeros(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let decimal = 00123
	`)

	require.NoError(t, err)

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(123),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

// TODO: remove
func TestParseBinaryIntegerLiteralWithLeadingZeros(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
		let binary = 0b001000
	`)

	require.NoError(t, err)

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "binary",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(8),
			Base:  2,
			Range: Range{
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 14, Line: 2, Column: 13},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

func TestParseIntegerTypes(t *testing.T) {

	t.Parallel()

	const code = `
		let a: Int8 = 1
		let b: Int16 = 2
		let c: Int32 = 3
		let d: Int64 = 4
		let e: UInt8 = 5
		let f: UInt16 = 6
		let g: UInt32 = 7
		let h: UInt64 = 8
	`

	a := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int8",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
			},
			StartPos: Position{Offset: 10, Line: 2, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(1),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 17, Line: 2, Column: 16},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}
	b := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "b",
			Pos:        Position{Offset: 25, Line: 3, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int16",
					Pos:        Position{Offset: 28, Line: 3, Column: 9},
				},
			},
			StartPos: Position{Offset: 28, Line: 3, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 34, Line: 3, Column: 15},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(2),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 36, Line: 3, Column: 17},
				EndPos:   Position{Offset: 36, Line: 3, Column: 17},
			},
		},
		StartPos: Position{Offset: 21, Line: 3, Column: 2},
	}
	c := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "c",
			Pos:        Position{Offset: 44, Line: 4, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int32",
					Pos:        Position{Offset: 47, Line: 4, Column: 9},
				},
			},
			StartPos: Position{Offset: 47, Line: 4, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 53, Line: 4, Column: 15},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(3),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 55, Line: 4, Column: 17},
				EndPos:   Position{Offset: 55, Line: 4, Column: 17},
			},
		},
		StartPos: Position{Offset: 40, Line: 4, Column: 2},
	}
	d := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "d",
			Pos:        Position{Offset: 63, Line: 5, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int64",
					Pos:        Position{Offset: 66, Line: 5, Column: 9},
				},
			},
			StartPos: Position{Offset: 66, Line: 5, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 72, Line: 5, Column: 15},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(4),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 74, Line: 5, Column: 17},
				EndPos:   Position{Offset: 74, Line: 5, Column: 17},
			},
		},
		StartPos: Position{Offset: 59, Line: 5, Column: 2},
	}
	e := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "e",
			Pos:        Position{Offset: 82, Line: 6, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt8",
					Pos:        Position{Offset: 85, Line: 6, Column: 9},
				},
			},
			StartPos: Position{Offset: 85, Line: 6, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 91, Line: 6, Column: 15},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(5),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 93, Line: 6, Column: 17},
				EndPos:   Position{Offset: 93, Line: 6, Column: 17},
			},
		},
		StartPos: Position{Offset: 78, Line: 6, Column: 2},
	}
	f := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "f",
			Pos:        Position{Offset: 101, Line: 7, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt16",
					Pos:        Position{Offset: 104, Line: 7, Column: 9},
				},
			},
			StartPos: Position{Offset: 104, Line: 7, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 111, Line: 7, Column: 16},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(6),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 113, Line: 7, Column: 18},
				EndPos:   Position{Offset: 113, Line: 7, Column: 18},
			},
		},
		StartPos: Position{Offset: 97, Line: 7, Column: 2},
	}
	g := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "g",
			Pos:        Position{Offset: 121, Line: 8, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt32",
					Pos:        Position{Offset: 124, Line: 8, Column: 9},
				},
			},
			StartPos: Position{Offset: 124, Line: 8, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 131, Line: 8, Column: 16},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(7),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 133, Line: 8, Column: 18},
				EndPos:   Position{Offset: 133, Line: 8, Column: 18},
			},
		},
		StartPos: Position{Offset: 117, Line: 8, Column: 2},
	}
	h := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "h",
			Pos:        Position{Offset: 141, Line: 9, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt64",
					Pos:        Position{Offset: 144, Line: 9, Column: 9},
				},
			},
			StartPos: Position{Offset: 144, Line: 9, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 151, Line: 9, Column: 16},
		},
		Value: &IntegerExpression{
			Value: big.NewInt(8),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 153, Line: 9, Column: 18},
				EndPos:   Position{Offset: 153, Line: 9, Column: 18},
			},
		},
		StartPos: Position{Offset: 137, Line: 9, Column: 2},
	}

	testParse(t, code, []Declaration{a, b, c, d, e, f, g, h}, nil)
}

func TestParseFunctionType(t *testing.T) {

	t.Parallel()

	const code = `
		let add: ((Int8, Int16): Int32) = nothing
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "add",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},
				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int8",
										Pos:        Position{Offset: 14, Line: 2, Column: 13},
									},
								},
								StartPos: Position{Offset: 14, Line: 2, Column: 13},
							},
							{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int16",
										Pos:        Position{Offset: 20, Line: 2, Column: 19},
									},
								},
								StartPos: Position{Offset: 20, Line: 2, Column: 19},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int32",
									Pos:        Position{Offset: 28, Line: 2, Column: 27},
								},
							},
							StartPos: Position{Offset: 28, Line: 2, Column: 27},
						},
						Range: Range{
							StartPos: Position{Offset: 12, Line: 2, Column: 11},
							EndPos:   Position{Offset: 33, Line: 2, Column: 32},
						},
					},
					StartPos: Position{Offset: 12, Line: 2, Column: 11},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 35, Line: 2, Column: 34},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "nothing",
						Pos:        Position{Offset: 37, Line: 2, Column: 36},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseFunctionArrayType(t *testing.T) {

	t.Parallel()

	const code = `
		let test: [((Int8): Int16); 2] = []
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},

				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &ConstantSizedType{
						Type: &FunctionType{
							ParameterTypeAnnotations: []*TypeAnnotation{
								{
									IsResource: false,
									Type: &NominalType{
										Identifier: Identifier{
											Identifier: "Int8",
											Pos:        Position{Offset: 16, Line: 2, Column: 15},
										},
									},
									StartPos: Position{Offset: 16, Line: 2, Column: 15},
								},
							},
							ReturnTypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int16",
										Pos:        Position{Offset: 23, Line: 2, Column: 22},
									},
								},
								StartPos: Position{Offset: 23, Line: 2, Column: 22},
							},
							Range: Range{
								StartPos: Position{Offset: 14, Line: 2, Column: 13},
								EndPos:   Position{Offset: 28, Line: 2, Column: 27},
							},
						},
						Size: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 31, Line: 2, Column: 30},
								EndPos:   Position{Offset: 31, Line: 2, Column: 30},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 32, Line: 2, Column: 31},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 34, Line: 2, Column: 33},
				},
				Value: &ArrayExpression{
					Range: Range{
						StartPos: Position{Offset: 36, Line: 2, Column: 35},
						EndPos:   Position{Offset: 37, Line: 2, Column: 36},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseFunctionTypeWithArrayReturnType(t *testing.T) {

	t.Parallel()

	const code = `
		let test: ((Int8): [Int16; 2]) = nothing
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},
				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int8",
										Pos:        Position{Offset: 15, Line: 2, Column: 14},
									},
								},
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &ConstantSizedType{
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int16",
										Pos:        Position{Offset: 23, Line: 2, Column: 22},
									},
								},
								Size: &IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 30, Line: 2, Column: 29},
										EndPos:   Position{Offset: 30, Line: 2, Column: 29},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 22, Line: 2, Column: 21},
									EndPos:   Position{Offset: 31, Line: 2, Column: 30},
								},
							},
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 32, Line: 2, Column: 31},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 34, Line: 2, Column: 33},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "nothing",
						Pos:        Position{Offset: 36, Line: 2, Column: 35},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseFunctionTypeWithFunctionReturnTypeInParentheses(t *testing.T) {

	t.Parallel()

	const code = `
		let test: ((Int8): ((Int16): Int32)) = nothing
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},
				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int8",
										Pos:        Position{Offset: 15, Line: 2, Column: 14},
									},
								},
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &FunctionType{
								ParameterTypeAnnotations: []*TypeAnnotation{
									{
										IsResource: false,
										Type: &NominalType{
											Identifier: Identifier{
												Identifier: "Int16",
												Pos:        Position{Offset: 24, Line: 2, Column: 23},
											},
										},
										StartPos: Position{Offset: 24, Line: 2, Column: 23},
									},
								},
								ReturnTypeAnnotation: &TypeAnnotation{
									IsResource: false,
									Type: &NominalType{
										Identifier: Identifier{
											Identifier: "Int32",
											Pos:        Position{Offset: 32, Line: 2, Column: 31},
										},
									},
									StartPos: Position{Offset: 32, Line: 2, Column: 31},
								},
								Range: Range{
									StartPos: Position{Offset: 22, Line: 2, Column: 21},
									EndPos:   Position{Offset: 37, Line: 2, Column: 36},
								},
							},
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 38, Line: 2, Column: 37},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 40, Line: 2, Column: 39},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "nothing",
						Pos:        Position{Offset: 42, Line: 2, Column: 41},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseFunctionTypeWithFunctionReturnType(t *testing.T) {

	t.Parallel()

	const code = `
		let test: ((Int8): ((Int16): Int32)) = nothing
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},
				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int8",
										Pos:        Position{Offset: 15, Line: 2, Column: 14},
									},
								},
								StartPos: Position{Offset: 15, Line: 2, Column: 14},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &FunctionType{
								ParameterTypeAnnotations: []*TypeAnnotation{
									{
										IsResource: false,
										Type: &NominalType{
											Identifier: Identifier{
												Identifier: "Int16",
												Pos:        Position{Offset: 24, Line: 2, Column: 23},
											},
										},
										StartPos: Position{Offset: 24, Line: 2, Column: 23},
									},
								},
								ReturnTypeAnnotation: &TypeAnnotation{
									IsResource: false,
									Type: &NominalType{
										Identifier: Identifier{
											Identifier: "Int32",
											Pos:        Position{Offset: 32, Line: 2, Column: 31},
										},
									},
									StartPos: Position{Offset: 32, Line: 2, Column: 31},
								},
								Range: Range{
									StartPos: Position{Offset: 22, Line: 2, Column: 21},
									EndPos:   Position{Offset: 37, Line: 2, Column: 36},
								},
							},
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 38, Line: 2, Column: 37},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 40, Line: 2, Column: 39},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "nothing",
						Pos:        Position{Offset: 42, Line: 2, Column: 41},
					},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseMissingReturnType(t *testing.T) {

	t.Parallel()

	const code = `
		let noop: ((): Void) =
            fun () { return }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				Identifier: Identifier{
					Identifier: "noop",
					Pos:        Position{Offset: 7, Line: 2, Column: 6},
				},

				IsConstant: true,
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Void",
									Pos:        Position{Offset: 18, Line: 2, Column: 17},
								},
							},
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
						},
						Range: Range{
							StartPos: Position{Offset: 13, Line: 2, Column: 12},
							EndPos:   Position{Offset: 22, Line: 2, Column: 21},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 24, Line: 2, Column: 23},
				},
				Value: &FunctionExpression{
					ParameterList: &ParameterList{
						Range: Range{
							StartPos: Position{Offset: 42, Line: 3, Column: 16},
							EndPos:   Position{Offset: 43, Line: 3, Column: 17},
						},
					},
					ReturnTypeAnnotation: &TypeAnnotation{
						IsResource: false,
						Type: &NominalType{
							Identifier: Identifier{
								Pos: Position{Offset: 43, Line: 3, Column: 17},
							},
						},
						StartPos: Position{Offset: 43, Line: 3, Column: 17},
					},
					FunctionBlock: &FunctionBlock{
						Block: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Range: Range{
										StartPos: Position{Offset: 47, Line: 3, Column: 21},
										EndPos:   Position{Offset: 52, Line: 3, Column: 26},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 45, Line: 3, Column: 19},
								EndPos:   Position{Offset: 54, Line: 3, Column: 28},
							},
						},
					},
					StartPos: Position{Offset: 38, Line: 3, Column: 12},
				},
				StartPos: Position{Offset: 3, Line: 2, Column: 2},
			},
		},
		nil,
	)
}

func TestParseLeftAssociativity(t *testing.T) {

	t.Parallel()

	const code = `
        let a = 1 + 2 + 3
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &BinaryExpression{
					Operation: OperationPlus,
					Left: &BinaryExpression{
						Operation: OperationPlus,
						Left: &IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 17, Line: 2, Column: 16},
								EndPos:   Position{Offset: 17, Line: 2, Column: 16},
							},
						},
						Right: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
					},
					Right: &IntegerExpression{
						Value: big.NewInt(3),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 25, Line: 2, Column: 24},
							EndPos:   Position{Offset: 25, Line: 2, Column: 24},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseNegativeInteger(t *testing.T) {

	t.Parallel()

	const code = `
      let a = -42
	`

	testParse(t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 13, Line: 2, Column: 12},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(-42),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseNegativeFixedPoint(t *testing.T) {

	t.Parallel()

	const code = `
      let a = -42.3
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 13, Line: 2, Column: 12},
				},
				Value: &FixedPointExpression{
					Negative:        true,
					UnsignedInteger: big.NewInt(42),
					Fractional:      big.NewInt(3),
					Scale:           1,
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 19, Line: 2, Column: 18},
					},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseInvalidDoubleIntegerUnary(t *testing.T) {

	t.Parallel()

	program, _, err := parser.ParseProgram(`
	   var a = 1
	   let b = --a
	`)

	assert.NotNil(t, program)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	assert.Equal(t,
		[]error{
			&parser.JuxtaposedUnaryOperatorsError{
				Pos: Position{Offset: 27, Line: 3, Column: 12},
			},
		},
		err.(parser.Error).Errors,
	)
}

func TestParseInvalidDoubleBooleanUnary(t *testing.T) {

	t.Parallel()

	program, _, err := parser.ParseProgram(`
	   let b = !!true
	`)

	assert.NotNil(t, program)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	assert.Equal(t,
		[]error{
			&parser.JuxtaposedUnaryOperatorsError{
				Pos: Position{Offset: 13, Line: 2, Column: 12},
			},
		},
		err.(parser.Error).Errors,
	)
}

func TestParseTernaryRightAssociativity(t *testing.T) {

	t.Parallel()

	const code = `
        let a = 2 > 1
          ? 0
          : 3 > 2 ? 1 : 2
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &ConditionalExpression{
					Test: &BinaryExpression{
						Operation: OperationGreater,
						Left: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 17, Line: 2, Column: 16},
								EndPos:   Position{Offset: 17, Line: 2, Column: 16},
							},
						},
						Right: &IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
					},
					Then: &IntegerExpression{
						Value: new(big.Int),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 35, Line: 3, Column: 12},
							EndPos:   Position{Offset: 35, Line: 3, Column: 12},
						},
					},
					Else: &ConditionalExpression{
						Test: &BinaryExpression{
							Operation: OperationGreater,
							Left: &IntegerExpression{
								Value: big.NewInt(3),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 49, Line: 4, Column: 12},
									EndPos:   Position{Offset: 49, Line: 4, Column: 12},
								},
							},
							Right: &IntegerExpression{
								Value: big.NewInt(2),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 53, Line: 4, Column: 16},
									EndPos:   Position{Offset: 53, Line: 4, Column: 16},
								},
							},
						},
						Then: &IntegerExpression{
							Value: big.NewInt(1),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 57, Line: 4, Column: 20},
								EndPos:   Position{Offset: 57, Line: 4, Column: 20},
							},
						},
						Else: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 61, Line: 4, Column: 24},
								EndPos:   Position{Offset: 61, Line: 4, Column: 24},
							},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

// TODO: remove
func TestParseStructure(t *testing.T) {

	t.Parallel()

	const code = `
        struct Test {
            pub(set) var foo: Int

            init(foo: Int) {
                self.foo = foo
            }

            pub fun getFoo(): Int {
                return self.foo
            }
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindStructure,
				Identifier: Identifier{
					Identifier: "Test",
					Pos:        Position{Offset: 16, Line: 2, Column: 15},
				},
				Members: &Members{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessPublicSettable,
							VariableKind: VariableKindVariable,
							Identifier: Identifier{
								Identifier: "foo",
								Pos:        Position{Offset: 48, Line: 3, Column: 25},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 53, Line: 3, Column: 30},
									},
								},
								StartPos: Position{Offset: 53, Line: 3, Column: 30},
							},
							Range: Range{
								StartPos: Position{Offset: 35, Line: 3, Column: 12},
								EndPos:   Position{Offset: 55, Line: 3, Column: 32},
							},
						},
					},
					SpecialFunctions: []*SpecialFunctionDeclaration{
						{
							Kind: common.DeclarationKindInitializer,
							FunctionDeclaration: &FunctionDeclaration{
								Identifier: Identifier{
									Identifier: "init",
									Pos:        Position{Offset: 70, Line: 5, Column: 12},
								},
								ParameterList: &ParameterList{
									Parameters: []*Parameter{
										{
											Label: "",
											Identifier: Identifier{
												Identifier: "foo",
												Pos:        Position{Offset: 75, Line: 5, Column: 17},
											},
											TypeAnnotation: &TypeAnnotation{
												IsResource: false,
												Type: &NominalType{
													Identifier: Identifier{
														Identifier: "Int",
														Pos:        Position{Offset: 80, Line: 5, Column: 22},
													},
												},
												StartPos: Position{Offset: 80, Line: 5, Column: 22},
											},
											Range: Range{
												StartPos: Position{Offset: 75, Line: 5, Column: 17},
												EndPos:   Position{Offset: 82, Line: 5, Column: 24},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 74, Line: 5, Column: 16},
										EndPos:   Position{Offset: 83, Line: 5, Column: 25},
									},
								},
								FunctionBlock: &FunctionBlock{
									Block: &Block{
										Statements: []Statement{
											&AssignmentStatement{
												Target: &MemberExpression{
													Expression: &IdentifierExpression{
														Identifier: Identifier{
															Identifier: "self",
															Pos:        Position{Offset: 103, Line: 6, Column: 16},
														},
													},
													Identifier: Identifier{
														Identifier: "foo",
														Pos:        Position{Offset: 108, Line: 6, Column: 21},
													},
												},
												Transfer: &Transfer{
													Operation: TransferOperationCopy,
													Pos:       Position{Offset: 112, Line: 6, Column: 25},
												},
												Value: &IdentifierExpression{
													Identifier: Identifier{
														Identifier: "foo",
														Pos:        Position{Offset: 114, Line: 6, Column: 27},
													},
												},
											},
										},
										Range: Range{
											StartPos: Position{Offset: 85, Line: 5, Column: 27},
											EndPos:   Position{Offset: 130, Line: 7, Column: 12},
										},
									},
								},
								StartPos: Position{Offset: 70, Line: 5, Column: 12},
							},
						},
					},
					Functions: []*FunctionDeclaration{
						{
							Access: AccessPublic,
							Identifier: Identifier{
								Identifier: "getFoo",
								Pos:        Position{Offset: 153, Line: 9, Column: 20},
							},
							ParameterList: &ParameterList{
								Range: Range{
									StartPos: Position{Offset: 159, Line: 9, Column: 26},
									EndPos:   Position{Offset: 160, Line: 9, Column: 27},
								},
							},
							ReturnTypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 163, Line: 9, Column: 30},
									},
								},
								StartPos: Position{Offset: 163, Line: 9, Column: 30},
							},
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&ReturnStatement{
											Expression: &MemberExpression{
												Expression: &IdentifierExpression{
													Identifier: Identifier{
														Identifier: "self",
														Pos:        Position{Offset: 192, Line: 10, Column: 23},
													},
												},
												Identifier: Identifier{
													Identifier: "foo",
													Pos:        Position{Offset: 197, Line: 10, Column: 28},
												},
											},
											Range: Range{
												StartPos: Position{Offset: 185, Line: 10, Column: 16},
												EndPos:   Position{Offset: 199, Line: 10, Column: 30},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 167, Line: 9, Column: 34},
										EndPos:   Position{Offset: 213, Line: 11, Column: 12},
									},
								},
							},
							StartPos: Position{Offset: 145, Line: 9, Column: 12},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 223, Line: 12, Column: 8},
				},
			},
		},
		nil,
	)
}

func TestParseStructureWithConformances(t *testing.T) {

	t.Parallel()

	const code = `
        struct Test: Foo, Bar {}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindStructure,
				Identifier: Identifier{
					Identifier: "Test",
					Pos:        Position{Offset: 16, Line: 2, Column: 15},
				},
				Conformances: []*NominalType{
					{
						Identifier: Identifier{
							Identifier: "Foo",
							Pos:        Position{Offset: 22, Line: 2, Column: 21},
						},
					},
					{
						Identifier: Identifier{
							Identifier: "Bar",
							Pos:        Position{Offset: 27, Line: 2, Column: 26},
						},
					},
				},
				Members: &Members{},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 32, Line: 2, Column: 31},
				},
			},
		},
		nil,
	)
}

func TestParsePreAndPostConditions(t *testing.T) {

	t.Parallel()

	const code = `
        fun test(n: Int) {
            pre {
                n != 0
                n > 0
            }
            post {
                result == 0
            }
            return 0
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Label: "",
							Identifier: Identifier{
								Identifier: "n",
								Pos:        Position{Offset: 18, Line: 2, Column: 17},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 21, Line: 2, Column: 20},
									},
								},
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
							},
							Range: Range{
								StartPos: Position{Offset: 18, Line: 2, Column: 17},
								EndPos:   Position{Offset: 23, Line: 2, Column: 22},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 24, Line: 2, Column: 23},
						},
					},
					StartPos: Position{Offset: 24, Line: 2, Column: 23},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 185, Line: 10, Column: 19},
										EndPos:   Position{Offset: 185, Line: 10, Column: 19},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 178, Line: 10, Column: 12},
									EndPos:   Position{Offset: 185, Line: 10, Column: 19},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 195, Line: 11, Column: 8},
						},
					},
					PreConditions: &Conditions{
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationNotEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "n",
										Pos:        Position{Offset: 62, Line: 4, Column: 16},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 67, Line: 4, Column: 21},
										EndPos:   Position{Offset: 67, Line: 4, Column: 21},
									},
								},
							},
						},
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationGreater,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "n",
										Pos:        Position{Offset: 85, Line: 5, Column: 16},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 89, Line: 5, Column: 20},
										EndPos:   Position{Offset: 89, Line: 5, Column: 20},
									},
								},
							},
						},
					},
					PostConditions: &Conditions{
						{
							Kind: ConditionKindPost,
							Test: &BinaryExpression{
								Operation: OperationEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "result",
										Pos:        Position{Offset: 140, Line: 8, Column: 16},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 150, Line: 8, Column: 26},
										EndPos:   Position{Offset: 150, Line: 8, Column: 26},
									},
								},
							},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseExpression(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseExpression(`
        before(x + before(y)) + z
	`)

	require.NoError(t, err)

	expected := &BinaryExpression{
		Operation: OperationPlus,
		Left: &InvocationExpression{
			InvokedExpression: &IdentifierExpression{
				Identifier: Identifier{
					Identifier: "before",
					Pos:        Position{Offset: 9, Line: 2, Column: 8},
				},
			},
			Arguments: []*Argument{
				{
					Label:         "",
					LabelStartPos: nil,
					LabelEndPos:   nil,
					Expression: &BinaryExpression{
						Operation: OperationPlus,
						Left: &IdentifierExpression{
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 16, Line: 2, Column: 15},
							},
						},
						Right: &InvocationExpression{
							InvokedExpression: &IdentifierExpression{
								Identifier: Identifier{
									Identifier: "before",
									Pos:        Position{Offset: 20, Line: 2, Column: 19},
								},
							},
							Arguments: []*Argument{
								{
									Label:         "",
									LabelStartPos: nil,
									LabelEndPos:   nil,
									Expression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "y",
											Pos:        Position{Offset: 27, Line: 2, Column: 26},
										},
									},
								},
							},
							EndPos: Position{Offset: 28, Line: 2, Column: 27},
						},
					},
				},
			},
			EndPos: Position{Offset: 29, Line: 2, Column: 28},
		},
		Right: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "z",
				Pos:        Position{Offset: 33, Line: 2, Column: 32},
			},
		},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

func TestParseString(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseExpression(`
       "test \0\n\r\t\"\'\\ xyz"
	`)

	require.NoError(t, err)

	expected := &StringExpression{
		Value: "test \x00\n\r\t\"'\\ xyz",
		Range: Range{
			StartPos: Position{Offset: 8, Line: 2, Column: 7},
			EndPos:   Position{Offset: 32, Line: 2, Column: 31},
		},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

func TestParseStringWithUnicode(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseExpression(`
      "this is a test \t\\new line and race car:\n\u{1F3CE}\u{FE0F}"
	`)

	require.NoError(t, err)

	expected := &StringExpression{
		Value: "this is a test \t\\new line and race car:\n\U0001F3CE\uFE0F",
		Range: Range{
			StartPos: Position{Offset: 7, Line: 2, Column: 6},
			EndPos:   Position{Offset: 68, Line: 2, Column: 67},
		},
	}

	utils.AssertEqualWithDiff(t, expected, actual)
}

func TestParseConditionMessage(t *testing.T) {

	t.Parallel()

	const code = `
        fun test(n: Int) {
            pre {
                n >= 0: "n must be positive"
            }
            return n
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Label: "",
							Identifier: Identifier{Identifier: "n",
								Pos: Position{Offset: 18, Line: 2, Column: 17},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 21, Line: 2, Column: 20},
									},
								},
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
							},
							Range: Range{
								StartPos: Position{Offset: 18, Line: 2, Column: 17},
								EndPos:   Position{Offset: 23, Line: 2, Column: 22},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 24, Line: 2, Column: 23},
						},
					},
					StartPos: Position{Offset: 24, Line: 2, Column: 23},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "n",
										Pos:        Position{Offset: 124, Line: 6, Column: 19},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 117, Line: 6, Column: 12},
									EndPos:   Position{Offset: 124, Line: 6, Column: 19},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 134, Line: 7, Column: 8},
						},
					},
					PreConditions: &Conditions{
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationGreaterEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "n",
										Pos:        Position{Offset: 62, Line: 4, Column: 16},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 67, Line: 4, Column: 21},
										EndPos:   Position{Offset: 67, Line: 4, Column: 21},
									},
								},
							},
							Message: &StringExpression{
								Value: "n must be positive",
								Range: Range{
									StartPos: Position{Offset: 70, Line: 4, Column: 24},
									EndPos:   Position{Offset: 89, Line: 4, Column: 43},
								},
							},
						},
					},
					PostConditions: nil,
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseOptionalType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: Int?? = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &OptionalType{
						Type: &OptionalType{
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 15, Line: 2, Column: 14},
								},
							},
							EndPos: Position{Offset: 18, Line: 2, Column: 17},
						},
						EndPos: Position{Offset: 19, Line: 2, Column: 18},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 21, Line: 2, Column: 20},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseNilCoalescing(t *testing.T) {

	t.Parallel()

	const code = `
       let x = nil ?? 1
	`

	testParse(t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 14, Line: 2, Column: 13},
				},
				Value: &BinaryExpression{
					Operation: OperationNilCoalesce,
					Left: &NilExpression{
						Pos: Position{Offset: 16, Line: 2, Column: 15},
					},
					Right: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 23, Line: 2, Column: 22},
							EndPos:   Position{Offset: 23, Line: 2, Column: 22},
						},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseNilCoalescingRightAssociativity(t *testing.T) {

	t.Parallel()

	// NOTE: only syntactically, not semantically valid
	const code = `
       let x = 1 ?? 2 ?? 3
	`

	testParse(t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 14, Line: 2, Column: 13},
				},
				Value: &BinaryExpression{
					Operation: OperationNilCoalesce,
					Left: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					Right: &BinaryExpression{
						Operation: OperationNilCoalesce,
						Left: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
						Right: &IntegerExpression{
							Value: big.NewInt(3),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 26, Line: 2, Column: 25},
								EndPos:   Position{Offset: 26, Line: 2, Column: 25},
							},
						},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseFailableCasting(t *testing.T) {

	t.Parallel()

	const code = `
       let x = 0 as? Int
	`

	failableDowncast := &CastingExpression{
		Expression: &IntegerExpression{
			Value: new(big.Int),
			Base:  10,
			Range: Range{
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 16, Line: 2, Column: 15},
			},
		},
		Operation: OperationFailableCast,
		TypeAnnotation: &TypeAnnotation{
			IsResource: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int",
					Pos:        Position{Offset: 22, Line: 2, Column: 21},
				},
			},
			StartPos: Position{Offset: 22, Line: 2, Column: 21},
		},
	}

	variableDeclaration := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "x",
			Pos:        Position{Offset: 12, Line: 2, Column: 11},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 14, Line: 2, Column: 13},
		},
		Value:    failableDowncast,
		StartPos: Position{Offset: 8, Line: 2, Column: 7},
	}

	failableDowncast.ParentVariableDeclaration = variableDeclaration

	testParse(
		t,
		code,
		[]Declaration{
			variableDeclaration,
		},
		nil,
	)
}

func TestParseInterface(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		actual, _, err := parser.ParseProgram(fmt.Sprintf(`
            %s interface Test {
                foo: Int

                init(foo: Int)

                fun getFoo(): Int
            }
	    `, kind.Keyword()))

		require.NoError(t, err)

		// only compare AST for one kind: structs

		if kind != common.CompositeKindStructure {
			continue
		}

		test := &InterfaceDeclaration{
			CompositeKind: common.CompositeKindStructure,
			Identifier: Identifier{
				Identifier: "Test",
				Pos:        Position{Offset: 30, Line: 2, Column: 29},
			},
			Members: &Members{
				Fields: []*FieldDeclaration{
					{
						Access:       AccessNotSpecified,
						VariableKind: VariableKindNotSpecified,
						Identifier: Identifier{
							Identifier: "foo",
							Pos:        Position{Offset: 53, Line: 3, Column: 16},
						},
						TypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 58, Line: 3, Column: 21},
								},
							},
							StartPos: Position{Offset: 58, Line: 3, Column: 21},
						},
						Range: Range{
							StartPos: Position{Offset: 53, Line: 3, Column: 16},
							EndPos:   Position{Offset: 60, Line: 3, Column: 23},
						},
					},
				},
				SpecialFunctions: []*SpecialFunctionDeclaration{
					{
						Kind: common.DeclarationKindInitializer,
						FunctionDeclaration: &FunctionDeclaration{
							Identifier: Identifier{
								Identifier: "init",
								Pos:        Position{Offset: 79, Line: 5, Column: 16},
							},
							ParameterList: &ParameterList{
								Parameters: []*Parameter{
									{
										Label: "",
										Identifier: Identifier{
											Identifier: "foo",
											Pos:        Position{Offset: 84, Line: 5, Column: 21},
										},
										TypeAnnotation: &TypeAnnotation{
											IsResource: false,
											Type: &NominalType{
												Identifier: Identifier{
													Identifier: "Int",
													Pos:        Position{Offset: 89, Line: 5, Column: 26},
												},
											},
											StartPos: Position{Offset: 89, Line: 5, Column: 26},
										},
										Range: Range{
											StartPos: Position{Offset: 84, Line: 5, Column: 21},
											EndPos:   Position{Offset: 91, Line: 5, Column: 28},
										},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 83, Line: 5, Column: 20},
									EndPos:   Position{Offset: 92, Line: 5, Column: 29},
								},
							},
							FunctionBlock: nil,
							StartPos:      Position{Offset: 79, Line: 5, Column: 16},
						},
					},
				},
				Functions: []*FunctionDeclaration{
					{
						Access: AccessNotSpecified,
						Identifier: Identifier{
							Identifier: "getFoo",
							Pos:        Position{Offset: 115, Line: 7, Column: 20},
						},
						ParameterList: &ParameterList{
							Range: Range{
								StartPos: Position{Offset: 121, Line: 7, Column: 26},
								EndPos:   Position{Offset: 122, Line: 7, Column: 27},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 125, Line: 7, Column: 30},
								},
							},
							StartPos: Position{Offset: 125, Line: 7, Column: 30},
						},
						FunctionBlock: nil,
						StartPos:      Position{Offset: 111, Line: 7, Column: 16},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 141, Line: 8, Column: 12},
			},
		}

		expected := &Program{
			Declarations: []Declaration{test},
		}

		utils.AssertEqualWithDiff(t, expected, actual)
	}
}

func TestParseImportWithString(t *testing.T) {

	t.Parallel()

	const code = `
        import "test.bpl"
	`

	testParse(
		t,
		code,
		[]Declaration{
			&ImportDeclaration{
				Identifiers: nil,
				Location:    StringLocation("test.bpl"),
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 25, Line: 2, Column: 24},
				},
				LocationPos: Position{Offset: 16, Line: 2, Column: 15},
			},
		},
		func(actual *Program) {

			importLocation := StringLocation("test.bpl")

			actualImports := actual.ImportedPrograms()

			assert.Equal(t,
				map[LocationID]*Program{},
				actualImports,
			)

			actualImports[importLocation.ID()] = &Program{}

			assert.Equal(t,
				map[LocationID]*Program{
					importLocation.ID(): {},
				},
				actualImports,
			)
		},
	)
}

func TestParseImportWithAddress(t *testing.T) {

	t.Parallel()

	const code = `
        import 0x1234
	`

	testParse(
		t,
		code,
		[]Declaration{
			&ImportDeclaration{
				Identifiers: nil,
				Location:    AddressLocation{18, 52},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
				LocationPos: Position{Offset: 16, Line: 2, Column: 15},
			},
		},
		func(actual *Program) {

			importLocation := AddressLocation{18, 52}

			actualImports := actual.ImportedPrograms()

			assert.Equal(t,
				map[LocationID]*Program{},
				actualImports,
			)

			actualImports[importLocation.ID()] = &Program{}

			assert.Equal(t,
				map[LocationID]*Program{
					importLocation.ID(): {},
				},
				actualImports,
			)
		},
	)
}

func TestParseImportWithIdentifiers(t *testing.T) {

	t.Parallel()

	const code = `
        import A, b from 0x0
	`

	testParse(
		t,
		code,
		[]Declaration{
			&ImportDeclaration{
				Identifiers: []Identifier{
					{
						Identifier: "A",
						Pos:        Position{Offset: 16, Line: 2, Column: 15},
					},
					{
						Identifier: "b",
						Pos:        Position{Offset: 19, Line: 2, Column: 18},
					},
				},
				Location: AddressLocation{0},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 28, Line: 2, Column: 27},
				},
				LocationPos: Position{Offset: 26, Line: 2, Column: 25},
			},
		},
		nil,
	)
}

func TestParseFieldWithFromIdentifier(t *testing.T) {

	t.Parallel()

	const code = `
      struct S {
          let from: String
      }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				Access:        AccessNotSpecified,
				CompositeKind: common.CompositeKindStructure,
				Identifier: Identifier{
					Identifier: "S",
					Pos:        Position{Offset: 14, Line: 2, Column: 13},
				},
				Members: &Members{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessNotSpecified,
							VariableKind: VariableKindConstant,
							Identifier: Identifier{
								Identifier: "from",
								Pos:        Position{Offset: 32, Line: 3, Column: 14},
							},
							TypeAnnotation: &TypeAnnotation{
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "String",
										Pos:        Position{Offset: 38, Line: 3, Column: 20},
									},
								},
								StartPos: Position{Offset: 38, Line: 3, Column: 20},
							},
							Range: Range{
								StartPos: Position{Offset: 28, Line: 3, Column: 10},
								EndPos:   Position{Offset: 43, Line: 3, Column: 25},
							},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 7, Line: 2, Column: 6},
					EndPos:   Position{Offset: 51, Line: 4, Column: 6},
				},
			},
		},
		nil,
	)
}

func TestParseFunctionWithFromIdentifier(t *testing.T) {

	t.Parallel()

	const code = `
        fun send(from: String, to: String) {}
	`

	testParse(t, code, nil, nil)
}

func TestParseImportWithFromIdentifier(t *testing.T) {

	t.Parallel()

	const code = `
        import from from 0x0
	`

	testParse(t,
		code,
		[]Declaration{
			&ImportDeclaration{
				Identifiers: []Identifier{
					{
						Identifier: "from",
						Pos:        Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				Location: AddressLocation{0},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 28, Line: 2, Column: 27},
				},
				LocationPos: Position{Offset: 26, Line: 2, Column: 25},
			},
		},
		nil,
	)
}

func TestParseSemicolonsBetweenDeclarations(t *testing.T) {

	t.Parallel()

	const code = `
        import from from 0x0;
        fun foo() {};
	`

	testParse(t, code, nil, nil)
}

// TODO: remove
func TestParseInvalidMultipleSemicolonsBetweenDeclarations(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
        let x = 1;;let y = 2
	`)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 19, Line: 2, Column: 18},
		syntaxError.Pos,
	)

	assert.Contains(t, syntaxError.Message, "extraneous input ';'")
}

// TODO: remove
func TestParseInvalidTypeWithWhitespace(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
	    let x: Int ? = 1
	`)

	assert.Nil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 17, Line: 2, Column: 16},
		syntaxError.Pos,
	)

	assert.Contains(t, syntaxError.Message, "no viable alternative")
}

func TestParseResource(t *testing.T) {

	t.Parallel()

	const code = `
        resource Test {}
	`
	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindResource,
				Identifier: Identifier{
					Identifier: "Test",
					Pos:        Position{Offset: 18, Line: 2, Column: 17},
				},
				Members: &Members{},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 24, Line: 2, Column: 23},
				},
			},
		},
		nil,
	)
}

func TestParseEvent(t *testing.T) {

	t.Parallel()

	const code = `
        event Transfer(to: Address, from: Address)
	`
	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindEvent,
				Identifier: Identifier{
					Identifier: "Transfer",
					Pos:        Position{Offset: 15, Line: 2, Column: 14},
				},
				Members: &Members{
					SpecialFunctions: []*SpecialFunctionDeclaration{
						{
							Kind: common.DeclarationKindInitializer,
							FunctionDeclaration: &FunctionDeclaration{
								ParameterList: &ParameterList{
									Parameters: []*Parameter{
										{
											Label: "",
											Identifier: Identifier{
												Identifier: "to",
												Pos:        Position{Offset: 24, Line: 2, Column: 23},
											},
											TypeAnnotation: &TypeAnnotation{
												IsResource: false,
												Type: &NominalType{
													Identifier: Identifier{
														Identifier: "Address",
														Pos:        Position{Offset: 28, Line: 2, Column: 27},
													},
												},
												StartPos: Position{Offset: 28, Line: 2, Column: 27},
											},
											Range: Range{
												StartPos: Position{Offset: 24, Line: 2, Column: 23},
												EndPos:   Position{Offset: 34, Line: 2, Column: 33},
											},
										},
										{
											Label: "",
											Identifier: Identifier{
												Identifier: "from",
												Pos:        Position{Offset: 37, Line: 2, Column: 36},
											},
											TypeAnnotation: &TypeAnnotation{
												IsResource: false,
												Type: &NominalType{
													Identifier: Identifier{
														Identifier: "Address",
														Pos:        Position{Offset: 43, Line: 2, Column: 42},
													},
												},
												StartPos: Position{Offset: 43, Line: 2, Column: 42},
											},
											Range: Range{
												StartPos: Position{Offset: 37, Line: 2, Column: 36},
												EndPos:   Position{Offset: 49, Line: 2, Column: 48},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 23, Line: 2, Column: 22},
										EndPos:   Position{Offset: 50, Line: 2, Column: 49},
									},
								},
								StartPos: Position{Offset: 23, Line: 2, Column: 22},
							},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 50, Line: 2, Column: 49},
				},
			},
		},
		nil,
	)
}

func TestParseEventEmitStatement(t *testing.T) {

	t.Parallel()

	const code = `
      fun test() {
        emit Transfer(to: 1, from: 2)
      }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&EmitStatement{
								InvocationExpression: &InvocationExpression{
									InvokedExpression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "Transfer",
											Pos:        Position{Offset: 33, Line: 3, Column: 13},
										},
									},
									Arguments: Arguments{
										{
											Label:         "to",
											LabelStartPos: &Position{Offset: 42, Line: 3, Column: 22},
											LabelEndPos:   &Position{Offset: 43, Line: 3, Column: 23},
											Expression: &IntegerExpression{
												Value: big.NewInt(1),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 46, Line: 3, Column: 26},
													EndPos:   Position{Offset: 46, Line: 3, Column: 26},
												},
											},
										},
										{
											Label:         "from",
											LabelStartPos: &Position{Offset: 49, Line: 3, Column: 29},
											LabelEndPos:   &Position{Offset: 52, Line: 3, Column: 32},
											Expression: &IntegerExpression{
												Value: big.NewInt(2),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 55, Line: 3, Column: 35},
													EndPos:   Position{Offset: 55, Line: 3, Column: 35},
												},
											},
										},
									},
									EndPos: Position{Offset: 56, Line: 3, Column: 36},
								},
								StartPos: Position{Offset: 28, Line: 3, Column: 8},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
							EndPos:   Position{Offset: 64, Line: 4, Column: 6},
						},
					},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseResourceReturnType(t *testing.T) {

	t.Parallel()

	const code = `
        fun test(): @X {}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 18, Line: 2, Column: 17},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: true,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "X",
							Pos:        Position{Offset: 22, Line: 2, Column: 21},
						},
					},
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 24, Line: 2, Column: 23},
							EndPos:   Position{Offset: 25, Line: 2, Column: 24},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseMovingVariableDeclaration(t *testing.T) {

	t.Parallel()

	const code = `
        let x <- y
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "y",
						Pos:        Position{Offset: 18, Line: 2, Column: 17},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationMove,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseMoveStatement(t *testing.T) {

	t.Parallel()

	const code = `
        fun test() {
            x <- y
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 18, Line: 2, Column: 17},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 18, Line: 2, Column: 17},
						},
					},
					StartPos: Position{Offset: 18, Line: 2, Column: 17},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&AssignmentStatement{
								Target: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "x",
										Pos:        Position{Offset: 34, Line: 3, Column: 12},
									},
								},
								Transfer: &Transfer{
									Operation: TransferOperationMove,
									Pos:       Position{Offset: 36, Line: 3, Column: 14},
								},
								Value: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "y",
										Pos:        Position{Offset: 39, Line: 3, Column: 17},
									},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 20, Line: 2, Column: 19},
							EndPos:   Position{Offset: 49, Line: 4, Column: 8},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseMoveOperator(t *testing.T) {

	t.Parallel()

	const code = `
      let x = foo(<-y)
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 13, Line: 2, Column: 12},
				},
				Value: &InvocationExpression{
					InvokedExpression: &IdentifierExpression{
						Identifier: Identifier{
							Identifier: "foo",
							Pos:        Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					Arguments: []*Argument{
						{
							Label:         "",
							LabelStartPos: nil,
							LabelEndPos:   nil,
							Expression: &UnaryExpression{
								Operation: OperationMove,
								Expression: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "y",
										Pos:        Position{Offset: 21, Line: 2, Column: 20},
									},
								},
								StartPos: Position{Offset: 19, Line: 2, Column: 18},
							},
						},
					},
					EndPos: Position{Offset: 22, Line: 2, Column: 21},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseResourceParameterType(t *testing.T) {

	t.Parallel()

	const code = `
        fun test(x: @X) {}
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 23, Line: 2, Column: 22},
						},
					},
					StartPos: Position{Offset: 23, Line: 2, Column: 22},
				},
				ParameterList: &ParameterList{
					Parameters: []*Parameter{
						{
							Label: "",
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 18, Line: 2, Column: 17},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: true,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "X",
										Pos:        Position{Offset: 22, Line: 2, Column: 21},
									},
								},
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
							},
							Range: Range{
								StartPos: Position{Offset: 18, Line: 2, Column: 17},
								EndPos:   Position{Offset: 22, Line: 2, Column: 21},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 25, Line: 2, Column: 24},
							EndPos:   Position{Offset: 26, Line: 2, Column: 25},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseMovingVariableDeclarationWithTypeAnnotation(t *testing.T) {

	t.Parallel()

	const code = `
        let x: @R <- y
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: true,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "R",
							Pos:        Position{Offset: 17, Line: 2, Column: 16},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "y",
						Pos:        Position{Offset: 22, Line: 2, Column: 21},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationMove,
					Pos:       Position{Offset: 19, Line: 2, Column: 18},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseFieldDeclarationWithMoveTypeAnnotation(t *testing.T) {

	t.Parallel()

	const code = `
        struct X { x: @R }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindStructure,
				Identifier: Identifier{
					Identifier: "X",
					Pos:        Position{Offset: 16, Line: 2, Column: 15},
				},
				Members: &Members{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessNotSpecified,
							VariableKind: VariableKindNotSpecified,
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 20, Line: 2, Column: 19},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: true,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "R",
										Pos:        Position{Offset: 24, Line: 2, Column: 23},
									},
								},
								StartPos: Position{Offset: 23, Line: 2, Column: 22},
							},
							Range: Range{
								StartPos: Position{Offset: 20, Line: 2, Column: 19},
								EndPos:   Position{Offset: 24, Line: 2, Column: 23},
							},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 26, Line: 2, Column: 25},
				},
			},
		},
		nil,
	)
}

func TestParseFunctionTypeWithResourceTypeAnnotation(t *testing.T) {

	t.Parallel()

	const code = `
        let f: ((): @R) = g
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "f",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: nil,
						ReturnTypeAnnotation: &TypeAnnotation{
							IsResource: true,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "R",
									Pos:        Position{Offset: 22, Line: 2, Column: 21},
								},
							},
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
						},
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 23, Line: 2, Column: 22},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 25, Line: 2, Column: 24},
				},
				Value: &IdentifierExpression{
					Identifier: Identifier{
						Identifier: "g",
						Pos:        Position{Offset: 27, Line: 2, Column: 26},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseFunctionExpressionWithResourceTypeAnnotation(t *testing.T) {

	t.Parallel()

	const code = `
        let f = fun (): @R { return X }
	`

	testParse(
		t,
		code,
		[]Declaration{

			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "f",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &FunctionExpression{
					ParameterList: &ParameterList{
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 22, Line: 2, Column: 21},
						},
					},
					ReturnTypeAnnotation: &TypeAnnotation{
						IsResource: true,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "R",
								Pos:        Position{Offset: 26, Line: 2, Column: 25},
							},
						},
						StartPos: Position{Offset: 25, Line: 2, Column: 24},
					},
					FunctionBlock: &FunctionBlock{
						Block: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Expression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "X",
											Pos:        Position{Offset: 37, Line: 2, Column: 36},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 30, Line: 2, Column: 29},
										EndPos:   Position{Offset: 37, Line: 2, Column: 36},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 28, Line: 2, Column: 27},
								EndPos:   Position{Offset: 39, Line: 2, Column: 38},
							},
						},
					},
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
		nil,
	)
}

func TestParseFailableCastingResourceTypeAnnotation(t *testing.T) {

	t.Parallel()

	const code = `
        let y = x as? @R
	`

	failableDowncast := &CastingExpression{
		Expression: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "x",
				Pos:        Position{Offset: 17, Line: 2, Column: 16},
			},
		},
		Operation: OperationFailableCast,
		TypeAnnotation: &TypeAnnotation{
			IsResource: true,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "R",
					Pos:        Position{Offset: 24, Line: 2, Column: 23},
				},
			},
			StartPos: Position{Offset: 23, Line: 2, Column: 22},
		},
	}

	variableDeclaration := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "y",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		Value:    failableDowncast,
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	failableDowncast.ParentVariableDeclaration = variableDeclaration

	testParse(
		t,
		code,
		[]Declaration{
			variableDeclaration,
		},
		nil,
	)
}

func TestParseCasting(t *testing.T) {

	t.Parallel()

	const code = `
        let y = x as Y
	`

	cast := &CastingExpression{
		Expression: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "x",
				Pos:        Position{Offset: 17, Line: 2, Column: 16},
			},
		},
		Operation: OperationCast,
		TypeAnnotation: &TypeAnnotation{
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Y",
					Pos:        Position{Offset: 22, Line: 2, Column: 21},
				},
			},
			StartPos: Position{Offset: 22, Line: 2, Column: 21},
		},
	}

	variableDeclaration := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "y",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		Value:    cast,
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	cast.ParentVariableDeclaration = variableDeclaration

	testParse(
		t,
		code,
		[]Declaration{
			variableDeclaration,
		},
		nil,
	)
}

func TestParseFunctionExpressionStatementAfterVariableDeclarationWithCreateExpression(t *testing.T) {

	t.Parallel()

	const code = `
      fun test() {
          let r <- create R()
          (fun () {})()
      }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&VariableDeclaration{
								IsConstant: true,
								Identifier: Identifier{
									Identifier: "r",
									Pos:        Position{Offset: 34, Line: 3, Column: 14},
								},
								TypeAnnotation: nil,
								Value: &CreateExpression{
									InvocationExpression: &InvocationExpression{
										InvokedExpression: &IdentifierExpression{
											Identifier: Identifier{
												Identifier: "R",
												Pos:        Position{Offset: 46, Line: 3, Column: 26},
											},
										},
										Arguments: nil,
										EndPos:    Position{Offset: 48, Line: 3, Column: 28},
									},
									StartPos: Position{Offset: 39, Line: 3, Column: 19},
								},
								Transfer: &Transfer{
									Operation: TransferOperationMove,
									Pos:       Position{Offset: 36, Line: 3, Column: 16},
								},
								StartPos: Position{Offset: 30, Line: 3, Column: 10},
							},
							&ExpressionStatement{
								Expression: &InvocationExpression{
									InvokedExpression: &FunctionExpression{
										ParameterList: &ParameterList{
											Range: Range{
												StartPos: Position{Offset: 65, Line: 4, Column: 15},
												EndPos:   Position{Offset: 66, Line: 4, Column: 16},
											},
										},
										ReturnTypeAnnotation: &TypeAnnotation{
											IsResource: false,
											Type: &NominalType{
												Identifier: Identifier{
													Identifier: "",
													Pos:        Position{Offset: 66, Line: 4, Column: 16},
												},
											},
											StartPos: Position{Offset: 66, Line: 4, Column: 16},
										},
										FunctionBlock: &FunctionBlock{
											Block: &Block{
												Statements: nil,
												Range: Range{
													StartPos: Position{Offset: 68, Line: 4, Column: 18},
													EndPos:   Position{Offset: 69, Line: 4, Column: 19},
												},
											},
											PreConditions:  nil,
											PostConditions: nil,
										},
										StartPos: Position{Offset: 61, Line: 4, Column: 11},
									},
									Arguments: nil,
									EndPos:    Position{Offset: 72, Line: 4, Column: 22},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
							EndPos:   Position{Offset: 80, Line: 5, Column: 6},
						},
					},
					PreConditions:  nil,
					PostConditions: nil,
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseIdentifiers(t *testing.T) {

	t.Parallel()

	for _, name := range []string{"foo", "from", "create", "destroy"} {
		t.Run(name, func(t *testing.T) {
			code := fmt.Sprintf(`let %s = 1`, name)
			testParse(t, code, nil, nil)
		})
	}
}

// TestParseExpressionStatementAfterReturnStatement tests that a return statement
// does *not* consume an expression from the next statement as the return value
//
func TestParseExpressionStatementAfterReturnStatement(t *testing.T) {

	t.Parallel()

	const code = `
      fun test() {
          return
          destroy x
      }
	`
	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: nil,
								Range: Range{
									StartPos: Position{Offset: 30, Line: 3, Column: 10},
									EndPos:   Position{Offset: 35, Line: 3, Column: 15},
								},
							},
							&ExpressionStatement{
								Expression: &DestroyExpression{
									Expression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "x",
											Pos:        Position{Offset: 55, Line: 4, Column: 18},
										},
									},
									StartPos: Position{Offset: 47, Line: 4, Column: 10},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
							EndPos:   Position{Offset: 63, Line: 5, Column: 6},
						},
					},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseSwapStatement(t *testing.T) {

	t.Parallel()

	const code = `
      fun test() {
          foo[0] <-> bar.baz
      }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&SwapStatement{
								Left: &IndexExpression{
									TargetExpression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "foo",
											Pos:        Position{Offset: 30, Line: 3, Column: 10},
										},
									},
									IndexingExpression: &IntegerExpression{
										Value: new(big.Int),
										Base:  10,
										Range: Range{
											StartPos: Position{Offset: 34, Line: 3, Column: 14},
											EndPos:   Position{Offset: 34, Line: 3, Column: 14},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 33, Line: 3, Column: 13},
										EndPos:   Position{Offset: 35, Line: 3, Column: 15},
									},
								},
								Right: &MemberExpression{
									Expression: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "bar",
											Pos:        Position{Offset: 41, Line: 3, Column: 21},
										},
									},
									Identifier: Identifier{
										Identifier: "baz",
										Pos:        Position{Offset: 45, Line: 3, Column: 25},
									},
								},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
							EndPos:   Position{Offset: 55, Line: 4, Column: 6},
						},
					},
					PreConditions:  nil,
					PostConditions: nil,
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

func TestParseDestructor(t *testing.T) {

	t.Parallel()

	const code = `
        resource Test {
            destroy() {}
        }
	`

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindResource,
				Identifier: Identifier{
					Identifier: "Test",
					Pos:        Position{Offset: 18, Line: 2, Column: 17},
				},
				Members: &Members{
					SpecialFunctions: []*SpecialFunctionDeclaration{
						{
							Kind: common.DeclarationKindDestructor,
							FunctionDeclaration: &FunctionDeclaration{
								Identifier: Identifier{
									Identifier: "destroy",
									Pos:        Position{Offset: 37, Line: 3, Column: 12},
								},
								ParameterList: &ParameterList{
									Range: Range{
										StartPos: Position{Offset: 44, Line: 3, Column: 19},
										EndPos:   Position{Offset: 45, Line: 3, Column: 20},
									},
								},
								FunctionBlock: &FunctionBlock{
									Block: &Block{
										Range: Range{
											StartPos: Position{Offset: 47, Line: 3, Column: 22},
											EndPos:   Position{Offset: 48, Line: 3, Column: 23},
										},
									},
								},
								StartPos: Position{Offset: 37, Line: 3, Column: 12},
							},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 58, Line: 4, Column: 8},
				},
			},
		},
		nil,
	)
}

func TestParseReferenceType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: &[&R] = 1
	`
	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &ReferenceType{
						Type: &VariableSizedType{
							Type: &ReferenceType{
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "R",
										Pos:        Position{Offset: 18, Line: 2, Column: 17},
									},
								},
								StartPos: Position{Offset: 17, Line: 2, Column: 16},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 19, Line: 2, Column: 18},
							},
						},
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 21, Line: 2, Column: 20},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseOptionalReference(t *testing.T) {

	t.Parallel()

	const code = `
       let x: &R? = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &OptionalType{
						Type: &ReferenceType{
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "R",
									Pos:        Position{Offset: 16, Line: 2, Column: 15},
								},
							},
							StartPos: Position{Offset: 15, Line: 2, Column: 14},
						},
						EndPos: Position{Offset: 17, Line: 2, Column: 16},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 19, Line: 2, Column: 18},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 21, Line: 2, Column: 20},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseRestrictedReferenceTypeWithBaseType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: &R{I} = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &ReferenceType{
						Type: &RestrictedType{
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "R",
									Pos:        Position{Offset: 16, Line: 2, Column: 15},
								},
							},
							Restrictions: []*NominalType{
								{
									Identifier: Identifier{
										Identifier: "I",
										Pos:        Position{Offset: 18, Line: 2, Column: 17},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 19, Line: 2, Column: 18},
							},
						},
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 21, Line: 2, Column: 20},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseRestrictedReferenceTypeWithoutBaseType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: &{I} = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &ReferenceType{
						Type: &RestrictedType{
							Restrictions: []*NominalType{
								{
									Identifier: Identifier{
										Identifier: "I",
										Pos:        Position{Offset: 17, Line: 2, Column: 16},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 18, Line: 2, Column: 17},
							},
						},
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 22, Line: 2, Column: 21},
						EndPos:   Position{Offset: 22, Line: 2, Column: 21},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 20, Line: 2, Column: 19},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseOptionalRestrictedType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: @R{I}? = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: true,
					Type: &OptionalType{
						Type: &RestrictedType{
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "R",
									Pos:        Position{Offset: 16, Line: 2, Column: 15},
								},
							},
							Restrictions: []*NominalType{
								{
									Identifier: Identifier{
										Identifier: "I",
										Pos:        Position{Offset: 18, Line: 2, Column: 17},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 19, Line: 2, Column: 18},
							},
						},
						EndPos: Position{Offset: 20, Line: 2, Column: 19},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 24, Line: 2, Column: 23},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 22, Line: 2, Column: 21},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseOptionalRestrictedTypeOnlyRestrictions(t *testing.T) {

	t.Parallel()

	const code = `
       let x: @{I}? = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: true,
					Type: &OptionalType{
						Type: &RestrictedType{
							Restrictions: []*NominalType{
								{
									Identifier: Identifier{
										Identifier: "I",
										Pos:        Position{Offset: 17, Line: 2, Column: 16},
									},
								},
							},
							Range: Range{
								StartPos: Position{Offset: 16, Line: 2, Column: 15},
								EndPos:   Position{Offset: 18, Line: 2, Column: 17},
							},
						},
						EndPos: Position{Offset: 19, Line: 2, Column: 18},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 21, Line: 2, Column: 20},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseReference(t *testing.T) {

	t.Parallel()

	const code = `
       let x = &account.storage[R] as &R
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &ReferenceExpression{
					Expression: &IndexExpression{
						TargetExpression: &MemberExpression{
							Expression: &IdentifierExpression{
								Identifier: Identifier{
									Identifier: "account",
									Pos:        Position{Offset: 17, Line: 2, Column: 16},
								},
							},
							Identifier: Identifier{
								Identifier: "storage",
								Pos:        Position{Offset: 25, Line: 2, Column: 24},
							},
						},
						IndexingExpression: &IdentifierExpression{
							Identifier: Identifier{
								Identifier: "R",
								Pos:        Position{Offset: 33, Line: 2, Column: 32},
							},
						},
						Range: Range{
							StartPos: Position{Offset: 32, Line: 2, Column: 31},
							EndPos:   Position{Offset: 34, Line: 2, Column: 33},
						},
					},
					Type: &ReferenceType{
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "R",
								Pos:        Position{Offset: 40, Line: 2, Column: 39},
							},
						},
						StartPos: Position{Offset: 39, Line: 2, Column: 38},
					},
					StartPos: Position{Offset: 16, Line: 2, Column: 15},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 14, Line: 2, Column: 13},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
		nil,
	)
}

func TestParseCompositeDeclarationWithSemicolonSeparatedMembers(t *testing.T) {

	t.Parallel()

	const code = `
        struct Kitty { let id: Int ; init(id: Int) { self.id = id } }
    `

	testParse(
		t,
		code,
		[]Declaration{
			&CompositeDeclaration{
				CompositeKind: common.CompositeKindStructure,
				Identifier: Identifier{
					Identifier: "Kitty",
					Pos:        Position{Offset: 16, Line: 2, Column: 15},
				},
				Members: &Members{
					Fields: []*FieldDeclaration{
						{
							VariableKind: VariableKindConstant,
							Identifier: Identifier{
								Identifier: "id",
								Pos:        Position{Offset: 28, Line: 2, Column: 27},
							},
							TypeAnnotation: &TypeAnnotation{
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 32, Line: 2, Column: 31},
									},
								},
								StartPos: Position{Offset: 32, Line: 2, Column: 31},
							},
							Range: Range{
								StartPos: Position{Offset: 24, Line: 2, Column: 23},
								EndPos:   Position{Offset: 34, Line: 2, Column: 33},
							},
						},
					},
					SpecialFunctions: []*SpecialFunctionDeclaration{
						{
							Kind: common.DeclarationKindInitializer,
							FunctionDeclaration: &FunctionDeclaration{
								Identifier: Identifier{
									Identifier: "init",
									Pos:        Position{Offset: 38, Line: 2, Column: 37},
								},
								ParameterList: &ParameterList{
									Parameters: []*Parameter{
										{
											Identifier: Identifier{
												Identifier: "id",
												Pos:        Position{Offset: 43, Line: 2, Column: 42},
											},
											TypeAnnotation: &TypeAnnotation{
												Type: &NominalType{
													Identifier: Identifier{
														Identifier: "Int",
														Pos:        Position{Offset: 47, Line: 2, Column: 46},
													},
												},
												StartPos: Position{Offset: 47, Line: 2, Column: 46},
											},
											Range: Range{
												StartPos: Position{Offset: 43, Line: 2, Column: 42},
												EndPos:   Position{Offset: 49, Line: 2, Column: 48},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 42, Line: 2, Column: 41},
										EndPos:   Position{Offset: 50, Line: 2, Column: 49},
									},
								},
								FunctionBlock: &FunctionBlock{
									Block: &Block{
										Statements: []Statement{
											&AssignmentStatement{
												Target: &MemberExpression{
													Expression: &IdentifierExpression{
														Identifier: Identifier{
															Identifier: "self",
															Pos:        Position{Offset: 54, Line: 2, Column: 53},
														},
													},
													Identifier: Identifier{
														Identifier: "id",
														Pos:        Position{Offset: 59, Line: 2, Column: 58},
													},
												},
												Transfer: &Transfer{
													Operation: TransferOperationCopy,
													Pos:       Position{Offset: 62, Line: 2, Column: 61},
												},
												Value: &IdentifierExpression{
													Identifier: Identifier{
														Identifier: "id",
														Pos:        Position{Offset: 64, Line: 2, Column: 63},
													},
												},
											},
										},
										Range: Range{
											StartPos: Position{Offset: 52, Line: 2, Column: 51},
											EndPos:   Position{Offset: 67, Line: 2, Column: 66},
										},
									},
								},
								StartPos: Position{Offset: 38, Line: 2, Column: 37},
							},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 9, Line: 2, Column: 8},
					EndPos:   Position{Offset: 69, Line: 2, Column: 68},
				},
			},
		},
		nil,
	)
}

func TestParseAccessModifiers(t *testing.T) {

	t.Parallel()

	type declaration struct {
		name, code string
	}

	declarations := []declaration{
		{"variable", "%s var test = 1"},
		{"constant", "%s let test = 1"},
		{"function", "%s fun test() {}"},
	}

	for _, compositeKind := range common.AllCompositeKinds {

		for _, isInterface := range []bool{true, false} {

			if !compositeKind.SupportsInterfaces() && isInterface {
				continue
			}

			interfaceKeyword := ""
			if isInterface {
				interfaceKeyword = "interface"
			}

			formatName := func(name string) string {
				return fmt.Sprintf(
					"%s %s %s",
					compositeKind.Keyword(),
					interfaceKeyword,
					name,
				)
			}

			formatCode := func(format string) string {
				return fmt.Sprintf(format, compositeKind.Keyword(), interfaceKeyword)
			}

			if compositeKind == common.CompositeKindEvent {
				declarations = append(declarations,
					declaration{
						formatName("itself"),
						formatCode("%%s %s %s Test()"),
					},
				)
			} else {
				declarations = append(declarations,
					declaration{
						formatName("itself"),
						formatCode("%%s %s %s Test {}"),
					},
					declaration{
						formatName("field"),
						formatCode("%s %s Test { %%s let test: Int ; init() { self.test = 1 } }"),
					},
					declaration{
						formatName("function"),
						formatCode("%s %s Test { %%s fun test() {} }"),
					},
				)
			}
		}
	}

	for _, declaration := range declarations {
		for _, access := range BasicAccesses {
			testName := fmt.Sprintf("%s/%s", declaration.name, access)
			t.Run(testName, func(t *testing.T) {
				program := fmt.Sprintf(declaration.code, access.Keyword())
				testParse(t, program, nil, nil)
			})
		}
	}
}

func TestParseTransactionDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("EmptyTransaction", func(t *testing.T) {

		const code = `
		  transaction {}
		`

		testParse(
			t,
			code,
			[]Declaration{
				&TransactionDeclaration{
					Fields:         nil,
					Prepare:        nil,
					PreConditions:  nil,
					PostConditions: nil,
					Execute:        nil,
					Range: Range{
						StartPos: Position{Offset: 5, Line: 2, Column: 4},
						EndPos:   Position{Offset: 18, Line: 2, Column: 17},
					},
				},
			},
			nil,
		)
	})

	t.Run("SimpleTransaction", func(t *testing.T) {
		const code = `
		  transaction {
	
		    var x: Int
	
		    prepare(signer: AuthAccount) {
	          x = 0
			}
	
		    execute {
	          x = 1 + 1
			}
		  }
		`

		testParse(
			t,
			code,
			[]Declaration{
				&TransactionDeclaration{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessNotSpecified,
							VariableKind: VariableKindVariable,
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 31, Line: 4, Column: 10},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 34, Line: 4, Column: 13},
									},
								},
								StartPos: Position{Offset: 34, Line: 4, Column: 13},
							},
							Range: Range{
								StartPos: Position{Offset: 27, Line: 4, Column: 6},
								EndPos:   Position{Offset: 36, Line: 4, Column: 15},
							},
						},
					},
					Prepare: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindPrepare,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "prepare",
								Pos:        Position{Offset: 46, Line: 6, Column: 6},
							},
							ParameterList: &ParameterList{
								Parameters: []*Parameter{
									{
										Label: "",
										Identifier: Identifier{
											Identifier: "signer",
											Pos:        Position{Offset: 54, Line: 6, Column: 14},
										},
										TypeAnnotation: &TypeAnnotation{
											IsResource: false,
											Type: &NominalType{
												Identifier: Identifier{
													Identifier: "AuthAccount",
													Pos:        Position{Offset: 62, Line: 6, Column: 22},
												},
											},
											StartPos: Position{Offset: 62, Line: 6, Column: 22},
										},
										Range: Range{
											StartPos: Position{Offset: 54, Line: 6, Column: 14},
											EndPos:   Position{Offset: 72, Line: 6, Column: 32},
										},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 53, Line: 6, Column: 13},
									EndPos:   Position{Offset: 73, Line: 6, Column: 33},
								},
							},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 88, Line: 7, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 90, Line: 7, Column: 13},
											},
											Value: &IntegerExpression{
												Value: new(big.Int),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 92, Line: 7, Column: 15},
													EndPos:   Position{Offset: 92, Line: 7, Column: 15},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 75, Line: 6, Column: 35},
										EndPos:   Position{Offset: 97, Line: 8, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 46, Line: 6, Column: 6},
						},
					},
					PreConditions:  nil,
					PostConditions: nil,
					Execute: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindExecute,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "execute",
								Pos:        Position{Offset: 107, Line: 10, Column: 6},
							},
							ParameterList:        &ParameterList{},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 128, Line: 11, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 130, Line: 11, Column: 13},
											},
											Value: &BinaryExpression{
												Operation: OperationPlus,
												Left: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 132, Line: 11, Column: 15},
														EndPos:   Position{Offset: 132, Line: 11, Column: 15},
													},
												},
												Right: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 136, Line: 11, Column: 19},
														EndPos:   Position{Offset: 136, Line: 11, Column: 19},
													},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 115, Line: 10, Column: 14},
										EndPos:   Position{Offset: 141, Line: 12, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 107, Line: 10, Column: 6},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 5, Line: 2, Column: 4},
						EndPos:   Position{Offset: 147, Line: 13, Column: 4},
					},
				},
			},
			nil,
		)
	})

	t.Run("PreExecutePost", func(t *testing.T) {
		const code = `
		  transaction {
	
		    var x: Int
	
		    prepare(signer: AuthAccount) {
	          x = 0
			}
	
			pre {
	      	  x == 0
			}
	
		    execute {
	          x = 1 + 1
			}
	
		    post {
	          x == 2
	        }
		  }
		`
		testParse(
			t,
			code,
			[]Declaration{
				&TransactionDeclaration{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessNotSpecified,
							VariableKind: VariableKindVariable,
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 31, Line: 4, Column: 10},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 34, Line: 4, Column: 13},
									},
								},
								StartPos: Position{Offset: 34, Line: 4, Column: 13},
							},
							Range: Range{
								StartPos: Position{Offset: 27, Line: 4, Column: 6},
								EndPos:   Position{Offset: 36, Line: 4, Column: 15},
							},
						},
					},
					Prepare: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindPrepare,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "prepare",
								Pos:        Position{Offset: 46, Line: 6, Column: 6},
							},
							ParameterList: &ParameterList{
								Parameters: []*Parameter{
									{
										Label: "",
										Identifier: Identifier{
											Identifier: "signer",
											Pos:        Position{Offset: 54, Line: 6, Column: 14},
										},
										TypeAnnotation: &TypeAnnotation{
											IsResource: false,
											Type: &NominalType{
												Identifier: Identifier{
													Identifier: "AuthAccount",
													Pos:        Position{Offset: 62, Line: 6, Column: 22},
												},
											},
											StartPos: Position{Offset: 62, Line: 6, Column: 22},
										},
										Range: Range{
											StartPos: Position{Offset: 54, Line: 6, Column: 14},
											EndPos:   Position{Offset: 72, Line: 6, Column: 32},
										},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 53, Line: 6, Column: 13},
									EndPos:   Position{Offset: 73, Line: 6, Column: 33},
								},
							},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 88, Line: 7, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 90, Line: 7, Column: 13},
											},
											Value: &IntegerExpression{
												Value: new(big.Int),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 92, Line: 7, Column: 15},
													EndPos:   Position{Offset: 92, Line: 7, Column: 15},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 75, Line: 6, Column: 35},
										EndPos:   Position{Offset: 97, Line: 8, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 46, Line: 6, Column: 6},
						},
					},
					PreConditions: &Conditions{
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "x",
										Pos:        Position{Offset: 120, Line: 11, Column: 10},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 125, Line: 11, Column: 15},
										EndPos:   Position{Offset: 125, Line: 11, Column: 15},
									},
								},
							},
						},
					},
					PostConditions: &Conditions{
						{
							Kind: ConditionKindPost,
							Test: &BinaryExpression{
								Operation: OperationEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "x",
										Pos:        Position{Offset: 202, Line: 19, Column: 11},
									},
								},
								Right: &IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 207, Line: 19, Column: 16},
										EndPos:   Position{Offset: 207, Line: 19, Column: 16},
									},
								},
							},
						},
					},
					Execute: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindExecute,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "execute",
								Pos:        Position{Offset: 140, Line: 14, Column: 6},
							},
							ParameterList:        &ParameterList{},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 161, Line: 15, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 163, Line: 15, Column: 13},
											},
											Value: &BinaryExpression{
												Operation: OperationPlus,
												Left: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 165, Line: 15, Column: 15},
														EndPos:   Position{Offset: 165, Line: 15, Column: 15},
													},
												},
												Right: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 169, Line: 15, Column: 19},
														EndPos:   Position{Offset: 169, Line: 15, Column: 19},
													},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 148, Line: 14, Column: 14},
										EndPos:   Position{Offset: 174, Line: 16, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 140, Line: 14, Column: 6},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 5, Line: 2, Column: 4},
						EndPos:   Position{Offset: 224, Line: 21, Column: 4},
					},
				},
			},
			nil,
		)
	})

	t.Run("PrePostExecute", func(t *testing.T) {
		const code = `
		  transaction {
	
		    var x: Int
	
		    prepare(signer: AuthAccount) {
	          x = 0
			}
	
			pre {
	      	  x == 0
			}

		    post {
	          x == 2
	        }
	
		    execute {
	          x = 1 + 1
			}
		  }
		`

		testParse(
			t,
			code,
			[]Declaration{
				&TransactionDeclaration{
					Fields: []*FieldDeclaration{
						{
							Access:       AccessNotSpecified,
							VariableKind: VariableKindVariable,
							Identifier: Identifier{
								Identifier: "x",
								Pos:        Position{Offset: 31, Line: 4, Column: 10},
							},
							TypeAnnotation: &TypeAnnotation{
								IsResource: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 34, Line: 4, Column: 13},
									},
								},
								StartPos: Position{Offset: 34, Line: 4, Column: 13},
							},
							Range: Range{
								StartPos: Position{Offset: 27, Line: 4, Column: 6},
								EndPos:   Position{Offset: 36, Line: 4, Column: 15},
							},
						},
					},
					Prepare: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindPrepare,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "prepare",
								Pos:        Position{Offset: 46, Line: 6, Column: 6},
							},
							ParameterList: &ParameterList{
								Parameters: []*Parameter{
									{
										Label: "",
										Identifier: Identifier{
											Identifier: "signer",
											Pos:        Position{Offset: 54, Line: 6, Column: 14},
										},
										TypeAnnotation: &TypeAnnotation{
											IsResource: false,
											Type: &NominalType{
												Identifier: Identifier{
													Identifier: "AuthAccount",
													Pos:        Position{Offset: 62, Line: 6, Column: 22},
												},
											},
											StartPos: Position{Offset: 62, Line: 6, Column: 22},
										},
										Range: Range{
											StartPos: Position{Offset: 54, Line: 6, Column: 14},
											EndPos:   Position{Offset: 72, Line: 6, Column: 32},
										},
									},
								},
								Range: Range{
									StartPos: Position{Offset: 53, Line: 6, Column: 13},
									EndPos:   Position{Offset: 73, Line: 6, Column: 33},
								},
							},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 88, Line: 7, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 90, Line: 7, Column: 13},
											},
											Value: &IntegerExpression{
												Value: new(big.Int),
												Base:  10,
												Range: Range{
													StartPos: Position{Offset: 92, Line: 7, Column: 15},
													EndPos:   Position{Offset: 92, Line: 7, Column: 15},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 75, Line: 6, Column: 35},
										EndPos:   Position{Offset: 97, Line: 8, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 46, Line: 6, Column: 6},
						},
					},
					PreConditions: &Conditions{
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "x",
										Pos:        Position{Offset: 120, Line: 11, Column: 10},
									},
								},
								Right: &IntegerExpression{
									Value: new(big.Int),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 125, Line: 11, Column: 15},
										EndPos:   Position{Offset: 125, Line: 11, Column: 15},
									},
								},
							},
						},
					},
					PostConditions: &Conditions{
						{
							Kind: ConditionKindPost,
							Test: &BinaryExpression{
								Operation: OperationEqual,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "x",
										Pos:        Position{Offset: 157, Line: 15, Column: 11},
									},
								},
								Right: &IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 162, Line: 15, Column: 16},
										EndPos:   Position{Offset: 162, Line: 15, Column: 16},
									},
								},
							},
						},
					},
					Execute: &SpecialFunctionDeclaration{
						Kind: common.DeclarationKindExecute,
						FunctionDeclaration: &FunctionDeclaration{
							Access: AccessNotSpecified,
							Identifier: Identifier{
								Identifier: "execute",
								Pos:        Position{Offset: 183, Line: 18, Column: 6},
							},
							ParameterList:        &ParameterList{},
							ReturnTypeAnnotation: nil,
							FunctionBlock: &FunctionBlock{
								Block: &Block{
									Statements: []Statement{
										&AssignmentStatement{
											Target: &IdentifierExpression{
												Identifier: Identifier{
													Identifier: "x",
													Pos:        Position{Offset: 204, Line: 19, Column: 11},
												},
											},
											Transfer: &Transfer{
												Operation: TransferOperationCopy,
												Pos:       Position{Offset: 206, Line: 19, Column: 13},
											},
											Value: &BinaryExpression{
												Operation: OperationPlus,
												Left: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 208, Line: 19, Column: 15},
														EndPos:   Position{Offset: 208, Line: 19, Column: 15},
													},
												},
												Right: &IntegerExpression{
													Value: big.NewInt(1),
													Base:  10,
													Range: Range{
														StartPos: Position{Offset: 212, Line: 19, Column: 19},
														EndPos:   Position{Offset: 212, Line: 19, Column: 19},
													},
												},
											},
										},
									},
									Range: Range{
										StartPos: Position{Offset: 191, Line: 18, Column: 14},
										EndPos:   Position{Offset: 217, Line: 20, Column: 3},
									},
								},
								PreConditions:  nil,
								PostConditions: nil,
							},
							StartPos: Position{Offset: 183, Line: 18, Column: 6},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 5, Line: 2, Column: 4},
						EndPos:   Position{Offset: 223, Line: 21, Column: 4},
					},
				},
			},
			nil,
		)

	})
}

func TestParseAuthorizedReferenceType(t *testing.T) {

	t.Parallel()

	const code = `
       let x: auth &R = 1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					IsResource: false,
					Type: &ReferenceType{
						Authorized: true,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "R", Pos: Position{Offset: 21, Line: 2, Column: 20}},
						},
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
					},
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Value: &IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: Range{
						StartPos: Position{Offset: 25, Line: 2, Column: 24},
						EndPos:   Position{Offset: 25, Line: 2, Column: 24},
					},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPos:          Position{Offset: 8, Line: 2, Column: 7},
				SecondTransfer:    nil,
				SecondValue:       nil,
				ParentIfStatement: nil,
			},
		},
		nil,
	)
}

func TestParseFixedPointExpression(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = -1234_5678_90.0009_8765_4321
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "a",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &FixedPointExpression{
					Negative:        true,
					UnsignedInteger: big.NewInt(1234567890),
					Fractional:      big.NewInt(987654321),
					Scale:           12,
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 41, Line: 2, Column: 40},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseFixedPointExpressionZeroInteger(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = -0.1
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "a",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &FixedPointExpression{
					Negative:        true,
					UnsignedInteger: new(big.Int),
					Fractional:      big.NewInt(1),
					Scale:           1,
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func BenchmarkParseDeploy(b *testing.B) {

	b.Run("byte array", func(b *testing.B) {

		var builder strings.Builder
		for i := 0; i < 15000; i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(strconv.Itoa(rand.Intn(math.MaxUint8)))
		}

		transaction := fmt.Sprintf(`
              transaction {
                execute {
                  AuthAccount(publicKeys: [], code: [%s])
                }
              }
            `,
			builder.String(),
		)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, err := parser.ParseProgram(transaction)
			if err != nil {
				b.FailNow()
			}
		}
	})

	b.Run("decode hex", func(b *testing.B) {

		var builder strings.Builder
		for i := 0; i < 15000; i++ {
			builder.WriteString(fmt.Sprintf("%02x", i))
		}

		transaction := fmt.Sprintf(`
              transaction {
                execute {
                  AuthAccount(publicKeys: [], code: "%s".decodeHex())
                }
              }
            `,
			builder.String(),
		)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, err := parser.ParseProgram(transaction)
			if err != nil {
				b.FailNow()
			}
		}
	})
}

const fungibleTokenContract = `
pub contract FungibleToken {

    pub resource interface Provider {
        pub fun withdraw(amount: Int): @Vault {
            pre {
                amount > 0:
                    "Withdrawal amount must be positive"
            }
            post {
                result.balance == amount:
                    "Incorrect amount returned"
            }
        }
    }

    pub resource interface Receiver {
        pub balance: Int

        init(balance: Int) {
            pre {
                balance >= 0:
                    "Initial balance must be non-negative"
            }
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }

        pub fun deposit(from: @Receiver) {
            pre {
                from.balance > 0:
                    "Deposit balance needs to be positive!"
            }
            post {
                self.balance == before(self.balance) + before(from.balance):
                    "Incorrect amount removed"
            }
        }
    }

    pub resource Vault: Provider, Receiver {

        pub var balance: Int

        init(balance: Int) {
            self.balance = balance
        }

        pub fun withdraw(amount: Int): @Vault {
            self.balance = self.balance - amount
            return <-create Vault(balance: amount)
        }

        // transfer combines withdraw and deposit into one function call
        pub fun transfer(to: &Receiver, amount: Int) {
            pre {
                amount <= self.balance:
                    "Insufficient funds"
            }
            post {
                self.balance == before(self.balance) - amount:
                    "Incorrect amount removed"
            }
            to.deposit(from: <-self.withdraw(amount: amount))
        }

        pub fun deposit(from: @Receiver) {
            self.balance = self.balance + from.balance
            destroy from
        }

        pub fun createEmptyVault(): @Vault {
            return <-create Vault(balance: 0)
        }
    }

    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balance: 0)
    }

    pub resource VaultMinter {
        pub fun mintTokens(amount: Int, recipient: &Receiver) {
            recipient.deposit(from: <-create Vault(balance: amount))
        }
    }

    init() {
        let oldVault <- self.account.storage[Vault] <- create Vault(balance: 30)
        destroy oldVault

        let oldMinter <- self.account.storage[VaultMinter] <- create VaultMinter()
        destroy oldMinter
    }
}
`

func BenchmarkParseFungibleToken(b *testing.B) {

	b.Run("old", func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			_, _, err := parser.ParseProgram(fungibleTokenContract)
			if err != nil {
				b.FailNow()
			}
		}
	})

	b.Run("new", func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			_, err := parser2.ParseProgram(fungibleTokenContract)
			if err != nil {
				b.FailNow()
			}
		}
	})
}

func TestParsePathLiteral(t *testing.T) {

	t.Parallel()

	const code = `
	    let a = /foo/bar
	`
	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{Identifier: "a",
					Pos: Position{Offset: 10, Line: 2, Column: 9},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &PathExpression{
					StartPos: Position{Offset: 14, Line: 2, Column: 13},
					Domain: Identifier{
						Identifier: "foo",
						Pos:        Position{Offset: 15, Line: 2, Column: 14},
					},
					Identifier: Identifier{
						Identifier: "bar",
						Pos:        Position{Offset: 19, Line: 2, Column: 18},
					},
				},
				StartPos: Position{Offset: 6, Line: 2, Column: 5},
			},
		},
		nil,
	)
}

func TestParseInvalidForceCast(t *testing.T) {

	t.Parallel()

	_, _, err := parser.ParseReplInput("1 as!! Int\n")

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 5, Line: 1, Column: 5},
		syntaxError.Pos,
	)
}

func TestParseInvalidNegativeIntegerLiteralWithIncorrectPrefix(t *testing.T) {

	t.Parallel()

	_, _, err := parser.ParseProgram(`
	    let e = -0K0
	`)

	require.Error(t, err)
}

func TestParseConstantSizedSizedArrayWithTrailingUnderscoreSize(t *testing.T) {

	t.Parallel()

	actual, _, err := parser.ParseProgram(`
	  let T:[d;0_]=0
	`)

	assert.NotNil(t, actual)

	require.Error(t, err)

	require.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	require.IsType(t, &parser.InvalidIntegerLiteralError{}, errors[0])
}

func TestParsePreconditionWithUnaryNegation(t *testing.T) {

	t.Parallel()

	const code = `
	  fun test() {
          pre {
              true: "one"
              !false: "two"
          }
      }
	`
	testParse(
		t,
		code,
		[]Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 8, Line: 2, Column: 7},
				},
				ParameterList: &ParameterList{
					Range: Range{
						StartPos: Position{Offset: 12, Line: 2, Column: 11},
						EndPos:   Position{Offset: 13, Line: 2, Column: 12},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Type: &NominalType{
						Identifier: Identifier{
							Pos: Position{Offset: 13, Line: 2, Column: 12},
						},
					},
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Range: Range{
							StartPos: Position{Offset: 15, Line: 2, Column: 14},
							EndPos:   Position{Offset: 105, Line: 7, Column: 6},
						},
					},
					PreConditions: &Conditions{
						{
							Kind: ConditionKindPre,
							Test: &BoolExpression{
								Value: true,
								Range: Range{
									StartPos: Position{Offset: 47, Line: 4, Column: 14},
									EndPos:   Position{Offset: 50, Line: 4, Column: 17},
								},
							},
							Message: &StringExpression{
								Value: "one",
								Range: Range{
									StartPos: Position{Offset: 53, Line: 4, Column: 20},
									EndPos:   Position{Offset: 57, Line: 4, Column: 24},
								},
							},
						},
						{
							Kind: ConditionKindPre,
							Test: &UnaryExpression{
								Operation: OperationNegate,
								Expression: &BoolExpression{
									Value: false,
									Range: Range{
										StartPos: Position{Offset: 74, Line: 5, Column: 15},
										EndPos:   Position{Offset: 78, Line: 5, Column: 19},
									},
								},
								StartPos: Position{Offset: 73, Line: 5, Column: 14},
							},
							Message: &StringExpression{
								Value: "two",
								Range: Range{
									StartPos: Position{Offset: 81, Line: 5, Column: 22},
									EndPos:   Position{Offset: 85, Line: 5, Column: 26},
								},
							},
						},
					},
				},
				StartPos: Position{Offset: 4, Line: 2, Column: 3},
			},
		},
		nil,
	)
}

func TestParseBitwiseExpression(t *testing.T) {

	t.Parallel()

	const code = `
      let a = 1 | 2 ^ 3 & 4 << 5 >> 6
	`

	testParse(
		t,
		code,
		[]Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 11, Line: 2, Column: 10},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 13, Line: 2, Column: 12},
				},
				Value: &BinaryExpression{
					Operation: OperationBitwiseOr,
					Left: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: Range{
							StartPos: Position{Offset: 15, Line: 2, Column: 14},
							EndPos:   Position{Offset: 15, Line: 2, Column: 14},
						},
					},
					Right: &BinaryExpression{
						Operation: OperationBitwiseXor,
						Left: &IntegerExpression{
							Value: big.NewInt(2),
							Base:  10,
							Range: Range{
								StartPos: Position{Offset: 19, Line: 2, Column: 18},
								EndPos:   Position{Offset: 19, Line: 2, Column: 18},
							},
						},
						Right: &BinaryExpression{
							Operation: OperationBitwiseAnd,
							Left: &IntegerExpression{
								Value: big.NewInt(3),
								Base:  10,
								Range: Range{
									StartPos: Position{Offset: 23, Line: 2, Column: 22},
									EndPos:   Position{Offset: 23, Line: 2, Column: 22},
								},
							},
							Right: &BinaryExpression{
								Operation: OperationBitwiseRightShift,
								Left: &BinaryExpression{
									Operation: OperationBitwiseLeftShift,
									Left: &IntegerExpression{
										Value: big.NewInt(4),
										Base:  10,
										Range: Range{
											StartPos: Position{Offset: 27, Line: 2, Column: 26},
											EndPos:   Position{Offset: 27, Line: 2, Column: 26},
										},
									},
									Right: &IntegerExpression{
										Value: big.NewInt(5),
										Base:  10,
										Range: Range{
											StartPos: Position{Offset: 32, Line: 2, Column: 31},
											EndPos:   Position{Offset: 32, Line: 2, Column: 31},
										},
									},
								},
								Right: &IntegerExpression{
									Value: big.NewInt(6),
									Base:  10,
									Range: Range{
										StartPos: Position{Offset: 37, Line: 2, Column: 36},
										EndPos:   Position{Offset: 37, Line: 2, Column: 36},
									},
								},
							},
						},
					},
				},
				StartPos: Position{Offset: 7, Line: 2, Column: 6},
			},
		},
		nil,
	)
}

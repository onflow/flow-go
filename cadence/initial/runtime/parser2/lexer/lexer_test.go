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

package lexer

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/goleak"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/tests/utils"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func withTokens(tokenChan chan Token, fn func([]Token)) {
	tokens := make([]Token, 0)
	for {
		token, ok := <-tokenChan
		if !ok {
			fn(tokens)
			return
		}
		tokens = append(tokens, token)
	}
}

func testLex(t *testing.T, input string, expected []Token) {

	t.Parallel()

	ctx := context.Background()

	withTokens(Lex(ctx, input), func(tokens []Token) {
		utils.AssertEqualWithDiff(t, expected, tokens)
	})
}

func TestLexBasic(t *testing.T) {

	t.Parallel()

	t.Run("two numbers separated by whitespace", func(t *testing.T) {
		testLex(t,
			" 01\t  10",
			[]Token{
				{

					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "01",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\t  ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "10",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("simple arithmetic: plus and times", func(t *testing.T) {
		testLex(t,
			"(2 + 3) * 4",
			[]Token{
				{
					Type: TokenParenOpen,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "2",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenPlus,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "3",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				{
					Type: TokenParenClose,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenStar,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "4",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
						EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
			},
		)
	})

	t.Run("simple arithmetic: minus and div", func(t *testing.T) {
		testLex(t,
			"(2 - 3) / 4",
			[]Token{
				{
					Type: TokenParenOpen,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "2",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenMinus,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "3",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				{
					Type: TokenParenClose,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenSlash,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "4",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
						EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
			},
		)
	})

	t.Run("multiple lines", func(t *testing.T) {
		testLex(t,
			"1 \n  2\n",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "1",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" \n  ", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 2, Column: 1, Offset: 4},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "2",
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 2, Offset: 5},
						EndPos:   ast.Position{Line: 2, Column: 2, Offset: 5},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\n", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 3, Offset: 6},
						EndPos:   ast.Position{Line: 2, Column: 3, Offset: 6},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 3, Column: 0, Offset: 7},
						EndPos:   ast.Position{Line: 3, Column: 0, Offset: 7},
					},
				},
			},
		)
	})

	t.Run("nil-coalesce", func(t *testing.T) {
		testLex(t,
			"1 ?? 2",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "1",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenDoubleQuestionMark,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{" ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "2",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
			},
		)
	})

	t.Run("identifier", func(t *testing.T) {
		testLex(t,
			"test",
			[]Token{
				{
					Type:  TokenIdentifier,
					Value: "test",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
		)
	})

	t.Run("identifier with leading underscore and trailing numbers", func(t *testing.T) {
		testLex(t,
			"_test_123",
			[]Token{
				{
					Type:  TokenIdentifier,
					Value: "_test_123",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
			},
		)
	})

	t.Run("colon, comma, semicolon, question mark", func(t *testing.T) {
		testLex(t,
			":,;.?",

			[]Token{
				{
					Type: TokenColon,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type: TokenComma,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenSemicolon,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenDot,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenQuestionMark,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
			},
		)
	})

	t.Run("brackets and braces", func(t *testing.T) {
		testLex(t,
			"[}]{",
			[]Token{
				{
					Type: TokenBracketOpen,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type: TokenBraceClose,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenBracketClose,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},

				{
					Type: TokenBraceOpen,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
		)
	})

	t.Run("comparisons", func(t *testing.T) {
		testLex(t,
			"=<><-<=>=",
			[]Token{
				{
					Type: TokenEqual,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type: TokenLess,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenGreater,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenLeftArrow,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type: TokenLessEqual,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
				{
					Type: TokenGreaterEqual,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
			},
		)
	})
}

func TestLexString(t *testing.T) {

	t.Parallel()

	t.Run("valid, empty", func(t *testing.T) {
		testLex(t,
			`""`,
			[]Token{
				{
					Type:  TokenString,
					Value: `""`,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("valid, non-empty", func(t *testing.T) {
		testLex(t,
			`"test"`,
			[]Token{
				{
					Type:  TokenString,
					Value: `"test"`,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
			},
		)
	})

	t.Run("valid, with valid tab escape", func(t *testing.T) {
		testLex(t,
			`"te\tst"`,
			[]Token{
				{
					Type:  TokenString,
					Value: `"te\tst"`,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("valid, with invalid escape character", func(t *testing.T) {
		testLex(t,
			`"te\Xst"`,
			[]Token{
				{
					Type:  TokenString,
					Value: `"te\Xst"`,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("valid, with valid quote escape", func(t *testing.T) {
		testLex(t,
			`"te\"st"`,
			[]Token{
				{
					Type:  TokenString,
					Value: `"te\"st"`,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("invalid, empty, not terminated at line end", func(t *testing.T) {
		testLex(t,
			"\"\n",
			[]Token{
				{
					Type:  TokenString,
					Value: "\"",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\n", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 0, Offset: 2},
						EndPos:   ast.Position{Line: 2, Column: 0, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("invalid, non-empty, not terminated at line end", func(t *testing.T) {
		testLex(t,
			"\"te\n",
			[]Token{
				{
					Type:  TokenString,
					Value: "\"te",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\n", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 0, Offset: 4},
						EndPos:   ast.Position{Line: 2, Column: 0, Offset: 4},
					},
				},
			},
		)
	})

	t.Run("invalid, empty, not terminated at end of file", func(t *testing.T) {
		testLex(t,
			"\"",
			[]Token{
				{
					Type:  TokenString,
					Value: "\"",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
			},
		)
	})

	t.Run("invalid, non-empty, not terminated at end of file", func(t *testing.T) {
		testLex(t,
			"\"te",
			[]Token{
				{
					Type:  TokenString,
					Value: "\"te",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
			},
		)
	})

	t.Run("invalid, missing escape character", func(t *testing.T) {
		testLex(t,
			"\"\\\n",
			[]Token{
				{
					Type:  TokenString,
					Value: "\"\\",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\n", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 0, Offset: 3},
						EndPos:   ast.Position{Line: 2, Column: 0, Offset: 3},
					},
				},
			},
		)
	})
}

func TestLexBlockComment(t *testing.T) {

	t.Parallel()

	t.Run("nested 1", func(t *testing.T) {
		testLex(t,
			`/*  // *X /* \\*  */`,
			[]Token{
				{
					Type: TokenBlockCommentStart,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenBlockCommentContent,
					Value: `  // *X `,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
				{
					Type: TokenBlockCommentStart,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				{
					Type:  TokenBlockCommentContent,
					Value: ` \\*  `,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
						EndPos:   ast.Position{Line: 1, Column: 17, Offset: 17},
					},
				},
				{
					Type: TokenBlockCommentEnd,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 18, Offset: 18},
						EndPos:   ast.Position{Line: 1, Column: 19, Offset: 19},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 20, Offset: 20},
						EndPos:   ast.Position{Line: 1, Column: 20, Offset: 20},
					},
				},
			},
		)
	})

	t.Run("nested 2", func(t *testing.T) {
		testLex(t,
			`/* test foo /* bar */ asd */  `,
			[]Token{
				{
					Type: TokenBlockCommentStart,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenBlockCommentContent,
					Value: ` test foo `,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				{
					Type: TokenBlockCommentStart,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
						EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
					},
				},
				{
					Type:  TokenBlockCommentContent,
					Value: ` bar `,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 14, Offset: 14},
						EndPos:   ast.Position{Line: 1, Column: 18, Offset: 18},
					},
				},
				{
					Type: TokenBlockCommentEnd,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 19, Offset: 19},
						EndPos:   ast.Position{Line: 1, Column: 20, Offset: 20},
					},
				},
				{
					Type:  TokenBlockCommentContent,
					Value: ` asd `,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 21, Offset: 21},
						EndPos:   ast.Position{Line: 1, Column: 25, Offset: 25},
					},
				},
				{
					Type: TokenBlockCommentEnd,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 26, Offset: 26},
						EndPos:   ast.Position{Line: 1, Column: 27, Offset: 27},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"  ", false},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 28, Offset: 28},
						EndPos:   ast.Position{Line: 1, Column: 29, Offset: 29},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 30, Offset: 30},
						EndPos:   ast.Position{Line: 1, Column: 30, Offset: 30},
					},
				},
			},
		)
	})
}

func TestLexIntegerLiterals(t *testing.T) {

	t.Parallel()

	t.Run("binary prefix, missing trailing digits", func(t *testing.T) {
		testLex(t,
			`0b`,
			[]Token{
				{
					Type:  TokenError,
					Value: errors.New("missing digits"),
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenBinaryLiteral,
					Value: "0b",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("binary", func(t *testing.T) {
		testLex(t,
			`0b101010`,
			[]Token{
				{
					Type:  TokenBinaryLiteral,
					Value: "0b101010",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("binary with leading zeros", func(t *testing.T) {
		testLex(t,
			`0b001000`,
			[]Token{
				{
					Type:  TokenBinaryLiteral,
					Value: "0b001000",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("binary with underscores", func(t *testing.T) {
		testLex(t,
			`0b101010_101010`,
			[]Token{
				{
					Type:  TokenBinaryLiteral,
					Value: "0b101010_101010",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 14, Offset: 14},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 15, Offset: 15},
						EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
					},
				},
			},
		)
	})

	t.Run("binary with leading underscore", func(t *testing.T) {
		testLex(t,
			`0b_101010_101010`,
			[]Token{
				{
					Type:  TokenBinaryLiteral,
					Value: "0b_101010_101010",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 16, Offset: 16},
						EndPos:   ast.Position{Line: 1, Column: 16, Offset: 16},
					},
				},
			},
		)
	})

	t.Run("binary with trailing underscore", func(t *testing.T) {
		testLex(t,
			`0b101010_101010_`,
			[]Token{
				{
					Type:  TokenBinaryLiteral,
					Value: "0b101010_101010_",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 16, Offset: 16},
						EndPos:   ast.Position{Line: 1, Column: 16, Offset: 16},
					},
				},
			},
		)
	})

	t.Run("octal prefix, missing trailing digits", func(t *testing.T) {
		testLex(t,
			`0o`,
			[]Token{
				{
					Type:  TokenError,
					Value: errors.New("missing digits"),
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenOctalLiteral,
					Value: "0o",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("octal", func(t *testing.T) {
		testLex(t,
			`0o32`,
			[]Token{
				{
					Type:  TokenOctalLiteral,
					Value: "0o32",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
		)
	})

	t.Run("octal with underscores", func(t *testing.T) {
		testLex(t,
			`0o32_45`,
			[]Token{
				{
					Type:  TokenOctalLiteral,
					Value: "0o32_45",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
		)
	})

	t.Run("octal with leading underscore", func(t *testing.T) {
		testLex(t,
			`0o_32_45`,
			[]Token{
				{
					Type:  TokenOctalLiteral,
					Value: "0o_32_45",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("octal with trailing underscore", func(t *testing.T) {
		testLex(t,
			`0o32_45_`,
			[]Token{
				{
					Type:  TokenOctalLiteral,
					Value: "0o32_45_",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("decimal", func(t *testing.T) {
		testLex(t,
			`1234567890`,
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "1234567890",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
						EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
					},
				},
			},
		)
	})

	t.Run("decimal with underscores", func(t *testing.T) {
		testLex(t,
			`1_234_567_890`,
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "1_234_567_890",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 13, Offset: 13},
						EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
					},
				},
			},
		)
	})

	t.Run("decimal with trailing underscore", func(t *testing.T) {
		testLex(t,
			`1_234_567_890_`,
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "1_234_567_890_",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 14, Offset: 14},
						EndPos:   ast.Position{Line: 1, Column: 14, Offset: 14},
					},
				},
			},
		)
	})

	t.Run("hexadecimal prefix, missing trailing digits", func(t *testing.T) {
		testLex(t,
			`0x`,
			[]Token{
				{
					Type:  TokenError,
					Value: errors.New("missing digits"),
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenHexadecimalLiteral,
					Value: "0x",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("hexadecimal", func(t *testing.T) {
		testLex(t,
			`0xf2`,
			[]Token{
				{
					Type:  TokenHexadecimalLiteral,
					Value: "0xf2",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
			},
		)
	})

	t.Run("hexadecimal with underscores", func(t *testing.T) {
		testLex(t,
			`0xf2_09`,
			[]Token{
				{
					Type:  TokenHexadecimalLiteral,
					Value: "0xf2_09",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
		)
	})

	t.Run("hexadecimal with leading underscore", func(t *testing.T) {
		testLex(t,
			`0x_f2_09`,
			[]Token{
				{
					Type:  TokenHexadecimalLiteral,
					Value: "0x_f2_09",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("hexadecimal with trailing underscore", func(t *testing.T) {
		testLex(t,
			`0xf2_09_`,
			[]Token{
				{
					Type:  TokenHexadecimalLiteral,
					Value: "0xf2_09_",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
					},
				},
			},
		)
	})

	t.Run("0", func(t *testing.T) {
		testLex(t,
			"0",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "0",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
			},
		)
	})

	t.Run("01", func(t *testing.T) {
		testLex(t,
			"01",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "01",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("whitespace after 0", func(t *testing.T) {
		testLex(t,
			"0\n",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "0",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenSpace,
					Value: Space{"\n", true},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 0, Offset: 2},
						EndPos:   ast.Position{Line: 2, Column: 0, Offset: 2},
					},
				},
			},
		)
	})

	t.Run("leading zeros", func(t *testing.T) {
		testLex(t,
			"00123",
			[]Token{
				{
					Type:  TokenDecimalLiteral,
					Value: "00123",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
			},
		)
	})

	t.Run("invalid prefix", func(t *testing.T) {
		testLex(t,
			"0z123",
			[]Token{
				{
					Type:  TokenError,
					Value: errors.New("invalid number literal prefix: 'z'"),
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenDecimalLiteral,
					Value: "0z123",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
			},
		)
	})
}

func TestLexFixedPoint(t *testing.T) {

	t.Parallel()

	t.Run("with underscores", func(t *testing.T) {
		testLex(t,
			"1234_5678_90.0009_8765_4321",
			[]Token{
				{
					Type:  TokenFixedPointLiteral,
					Value: "1234_5678_90.0009_8765_4321",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 26, Offset: 26},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 27, Offset: 27},
						EndPos:   ast.Position{Line: 1, Column: 27, Offset: 27},
					},
				},
			},
		)
	})

	t.Run("leading zero", func(t *testing.T) {
		testLex(t,
			"0.1",
			[]Token{
				{
					Type:  TokenFixedPointLiteral,
					Value: "0.1",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
			},
		)
	})

	t.Run("missing fractional digits", func(t *testing.T) {
		testLex(t,
			"0.",
			[]Token{
				{
					Type:  TokenError,
					Value: errors.New("missing fractional digits"),
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type:  TokenFixedPointLiteral,
					Value: "0.",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
						EndPos:   ast.Position{Line: 1, Column: 2, Offset: 2},
					},
				},
			},
		)
	})
}

func TestLexLineComment(t *testing.T) {

	t.Parallel()

	t.Run("no newline", func(t *testing.T) {

		testLex(t,
			` foo // bar `,
			[]Token{
				{
					Type: TokenSpace,
					Value: Space{
						String:          " ",
						ContainsNewline: false,
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenIdentifier,
					Value: "foo",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenSpace,
					Value: Space{
						String:          " ",
						ContainsNewline: false,
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type:  TokenLineComment,
					Value: "// bar ",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
						EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
					},
				},
			},
		)
	})

	t.Run("newline", func(t *testing.T) {

		testLex(
			t,
			" foo // bar \n baz",
			[]Token{
				{
					Type: TokenSpace,
					Value: Space{
						String:          " ",
						ContainsNewline: false,
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
				{
					Type:  TokenIdentifier,
					Value: "foo",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 3, Offset: 3},
					},
				},
				{
					Type: TokenSpace,
					Value: Space{
						String:          " ",
						ContainsNewline: false,
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 4, Offset: 4},
						EndPos:   ast.Position{Line: 1, Column: 4, Offset: 4},
					},
				},
				{
					Type:  TokenLineComment,
					Value: "// bar ",
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				{
					Type: TokenSpace,
					Value: Space{
						String:          "\n ",
						ContainsNewline: true,
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
						EndPos:   ast.Position{Line: 2, Column: 0, Offset: 13},
					},
				},
				{
					Type:  TokenIdentifier,
					Value: "baz",
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 1, Offset: 14},
						EndPos:   ast.Position{Line: 2, Column: 3, Offset: 16},
					},
				},
				{
					Type: TokenEOF,
					Range: ast.Range{
						StartPos: ast.Position{Line: 2, Column: 4, Offset: 17},
						EndPos:   ast.Position{Line: 2, Column: 4, Offset: 17},
					},
				},
			},
		)
	})
}

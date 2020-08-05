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
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func TestParseVariableDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("var, no type annotation, copy, one value", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("var x = 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.VariableDeclaration{
					IsConstant: false,
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationCopy,
						Pos:       ast.Position{Line: 1, Column: 6, Offset: 6},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("var, no type annotation, copy, one value, pub", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" pub var x = 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.VariableDeclaration{
					Access:     ast.AccessPublic,
					IsConstant: false,
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 9, Offset: 9},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 13, Offset: 13},
							EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationCopy,
						Pos:       ast.Position{Line: 1, Column: 11, Offset: 11},
					},
					StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("let, no type annotation, copy, one value", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("let x = 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.VariableDeclaration{
					IsConstant: true,
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationCopy,
						Pos:       ast.Position{Line: 1, Column: 6, Offset: 6},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("let, no type annotation, move, one value", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("let x <- 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.VariableDeclaration{
					IsConstant: true,
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationMove,
						Pos:       ast.Position{Line: 1, Column: 6, Offset: 6},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("let, resource type annotation, move, one value", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("let r2: @R <- r")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.VariableDeclaration{
					IsConstant: true,
					Identifier: ast.Identifier{
						Identifier: "r2",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					TypeAnnotation: &ast.TypeAnnotation{
						IsResource: true,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "R",
								Pos:        ast.Position{Line: 1, Column: 9, Offset: 9},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
					},
					Value: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "r",
							Pos:        ast.Position{Line: 1, Column: 14, Offset: 14},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationMove,
						Pos:       ast.Position{Line: 1, Column: 11, Offset: 11},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("var, no type annotation, copy, two values ", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("var x <- y <- z")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.VariableDeclaration{
					IsConstant: false,
					Identifier: ast.Identifier{
						Identifier: "x",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					Value: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "y",
							Pos:        ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationMove,
						Pos:       ast.Position{Line: 1, Column: 6, Offset: 6},
					},
					SecondValue: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "z",
							Pos:        ast.Position{Line: 1, Column: 14, Offset: 14},
						},
					},
					SecondTransfer: &ast.Transfer{
						Operation: ast.TransferOperationMove,
						Pos:       ast.Position{Line: 1, Column: 11, Offset: 11},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

}

func TestParseParameterList(t *testing.T) {

	t.Parallel()

	parse := func(input string) (interface{}, []error) {
		return Parse(
			input,
			func(p *parser) interface{} {
				return parseParameterList(p)
			},
		)
	}

	t.Run("empty", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ParameterList{
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 1, Offset: 1},
				},
			},
			result,
		)
	})

	t.Run("space", func(t *testing.T) {

		t.Parallel()

		result, errs := parse(" (   )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ParameterList{
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
					EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
				},
			},
			result,
		)
	})

	t.Run("one, without argument label", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("( a : Int )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ParameterList{
				Parameters: []*ast.Parameter{
					{
						Label: "",
						Identifier: ast.Identifier{
							Identifier: "a",
							Pos:        ast.Position{Line: 1, Column: 2, Offset: 2},
						},
						TypeAnnotation: &ast.TypeAnnotation{
							IsResource: false,
							Type: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "Int",
									Pos:        ast.Position{Line: 1, Column: 6, Offset: 6},
								},
							},
							StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
							EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
				},
			},
			result,
		)
	})

	t.Run("one, with argument label", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("( a b : Int )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ParameterList{
				Parameters: []*ast.Parameter{
					{
						Label: "a",
						Identifier: ast.Identifier{
							Identifier: "b",
							Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
						},
						TypeAnnotation: &ast.TypeAnnotation{
							IsResource: false,
							Type: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "Int",
									Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
								},
							},
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
							EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
						},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
				},
			},
			result,
		)
	})

	t.Run("two, with and without argument label", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("( a b : Int , c : Int )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.ParameterList{
				Parameters: []*ast.Parameter{
					{
						Label: "a",
						Identifier: ast.Identifier{
							Identifier: "b",
							Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
						},
						TypeAnnotation: &ast.TypeAnnotation{
							IsResource: false,
							Type: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "Int",
									Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
								},
							},
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 2, Offset: 2},
							EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
						},
					},
					{
						Label: "",
						Identifier: ast.Identifier{
							Identifier: "c",
							Pos:        ast.Position{Line: 1, Column: 14, Offset: 14},
						},
						TypeAnnotation: &ast.TypeAnnotation{
							IsResource: false,
							Type: &ast.NominalType{
								Identifier: ast.Identifier{
									Identifier: "Int",
									Pos:        ast.Position{Line: 1, Column: 18, Offset: 18},
								},
							},
							StartPos: ast.Position{Line: 1, Column: 18, Offset: 18},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 14, Offset: 14},
							EndPos:   ast.Position{Line: 1, Column: 20, Offset: 20},
						},
					},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 22, Offset: 22},
				},
			},
			result,
		)
	})
}

func TestParseFunctionDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("without return type", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("fun foo () { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.FunctionDeclaration{
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					ParameterList: &ast.ParameterList{
						Parameters: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					ReturnTypeAnnotation: &ast.TypeAnnotation{
						IsResource: false,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "",
								Pos:        ast.Position{Line: 1, Column: 9, Offset: 9},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 9, Offset: 9},
					},
					FunctionBlock: &ast.FunctionBlock{
						Block: &ast.Block{
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
								EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
							},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("without return type, pub", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("pub fun foo () { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.FunctionDeclaration{
					Access: ast.AccessPublic,
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
					},
					ParameterList: &ast.ParameterList{
						Parameters: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
							EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
						},
					},
					ReturnTypeAnnotation: &ast.TypeAnnotation{
						IsResource: false,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "",
								Pos:        ast.Position{Line: 1, Column: 13, Offset: 13},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 13, Offset: 13},
					},
					FunctionBlock: &ast.FunctionBlock{
						Block: &ast.Block{
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 15, Offset: 15},
								EndPos:   ast.Position{Line: 1, Column: 17, Offset: 17},
							},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("with return type", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("fun foo (): X { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.FunctionDeclaration{
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					ParameterList: &ast.ParameterList{
						Parameters: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					ReturnTypeAnnotation: &ast.TypeAnnotation{
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "X",
								Pos:        ast.Position{Line: 1, Column: 12, Offset: 12},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 12, Offset: 12},
					},
					FunctionBlock: &ast.FunctionBlock{
						Block: &ast.Block{
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 14, Offset: 14},
								EndPos:   ast.Position{Line: 1, Column: 16, Offset: 16},
							},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("without return type, with pre and post conditions", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements(`
          fun foo () {
              pre {
                 true : "test"
                 2 > 1 : "foo"
              }

              post {
                 false
              }

              bar()
          }
        `)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.FunctionDeclaration{
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 2, Column: 14, Offset: 15},
					},
					ParameterList: &ast.ParameterList{
						Parameters: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 2, Column: 18, Offset: 19},
							EndPos:   ast.Position{Line: 2, Column: 19, Offset: 20},
						},
					},
					ReturnTypeAnnotation: &ast.TypeAnnotation{
						IsResource: false,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "",
								Pos:        ast.Position{Line: 2, Column: 19, Offset: 20},
							},
						},
						StartPos: ast.Position{Line: 2, Column: 19, Offset: 20},
					},
					FunctionBlock: &ast.FunctionBlock{
						PreConditions: &ast.Conditions{
							{
								Kind: ast.ConditionKindPre,
								Test: &ast.BoolExpression{
									Value: true,
									Range: ast.Range{
										StartPos: ast.Position{Line: 4, Column: 17, Offset: 61},
										EndPos:   ast.Position{Line: 4, Column: 20, Offset: 64},
									},
								},
								Message: &ast.StringExpression{
									Value: "test",
									Range: ast.Range{
										StartPos: ast.Position{Line: 4, Column: 24, Offset: 68},
										EndPos:   ast.Position{Line: 4, Column: 29, Offset: 73},
									},
								},
							},
							{
								Kind: ast.ConditionKindPre,
								Test: &ast.BinaryExpression{
									Operation: ast.OperationGreater,
									Left: &ast.IntegerExpression{
										Value: big.NewInt(2),
										Base:  10,
										Range: ast.Range{
											StartPos: ast.Position{Line: 5, Column: 17, Offset: 92},
											EndPos:   ast.Position{Line: 5, Column: 17, Offset: 92},
										},
									},
									Right: &ast.IntegerExpression{
										Value: big.NewInt(1),
										Base:  10,
										Range: ast.Range{
											StartPos: ast.Position{Line: 5, Column: 21, Offset: 96},
											EndPos:   ast.Position{Line: 5, Column: 21, Offset: 96},
										},
									},
								},
								Message: &ast.StringExpression{
									Value: "foo",
									Range: ast.Range{
										StartPos: ast.Position{Line: 5, Column: 25, Offset: 100},
										EndPos:   ast.Position{Line: 5, Column: 29, Offset: 104},
									},
								},
							},
						},
						PostConditions: &ast.Conditions{
							{
								Kind: ast.ConditionKindPost,
								Test: &ast.BoolExpression{
									Value: false,
									Range: ast.Range{
										StartPos: ast.Position{Line: 9, Column: 17, Offset: 161},
										EndPos:   ast.Position{Line: 9, Column: 21, Offset: 165},
									},
								},
								Message: nil,
							},
						},
						Block: &ast.Block{
							Statements: []ast.Statement{
								&ast.ExpressionStatement{
									Expression: &ast.InvocationExpression{
										InvokedExpression: &ast.IdentifierExpression{
											Identifier: ast.Identifier{
												Identifier: "bar",
												Pos:        ast.Position{Line: 12, Column: 14, Offset: 198},
											},
										},
										EndPos: ast.Position{Line: 12, Column: 18, Offset: 202},
									},
								},
							},
							Range: ast.Range{
								StartPos: ast.Position{Line: 2, Column: 21, Offset: 22},
								EndPos:   ast.Position{Line: 13, Column: 10, Offset: 214},
							},
						},
					},
					StartPos: ast.Position{Line: 2, Column: 10, Offset: 11},
				},
			},
			result,
		)
	})
}

func TestParseAccess(t *testing.T) {

	t.Parallel()

	parse := func(input string) (interface{}, []error) {
		return Parse(
			input,
			func(p *parser) interface{} {
				return parseAccess(p)
			},
		)
	}

	t.Run("pub", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("pub")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessPublic,
			result,
		)
	})

	t.Run("pub(set)", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("pub ( set )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessPublicSettable,
			result,
		)
	})

	t.Run("pub, missing set keyword", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("pub ( ")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected keyword \"set\", got EOF"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})

	t.Run("pub, missing closing paren", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("pub ( set ")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected token ')'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})

	t.Run("pub, invalid inner keyword", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("pub ( foo )")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected keyword \"set\", got \"foo\""),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})

	t.Run("priv", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("priv")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessPrivate,
			result,
		)
	})

	t.Run("access(all)", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( all )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessPublic,
			result,
		)
	})

	t.Run("access(account)", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( account )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessAccount,
			result,
		)
	})

	t.Run("access(contract)", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( contract )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessContract,
			result,
		)
	})

	t.Run("access(self)", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( self )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.AccessPrivate,
			result,
		)
	})

	t.Run("access, missing keyword", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( ")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected keyword \"all\", \"account\", \"contract\", or \"self\", got EOF"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})

	t.Run("access, missing closing paren", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( self ")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected token ')'"),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})

	t.Run("access, invalid inner keyword", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("access ( foo )")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected keyword \"all\", \"account\", \"contract\", or \"self\", got \"foo\""),
			},
			errs,
		)

		utils.AssertEqualWithDiff(t,
			nil,
			result,
		)
	})
}

func TestParseImportDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("no identifiers, missing location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import`)
		require.Equal(t,
			[]error{
				errors.New("unexpected end in import declaration: expected string, address, or identifier"),
			},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})

	t.Run("no identifiers, string location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import "foo"`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.ImportDeclaration{
					Identifiers: nil,
					Location:    ast.StringLocation("foo"),
					LocationPos: ast.Position{Line: 1, Column: 8, Offset: 8},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
					},
				},
			},
			result,
		)
	})

	t.Run("no identifiers, address location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import 0x42`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.ImportDeclaration{
					Identifiers: nil,
					Location:    ast.AddressLocation{0x42},
					LocationPos: ast.Position{Line: 1, Column: 8, Offset: 8},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
			},
			result,
		)
	})

	t.Run("no identifiers, integer location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import 1`)
		require.Equal(t,
			[]error{
				errors.New(
					"unexpected token in import declaration: " +
						"got decimal integer, expected string, address, or identifier",
				),
			},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)

	})

	t.Run("one identifier, string location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import foo from "bar"`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.ImportDeclaration{
					Identifiers: []ast.Identifier{
						{
							Identifier: "foo",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					Location:    ast.StringLocation("bar"),
					LocationPos: ast.Position{Line: 1, Column: 17, Offset: 17},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 21, Offset: 21},
					},
				},
			},
			result,
		)
	})

	t.Run("one identifier, string location, missing from keyword", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import foo "bar"`)
		require.Equal(t,
			[]error{
				errors.New(
					"unexpected token in import declaration: " +
						"got string, expected keyword \"from\" or ','",
				),
			},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})

	t.Run("three identifiers, address location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import foo , bar , baz from 0x42`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.ImportDeclaration{
					Identifiers: []ast.Identifier{
						{
							Identifier: "foo",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
						{
							Identifier: "bar",
							Pos:        ast.Position{Line: 1, Column: 14, Offset: 14},
						},
						{
							Identifier: "baz",
							Pos:        ast.Position{Line: 1, Column: 20, Offset: 20},
						},
					},
					Location:    ast.AddressLocation{0x42},
					LocationPos: ast.Position{Line: 1, Column: 29, Offset: 29},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 32, Offset: 32},
					},
				},
			},
			result,
		)
	})

	t.Run("two identifiers, address location, extra comma", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import foo , bar , from 0x42`)
		require.Equal(t,
			[]error{
				errors.New(`expected identifier, got keyword "from"`),
			},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})

	t.Run("no identifiers, identifier location", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(` import foo`)
		require.Equal(t,
			[]error{errors.New("identifier imports are not supported yet")},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})
}

func TestParseEvent(t *testing.T) {

	t.Parallel()

	t.Run("no parameters", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("event E()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{

				&ast.CompositeDeclaration{
					CompositeKind: common.CompositeKindEvent,
					Identifier: ast.Identifier{
						Identifier: "E",
						Pos:        ast.Position{Offset: 6, Line: 1, Column: 6},
					},
					Members: &ast.Members{
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								Kind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
									ParameterList: &ast.ParameterList{
										Range: ast.Range{
											StartPos: ast.Position{Offset: 7, Line: 1, Column: 7},
											EndPos:   ast.Position{Offset: 8, Line: 1, Column: 8},
										},
									},
									StartPos: ast.Position{Offset: 7, Line: 1, Column: 7},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Offset: 0, Line: 1, Column: 0},
						EndPos:   ast.Position{Offset: 8, Line: 1, Column: 8},
					},
				},
			},
			result,
		)
	})

	t.Run("two parameters, private", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" priv event E2 ( a : Int , b : String )")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{

				&ast.CompositeDeclaration{
					Access:        ast.AccessPrivate,
					CompositeKind: common.CompositeKindEvent,
					Identifier: ast.Identifier{
						Identifier: "E2",
						Pos:        ast.Position{Offset: 12, Line: 1, Column: 12},
					},
					Members: &ast.Members{
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								Kind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
									ParameterList: &ast.ParameterList{
										Parameters: []*ast.Parameter{
											{
												Label: "",
												Identifier: ast.Identifier{
													Identifier: "a",
													Pos:        ast.Position{Offset: 17, Line: 1, Column: 17},
												},
												TypeAnnotation: &ast.TypeAnnotation{
													IsResource: false,
													Type: &ast.NominalType{
														Identifier: ast.Identifier{
															Identifier: "Int",
															Pos:        ast.Position{Offset: 21, Line: 1, Column: 21},
														},
													},
													StartPos: ast.Position{Offset: 21, Line: 1, Column: 21},
												},
												Range: ast.Range{
													StartPos: ast.Position{Offset: 17, Line: 1, Column: 17},
													EndPos:   ast.Position{Offset: 23, Line: 1, Column: 23},
												},
											},
											{
												Label: "",
												Identifier: ast.Identifier{
													Identifier: "b",
													Pos:        ast.Position{Offset: 27, Line: 1, Column: 27},
												},
												TypeAnnotation: &ast.TypeAnnotation{
													IsResource: false,
													Type: &ast.NominalType{
														Identifier: ast.Identifier{
															Identifier: "String",
															Pos:        ast.Position{Offset: 31, Line: 1, Column: 31},
														},
													},
													StartPos: ast.Position{Offset: 31, Line: 1, Column: 31},
												},
												Range: ast.Range{
													StartPos: ast.Position{Offset: 27, Line: 1, Column: 27},
													EndPos:   ast.Position{Offset: 36, Line: 1, Column: 36},
												},
											},
										},
										Range: ast.Range{
											StartPos: ast.Position{Offset: 15, Line: 1, Column: 15},
											EndPos:   ast.Position{Offset: 38, Line: 1, Column: 38},
										},
									},
									StartPos: ast.Position{Offset: 15, Line: 1, Column: 15},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Offset: 1, Line: 1, Column: 1},
						EndPos:   ast.Position{Offset: 38, Line: 1, Column: 38},
					},
				},
			},
			result,
		)
	})
}

func TestParseFieldWithVariableKind(t *testing.T) {

	t.Parallel()

	parse := func(input string) (interface{}, []error) {
		return Parse(
			input,
			func(p *parser) interface{} {
				return parseFieldWithVariableKind(p, ast.AccessNotSpecified, nil)
			},
		)
	}

	t.Run("variable", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("var x : Int")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FieldDeclaration{
				Access:       ast.AccessNotSpecified,
				VariableKind: ast.VariableKindVariable,
				Identifier: ast.Identifier{
					Identifier: "x",
					Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
				},
				TypeAnnotation: &ast.TypeAnnotation{
					IsResource: false,
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "Int",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
				},
			},
			result,
		)
	})

	t.Run("constant", func(t *testing.T) {

		t.Parallel()

		result, errs := parse("let x : Int")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			&ast.FieldDeclaration{
				Access:       ast.AccessNotSpecified,
				VariableKind: ast.VariableKindConstant,
				Identifier: ast.Identifier{
					Identifier: "x",
					Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
				},
				TypeAnnotation: &ast.TypeAnnotation{
					IsResource: false,
					Type: &ast.NominalType{
						Identifier: ast.Identifier{
							Identifier: "Int",
							Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
				},
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
				},
			},
			result,
		)
	})
}

func TestParseCompositeDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("struct, no conformances", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" pub struct S { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.CompositeDeclaration{
					Access:        ast.AccessPublic,
					CompositeKind: common.CompositeKindStructure,
					Identifier: ast.Identifier{
						Identifier: "S",
						Pos:        ast.Position{Line: 1, Column: 12, Offset: 12},
					},
					Members: &ast.Members{},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 16, Offset: 16},
					},
				},
			},
			result,
		)
	})

	t.Run("resource, one conformance", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" pub resource R : RI { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.CompositeDeclaration{
					Access:        ast.AccessPublic,
					CompositeKind: common.CompositeKindResource,
					Identifier: ast.Identifier{
						Identifier: "R",
						Pos:        ast.Position{Line: 1, Column: 14, Offset: 14},
					},
					Conformances: []*ast.NominalType{
						{
							Identifier: ast.Identifier{
								Identifier: "RI",
								Pos:        ast.Position{Line: 1, Column: 18, Offset: 18},
							},
						},
					},
					Members: &ast.Members{},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 23, Offset: 23},
					},
				},
			},
			result,
		)
	})

	t.Run("struct, with fields, functions, and special functions", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(`
          struct Test {
              pub(set) var foo: Int

              init(foo: Int) {
                  self.foo = foo
              }

              pub fun getFoo(): Int {
                  return self.foo
              }
          }
	    `)

		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.CompositeDeclaration{
					CompositeKind: common.CompositeKindStructure,
					Identifier: ast.Identifier{
						Identifier: "Test",
						Pos:        ast.Position{Offset: 18, Line: 2, Column: 17},
					},
					Members: &ast.Members{
						Fields: []*ast.FieldDeclaration{
							{
								Access:       ast.AccessPublicSettable,
								VariableKind: ast.VariableKindVariable,
								Identifier: ast.Identifier{
									Identifier: "foo",
									Pos:        ast.Position{Offset: 52, Line: 3, Column: 27},
								},
								TypeAnnotation: &ast.TypeAnnotation{
									IsResource: false,
									Type: &ast.NominalType{
										Identifier: ast.Identifier{
											Identifier: "Int",
											Pos:        ast.Position{Offset: 57, Line: 3, Column: 32},
										},
									},
									StartPos: ast.Position{Offset: 57, Line: 3, Column: 32},
								},
								Range: ast.Range{
									StartPos: ast.Position{Offset: 39, Line: 3, Column: 14},
									EndPos:   ast.Position{Offset: 59, Line: 3, Column: 34},
								},
							},
						},
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								Kind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
									Identifier: ast.Identifier{
										Identifier: "init",
										Pos:        ast.Position{Offset: 76, Line: 5, Column: 14},
									},
									ParameterList: &ast.ParameterList{
										Parameters: []*ast.Parameter{
											{
												Label: "",
												Identifier: ast.Identifier{
													Identifier: "foo",
													Pos:        ast.Position{Offset: 81, Line: 5, Column: 19},
												},
												TypeAnnotation: &ast.TypeAnnotation{
													IsResource: false,
													Type: &ast.NominalType{
														Identifier: ast.Identifier{
															Identifier: "Int",
															Pos:        ast.Position{Offset: 86, Line: 5, Column: 24},
														},
													},
													StartPos: ast.Position{Offset: 86, Line: 5, Column: 24},
												},
												Range: ast.Range{
													StartPos: ast.Position{Offset: 81, Line: 5, Column: 19},
													EndPos:   ast.Position{Offset: 88, Line: 5, Column: 26},
												},
											},
										},
										Range: ast.Range{
											StartPos: ast.Position{Offset: 80, Line: 5, Column: 18},
											EndPos:   ast.Position{Offset: 89, Line: 5, Column: 27},
										},
									},
									FunctionBlock: &ast.FunctionBlock{
										Block: &ast.Block{
											Statements: []ast.Statement{
												&ast.AssignmentStatement{
													Target: &ast.MemberExpression{
														Expression: &ast.IdentifierExpression{
															Identifier: ast.Identifier{
																Identifier: "self",
																Pos:        ast.Position{Offset: 111, Line: 6, Column: 18},
															},
														},
														Identifier: ast.Identifier{
															Identifier: "foo",
															Pos:        ast.Position{Offset: 116, Line: 6, Column: 23},
														},
													},
													Transfer: &ast.Transfer{
														Operation: ast.TransferOperationCopy,
														Pos:       ast.Position{Offset: 120, Line: 6, Column: 27},
													},
													Value: &ast.IdentifierExpression{
														Identifier: ast.Identifier{
															Identifier: "foo",
															Pos:        ast.Position{Offset: 122, Line: 6, Column: 29},
														},
													},
												},
											},
											Range: ast.Range{
												StartPos: ast.Position{Offset: 91, Line: 5, Column: 29},
												EndPos:   ast.Position{Offset: 140, Line: 7, Column: 14},
											},
										},
									},
									StartPos: ast.Position{Offset: 76, Line: 5, Column: 14},
								},
							},
						},
						Functions: []*ast.FunctionDeclaration{
							{
								Access: ast.AccessPublic,
								Identifier: ast.Identifier{
									Identifier: "getFoo",
									Pos:        ast.Position{Offset: 165, Line: 9, Column: 22},
								},
								ParameterList: &ast.ParameterList{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 171, Line: 9, Column: 28},
										EndPos:   ast.Position{Offset: 172, Line: 9, Column: 29},
									},
								},
								ReturnTypeAnnotation: &ast.TypeAnnotation{
									IsResource: false,
									Type: &ast.NominalType{
										Identifier: ast.Identifier{
											Identifier: "Int",
											Pos:        ast.Position{Offset: 175, Line: 9, Column: 32},
										},
									},
									StartPos: ast.Position{Offset: 175, Line: 9, Column: 32},
								},
								FunctionBlock: &ast.FunctionBlock{
									Block: &ast.Block{
										Statements: []ast.Statement{
											&ast.ReturnStatement{
												Expression: &ast.MemberExpression{
													Expression: &ast.IdentifierExpression{
														Identifier: ast.Identifier{
															Identifier: "self",
															Pos:        ast.Position{Offset: 206, Line: 10, Column: 25},
														},
													},
													Identifier: ast.Identifier{
														Identifier: "foo",
														Pos:        ast.Position{Offset: 211, Line: 10, Column: 30},
													},
												},
												Range: ast.Range{
													StartPos: ast.Position{Offset: 199, Line: 10, Column: 18},
													EndPos:   ast.Position{Offset: 213, Line: 10, Column: 32},
												},
											},
										},
										Range: ast.Range{
											StartPos: ast.Position{Offset: 179, Line: 9, Column: 36},
											EndPos:   ast.Position{Offset: 229, Line: 11, Column: 14},
										},
									},
								},
								StartPos: ast.Position{Offset: 157, Line: 9, Column: 14},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Offset: 11, Line: 2, Column: 10},
						EndPos:   ast.Position{Offset: 241, Line: 12, Column: 10},
					},
				},
			},
			result,
		)
	})
}

func TestParseInterfaceDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("struct, no conformances", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" pub struct interface S { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.InterfaceDeclaration{
					Access:        ast.AccessPublic,
					CompositeKind: common.CompositeKindStructure,
					Identifier: ast.Identifier{
						Identifier: "S",
						Pos:        ast.Position{Line: 1, Column: 22, Offset: 22},
					},
					Members: &ast.Members{},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 1, Offset: 1},
						EndPos:   ast.Position{Line: 1, Column: 26, Offset: 26},
					},
				},
			},
			result,
		)
	})

	t.Run("struct, interface keyword as name", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(" pub struct interface interface { }")
		require.Equal(t,
			[]error{
				fmt.Errorf("expected interface name, got keyword \"interface\""),
			},
			errs,
		)

		var expected []ast.Declaration

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})

	t.Run("struct, with fields, functions, and special functions; with and without blocks", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations(`
          struct interface Test {
              pub(set) var foo: Int

              init(foo: Int)

              pub fun getFoo(): Int

              pub fun getBar(): Int {}

              destroy() {}
          }
	    `)

		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.InterfaceDeclaration{
					CompositeKind: common.CompositeKindStructure,
					Identifier: ast.Identifier{
						Identifier: "Test",
						Pos:        ast.Position{Offset: 28, Line: 2, Column: 27},
					},
					Members: &ast.Members{
						Fields: []*ast.FieldDeclaration{
							{
								Access:       ast.AccessPublicSettable,
								VariableKind: ast.VariableKindVariable,
								Identifier: ast.Identifier{
									Identifier: "foo",
									Pos:        ast.Position{Offset: 62, Line: 3, Column: 27},
								},
								TypeAnnotation: &ast.TypeAnnotation{
									IsResource: false,
									Type: &ast.NominalType{
										Identifier: ast.Identifier{
											Identifier: "Int",
											Pos:        ast.Position{Offset: 67, Line: 3, Column: 32},
										},
									},
									StartPos: ast.Position{Offset: 67, Line: 3, Column: 32},
								},
								Range: ast.Range{
									StartPos: ast.Position{Offset: 49, Line: 3, Column: 14},
									EndPos:   ast.Position{Offset: 69, Line: 3, Column: 34},
								},
							},
						},
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								Kind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
									Identifier: ast.Identifier{
										Identifier: "init",
										Pos:        ast.Position{Offset: 86, Line: 5, Column: 14},
									},
									ParameterList: &ast.ParameterList{
										Parameters: []*ast.Parameter{
											{
												Label: "",
												Identifier: ast.Identifier{
													Identifier: "foo",
													Pos:        ast.Position{Offset: 91, Line: 5, Column: 19},
												},
												TypeAnnotation: &ast.TypeAnnotation{
													IsResource: false,
													Type: &ast.NominalType{
														Identifier: ast.Identifier{
															Identifier: "Int",
															Pos:        ast.Position{Offset: 96, Line: 5, Column: 24},
														},
													},
													StartPos: ast.Position{Offset: 96, Line: 5, Column: 24},
												},
												Range: ast.Range{
													StartPos: ast.Position{Offset: 91, Line: 5, Column: 19},
													EndPos:   ast.Position{Offset: 98, Line: 5, Column: 26},
												},
											},
										},
										Range: ast.Range{
											StartPos: ast.Position{Offset: 90, Line: 5, Column: 18},
											EndPos:   ast.Position{Offset: 99, Line: 5, Column: 27},
										},
									},
									StartPos: ast.Position{Offset: 86, Line: 5, Column: 14},
								},
							},
							{
								Kind: common.DeclarationKindDestructor,
								FunctionDeclaration: &ast.FunctionDeclaration{
									Identifier: ast.Identifier{
										Identifier: "destroy",
										Pos:        ast.Position{Offset: 193, Line: 11, Column: 14},
									},
									ParameterList: &ast.ParameterList{
										Range: ast.Range{
											StartPos: ast.Position{Offset: 200, Line: 11, Column: 21},
											EndPos:   ast.Position{Offset: 201, Line: 11, Column: 22},
										},
									},
									FunctionBlock: &ast.FunctionBlock{
										Block: &ast.Block{
											Range: ast.Range{
												StartPos: ast.Position{Offset: 203, Line: 11, Column: 24},
												EndPos:   ast.Position{Offset: 204, Line: 11, Column: 25},
											},
										},
									},
									StartPos: ast.Position{Offset: 193, Line: 11, Column: 14},
								},
							},
						},
						Functions: []*ast.FunctionDeclaration{
							{
								Access: ast.AccessPublic,
								Identifier: ast.Identifier{
									Identifier: "getFoo",
									Pos:        ast.Position{Offset: 124, Line: 7, Column: 22},
								},
								ParameterList: &ast.ParameterList{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 130, Line: 7, Column: 28},
										EndPos:   ast.Position{Offset: 131, Line: 7, Column: 29},
									},
								},
								ReturnTypeAnnotation: &ast.TypeAnnotation{
									IsResource: false,
									Type: &ast.NominalType{
										Identifier: ast.Identifier{
											Identifier: "Int",
											Pos:        ast.Position{Offset: 134, Line: 7, Column: 32},
										},
									},
									StartPos: ast.Position{Offset: 134, Line: 7, Column: 32},
								},
								StartPos: ast.Position{Offset: 116, Line: 7, Column: 14},
							},
							{
								Access: ast.AccessPublic,
								Identifier: ast.Identifier{
									Identifier: "getBar",
									Pos:        ast.Position{Offset: 161, Line: 9, Column: 22},
								},
								ParameterList: &ast.ParameterList{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 167, Line: 9, Column: 28},
										EndPos:   ast.Position{Offset: 168, Line: 9, Column: 29},
									},
								},
								ReturnTypeAnnotation: &ast.TypeAnnotation{
									IsResource: false,
									Type: &ast.NominalType{
										Identifier: ast.Identifier{
											Identifier: "Int",
											Pos:        ast.Position{Offset: 171, Line: 9, Column: 32},
										},
									},
									StartPos: ast.Position{Offset: 171, Line: 9, Column: 32},
								},
								FunctionBlock: &ast.FunctionBlock{
									Block: &ast.Block{
										Range: ast.Range{
											StartPos: ast.Position{Offset: 175, Line: 9, Column: 36},
											EndPos:   ast.Position{Offset: 176, Line: 9, Column: 37},
										},
									},
								},
								StartPos: ast.Position{Offset: 153, Line: 9, Column: 14},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Offset: 11, Line: 2, Column: 10},
						EndPos:   ast.Position{Offset: 216, Line: 12, Column: 10},
					},
				},
			},
			result,
		)
	})
}

func TestParseTransactionDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("no prepare, execute", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseDeclarations("transaction { execute {} }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Declaration{
				&ast.TransactionDeclaration{
					Execute: &ast.SpecialFunctionDeclaration{
						Kind: common.DeclarationKindExecute,
						FunctionDeclaration: &ast.FunctionDeclaration{
							Access: ast.AccessNotSpecified,
							Identifier: ast.Identifier{
								Identifier: "execute",
								Pos:        ast.Position{Offset: 14, Line: 1, Column: 14},
							},
							ParameterList: &ast.ParameterList{},
							FunctionBlock: &ast.FunctionBlock{
								Block: &ast.Block{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 22, Line: 1, Column: 22},
										EndPos:   ast.Position{Offset: 23, Line: 1, Column: 23},
									},
								},
							},
							StartPos: ast.Position{Offset: 14, Line: 1, Column: 14},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 25, Offset: 25},
					},
				},
			},
			result,
		)
	})
}

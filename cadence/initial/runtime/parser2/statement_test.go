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
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func TestParseReturnStatement(t *testing.T) {

	t.Parallel()

	t.Run("no expression", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("return")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ReturnStatement{
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
			},
			result,
		)
	})

	t.Run("expression on same line", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("return 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ReturnStatement{
					Expression: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
							EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
					},
				},
			},
			result,
		)
	})

	t.Run("expression on next line, no semicolon", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("return \n1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ReturnStatement{
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				&ast.ExpressionStatement{
					Expression: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 2, Column: 0, Offset: 8},
							EndPos:   ast.Position{Line: 2, Column: 0, Offset: 8},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("expression on next line, semicolon", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("return ;\n1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ReturnStatement{
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
						EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
					},
				},
				&ast.ExpressionStatement{
					Expression: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 2, Column: 0, Offset: 9},
							EndPos:   ast.Position{Line: 2, Column: 0, Offset: 9},
						},
					},
				},
			},
			result,
		)
	})
}

func TestParseIfStatement(t *testing.T) {

	t.Parallel()

	t.Run("only empty then", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if true { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.IfStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					Then: &ast.Block{
						Statements: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("only then, two statements on one line", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if true { 1 ; 2 }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.IfStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					Then: &ast.Block{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
										EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
									},
								},
							},
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 14, Offset: 14},
										EndPos:   ast.Position{Line: 1, Column: 14, Offset: 14},
									},
								},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 16, Offset: 16},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("only then, two statements on multiple lines", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if true { 1 \n 2 }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.IfStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					Then: &ast.Block{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
										EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
									},
								},
							},
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 2, Column: 1, Offset: 14},
										EndPos:   ast.Position{Line: 2, Column: 1, Offset: 14},
									},
								},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 2, Column: 3, Offset: 16},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("with else", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if true { 1 } else { 2 }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.IfStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					Then: &ast.Block{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
										EndPos:   ast.Position{Line: 1, Column: 10, Offset: 10},
									},
								},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
							EndPos:   ast.Position{Line: 1, Column: 12, Offset: 12},
						},
					},
					Else: &ast.Block{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(2),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 21, Offset: 21},
										EndPos:   ast.Position{Line: 1, Column: 21, Offset: 21},
									},
								},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 19, Offset: 19},
							EndPos:   ast.Position{Line: 1, Column: 23, Offset: 23},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("with else if and else, no space", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if true{1}else if true {2} else{3}")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.IfStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
					Then: &ast.Block{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.IntegerExpression{
									Value: big.NewInt(1),
									Base:  10,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
										EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
									},
								},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					Else: &ast.Block{
						Statements: []ast.Statement{
							&ast.IfStatement{
								Test: &ast.BoolExpression{
									Value: true,
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 18, Offset: 18},
										EndPos:   ast.Position{Line: 1, Column: 21, Offset: 21},
									},
								},
								Then: &ast.Block{
									Statements: []ast.Statement{
										&ast.ExpressionStatement{
											Expression: &ast.IntegerExpression{
												Value: big.NewInt(2),
												Base:  10,
												Range: ast.Range{
													StartPos: ast.Position{Line: 1, Column: 24, Offset: 24},
													EndPos:   ast.Position{Line: 1, Column: 24, Offset: 24},
												},
											},
										},
									},
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 23, Offset: 23},
										EndPos:   ast.Position{Line: 1, Column: 25, Offset: 25},
									},
								},
								Else: &ast.Block{
									Statements: []ast.Statement{
										&ast.ExpressionStatement{
											Expression: &ast.IntegerExpression{
												Value: big.NewInt(3),
												Base:  10,
												Range: ast.Range{
													StartPos: ast.Position{Line: 1, Column: 32, Offset: 32},
													EndPos:   ast.Position{Line: 1, Column: 32, Offset: 32},
												},
											},
										},
									},
									Range: ast.Range{
										StartPos: ast.Position{Line: 1, Column: 31, Offset: 31},
										EndPos:   ast.Position{Line: 1, Column: 33, Offset: 33},
									},
								},
								StartPos: ast.Position{Line: 1, Column: 15, Offset: 15},
							},
						},
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 15, Offset: 15},
							EndPos:   ast.Position{Line: 1, Column: 33, Offset: 33},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("if-var", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if var x = 1 { }")
		require.Empty(t, errs)

		expected := &ast.IfStatement{
			Test: &ast.VariableDeclaration{
				IsConstant: false,
				Identifier: ast.Identifier{
					Identifier: "x",
					Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
				},
				Value: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				Transfer: &ast.Transfer{
					Operation: ast.TransferOperationCopy,
					Pos:       ast.Position{Line: 1, Column: 9, Offset: 9},
				},
				StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
			},
			Then: &ast.Block{
				Statements: nil,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 13, Offset: 13},
					EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
				},
			},
			StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
		}

		expected.Test.(*ast.VariableDeclaration).ParentIfStatement = expected

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				expected,
			},
			result,
		)
	})

	t.Run("if-let", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("if let x = 1 { }")
		require.Empty(t, errs)

		expected := &ast.IfStatement{
			Test: &ast.VariableDeclaration{
				IsConstant: true,
				Identifier: ast.Identifier{
					Identifier: "x",
					Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
				},
				Value: &ast.IntegerExpression{
					Value: big.NewInt(1),
					Base:  10,
					Range: ast.Range{
						StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
						EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
					},
				},
				Transfer: &ast.Transfer{
					Operation: ast.TransferOperationCopy,
					Pos:       ast.Position{Line: 1, Column: 9, Offset: 9},
				},
				StartPos: ast.Position{Line: 1, Column: 3, Offset: 3},
			},
			Then: &ast.Block{
				Statements: nil,
				Range: ast.Range{
					StartPos: ast.Position{Line: 1, Column: 13, Offset: 13},
					EndPos:   ast.Position{Line: 1, Column: 15, Offset: 15},
				},
			},
			StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
		}

		expected.Test.(*ast.VariableDeclaration).ParentIfStatement = expected

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				expected,
			},
			result,
		)
	})

}

func TestParseWhileStatement(t *testing.T) {

	t.Parallel()

	t.Run("empty block", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("while true { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.WhileStatement{
					Test: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
							EndPos:   ast.Position{Line: 1, Column: 9, Offset: 9},
						},
					},
					Block: &ast.Block{
						Statements: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
							EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})
}

func TestParseAssignmentStatement(t *testing.T) {

	t.Parallel()

	t.Run("copy", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements(" x = 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.AssignmentStatement{
					Target: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "x",
							Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationCopy,
						Pos:       ast.Position{Line: 1, Column: 3, Offset: 3},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 5, Offset: 5},
							EndPos:   ast.Position{Line: 1, Column: 5, Offset: 5},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("move", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements(" x <- 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.AssignmentStatement{
					Target: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "x",
							Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationMove,
						Pos:       ast.Position{Line: 1, Column: 3, Offset: 3},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 6, Offset: 6},
							EndPos:   ast.Position{Line: 1, Column: 6, Offset: 6},
						},
					},
				},
			},
			result,
		)
	})

	t.Run("force move", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements(" x <-! 1")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.AssignmentStatement{
					Target: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "x",
							Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
						},
					},
					Transfer: &ast.Transfer{
						Operation: ast.TransferOperationMoveForced,
						Pos:       ast.Position{Line: 1, Column: 3, Offset: 3},
					},
					Value: &ast.IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
							EndPos:   ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
				},
			},
			result,
		)
	})
}

func TestParseSwapStatement(t *testing.T) {

	t.Parallel()

	t.Run("simple", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements(" x <-> y")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.SwapStatement{
					Left: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "x",
							Pos:        ast.Position{Line: 1, Column: 1, Offset: 1},
						},
					},
					Right: &ast.IdentifierExpression{
						Identifier: ast.Identifier{
							Identifier: "y",
							Pos:        ast.Position{Line: 1, Column: 7, Offset: 7},
						},
					},
				},
			},
			result,
		)
	})
}

func TestParseForStatement(t *testing.T) {

	t.Parallel()

	t.Run("empty block", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("for x in y { }")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ForStatement{
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
					Block: &ast.Block{
						Statements: nil,
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 11, Offset: 11},
							EndPos:   ast.Position{Line: 1, Column: 13, Offset: 13},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})
}

func TestParseEmit(t *testing.T) {

	t.Parallel()

	t.Run("simple", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("emit T()")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.EmitStatement{
					InvocationExpression: &ast.InvocationExpression{
						InvokedExpression: &ast.IdentifierExpression{
							Identifier: ast.Identifier{
								Identifier: "T",
								Pos:        ast.Position{Line: 1, Column: 5, Offset: 5},
							},
						},
						EndPos: ast.Position{Line: 1, Column: 7, Offset: 7},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})
}

func TestParseFunctionStatementOrExpression(t *testing.T) {

	t.Parallel()

	t.Run("function declaration with name", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("fun foo() {}")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.FunctionDeclaration{
					Access: ast.AccessNotSpecified,
					Identifier: ast.Identifier{
						Identifier: "foo",
						Pos:        ast.Position{Line: 1, Column: 4, Offset: 4},
					},
					ParameterList: &ast.ParameterList{
						Range: ast.Range{
							StartPos: ast.Position{Line: 1, Column: 7, Offset: 7},
							EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
						},
					},
					ReturnTypeAnnotation: &ast.TypeAnnotation{
						IsResource: false,
						Type: &ast.NominalType{
							Identifier: ast.Identifier{
								Identifier: "",
								Pos:        ast.Position{Line: 1, Column: 8, Offset: 8},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 8, Offset: 8},
					},
					FunctionBlock: &ast.FunctionBlock{
						Block: &ast.Block{
							Range: ast.Range{
								StartPos: ast.Position{Line: 1, Column: 10, Offset: 10},
								EndPos:   ast.Position{Line: 1, Column: 11, Offset: 11},
							},
						},
					},
					StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
				},
			},
			result,
		)
	})

	t.Run("function expression without name", func(t *testing.T) {

		t.Parallel()

		result, errs := ParseStatements("fun () {}")
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			[]ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.FunctionExpression{
						ParameterList: &ast.ParameterList{
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
									EndPos:   ast.Position{Line: 1, Column: 8, Offset: 8},
								},
							},
						},
						StartPos: ast.Position{Line: 1, Column: 0, Offset: 0},
					},
				},
			},
			result,
		)
	})
}

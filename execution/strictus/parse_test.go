package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"math/big"
	"testing"
)

func init() {
	format.TruncatedDiff = false
	format.MaxDepth = 100
}

func parse(code string) Program {
	input := antlr.NewInputStream(code)
	lexer := NewStrictusLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, 0)
	parser := NewStrictusParser(stream)
	// diagnostics, for debugging only:
	// parser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
	parser.AddErrorListener(antlr.NewConsoleErrorListener())
	return parser.Program().Accept(&ProgramVisitor{}).(Program)
}

func TestParseBoolExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = true
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: BoolExpression{
			Value:    true,
			Position: Position{Offset: 16, Line: 2, Column: 15},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 16, Line: 2, Column: 15},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIdentifierExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const b = a
	`)

	b := VariableDeclaration{
		IsConst:    true,
		Identifier: "b",
		Value: IdentifierExpression{
			Identifier: "a",
			Position:   Position{Offset: 16, Line: 2, Column: 15},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 16, Line: 2, Column: 15},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{b},
		Declarations:    map[string]Declaration{"b": b},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseArrayExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = [1, 2]
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: ArrayExpression{
			Values: []Expression{
				IntExpression{
					Value:    big.NewInt(1),
					Position: Position{Offset: 17, Line: 2, Column: 16},
				},
				IntExpression{
					Value:    big.NewInt(2),
					Position: Position{Offset: 20, Line: 2, Column: 19},
				},
			},
			StartPosition: Position{Offset: 16, Line: 2, Column: 15},
			EndPosition:   Position{Offset: 21, Line: 2, Column: 20},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 21, Line: 2, Column: 20},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseInvocationExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = b(1, 2)
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: InvocationExpression{
			Expression: IdentifierExpression{
				Identifier: "b",
				Position:   Position{Offset: 16, Line: 2, Column: 15},
			},
			Arguments: []Expression{
				IntExpression{
					Value:    big.NewInt(1),
					Position: Position{Offset: 18, Line: 2, Column: 17},
				},
				IntExpression{
					Value:    big.NewInt(2),
					Position: Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			StartPosition: Position{Offset: 17, Line: 2, Column: 16},
			EndPosition:   Position{Offset: 22, Line: 2, Column: 21},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 22, Line: 2, Column: 21},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMemberExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = b.c
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: MemberExpression{
			Expression: IdentifierExpression{
				Identifier: "b",
				Position:   Position{Offset: 16, Line: 2, Column: 15},
			},
			Identifier:    "c",
			StartPosition: Position{Offset: 17, Line: 2, Column: 16},
			EndPosition:   Position{Offset: 18, Line: 2, Column: 17},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 18, Line: 2, Column: 17},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIndexExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = b[1]
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: IndexExpression{
			Expression: IdentifierExpression{
				Identifier: "b",
				Position:   Position{Offset: 16, Line: 2, Column: 15},
			},
			Index: IntExpression{
				Value:    big.NewInt(1),
				Position: Position{Offset: 18, Line: 2, Column: 17},
			},
			StartPosition: Position{Offset: 17, Line: 2, Column: 16},
			EndPosition:   Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 19, Line: 2, Column: 18},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseUnaryExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const a = -b
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: UnaryExpression{
			Operation: OperationMinus,
			Expression: IdentifierExpression{
				Identifier: "b",
				Position:   Position{Offset: 17, Line: 2, Column: 16},
			},
			StartPosition: Position{Offset: 16, Line: 2, Column: 15},
			EndPosition:   Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 17, Line: 2, Column: 16},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{a},
		Declarations:    map[string]Declaration{"a": a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseOrExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = false || true
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationOr,
			Left: BoolExpression{
				Value:    false,
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: BoolExpression{
				Value:    true,
				Position: Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAndExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = false && true
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationAnd,
			Left: BoolExpression{
				Value:    false,
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: BoolExpression{
				Value:    true,
				Position: Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseEqualityExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = false == true
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationEqual,
			Left: BoolExpression{
				Value:    false,
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: BoolExpression{
				Value:    true,
				Position: Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseRelationalExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = 1 < 2
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationLess,
			Left: IntExpression{
				Value:    big.NewInt(1),
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: IntExpression{
				Value:    big.NewInt(2),
				Position: Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAdditiveExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = 1 + 2
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationPlus,
			Left: IntExpression{
				Value:    big.NewInt(1),
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: IntExpression{
				Value:    big.NewInt(2),
				Position: Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMultiplicativeExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = 1 * 2
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationMul,
			Left: IntExpression{
				Value:    big.NewInt(1),
				Position: Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: IntExpression{
				Value:    big.NewInt(2),
				Position: Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionExpressionAndReturn(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const test = fun (): Int { return 1 }
	`)

	test := VariableDeclaration{
		IsConst:    true,
		Identifier: "test",
		Value: FunctionExpression{
			ReturnType: BaseType{
				Identifier: "Int",
				Position:   Position{Offset: 27, Line: 2, Column: 26},
			},
			Block: Block{
				Statements: []Statement{
					ReturnStatement{
						Expression: IntExpression{
							Value:    big.NewInt(1),
							Position: Position{Offset: 40, Line: 2, Column: 39},
						},
						StartPosition: Position{Offset: 33, Line: 2, Column: 32},
						EndPosition:   Position{Offset: 40, Line: 2, Column: 39},
					},
				},
				// NOTE: block is statements *inside* curly braces
				StartPosition: Position{Offset: 33, Line: 2, Column: 32},
				EndPosition:   Position{Offset: 40, Line: 2, Column: 39},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 42, Line: 2, Column: 41},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 42, Line: 2, Column: 41},
		IdentifierPosition: Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionAndBlock(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    fun test() { return }
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				ReturnStatement{
					StartPosition: Position{Offset: 19, Line: 2, Column: 18},
					EndPosition:   Position{Offset: 19, Line: 2, Column: 18},
				},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 26, Line: 2, Column: 25},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIfStatement(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
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
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				IfStatement{
					Test: BoolExpression{
						Value:    true,
						Position: Position{Offset: 34, Line: 3, Column: 15},
					},
					Then: Block{
						Statements: []Statement{
							ReturnStatement{
								Expression:    nil,
								StartPosition: Position{Offset: 57, Line: 4, Column: 16},
								EndPosition:   Position{Offset: 57, Line: 4, Column: 16},
							},
						},
						StartPosition: Position{Offset: 57, Line: 4, Column: 16},
						EndPosition:   Position{Offset: 57, Line: 4, Column: 16},
					},
					Else: Block{
						Statements: []Statement{
							IfStatement{
								Test: BoolExpression{
									Value:    false,
									Position: Position{Offset: 86, Line: 5, Column: 22},
								},
								Then: Block{
									Statements: []Statement{
										ExpressionStatement{
											Expression: BoolExpression{
												Value:    false,
												Position: Position{Offset: 110, Line: 6, Column: 16},
											},
										},
										ExpressionStatement{
											Expression: IntExpression{
												Value:    big.NewInt(1),
												Position: Position{Offset: 132, Line: 7, Column: 16},
											},
										},
									},
									StartPosition: Position{Offset: 110, Line: 6, Column: 16},
									EndPosition:   Position{Offset: 132, Line: 7, Column: 16},
								},
								Else: Block{
									Statements: []Statement{
										ExpressionStatement{
											Expression: IntExpression{
												Value:    big.NewInt(2),
												Position: Position{Offset: 171, Line: 9, Column: 16},
											},
										},
									},
									StartPosition: Position{Offset: 171, Line: 9, Column: 16},
									EndPosition:   Position{Offset: 171, Line: 9, Column: 16},
								},
								StartPosition: Position{Offset: 83, Line: 5, Column: 19},
								EndPosition:   Position{Offset: 185, Line: 10, Column: 12},
							},
						},
						StartPosition: Position{Offset: 83, Line: 5, Column: 19},
						EndPosition:   Position{Offset: 185, Line: 10, Column: 12},
					},
					StartPosition: Position{Offset: 31, Line: 3, Column: 12},
					EndPosition:   Position{Offset: 185, Line: 10, Column: 12},
				},
			},
			StartPosition: Position{Offset: 31, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 185, Line: 10, Column: 12},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 195, Line: 11, Column: 8},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    fun test() {
            while true {
              return
            }
        }
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				WhileStatement{
					Test: BoolExpression{
						Value:    true,
						Position: Position{Offset: 37, Line: 3, Column: 18},
					},
					Block: Block{
						Statements: []Statement{
							ReturnStatement{
								Expression:    nil,
								StartPosition: Position{Offset: 58, Line: 4, Column: 14},
								EndPosition:   Position{Offset: 58, Line: 4, Column: 14},
							},
						},
						// NOTE: block is statements *inside* curly braces
						StartPosition: Position{Offset: 58, Line: 4, Column: 14},
						EndPosition:   Position{Offset: 58, Line: 4, Column: 14},
					},
					StartPosition: Position{Offset: 31, Line: 3, Column: 12},
					EndPosition:   Position{Offset: 77, Line: 5, Column: 12},
				},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 31, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 77, Line: 5, Column: 12},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 87, Line: 6, Column: 8},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    fun test() {
            a = 1
        }
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				AssignmentStatement{
					Target: IdentifierExpression{
						Identifier: "a",
						Position:   Position{Offset: 31, Line: 3, Column: 12},
					},
					Value: IntExpression{
						Value:    big.NewInt(1),
						Position: Position{Offset: 35, Line: 3, Column: 16},
					},
					StartPosition: Position{Offset: 31, Line: 3, Column: 12},
					EndPosition:   Position{Offset: 35, Line: 3, Column: 16},
				},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 31, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 35, Line: 3, Column: 16},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 45, Line: 4, Column: 8},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAccessAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    fun test() {
            x.foo.bar[0][1].baz = 1
        }
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				AssignmentStatement{
					Target: MemberExpression{
						Expression: IndexExpression{
							Expression: IndexExpression{
								Expression: MemberExpression{
									Expression: MemberExpression{
										Expression: IdentifierExpression{
											Identifier: "x",
											Position:   Position{Offset: 31, Line: 3, Column: 12},
										},
										Identifier:    "foo",
										StartPosition: Position{Offset: 32, Line: 3, Column: 13},
										EndPosition:   Position{Offset: 33, Line: 3, Column: 14},
									},
									Identifier:    "bar",
									StartPosition: Position{Offset: 36, Line: 3, Column: 17},
									EndPosition:   Position{Offset: 37, Line: 3, Column: 18},
								},
								Index: IntExpression{
									Value:    big.NewInt(0),
									Position: Position{Offset: 41, Line: 3, Column: 22},
								},
								StartPosition: Position{Offset: 40, Line: 3, Column: 21},
								EndPosition:   Position{Offset: 42, Line: 3, Column: 23},
							},
							Index: IntExpression{
								Value:    big.NewInt(1),
								Position: Position{Offset: 44, Line: 3, Column: 25},
							},
							StartPosition: Position{Offset: 43, Line: 3, Column: 24},
							EndPosition:   Position{Offset: 45, Line: 3, Column: 26},
						},
						Identifier:    "baz",
						StartPosition: Position{Offset: 46, Line: 3, Column: 27},
						EndPosition:   Position{Offset: 47, Line: 3, Column: 28},
					},
					Value: IntExpression{
						Value:    big.NewInt(1),
						Position: Position{Offset: 53, Line: 3, Column: 34},
					},
					StartPosition: Position{Offset: 31, Line: 3, Column: 12},
					EndPosition:   Position{Offset: 53, Line: 3, Column: 34},
				},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 31, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 53, Line: 3, Column: 34},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 63, Line: 4, Column: 8},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseExpressionStatementWithAccess(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    fun test() { x.foo.bar[0][1].baz }
	`)

	test := FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: BaseType{
			Position: Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: Block{
			Statements: []Statement{
				ExpressionStatement{
					Expression: MemberExpression{
						Expression: IndexExpression{
							Expression: IndexExpression{
								Expression: MemberExpression{
									Expression: MemberExpression{
										Expression: IdentifierExpression{
											Identifier: "x",
											Position:   Position{Offset: 19, Line: 2, Column: 18},
										},
										Identifier:    "foo",
										StartPosition: Position{Offset: 20, Line: 2, Column: 19},
										EndPosition:   Position{Offset: 21, Line: 2, Column: 20},
									},
									Identifier:    "bar",
									StartPosition: Position{Offset: 24, Line: 2, Column: 23},
									EndPosition:   Position{Offset: 25, Line: 2, Column: 24},
								},
								Index: IntExpression{
									Value:    big.NewInt(0),
									Position: Position{Offset: 29, Line: 2, Column: 28},
								},
								StartPosition: Position{Offset: 28, Line: 2, Column: 27},
								EndPosition:   Position{Offset: 30, Line: 2, Column: 29},
							},
							Index: IntExpression{
								Value:    big.NewInt(1),
								Position: Position{Offset: 32, Line: 2, Column: 31},
							},
							StartPosition: Position{Offset: 31, Line: 2, Column: 30},
							EndPosition:   Position{Offset: 33, Line: 2, Column: 32},
						},
						Identifier:    "baz",
						StartPosition: Position{Offset: 34, Line: 2, Column: 33},
						EndPosition:   Position{Offset: 35, Line: 2, Column: 34},
					},
				},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 35, Line: 2, Column: 34},
		},
		StartPosition:      Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:        Position{Offset: 39, Line: 2, Column: 38},
		IdentifierPosition: Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseParametersAndArrayTypes(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		pub fun test(a: Int32, b: Int32[2], c: Int32[][3]): Int64[][] {}
	`)

	test := FunctionDeclaration{
		IsPublic:   true,
		Identifier: "test",
		Parameters: []Parameter{
			{
				Identifier: "a",
				Type: BaseType{
					Identifier: "Int32",
					Position:   Position{Offset: 19, Line: 2, Column: 18},
				},
				StartPosition: Position{Offset: 16, Line: 2, Column: 15},
				EndPosition:   Position{Offset: 19, Line: 2, Column: 18},
			},
			{
				Identifier: "b",
				Type: ConstantSizedType{
					Type: BaseType{
						Identifier: "Int32",
						Position:   Position{Offset: 29, Line: 2, Column: 28},
					},
					Size:          2,
					StartPosition: Position{Offset: 34, Line: 2, Column: 33},
					EndPosition:   Position{Offset: 36, Line: 2, Column: 35},
				},
				StartPosition: Position{Offset: 26, Line: 2, Column: 25},
				EndPosition:   Position{Offset: 36, Line: 2, Column: 35},
			},
			{
				Identifier: "c",
				Type: VariableSizedType{
					Type: ConstantSizedType{
						Type: BaseType{
							Identifier: "Int32",
							Position:   Position{Offset: 42, Line: 2, Column: 41},
						},
						Size:          3,
						StartPosition: Position{Offset: 49, Line: 2, Column: 48},
						EndPosition:   Position{Offset: 51, Line: 2, Column: 50},
					},
					StartPosition: Position{Offset: 47, Line: 2, Column: 46},
					EndPosition:   Position{Offset: 48, Line: 2, Column: 47},
				},
				StartPosition: Position{Offset: 39, Line: 2, Column: 38},
				EndPosition:   Position{Offset: 51, Line: 2, Column: 50},
			},
		},
		ReturnType: VariableSizedType{
			Type: VariableSizedType{
				Type: BaseType{
					Identifier: "Int64",
					Position:   Position{Offset: 55, Line: 2, Column: 54},
				},
				StartPosition: Position{Offset: 62, Line: 2, Column: 61},
				EndPosition:   Position{Offset: 63, Line: 2, Column: 62},
			},
			StartPosition: Position{Offset: 60, Line: 2, Column: 59},
			EndPosition:   Position{Offset: 61, Line: 2, Column: 60},
		},
		Block: Block{
			StartPosition: Position{Offset: 66, Line: 2, Column: 65},
			EndPosition:   Position{Offset: 65, Line: 2, Column: 64},
		},
		StartPosition:      Position{Offset: 3, Line: 2, Column: 2},
		EndPosition:        Position{Offset: 66, Line: 2, Column: 65},
		IdentifierPosition: Position{Offset: 11, Line: 2, Column: 10},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIntegerLiterals(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		const octal = 0o32
        const hex = 0xf2
        const binary = 0b101010
	`)

	octal := VariableDeclaration{
		Identifier: "octal",
		IsConst:    true,
		Value: IntExpression{
			Value:    big.NewInt(26),
			Position: Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPosition:      Position{Offset: 3, Line: 2, Column: 2},
		EndPosition:        Position{Offset: 17, Line: 2, Column: 16},
		IdentifierPosition: Position{Offset: 9, Line: 2, Column: 8},
	}

	hex := VariableDeclaration{
		Identifier: "hex",
		IsConst:    true,
		Value: IntExpression{
			Value:    big.NewInt(242),
			Position: Position{Offset: 42, Line: 3, Column: 20},
		},
		StartPosition:      Position{Offset: 30, Line: 3, Column: 8},
		EndPosition:        Position{Offset: 42, Line: 3, Column: 20},
		IdentifierPosition: Position{Offset: 36, Line: 3, Column: 14},
	}

	binary := VariableDeclaration{
		Identifier: "binary",
		IsConst:    true,
		Value: IntExpression{
			Value:    big.NewInt(42),
			Position: Position{Offset: 70, Line: 4, Column: 23},
		},
		StartPosition:      Position{Offset: 55, Line: 4, Column: 8},
		EndPosition:        Position{Offset: 70, Line: 4, Column: 23},
		IdentifierPosition: Position{Offset: 61, Line: 4, Column: 14},
	}

	expected := Program{
		AllDeclarations: []Declaration{octal, hex, binary},
		Declarations:    map[string]Declaration{"octal": octal, "hex": hex, "binary": binary},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIntegerTypes(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		const a: Int8 = 1
		const b: Int16 = 2
		const c: Int32 = 3
		const d: Int64 = 4
		const e: UInt8 = 5
		const f: UInt16 = 6
		const g: UInt32 = 7
		const h: UInt64 = 8
	`)

	a := VariableDeclaration{
		Identifier: "a",
		IsConst:    true,
		Type: BaseType{
			Identifier: "Int8",
			Position:   Position{Offset: 12, Line: 2, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(1),
			Position: Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPosition:      Position{Offset: 3, Line: 2, Column: 2},
		EndPosition:        Position{Offset: 19, Line: 2, Column: 18},
		IdentifierPosition: Position{Offset: 9, Line: 2, Column: 8},
	}
	b := VariableDeclaration{
		Identifier: "b",
		IsConst:    true,
		Type: BaseType{
			Identifier: "Int16",
			Position:   Position{Offset: 32, Line: 3, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(2),
			Position: Position{Offset: 40, Line: 3, Column: 19},
		},
		StartPosition:      Position{Offset: 23, Line: 3, Column: 2},
		EndPosition:        Position{Offset: 40, Line: 3, Column: 19},
		IdentifierPosition: Position{Offset: 29, Line: 3, Column: 8},
	}
	c := VariableDeclaration{
		Identifier: "c",
		IsConst:    true,
		Type: BaseType{
			Identifier: "Int32",
			Position:   Position{Offset: 53, Line: 4, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(3),
			Position: Position{Offset: 61, Line: 4, Column: 19},
		},
		StartPosition:      Position{Offset: 44, Line: 4, Column: 2},
		EndPosition:        Position{Offset: 61, Line: 4, Column: 19},
		IdentifierPosition: Position{Offset: 50, Line: 4, Column: 8},
	}
	d := VariableDeclaration{
		Identifier: "d",
		IsConst:    true,
		Type: BaseType{
			Identifier: "Int64",
			Position:   Position{Offset: 74, Line: 5, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(4),
			Position: Position{Offset: 82, Line: 5, Column: 19},
		},
		StartPosition:      Position{Offset: 65, Line: 5, Column: 2},
		EndPosition:        Position{Offset: 82, Line: 5, Column: 19},
		IdentifierPosition: Position{Offset: 71, Line: 5, Column: 8},
	}
	e := VariableDeclaration{
		Identifier: "e",
		IsConst:    true,
		Type: BaseType{
			Identifier: "UInt8",
			Position:   Position{Offset: 95, Line: 6, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(5),
			Position: Position{Offset: 103, Line: 6, Column: 19},
		},
		StartPosition:      Position{Offset: 86, Line: 6, Column: 2},
		EndPosition:        Position{Offset: 103, Line: 6, Column: 19},
		IdentifierPosition: Position{Offset: 92, Line: 6, Column: 8},
	}
	f := VariableDeclaration{
		Identifier: "f",
		IsConst:    true,
		Type: BaseType{
			Identifier: "UInt16",
			Position:   Position{Offset: 116, Line: 7, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(6),
			Position: Position{Offset: 125, Line: 7, Column: 20},
		},
		StartPosition:      Position{Offset: 107, Line: 7, Column: 2},
		EndPosition:        Position{Offset: 125, Line: 7, Column: 20},
		IdentifierPosition: Position{Offset: 113, Line: 7, Column: 8},
	}
	g := VariableDeclaration{
		Identifier: "g",
		IsConst:    true,
		Type: BaseType{
			Identifier: "UInt32",
			Position:   Position{Offset: 138, Line: 8, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(7),
			Position: Position{Offset: 147, Line: 8, Column: 20},
		},
		StartPosition:      Position{Offset: 129, Line: 8, Column: 2},
		EndPosition:        Position{Offset: 147, Line: 8, Column: 20},
		IdentifierPosition: Position{Offset: 135, Line: 8, Column: 8},
	}
	h := VariableDeclaration{
		Identifier: "h",
		IsConst:    true,
		Type: BaseType{
			Identifier: "UInt64",
			Position:   Position{Offset: 160, Line: 9, Column: 11},
		},
		Value: IntExpression{
			Value:    big.NewInt(8),
			Position: Position{Offset: 169, Line: 9, Column: 20},
		},
		StartPosition:      Position{Offset: 151, Line: 9, Column: 2},
		EndPosition:        Position{Offset: 169, Line: 9, Column: 20},
		IdentifierPosition: Position{Offset: 157, Line: 9, Column: 8},
	}

	expected := Program{
		AllDeclarations: []Declaration{a, b, c, d, e, f, g, h},
		Declarations:    map[string]Declaration{"a": a, "b": b, "c": c, "d": d, "e": e, "f": f, "g": g, "h": h},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionType(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		const add: (Int8, Int16) => Int32 = nothing
	`)

	add := VariableDeclaration{
		Identifier: "add",
		IsConst:    true,
		Type: FunctionType{
			ParameterTypes: []Type{
				BaseType{
					Identifier: "Int8",
					Position:   Position{Offset: 15, Line: 2, Column: 14},
				},
				BaseType{
					Identifier: "Int16",
					Position:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			ReturnType: BaseType{
				Identifier: "Int32",
				Position:   Position{Offset: 31, Line: 2, Column: 30},
			},
			StartPosition: Position{Offset: 14, Line: 2, Column: 13},
			EndPosition:   Position{Offset: 31, Line: 2, Column: 30},
		},
		Value: IdentifierExpression{
			Identifier: "nothing",
			Position:   Position{Offset: 39, Line: 2, Column: 38},
		},
		StartPosition:      Position{Offset: 3, Line: 2, Column: 2},
		EndPosition:        Position{Offset: 39, Line: 2, Column: 38},
		IdentifierPosition: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := Program{
		AllDeclarations: []Declaration{add},
		Declarations:    map[string]Declaration{"add": add},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMissingReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		const noop: () => Void =
            fun () { return }
	`)

	noop := VariableDeclaration{
		Identifier: "noop",
		IsConst:    true,
		Type: FunctionType{
			ReturnType: BaseType{
				Identifier: "Void",
				Position:   Position{Offset: 21, Line: 2, Column: 20},
			},
			StartPosition: Position{Offset: 15, Line: 2, Column: 14},
			EndPosition:   Position{Offset: 21, Line: 2, Column: 20},
		},
		Value: FunctionExpression{
			ReturnType: BaseType{
				Position: Position{Offset: 45, Line: 3, Column: 17},
			},
			Block: Block{
				Statements: []Statement{
					ReturnStatement{
						StartPosition: Position{Offset: 49, Line: 3, Column: 21},
						EndPosition:   Position{Offset: 49, Line: 3, Column: 21},
					},
				},
				// NOTE: block is statements *inside* curly braces
				StartPosition: Position{Offset: 49, Line: 3, Column: 21},
				EndPosition:   Position{Offset: 49, Line: 3, Column: 21},
			},
			StartPosition: Position{Offset: 40, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 56, Line: 3, Column: 28},
		},
		StartPosition:      Position{Offset: 3, Line: 2, Column: 2},
		EndPosition:        Position{Offset: 56, Line: 3, Column: 28},
		IdentifierPosition: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := Program{
		AllDeclarations: []Declaration{noop},
		Declarations:    map[string]Declaration{"noop": noop},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseLeftAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = 1 + 2 + 3
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: BinaryExpression{
			Operation: OperationPlus,
			Left: BinaryExpression{
				Operation: OperationPlus,
				Left: IntExpression{
					Value:    big.NewInt(1),
					Position: Position{Offset: 19, Line: 2, Column: 18},
				},
				Right: IntExpression{
					Value:    big.NewInt(2),
					Position: Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPosition: Position{Offset: 19, Line: 2, Column: 18},
				EndPosition:   Position{Offset: 23, Line: 2, Column: 22},
			},
			Right: IntExpression{
				Value:    big.NewInt(3),
				Position: Position{Offset: 27, Line: 2, Column: 26},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 27, Line: 2, Column: 26},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 27, Line: 2, Column: 26},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseInvalidDoubleIntegerUnary(t *testing.T) {
	RegisterTestingT(t)

	Expect(func() {
		parse(`
            var a = 1
            const b = --a
	    `)
	}).To(Panic())
}

func TestParseInvalidDoubleBooleanUnary(t *testing.T) {
	RegisterTestingT(t)

	Expect(func() {
		parse(`
            const b = !!true
	    `)
	}).To(Panic())
}

func TestParseTernaryRightAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
        const a = 2 > 1
          ? 0
          : 3 > 2 ? 1 : 2
	`)

	a := VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: ConditionalExpression{
			Test: BinaryExpression{
				Operation: OperationGreater,
				Left: IntExpression{
					Value:    big.NewInt(2),
					Position: Position{Offset: 19, Line: 2, Column: 18},
				},
				Right: IntExpression{
					Value:    big.NewInt(1),
					Position: Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPosition: Position{Offset: 19, Line: 2, Column: 18},
				EndPosition:   Position{Offset: 23, Line: 2, Column: 22},
			},
			Then: IntExpression{
				Value:    big.NewInt(0),
				Position: Position{Offset: 37, Line: 3, Column: 12},
			},
			Else: ConditionalExpression{
				Test: BinaryExpression{
					Operation: OperationGreater,
					Left: IntExpression{
						Value:    big.NewInt(3),
						Position: Position{Offset: 51, Line: 4, Column: 12},
					},
					Right: IntExpression{
						Value:    big.NewInt(2),
						Position: Position{Offset: 55, Line: 4, Column: 16},
					},
					StartPosition: Position{Offset: 51, Line: 4, Column: 12},
					EndPosition:   Position{Offset: 55, Line: 4, Column: 16},
				},
				Then: IntExpression{
					Value:    big.NewInt(1),
					Position: Position{Offset: 59, Line: 4, Column: 20},
				},
				Else: IntExpression{
					Value:    big.NewInt(2),
					Position: Position{Offset: 63, Line: 4, Column: 24},
				},
				StartPosition: Position{Offset: 51, Line: 4, Column: 12},
				EndPosition:   Position{Offset: 63, Line: 4, Column: 24},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 63, Line: 4, Column: 24},
		},
		StartPosition:      Position{Offset: 9, Line: 2, Column: 8},
		EndPosition:        Position{Offset: 63, Line: 4, Column: 24},
		IdentifierPosition: Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

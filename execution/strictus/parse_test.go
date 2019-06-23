package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

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
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionExpression(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
	    const test = fun () { return }
	`)

	test := VariableDeclaration{
		IsConst:    true,
		Identifier: "test",
		Value: FunctionExpression{
			ReturnType: VoidType{},
			Block: Block{
				Statements: []Statement{
					ReturnStatement{},
				},
				// NOTE: block is statements *inside* curly braces
				StartPosition: Position{Offset: 28, Line: 2, Column: 27},
				EndPosition:   Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 35, Line: 2, Column: 34},
		},
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
		ReturnType: VoidType{},
		Block: Block{
			Statements: []Statement{
				ReturnStatement{},
			},
			// NOTE: block is statements *inside* curly braces
			StartPosition: Position{Offset: 19, Line: 2, Column: 18},
			EndPosition:   Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPosition: Position{Offset: 6, Line: 2, Column: 5},
		EndPosition:   Position{Offset: 26, Line: 2, Column: 25},
	}

	expected := Program{
		AllDeclarations: []Declaration{test},
		Declarations:    map[string]Declaration{"test": test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseComplexFunction(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		pub fun sum(a: Int32, b: Int32[2], c: Int32[][3]): Int64 {
            const x = 1
            var y: Int32 = 2
            y = (3)
            x.foo.bar[0][1].baz
            z = sum(0o3, 0x2, 0b1) % 42
            return a
            while x < 2 {
                x = x + 1
            }
            if true {
                return 1
            } else if false {
                return 2 > 3 ? 4 : 5
            } else {
                return [2, true]
            }
        }
	`)

	sum := FunctionDeclaration{
		IsPublic:   true,
		Identifier: "sum",
		Parameters: []Parameter{
			{Identifier: "a", Type: Int32Type{}},
			{Identifier: "b", Type: ConstantSizedType{Type: Int32Type{}, Size: 2}},
			{Identifier: "c", Type: VariableSizedType{Type: ConstantSizedType{Type: Int32Type{}, Size: 3}}},
		},
		ReturnType: Int64Type{},
		Block: Block{
			Statements: []Statement{
				VariableDeclaration{
					IsConst:    true,
					Identifier: "x",
					Type:       nil,
					Value:      IntExpression{Value: big.NewInt(1)},
				},
				VariableDeclaration{
					IsConst:    false,
					Identifier: "y",
					Type:       Int32Type{},
					Value:      IntExpression{Value: big.NewInt(2)},
				},
				Assignment{
					Target: IdentifierExpression{Identifier: "y"},
					Value:  IntExpression{Value: big.NewInt(3)},
				},
				ExpressionStatement{
					Expression: MemberExpression{
						Expression: IndexExpression{
							Expression: IndexExpression{
								Expression: MemberExpression{
									Expression: MemberExpression{
										Expression: IdentifierExpression{Identifier: "x"},
										Identifier: "foo",
									},
									Identifier: "bar",
								},
								Index: IntExpression{Value: big.NewInt(0)},
							},
							Index: IntExpression{Value: big.NewInt(1)},
						},
						Identifier: "baz",
					},
				},
				Assignment{
					Target: IdentifierExpression{Identifier: "z"},
					Value: BinaryExpression{
						Operation: OperationMod,
						Left: InvocationExpression{
							Expression: IdentifierExpression{Identifier: "sum"},
							Arguments: []Expression{
								IntExpression{Value: big.NewInt(3)},
								IntExpression{Value: big.NewInt(2)},
								IntExpression{Value: big.NewInt(1)},
							},
						},
						Right: IntExpression{Value: big.NewInt(42)},
					},
				},
				ReturnStatement{Expression: IdentifierExpression{Identifier: "a"}},
				WhileStatement{
					Test: BinaryExpression{
						Operation: OperationLess,
						Left:      IdentifierExpression{Identifier: "x"},
						Right:     IntExpression{Value: big.NewInt(2)},
					},
					Block: Block{
						Statements: []Statement{
							Assignment{
								Target: IdentifierExpression{Identifier: "x"},
								Value: BinaryExpression{
									Operation: OperationPlus,
									Left:      IdentifierExpression{Identifier: "x"},
									Right:     IntExpression{Value: big.NewInt(1)},
								},
							},
						},
					},
				},
				IfStatement{
					Test: BoolExpression{Value: true},
					Then: Block{
						Statements: []Statement{
							ReturnStatement{Expression: IntExpression{Value: big.NewInt(1)}},
						},
					},
					Else: Block{
						Statements: []Statement{
							IfStatement{
								Test: BoolExpression{Value: false},
								Then: Block{
									Statements: []Statement{
										ReturnStatement{
											Expression: ConditionalExpression{
												Test: BinaryExpression{
													Operation: OperationGreater,
													Left:      IntExpression{Value: big.NewInt(2)},
													Right:     IntExpression{Value: big.NewInt(3)},
												},
												Then: IntExpression{Value: big.NewInt(4)},
												Else: IntExpression{Value: big.NewInt(5)},
											},
										},
									},
								},
								Else: Block{
									Statements: []Statement{
										ReturnStatement{
											Expression: ArrayExpression{
												Values: []Expression{
													IntExpression{Value: big.NewInt(2)},
													BoolExpression{Value: false},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	expected := Program{
		AllDeclarations: []Declaration{sum},
		Declarations:    map[string]Declaration{"sum": sum},
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
		Type:       Int8Type{},
		Value: IntExpression{
			Value:    big.NewInt(1),
			Position: Position{Offset: 19, Line: 2, Column: 18},
		},
	}
	b := VariableDeclaration{
		Identifier: "b",
		IsConst:    true,
		Type:       Int16Type{},
		Value: IntExpression{
			Value:    big.NewInt(2),
			Position: Position{Offset: 40, Line: 3, Column: 19},
		},
	}
	c := VariableDeclaration{
		Identifier: "c",
		IsConst:    true,
		Type:       Int32Type{},
		Value: IntExpression{
			Value:    big.NewInt(3),
			Position: Position{Offset: 61, Line: 4, Column: 19},
		},
	}
	d := VariableDeclaration{
		Identifier: "d",
		IsConst:    true,
		Type:       Int64Type{},
		Value: IntExpression{
			Value:    big.NewInt(4),
			Position: Position{Offset: 82, Line: 5, Column: 19},
		},
	}
	e := VariableDeclaration{
		Identifier: "e",
		IsConst:    true,
		Type:       UInt8Type{},
		Value: IntExpression{
			Value:    big.NewInt(5),
			Position: Position{Offset: 103, Line: 6, Column: 19},
		},
	}
	f := VariableDeclaration{
		Identifier: "f",
		IsConst:    true,
		Type:       UInt16Type{},
		Value: IntExpression{
			Value:    big.NewInt(6),
			Position: Position{Offset: 125, Line: 7, Column: 20},
		},
	}
	g := VariableDeclaration{
		Identifier: "g",
		IsConst:    true,
		Type:       UInt32Type{},
		Value: IntExpression{
			Value:    big.NewInt(7),
			Position: Position{Offset: 147, Line: 8, Column: 20},
		},
	}
	h := VariableDeclaration{
		Identifier: "h",
		IsConst:    true,
		Type:       UInt64Type{},
		Value: IntExpression{
			Value:    big.NewInt(8),
			Position: Position{Offset: 169, Line: 9, Column: 20},
		},
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
				Int8Type{},
				Int16Type{},
			},
			ReturnType: Int32Type{},
		},
		Value: IdentifierExpression{
			Identifier: "nothing",
			Position:   Position{Offset: 39, Line: 2, Column: 38},
		},
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
			ReturnType: VoidType{},
		},
		Value: FunctionExpression{
			ReturnType: VoidType{},
			Block: Block{
				Statements: []Statement{
					ReturnStatement{},
				},
				// NOTE: block is statements *inside* curly braces
				StartPosition: Position{Offset: 49, Line: 3, Column: 21},
				EndPosition:   Position{Offset: 49, Line: 3, Column: 21},
			},
			StartPosition: Position{Offset: 40, Line: 3, Column: 12},
			EndPosition:   Position{Offset: 56, Line: 3, Column: 28},
		},
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
	}

	expected := Program{
		Declarations:    map[string]Declaration{"a": a},
		AllDeclarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	. "github.com/onsi/gomega"
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
					Value:      Int64Expression(1),
				},
				VariableDeclaration{
					IsConst:    false,
					Identifier: "y",
					Type:       Int32Type{},
					Value:      Int64Expression(2),
				},
				Assignment{
					Target: IdentifierExpression{Identifier: "y"},
					Value:  Int64Expression(3),
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
								Index: Int64Expression(0),
							},
							Index: Int64Expression(1),
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
								Int64Expression(3),
								Int64Expression(2),
								Int64Expression(1),
							},
						},
						Right: Int64Expression(42),
					},
				},
				ReturnStatement{Expression: IdentifierExpression{Identifier: "a"}},
				WhileStatement{
					Test: BinaryExpression{
						Operation: OperationLess,
						Left:      IdentifierExpression{Identifier: "x"},
						Right:     Int64Expression(2),
					},
					Block: Block{
						Statements: []Statement{
							Assignment{
								Target: IdentifierExpression{Identifier: "x"},
								Value: BinaryExpression{
									Operation: OperationPlus,
									Left:      IdentifierExpression{Identifier: "x"},
									Right:     Int64Expression(1),
								},
							},
						},
					},
				},
				IfStatement{
					Test: BoolExpression(true),
					Then: Block{
						Statements: []Statement{
							ReturnStatement{Expression: Int64Expression(1)},
						},
					},
					Else: Block{
						Statements: []Statement{
							IfStatement{
								Test: BoolExpression(false),
								Then: Block{
									Statements: []Statement{
										ReturnStatement{
											Expression: ConditionalExpression{
												Test: BinaryExpression{
													Operation: OperationGreater,
													Left:      Int64Expression(2),
													Right:     Int64Expression(3),
												},
												Then: Int64Expression(4),
												Else: Int64Expression(5),
											},
										},
									},
								},
								Else: Block{
									Statements: []Statement{
										ReturnStatement{
											Expression: ArrayExpression{
												Values: []Expression{
													Int64Expression(2),
													BoolExpression(true),
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

	a := VariableDeclaration{Identifier: "a", IsConst: true, Type: Int8Type{}, Value: Int64Expression(1)}
	b := VariableDeclaration{Identifier: "b", IsConst: true, Type: Int16Type{}, Value: Int64Expression(2)}
	c := VariableDeclaration{Identifier: "c", IsConst: true, Type: Int32Type{}, Value: Int64Expression(3)}
	d := VariableDeclaration{Identifier: "d", IsConst: true, Type: Int64Type{}, Value: Int64Expression(4)}
	e := VariableDeclaration{Identifier: "e", IsConst: true, Type: UInt8Type{}, Value: Int64Expression(5)}
	f := VariableDeclaration{Identifier: "f", IsConst: true, Type: UInt16Type{}, Value: Int64Expression(6)}
	g := VariableDeclaration{Identifier: "g", IsConst: true, Type: UInt32Type{}, Value: Int64Expression(7)}
	h := VariableDeclaration{Identifier: "h", IsConst: true, Type: UInt64Type{}, Value: Int64Expression(8)}

	expected := Program{
		AllDeclarations: []Declaration{a, b, c, d, e, f, g, h},
		Declarations:    map[string]Declaration{"a": a, "b": b, "c": c, "d": d, "e": e, "f": f, "g": g, "h": h},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionType(t *testing.T) {
	RegisterTestingT(t)

	actual := parse(`
		const add: (Int8, Int8) => Int16 =
            fun (a: Int8, b: Int8): Int16 {
                return a + b
            }
	`)

	add := VariableDeclaration{
		Identifier: "add",
		IsConst:    true,
		Type: FunctionType{
			ParameterTypes: []Type{
				Int8Type{},
				Int8Type{},
			},
			ReturnType: Int16Type{},
		},
		Value: FunctionExpression{
			Parameters: []Parameter{
				{Identifier: "a", Type: Int8Type{}},
				{Identifier: "b", Type: Int8Type{}},
			},
			ReturnType: Int16Type{},
			Block: Block{
				Statements: []Statement{
					ReturnStatement{
						Expression: BinaryExpression{
							Operation: OperationPlus,
							Left:      IdentifierExpression{Identifier: "a"},
							Right:     IdentifierExpression{Identifier: "b"},
						},
					},
				},
			},
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
			},
		},
	}

	expected := Program{
		AllDeclarations: []Declaration{noop},
		Declarations:    map[string]Declaration{"noop": noop},
	}

	Expect(actual).Should(Equal(expected))
}

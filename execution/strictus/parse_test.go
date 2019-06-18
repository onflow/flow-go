package strictus

import (
	. "bamboo-emulator/execution/strictus/ast"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	. "github.com/onsi/gomega"
	"testing"
)

func TestParse(t *testing.T) {

	input := antlr.NewInputStream(`
		pub fun sum(a: i32, b: i32[2], c: i32[][3]): i64 {
            const x = 1
            var y: i32 = 2
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

	lexer := NewStrictusLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, 0)
	parser := NewStrictusParser(stream)
	// diagnostics, for debugging only:
	// parser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
	parser.AddErrorListener(antlr.NewConsoleErrorListener())
	actual := parser.Program().Accept(&ProgramVisitor{}).(Program)

	var int32Type Type = Int32Type{}

	sum := FunctionDeclaration{
		IsPublic:   true,
		Identifier: "sum",
		Parameters: []Parameter{
			{Identifier: "a", Type: Int32Type{}},
			{Identifier: "b", Type: FixedType{Type: Int32Type{}, Size: 2}},
			{Identifier: "c", Type: DynamicType{Type: FixedType{Type: Int32Type{}, Size: 3}}},
		},
		ReturnType: Int64Type{},
		Block: Block{
			Statements: []Statement{
				VariableDeclaration{
					IsConst:    true,
					Identifier: "x",
					Type:       nil,
					Value:      IntExpression{Value: 1},
				},
				VariableDeclaration{
					IsConst:    false,
					Identifier: "y",
					Type:       int32Type,
					Value:      IntExpression{Value: 2},
				},
				Assignment{
					Target: IdentifierExpression{Identifier: "y"},
					Value:  IntExpression{Value: 3},
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
								Index: IntExpression{Value: 0},
							},
							Index: IntExpression{Value: 1},
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
								IntExpression{Value: 3},
								IntExpression{Value: 2},
								IntExpression{Value: 1},
							},
						},
						Right: IntExpression{Value: 42},
					},
				},
				ReturnStatement{Expression: IdentifierExpression{Identifier: "a"}},
				WhileStatement{
					Test: BinaryExpression{
						Operation: OperationLess,
						Left:      IdentifierExpression{Identifier: "x"},
						Right:     IntExpression{Value: 2},
					},
					Block: Block{
						Statements: []Statement{
							Assignment{
								Target: IdentifierExpression{Identifier: "x"},
								Value: BinaryExpression{
									Operation: OperationPlus,
									Left:      IdentifierExpression{Identifier: "x"},
									Right:     IntExpression{Value: 1},
								},
							},
						},
					},
				},
				IfStatement{
					Test: BoolExpression{Value: true},
					Then: Block{
						Statements: []Statement{
							ReturnStatement{Expression: IntExpression{Value: 1}},
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
													Left:      IntExpression{Value: 2},
													Right:     IntExpression{Value: 3},
												},
												Then: IntExpression{Value: 4},
												Else: IntExpression{Value: 5},
											},
										},
									},
								},
								Else: Block{
									Statements: []Statement{
										ReturnStatement{
											Expression: ArrayExpression{
												Values: []Expression{
													IntExpression{Value: 2},
													BoolExpression{Value: true},
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

	NewWithT(t).Expect(actual).Should(Equal(expected))
}

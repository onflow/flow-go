package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"bamboo-runtime/execution/strictus/parser"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gstruct"
	"math/big"
	"testing"
)

func init() {
	format.TruncatedDiff = false
	format.MaxDepth = 100
}

func TestParseIncompleteConstKeyword(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    cons
	`)

	Expect(actual).Should(BeNil())

	Expect(errors).Should(HaveLen(1))
	syntaxError := errors[0].(*parser.SyntaxError)
	Expect(*syntaxError).To(MatchAllFields(Fields{
		"Pos":     Equal(&Position{Offset: 6, Line: 2, Column: 5}),
		"Message": ContainSubstring("extraneous input"),
	}))
}

func TestParseIncompleteConstantDeclaration(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const = 
	`)

	Expect(actual).Should(BeNil())

	Expect(errors).Should(HaveLen(2))
	Expect(*errors[0].(*parser.SyntaxError)).To(MatchAllFields(Fields{
		"Pos":     Equal(&Position{Offset: 12, Line: 2, Column: 11}),
		"Message": ContainSubstring("missing Identifier"),
	}))
	Expect(*errors[1].(*parser.SyntaxError)).To(MatchAllFields(Fields{
		"Pos":     Equal(&Position{Offset: 16, Line: 3, Column: 1}),
		"Message": ContainSubstring("mismatched input"),
	}))
}

func TestParseBoolExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = true
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &BoolExpression{
			Value: true,
			Pos:   &Position{Offset: 16, Line: 2, Column: 15},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 16, Line: 2, Column: 15},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIdentifierExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const b = a
	`)

	Expect(errors).Should(BeEmpty())

	b := &VariableDeclaration{
		IsConst:    true,
		Identifier: "b",
		Value: &IdentifierExpression{
			Identifier: "a",
			StartPos:   &Position{Offset: 16, Line: 2, Column: 15},
			EndPos:     &Position{Offset: 16, Line: 2, Column: 15},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 16, Line: 2, Column: 15},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{b},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseArrayExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = [1, 2]
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &ArrayExpression{
			Values: []Expression{
				&IntExpression{
					Value: big.NewInt(1),
					Pos:   &Position{Offset: 17, Line: 2, Column: 16},
				},
				&IntExpression{
					Value: big.NewInt(2),
					Pos:   &Position{Offset: 20, Line: 2, Column: 19},
				},
			},
			StartPos: &Position{Offset: 16, Line: 2, Column: 15},
			EndPos:   &Position{Offset: 21, Line: 2, Column: 20},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 21, Line: 2, Column: 20},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseInvocationExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = b(1, 2)
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &InvocationExpression{
			Expression: &IdentifierExpression{
				Identifier: "b",
				StartPos:   &Position{Offset: 16, Line: 2, Column: 15},
				EndPos:     &Position{Offset: 16, Line: 2, Column: 15},
			},
			Arguments: []Expression{
				&IntExpression{
					Value: big.NewInt(1),
					Pos:   &Position{Offset: 18, Line: 2, Column: 17},
				},
				&IntExpression{
					Value: big.NewInt(2),
					Pos:   &Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 22, Line: 2, Column: 21},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 22, Line: 2, Column: 21},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMemberExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = b.c
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &MemberExpression{
			Expression: &IdentifierExpression{
				Identifier: "b",
				StartPos:   &Position{Offset: 16, Line: 2, Column: 15},
				EndPos:     &Position{Offset: 16, Line: 2, Column: 15},
			},
			Identifier: "c",
			StartPos:   &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:     &Position{Offset: 18, Line: 2, Column: 17},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 18, Line: 2, Column: 17},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIndexExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = b[1]
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &IndexExpression{
			Expression: &IdentifierExpression{
				Identifier: "b",
				StartPos:   &Position{Offset: 16, Line: 2, Column: 15},
				EndPos:     &Position{Offset: 16, Line: 2, Column: 15},
			},
			Index: &IntExpression{
				Value: big.NewInt(1),
				Pos:   &Position{Offset: 18, Line: 2, Column: 17},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 19, Line: 2, Column: 18},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseUnaryExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const a = -b
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Value: &UnaryExpression{
			Operation: OperationMinus,
			Expression: &IdentifierExpression{
				Identifier: "b",
				StartPos:   &Position{Offset: 17, Line: 2, Column: 16},
				EndPos:     &Position{Offset: 17, Line: 2, Column: 16},
			},
			StartPos: &Position{Offset: 16, Line: 2, Column: 15},
			EndPos:   &Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 17, Line: 2, Column: 16},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseOrExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = false || true
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationOr,
			Left: &BoolExpression{
				Value: false,
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &BoolExpression{
				Value: true,
				Pos:   &Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAndExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = false && true
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationAnd,
			Left: &BoolExpression{
				Value: false,
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &BoolExpression{
				Value: true,
				Pos:   &Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseEqualityExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = false == true
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationEqual,
			Left: &BoolExpression{
				Value: false,
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &BoolExpression{
				Value: true,
				Pos:   &Position{Offset: 28, Line: 2, Column: 27},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 28, Line: 2, Column: 27},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 28, Line: 2, Column: 27},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseRelationalExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = 1 < 2
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationLess,
			Left: &IntExpression{
				Value: big.NewInt(1),
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Pos:   &Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAdditiveExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = 1 + 2
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationPlus,
			Left: &IntExpression{
				Value: big.NewInt(1),
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Pos:   &Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMultiplicativeExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = 1 * 2
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationMul,
			Left: &IntExpression{
				Value: big.NewInt(1),
				Pos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Pos:   &Position{Offset: 23, Line: 2, Column: 22},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 23, Line: 2, Column: 22},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 23, Line: 2, Column: 22},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionExpressionAndReturn(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    const test = fun () -> Int { return 1 }
	`)

	Expect(errors).Should(BeEmpty())

	test := &VariableDeclaration{
		IsConst:    true,
		Identifier: "test",
		Value: &FunctionExpression{
			ReturnType: &BaseType{
				Identifier: "Int",
				Pos:        &Position{Offset: 29, Line: 2, Column: 28},
			},
			Block: &Block{
				Statements: []Statement{
					&ReturnStatement{
						Expression: &IntExpression{
							Value: big.NewInt(1),
							Pos:   &Position{Offset: 42, Line: 2, Column: 41},
						},
						StartPos: &Position{Offset: 35, Line: 2, Column: 34},
						EndPos:   &Position{Offset: 42, Line: 2, Column: 41},
					},
				},
				StartPos: &Position{Offset: 33, Line: 2, Column: 32},
				EndPos:   &Position{Offset: 44, Line: 2, Column: 43},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 44, Line: 2, Column: 43},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 44, Line: 2, Column: 43},
		IdentifierPos: &Position{Offset: 12, Line: 2, Column: 11},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionAndBlock(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() { return }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&ReturnStatement{
					StartPos: &Position{Offset: 19, Line: 2, Column: 18},
					EndPos:   &Position{Offset: 19, Line: 2, Column: 18},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 26, Line: 2, Column: 25},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 26, Line: 2, Column: 25},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIfStatement(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
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

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&IfStatement{
					Test: &BoolExpression{
						Value: true,
						Pos:   &Position{Offset: 34, Line: 3, Column: 15},
					},
					Then: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: nil,
								StartPos:   &Position{Offset: 57, Line: 4, Column: 16},
								EndPos:     &Position{Offset: 57, Line: 4, Column: 16},
							},
						},
						StartPos: &Position{Offset: 39, Line: 3, Column: 20},
						EndPos:   &Position{Offset: 76, Line: 5, Column: 12},
					},
					Else: &Block{
						Statements: []Statement{
							&IfStatement{
								Test: &BoolExpression{
									Value: false,
									Pos:   &Position{Offset: 86, Line: 5, Column: 22},
								},
								Then: &Block{
									Statements: []Statement{
										&ExpressionStatement{
											Expression: &BoolExpression{
												Value: false,
												Pos:   &Position{Offset: 110, Line: 6, Column: 16},
											},
										},
										&ExpressionStatement{
											Expression: &IntExpression{
												Value: big.NewInt(1),
												Pos:   &Position{Offset: 132, Line: 7, Column: 16},
											},
										},
									},
									StartPos: &Position{Offset: 92, Line: 5, Column: 28},
									EndPos:   &Position{Offset: 146, Line: 8, Column: 12},
								},
								Else: &Block{
									Statements: []Statement{
										&ExpressionStatement{
											Expression: &IntExpression{
												Value: big.NewInt(2),
												Pos:   &Position{Offset: 171, Line: 9, Column: 16},
											},
										},
									},
									StartPos: &Position{Offset: 153, Line: 8, Column: 19},
									EndPos:   &Position{Offset: 185, Line: 10, Column: 12},
								},
								StartPos: &Position{Offset: 83, Line: 5, Column: 19},
								EndPos:   &Position{Offset: 185, Line: 10, Column: 12},
							},
						},
						StartPos: &Position{Offset: 83, Line: 5, Column: 19},
						EndPos:   &Position{Offset: 185, Line: 10, Column: 12},
					},
					StartPos: &Position{Offset: 31, Line: 3, Column: 12},
					EndPos:   &Position{Offset: 185, Line: 10, Column: 12},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 195, Line: 11, Column: 8},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 195, Line: 11, Column: 8},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIfStatementNoElse(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() {
            if true {
                return
            }
        }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&IfStatement{
					Test: &BoolExpression{
						Value: true,
						Pos:   &Position{Offset: 34, Line: 3, Column: 15},
					},
					Then: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: nil,
								StartPos:   &Position{Offset: 57, Line: 4, Column: 16},
								EndPos:     &Position{Offset: 57, Line: 4, Column: 16},
							},
						},
						StartPos: &Position{Offset: 39, Line: 3, Column: 20},
						EndPos:   &Position{Offset: 76, Line: 5, Column: 12},
					},
					StartPos: &Position{Offset: 31, Line: 3, Column: 12},
					EndPos:   &Position{Offset: 76, Line: 5, Column: 12},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 86, Line: 6, Column: 8},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 86, Line: 6, Column: 8},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() {
            while true {
              return
            }
        }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&WhileStatement{
					Test: &BoolExpression{
						Value: true,
						Pos:   &Position{Offset: 37, Line: 3, Column: 18},
					},
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: nil,
								StartPos:   &Position{Offset: 58, Line: 4, Column: 14},
								EndPos:     &Position{Offset: 58, Line: 4, Column: 14},
							},
						},
						StartPos: &Position{Offset: 42, Line: 3, Column: 23},
						EndPos:   &Position{Offset: 77, Line: 5, Column: 12},
					},
					StartPos: &Position{Offset: 31, Line: 3, Column: 12},
					EndPos:   &Position{Offset: 77, Line: 5, Column: 12},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 87, Line: 6, Column: 8},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 87, Line: 6, Column: 8},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() {
            a = 1
        }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&AssignmentStatement{
					Target: &IdentifierExpression{
						Identifier: "a",
						StartPos:   &Position{Offset: 31, Line: 3, Column: 12},
						EndPos:     &Position{Offset: 31, Line: 3, Column: 12},
					},
					Value: &IntExpression{
						Value: big.NewInt(1),
						Pos:   &Position{Offset: 35, Line: 3, Column: 16},
					},
					StartPos: &Position{Offset: 31, Line: 3, Column: 12},
					EndPos:   &Position{Offset: 35, Line: 3, Column: 16},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 45, Line: 4, Column: 8},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 45, Line: 4, Column: 8},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseAccessAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() {
            x.foo.bar[0][1].baz = 1
        }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&AssignmentStatement{
					Target: &MemberExpression{
						Expression: &IndexExpression{
							Expression: &IndexExpression{
								Expression: &MemberExpression{
									Expression: &MemberExpression{
										Expression: &IdentifierExpression{
											Identifier: "x",
											StartPos:   &Position{Offset: 31, Line: 3, Column: 12},
											EndPos:     &Position{Offset: 31, Line: 3, Column: 12},
										},
										Identifier: "foo",
										StartPos:   &Position{Offset: 32, Line: 3, Column: 13},
										EndPos:     &Position{Offset: 33, Line: 3, Column: 14},
									},
									Identifier: "bar",
									StartPos:   &Position{Offset: 36, Line: 3, Column: 17},
									EndPos:     &Position{Offset: 37, Line: 3, Column: 18},
								},
								Index: &IntExpression{
									Value: big.NewInt(0),
									Pos:   &Position{Offset: 41, Line: 3, Column: 22},
								},
								StartPos: &Position{Offset: 40, Line: 3, Column: 21},
								EndPos:   &Position{Offset: 42, Line: 3, Column: 23},
							},
							Index: &IntExpression{
								Value: big.NewInt(1),
								Pos:   &Position{Offset: 44, Line: 3, Column: 25},
							},
							StartPos: &Position{Offset: 43, Line: 3, Column: 24},
							EndPos:   &Position{Offset: 45, Line: 3, Column: 26},
						},
						Identifier: "baz",
						StartPos:   &Position{Offset: 46, Line: 3, Column: 27},
						EndPos:     &Position{Offset: 47, Line: 3, Column: 28},
					},
					Value: &IntExpression{
						Value: big.NewInt(1),
						Pos:   &Position{Offset: 53, Line: 3, Column: 34},
					},
					StartPos: &Position{Offset: 31, Line: 3, Column: 12},
					EndPos:   &Position{Offset: 53, Line: 3, Column: 34},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 63, Line: 4, Column: 8},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 63, Line: 4, Column: 8},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseExpressionStatementWithAccess(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
	    fun test() { x.foo.bar[0][1].baz }
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   false,
		Identifier: "test",
		ReturnType: &BaseType{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
		Block: &Block{
			Statements: []Statement{
				&ExpressionStatement{
					Expression: &MemberExpression{
						Expression: &IndexExpression{
							Expression: &IndexExpression{
								Expression: &MemberExpression{
									Expression: &MemberExpression{
										Expression: &IdentifierExpression{
											Identifier: "x",
											StartPos:   &Position{Offset: 19, Line: 2, Column: 18},
											EndPos:     &Position{Offset: 19, Line: 2, Column: 18},
										},
										Identifier: "foo",
										StartPos:   &Position{Offset: 20, Line: 2, Column: 19},
										EndPos:     &Position{Offset: 21, Line: 2, Column: 20},
									},
									Identifier: "bar",
									StartPos:   &Position{Offset: 24, Line: 2, Column: 23},
									EndPos:     &Position{Offset: 25, Line: 2, Column: 24},
								},
								Index: &IntExpression{
									Value: big.NewInt(0),
									Pos:   &Position{Offset: 29, Line: 2, Column: 28},
								},
								StartPos: &Position{Offset: 28, Line: 2, Column: 27},
								EndPos:   &Position{Offset: 30, Line: 2, Column: 29},
							},
							Index: &IntExpression{
								Value: big.NewInt(1),
								Pos:   &Position{Offset: 32, Line: 2, Column: 31},
							},
							StartPos: &Position{Offset: 31, Line: 2, Column: 30},
							EndPos:   &Position{Offset: 33, Line: 2, Column: 32},
						},
						Identifier: "baz",
						StartPos:   &Position{Offset: 34, Line: 2, Column: 33},
						EndPos:     &Position{Offset: 35, Line: 2, Column: 34},
					},
				},
			},
			StartPos: &Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   &Position{Offset: 39, Line: 2, Column: 38},
		},
		StartPos:      &Position{Offset: 6, Line: 2, Column: 5},
		EndPos:        &Position{Offset: 39, Line: 2, Column: 38},
		IdentifierPos: &Position{Offset: 10, Line: 2, Column: 9},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseParametersAndArrayTypes(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
		pub fun test(a: Int32, b: Int32[2], c: Int32[][3]) -> Int64[][] {}
	`)

	Expect(errors).Should(BeEmpty())

	test := &FunctionDeclaration{
		IsPublic:   true,
		Identifier: "test",
		Parameters: []*Parameter{
			{
				Identifier: "a",
				Type: &BaseType{
					Identifier: "Int32",
					Pos:        &Position{Offset: 19, Line: 2, Column: 18},
				},
				StartPos: &Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   &Position{Offset: 19, Line: 2, Column: 18},
			},
			{
				Identifier: "b",
				Type: &ConstantSizedType{
					Type: &BaseType{
						Identifier: "Int32",
						Pos:        &Position{Offset: 29, Line: 2, Column: 28},
					},
					Size:     2,
					StartPos: &Position{Offset: 34, Line: 2, Column: 33},
					EndPos:   &Position{Offset: 36, Line: 2, Column: 35},
				},
				StartPos: &Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   &Position{Offset: 36, Line: 2, Column: 35},
			},
			{
				Identifier: "c",
				Type: &VariableSizedType{
					Type: &ConstantSizedType{
						Type: &BaseType{
							Identifier: "Int32",
							Pos:        &Position{Offset: 42, Line: 2, Column: 41},
						},
						Size:     3,
						StartPos: &Position{Offset: 49, Line: 2, Column: 48},
						EndPos:   &Position{Offset: 51, Line: 2, Column: 50},
					},
					StartPos: &Position{Offset: 47, Line: 2, Column: 46},
					EndPos:   &Position{Offset: 48, Line: 2, Column: 47},
				},
				StartPos: &Position{Offset: 39, Line: 2, Column: 38},
				EndPos:   &Position{Offset: 51, Line: 2, Column: 50},
			},
		},
		ReturnType: &VariableSizedType{
			Type: &VariableSizedType{
				Type: &BaseType{
					Identifier: "Int64",
					Pos:        &Position{Offset: 57, Line: 2, Column: 56},
				},
				StartPos: &Position{Offset: 64, Line: 2, Column: 63},
				EndPos:   &Position{Offset: 65, Line: 2, Column: 64},
			},
			StartPos: &Position{Offset: 62, Line: 2, Column: 61},
			EndPos:   &Position{Offset: 63, Line: 2, Column: 62},
		},
		Block: &Block{
			StartPos: &Position{Offset: 67, Line: 2, Column: 66},
			EndPos:   &Position{Offset: 68, Line: 2, Column: 67},
		},
		StartPos:      &Position{Offset: 3, Line: 2, Column: 2},
		EndPos:        &Position{Offset: 68, Line: 2, Column: 67},
		IdentifierPos: &Position{Offset: 11, Line: 2, Column: 10},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIntegerLiterals(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
		const octal = 0o32
        const hex = 0xf2
        const binary = 0b101010
	`)

	Expect(errors).Should(BeEmpty())

	octal := &VariableDeclaration{
		Identifier: "octal",
		IsConst:    true,
		Value: &IntExpression{
			Value: big.NewInt(26),
			Pos:   &Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos:      &Position{Offset: 3, Line: 2, Column: 2},
		EndPos:        &Position{Offset: 17, Line: 2, Column: 16},
		IdentifierPos: &Position{Offset: 9, Line: 2, Column: 8},
	}

	hex := &VariableDeclaration{
		Identifier: "hex",
		IsConst:    true,
		Value: &IntExpression{
			Value: big.NewInt(242),
			Pos:   &Position{Offset: 42, Line: 3, Column: 20},
		},
		StartPos:      &Position{Offset: 30, Line: 3, Column: 8},
		EndPos:        &Position{Offset: 42, Line: 3, Column: 20},
		IdentifierPos: &Position{Offset: 36, Line: 3, Column: 14},
	}

	binary := &VariableDeclaration{
		Identifier: "binary",
		IsConst:    true,
		Value: &IntExpression{
			Value: big.NewInt(42),
			Pos:   &Position{Offset: 70, Line: 4, Column: 23},
		},
		StartPos:      &Position{Offset: 55, Line: 4, Column: 8},
		EndPos:        &Position{Offset: 70, Line: 4, Column: 23},
		IdentifierPos: &Position{Offset: 61, Line: 4, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{octal, hex, binary},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseIntegerTypes(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
		const a: Int8 = 1
		const b: Int16 = 2
		const c: Int32 = 3
		const d: Int64 = 4
		const e: UInt8 = 5
		const f: UInt16 = 6
		const g: UInt32 = 7
		const h: UInt64 = 8
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		Identifier: "a",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "Int8",
			Pos:        &Position{Offset: 12, Line: 2, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(1),
			Pos:   &Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPos:      &Position{Offset: 3, Line: 2, Column: 2},
		EndPos:        &Position{Offset: 19, Line: 2, Column: 18},
		IdentifierPos: &Position{Offset: 9, Line: 2, Column: 8},
	}
	b := &VariableDeclaration{
		Identifier: "b",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "Int16",
			Pos:        &Position{Offset: 32, Line: 3, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(2),
			Pos:   &Position{Offset: 40, Line: 3, Column: 19},
		},
		StartPos:      &Position{Offset: 23, Line: 3, Column: 2},
		EndPos:        &Position{Offset: 40, Line: 3, Column: 19},
		IdentifierPos: &Position{Offset: 29, Line: 3, Column: 8},
	}
	c := &VariableDeclaration{
		Identifier: "c",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "Int32",
			Pos:        &Position{Offset: 53, Line: 4, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(3),
			Pos:   &Position{Offset: 61, Line: 4, Column: 19},
		},
		StartPos:      &Position{Offset: 44, Line: 4, Column: 2},
		EndPos:        &Position{Offset: 61, Line: 4, Column: 19},
		IdentifierPos: &Position{Offset: 50, Line: 4, Column: 8},
	}
	d := &VariableDeclaration{
		Identifier: "d",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "Int64",
			Pos:        &Position{Offset: 74, Line: 5, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(4),
			Pos:   &Position{Offset: 82, Line: 5, Column: 19},
		},
		StartPos:      &Position{Offset: 65, Line: 5, Column: 2},
		EndPos:        &Position{Offset: 82, Line: 5, Column: 19},
		IdentifierPos: &Position{Offset: 71, Line: 5, Column: 8},
	}
	e := &VariableDeclaration{
		Identifier: "e",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "UInt8",
			Pos:        &Position{Offset: 95, Line: 6, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(5),
			Pos:   &Position{Offset: 103, Line: 6, Column: 19},
		},
		StartPos:      &Position{Offset: 86, Line: 6, Column: 2},
		EndPos:        &Position{Offset: 103, Line: 6, Column: 19},
		IdentifierPos: &Position{Offset: 92, Line: 6, Column: 8},
	}
	f := &VariableDeclaration{
		Identifier: "f",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "UInt16",
			Pos:        &Position{Offset: 116, Line: 7, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(6),
			Pos:   &Position{Offset: 125, Line: 7, Column: 20},
		},
		StartPos:      &Position{Offset: 107, Line: 7, Column: 2},
		EndPos:        &Position{Offset: 125, Line: 7, Column: 20},
		IdentifierPos: &Position{Offset: 113, Line: 7, Column: 8},
	}
	g := &VariableDeclaration{
		Identifier: "g",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "UInt32",
			Pos:        &Position{Offset: 138, Line: 8, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(7),
			Pos:   &Position{Offset: 147, Line: 8, Column: 20},
		},
		StartPos:      &Position{Offset: 129, Line: 8, Column: 2},
		EndPos:        &Position{Offset: 147, Line: 8, Column: 20},
		IdentifierPos: &Position{Offset: 135, Line: 8, Column: 8},
	}
	h := &VariableDeclaration{
		Identifier: "h",
		IsConst:    true,
		Type: &BaseType{
			Identifier: "UInt64",
			Pos:        &Position{Offset: 160, Line: 9, Column: 11},
		},
		Value: &IntExpression{
			Value: big.NewInt(8),
			Pos:   &Position{Offset: 169, Line: 9, Column: 20},
		},
		StartPos:      &Position{Offset: 151, Line: 9, Column: 2},
		EndPos:        &Position{Offset: 169, Line: 9, Column: 20},
		IdentifierPos: &Position{Offset: 157, Line: 9, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a, b, c, d, e, f, g, h},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseFunctionType(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
		const add: (Int8, Int16) -> Int32 = nothing
	`)

	Expect(errors).Should(BeEmpty())

	add := &VariableDeclaration{
		Identifier: "add",
		IsConst:    true,
		Type: &FunctionType{
			ParameterTypes: []Type{
				&BaseType{
					Identifier: "Int8",
					Pos:        &Position{Offset: 15, Line: 2, Column: 14},
				},
				&BaseType{
					Identifier: "Int16",
					Pos:        &Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			ReturnType: &BaseType{
				Identifier: "Int32",
				Pos:        &Position{Offset: 31, Line: 2, Column: 30},
			},
			StartPos: &Position{Offset: 14, Line: 2, Column: 13},
			EndPos:   &Position{Offset: 31, Line: 2, Column: 30},
		},
		Value: &IdentifierExpression{
			Identifier: "nothing",
			StartPos:   &Position{Offset: 39, Line: 2, Column: 38},
			EndPos:     &Position{Offset: 45, Line: 2, Column: 44},
		},
		StartPos:      &Position{Offset: 3, Line: 2, Column: 2},
		EndPos:        &Position{Offset: 39, Line: 2, Column: 38},
		IdentifierPos: &Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{add},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseMissingReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
		const noop: () -> Void =
            fun () { return }
	`)

	Expect(errors).Should(BeEmpty())

	noop := &VariableDeclaration{
		Identifier: "noop",
		IsConst:    true,
		Type: &FunctionType{
			ReturnType: &BaseType{
				Identifier: "Void",
				Pos:        &Position{Offset: 21, Line: 2, Column: 20},
			},
			StartPos: &Position{Offset: 15, Line: 2, Column: 14},
			EndPos:   &Position{Offset: 21, Line: 2, Column: 20},
		},
		Value: &FunctionExpression{
			ReturnType: &BaseType{
				Pos: &Position{Offset: 45, Line: 3, Column: 17},
			},
			Block: &Block{
				Statements: []Statement{
					&ReturnStatement{
						StartPos: &Position{Offset: 49, Line: 3, Column: 21},
						EndPos:   &Position{Offset: 49, Line: 3, Column: 21},
					},
				},
				StartPos: &Position{Offset: 47, Line: 3, Column: 19},
				EndPos:   &Position{Offset: 56, Line: 3, Column: 28},
			},
			StartPos: &Position{Offset: 40, Line: 3, Column: 12},
			EndPos:   &Position{Offset: 56, Line: 3, Column: 28},
		},
		StartPos:      &Position{Offset: 3, Line: 2, Column: 2},
		EndPos:        &Position{Offset: 56, Line: 3, Column: 28},
		IdentifierPos: &Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{noop},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseLeftAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = 1 + 2 + 3
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &BinaryExpression{
			Operation: OperationPlus,
			Left: &BinaryExpression{
				Operation: OperationPlus,
				Left: &IntExpression{
					Value: big.NewInt(1),
					Pos:   &Position{Offset: 19, Line: 2, Column: 18},
				},
				Right: &IntExpression{
					Value: big.NewInt(2),
					Pos:   &Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPos: &Position{Offset: 19, Line: 2, Column: 18},
				EndPos:   &Position{Offset: 23, Line: 2, Column: 22},
			},
			Right: &IntExpression{
				Value: big.NewInt(3),
				Pos:   &Position{Offset: 27, Line: 2, Column: 26},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 27, Line: 2, Column: 26},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 27, Line: 2, Column: 26},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

func TestParseInvalidDoubleIntegerUnary(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
	   var a = 1
	   const b = --a
	`)

	Expect(program).To(BeNil())
	Expect(errors).To(Equal([]error{
		&parser.JuxtaposedUnaryOperatorsError{
			Pos: &Position{Offset: 29, Line: 3, Column: 14},
		},
	}))
}

func TestParseInvalidDoubleBooleanUnary(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
	   const b = !!true
	`)

	Expect(program).To(BeNil())
	Expect(errors).To(Equal([]error{
		&parser.JuxtaposedUnaryOperatorsError{
			Pos: &Position{Offset: 15, Line: 2, Column: 14},
		},
	}))
}

func TestParseTernaryRightAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual, errors := parser.Parse(`
        const a = 2 > 1
          ? 0
          : 3 > 2 ? 1 : 2
	`)

	Expect(errors).Should(BeEmpty())

	a := &VariableDeclaration{
		IsConst:    true,
		Identifier: "a",
		Type:       Type(nil),
		Value: &ConditionalExpression{
			Test: &BinaryExpression{
				Operation: OperationGreater,
				Left: &IntExpression{
					Value: big.NewInt(2),
					Pos:   &Position{Offset: 19, Line: 2, Column: 18},
				},
				Right: &IntExpression{
					Value: big.NewInt(1),
					Pos:   &Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPos: &Position{Offset: 19, Line: 2, Column: 18},
				EndPos:   &Position{Offset: 23, Line: 2, Column: 22},
			},
			Then: &IntExpression{
				Value: big.NewInt(0),
				Pos:   &Position{Offset: 37, Line: 3, Column: 12},
			},
			Else: &ConditionalExpression{
				Test: &BinaryExpression{
					Operation: OperationGreater,
					Left: &IntExpression{
						Value: big.NewInt(3),
						Pos:   &Position{Offset: 51, Line: 4, Column: 12},
					},
					Right: &IntExpression{
						Value: big.NewInt(2),
						Pos:   &Position{Offset: 55, Line: 4, Column: 16},
					},
					StartPos: &Position{Offset: 51, Line: 4, Column: 12},
					EndPos:   &Position{Offset: 55, Line: 4, Column: 16},
				},
				Then: &IntExpression{
					Value: big.NewInt(1),
					Pos:   &Position{Offset: 59, Line: 4, Column: 20},
				},
				Else: &IntExpression{
					Value: big.NewInt(2),
					Pos:   &Position{Offset: 63, Line: 4, Column: 24},
				},
				StartPos: &Position{Offset: 51, Line: 4, Column: 12},
				EndPos:   &Position{Offset: 63, Line: 4, Column: 24},
			},
			StartPos: &Position{Offset: 19, Line: 2, Column: 18},
			EndPos:   &Position{Offset: 63, Line: 4, Column: 24},
		},
		StartPos:      &Position{Offset: 9, Line: 2, Column: 8},
		EndPos:        &Position{Offset: 63, Line: 4, Column: 24},
		IdentifierPos: &Position{Offset: 15, Line: 2, Column: 14},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).Should(Equal(expected))
}

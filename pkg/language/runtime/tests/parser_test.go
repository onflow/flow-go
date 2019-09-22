package tests

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"math/big"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	. "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
)

func init() {
	format.TruncatedDiff = false
	format.MaxDepth = 100
}

func TestParseInvalidIncompleteConstKeyword(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    le
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.SyntaxError)

	Expect(syntaxError.Pos).
		To(Equal(Position{Offset: 6, Line: 2, Column: 5}))

	Expect(syntaxError.Message).
		To(ContainSubstring("extraneous input"))
}

func TestParseInvalidIncompleteConstantDeclaration1(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError1 := errors[0].(*parser.SyntaxError)

	Expect(syntaxError1.Pos).
		To(Equal(Position{Offset: 11, Line: 3, Column: 1}))

	Expect(syntaxError1.Message).
		To(ContainSubstring("mismatched input"))
}

func TestParseInvalidIncompleteConstantDeclaration2(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let =
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(2))

	syntaxError1 := errors[0].(*parser.SyntaxError)

	Expect(syntaxError1.Pos).
		To(Equal(Position{Offset: 10, Line: 2, Column: 9}))

	Expect(syntaxError1.Message).
		To(ContainSubstring("missing"))

	syntaxError2 := errors[1].(*parser.SyntaxError)

	Expect(syntaxError2.Pos).
		To(Equal(Position{Offset: 13, Line: 3, Column: 1}))

	Expect(syntaxError2.Message).
		To(ContainSubstring("mismatched input"))
}

func TestParseBoolExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &BoolExpression{
			Value:    true,
			StartPos: Position{Offset: 14, Line: 2, Column: 13},
			EndPos:   Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIdentifierExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let b = a
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	b := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "b",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "a",
				Pos:        Position{Offset: 14, Line: 2, Column: 13},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{b},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseArrayExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = [1, 2]
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{Identifier: "a",
			Pos: Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &ArrayExpression{
			Values: []Expression{
				&IntExpression{
					Value:    big.NewInt(1),
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
					EndPos:   Position{Offset: 15, Line: 2, Column: 14},
				},
				&IntExpression{
					Value:    big.NewInt(2),
					StartPos: Position{Offset: 18, Line: 2, Column: 17},
					EndPos:   Position{Offset: 18, Line: 2, Column: 17},
				},
			},
			StartPos: Position{Offset: 14, Line: 2, Column: 13},
			EndPos:   Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseDictionaryExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let x = {"a": 1, "b": 2}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	x := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{Identifier: "x",
			Pos: Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &DictionaryExpression{
			Entries: []Entry{
				{
					Key: &StringExpression{
						Value:    "a",
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
					Value: &IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 20, Line: 2, Column: 19},
						EndPos:   Position{Offset: 20, Line: 2, Column: 19},
					},
				},
				{
					Key: &StringExpression{
						Value:    "b",
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 25, Line: 2, Column: 24},
					},
					Value: &IntExpression{
						Value:    big.NewInt(2),
						StartPos: Position{Offset: 28, Line: 2, Column: 27},
						EndPos:   Position{Offset: 28, Line: 2, Column: 27},
					},
				},
			},
			StartPos: Position{Offset: 14, Line: 2, Column: 13},
			EndPos:   Position{Offset: 29, Line: 2, Column: 28},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{x},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInvocationExpressionWithoutLabels(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = b(1, 2)
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
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
					Expression: &IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 16, Line: 2, Column: 15},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
				},
				{
					Label: "",
					Expression: &IntExpression{
						Value:    big.NewInt(2),
						StartPos: Position{Offset: 19, Line: 2, Column: 18},
						EndPos:   Position{Offset: 19, Line: 2, Column: 18},
					},
				},
			},
			EndPos: Position{Offset: 20, Line: 2, Column: 19},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInvocationExpressionWithLabels(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = b(x: 1, y: 2)
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
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
					Expression: &IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 19, Line: 2, Column: 18},
						EndPos:   Position{Offset: 19, Line: 2, Column: 18},
					},
				},
				{
					Label:         "y",
					LabelStartPos: &Position{Offset: 22, Line: 2, Column: 21},
					LabelEndPos:   &Position{Offset: 22, Line: 2, Column: 21},
					Expression: &IntExpression{
						Value:    big.NewInt(2),
						StartPos: Position{Offset: 25, Line: 2, Column: 24},
						EndPos:   Position{Offset: 25, Line: 2, Column: 24},
					},
				},
			},
			EndPos: Position{Offset: 26, Line: 2, Column: 25},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMemberExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = b.c
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIndexExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let a = b[1]
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &IndexExpression{
			Expression: &IdentifierExpression{
				Identifier: Identifier{
					Identifier: "b",
					Pos:        Position{Offset: 14, Line: 2, Column: 13},
				},
			},
			Index: &IntExpression{
				Value:    big.NewInt(1),
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 16, Line: 2, Column: 15},
			},
			StartPos: Position{Offset: 15, Line: 2, Column: 14},
			EndPos:   Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseUnaryExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let foo = -boo
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "foo",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
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
			EndPos:   Position{Offset: 19, Line: 2, Column: 18},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseOrExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = false || true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationOr,
			Left: &BoolExpression{
				Value:    false,
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
			Right: &BoolExpression{
				Value:    true,
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 29, Line: 2, Column: 28},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseAndExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = false && true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationAnd,
			Left: &BoolExpression{
				Value:    false,
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
			Right: &BoolExpression{
				Value:    true,
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 29, Line: 2, Column: 28},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseEqualityExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = false == true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationEqual,
			Left: &BoolExpression{
				Value:    false,
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
			Right: &BoolExpression{
				Value:    true,
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 29, Line: 2, Column: 28},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseRelationalExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = 1 < 2
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationLess,
			Left: &IntExpression{
				Value:    big.NewInt(1),
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 17, Line: 2, Column: 16},
			},
			Right: &IntExpression{
				Value:    big.NewInt(2),
				StartPos: Position{Offset: 21, Line: 2, Column: 20},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseAdditiveExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = 1 + 2
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationPlus,
			Left: &IntExpression{
				Value:    big.NewInt(1),
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 17, Line: 2, Column: 16},
			},
			Right: &IntExpression{
				Value:    big.NewInt(2),
				StartPos: Position{Offset: 21, Line: 2, Column: 20},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMultiplicativeExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = 1 * 2
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationMul,
			Left: &IntExpression{
				Value:    big.NewInt(1),
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 17, Line: 2, Column: 16},
			},
			Right: &IntExpression{
				Value:    big.NewInt(2),
				StartPos: Position{Offset: 21, Line: 2, Column: 20},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseConcatenatingExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = [1, 2] & [3, 4]
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationConcat,
			Left: &ArrayExpression{
				Values: []Expression{
					&IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 18, Line: 2, Column: 17},
						EndPos:   Position{Offset: 18, Line: 2, Column: 17},
					},
					&IntExpression{
						Value:    big.NewInt(2),
						StartPos: Position{Offset: 21, Line: 2, Column: 20},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 22, Line: 2, Column: 21},
			},
			Right: &ArrayExpression{
				Values: []Expression{
					&IntExpression{
						Value:    big.NewInt(3),
						StartPos: Position{Offset: 27, Line: 2, Column: 26},
						EndPos:   Position{Offset: 27, Line: 2, Column: 26},
					},
					&IntExpression{
						Value:    big.NewInt(4),
						StartPos: Position{Offset: 30, Line: 2, Column: 29},
						EndPos:   Position{Offset: 30, Line: 2, Column: 29},
					},
				},
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 31, Line: 2, Column: 30},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionExpressionAndReturn(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let test = fun (): Int { return 1 }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Value: &FunctionExpression{
			ReturnTypeAnnotation: &TypeAnnotation{
				Move: false,
				Type: &NominalType{
					Identifier: Identifier{
						Identifier: "Int",
						Pos:        Position{Offset: 25, Line: 2, Column: 24},
					},
				},
			},
			FunctionBlock: &FunctionBlock{
				Block: &Block{
					Statements: []Statement{
						&ReturnStatement{
							Expression: &IntExpression{
								Value:    big.NewInt(1),
								StartPos: Position{Offset: 38, Line: 2, Column: 37},
								EndPos:   Position{Offset: 38, Line: 2, Column: 37},
							},
							StartPos: Position{Offset: 31, Line: 2, Column: 30},
							EndPos:   Position{Offset: 38, Line: 2, Column: 37},
						},
					},
					StartPos: Position{Offset: 29, Line: 2, Column: 28},
					EndPos:   Position{Offset: 40, Line: 2, Column: 39},
				},
			},
			StartPos: Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionAndBlock(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() { return }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&ReturnStatement{
						StartPos: Position{Offset: 19, Line: 2, Column: 18},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 26, Line: 2, Column: 25},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionParameterWithoutLabel(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test(x: Int) { }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Parameters: []*Parameter{
			{
				Label: "",
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 15, Line: 2, Column: 14},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "Int",
							Pos:        Position{Offset: 18, Line: 2, Column: 17},
						},
					},
				},
				StartPos: Position{Offset: 15, Line: 2, Column: 14},
				EndPos:   Position{Offset: 18, Line: 2, Column: 17},
			},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 21, Line: 2, Column: 20},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				StartPos: Position{Offset: 23, Line: 2, Column: 22},
				EndPos:   Position{Offset: 25, Line: 2, Column: 24},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionParameterWithLabel(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test(x y: Int) { }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		Parameters: []*Parameter{
			{
				Label: "x",
				Identifier: Identifier{
					Identifier: "y",
					Pos:        Position{Offset: 17, Line: 2, Column: 16},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "Int",
							Pos:        Position{Offset: 20, Line: 2, Column: 19},
						},
					},
				},
				StartPos: Position{Offset: 15, Line: 2, Column: 14},
				EndPos:   Position{Offset: 20, Line: 2, Column: 19},
			},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 23, Line: 2, Column: 22},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				StartPos: Position{Offset: 25, Line: 2, Column: 24},
				EndPos:   Position{Offset: 27, Line: 2, Column: 26},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIfStatement(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
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

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&IfStatement{
						Test: &BoolExpression{
							Value:    true,
							StartPos: Position{Offset: 34, Line: 3, Column: 15},
							EndPos:   Position{Offset: 37, Line: 3, Column: 18},
						},
						Then: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Expression: nil,
									StartPos:   Position{Offset: 57, Line: 4, Column: 16},
									EndPos:     Position{Offset: 62, Line: 4, Column: 21},
								},
							},
							StartPos: Position{Offset: 39, Line: 3, Column: 20},
							EndPos:   Position{Offset: 76, Line: 5, Column: 12},
						},
						Else: &Block{
							Statements: []Statement{
								&IfStatement{
									Test: &BoolExpression{
										Value:    false,
										StartPos: Position{Offset: 86, Line: 5, Column: 22},
										EndPos:   Position{Offset: 90, Line: 5, Column: 26},
									},
									Then: &Block{
										Statements: []Statement{
											&ExpressionStatement{
												Expression: &BoolExpression{
													Value:    false,
													StartPos: Position{Offset: 110, Line: 6, Column: 16},
													EndPos:   Position{Offset: 114, Line: 6, Column: 20},
												},
											},
											&ExpressionStatement{
												Expression: &IntExpression{
													Value:    big.NewInt(1),
													StartPos: Position{Offset: 132, Line: 7, Column: 16},
													EndPos:   Position{Offset: 132, Line: 7, Column: 16},
												},
											},
										},
										StartPos: Position{Offset: 92, Line: 5, Column: 28},
										EndPos:   Position{Offset: 146, Line: 8, Column: 12},
									},
									Else: &Block{
										Statements: []Statement{
											&ExpressionStatement{
												Expression: &IntExpression{
													Value:    big.NewInt(2),
													StartPos: Position{Offset: 171, Line: 9, Column: 16},
													EndPos:   Position{Offset: 171, Line: 9, Column: 16},
												},
											},
										},
										StartPos: Position{Offset: 153, Line: 8, Column: 19},
										EndPos:   Position{Offset: 185, Line: 10, Column: 12},
									},
									StartPos: Position{Offset: 83, Line: 5, Column: 19},
								},
							},
							StartPos: Position{Offset: 83, Line: 5, Column: 19},
							EndPos:   Position{Offset: 185, Line: 10, Column: 12},
						},
						StartPos: Position{Offset: 31, Line: 3, Column: 12},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 195, Line: 11, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIfStatementWithVariableDeclaration(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() {
            if var y = x {
                1
            } else {
                2
            }
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&IfStatement{
						Test: &VariableDeclaration{
							IsConstant: false,
							Identifier: Identifier{
								Identifier: "y",
								Pos:        Position{Offset: 38, Line: 3, Column: 19},
							},
							Value: &IdentifierExpression{
								Identifier: Identifier{
									Identifier: "x",
									Pos:        Position{Offset: 42, Line: 3, Column: 23},
								},
							},
							StartPos: Position{Offset: 34, Line: 3, Column: 15},
						},
						Then: &Block{
							Statements: []Statement{
								&ExpressionStatement{
									Expression: &IntExpression{
										Value:    big.NewInt(1),
										StartPos: Position{Offset: 62, Line: 4, Column: 16},
										EndPos:   Position{Offset: 62, Line: 4, Column: 16},
									},
								},
							},
							StartPos: Position{Offset: 44, Line: 3, Column: 25},
							EndPos:   Position{Offset: 76, Line: 5, Column: 12},
						},
						Else: &Block{
							Statements: []Statement{
								&ExpressionStatement{
									Expression: &IntExpression{
										Value:    big.NewInt(2),
										StartPos: Position{Offset: 101, Line: 6, Column: 16},
										EndPos:   Position{Offset: 101, Line: 6, Column: 16},
									},
								},
							},
							StartPos: Position{Offset: 83, Line: 5, Column: 19},
							EndPos:   Position{Offset: 115, Line: 7, Column: 12},
						},
						StartPos: Position{Offset: 31, Line: 3, Column: 12},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 125, Line: 8, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIfStatementNoElse(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() {
            if true {
                return
            }
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&IfStatement{
						Test: &BoolExpression{
							Value:    true,
							StartPos: Position{Offset: 34, Line: 3, Column: 15},
							EndPos:   Position{Offset: 37, Line: 3, Column: 18},
						},
						Then: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Expression: nil,
									StartPos:   Position{Offset: 57, Line: 4, Column: 16},
									EndPos:     Position{Offset: 62, Line: 4, Column: 21},
								},
							},
							StartPos: Position{Offset: 39, Line: 3, Column: 20},
							EndPos:   Position{Offset: 76, Line: 5, Column: 12},
						},
						StartPos: Position{Offset: 31, Line: 3, Column: 12},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 86, Line: 6, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() {
            while true {
              return
              break
              continue
            }
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&WhileStatement{
						Test: &BoolExpression{
							Value:    true,
							StartPos: Position{Offset: 37, Line: 3, Column: 18},
							EndPos:   Position{Offset: 40, Line: 3, Column: 21},
						},
						Block: &Block{
							Statements: []Statement{
								&ReturnStatement{
									Expression: nil,
									StartPos:   Position{Offset: 58, Line: 4, Column: 14},
									EndPos:     Position{Offset: 63, Line: 4, Column: 19},
								},
								&BreakStatement{
									StartPos: Position{Offset: 79, Line: 5, Column: 14},
									EndPos:   Position{Offset: 83, Line: 5, Column: 18},
								},
								&ContinueStatement{
									StartPos: Position{Offset: 99, Line: 6, Column: 14},
									EndPos:   Position{Offset: 106, Line: 6, Column: 21},
								},
							},
							StartPos: Position{Offset: 42, Line: 3, Column: 23},
							EndPos:   Position{Offset: 120, Line: 7, Column: 12},
						},
						StartPos: Position{Offset: 31, Line: 3, Column: 12},
						EndPos:   Position{Offset: 120, Line: 7, Column: 12},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 130, Line: 8, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() {
            a = 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
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
						Value: &IntExpression{
							Value:    big.NewInt(1),
							StartPos: Position{Offset: 35, Line: 3, Column: 16},
							EndPos:   Position{Offset: 35, Line: 3, Column: 16},
						},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 45, Line: 4, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseAccessAssignment(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() {
            x.foo.bar[0][1].baz = 1
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&AssignmentStatement{
						Target: &MemberExpression{
							Expression: &IndexExpression{
								Expression: &IndexExpression{
									Expression: &MemberExpression{
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
									Index: &IntExpression{
										Value:    big.NewInt(0),
										StartPos: Position{Offset: 41, Line: 3, Column: 22},
										EndPos:   Position{Offset: 41, Line: 3, Column: 22},
									},
									StartPos: Position{Offset: 40, Line: 3, Column: 21},
									EndPos:   Position{Offset: 42, Line: 3, Column: 23},
								},
								Index: &IntExpression{
									Value:    big.NewInt(1),
									StartPos: Position{Offset: 44, Line: 3, Column: 25},
									EndPos:   Position{Offset: 44, Line: 3, Column: 25},
								},
								StartPos: Position{Offset: 43, Line: 3, Column: 24},
								EndPos:   Position{Offset: 45, Line: 3, Column: 26},
							},
							Identifier: Identifier{
								Identifier: "baz",
								Pos:        Position{Offset: 47, Line: 3, Column: 28},
							},
						},
						Value: &IntExpression{
							Value:    big.NewInt(1),
							StartPos: Position{Offset: 53, Line: 3, Column: 34},
							EndPos:   Position{Offset: 53, Line: 3, Column: 34},
						},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 63, Line: 4, Column: 8},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseExpressionStatementWithAccess(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    fun test() { x.foo.bar[0][1].baz }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessNotSpecified,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 10, Line: 2, Column: 9},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Pos: Position{Offset: 15, Line: 2, Column: 14},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Statements: []Statement{
					&ExpressionStatement{
						Expression: &MemberExpression{
							Expression: &IndexExpression{
								Expression: &IndexExpression{
									Expression: &MemberExpression{
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
									Index: &IntExpression{
										Value:    big.NewInt(0),
										StartPos: Position{Offset: 29, Line: 2, Column: 28},
										EndPos:   Position{Offset: 29, Line: 2, Column: 28},
									},
									StartPos: Position{Offset: 28, Line: 2, Column: 27},
									EndPos:   Position{Offset: 30, Line: 2, Column: 29},
								},
								Index: &IntExpression{
									Value:    big.NewInt(1),
									StartPos: Position{Offset: 32, Line: 2, Column: 31},
									EndPos:   Position{Offset: 32, Line: 2, Column: 31},
								},
								StartPos: Position{Offset: 31, Line: 2, Column: 30},
								EndPos:   Position{Offset: 33, Line: 2, Column: 32},
							},
							Identifier: Identifier{
								Identifier: "baz",
								Pos:        Position{Offset: 35, Line: 2, Column: 34},
							},
						},
					},
				},
				StartPos: Position{Offset: 17, Line: 2, Column: 16},
				EndPos:   Position{Offset: 39, Line: 2, Column: 38},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseParametersAndArrayTypes(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		pub fun test(a: Int32, b: [Int32; 2], c: [[Int32; 3]]): [[Int64]] {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Access: AccessPublic,
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 11, Line: 2, Column: 10},
		},
		Parameters: []*Parameter{
			{
				Identifier: Identifier{
					Identifier: "a",
					Pos:        Position{Offset: 16, Line: 2, Column: 15},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "Int32",
							Pos:        Position{Offset: 19, Line: 2, Column: 18},
						},
					},
				},
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 19, Line: 2, Column: 18},
			},
			{
				Identifier: Identifier{
					Identifier: "b",
					Pos:        Position{Offset: 26, Line: 2, Column: 25},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &ConstantSizedType{
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int32",
								Pos:        Position{Offset: 30, Line: 2, Column: 29},
							},
						},
						Size:     2,
						StartPos: Position{Offset: 29, Line: 2, Column: 28},
						EndPos:   Position{Offset: 38, Line: 2, Column: 37},
					},
				},
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 38, Line: 2, Column: 37},
			},
			{
				Identifier: Identifier{
					Identifier: "c",
					Pos:        Position{Offset: 41, Line: 2, Column: 40},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &VariableSizedType{
						Type: &ConstantSizedType{
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int32",
									Pos:        Position{Offset: 46, Line: 2, Column: 45},
								},
							},
							Size:     3,
							StartPos: Position{Offset: 45, Line: 2, Column: 44},
							EndPos:   Position{Offset: 54, Line: 2, Column: 53},
						},
						StartPos: Position{Offset: 44, Line: 2, Column: 43},
						EndPos:   Position{Offset: 55, Line: 2, Column: 54},
					},
				},
				StartPos: Position{Offset: 41, Line: 2, Column: 40},
				EndPos:   Position{Offset: 55, Line: 2, Column: 54},
			},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &VariableSizedType{
				Type: &VariableSizedType{
					Type: &NominalType{
						Identifier: Identifier{Identifier: "Int64",
							Pos: Position{Offset: 61, Line: 2, Column: 60},
						},
					},
					StartPos: Position{Offset: 60, Line: 2, Column: 59},
					EndPos:   Position{Offset: 66, Line: 2, Column: 65},
				},
				StartPos: Position{Offset: 59, Line: 2, Column: 58},
				EndPos:   Position{Offset: 67, Line: 2, Column: 66},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				StartPos: Position{Offset: 69, Line: 2, Column: 68},
				EndPos:   Position{Offset: 70, Line: 2, Column: 69},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseDictionaryType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let x: {String: Int} = {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	x := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{Identifier: "x",
			Pos: Position{Offset: 10, Line: 2, Column: 9},
		},
		TypeAnnotation: &TypeAnnotation{
			Move: false,
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
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 25, Line: 2, Column: 24},
			},
		},
		Value: &DictionaryExpression{
			StartPos: Position{Offset: 29, Line: 2, Column: 28},
			EndPos:   Position{Offset: 30, Line: 2, Column: 29},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{x},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIntegerLiterals(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let octal = 0o32
        let hex = 0xf2
        let binary = 0b101010
        let decimal = 1234567890
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	octal := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "octal",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(26),
			StartPos: Position{Offset: 15, Line: 2, Column: 14},
			EndPos:   Position{Offset: 18, Line: 2, Column: 17},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	hex := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "hex",
			Pos:        Position{Offset: 32, Line: 3, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(242),
			StartPos: Position{Offset: 38, Line: 3, Column: 18},
			EndPos:   Position{Offset: 41, Line: 3, Column: 21},
		},
		StartPos: Position{Offset: 28, Line: 3, Column: 8},
	}

	binary := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "binary",
			Pos:        Position{Offset: 55, Line: 4, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(42),
			StartPos: Position{Offset: 64, Line: 4, Column: 21},
			EndPos:   Position{Offset: 71, Line: 4, Column: 28},
		},
		StartPos: Position{Offset: 51, Line: 4, Column: 8},
	}

	decimal := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 85, Line: 5, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(1234567890),
			StartPos: Position{Offset: 95, Line: 5, Column: 22},
			EndPos:   Position{Offset: 104, Line: 5, Column: 31},
		},
		StartPos: Position{Offset: 81, Line: 5, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{octal, hex, binary, decimal},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseIntegerLiteralsWithUnderscores(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let octal = 0o32_45
        let hex = 0xf2_09
        let binary = 0b101010_101010
        let decimal = 1_234_567_890
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	octal := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "octal",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(1701),
			StartPos: Position{Offset: 15, Line: 2, Column: 14},
			EndPos:   Position{Offset: 21, Line: 2, Column: 20},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	hex := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "hex",
			Pos:        Position{Offset: 35, Line: 3, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(61961),
			StartPos: Position{Offset: 41, Line: 3, Column: 18},
			EndPos:   Position{Offset: 47, Line: 3, Column: 24},
		},
		StartPos: Position{Offset: 31, Line: 3, Column: 8},
	}

	binary := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "binary",
			Pos:        Position{Offset: 61, Line: 4, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(2730),
			StartPos: Position{Offset: 70, Line: 4, Column: 21},
			EndPos:   Position{Offset: 84, Line: 4, Column: 35},
		},
		StartPos: Position{Offset: 57, Line: 4, Column: 8},
	}

	decimal := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "decimal",
			Pos:        Position{Offset: 98, Line: 5, Column: 12},
		},

		IsConstant: true,
		Value: &IntExpression{
			Value:    big.NewInt(1234567890),
			StartPos: Position{Offset: 108, Line: 5, Column: 22},
			EndPos:   Position{Offset: 120, Line: 5, Column: 34},
		},
		StartPos: Position{Offset: 94, Line: 5, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{octal, hex, binary, decimal},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInvalidOctalIntegerLiteralWithLeadingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let octal = 0o_32_45
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 15, Line: 2, Column: 14}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 22, Line: 2, Column: 21}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindOctal))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindLeadingUnderscore))
}

func TestParseInvalidOctalIntegerLiteralWithTrailingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let octal = 0o32_45_
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 15, Line: 2, Column: 14}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 22, Line: 2, Column: 21}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindOctal))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindTrailingUnderscore))
}

func TestParseInvalidBinaryIntegerLiteralWithLeadingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let binary = 0b_101010_101010
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 16, Line: 2, Column: 15}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 31, Line: 2, Column: 30}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindBinary))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindLeadingUnderscore))
}

func TestParseInvalidBinaryIntegerLiteralWithTrailingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let binary = 0b101010_101010_
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 16, Line: 2, Column: 15}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 31, Line: 2, Column: 30}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindBinary))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindTrailingUnderscore))
}

func TestParseInvalidDecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let decimal = 1_234_567_890_
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 17, Line: 2, Column: 16}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 30, Line: 2, Column: 29}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindDecimal))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindTrailingUnderscore))
}

func TestParseInvalidHexadecimalIntegerLiteralWithLeadingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let hex = 0x_f2_09
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 13, Line: 2, Column: 12}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 20, Line: 2, Column: 19}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindHexadecimal))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindLeadingUnderscore))
}

func TestParseInvalidHexadecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let hex = 0xf2_09_
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 13, Line: 2, Column: 12}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 20, Line: 2, Column: 19}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindHexadecimal))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindTrailingUnderscore))

}

func TestParseInvalidIntegerLiteral(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let hex = 0z123
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.InvalidIntegerLiteralError)

	Expect(syntaxError.StartPos).
		To(Equal(Position{Offset: 13, Line: 2, Column: 12}))

	Expect(syntaxError.EndPos).
		To(Equal(Position{Offset: 17, Line: 2, Column: 16}))

	Expect(syntaxError.IntegerLiteralKind).
		To(Equal(parser.IntegerLiteralKindUnknown))

	Expect(syntaxError.InvalidIntegerLiteralKind).
		To(Equal(parser.InvalidIntegerLiteralKindUnknownPrefix))
}

func TestParseIntegerTypes(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let a: Int8 = 1
		let b: Int16 = 2
		let c: Int32 = 3
		let d: Int64 = 4
		let e: UInt8 = 5
		let f: UInt16 = 6
		let g: UInt32 = 7
		let h: UInt64 = 8
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int8",
					Pos:        Position{Offset: 10, Line: 2, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(1),
			StartPos: Position{Offset: 17, Line: 2, Column: 16},
			EndPos:   Position{Offset: 17, Line: 2, Column: 16},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int16",
					Pos:        Position{Offset: 28, Line: 3, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(2),
			StartPos: Position{Offset: 36, Line: 3, Column: 17},
			EndPos:   Position{Offset: 36, Line: 3, Column: 17},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "Int32",
					Pos:        Position{Offset: 47, Line: 4, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(3),
			StartPos: Position{Offset: 55, Line: 4, Column: 17},
			EndPos:   Position{Offset: 55, Line: 4, Column: 17},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{

					Identifier: "Int64",
					Pos:        Position{Offset: 66, Line: 5, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(4),
			StartPos: Position{Offset: 74, Line: 5, Column: 17},
			EndPos:   Position{Offset: 74, Line: 5, Column: 17},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt8",
					Pos:        Position{Offset: 85, Line: 6, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(5),
			StartPos: Position{Offset: 93, Line: 6, Column: 17},
			EndPos:   Position{Offset: 93, Line: 6, Column: 17},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt16",
					Pos:        Position{Offset: 104, Line: 7, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(6),
			StartPos: Position{Offset: 113, Line: 7, Column: 18},
			EndPos:   Position{Offset: 113, Line: 7, Column: 18},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt32",
					Pos:        Position{Offset: 124, Line: 8, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(7),
			StartPos: Position{Offset: 133, Line: 8, Column: 18},
			EndPos:   Position{Offset: 133, Line: 8, Column: 18},
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
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "UInt64",
					Pos:        Position{Offset: 144, Line: 9, Column: 9},
				},
			},
		},
		Value: &IntExpression{
			Value:    big.NewInt(8),
			StartPos: Position{Offset: 153, Line: 9, Column: 18},
			EndPos:   Position{Offset: 153, Line: 9, Column: 18},
		},
		StartPos: Position{Offset: 137, Line: 9, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{a, b, c, d, e, f, g, h},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let add: ((Int8, Int16): Int32) = nothing
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	add := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "add",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},
		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{
					{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int8",
								Pos:        Position{Offset: 14, Line: 2, Column: 13},
							},
						},
					},
					{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int16",
								Pos:        Position{Offset: 20, Line: 2, Column: 19},
							},
						},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "Int32",
							Pos:        Position{Offset: 28, Line: 2, Column: 27},
						},
					},
				},
				StartPos: Position{Offset: 12, Line: 2, Column: 11},
				EndPos:   Position{Offset: 32, Line: 2, Column: 31},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "nothing",
				Pos:        Position{Offset: 37, Line: 2, Column: 36},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{add},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionArrayType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let test: [((Int8): Int16); 2] = []
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &ConstantSizedType{
				Type: &FunctionType{
					ParameterTypeAnnotations: []*TypeAnnotation{
						{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int8",
									Pos:        Position{Offset: 16, Line: 2, Column: 15},
								},
							},
						},
					},
					ReturnTypeAnnotation: &TypeAnnotation{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int16",
								Pos:        Position{Offset: 23, Line: 2, Column: 22},
							},
						},
					},
					StartPos: Position{Offset: 14, Line: 2, Column: 13},
					EndPos:   Position{Offset: 27, Line: 2, Column: 26},
				},
				Size:     2,
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 32, Line: 2, Column: 31},
			},
		},
		Value: &ArrayExpression{
			StartPos: Position{Offset: 36, Line: 2, Column: 35},
			EndPos:   Position{Offset: 37, Line: 2, Column: 36},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionTypeWithArrayReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let test: ((Int8): [Int16; 2]) = nothing
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{
					{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int8",
								Pos:        Position{Offset: 15, Line: 2, Column: 14},
							},
						},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &ConstantSizedType{
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int16",
								Pos:        Position{Offset: 23, Line: 2, Column: 22},
							},
						},
						Size:     2,
						StartPos: Position{Offset: 22, Line: 2, Column: 21},
						EndPos:   Position{Offset: 31, Line: 2, Column: 30},
					},
				},
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 31, Line: 2, Column: 30},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "nothing",
				Pos:        Position{Offset: 36, Line: 2, Column: 35},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionTypeWithFunctionReturnTypeInParentheses(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let test: ((Int8): ((Int16): Int32)) = nothing
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{
					{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int8",
								Pos:        Position{Offset: 15, Line: 2, Column: 14},
							},
						},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								Move: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int16",
										Pos:        Position{Offset: 24, Line: 2, Column: 23},
									},
								},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int32",
									Pos:        Position{Offset: 32, Line: 2, Column: 31},
								},
							},
						},
						StartPos: Position{Offset: 22, Line: 2, Column: 21},
						EndPos:   Position{Offset: 36, Line: 2, Column: 35},
					},
				},
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 36, Line: 2, Column: 35},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "nothing",
				Pos:        Position{Offset: 42, Line: 2, Column: 41},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionTypeWithFunctionReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let test: ((Int8): ((Int16): Int32)) = nothing
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{
					{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int8",
								Pos:        Position{Offset: 15, Line: 2, Column: 14},
							},
						},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &FunctionType{
						ParameterTypeAnnotations: []*TypeAnnotation{
							{
								Move: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int16",
										Pos:        Position{Offset: 24, Line: 2, Column: 23},
									},
								},
							},
						},
						ReturnTypeAnnotation: &TypeAnnotation{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int32",
									Pos:        Position{Offset: 32, Line: 2, Column: 31},
								},
							},
						},
						StartPos: Position{Offset: 22, Line: 2, Column: 21},
						EndPos:   Position{Offset: 36, Line: 2, Column: 35},
					},
				},
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 36, Line: 2, Column: 35},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "nothing",
				Pos:        Position{Offset: 42, Line: 2, Column: 41},
			},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMissingReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
		let noop: ((): Void) =
            fun () { return }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	noop := &VariableDeclaration{
		Identifier: Identifier{
			Identifier: "noop",
			Pos:        Position{Offset: 7, Line: 2, Column: 6},
		},

		IsConstant: true,
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "Void",
							Pos:        Position{Offset: 18, Line: 2, Column: 17},
						},
					},
				},
				StartPos: Position{Offset: 13, Line: 2, Column: 12},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		Value: &FunctionExpression{
			ReturnTypeAnnotation: &TypeAnnotation{
				Move: false,
				Type: &NominalType{
					Identifier: Identifier{
						Pos: Position{Offset: 43, Line: 3, Column: 17},
					},
				},
			},
			FunctionBlock: &FunctionBlock{
				Block: &Block{
					Statements: []Statement{
						&ReturnStatement{
							StartPos: Position{Offset: 47, Line: 3, Column: 21},
							EndPos:   Position{Offset: 52, Line: 3, Column: 26},
						},
					},
					StartPos: Position{Offset: 45, Line: 3, Column: 19},
					EndPos:   Position{Offset: 54, Line: 3, Column: 28},
				},
			},
			StartPos: Position{Offset: 38, Line: 3, Column: 12},
		},
		StartPos: Position{Offset: 3, Line: 2, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{noop},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseLeftAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = 1 + 2 + 3
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &BinaryExpression{
			Operation: OperationPlus,
			Left: &BinaryExpression{
				Operation: OperationPlus,
				Left: &IntExpression{
					Value:    big.NewInt(1),
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 17, Line: 2, Column: 16},
				},
				Right: &IntExpression{
					Value:    big.NewInt(2),
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			Right: &IntExpression{
				Value:    big.NewInt(3),
				StartPos: Position{Offset: 25, Line: 2, Column: 24},
				EndPos:   Position{Offset: 25, Line: 2, Column: 24},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInvalidDoubleIntegerUnary(t *testing.T) {
	RegisterTestingT(t)

	program, err := parser.ParseProgram(`
	   var a = 1
	   let b = --a
	`)

	Expect(program).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	Expect(err.(parser.Error).Errors).
		To(Equal([]error{
			&parser.JuxtaposedUnaryOperatorsError{
				Pos: Position{Offset: 27, Line: 3, Column: 12},
			},
		}))
}

func TestParseInvalidDoubleBooleanUnary(t *testing.T) {
	RegisterTestingT(t)

	program, err := parser.ParseProgram(`
	   let b = !!true
	`)

	Expect(program).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	Expect(err.(parser.Error).Errors).
		To(Equal([]error{
			&parser.JuxtaposedUnaryOperatorsError{
				Pos: Position{Offset: 13, Line: 2, Column: 12},
			},
		}))
}

func TestParseTernaryRightAssociativity(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let a = 2 > 1
          ? 0
          : 3 > 2 ? 1 : 2
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	a := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "a",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &ConditionalExpression{
			Test: &BinaryExpression{
				Operation: OperationGreater,
				Left: &IntExpression{
					Value:    big.NewInt(2),
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 17, Line: 2, Column: 16},
				},
				Right: &IntExpression{
					Value:    big.NewInt(1),
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
			Then: &IntExpression{
				Value:    big.NewInt(0),
				StartPos: Position{Offset: 35, Line: 3, Column: 12},
				EndPos:   Position{Offset: 35, Line: 3, Column: 12},
			},
			Else: &ConditionalExpression{
				Test: &BinaryExpression{
					Operation: OperationGreater,
					Left: &IntExpression{
						Value:    big.NewInt(3),
						StartPos: Position{Offset: 49, Line: 4, Column: 12},
						EndPos:   Position{Offset: 49, Line: 4, Column: 12},
					},
					Right: &IntExpression{
						Value:    big.NewInt(2),
						StartPos: Position{Offset: 53, Line: 4, Column: 16},
						EndPos:   Position{Offset: 53, Line: 4, Column: 16},
					},
				},
				Then: &IntExpression{
					Value:    big.NewInt(1),
					StartPos: Position{Offset: 57, Line: 4, Column: 20},
					EndPos:   Position{Offset: 57, Line: 4, Column: 20},
				},
				Else: &IntExpression{
					Value:    big.NewInt(2),
					StartPos: Position{Offset: 61, Line: 4, Column: 24},
					EndPos:   Position{Offset: 61, Line: 4, Column: 24},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseStructure(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
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

	Expect(err).
		To(Not(HaveOccurred()))

	test := &CompositeDeclaration{
		CompositeKind: common.CompositeKindStructure,
		Identifier: Identifier{
			Identifier: "Test",
			Pos:        Position{Offset: 16, Line: 2, Column: 15},
		},
		Conformances: []*NominalType{},
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
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 53, Line: 3, Column: 30},
							},
						},
					},
					StartPos: Position{Offset: 35, Line: 3, Column: 12},
					EndPos:   Position{Offset: 55, Line: 3, Column: 32},
				},
			},
			Initializers: []*InitializerDeclaration{
				{
					Identifier: Identifier{
						Identifier: "init",
						Pos:        Position{Offset: 70, Line: 5, Column: 12},
					},
					Parameters: []*Parameter{
						{
							Label: "",
							Identifier: Identifier{
								Identifier: "foo",
								Pos:        Position{Offset: 75, Line: 5, Column: 17},
							},
							TypeAnnotation: &TypeAnnotation{
								Move: false,
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int",
										Pos:        Position{Offset: 80, Line: 5, Column: 22},
									},
								},
							},
							StartPos: Position{Offset: 75, Line: 5, Column: 17},
							EndPos:   Position{Offset: 80, Line: 5, Column: 22},
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
									Value: &IdentifierExpression{
										Identifier: Identifier{
											Identifier: "foo",
											Pos:        Position{Offset: 114, Line: 6, Column: 27},
										},
									},
								},
							},
							StartPos: Position{Offset: 85, Line: 5, Column: 27},
							EndPos:   Position{Offset: 130, Line: 7, Column: 12},
						},
					},
					StartPos: Position{Offset: 70, Line: 5, Column: 12},
				},
			},
			Functions: []*FunctionDeclaration{
				{
					Access: AccessPublic,
					Identifier: Identifier{
						Identifier: "getFoo",
						Pos:        Position{Offset: 153, Line: 9, Column: 20},
					},

					Parameters: nil,
					ReturnTypeAnnotation: &TypeAnnotation{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 163, Line: 9, Column: 30},
							},
						},
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
									StartPos: Position{Offset: 185, Line: 10, Column: 16},
									EndPos:   Position{Offset: 199, Line: 10, Column: 30},
								},
							},
							StartPos: Position{Offset: 167, Line: 9, Column: 34},
							EndPos:   Position{Offset: 213, Line: 11, Column: 12},
						},
					},
					StartPos: Position{Offset: 145, Line: 9, Column: 12},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
		EndPos:   Position{Offset: 223, Line: 12, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseStructureWithConformances(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        struct Test: Foo, Bar {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &CompositeDeclaration{
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
		Members:  &Members{},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
		EndPos:   Position{Offset: 32, Line: 2, Column: 31},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInvalidStructureWithMissingFunctionBlock(t *testing.T) {
	RegisterTestingT(t)

	_, err := parser.ParseProgram(`
        struct Test {
            pub fun getFoo(): Int
        }
	`)

	Expect(err).
		To(HaveOccurred())
}

func TestParsePreAndPostConditions(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
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
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Parameters: []*Parameter{
					{
						Label: "",
						Identifier: Identifier{
							Identifier: "n",
							Pos:        Position{Offset: 18, Line: 2, Column: 17},
						},
						TypeAnnotation: &TypeAnnotation{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 21, Line: 2, Column: 20},
								},
							},
						},
						StartPos: Position{Offset: 18, Line: 2, Column: 17},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 24, Line: 2, Column: 23},
						},
					},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: &IntExpression{
									Value:    big.NewInt(0),
									StartPos: Position{Offset: 185, Line: 10, Column: 19},
									EndPos:   Position{Offset: 185, Line: 10, Column: 19},
								},
								StartPos: Position{Offset: 178, Line: 10, Column: 12},
								EndPos:   Position{Offset: 185, Line: 10, Column: 19},
							},
						},
						StartPos: Position{Offset: 26, Line: 2, Column: 25},
						EndPos:   Position{Offset: 195, Line: 11, Column: 8},
					},
					PreConditions: []*Condition{
						{
							Kind: ConditionKindPre,
							Test: &BinaryExpression{
								Operation: OperationUnequal,
								Left: &IdentifierExpression{
									Identifier: Identifier{
										Identifier: "n",
										Pos:        Position{Offset: 62, Line: 4, Column: 16},
									},
								},
								Right: &IntExpression{
									Value:    big.NewInt(0),
									StartPos: Position{Offset: 67, Line: 4, Column: 21},
									EndPos:   Position{Offset: 67, Line: 4, Column: 21},
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
								Right: &IntExpression{
									Value:    big.NewInt(0),
									StartPos: Position{Offset: 89, Line: 5, Column: 20},
									EndPos:   Position{Offset: 89, Line: 5, Column: 20},
								},
							},
						},
					},
					PostConditions: []*Condition{
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
								Right: &IntExpression{
									Value:    big.NewInt(0),
									StartPos: Position{Offset: 150, Line: 8, Column: 26},
									EndPos:   Position{Offset: 150, Line: 8, Column: 26},
								},
							},
						},
					},
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseExpression(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseExpression(`
        before(x + before(y)) + z
	`)

	Expect(err).
		To(Not(HaveOccurred()))

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

	Expect(actual).
		To(Equal(expected))
}

func TestParseString(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseExpression(`
       "test \0\n\r\t\"\'\\ xyz"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &StringExpression{
		Value:    "test \x00\n\r\t\"'\\ xyz",
		StartPos: Position{Offset: 8, Line: 2, Column: 7},
		EndPos:   Position{Offset: 32, Line: 2, Column: 31},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseStringWithUnicode(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseExpression(`
      "this is a test \t\\new line and race car:\n\u{1F3CE}\u{FE0F}"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &StringExpression{
		Value:    "this is a test \t\\new line and race car:\n\U0001F3CE\uFE0F",
		StartPos: Position{Offset: 7, Line: 2, Column: 6},
		EndPos:   Position{Offset: 68, Line: 2, Column: 67},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseConditionMessage(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        fun test(n: Int) {
            pre {
                n >= 0: "n must be positive"
            }
            return n
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&FunctionDeclaration{
				Access: AccessNotSpecified,
				Identifier: Identifier{
					Identifier: "test",
					Pos:        Position{Offset: 13, Line: 2, Column: 12},
				},
				Parameters: []*Parameter{
					{
						Label: "",
						Identifier: Identifier{Identifier: "n",
							Pos: Position{Offset: 18, Line: 2, Column: 17},
						},
						TypeAnnotation: &TypeAnnotation{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 21, Line: 2, Column: 20},
								},
							},
						},
						StartPos: Position{Offset: 18, Line: 2, Column: 17},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: false,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "",
							Pos:        Position{Offset: 24, Line: 2, Column: 23},
						},
					},
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
								StartPos: Position{Offset: 117, Line: 6, Column: 12},
								EndPos:   Position{Offset: 124, Line: 6, Column: 19},
							},
						},
						StartPos: Position{Offset: 26, Line: 2, Column: 25},
						EndPos:   Position{Offset: 134, Line: 7, Column: 8},
					},
					PreConditions: []*Condition{
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
								Right: &IntExpression{
									Value:    big.NewInt(0),
									StartPos: Position{Offset: 67, Line: 4, Column: 21},
									EndPos:   Position{Offset: 67, Line: 4, Column: 21},
								},
							},
							Message: &StringExpression{
								Value:    "n must be positive",
								StartPos: Position{Offset: 70, Line: 4, Column: 24},
								EndPos:   Position{Offset: 89, Line: 4, Column: 43},
							},
						},
					},
					PostConditions: nil,
				},
				StartPos: Position{Offset: 9, Line: 2, Column: 8},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseOptionalType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
       let x: Int?? = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: false,
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
				},
				Value: &IntExpression{
					Value:    big.NewInt(1),
					StartPos: Position{Offset: 23, Line: 2, Column: 22},
					EndPos:   Position{Offset: 23, Line: 2, Column: 22},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseNilCoalescing(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
       let x = nil ?? 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &BinaryExpression{
					Operation: OperationNilCoalesce,
					Left: &NilExpression{
						Pos: Position{Offset: 16, Line: 2, Column: 15},
					},
					Right: &IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseNilCoalescingRightAssociativity(t *testing.T) {
	RegisterTestingT(t)

	// NOTE: only syntactically, not semantically valid
	actual, err := parser.ParseProgram(`
       let x = 1 ?? 2 ?? 3
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &BinaryExpression{
					Operation: OperationNilCoalesce,
					Left: &IntExpression{
						Value:    big.NewInt(1),
						StartPos: Position{Offset: 16, Line: 2, Column: 15},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
					Right: &BinaryExpression{
						Operation: OperationNilCoalesce,
						Left: &IntExpression{
							Value:    big.NewInt(2),
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
						Right: &IntExpression{
							Value:    big.NewInt(3),
							StartPos: Position{Offset: 26, Line: 2, Column: 25},
							EndPos:   Position{Offset: 26, Line: 2, Column: 25},
						},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFailableDowncasting(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
       let x = 0 as? Int
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	expected := &Program{
		Declarations: []Declaration{
			&VariableDeclaration{
				IsConstant: true,
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 12, Line: 2, Column: 11},
				},
				Value: &FailableDowncastExpression{
					Expression: &IntExpression{
						Value:    big.NewInt(0),
						StartPos: Position{Offset: 16, Line: 2, Column: 15},
						EndPos:   Position{Offset: 16, Line: 2, Column: 15},
					},
					TypeAnnotation: &TypeAnnotation{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 22, Line: 2, Column: 21},
							},
						},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseInterface(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		actual, err := parser.ParseProgram(fmt.Sprintf(`
            %s interface Test {
                foo: Int

                init(foo: Int)

                fun getFoo(): Int
            }
	    `, kind.Keyword()))

		Expect(err).
			To(Not(HaveOccurred()))

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
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 58, Line: 3, Column: 21},
								},
							},
						},
						StartPos: Position{Offset: 53, Line: 3, Column: 16},
						EndPos:   Position{Offset: 60, Line: 3, Column: 23},
					},
				},
				Initializers: []*InitializerDeclaration{
					{
						Identifier: Identifier{
							Identifier: "init",
							Pos:        Position{Offset: 79, Line: 5, Column: 16},
						},
						Parameters: []*Parameter{
							{
								Label: "",
								Identifier: Identifier{
									Identifier: "foo",
									Pos:        Position{Offset: 84, Line: 5, Column: 21},
								},
								TypeAnnotation: &TypeAnnotation{
									Move: false,
									Type: &NominalType{
										Identifier: Identifier{
											Identifier: "Int",
											Pos:        Position{Offset: 89, Line: 5, Column: 26},
										},
									},
								},
								StartPos: Position{Offset: 84, Line: 5, Column: 21},
								EndPos:   Position{Offset: 89, Line: 5, Column: 26},
							},
						},
						FunctionBlock: nil,
						StartPos:      Position{Offset: 79, Line: 5, Column: 16},
					},
				},
				Functions: []*FunctionDeclaration{
					{
						Access: AccessNotSpecified,
						Identifier: Identifier{
							Identifier: "getFoo",
							Pos:        Position{Offset: 115, Line: 7, Column: 20},
						},
						Parameters: nil,
						ReturnTypeAnnotation: &TypeAnnotation{
							Move: false,
							Type: &NominalType{
								Identifier: Identifier{
									Identifier: "Int",
									Pos:        Position{Offset: 125, Line: 7, Column: 30},
								},
							},
						},
						FunctionBlock: nil,
						StartPos:      Position{Offset: 111, Line: 7, Column: 16},
					},
				},
			},
			StartPos: Position{Offset: 13, Line: 2, Column: 12},
			EndPos:   Position{Offset: 141, Line: 8, Column: 12},
		}

		expected := &Program{
			Declarations: []Declaration{test},
		}

		Expect(actual).
			To(Equal(expected))
	}
}

func TestParseImportWithString(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        import "test.bpl"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &ImportDeclaration{
		Identifiers: []Identifier{},
		Location:    StringImportLocation("test.bpl"),
		StartPos:    Position{Offset: 9, Line: 2, Column: 8},
		EndPos:      Position{Offset: 25, Line: 2, Column: 24},
		LocationPos: Position{Offset: 16, Line: 2, Column: 15},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))

	Expect(actual.Imports()).
		To(Equal(map[ImportLocation]*Program{
			StringImportLocation("test.bpl"): nil,
		}))

	actual.Imports()[StringImportLocation("test.bpl")] = &Program{}

	Expect(actual.Imports()).
		To(Equal(map[ImportLocation]*Program{
			StringImportLocation("test.bpl"): {},
		}))
}

func TestParseImportWithAddress(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        import 0x1234
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &ImportDeclaration{
		Identifiers: []Identifier{},
		Location:    AddressImportLocation([]byte{18, 52}),
		StartPos:    Position{Offset: 9, Line: 2, Column: 8},
		EndPos:      Position{Offset: 21, Line: 2, Column: 20},
		LocationPos: Position{Offset: 16, Line: 2, Column: 15},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))

	Expect(actual.Imports()).
		To(Equal(map[ImportLocation]*Program{
			AddressImportLocation([]byte{18, 52}): nil,
		}))

	actual.Imports()[AddressImportLocation([]byte{18, 52})] = &Program{}

	Expect(actual.Imports()).
		To(Equal(map[ImportLocation]*Program{
			AddressImportLocation([]byte{18, 52}): {},
		}))

}

func TestParseImportWithIdentifiers(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        import A, b from 0x0
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &ImportDeclaration{
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
		Location:    AddressImportLocation([]byte{0}),
		StartPos:    Position{Offset: 9, Line: 2, Column: 8},
		EndPos:      Position{Offset: 28, Line: 2, Column: 27},
		LocationPos: Position{Offset: 26, Line: 2, Column: 25},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFieldWithFromIdentifier(t *testing.T) {
	RegisterTestingT(t)

	_, err := parser.ParseProgram(`
      struct S {
          let from: String
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestParseFunctionWithFromIdentifier(t *testing.T) {
	RegisterTestingT(t)

	_, err := parser.ParseProgram(`
        fun send(from: String, to: String) {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestParseImportWithFromIdentifier(t *testing.T) {
	RegisterTestingT(t)

	_, err := parser.ParseProgram(`
        import from from 0x0
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestParseSemicolonsBetweenDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := parser.ParseProgram(`
        import from from 0x0;
        fun foo() {}; 
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestParseInvalidTypeWithWhitespace(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
	    let x: Int ? = 1
	`)

	Expect(actual).
		To(BeNil())

	Expect(err).
		To(BeAssignableToTypeOf(parser.Error{}))

	errors := err.(parser.Error).Errors
	Expect(errors).
		To(HaveLen(1))

	syntaxError := errors[0].(*parser.SyntaxError)

	Expect(syntaxError.Pos).
		To(Equal(Position{Offset: 17, Line: 2, Column: 16}))

	Expect(syntaxError.Message).
		To(ContainSubstring("no viable alternative"))
}

func TestParseResource(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        resource Test {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &CompositeDeclaration{
		CompositeKind: common.CompositeKindResource,
		Identifier: Identifier{
			Identifier: "Test",
			Pos:        Position{Offset: 18, Line: 2, Column: 17},
		},
		Conformances: []*NominalType{},
		Members:      &Members{},
		StartPos:     Position{Offset: 9, Line: 2, Column: 8},
		EndPos:       Position{Offset: 24, Line: 2, Column: 23},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMoveReturnType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        fun test(): <-X {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: true,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "X",
					Pos:        Position{Offset: 23, Line: 2, Column: 22},
				},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				StartPos: Position{Offset: 25, Line: 2, Column: 24},
				EndPos:   Position{Offset: 26, Line: 2, Column: 25},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMovingVariableDeclaration(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let x <- y
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
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
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMoveStatement(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        fun test() {
            x <- y
        }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "",
					Pos:        Position{Offset: 18, Line: 2, Column: 17},
				},
			},
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
						Value: &IdentifierExpression{
							Identifier: Identifier{
								Identifier: "y",
								Pos:        Position{Offset: 39, Line: 3, Column: 17},
							},
						},
					},
				},
				StartPos: Position{Offset: 20, Line: 2, Column: 19},
				EndPos:   Position{Offset: 49, Line: 4, Column: 8},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMoveOperator(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
      let x = foo(<-y)
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "x",
			Pos:        Position{Offset: 11, Line: 2, Column: 10},
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
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
			},
			EndPos: Position{Offset: 22, Line: 2, Column: 21},
		},
		StartPos: Position{Offset: 7, Line: 2, Column: 6},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMoveParameterType(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        fun test(x: <-X) {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &FunctionDeclaration{
		Identifier: Identifier{
			Identifier: "test",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "",
					Pos:        Position{Offset: 24, Line: 2, Column: 23},
				},
			},
		},
		Parameters: []*Parameter{
			{
				Label: "",
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 18, Line: 2, Column: 17},
				},
				TypeAnnotation: &TypeAnnotation{
					Move: true,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "X",
							Pos:        Position{Offset: 23, Line: 2, Column: 22},
						},
					},
				},
				StartPos: Position{Offset: 18, Line: 2, Column: 17},
				EndPos:   Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				StartPos: Position{Offset: 26, Line: 2, Column: 25},
				EndPos:   Position{Offset: 27, Line: 2, Column: 26},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseMovingVariableDeclarationWithTypeAnnotation(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let x: <-R <- y
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "x",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		TypeAnnotation: &TypeAnnotation{
			Move: true,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "R",
					Pos:        Position{Offset: 18, Line: 2, Column: 17},
				},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "y",
				Pos:        Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFieldDeclarationWithMoveTypeAnnotation(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        struct X { x: <-R }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &CompositeDeclaration{
		CompositeKind: common.CompositeKindStructure,
		Identifier: Identifier{
			Identifier: "X",
			Pos:        Position{Offset: 16, Line: 2, Column: 15},
		},
		Conformances: []*NominalType{},
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
						Move: true,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "R",
								Pos:        Position{Offset: 25, Line: 2, Column: 24},
							},
						},
					},
					StartPos: Position{Offset: 20, Line: 2, Column: 19},
					EndPos:   Position{Offset: 25, Line: 2, Column: 24},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
		EndPos:   Position{Offset: 27, Line: 2, Column: 26},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionTypeWithMoveTypeAnnotation(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let f: ((): <-R) = g
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "f",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		TypeAnnotation: &TypeAnnotation{
			Move: false,
			Type: &FunctionType{
				ParameterTypeAnnotations: nil,
				ReturnTypeAnnotation: &TypeAnnotation{
					Move: true,
					Type: &NominalType{
						Identifier: Identifier{
							Identifier: "R",
							Pos:        Position{Offset: 23, Line: 2, Column: 22},
						},
					},
				},
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "g",
				Pos:        Position{Offset: 28, Line: 2, Column: 27},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFunctionExpressionWithMoveTypeAnnotation(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let f = fun (): <-R { return X }
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "f",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Value: &FunctionExpression{
			ReturnTypeAnnotation: &TypeAnnotation{
				Move: true,
				Type: &NominalType{
					Identifier: Identifier{
						Identifier: "R",
						Pos:        Position{Offset: 27, Line: 2, Column: 26},
					},
				},
			},
			FunctionBlock: &FunctionBlock{
				Block: &Block{
					Statements: []Statement{
						&ReturnStatement{
							Expression: &IdentifierExpression{
								Identifier: Identifier{
									Identifier: "X",
									Pos:        Position{Offset: 38, Line: 2, Column: 37},
								},
							},
							StartPos: Position{Offset: 31, Line: 2, Column: 30},
							EndPos:   Position{Offset: 38, Line: 2, Column: 37},
						},
					},
					StartPos: Position{Offset: 29, Line: 2, Column: 28},
					EndPos:   Position{Offset: 40, Line: 2, Column: 39},
				},
			},
			StartPos: Position{Offset: 17, Line: 2, Column: 16},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

func TestParseFailableDowncastingMoveTypeAnnotation(t *testing.T) {
	RegisterTestingT(t)

	actual, err := parser.ParseProgram(`
        let y = x as? <-R
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "y",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		TypeAnnotation: nil,
		Value: &FailableDowncastExpression{
			Expression: &IdentifierExpression{
				Identifier: Identifier{
					Identifier: "x",
					Pos:        Position{Offset: 17, Line: 2, Column: 16},
				},
			},
			TypeAnnotation: &TypeAnnotation{
				Move: true,
				Type: &NominalType{
					Identifier: Identifier{
						Identifier: "R",
						Pos:        Position{Offset: 25, Line: 2, Column: 24},
					},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	Expect(actual).
		To(Equal(expected))
}

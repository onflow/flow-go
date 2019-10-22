package tests

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	. "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
)

func TestParseInvalidIncompleteConstKeyword(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    le
	`)

	assert.Nil(t, actual)

	assert.IsType(t, parser.Error{}, err)

	errors := err.(parser.Error).Errors
	assert.Len(t, errors, 1)

	syntaxError := errors[0].(*parser.SyntaxError)

	assert.Equal(t,
		Position{Offset: 6, Line: 2, Column: 5},
		syntaxError.Pos,
	)

	assert.Contains(t, syntaxError.Message, "extraneous input")
}

func TestParseInvalidIncompleteConstantDeclaration1(t *testing.T) {

	actual, inputIsComplete, err := parser.ParseProgram(`
	    let
	`)

	assert.False(t, inputIsComplete)

	assert.Nil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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

	actual, inputIsComplete, err := parser.ParseProgram(`
	    let =
	`)

	assert.False(t, inputIsComplete)

	assert.Nil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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

func TestParseBoolExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = true
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIdentifierExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let b = a
	`)

	assert.Nil(t, err)

	b := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{b},
	}

	assert.Equal(t, expected, actual)
}

func TestParseArrayExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = [1, 2]
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
				&IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
						EndPos:   Position{Offset: 15, Line: 2, Column: 14},
					},
				},
				&IntExpression{
					Value: big.NewInt(2),
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseDictionaryExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let x = {"a": 1, "b": 2}
	`)

	assert.Nil(t, err)

	x := &VariableDeclaration{
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
					Value: &IntExpression{
						Value: big.NewInt(1),
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
					Value: &IntExpression{
						Value: big.NewInt(2),
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
	}

	expected := &Program{
		Declarations: []Declaration{x},
	}

	assert.Equal(t, expected, actual)
}

func TestParseInvocationExpressionWithoutLabels(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = b(1, 2)
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
					Expression: &IntExpression{
						Value: big.NewInt(1),
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 16, Line: 2, Column: 15},
						},
					},
				},
				{
					Label: "",
					Expression: &IntExpression{
						Value: big.NewInt(2),
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseInvocationExpressionWithLabels(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = b(x: 1, y: 2)
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
					Expression: &IntExpression{
						Value: big.NewInt(1),
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
					Expression: &IntExpression{
						Value: big.NewInt(2),
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMemberExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = b.c
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIndexExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let a = b[1]
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			IndexingExpression: &IntExpression{
				Value: big.NewInt(1),
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseUnaryExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let foo = -boo
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			Range: Range{
				StartPos: Position{Offset: 16, Line: 2, Column: 15},
				EndPos:   Position{Offset: 19, Line: 2, Column: 18},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseOrExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = false || true
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseAndExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = false && true
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseEqualityExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = false == true
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseRelationalExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = 1 < 2
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			Left: &IntExpression{
				Value: big.NewInt(1),
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 17, Line: 2, Column: 16},
				},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Range: Range{
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseAdditiveExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = 1 + 2
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			Left: &IntExpression{
				Value: big.NewInt(1),
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 17, Line: 2, Column: 16},
				},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Range: Range{
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMultiplicativeExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = 1 * 2
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			Left: &IntExpression{
				Value: big.NewInt(1),
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 17, Line: 2, Column: 16},
				},
			},
			Right: &IntExpression{
				Value: big.NewInt(2),
				Range: Range{
					StartPos: Position{Offset: 21, Line: 2, Column: 20},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseConcatenatingExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = [1, 2] & [3, 4]
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
			Operation: OperationConcat,
			Left: &ArrayExpression{
				Values: []Expression{
					&IntExpression{
						Value: big.NewInt(1),
						Range: Range{
							StartPos: Position{Offset: 18, Line: 2, Column: 17},
							EndPos:   Position{Offset: 18, Line: 2, Column: 17},
						},
					},
					&IntExpression{
						Value: big.NewInt(2),
						Range: Range{
							StartPos: Position{Offset: 21, Line: 2, Column: 20},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 22, Line: 2, Column: 21},
				},
			},
			Right: &ArrayExpression{
				Values: []Expression{
					&IntExpression{
						Value: big.NewInt(3),
						Range: Range{
							StartPos: Position{Offset: 27, Line: 2, Column: 26},
							EndPos:   Position{Offset: 27, Line: 2, Column: 26},
						},
					},
					&IntExpression{
						Value: big.NewInt(4),
						Range: Range{
							StartPos: Position{Offset: 30, Line: 2, Column: 29},
							EndPos:   Position{Offset: 30, Line: 2, Column: 29},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 26, Line: 2, Column: 25},
					EndPos:   Position{Offset: 31, Line: 2, Column: 30},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionExpressionAndReturn(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let test = fun (): Int { return 1 }
	`)

	assert.Nil(t, err)

	test := &VariableDeclaration{
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
				Move: false,
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
							Expression: &IntExpression{
								Value: big.NewInt(1),
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionAndBlock(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() { return }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionParameterWithoutLabel(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test(x: Int) { }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
						Move: false,
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
						EndPos:   Position{Offset: 18, Line: 2, Column: 17},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 14, Line: 2, Column: 13},
				EndPos:   Position{Offset: 21, Line: 2, Column: 20},
			},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionParameterWithLabel(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test(x y: Int) { }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
						Move: false,
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
						EndPos:   Position{Offset: 20, Line: 2, Column: 19},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 14, Line: 2, Column: 13},
				EndPos:   Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		ReturnTypeAnnotation: &TypeAnnotation{
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIfStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
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

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
												Expression: &IntExpression{
													Value: big.NewInt(1),
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
												Expression: &IntExpression{
													Value: big.NewInt(2),
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIfStatementWithVariableDeclaration(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() {
            if var y = x {
                1
            } else {
                2
            }
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
						Test: &VariableDeclaration{
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
							StartPos: Position{Offset: 34, Line: 3, Column: 15},
						},
						Then: &Block{
							Statements: []Statement{
								&ExpressionStatement{
									Expression: &IntExpression{
										Value: big.NewInt(1),
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
									Expression: &IntExpression{
										Value: big.NewInt(2),
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
					},
				},
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 125, Line: 8, Column: 8},
				},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIfStatementNoElse(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() {
            if true {
                return
            }
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseWhileStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() {
            while true {
              return
              break
              continue
            }
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
						Range: Range{
							StartPos: Position{Offset: 31, Line: 3, Column: 12},
							EndPos:   Position{Offset: 120, Line: 7, Column: 12},
						},
					},
				},
				Range: Range{
					StartPos: Position{Offset: 17, Line: 2, Column: 16},
					EndPos:   Position{Offset: 130, Line: 8, Column: 8},
				},
			},
		},
		StartPos: Position{Offset: 6, Line: 2, Column: 5},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseAssignment(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() {
            a = 1
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
						Value: &IntExpression{
							Value: big.NewInt(1),
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseAccessAssignment(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() {
            x.foo.bar[0][1].baz = 1
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
									IndexingExpression: &IntExpression{
										Value: big.NewInt(0),
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
								IndexingExpression: &IntExpression{
									Value: big.NewInt(1),
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
						Value: &IntExpression{
							Value: big.NewInt(1),
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseExpressionStatementWithAccess(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    fun test() { x.foo.bar[0][1].baz }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
									IndexingExpression: &IntExpression{
										Value: big.NewInt(0),
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
								IndexingExpression: &IntExpression{
									Value: big.NewInt(1),
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseParametersAndArrayTypes(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		pub fun test(a: Int32, b: [Int32; 2], c: [[Int32; 3]]): [[Int64]] {}
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
						Move: false,
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
						EndPos:   Position{Offset: 19, Line: 2, Column: 18},
					},
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
							Size: 2,
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
						Move: false,
						Type: &VariableSizedType{
							Type: &ConstantSizedType{
								Type: &NominalType{
									Identifier: Identifier{
										Identifier: "Int32",
										Pos:        Position{Offset: 46, Line: 2, Column: 45},
									},
								},
								Size: 3,
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
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseDictionaryType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let x: {String: Int} = {}
	`)

	assert.Nil(t, err)

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
	}

	expected := &Program{
		Declarations: []Declaration{x},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIntegerLiterals(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32
        let hex = 0xf2
        let binary = 0b101010
        let decimal = 1234567890
	`)

	assert.Nil(t, err)

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
		Value: &IntExpression{
			Value: big.NewInt(26),
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
		Value: &IntExpression{
			Value: big.NewInt(242),
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
		Value: &IntExpression{
			Value: big.NewInt(42),
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
		Value: &IntExpression{
			Value: big.NewInt(1234567890),
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

	assert.Equal(t, expected, actual)
}

func TestParseIntegerLiteralsWithUnderscores(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32_45
        let hex = 0xf2_09
        let binary = 0b101010_101010
        let decimal = 1_234_567_890
	`)

	assert.Nil(t, err)

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
		Value: &IntExpression{
			Value: big.NewInt(1701),
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
		Value: &IntExpression{
			Value: big.NewInt(61961),
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
		Value: &IntExpression{
			Value: big.NewInt(2730),
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
		Value: &IntExpression{
			Value: big.NewInt(1234567890),
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

	assert.Equal(t, expected, actual)
}

func TestParseInvalidOctalIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let octal = 0o_32_45
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidOctalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let octal = 0o32_45_
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidBinaryIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let binary = 0b_101010_101010
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidBinaryIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let binary = 0b101010_101010_
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidDecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let decimal = 1_234_567_890_
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidHexadecimalIntegerLiteralWithLeadingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let hex = 0x_f2_09
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindLeadingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseInvalidHexadecimalIntegerLiteralWithTrailingUnderscore(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let hex = 0xf2_09_
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindTrailingUnderscore,
		syntaxError.InvalidIntegerLiteralKind,
	)

}

func TestParseInvalidIntegerLiteral(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let hex = 0z123
	`)

	assert.NotNil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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
		parser.InvalidIntegerLiteralKindUnknownPrefix,
		syntaxError.InvalidIntegerLiteralKind,
	)
}

func TestParseIntegerTypes(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let a: Int8 = 1
		let b: Int16 = 2
		let c: Int32 = 3
		let d: Int64 = 4
		let e: UInt8 = 5
		let f: UInt16 = 6
		let g: UInt32 = 7
		let h: UInt64 = 8
	`)

	assert.Nil(t, err)

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
			StartPos: Position{Offset: 10, Line: 2, Column: 9},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		Value: &IntExpression{
			Value: big.NewInt(1),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(2),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(3),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(4),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(5),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(6),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(7),
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
			Move: false,
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
		Value: &IntExpression{
			Value: big.NewInt(8),
			Range: Range{
				StartPos: Position{Offset: 153, Line: 9, Column: 18},
				EndPos:   Position{Offset: 153, Line: 9, Column: 18},
			},
		},
		StartPos: Position{Offset: 137, Line: 9, Column: 2},
	}

	expected := &Program{
		Declarations: []Declaration{a, b, c, d, e, f, g, h},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let add: ((Int8, Int16): Int32) = nothing
	`)

	assert.Nil(t, err)

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
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
					},
					{
						Move: false,
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
					Move: false,
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
					EndPos:   Position{Offset: 32, Line: 2, Column: 31},
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
	}

	expected := &Program{
		Declarations: []Declaration{add},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionArrayType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let test: [((Int8): Int16); 2] = []
	`)

	assert.Nil(t, err)

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
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
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
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
					},
					Range: Range{
						StartPos: Position{Offset: 14, Line: 2, Column: 13},
						EndPos:   Position{Offset: 27, Line: 2, Column: 26},
					},
				},
				Size: 2,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionTypeWithArrayReturnType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let test: ((Int8): [Int16; 2]) = nothing
	`)

	assert.Nil(t, err)

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
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
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
						Size: 2,
						Range: Range{
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
							EndPos:   Position{Offset: 31, Line: 2, Column: 30},
						},
					},
					StartPos: Position{Offset: 22, Line: 2, Column: 21},
				},
				Range: Range{
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
					EndPos:   Position{Offset: 31, Line: 2, Column: 30},
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionTypeWithFunctionReturnTypeInParentheses(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let test: ((Int8): ((Int16): Int32)) = nothing
	`)

	assert.Nil(t, err)

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
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
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
								StartPos: Position{Offset: 24, Line: 2, Column: 23},
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
							StartPos: Position{Offset: 32, Line: 2, Column: 31},
						},
						Range: Range{
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
							EndPos:   Position{Offset: 36, Line: 2, Column: 35},
						},
					},
					StartPos: Position{Offset: 22, Line: 2, Column: 21},
				},
				Range: Range{
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
					EndPos:   Position{Offset: 36, Line: 2, Column: 35},
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionTypeWithFunctionReturnType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let test: ((Int8): ((Int16): Int32)) = nothing
	`)

	assert.Nil(t, err)

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
						StartPos: Position{Offset: 15, Line: 2, Column: 14},
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
								StartPos: Position{Offset: 24, Line: 2, Column: 23},
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
							StartPos: Position{Offset: 32, Line: 2, Column: 31},
						},
						Range: Range{
							StartPos: Position{Offset: 22, Line: 2, Column: 21},
							EndPos:   Position{Offset: 36, Line: 2, Column: 35},
						},
					},
					StartPos: Position{Offset: 22, Line: 2, Column: 21},
				},
				Range: Range{
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
					EndPos:   Position{Offset: 36, Line: 2, Column: 35},
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMissingReturnType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
		let noop: ((): Void) =
            fun () { return }
	`)

	assert.Nil(t, err)

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
					StartPos: Position{Offset: 18, Line: 2, Column: 17},
				},
				Range: Range{
					StartPos: Position{Offset: 13, Line: 2, Column: 12},
					EndPos:   Position{Offset: 21, Line: 2, Column: 20},
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
				Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{noop},
	}

	assert.Equal(t, expected, actual)
}

func TestParseLeftAssociativity(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let a = 1 + 2 + 3
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
				Left: &IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				Right: &IntExpression{
					Value: big.NewInt(2),
					Range: Range{
						StartPos: Position{Offset: 21, Line: 2, Column: 20},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
			},
			Right: &IntExpression{
				Value: big.NewInt(3),
				Range: Range{
					StartPos: Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   Position{Offset: 25, Line: 2, Column: 24},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseInvalidDoubleIntegerUnary(t *testing.T) {

	program, _, err := parser.ParseProgram(`
	   var a = 1
	   let b = --a
	`)

	assert.NotNil(t, program)

	assert.IsType(t, parser.Error{}, err)

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

	program, _, err := parser.ParseProgram(`
	   let b = !!true
	`)

	assert.NotNil(t, program)

	assert.IsType(t, parser.Error{}, err)

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

	actual, _, err := parser.ParseProgram(`
        let a = 2 > 1
          ? 0
          : 3 > 2 ? 1 : 2
	`)

	assert.Nil(t, err)

	a := &VariableDeclaration{
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
				Left: &IntExpression{
					Value: big.NewInt(2),
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 17, Line: 2, Column: 16},
					},
				},
				Right: &IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 21, Line: 2, Column: 20},
						EndPos:   Position{Offset: 21, Line: 2, Column: 20},
					},
				},
			},
			Then: &IntExpression{
				Value: big.NewInt(0),
				Range: Range{
					StartPos: Position{Offset: 35, Line: 3, Column: 12},
					EndPos:   Position{Offset: 35, Line: 3, Column: 12},
				},
			},
			Else: &ConditionalExpression{
				Test: &BinaryExpression{
					Operation: OperationGreater,
					Left: &IntExpression{
						Value: big.NewInt(3),
						Range: Range{
							StartPos: Position{Offset: 49, Line: 4, Column: 12},
							EndPos:   Position{Offset: 49, Line: 4, Column: 12},
						},
					},
					Right: &IntExpression{
						Value: big.NewInt(2),
						Range: Range{
							StartPos: Position{Offset: 53, Line: 4, Column: 16},
							EndPos:   Position{Offset: 53, Line: 4, Column: 16},
						},
					},
				},
				Then: &IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 57, Line: 4, Column: 20},
						EndPos:   Position{Offset: 57, Line: 4, Column: 20},
					},
				},
				Else: &IntExpression{
					Value: big.NewInt(2),
					Range: Range{
						StartPos: Position{Offset: 61, Line: 4, Column: 24},
						EndPos:   Position{Offset: 61, Line: 4, Column: 24},
					},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{a},
	}

	assert.Equal(t, expected, actual)
}

func TestParseStructure(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
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

	assert.Nil(t, err)

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
					DeclarationKind: common.DeclarationKindInitializer,
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
										Move: false,
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
										EndPos:   Position{Offset: 80, Line: 5, Column: 22},
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
						Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseStructureWithConformances(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        struct Test: Foo, Bar {}
	`)

	assert.Nil(t, err)

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
		Members: &Members{},
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 32, Line: 2, Column: 31},
		},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseInvalidStructureWithMissingFunctionBlock(t *testing.T) {

	_, _, err := parser.ParseProgram(`
        struct Test {
            pub fun getFoo(): Int
        }
	`)

	assert.NotNil(t, err)
}

func TestParsePreAndPostConditions(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
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

	assert.Nil(t, err)

	expected := &Program{
		Declarations: []Declaration{
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
								Move: false,
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
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
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
					StartPos: Position{Offset: 24, Line: 2, Column: 23},
				},
				FunctionBlock: &FunctionBlock{
					Block: &Block{
						Statements: []Statement{
							&ReturnStatement{
								Expression: &IntExpression{
									Value: big.NewInt(0),
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
									Value: big.NewInt(0),
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
								Right: &IntExpression{
									Value: big.NewInt(0),
									Range: Range{
										StartPos: Position{Offset: 89, Line: 5, Column: 20},
										EndPos:   Position{Offset: 89, Line: 5, Column: 20},
									},
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
									Value: big.NewInt(0),
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
	}

	assert.Equal(t, expected, actual)
}

func TestParseExpression(t *testing.T) {

	actual, _, err := parser.ParseExpression(`
        before(x + before(y)) + z
	`)

	assert.Nil(t, err)

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

	assert.Equal(t, expected, actual)
}

func TestParseString(t *testing.T) {

	actual, _, err := parser.ParseExpression(`
       "test \0\n\r\t\"\'\\ xyz"
	`)

	assert.Nil(t, err)

	expected := &StringExpression{
		Value: "test \x00\n\r\t\"'\\ xyz",
		Range: Range{
			StartPos: Position{Offset: 8, Line: 2, Column: 7},
			EndPos:   Position{Offset: 32, Line: 2, Column: 31},
		},
	}

	assert.Equal(t, expected, actual)
}

func TestParseStringWithUnicode(t *testing.T) {

	actual, _, err := parser.ParseExpression(`
      "this is a test \t\\new line and race car:\n\u{1F3CE}\u{FE0F}"
	`)

	assert.Nil(t, err)

	expected := &StringExpression{
		Value: "this is a test \t\\new line and race car:\n\U0001F3CE\uFE0F",
		Range: Range{
			StartPos: Position{Offset: 7, Line: 2, Column: 6},
			EndPos:   Position{Offset: 68, Line: 2, Column: 67},
		},
	}

	assert.Equal(t, expected, actual)
}

func TestParseConditionMessage(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        fun test(n: Int) {
            pre {
                n >= 0: "n must be positive"
            }
            return n
        }
	`)

	assert.Nil(t, err)

	expected := &Program{
		Declarations: []Declaration{
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
								Move: false,
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
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
					},
					Range: Range{
						StartPos: Position{Offset: 17, Line: 2, Column: 16},
						EndPos:   Position{Offset: 24, Line: 2, Column: 23},
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
									Value: big.NewInt(0),
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
	}

	assert.Equal(t, expected, actual)
}

func TestParseOptionalType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
       let x: Int?? = 1
	`)

	assert.Nil(t, err)

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
					StartPos: Position{Offset: 15, Line: 2, Column: 14},
				},
				Transfer: &Transfer{
					Operation: TransferOperationCopy,
					Pos:       Position{Offset: 21, Line: 2, Column: 20},
				},
				Value: &IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	assert.Equal(t, expected, actual)
}

func TestParseNilCoalescing(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
       let x = nil ?? 1
	`)

	assert.Nil(t, err)

	expected := &Program{
		Declarations: []Declaration{
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
					Right: &IntExpression{
						Value: big.NewInt(1),
						Range: Range{
							StartPos: Position{Offset: 23, Line: 2, Column: 22},
							EndPos:   Position{Offset: 23, Line: 2, Column: 22},
						},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	assert.Equal(t, expected, actual)
}

func TestParseNilCoalescingRightAssociativity(t *testing.T) {

	// NOTE: only syntactically, not semantically valid
	actual, _, err := parser.ParseProgram(`
       let x = 1 ?? 2 ?? 3
	`)

	assert.Nil(t, err)

	expected := &Program{
		Declarations: []Declaration{
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
					Left: &IntExpression{
						Value: big.NewInt(1),
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					Right: &BinaryExpression{
						Operation: OperationNilCoalesce,
						Left: &IntExpression{
							Value: big.NewInt(2),
							Range: Range{
								StartPos: Position{Offset: 21, Line: 2, Column: 20},
								EndPos:   Position{Offset: 21, Line: 2, Column: 20},
							},
						},
						Right: &IntExpression{
							Value: big.NewInt(3),
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
	}

	assert.Equal(t, expected, actual)
}

func TestParseFailableDowncasting(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
       let x = 0 as? Int
	`)

	assert.Nil(t, err)

	expected := &Program{
		Declarations: []Declaration{
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
				Value: &FailableDowncastExpression{
					Expression: &IntExpression{
						Value: big.NewInt(0),
						Range: Range{
							StartPos: Position{Offset: 16, Line: 2, Column: 15},
							EndPos:   Position{Offset: 16, Line: 2, Column: 15},
						},
					},
					TypeAnnotation: &TypeAnnotation{
						Move: false,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "Int",
								Pos:        Position{Offset: 22, Line: 2, Column: 21},
							},
						},
						StartPos: Position{Offset: 22, Line: 2, Column: 21},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	assert.Equal(t, expected, actual)
}

func TestParseInterface(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		actual, _, err := parser.ParseProgram(fmt.Sprintf(`
            %s interface Test {
                foo: Int

                init(foo: Int)

                fun getFoo(): Int
            }
	    `, kind.Keyword()))

		assert.Nil(t, err)

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
						DeclarationKind: common.DeclarationKindInitializer,
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
											Move: false,
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
											EndPos:   Position{Offset: 89, Line: 5, Column: 26},
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
							Move: false,
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

		assert.Equal(t, expected, actual)
	}
}

func TestParseImportWithString(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        import "test.bpl"
	`)

	assert.Nil(t, err)

	test := &ImportDeclaration{
		Identifiers: []Identifier{},
		Location:    StringImportLocation("test.bpl"),
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 25, Line: 2, Column: 24},
		},
		LocationPos: Position{Offset: 16, Line: 2, Column: 15},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)

	assert.Equal(t,
		map[ImportLocation]*Program{
			StringImportLocation("test.bpl"): nil,
		},
		actual.Imports(),
	)

	actual.Imports()[StringImportLocation("test.bpl")] = &Program{}

	assert.Equal(t,
		map[ImportLocation]*Program{
			StringImportLocation("test.bpl"): {},
		},
		actual.Imports(),
	)
}

func TestParseImportWithAddress(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        import 0x1234
	`)

	assert.Nil(t, err)

	test := &ImportDeclaration{
		Identifiers: []Identifier{},
		Location:    AddressImportLocation([]byte{18, 52}),
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 21, Line: 2, Column: 20},
		},
		LocationPos: Position{Offset: 16, Line: 2, Column: 15},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)

	assert.Equal(t,
		map[ImportLocation]*Program{
			AddressImportLocation([]byte{18, 52}): nil,
		},
		actual.Imports(),
	)

	actual.Imports()[AddressImportLocation([]byte{18, 52})] = &Program{}

	assert.Equal(t,
		map[ImportLocation]*Program{
			AddressImportLocation([]byte{18, 52}): {},
		},
		actual.Imports(),
	)
}

func TestParseImportWithIdentifiers(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        import A, b from 0x0
	`)

	assert.Nil(t, err)

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
		Location: AddressImportLocation([]byte{0}),
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 28, Line: 2, Column: 27},
		},
		LocationPos: Position{Offset: 26, Line: 2, Column: 25},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFieldWithFromIdentifier(t *testing.T) {

	_, _, err := parser.ParseProgram(`
      struct S {
          let from: String
      }
	`)

	assert.Nil(t, err)
}

func TestParseFunctionWithFromIdentifier(t *testing.T) {

	_, _, err := parser.ParseProgram(`
        fun send(from: String, to: String) {}
	`)

	assert.Nil(t, err)
}

func TestParseImportWithFromIdentifier(t *testing.T) {

	_, _, err := parser.ParseProgram(`
        import from from 0x0
	`)

	assert.Nil(t, err)
}

func TestParseSemicolonsBetweenDeclarations(t *testing.T) {

	_, _, err := parser.ParseProgram(`
        import from from 0x0;
        fun foo() {};
	`)

	assert.Nil(t, err)
}

func TestParseInvalidTypeWithWhitespace(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
	    let x: Int ? = 1
	`)

	assert.Nil(t, actual)

	assert.IsType(t, parser.Error{}, err)

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

	actual, _, err := parser.ParseProgram(`
        resource Test {}
	`)

	assert.Nil(t, err)

	test := &CompositeDeclaration{
		CompositeKind: common.CompositeKindResource,
		Identifier: Identifier{
			Identifier: "Test",
			Pos:        Position{Offset: 18, Line: 2, Column: 17},
		},
		Conformances: []*NominalType{},
		Members:      &Members{},
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 24, Line: 2, Column: 23},
		},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseEvent(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        event Transfer(to: Address, from: Address)
	`)

	require.Nil(t, err)

	transfer := &EventDeclaration{
		Identifier: Identifier{
			Identifier: "Transfer",
			Pos:        Position{Offset: 15, Line: 2, Column: 14},
		},
		ParameterList: &ParameterList{
			Parameters: []*Parameter{
				{
					Label: "",
					Identifier: Identifier{
						Identifier: "to",
						Pos:        Position{Offset: 24, Line: 2, Column: 23},
					},
					TypeAnnotation: &TypeAnnotation{
						Move: false,
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
						EndPos:   Position{Offset: 28, Line: 2, Column: 27},
					},
				},
				{
					Label: "",
					Identifier: Identifier{
						Identifier: "from",
						Pos:        Position{Offset: 37, Line: 2, Column: 36},
					},
					TypeAnnotation: &TypeAnnotation{
						Move: false,
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
						EndPos:   Position{Offset: 43, Line: 2, Column: 42},
					},
				},
			},
			Range: Range{
				StartPos: Position{Offset: 23, Line: 2, Column: 22},
				EndPos:   Position{Offset: 50, Line: 2, Column: 49},
			},
		},
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 50, Line: 2, Column: 49},
		},
	}

	expected := &Program{
		Declarations: []Declaration{transfer},
	}

	assert.Equal(t, expected, actual)
}

func TestParseEventEmitStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
      fun test() {
        emit Transfer(to: 1, from: 2)
      }
	`)

	actualStatements := actual.Declarations[0].(*FunctionDeclaration).FunctionBlock.Block.Statements

	expectedStatements := []Statement{
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
						Expression: &IntExpression{
							Value: big.NewInt(1),
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
						Expression: &IntExpression{
							Value: big.NewInt(2),
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
	}

	require.Nil(t, err)

	assert.Equal(t, expectedStatements, actualStatements)
}

func TestParseMoveReturnType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        fun test(): <-X {}
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: true,
			Type: &NominalType{
				Identifier: Identifier{
					Identifier: "X",
					Pos:        Position{Offset: 23, Line: 2, Column: 22},
				},
			},
			StartPos: Position{Offset: 21, Line: 2, Column: 20},
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMovingVariableDeclaration(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let x <- y
	`)

	assert.Nil(t, err)

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
		Transfer: &Transfer{
			Operation: TransferOperationMove,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMoveStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        fun test() {
            x <- y
        }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMoveOperator(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
      let x = foo(<-y)
	`)

	assert.Nil(t, err)

	test := &VariableDeclaration{
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
						Range: Range{
							StartPos: Position{Offset: 19, Line: 2, Column: 18},
							EndPos:   Position{Offset: 21, Line: 2, Column: 20},
						},
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

	assert.Equal(t, expected, actual)
}

func TestParseMoveParameterType(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        fun test(x: <-X) {}
	`)

	assert.Nil(t, err)

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
			StartPos: Position{Offset: 24, Line: 2, Column: 23},
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
						Move: true,
						Type: &NominalType{
							Identifier: Identifier{
								Identifier: "X",
								Pos:        Position{Offset: 23, Line: 2, Column: 22},
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
		FunctionBlock: &FunctionBlock{
			Block: &Block{
				Range: Range{
					StartPos: Position{Offset: 26, Line: 2, Column: 25},
					EndPos:   Position{Offset: 27, Line: 2, Column: 26},
				},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseMovingVariableDeclarationWithTypeAnnotation(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let x: <-R <- y
	`)

	assert.Nil(t, err)

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
			StartPos: Position{Offset: 16, Line: 2, Column: 15},
		},
		Value: &IdentifierExpression{
			Identifier: Identifier{
				Identifier: "y",
				Pos:        Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		Transfer: &Transfer{
			Operation: TransferOperationMove,
			Pos:       Position{Offset: 20, Line: 2, Column: 19},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFieldDeclarationWithMoveTypeAnnotation(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        struct X { x: <-R }
	`)

	assert.Nil(t, err)

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
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
					},
					Range: Range{
						StartPos: Position{Offset: 20, Line: 2, Column: 19},
						EndPos:   Position{Offset: 25, Line: 2, Column: 24},
					},
				},
			},
		},
		Range: Range{
			StartPos: Position{Offset: 9, Line: 2, Column: 8},
			EndPos:   Position{Offset: 27, Line: 2, Column: 26},
		},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionTypeWithMoveTypeAnnotation(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let f: ((): <-R) = g
	`)

	assert.Nil(t, err)

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
			Pos:       Position{Offset: 26, Line: 2, Column: 25},
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

	assert.Equal(t, expected, actual)
}

func TestParseFunctionExpressionWithMoveTypeAnnotation(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let f = fun (): <-R { return X }
	`)

	assert.Nil(t, err)

	test := &VariableDeclaration{
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
				Move: true,
				Type: &NominalType{
					Identifier: Identifier{
						Identifier: "R",
						Pos:        Position{Offset: 27, Line: 2, Column: 26},
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
									Pos:        Position{Offset: 38, Line: 2, Column: 37},
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
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFailableDowncastingMoveTypeAnnotation(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        let y = x as? <-R
	`)

	assert.Nil(t, err)

	test := &VariableDeclaration{
		IsConstant: true,
		Identifier: Identifier{
			Identifier: "y",
			Pos:        Position{Offset: 13, Line: 2, Column: 12},
		},
		Transfer: &Transfer{
			Operation: TransferOperationCopy,
			Pos:       Position{Offset: 15, Line: 2, Column: 14},
		},
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
				StartPos: Position{Offset: 23, Line: 2, Column: 22},
			},
		},
		StartPos: Position{Offset: 9, Line: 2, Column: 8},
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseFunctionExpressionStatementAfterVariableDeclarationWithCreateExpression(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
      fun test() {
          let r <- create R()
          (fun () {})()
      }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
									Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseIdentifiers(t *testing.T) {

	for _, name := range []string{"foo", "from", "create", "destroy"} {
		_, _, err := parser.ParseProgram(fmt.Sprintf(`
          let %s = 1
	    `, name))

		assert.Nil(t, err)
	}
}

// TestParseExpressionStatementAfterReturnStatement tests that a return statement
// does *not* consume an expression from the next statement as the return value
//
func TestParseExpressionStatementAfterReturnStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
      fun test() {
          return
          destroy x
      }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseSwapStatement(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
      fun test() {
          foo[0] <-> bar.baz
      }
	`)

	assert.Nil(t, err)

	test := &FunctionDeclaration{
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
			Move: false,
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
							IndexingExpression: &IntExpression{
								Value: big.NewInt(0),
								Range: Range{
									StartPos: Position{Offset: 34, Line: 3, Column: 14},
									EndPos:   Position{Offset: 34, Line: 3, Column: 14},
								},
							},
							IndexingType: nil,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseDestructor(t *testing.T) {

	actual, _, err := parser.ParseProgram(`
        resource Test {
            destroy() {}
        }
	`)

	assert.Nil(t, err)

	test := &CompositeDeclaration{
		CompositeKind: common.CompositeKindResource,
		Identifier: Identifier{
			Identifier: "Test",
			Pos:        Position{Offset: 18, Line: 2, Column: 17},
		},
		Conformances: []*NominalType{},
		Members: &Members{
			SpecialFunctions: []*SpecialFunctionDeclaration{
				{
					DeclarationKind: common.DeclarationKindDestructor,
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
	}

	expected := &Program{
		Declarations: []Declaration{test},
	}

	assert.Equal(t, expected, actual)
}

func TestParseReferenceType(t *testing.T) {

	// NOTE: syntactically legal, but semantically invalid

	actual, _, err := parser.ParseProgram(`
       let x: &[&R] = 1
	`)

	assert.Nil(t, err)

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
				Value: &IntExpression{
					Value: big.NewInt(1),
					Range: Range{
						StartPos: Position{Offset: 23, Line: 2, Column: 22},
						EndPos:   Position{Offset: 23, Line: 2, Column: 22},
					},
				},
				StartPos: Position{Offset: 8, Line: 2, Column: 7},
			},
		},
	}

	assert.Equal(t, expected, actual)
}

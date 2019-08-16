package interpreter

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
)

func TestBeforeExtractor(t *testing.T) {
	RegisterTestingT(t)

	expression, errors := parser.ParseExpression(`
        before(x + before(y)) + z
	`)

	Expect(errors).
		To(BeEmpty())

	extractor := NewBeforeExtractor()

	identifier1 := extractor.ExpressionExtractor.FormatIdentifier(0)
	identifier2 := extractor.ExpressionExtractor.FormatIdentifier(1)

	result := extractor.ExtractBefore(expression)

	Expect(result).
		To(Equal(ast.ExpressionExtraction{
			RewrittenExpression: &ast.BinaryExpression{
				Operation: ast.OperationPlus,
				Left: &ast.IdentifierExpression{
					Identifier: identifier2,
					StartPos:   ast.Position{Offset: 0, Line: 0, Column: 0},
					EndPos:     ast.Position{Offset: 0, Line: 0, Column: 0},
				},
				Right: &ast.IdentifierExpression{
					Identifier: "z",
					StartPos:   ast.Position{Offset: 33, Line: 2, Column: 32},
					EndPos:     ast.Position{Offset: 33, Line: 2, Column: 32},
				},
			},
			ExtractedExpressions: []ast.ExtractedExpression{
				{
					Identifier: identifier1,
					Expression: &ast.IdentifierExpression{
						Identifier: "y",
						StartPos:   ast.Position{Offset: 27, Line: 2, Column: 26},
						EndPos:     ast.Position{Offset: 27, Line: 2, Column: 26},
					},
				},
				{
					Identifier: identifier2,
					Expression: &ast.BinaryExpression{
						Operation: ast.OperationPlus,
						Left: &ast.IdentifierExpression{
							Identifier: "x",
							StartPos:   ast.Position{Offset: 16, Line: 2, Column: 15},
							EndPos:     ast.Position{Offset: 16, Line: 2, Column: 15},
						},
						Right: &ast.IdentifierExpression{
							Identifier: identifier1,
							StartPos:   ast.Position{Offset: 0, Line: 0, Column: 0},
							EndPos:     ast.Position{Offset: 0, Line: 0, Column: 0},
						},
					},
				},
			},
		}))
}

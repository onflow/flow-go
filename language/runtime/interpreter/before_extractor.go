package interpreter

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

type BeforeExtractor struct {
	ExpressionExtractor *ast.ExpressionExtractor
}

func NewBeforeExtractor() *BeforeExtractor {
	beforeExtractor := &BeforeExtractor{}
	expressionExtractor := &ast.ExpressionExtractor{
		InvocationExtractor: beforeExtractor,
	}
	beforeExtractor.ExpressionExtractor = expressionExtractor
	return beforeExtractor
}

func (beforeExtractor *BeforeExtractor) ExtractBefore(expression ast.Expression) ast.ExpressionExtraction {
	return beforeExtractor.ExpressionExtractor.Extract(expression)
}

func (BeforeExtractor) ExtractInvocation(
	extractor *ast.ExpressionExtractor,
	expression *ast.InvocationExpression,
) ast.ExpressionExtraction {

	invokedExpression := expression.InvokedExpression

	if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {
		if identifierExpression.Identifier.Identifier == sema.BeforeIdentifier {

			// semantic analysis should have rejected calls
			// which do not have exactly one argument

			if len(expression.Arguments) != 1 {
				panic(&errors.UnreachableError{})
			}

			// rewrite the argument

			argumentExpression := expression.Arguments[0].Expression
			argumentResult := extractor.Extract(argumentExpression)

			extractedExpressions := argumentResult.ExtractedExpressions

			// create a fresh identifier which has the rewritten argument
			// as its initial value

			newIdentifier := ast.Identifier{
				Identifier: extractor.FreshIdentifier(),
			}
			newExpression := &ast.IdentifierExpression{
				Identifier: newIdentifier,
			}

			extractedExpressions = append(extractedExpressions,
				ast.ExtractedExpression{
					Identifier: newIdentifier,
					Expression: argumentResult.RewrittenExpression,
				},
			)

			return ast.ExpressionExtraction{
				RewrittenExpression:  newExpression,
				ExtractedExpressions: extractedExpressions,
			}
		}
	}

	// not an invocation of `before`, perform default extraction

	return extractor.ExtractInvocation(expression)
}

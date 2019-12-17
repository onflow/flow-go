package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type BeforeExtractor struct {
	ExpressionExtractor *ast.ExpressionExtractor
	report              func(error)
}

func NewBeforeExtractor(report func(error)) *BeforeExtractor {
	beforeExtractor := &BeforeExtractor{
		report: report,
	}
	expressionExtractor := &ast.ExpressionExtractor{
		InvocationExtractor: beforeExtractor,
		FunctionExtractor:   beforeExtractor,
	}
	beforeExtractor.ExpressionExtractor = expressionExtractor
	return beforeExtractor
}

func (e *BeforeExtractor) ExtractBefore(expression ast.Expression) ast.ExpressionExtraction {
	return e.ExpressionExtractor.Extract(expression)
}

func (e *BeforeExtractor) ExtractInvocation(
	extractor *ast.ExpressionExtractor,
	expression *ast.InvocationExpression,
) ast.ExpressionExtraction {

	invokedExpression := expression.InvokedExpression

	if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {
		const expectedArgumentCount = 1

		if identifierExpression.Identifier.Identifier == BeforeIdentifier &&
			len(expression.Arguments) == expectedArgumentCount {

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

func (e *BeforeExtractor) ExtractFunction(
	_ *ast.ExpressionExtractor,
	expression *ast.FunctionExpression,
) ast.ExpressionExtraction {

	// NOTE: function expressions are not supported by the expression extractor, so return as-is
	// An error is reported when checking invocation expressions, so no need to report here

	return ast.ExpressionExtraction{
		RewrittenExpression: expression,
	}
}

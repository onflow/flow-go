package ast

import (
	"math/big"
	"testing"

	. "github.com/onsi/gomega"
)

type testIntExtractor struct{}

func (testIntExtractor) ExtractInt(
	extractor *ExpressionExtractor,
	expression *IntExpression,
) ExpressionExtraction {

	newIdentifier := extractor.FreshIdentifier()
	newExpression := &IdentifierExpression{Identifier: newIdentifier}
	return ExpressionExtraction{
		RewrittenExpression: newExpression,
		ExtractedExpressions: []ExtractedExpression{
			{
				Identifier: newIdentifier,
				Expression: expression,
			},
		},
	}
}

func TestExpressionExtractorBinaryExpressionNothingExtracted(t *testing.T) {
	RegisterTestingT(t)

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left:      &IdentifierExpression{Identifier: "x"},
		Right:     &IdentifierExpression{Identifier: "y"},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	Expect(result).
		To(Equal(ExpressionExtraction{
			RewrittenExpression: &BinaryExpression{
				Operation: OperationEqual,
				Left:      &IdentifierExpression{Identifier: "x"},
				Right:     &IdentifierExpression{Identifier: "y"},
			},
			ExtractedExpressions: nil,
		}))
}

func TestExpressionExtractorBinaryExpressionIntegerExtracted(t *testing.T) {
	RegisterTestingT(t)

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left:      &IdentifierExpression{Identifier: "x"},
		Right:     &IntExpression{Value: big.NewInt(1)},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	newIdentifier := extractor.FormatIdentifier(0)

	Expect(result).
		To(Equal(ExpressionExtraction{
			RewrittenExpression: &BinaryExpression{
				Operation: OperationEqual,
				Left:      &IdentifierExpression{Identifier: "x"},
				Right:     &IdentifierExpression{Identifier: newIdentifier},
			},
			ExtractedExpressions: []ExtractedExpression{
				{
					Identifier: newIdentifier,
					Expression: &IntExpression{Value: big.NewInt(1)},
				},
			},
		}))
}

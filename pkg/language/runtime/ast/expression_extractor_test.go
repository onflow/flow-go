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

	newIdentifier := Identifier{
		Identifier: extractor.FreshIdentifier(),
	}
	newExpression := &IdentifierExpression{
		Identifier: newIdentifier,
	}
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
		Left: &IdentifierExpression{
			Identifier: Identifier{Identifier: "x"},
		},
		Right: &IdentifierExpression{
			Identifier: Identifier{Identifier: "y"},
		},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	Expect(result).
		To(Equal(ExpressionExtraction{
			RewrittenExpression: &BinaryExpression{
				Operation: OperationEqual,
				Left: &IdentifierExpression{
					Identifier{Identifier: "x"},
				},
				Right: &IdentifierExpression{
					Identifier{Identifier: "y"},
				},
			},
			ExtractedExpressions: nil,
		}))
}

func TestExpressionExtractorBinaryExpressionIntegerExtracted(t *testing.T) {
	RegisterTestingT(t)

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left: &IdentifierExpression{
			Identifier{Identifier: "x"},
		},
		Right: &IntExpression{Value: big.NewInt(1)},
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
				Left: &IdentifierExpression{
					Identifier{Identifier: "x"},
				},
				Right: &IdentifierExpression{
					Identifier{Identifier: newIdentifier},
				},
			},
			ExtractedExpressions: []ExtractedExpression{
				{
					Identifier: Identifier{Identifier: newIdentifier},
					Expression: &IntExpression{Value: big.NewInt(1)},
				},
			},
		}))
}

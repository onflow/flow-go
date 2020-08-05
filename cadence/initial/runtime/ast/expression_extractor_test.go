/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ast

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testIntExtractor struct{}

func (testIntExtractor) ExtractInteger(
	extractor *ExpressionExtractor,
	expression *IntegerExpression,
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

	t.Parallel()

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

	assert.Equal(t,
		result,
		ExpressionExtraction{
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
		},
	)
}

func TestExpressionExtractorBinaryExpressionIntegerExtracted(t *testing.T) {

	t.Parallel()

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left: &IdentifierExpression{
			Identifier{Identifier: "x"},
		},
		Right: &IntegerExpression{
			Value: big.NewInt(1),
			Base:  10,
		},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	newIdentifier := extractor.FormatIdentifier(0)

	assert.Equal(t,
		result,
		ExpressionExtraction{
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
					Expression: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
					},
				},
			},
		},
	)
}

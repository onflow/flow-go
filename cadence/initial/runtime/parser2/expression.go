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

package parser2

import (
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/errors"
	"example.com/cadence-initial/runtime/parser2/lexer"
)

const (
	exprLeftBindingPowerTernary = 10 * (iota + 2)
	exprLeftBindingPowerLogicalOr
	exprLeftBindingPowerLogicalAnd
	exprLeftBindingPowerComparison
	exprLeftBindingPowerNilCoalescing
	exprLeftBindingPowerBitwiseOr
	exprLeftBindingPowerBitwiseXor
	exprLeftBindingPowerBitwiseAnd
	exprLeftBindingPowerBitwiseShift
	exprLeftBindingPowerAddition
	exprLeftBindingPowerMultiplication
	exprLeftBindingPowerCasting
	exprLeftBindingPowerUnaryPrefix
	exprLeftBindingPowerUnaryPostfix
	exprLeftBindingPowerAccess
)

type infixExprFunc func(left, right ast.Expression) ast.Expression
type prefixExprFunc func(right ast.Expression, tokenRange ast.Range) ast.Expression
type postfixExprFunc func(left ast.Expression, tokenRange ast.Range) ast.Expression
type exprNullDenotationFunc func(parser *parser, token lexer.Token) ast.Expression
type exprMetaLeftDenotationFunc func(
	p *parser,
	rightBindingPower int,
	left ast.Expression,
) (
	result ast.Expression,
	done bool,
)

type literalExpr struct {
	tokenType      lexer.TokenType
	nullDenotation exprNullDenotationFunc
}

type infixExpr struct {
	tokenType        lexer.TokenType
	leftBindingPower int
	rightAssociative bool
	leftDenotation   infixExprFunc
}

type binaryExpr struct {
	tokenType        lexer.TokenType
	leftBindingPower int
	rightAssociative bool
	operation        ast.Operation
}

type prefixExpr struct {
	tokenType      lexer.TokenType
	bindingPower   int
	nullDenotation prefixExprFunc
}

type unaryExpr struct {
	tokenType    lexer.TokenType
	bindingPower int
	operation    ast.Operation
}

type postfixExpr struct {
	tokenType      lexer.TokenType
	bindingPower   int
	leftDenotation postfixExprFunc
}

var exprNullDenotations = map[lexer.TokenType]exprNullDenotationFunc{}

type exprLeftDenotationFunc func(parser *parser, token lexer.Token, left ast.Expression) ast.Expression

var exprLeftBindingPowers = map[lexer.TokenType]int{}
var exprIdentifierLeftBindingPowers = map[string]int{}
var exprLeftDenotations = map[lexer.TokenType]exprLeftDenotationFunc{}
var exprMetaLeftDenotations = map[lexer.TokenType]exprMetaLeftDenotationFunc{}

func defineExpr(def interface{}) {
	switch def := def.(type) {
	case infixExpr:
		tokenType := def.tokenType

		setExprLeftBindingPower(tokenType, def.leftBindingPower)

		rightBindingPower := def.leftBindingPower
		if def.rightAssociative {
			rightBindingPower--
		}

		setExprLeftDenotation(
			tokenType,
			func(parser *parser, _ lexer.Token, left ast.Expression) ast.Expression {
				right := parseExpression(parser, rightBindingPower)
				return def.leftDenotation(left, right)
			},
		)

	case binaryExpr:
		defineExpr(infixExpr{
			tokenType:        def.tokenType,
			leftBindingPower: def.leftBindingPower,
			rightAssociative: def.rightAssociative,
			leftDenotation: func(left, right ast.Expression) ast.Expression {
				return &ast.BinaryExpression{
					Operation: def.operation,
					Left:      left,
					Right:     right,
				}
			},
		})

	case literalExpr:
		tokenType := def.tokenType
		setExprNullDenotation(tokenType, def.nullDenotation)

	case prefixExpr:
		tokenType := def.tokenType
		setExprNullDenotation(
			tokenType,
			func(parser *parser, token lexer.Token) ast.Expression {
				right := parseExpression(parser, def.bindingPower)
				return def.nullDenotation(right, token.Range)
			},
		)

	case unaryExpr:
		defineExpr(prefixExpr{
			tokenType:    def.tokenType,
			bindingPower: def.bindingPower,
			nullDenotation: func(right ast.Expression, tokenRange ast.Range) ast.Expression {
				return &ast.UnaryExpression{
					Operation:  def.operation,
					Expression: right,
					StartPos:   tokenRange.StartPos,
				}
			},
		})

	case postfixExpr:
		tokenType := def.tokenType
		setExprLeftBindingPower(tokenType, def.bindingPower)
		setExprLeftDenotation(
			tokenType,
			func(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
				return def.leftDenotation(left, token.Range)
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func setExprNullDenotation(tokenType lexer.TokenType, nullDenotation exprNullDenotationFunc) {
	current := exprNullDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"expression null denotation for token %s already exists",
			tokenType,
		))
	}
	exprNullDenotations[tokenType] = nullDenotation
}

func setExprLeftBindingPower(tokenType lexer.TokenType, power int) {
	current := exprLeftBindingPowers[tokenType]
	if current > power {
		return
	}
	exprLeftBindingPowers[tokenType] = power
}

func setExprIdentifierLeftBindingPower(keyword string, power int) {
	current := exprIdentifierLeftBindingPowers[keyword]
	if current > power {
		return
	}
	exprIdentifierLeftBindingPowers[keyword] = power
}

func setExprLeftDenotation(tokenType lexer.TokenType, leftDenotation exprLeftDenotationFunc) {
	current := exprLeftDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"expression left denotation for token %s already exists",
			tokenType,
		))
	}
	exprLeftDenotations[tokenType] = leftDenotation
}

func setExprMetaLeftDenotation(tokenType lexer.TokenType, metaLeftDenotation exprMetaLeftDenotationFunc) {
	current := exprMetaLeftDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"expression meta left denotation for token %s already exists",
			tokenType,
		))
	}
	exprMetaLeftDenotations[tokenType] = metaLeftDenotation
}

func init() {

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenVerticalBarVerticalBar,
		leftBindingPower: exprLeftBindingPowerLogicalOr,
		rightAssociative: true,
		operation:        ast.OperationOr,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenAmpersandAmpersand,
		leftBindingPower: exprLeftBindingPowerLogicalAnd,
		rightAssociative: true,
		operation:        ast.OperationAnd,
	})

	defineLessThanOrTypeArgumentsExpression()

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenLessEqual,
		leftBindingPower: exprLeftBindingPowerComparison,
		operation:        ast.OperationLessEqual,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenGreater,
		leftBindingPower: exprLeftBindingPowerComparison,
		operation:        ast.OperationGreater,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenGreaterEqual,
		leftBindingPower: exprLeftBindingPowerComparison,
		operation:        ast.OperationGreaterEqual,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenEqualEqual,
		leftBindingPower: exprLeftBindingPowerComparison,
		operation:        ast.OperationEqual,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenNotEqual,
		leftBindingPower: exprLeftBindingPowerComparison,
		operation:        ast.OperationNotEqual,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenDoubleQuestionMark,
		leftBindingPower: exprLeftBindingPowerNilCoalescing,
		operation:        ast.OperationNilCoalesce,
		rightAssociative: true,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenVerticalBar,
		leftBindingPower: exprLeftBindingPowerBitwiseOr,
		operation:        ast.OperationBitwiseOr,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenCaret,
		leftBindingPower: exprLeftBindingPowerBitwiseXor,
		operation:        ast.OperationBitwiseXor,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenAmpersand,
		leftBindingPower: exprLeftBindingPowerBitwiseAnd,
		operation:        ast.OperationBitwiseAnd,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenLessLess,
		leftBindingPower: exprLeftBindingPowerBitwiseShift,
		operation:        ast.OperationBitwiseLeftShift,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenGreaterGreater,
		leftBindingPower: exprLeftBindingPowerBitwiseShift,
		operation:        ast.OperationBitwiseRightShift,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenPlus,
		leftBindingPower: exprLeftBindingPowerAddition,
		operation:        ast.OperationPlus,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenMinus,
		leftBindingPower: exprLeftBindingPowerAddition,
		operation:        ast.OperationMinus,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenStar,
		leftBindingPower: exprLeftBindingPowerMultiplication,
		operation:        ast.OperationMul,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenSlash,
		leftBindingPower: exprLeftBindingPowerMultiplication,
		operation:        ast.OperationDiv,
	})

	defineExpr(binaryExpr{
		tokenType:        lexer.TokenPercent,
		leftBindingPower: exprLeftBindingPowerMultiplication,
		operation:        ast.OperationMod,
	})

	defineCastingExpression()

	defineExpr(literalExpr{
		tokenType: lexer.TokenBinaryLiteral,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			literal := token.Value.(string)
			return parseIntegerLiteral(
				p,
				literal,
				literal[2:],
				IntegerLiteralKindBinary,
				token.Range,
			)
		},
	})

	defineExpr(literalExpr{
		tokenType: lexer.TokenOctalLiteral,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			literal := token.Value.(string)
			return parseIntegerLiteral(
				p,
				literal,
				literal[2:],
				IntegerLiteralKindOctal,
				token.Range,
			)
		},
	})

	defineExpr(literalExpr{
		tokenType: lexer.TokenDecimalLiteral,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			literal := token.Value.(string)
			return parseIntegerLiteral(
				p,
				literal,
				literal,
				IntegerLiteralKindDecimal,
				token.Range,
			)
		},
	})

	defineExpr(literalExpr{
		tokenType: lexer.TokenHexadecimalLiteral,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			literal := token.Value.(string)
			return parseIntegerLiteral(
				p,
				literal,
				literal[2:],
				IntegerLiteralKindHexadecimal,
				token.Range,
			)
		},
	})

	defineExpr(literalExpr{
		tokenType: lexer.TokenFixedPointLiteral,
		nullDenotation: func(_ *parser, token lexer.Token) ast.Expression {
			return parseFixedPointLiteral(
				token.Value.(string),
				token.Range,
			)
		},
	})

	defineExpr(literalExpr{
		tokenType: lexer.TokenString,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			parsedString, errs := parseStringLiteral(token.Value.(string))
			p.report(errs...)
			return &ast.StringExpression{
				Value: parsedString,
				Range: token.Range,
			}
		},
	})

	defineExpr(prefixExpr{
		tokenType:    lexer.TokenMinus,
		bindingPower: exprLeftBindingPowerUnaryPrefix,
		nullDenotation: func(right ast.Expression, tokenRange ast.Range) ast.Expression {
			switch right := right.(type) {
			case *ast.IntegerExpression:
				if right.Value.Sign() > 0 {
					if right.Value != nil {
						right.Value.Neg(right.Value)
					}
					right.StartPos = tokenRange.StartPos
					return right
				}

			case *ast.FixedPointExpression:
				if !right.Negative {
					right.Negative = !right.Negative
					right.StartPos = tokenRange.StartPos
					return right
				}
			}

			return &ast.UnaryExpression{
				Operation:  ast.OperationMinus,
				Expression: right,
				StartPos:   tokenRange.StartPos,
			}
		},
	})

	defineExpr(unaryExpr{
		tokenType:    lexer.TokenExclamationMark,
		bindingPower: exprLeftBindingPowerUnaryPrefix,
		operation:    ast.OperationNegate,
	})

	defineExpr(unaryExpr{
		tokenType:    lexer.TokenLeftArrow,
		bindingPower: exprLeftBindingPowerUnaryPrefix,
		operation:    ast.OperationMove,
	})

	defineExpr(postfixExpr{
		tokenType:    lexer.TokenExclamationMark,
		bindingPower: exprLeftBindingPowerUnaryPostfix,
		leftDenotation: func(left ast.Expression, tokenRange ast.Range) ast.Expression {
			return &ast.ForceExpression{
				Expression: left,
				EndPos:     tokenRange.EndPos,
			}
		},
	})

	defineNestedExpression()
	defineInvocationExpression()
	defineArrayExpression()
	defineDictionaryExpression()
	defineIndexExpression()
	definePathExpression()
	defineConditionalExpression()
	defineReferenceExpression()
	defineMemberExpression()
	defineIdentifierExpression()

	setExprNullDenotation(lexer.TokenEOF, func(parser *parser, token lexer.Token) ast.Expression {
		panic("expected expression")
	})
}

func defineLessThanOrTypeArgumentsExpression() {

	// The less token `<` does not have a single left binding power,
	// but one depending on the tokens following it:
	//
	// Either an invocation with type arguments (zero or more, comma separated),
	// followed by a closing greater token `>` and argument list;
	// or a normal expression.
	//
	//     lessThenOrTypeArguments : '<'
	//         ( ( ( typeAnnotation ( ',' )* )? '>' argumentList )
	//         | expression
	//         )
	//
	//
	// Parse this ambiguity by first trying to parse type arguments
	// and a closing greater token `>` and start of an argument list,
	// i.e. the open paren token `(`.
	//
	// If that parse fails, the result expression must be a binary expression,
	// and a normal expression must follow.
	//
	// In both cases, the right binding power must be checked,
	// just like it is before a normal left denotation is applied.

	const binaryExpressionLeftBindingPower = exprLeftBindingPowerComparison
	const invocationExpressionLeftBindingPower = exprLeftBindingPowerAccess

	setExprMetaLeftDenotation(
		lexer.TokenLess,
		func(p *parser, rightBindingPower int, left ast.Expression) (result ast.Expression, done bool) {

			var isInvocation bool
			var typeArguments []*ast.TypeAnnotation

			// Start buffering before skipping the `<` token,
			// so it can be replayed in case the right binding power
			// was higher than the determined left binding power.

			p.startBuffering()

			// Skip the `<` token.
			p.next()
			p.skipSpaceAndComments(true)

			// First, try to to parse zero or more comma-separated
			// type arguments (type annotations), a closing greater token `>`,
			// and start of an argument list, i.e. the open paren token `(`.
			//
			// This parse may fail, in which case we just ignore the error.

			(func() {
				defer func() {
					_ = recover()
				}()

				typeArguments, _ =
					parseCommaSeparatedTypeAnnotations(p, lexer.TokenGreater)

				p.skipSpaceAndComments(true)
				p.mustOne(lexer.TokenGreater)
				p.skipSpaceAndComments(true)
				p.mustOne(lexer.TokenParenOpen)

				isInvocation = true
			})()

			if isInvocation {

				// The expression was determined to be an invocation.
				// Still, it should have maybe not been parsed if the right binding power
				// was higher. In that case, replay the buffered tokens and stop.

				if rightBindingPower >= invocationExpressionLeftBindingPower {
					p.replayBuffered()
					return left, true
				}

				// The previous attempt to parse an invocation succeeded,
				// accept the buffered tokens.

				p.acceptBuffered()

				arguments, endPos := parseArgumentListRemainder(p)

				invocationExpression := &ast.InvocationExpression{
					InvokedExpression: left,
					TypeArguments:     typeArguments,
					Arguments:         arguments,
					EndPos:            endPos,
				}

				return invocationExpression, false

			} else {

				// The previous attempt to parse an invocation failed,
				// replay the buffered tokens.

				p.replayBuffered()

				// The expression was determined to *not* be an invocation,
				// so it must be a binary expression.
				//
				// Like for a normal left denotation,
				// check if this left denotation applies.

				if rightBindingPower >= binaryExpressionLeftBindingPower {
					return left, true
				}

				// Skip the `<` token.
				// The token buffering started before this token,
				// because it should have maybe not been parsed in the first place
				// if the right binding power is higher.

				p.next()
				p.skipSpaceAndComments(true)

				right := parseExpression(p, binaryExpressionLeftBindingPower)

				binaryExpression := &ast.BinaryExpression{
					Operation: ast.OperationLess,
					Left:      left,
					Right:     right,
				}

				return binaryExpression, false
			}
		})
}

func defineIdentifierExpression() {
	defineExpr(literalExpr{
		tokenType: lexer.TokenIdentifier,
		nullDenotation: func(p *parser, token lexer.Token) ast.Expression {
			switch token.Value {
			case keywordTrue:
				return &ast.BoolExpression{
					Value: true,
					Range: token.Range,
				}

			case keywordFalse:
				return &ast.BoolExpression{
					Value: false,
					Range: token.Range,
				}

			case keywordNil:
				return &ast.NilExpression{
					Pos: token.Range.StartPos,
				}

			case keywordCreate:
				return parseCreateExpressionRemainder(p, token)

			case keywordDestroy:
				expression := parseExpression(p, lowestBindingPower)
				return &ast.DestroyExpression{
					Expression: expression,
					StartPos:   token.Range.StartPos,
				}

			case keywordFun:
				return parseFunctionExpression(p, token)

			default:
				return &ast.IdentifierExpression{
					Identifier: tokenToIdentifier(token),
				}
			}
		},
	})
}

func parseFunctionExpression(p *parser, token lexer.Token) *ast.FunctionExpression {

	parameterList, returnTypeAnnotation, functionBlock :=
		parseFunctionParameterListAndRest(p, false)

	return &ast.FunctionExpression{
		ParameterList:        parameterList,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             token.StartPos,
	}
}

func defineCastingExpression() {

	setExprIdentifierLeftBindingPower(keywordAs, exprLeftBindingPowerCasting)
	setExprLeftDenotation(
		lexer.TokenIdentifier,
		func(parser *parser, t lexer.Token, left ast.Expression) ast.Expression {
			switch t.Value.(string) {
			case keywordAs:
				right := parseTypeAnnotation(parser)
				return &ast.CastingExpression{
					Operation:      ast.OperationCast,
					Expression:     left,
					TypeAnnotation: right,
				}
			default:
				panic(errors.NewUnreachableError())
			}
		},
	)

	for tokenType, operation := range map[lexer.TokenType]ast.Operation{
		lexer.TokenAsExclamationMark: ast.OperationForceCast,
		lexer.TokenAsQuestionMark:    ast.OperationFailableCast,
	} {
		// Rebind operation, so the closure captures to current iteration's value,
		// i.e. the next iteration doesn't override `operation`

		leftDenotation := (func(operation ast.Operation) exprLeftDenotationFunc {
			return func(parser *parser, t lexer.Token, left ast.Expression) ast.Expression {
				right := parseTypeAnnotation(parser)
				return &ast.CastingExpression{
					Operation:      operation,
					Expression:     left,
					TypeAnnotation: right,
				}
			}
		})(operation)

		setExprLeftBindingPower(tokenType, exprLeftBindingPowerCasting)
		setExprLeftDenotation(tokenType, leftDenotation)
	}
}

func parseCreateExpressionRemainder(p *parser, token lexer.Token) *ast.CreateExpression {
	invocation := parseNominalTypeInvocationRemainder(p)
	return &ast.CreateExpression{
		InvocationExpression: invocation,
		StartPos:             token.StartPos,
	}
}

// Invocation Expression Grammar:
//
//     invocation : '(' ( argument ( ',' argument )* )? ')'
//
func defineInvocationExpression() {
	setExprLeftBindingPower(lexer.TokenParenOpen, exprLeftBindingPowerAccess)
	setExprLeftDenotation(
		lexer.TokenParenOpen,
		func(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
			arguments, endPos := parseArgumentListRemainder(p)
			return &ast.InvocationExpression{
				InvokedExpression: left,
				Arguments:         arguments,
				EndPos:            endPos,
			}
		},
	)
}

func parseArgumentListRemainder(p *parser) (arguments []*ast.Argument, endPos ast.Position) {
	atEnd := false
	expectArgument := true
	for !atEnd {
		p.skipSpaceAndComments(true)

		switch p.current.Type {
		case lexer.TokenComma:
			if expectArgument {
				panic(fmt.Errorf(
					"expected argument or end of argument list, got %s",
					p.current.Type,
				))
			}
			// Skip the comma
			p.next()
			expectArgument = true

		case lexer.TokenParenClose:
			endPos = p.current.EndPos
			// Skip the closing paren
			p.next()
			atEnd = true

		case lexer.TokenEOF:
			panic(fmt.Errorf("missing ')' at end of invocation argument list"))

		default:
			if !expectArgument {
				panic(fmt.Errorf(
					"unexpected argument in argument list (expecting delimiter or end of argument list), got %s",
					p.current.Type,
				))
			}
			arguments = append(arguments, parseArgument(p))
			expectArgument = false
		}
	}
	return
}

// parseArgument parses an argument in an invocation.
//
//     argument : (identifier ':' )? expression
//
func parseArgument(p *parser) *ast.Argument {
	var label string
	var labelStartPos, labelEndPos ast.Position

	expr := parseExpression(p, lowestBindingPower)
	p.skipSpaceAndComments(true)

	// If a colon follows the expression, the expression was our label.
	if p.current.Is(lexer.TokenColon) {
		identifier, ok := expr.(*ast.IdentifierExpression)
		if !ok {
			panic(fmt.Errorf(
				"expected identifier for label, got %s",
				expr,
			))
		}
		label = identifier.Identifier.Identifier
		labelStartPos = expr.StartPosition()
		labelEndPos = expr.EndPosition()

		// Skip the identifier
		p.next()
		p.skipSpaceAndComments(true)

		expr = parseExpression(p, lowestBindingPower)
	}

	if len(label) > 0 {
		return &ast.Argument{
			Label:         label,
			LabelStartPos: &labelStartPos,
			LabelEndPos:   &labelEndPos,
			Expression:    expr,
		}
	}
	return &ast.Argument{Expression: expr}
}

func defineNestedExpression() {
	setExprNullDenotation(
		lexer.TokenParenOpen,
		func(p *parser, token lexer.Token) ast.Expression {
			expression := parseExpression(p, lowestBindingPower)
			p.mustOne(lexer.TokenParenClose)
			return expression
		},
	)
}

func defineArrayExpression() {
	setExprNullDenotation(
		lexer.TokenBracketOpen,
		func(p *parser, startToken lexer.Token) ast.Expression {
			var values []ast.Expression
			for !p.current.Is(lexer.TokenBracketClose) {
				value := parseExpression(p, lowestBindingPower)
				values = append(values, value)
				if !p.current.Is(lexer.TokenComma) {
					break
				}
				p.mustOne(lexer.TokenComma)
			}
			endToken := p.mustOne(lexer.TokenBracketClose)
			return &ast.ArrayExpression{
				Values: values,
				Range: ast.Range{
					StartPos: startToken.StartPos,
					EndPos:   endToken.EndPos,
				},
			}
		},
	)
}

func defineDictionaryExpression() {
	setExprNullDenotation(
		lexer.TokenBraceOpen,
		func(p *parser, startToken lexer.Token) ast.Expression {
			var entries []ast.Entry
			for !p.current.Is(lexer.TokenBraceClose) {
				key := parseExpression(p, lowestBindingPower)
				p.mustOne(lexer.TokenColon)
				value := parseExpression(p, lowestBindingPower)
				entries = append(entries, ast.Entry{
					Key:   key,
					Value: value,
				})
				if !p.current.Is(lexer.TokenComma) {
					break
				}
				p.mustOne(lexer.TokenComma)
			}
			endToken := p.mustOne(lexer.TokenBraceClose)
			return &ast.DictionaryExpression{
				Entries: entries,
				Range: ast.Range{
					StartPos: startToken.StartPos,
					EndPos:   endToken.EndPos,
				},
			}
		},
	)
}

func defineIndexExpression() {
	setExprLeftBindingPower(lexer.TokenBracketOpen, exprLeftBindingPowerAccess)
	setExprLeftDenotation(
		lexer.TokenBracketOpen,
		func(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
			firstIndexExpr := parseExpression(p, lowestBindingPower)
			endToken := p.mustOne(lexer.TokenBracketClose)
			return &ast.IndexExpression{
				TargetExpression:   left,
				IndexingExpression: firstIndexExpr,
				Range: ast.Range{
					StartPos: token.StartPos,
					EndPos:   endToken.EndPos,
				},
			}
		},
	)
}

func defineConditionalExpression() {
	setExprLeftBindingPower(lexer.TokenQuestionMark, exprLeftBindingPowerTernary)
	setExprLeftDenotation(
		lexer.TokenQuestionMark,
		func(p *parser, _ lexer.Token, left ast.Expression) ast.Expression {
			testExpression := left
			thenExpression := parseExpression(p, lowestBindingPower)
			p.mustOne(lexer.TokenColon)
			elseExpression := parseExpression(p, lowestBindingPower)
			return &ast.ConditionalExpression{
				Test: testExpression,
				Then: thenExpression,
				Else: elseExpression,
			}
		},
	)
}

func definePathExpression() {
	setExprNullDenotation(
		lexer.TokenSlash,
		func(p *parser, token lexer.Token) ast.Expression {
			domain := mustIdentifier(p)
			p.mustOne(lexer.TokenSlash)
			identifier := mustIdentifier(p)
			return &ast.PathExpression{
				Domain:     domain,
				Identifier: identifier,
				StartPos:   token.StartPos,
			}
		},
	)
}

func defineReferenceExpression() {
	setExprNullDenotation(
		lexer.TokenAmpersand,
		func(p *parser, token lexer.Token) ast.Expression {
			p.skipSpaceAndComments(true)
			// TODO: maybe require above unary
			expression := parseExpression(p, lowestBindingPower)

			p.skipSpaceAndComments(true)

			castingExpression, ok := expression.(*ast.CastingExpression)
			if !ok {
				panic(fmt.Errorf("expected casting expression"))
			}

			return &ast.ReferenceExpression{
				Expression: castingExpression.Expression,
				Type:       castingExpression.TypeAnnotation.Type,
				StartPos:   token.StartPos,
			}
		},
	)
}

func defineMemberExpression() {

	setExprLeftBindingPower(lexer.TokenDot, exprLeftBindingPowerAccess)
	setExprLeftDenotation(
		lexer.TokenDot,
		func(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
			return parseMemberAccess(p, left, false)
		},
	)

	setExprLeftBindingPower(lexer.TokenQuestionMarkDot, exprLeftBindingPowerAccess)
	setExprLeftDenotation(
		lexer.TokenQuestionMarkDot,
		func(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
			return parseMemberAccess(p, left, true)
		},
	)
}

func parseMemberAccess(p *parser, left ast.Expression, optional bool) ast.Expression {
	p.skipSpaceAndComments(true)
	identifier := mustIdentifier(p)
	return &ast.MemberExpression{
		Optional:   optional,
		Expression: left,
		Identifier: identifier,
	}
}

func exprLeftDenotationAllowsNewline(tokenType lexer.TokenType) bool {

	// The postfix force unwrap, invocation expressions,
	// and indexing expressions don't support newlines before them,
	// as this clashes with a unary negations, nested expressions,
	// and array literals on a new line / separate statement.

	switch tokenType {
	case lexer.TokenExclamationMark, lexer.TokenParenOpen, lexer.TokenBracketOpen:
		return false
	default:
		return true
	}
}

func parseExpression(p *parser, rightBindingPower int) ast.Expression {
	p.skipSpaceAndComments(true)
	t := p.current
	p.next()

	newLineAfterLeft := p.skipSpaceAndComments(true)

	left := applyExprNullDenotation(p, t)

	for {
		newLineAfterLeft = p.skipSpaceAndComments(true) || newLineAfterLeft

		if newLineAfterLeft && !exprLeftDenotationAllowsNewline(p.current.Type) {
			break
		}

		var done bool
		left, done = applyExprMetaLeftDenotation(p, rightBindingPower, left)
		if done {
			break
		}

		newLineAfterLeft = false
	}

	return left
}

func applyExprMetaLeftDenotation(
	p *parser,
	rightBindingPower int,
	left ast.Expression,
) (
	result ast.Expression,
	done bool,
) {
	t := p.current

	// Normal left denotations are applied if the right binding power
	// is less than the left binding power of the current token.
	//
	// Meta-left denotations allow determining the left binding power
	// based on parsing more tokens.

	if metaLeftDenotation, ok := exprMetaLeftDenotations[t.Type]; ok {
		return metaLeftDenotation(p, rightBindingPower, left)
	}

	if rightBindingPower >= exprLeftBindingPower(t) {
		return left, true
	}

	t = p.current

	p.next()
	p.skipSpaceAndComments(true)

	result = applyExprLeftDenotation(p, t, left)
	return result, false
}

func exprLeftBindingPower(token lexer.Token) int {
	tokenType := token.Type
	if tokenType == lexer.TokenIdentifier {
		identifier := token.Value.(string)
		return exprIdentifierLeftBindingPowers[identifier]
	}
	return exprLeftBindingPowers[tokenType]
}

func applyExprNullDenotation(p *parser, token lexer.Token) ast.Expression {
	tokenType := token.Type
	nullDenotation, ok := exprNullDenotations[tokenType]
	if !ok {
		panic(fmt.Errorf("missing expression null denotation for token %s", token.Type))
	}
	return nullDenotation(p, token)
}

func applyExprLeftDenotation(p *parser, token lexer.Token, left ast.Expression) ast.Expression {
	leftDenotation, ok := exprLeftDenotations[token.Type]
	if !ok {
		panic(fmt.Errorf("missing expression left denotation for token %s", token.Type))
	}
	return leftDenotation(p, token, left)
}

// parseStringLiteral parses a whole string literal, including start and end quotes
//
func parseStringLiteral(literal string) (result string, errs []error) {
	report := func(err error) {
		errs = append(errs, err)
	}

	length := len(literal)
	if length == 0 {
		report(fmt.Errorf("missing start of string literal: expected '\"'"))
		return
	}

	if length >= 1 {
		first := literal[0]
		if first != '"' {
			report(fmt.Errorf("invalid start of string literal: expected '\"', got %q", first))
		}
	}

	missingEnd := false
	endOffset := length
	if length >= 2 {
		lastIndex := length - 1
		last := literal[lastIndex]
		if last != '"' {
			missingEnd = true
		} else {
			endOffset = lastIndex
		}
	} else {
		missingEnd = true
	}

	var innerErrs []error
	result, innerErrs = parseStringLiteralContent(literal[1:endOffset])
	errs = append(errs, innerErrs...)

	if missingEnd {
		report(fmt.Errorf("invalid end of string literal: missing '\"'"))
	}

	return
}

// parseStringLiteralContent parses the string literalExpr contents, excluding start and end quotes
//
func parseStringLiteralContent(s string) (result string, errs []error) {

	var builder strings.Builder
	defer func() {
		result = builder.String()
	}()

	report := func(err error) {
		errs = append(errs, err)
	}

	length := len(s)

	var r rune
	index := 0

	atEnd := index >= length

	advance := func() {
		if atEnd {
			r = lexer.EOF
			return
		}

		var width int
		r, width = utf8.DecodeRuneInString(s[index:])
		index += width

		atEnd = index >= length
	}

	for index < length {
		advance()

		if r != '\\' {
			builder.WriteRune(r)
			continue
		}

		if atEnd {
			report(fmt.Errorf("incomplete escape sequence: missing character after escape character"))
			return
		}

		advance()

		switch r {
		case '0':
			builder.WriteByte(0)
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '"':
			builder.WriteByte('"')
		case '\'':
			builder.WriteByte('\'')
		case '\\':
			builder.WriteByte('\\')
		case 'u':
			if atEnd {
				report(fmt.Errorf(
					"incomplete Unicode escape sequence: missing character '{' after escape character",
				))
				return
			}
			advance()
			if r != '{' {
				report(fmt.Errorf("invalid Unicode escape sequence: expected '{', got %q", r))
				continue
			}

			var r2 rune
			valid := true
			digitIndex := 0
			for ; !atEnd && digitIndex < 8; digitIndex++ {
				advance()
				if r == '}' {
					break
				}

				parsed := parseHex(r)

				if parsed < 0 {
					report(fmt.Errorf("invalid Unicode escape sequence: expected hex digit, got %q", r))
					valid = false
				} else {
					r2 = r2<<4 | parsed
				}
			}

			if digitIndex > 0 && valid {
				builder.WriteRune(r2)
			}

			if r != '}' {
				advance()
			}

			switch r {
			case '}':
				break
			case lexer.EOF:
				report(fmt.Errorf(
					"incomplete Unicode escape sequence: missing character '}' after escape character",
				))
			default:
				report(fmt.Errorf("incomplete Unicode escape sequence: expected '}', got %q", r))
			}

		default:
			// TODO: include index/column in error
			report(fmt.Errorf("invalid escape character: %q", r))
			// skip invalid escape character, don't write to result
		}
	}

	return
}

func parseHex(r rune) rune {
	switch {
	case '0' <= r && r <= '9':
		return r - '0'
	case 'a' <= r && r <= 'f':
		return r - 'a' + 10
	case 'A' <= r && r <= 'F':
		return r - 'A' + 10
	}

	return -1
}

func parseIntegerLiteral(p *parser, literal, text string, kind IntegerLiteralKind, tokenRange ast.Range) *ast.IntegerExpression {

	report := func(invalidKind InvalidNumberLiteralKind) {
		p.report(
			&InvalidIntegerLiteralError{
				IntegerLiteralKind:        kind,
				InvalidIntegerLiteralKind: invalidKind,
				// NOTE: not using text, because it has the base-prefix stripped
				Literal: literal,
				Range:   tokenRange,
			},
		)
	}

	// check literal has no leading underscore

	if strings.HasPrefix(text, "_") {
		report(InvalidNumberLiteralKindLeadingUnderscore)
	}

	// check literal has no trailing underscore
	if strings.HasSuffix(text, "_") {
		report(InvalidNumberLiteralKindTrailingUnderscore)
	}

	withoutUnderscores := strings.Replace(text, "_", "", -1)

	base := kind.Base()
	value, ok := new(big.Int).SetString(withoutUnderscores, base)
	if !ok {
		report(InvalidNumberLiteralKindUnknown)
	}

	return &ast.IntegerExpression{
		Value: value,
		Base:  base,
		Range: tokenRange,
	}
}

func parseFixedPointPart(part string) (integer *big.Int, scale uint) {
	withoutUnderscores := strings.Replace(part, "_", "", -1)
	integer, _ = new(big.Int).SetString(withoutUnderscores, 10)
	if integer == nil {
		integer = new(big.Int)
	}
	scale = uint(len(withoutUnderscores))
	if scale == 0 {
		scale = 1
	}
	return integer, scale
}

func parseFixedPointLiteral(text string, tokenRange ast.Range) *ast.FixedPointExpression {
	parts := strings.Split(text, ".")
	integer, _ := parseFixedPointPart(parts[0])
	fractional, scale := parseFixedPointPart(parts[1])

	return &ast.FixedPointExpression{
		Negative:        false,
		UnsignedInteger: integer,
		Fractional:      fractional,
		Scale:           scale,
		Range:           tokenRange,
	}
}

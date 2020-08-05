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

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/errors"
	"github.com/onflow/cadence-initial/runtime/parser2/lexer"
)

const (
	typeLeftBindingPowerOptional = 10 * (iota + 1)
	typeLeftBindingPowerReference
	typeLeftBindingPowerRestriction
)

type typeNullDenotationFunc func(parser *parser, token lexer.Token) ast.Type

var typeNullDenotations = map[lexer.TokenType]typeNullDenotationFunc{}

type typeLeftDenotationFunc func(parser *parser, token lexer.Token, left ast.Type) ast.Type

var typeLeftBindingPowers = map[lexer.TokenType]int{}
var typeLeftDenotations = map[lexer.TokenType]typeLeftDenotationFunc{}

func setTypeNullDenotation(tokenType lexer.TokenType, nullDenotation typeNullDenotationFunc) {
	current := typeNullDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"type null denotation for token %s already exists",
			tokenType,
		))
	}
	typeNullDenotations[tokenType] = nullDenotation
}

func setTypeLeftBindingPower(tokenType lexer.TokenType, power int) {
	current := typeLeftBindingPowers[tokenType]
	if current > power {
		return
	}
	typeLeftBindingPowers[tokenType] = power
}

func setTypeLeftDenotation(tokenType lexer.TokenType, leftDenotation typeLeftDenotationFunc) {
	current := typeLeftDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"type left denotation for token %s already exists",
			tokenType,
		))
	}
	typeLeftDenotations[tokenType] = leftDenotation
}

type prefixTypeFunc func(right ast.Type, tokenRange ast.Range) ast.Type
type postfixTypeFunc func(left ast.Type, tokenRange ast.Range) ast.Type

type literalType struct {
	tokenType      lexer.TokenType
	nullDenotation typeNullDenotationFunc
}

type prefixType struct {
	tokenType      lexer.TokenType
	bindingPower   int
	nullDenotation prefixTypeFunc
}

type postfixType struct {
	tokenType      lexer.TokenType
	bindingPower   int
	leftDenotation postfixTypeFunc
}

func defineType(def interface{}) {
	switch def := def.(type) {
	case prefixType:
		tokenType := def.tokenType
		setTypeNullDenotation(
			tokenType,
			func(parser *parser, token lexer.Token) ast.Type {
				right := parseType(parser, def.bindingPower)
				return def.nullDenotation(right, token.Range)
			},
		)
	case postfixType:
		tokenType := def.tokenType
		setTypeLeftBindingPower(tokenType, def.bindingPower)
		setTypeLeftDenotation(
			tokenType,
			func(p *parser, token lexer.Token, left ast.Type) ast.Type {
				return def.leftDenotation(left, token.Range)
			},
		)
	case literalType:
		tokenType := def.tokenType
		setTypeNullDenotation(tokenType, def.nullDenotation)
	default:
		panic(errors.NewUnreachableError())
	}
}

func init() {
	defineArrayType()
	defineOptionalType()
	defineReferenceType()
	defineRestrictedOrDictionaryType()
	defineFunctionType()

	setTypeNullDenotation(
		lexer.TokenIdentifier,
		func(p *parser, token lexer.Token) ast.Type {

			switch token.Value {
			case keywordAuth:
				p.skipSpaceAndComments(true)
				p.mustOne(lexer.TokenAmpersand)
				right := parseType(p, typeLeftBindingPowerReference)
				return &ast.ReferenceType{
					Authorized: true,
					Type:       right,
					StartPos:   token.StartPos,
				}

			default:
				return parseNominalTypeRemainder(p, token)
			}
		},
	)
}

func parseNominalTypeRemainder(p *parser, token lexer.Token) *ast.NominalType {
	var nestedIdentifiers []ast.Identifier

	for p.current.Is(lexer.TokenDot) {
		// Skip the dot
		p.next()

		nestedToken := p.current

		if !nestedToken.Is(lexer.TokenIdentifier) {
			panic(fmt.Errorf(
				"expected identifier after %s, got %s",
				lexer.TokenDot,
				nestedToken.Type,
			))
		}

		nestedIdentifier := tokenToIdentifier(nestedToken)

		// Skip the identifier
		p.next()

		nestedIdentifiers = append(
			nestedIdentifiers,
			nestedIdentifier,
		)

	}

	return &ast.NominalType{
		Identifier:        tokenToIdentifier(token),
		NestedIdentifiers: nestedIdentifiers,
	}
}

func defineArrayType() {
	setTypeNullDenotation(
		lexer.TokenBracketOpen,
		func(p *parser, startToken lexer.Token) ast.Type {

			elementType := parseType(p, lowestBindingPower)

			p.skipSpaceAndComments(true)

			var size *ast.IntegerExpression

			if p.current.Is(lexer.TokenSemicolon) {
				// Skip the semicolon
				p.next()

				p.skipSpaceAndComments(true)

				numberExpression := parseExpression(p, lowestBindingPower)

				integerExpression, ok := numberExpression.(*ast.IntegerExpression)
				if !ok {
					p.report(fmt.Errorf(
						"expected integer size for constant sized type, got %s",
						numberExpression,
					))
				} else {
					size = integerExpression
				}
			}

			p.skipSpaceAndComments(true)

			endToken := p.mustOne(lexer.TokenBracketClose)

			typeRange := ast.Range{
				StartPos: startToken.StartPos,
				EndPos:   endToken.EndPos,
			}

			if size != nil {
				return &ast.ConstantSizedType{
					Type:  elementType,
					Size:  size,
					Range: typeRange,
				}
			} else {
				return &ast.VariableSizedType{
					Type:  elementType,
					Range: typeRange,
				}
			}
		},
	)
}

func defineOptionalType() {
	defineType(postfixType{
		tokenType:    lexer.TokenQuestionMark,
		bindingPower: typeLeftBindingPowerOptional,
		leftDenotation: func(left ast.Type, tokenRange ast.Range) ast.Type {
			return &ast.OptionalType{
				Type:   left,
				EndPos: tokenRange.EndPos,
			}
		},
	})

	defineType(postfixType{
		tokenType:    lexer.TokenDoubleQuestionMark,
		bindingPower: typeLeftBindingPowerOptional,
		leftDenotation: func(left ast.Type, tokenRange ast.Range) ast.Type {
			return &ast.OptionalType{
				Type: &ast.OptionalType{
					Type:   left,
					EndPos: tokenRange.StartPos,
				},
				EndPos: tokenRange.EndPos,
			}
		},
	})
}

func defineReferenceType() {
	defineType(prefixType{
		tokenType:    lexer.TokenAmpersand,
		bindingPower: typeLeftBindingPowerReference,
		nullDenotation: func(right ast.Type, tokenRange ast.Range) ast.Type {
			return &ast.ReferenceType{
				Authorized: false,
				Type:       right,
				StartPos:   tokenRange.StartPos,
			}
		},
	})
}

func defineRestrictedOrDictionaryType() {

	// For the null denotation it is not clear after the start
	// if it is a restricted type or a dictionary type.
	//
	// If a colon is seen it is a dictionary type.
	// If no colon is seen it is a restricted type.

	setTypeNullDenotation(
		lexer.TokenBraceOpen,
		func(p *parser, startToken lexer.Token) ast.Type {

			var endPos ast.Position

			var dictionaryType *ast.DictionaryType
			var restrictedType *ast.RestrictedType

			var firstType ast.Type

			atEnd := false

			expectType := true

			for !atEnd {
				p.skipSpaceAndComments(true)

				switch p.current.Type {
				case lexer.TokenComma:
					if dictionaryType != nil {
						panic(fmt.Errorf("unexpected comma in dictionary type"))
					}
					if expectType {
						panic(fmt.Errorf("unexpected comma in restricted type"))
					}
					if restrictedType == nil {
						firstNominalType, ok := firstType.(*ast.NominalType)
						if !ok {
							panic(fmt.Errorf("non-nominal type in restriction list: %s", firstType))
						}
						restrictedType = &ast.RestrictedType{
							Restrictions: []*ast.NominalType{
								firstNominalType,
							},
							Range: ast.Range{
								StartPos: startToken.StartPos,
							},
						}
					}
					// Skip the comma
					p.next()
					expectType = true

				case lexer.TokenColon:
					if restrictedType != nil {
						panic(fmt.Errorf("unexpected colon in restricted type"))
					}
					if expectType {
						panic(fmt.Errorf("unexpected colon in dictionary type"))
					}
					if dictionaryType == nil {
						if firstType == nil {
							panic(fmt.Errorf("unexpected colon after missing dictionary key type"))
						}
						dictionaryType = &ast.DictionaryType{
							KeyType: firstType,
							Range: ast.Range{
								StartPos: startToken.StartPos,
							},
						}
					} else {
						panic(fmt.Errorf("unexpected colon in dictionary type"))
					}
					// Skip the colon
					p.next()
					expectType = true

				case lexer.TokenBraceClose:
					if expectType {
						switch {
						case dictionaryType != nil:
							p.report(fmt.Errorf("missing dictionary value type"))
						case restrictedType != nil:
							p.report(fmt.Errorf("missing type after comma"))
						}
					}
					endPos = p.current.EndPos
					// Skip the closing brace
					p.next()
					atEnd = true

				case lexer.TokenEOF:
					if expectType {
						panic(fmt.Errorf("invalid end of input, expected type"))
					} else {
						panic(fmt.Errorf("invalid end of input, expected %s", lexer.TokenBraceClose))
					}

				default:
					if !expectType {
						panic(fmt.Errorf("unexpected type"))
					}

					ty := parseType(p, lowestBindingPower)

					expectType = false

					switch {
					case dictionaryType != nil:
						dictionaryType.ValueType = ty

					case restrictedType != nil:
						nominalType, ok := ty.(*ast.NominalType)
						if !ok {
							panic(fmt.Errorf("non-nominal type in restriction list: %s", ty))
						}
						restrictedType.Restrictions = append(restrictedType.Restrictions, nominalType)

					default:
						firstType = ty
					}
				}
			}

			switch {
			case restrictedType != nil:
				restrictedType.EndPos = endPos
				return restrictedType
			case dictionaryType != nil:
				dictionaryType.EndPos = endPos
				return dictionaryType
			default:
				restrictedType = &ast.RestrictedType{
					Range: ast.Range{
						StartPos: startToken.StartPos,
						EndPos:   endPos,
					},
				}
				if firstType != nil {
					firstNominalType, ok := firstType.(*ast.NominalType)
					if !ok {
						panic(fmt.Errorf("non-nominal type in restriction list: %s", firstType))
					}
					restrictedType.Restrictions = append(restrictedType.Restrictions, firstNominalType)
				}
				return restrictedType
			}
		},
	)

	// For the left denotation we definitely know it is a restricted type

	setTypeLeftBindingPower(lexer.TokenBraceOpen, typeLeftBindingPowerRestriction)
	setTypeLeftDenotation(
		lexer.TokenBraceOpen,
		func(p *parser, token lexer.Token, left ast.Type) ast.Type {

			nominalTypes, endPos := parseNominalTypes(p, lexer.TokenBraceClose)

			// Skip the closing brace
			p.next()

			return &ast.RestrictedType{
				Type:         left,
				Restrictions: nominalTypes,
				Range: ast.Range{
					StartPos: left.StartPosition(),
					EndPos:   endPos,
				},
			}
		},
	)
}

// parseNominalTypes parses zero or more nominal types separated by comma.
//
func parseNominalTypes(
	p *parser,
	endTokenType lexer.TokenType,
) (
	nominalTypes []*ast.NominalType,
	endPos ast.Position,
) {
	expectType := true
	atEnd := false
	for !atEnd {
		p.skipSpaceAndComments(true)

		switch p.current.Type {
		case lexer.TokenComma:
			if expectType {
				panic(fmt.Errorf("unexpected comma"))
			}
			// Skip the comma
			p.next()
			expectType = true

		case endTokenType:
			if expectType && len(nominalTypes) > 0 {
				p.report(fmt.Errorf("missing type after comma"))
			}
			endPos = p.current.EndPos
			atEnd = true

		case lexer.TokenEOF:
			if expectType {
				panic(fmt.Errorf("invalid end of input, expected type"))
			} else {
				panic(fmt.Errorf("invalid end of input, expected %s", endTokenType))
			}

		default:
			if !expectType {
				panic(fmt.Errorf(
					"unexpected token: got %s, expected %s or %s",
					p.current.Type,
					lexer.TokenComma,
					endTokenType,
				))
			}

			ty := parseType(p, lowestBindingPower)

			expectType = false

			nominalType, ok := ty.(*ast.NominalType)
			if !ok {
				panic(fmt.Errorf("unexpected non-nominal type: %s", ty))
			}
			nominalTypes = append(nominalTypes, nominalType)
		}
	}

	return
}

func defineFunctionType() {
	setTypeNullDenotation(
		lexer.TokenParenOpen,
		func(p *parser, startToken lexer.Token) ast.Type {

			parameterTypeAnnotations := parseParameterTypeAnnotations(p)

			p.skipSpaceAndComments(true)
			p.mustOne(lexer.TokenColon)

			p.skipSpaceAndComments(true)
			returnTypeAnnotation := parseTypeAnnotation(p)

			p.skipSpaceAndComments(true)
			endToken := p.mustOne(lexer.TokenParenClose)

			return &ast.FunctionType{
				ParameterTypeAnnotations: parameterTypeAnnotations,
				ReturnTypeAnnotation:     returnTypeAnnotation,
				Range: ast.Range{
					StartPos: startToken.StartPos,
					EndPos:   endToken.EndPos,
				},
			}
		},
	)
}

func parseParameterTypeAnnotations(p *parser) (typeAnnotations []*ast.TypeAnnotation) {

	p.skipSpaceAndComments(true)
	p.mustOne(lexer.TokenParenOpen)

	expectTypeAnnotation := true

	atEnd := false
	for !atEnd {
		p.skipSpaceAndComments(true)
		switch p.current.Type {
		case lexer.TokenComma:
			if expectTypeAnnotation {
				panic(fmt.Errorf(
					"expected type annotation or end of list, got %q",
					p.current.Type,
				))
			}
			// Skip the comma
			p.next()
			expectTypeAnnotation = true

		case lexer.TokenParenClose:
			// Skip the closing paren
			p.next()
			atEnd = true

		case lexer.TokenEOF:
			panic(fmt.Errorf(
				"missing %q at end of list",
				lexer.TokenParenClose,
			))

		default:
			if !expectTypeAnnotation {
				panic(fmt.Errorf(
					"expected comma or end of list, got %q",
					p.current.Type,
				))
			}

			typeAnnotation := parseTypeAnnotation(p)
			typeAnnotations = append(typeAnnotations, typeAnnotation)

			expectTypeAnnotation = false
		}
	}

	return
}

func parseType(p *parser, rightBindingPower int) ast.Type {
	p.skipSpaceAndComments(true)
	t := p.current
	p.next()

	left := applyTypeNullDenotation(p, t)

	for rightBindingPower < typeLeftBindingPowers[p.current.Type] {
		t = p.current
		p.next()

		left = applyTypeLeftDenotation(p, t, left)
	}

	return left
}

func parseTypeAnnotation(p *parser) *ast.TypeAnnotation {
	startPos := p.current.StartPos

	isResource := false
	if p.current.Is(lexer.TokenAt) {
		// Skip the `@`
		p.next()
		isResource = true
	}

	ty := parseType(p, lowestBindingPower)

	return &ast.TypeAnnotation{
		IsResource: isResource,
		Type:       ty,
		StartPos:   startPos,
	}
}

func applyTypeNullDenotation(p *parser, token lexer.Token) ast.Type {
	tokenType := token.Type
	nullDenotation, ok := typeNullDenotations[tokenType]
	if !ok {
		panic(fmt.Errorf("missing type null denotation for token %s", token.Type))
	}
	return nullDenotation(p, token)
}

func applyTypeLeftDenotation(p *parser, token lexer.Token, left ast.Type) ast.Type {
	leftDenotation, ok := typeLeftDenotations[token.Type]
	if !ok {
		panic(fmt.Errorf("missing type left denotation for token %s", token.Type))
	}
	return leftDenotation(p, token, left)
}

func parseNominalTypeInvocationRemainder(p *parser) *ast.InvocationExpression {
	p.skipSpaceAndComments(true)
	identifier := p.mustOne(lexer.TokenIdentifier)
	ty := parseNominalTypeRemainder(p, identifier)

	p.skipSpaceAndComments(true)
	p.mustOne(lexer.TokenParenOpen)
	arguments, endPos := parseArgumentListRemainder(p)

	var invokedExpression ast.Expression = &ast.IdentifierExpression{
		Identifier: ty.Identifier,
	}

	for _, nestedIdentifier := range ty.NestedIdentifiers {
		invokedExpression = &ast.MemberExpression{
			Expression: invokedExpression,
			Identifier: nestedIdentifier,
		}
	}

	return &ast.InvocationExpression{
		InvokedExpression: invokedExpression,
		Arguments:         arguments,
		EndPos:            endPos,
	}
}

// parseCommaSeparatedTypeAnnotations parses zero or more type annotations separated by comma.
//
func parseCommaSeparatedTypeAnnotations(
	p *parser,
	endTokenType lexer.TokenType,
) (
	typeAnnotations []*ast.TypeAnnotation,
	endPos ast.Position,
) {
	expectTypeAnnotation := true
	atEnd := false
	for !atEnd {
		p.skipSpaceAndComments(true)

		switch p.current.Type {
		case lexer.TokenComma:
			if expectTypeAnnotation {
				panic(fmt.Errorf("unexpected comma"))
			}
			// Skip the comma
			p.next()
			expectTypeAnnotation = true

		case endTokenType:
			if expectTypeAnnotation && len(typeAnnotations) > 0 {
				p.report(fmt.Errorf("missing type annotation after comma"))
			}
			endPos = p.current.EndPos
			atEnd = true

		case lexer.TokenEOF:
			if expectTypeAnnotation {
				panic(fmt.Errorf("invalid end of input, expected type"))
			} else {
				panic(fmt.Errorf("invalid end of input, expected %s", endTokenType))
			}

		default:
			if !expectTypeAnnotation {
				panic(fmt.Errorf(
					"unexpected token: got %s, expected %s or %s",
					p.current.Type,
					lexer.TokenComma,
					endTokenType,
				))
			}

			typeAnnotation := parseTypeAnnotation(p)
			typeAnnotations = append(typeAnnotations, typeAnnotation)

			expectTypeAnnotation = false
		}
	}

	return
}

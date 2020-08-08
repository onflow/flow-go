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

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/parser2/lexer"
)

func parseParameterList(p *parser) (parameterList *ast.ParameterList) {
	var parameters []*ast.Parameter

	p.skipSpaceAndComments(true)

	if !p.current.Is(lexer.TokenParenOpen) {
		panic(fmt.Errorf(
			"expected %s as start of parameter list, got %s",
			lexer.TokenParenOpen,
			p.current.Type,
		))
	}

	startPos := p.current.StartPos
	// Skip the opening paren
	p.next()

	var endPos ast.Position

	expectParameter := true

	atEnd := false
	for !atEnd {
		p.skipSpaceAndComments(true)
		switch p.current.Type {
		case lexer.TokenIdentifier:
			parameter := parseParameter(p)
			parameters = append(parameters, parameter)
			expectParameter = false

		case lexer.TokenComma:
			if expectParameter {
				panic(fmt.Errorf(
					"expected parameter or end of parameter list, got %s",
					p.current.Type,
				))
			}
			// Skip the comma
			p.next()
			expectParameter = true

		case lexer.TokenParenClose:
			endPos = p.current.EndPos
			// Skip the closing paren
			p.next()
			atEnd = true

		case lexer.TokenEOF:
			panic(fmt.Errorf(
				"missing %s at end of parameter list",
				lexer.TokenParenClose,
			))

		default:
			if expectParameter {
				panic(fmt.Errorf(
					"expected parameter or end of parameter list, got %s",
					p.current.Type,
				))
			} else {
				panic(fmt.Errorf(
					"expected comma or end of parameter list, got %s",
					p.current.Type,
				))
			}
		}
	}

	return &ast.ParameterList{
		Parameters: parameters,
		Range: ast.Range{
			StartPos: startPos,
			EndPos:   endPos,
		},
	}
}

func parseParameter(p *parser) *ast.Parameter {
	p.skipSpaceAndComments(true)

	startPos := p.current.StartPos
	parameterPos := startPos

	if !p.current.Is(lexer.TokenIdentifier) {
		panic(fmt.Errorf(
			"expected argument label or parameter name, got %s",
			p.current.Type,
		))
	}
	argumentLabel := ""
	parameterName := p.current.Value.(string)
	// Skip the identifier
	p.next()

	// If another identifier is provided, then the previous identifier
	// is the argument label, and this identifier is the parameter name

	p.skipSpaceAndComments(true)
	if p.current.Is(lexer.TokenIdentifier) {
		argumentLabel = parameterName
		parameterName = p.current.Value.(string)
		parameterPos = p.current.StartPos
		// Skip the identifier
		p.next()
		p.skipSpaceAndComments(true)
	}

	if !p.current.Is(lexer.TokenColon) {
		panic(fmt.Errorf(
			"expected %s after argument label/parameter name, got %s",
			lexer.TokenColon,
			p.current.Type,
		))
	}

	// Skip the colon
	p.next()
	p.skipSpaceAndComments(true)

	typeAnnotation := parseTypeAnnotation(p)

	endPos := typeAnnotation.EndPosition()

	return &ast.Parameter{
		Label: argumentLabel,
		Identifier: ast.Identifier{
			Identifier: parameterName,
			Pos:        parameterPos,
		},
		TypeAnnotation: typeAnnotation,
		Range: ast.Range{
			StartPos: startPos,
			EndPos:   endPos,
		},
	}
}

func parseFunctionDeclaration(
	p *parser,
	functionBlockIsOptional bool,
	access ast.Access,
	accessPos *ast.Position,
) *ast.FunctionDeclaration {

	startPos := p.current.StartPos
	if accessPos != nil {
		startPos = *accessPos
	}

	// Skip the `fun` keyword
	p.next()

	p.skipSpaceAndComments(true)
	if !p.current.Is(lexer.TokenIdentifier) {
		panic(fmt.Errorf(
			"expected identifier after start of function declaration, got %s",
			p.current.Type,
		))
	}

	identifier := tokenToIdentifier(p.current)

	// Skip the identifier
	p.next()

	parameterList, returnTypeAnnotation, functionBlock :=
		parseFunctionParameterListAndRest(p, functionBlockIsOptional)

	return &ast.FunctionDeclaration{
		Access:               access,
		Identifier:           identifier,
		ParameterList:        parameterList,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             startPos,
	}
}

func parseFunctionParameterListAndRest(
	p *parser,
	functionBlockIsOptional bool,
) (
	parameterList *ast.ParameterList,
	returnTypeAnnotation *ast.TypeAnnotation,
	functionBlock *ast.FunctionBlock,
) {
	parameterList = parseParameterList(p)

	p.skipSpaceAndComments(true)
	if p.current.Is(lexer.TokenColon) {
		// Skip the colon
		p.next()
		p.skipSpaceAndComments(true)
		returnTypeAnnotation = parseTypeAnnotation(p)
		p.skipSpaceAndComments(true)
	} else {
		positionBeforeMissingReturnType := parameterList.EndPos
		returnType := &ast.NominalType{
			Identifier: ast.Identifier{
				Pos: positionBeforeMissingReturnType,
			},
		}
		returnTypeAnnotation = &ast.TypeAnnotation{
			IsResource: false,
			Type:       returnType,
			StartPos:   positionBeforeMissingReturnType,
		}
	}

	p.skipSpaceAndComments(true)

	if !functionBlockIsOptional ||
		p.current.Is(lexer.TokenBraceOpen) {

		functionBlock = parseFunctionBlock(p)
	}
	return
}

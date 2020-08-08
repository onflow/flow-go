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
	"example.com/cadence-initial/runtime/errors"
	"example.com/cadence-initial/runtime/parser2/lexer"
)

func parseStatements(p *parser, endTokenType lexer.TokenType) (statements []ast.Statement) {
	for {
		p.skipSpaceAndComments(true)
		switch p.current.Type {
		case lexer.TokenSemicolon:
			p.next()
			continue
		case endTokenType, lexer.TokenEOF:
			return
		default:
			statement := parseStatement(p)
			if statement == nil {
				return
			}

			statements = append(statements, statement)
		}
	}
}

func parseStatement(p *parser) ast.Statement {
	p.skipSpaceAndComments(true)

	// It might start with a keyword for a statement

	switch p.current.Type {
	case lexer.TokenIdentifier:
		switch p.current.Value {
		case keywordReturn:
			return parseReturnStatement(p)
		case keywordBreak:
			return parseBreakStatement(p)
		case keywordContinue:
			return parseContinueStatement(p)
		case keywordIf:
			return parseIfStatement(p)
		case keywordWhile:
			return parseWhileStatement(p)
		case keywordFor:
			return parseForStatement(p)
		case keywordEmit:
			return parseEmitStatement(p)
		case keywordFun:
			// The `fun` keyword is ambiguous: it either introduces a function expression
			// or a function declaration, depending on if an identifier follows, or not.
			return parseFunctionDeclarationOrFunctionExpressionStatement(p)
		}
	}

	// If it is not a keyword for a statement,
	// it might start with a keyword for a declaration

	declaration := parseDeclaration(p)
	if statement, ok := declaration.(ast.Statement); ok {
		return statement
	}

	// If it is not a statement or declaration,
	// it must be an expression

	expression := parseExpression(p, lowestBindingPower)

	// If the expression is followed by a transfer,
	// it is actually the target of an assignment or swap statement

	p.skipSpaceAndComments(true)
	switch p.current.Type {
	case lexer.TokenEqual, lexer.TokenLeftArrow, lexer.TokenLeftArrowExclamation:
		transfer := parseTransfer(p)
		p.next()

		value := parseExpression(p, lowestBindingPower)

		return &ast.AssignmentStatement{
			Target:   expression,
			Transfer: transfer,
			Value:    value,
		}

	case lexer.TokenSwap:
		p.next()

		right := parseExpression(p, lowestBindingPower)

		return &ast.SwapStatement{
			Left:  expression,
			Right: right,
		}

	default:
		return &ast.ExpressionStatement{
			Expression: expression,
		}
	}
}

func parseFunctionDeclarationOrFunctionExpressionStatement(p *parser) ast.Statement {

	startPos := p.current.StartPos

	// Skip the `fun` keyword
	p.next()

	p.skipSpaceAndComments(true)

	if p.current.Is(lexer.TokenIdentifier) {
		identifier := tokenToIdentifier(p.current)

		p.next()

		parameterList, returnTypeAnnotation, functionBlock :=
			parseFunctionParameterListAndRest(p, false)

		return &ast.FunctionDeclaration{
			Access:               ast.AccessNotSpecified,
			Identifier:           identifier,
			ParameterList:        parameterList,
			ReturnTypeAnnotation: returnTypeAnnotation,
			FunctionBlock:        functionBlock,
			StartPos:             startPos,
		}
	} else {
		parameterList, returnTypeAnnotation, functionBlock :=
			parseFunctionParameterListAndRest(p, false)

		return &ast.ExpressionStatement{
			Expression: &ast.FunctionExpression{
				ParameterList:        parameterList,
				ReturnTypeAnnotation: returnTypeAnnotation,
				FunctionBlock:        functionBlock,
				StartPos:             startPos,
			},
		}
	}
}

func parseReturnStatement(p *parser) *ast.ReturnStatement {
	tokenRange := p.current.Range
	endPosition := tokenRange.EndPos
	p.next()
	sawNewLine := p.skipSpaceAndComments(false)

	var expression ast.Expression
	switch p.current.Type {
	case lexer.TokenEOF, lexer.TokenSemicolon, lexer.TokenBraceClose:
		break
	default:
		if !sawNewLine {
			expression = parseExpression(p, lowestBindingPower)
			endPosition = expression.EndPosition()
		}
	}

	return &ast.ReturnStatement{
		Expression: expression,
		Range: ast.Range{
			StartPos: tokenRange.StartPos,
			EndPos:   endPosition,
		},
	}
}

func parseBreakStatement(p *parser) *ast.BreakStatement {
	tokenRange := p.current.Range
	p.next()

	return &ast.BreakStatement{
		Range: tokenRange,
	}
}

func parseContinueStatement(p *parser) *ast.ContinueStatement {
	tokenRange := p.current.Range
	p.next()

	return &ast.ContinueStatement{
		Range: tokenRange,
	}
}

func parseIfStatement(p *parser) *ast.IfStatement {

	var ifStatements []*ast.IfStatement

	for {
		startPos := p.current.StartPos
		p.next()

		p.skipSpaceAndComments(true)

		var variableDeclaration *ast.VariableDeclaration

		if p.current.Type == lexer.TokenIdentifier {
			switch p.current.Value {
			case keywordLet, keywordVar:
				variableDeclaration =
					parseVariableDeclaration(p, ast.AccessNotSpecified, nil)
			}
		}

		var expression ast.Expression

		if variableDeclaration == nil {
			expression = parseExpression(p, lowestBindingPower)
		}

		thenBlock := parseBlock(p)

		var elseBlock *ast.Block

		parseNested := false

		p.skipSpaceAndComments(true)
		if p.current.IsString(lexer.TokenIdentifier, keywordElse) {
			p.next()

			p.skipSpaceAndComments(true)
			if p.current.IsString(lexer.TokenIdentifier, keywordIf) {
				parseNested = true
			} else {
				elseBlock = parseBlock(p)
			}
		}

		var test ast.IfStatementTest
		switch {
		case variableDeclaration != nil:
			test = variableDeclaration
		case expression != nil:
			test = expression
		default:
			panic(errors.UnreachableError{})
		}

		ifStatement := &ast.IfStatement{
			Test:     test,
			Then:     thenBlock,
			Else:     elseBlock,
			StartPos: startPos,
		}

		if variableDeclaration != nil {
			variableDeclaration.ParentIfStatement = ifStatement
		}

		ifStatements = append(ifStatements, ifStatement)

		if !parseNested {
			break
		}
	}

	length := len(ifStatements)

	result := ifStatements[length-1]

	for i := length - 2; i >= 0; i-- {
		outer := ifStatements[i]
		outer.Else = &ast.Block{
			Statements: []ast.Statement{result},
			Range:      ast.NewRangeFromPositioned(result),
		}
		result = outer
	}

	return result
}

func parseWhileStatement(p *parser) *ast.WhileStatement {

	startPos := p.current.StartPos
	p.next()

	expression := parseExpression(p, lowestBindingPower)

	block := parseBlock(p)

	return &ast.WhileStatement{
		Test:     expression,
		Block:    block,
		StartPos: startPos,
	}
}

func parseForStatement(p *parser) *ast.ForStatement {

	startPos := p.current.StartPos
	p.next()

	p.skipSpaceAndComments(true)

	identifier := mustIdentifier(p)

	p.skipSpaceAndComments(true)

	if !p.current.IsString(lexer.TokenIdentifier, keywordIn) {
		panic(fmt.Errorf(
			"expected keyword %q, got %q",
			keywordIn,
			p.current.Type,
		))
	}

	p.next()

	expression := parseExpression(p, lowestBindingPower)

	block := parseBlock(p)

	return &ast.ForStatement{
		Identifier: identifier,
		Block:      block,
		Value:      expression,
		StartPos:   startPos,
	}
}

func parseBlock(p *parser) *ast.Block {
	startToken := p.mustOne(lexer.TokenBraceOpen)
	statements := parseStatements(p, lexer.TokenBraceClose)
	endToken := p.mustOne(lexer.TokenBraceClose)

	return &ast.Block{
		Statements: statements,
		Range: ast.Range{
			StartPos: startToken.StartPos,
			EndPos:   endToken.EndPos,
		},
	}
}

func parseFunctionBlock(p *parser) *ast.FunctionBlock {
	p.skipSpaceAndComments(true)

	startToken := p.mustOne(lexer.TokenBraceOpen)

	p.skipSpaceAndComments(true)

	var preConditions *ast.Conditions
	if p.current.IsString(lexer.TokenIdentifier, keywordPre) {
		p.next()
		conditions := parseConditions(p, ast.ConditionKindPre)
		preConditions = &conditions
	}

	p.skipSpaceAndComments(true)

	var postConditions *ast.Conditions
	if p.current.IsString(lexer.TokenIdentifier, keywordPost) {
		p.next()
		conditions := parseConditions(p, ast.ConditionKindPost)
		postConditions = &conditions
	}

	statements := parseStatements(p, lexer.TokenBraceClose)

	endToken := p.mustOne(lexer.TokenBraceClose)

	return &ast.FunctionBlock{
		Block: &ast.Block{
			Statements: statements,
			Range: ast.Range{
				StartPos: startToken.StartPos,
				EndPos:   endToken.EndPos,
			},
		},
		PreConditions:  preConditions,
		PostConditions: postConditions,
	}
}

// parseConditions parses conditions (pre/post)
//
func parseConditions(p *parser, kind ast.ConditionKind) (conditions ast.Conditions) {

	p.skipSpaceAndComments(true)
	p.mustOne(lexer.TokenBraceOpen)

	defer func() {
		p.skipSpaceAndComments(true)
		p.mustOne(lexer.TokenBraceClose)
	}()

	for {
		p.skipSpaceAndComments(true)
		switch p.current.Type {
		case lexer.TokenSemicolon:
			p.next()
			continue

		case lexer.TokenBraceClose, lexer.TokenEOF:
			return

		default:
			condition := parseCondition(p, kind)
			if condition == nil {
				return
			}

			conditions = append(conditions, condition)
		}
	}
}

// parseCondition parses a condition (pre/post)
//
//    condition : expression (':' expression )?
//
func parseCondition(p *parser, kind ast.ConditionKind) *ast.Condition {

	test := parseExpression(p, lowestBindingPower)

	p.skipSpaceAndComments(true)

	var message ast.Expression
	if p.current.Is(lexer.TokenColon) {
		p.next()

		message = parseExpression(p, lowestBindingPower)
	}

	return &ast.Condition{
		Kind:    kind,
		Test:    test,
		Message: message,
	}
}

func parseEmitStatement(p *parser) *ast.EmitStatement {
	startPos := p.current.StartPos
	p.next()

	invocation := parseNominalTypeInvocationRemainder(p)
	return &ast.EmitStatement{
		InvocationExpression: invocation,
		StartPos:             startPos,
	}
}

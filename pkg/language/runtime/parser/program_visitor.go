package parser

import (
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
	"math/big"
	"strconv"
	"strings"
)

type ProgramVisitor struct {
	*BaseStrictusVisitor
}

func (v *ProgramVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	var allDeclarations []ast.Declaration

	for _, declarationContext := range ctx.AllDeclaration() {
		declarationResult := declarationContext.Accept(v)
		if declarationResult == nil {
			return nil
		}
		declaration := declarationResult.(ast.Declaration)
		allDeclarations = append(allDeclarations, declaration)
	}

	return &ast.Program{
		Declarations: allDeclarations,
	}
}

func (v *ProgramVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	isPublic := ctx.Pub() != nil
	identifier := ctx.Identifier().GetText()
	parameterListEnd := ctx.ParameterList().GetStop()
	returnType := v.visitReturnType(ctx.returnType, parameterListEnd)
	var parameters []*ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]*ast.Parameter)
	}
	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)
	identifierPosition := ast.PositionFromToken(ctx.Identifier().GetSymbol())

	return &ast.FunctionDeclaration{
		IsPublic:      isPublic,
		Identifier:    identifier,
		Parameters:    parameters,
		ReturnType:    returnType,
		Block:         block,
		StartPos:      startPosition,
		EndPos:        endPosition,
		IdentifierPos: identifierPosition,
	}
}

// visitReturnType returns the return type.
// if none was given in the program, return an empty type with the position of tokenBefore
func (v *ProgramVisitor) visitReturnType(ctx IFullTypeContext, tokenBefore antlr.Token) ast.Type {
	if ctx == nil {
		positionBeforeMissingReturnType := ast.PositionFromToken(tokenBefore)
		return &ast.BaseType{
			Pos: positionBeforeMissingReturnType,
		}
	}
	result := ctx.Accept(v)
	if result == nil {
		return nil
	}
	return result.(ast.Type)
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	parameterListEnd := ctx.ParameterList().GetStop()
	returnType := v.visitReturnType(ctx.returnType, parameterListEnd)
	var parameters []*ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]*ast.Parameter)
	}
	block := ctx.Block().Accept(v).(*ast.Block)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.FunctionExpression{
		Parameters: parameters,
		ReturnType: returnType,
		Block:      block,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	var parameters []*ast.Parameter

	for _, parameter := range ctx.AllParameter() {
		parameters = append(
			parameters,
			parameter.Accept(v).(*ast.Parameter),
		)
	}

	return parameters
}

func (v *ProgramVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	// label
	label := ""
	var labelPos *ast.Position
	if ctx.argumentLabel != nil {
		label = ctx.argumentLabel.GetText()
		labelPos = ast.PositionFromToken(ctx.argumentLabel)
	}

	// identifier
	identifier := ctx.parameterName.GetText()
	identifierPos := ast.PositionFromToken(ctx.parameterName)

	fullType := ctx.FullType().Accept(v).(ast.Type)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.Parameter{
		Label:         label,
		Identifier:    identifier,
		Type:          fullType,
		LabelPos:      labelPos,
		IdentifierPos: identifierPos,
		StartPos:      startPosition,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	identifierNode := ctx.Identifier()
	// identifier?
	if identifierNode != nil {
		identifier := identifierNode.GetText()
		position := ast.PositionFromToken(identifierNode.GetSymbol())
		return &ast.BaseType{
			Identifier: identifier,
			Pos:        position,
		}
	}

	// alternative: function type
	functionTypeContext := ctx.FunctionType()
	if functionTypeContext != nil {
		return functionTypeContext.Accept(v)
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {

	var parameterTypes []ast.Type
	for _, fullType := range ctx.parameterTypes {
		parameterTypes = append(
			parameterTypes,
			fullType.Accept(v).(ast.Type),
		)
	}

	if ctx.returnType == nil {
		return nil
	}
	returnType := ctx.returnType.Accept(v).(ast.Type)

	startPosition := ast.PositionFromToken(ctx.OpenParen(0).GetSymbol())
	endPosition := returnType.EndPosition()

	return &ast.FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     returnType,
		StartPos:       startPosition,
		EndPos:         endPosition,
	}
}

func (v *ProgramVisitor) VisitFullType(ctx *FullTypeContext) interface{} {
	baseTypeResult := ctx.BaseType().Accept(v)
	if baseTypeResult == nil {
		return nil
	}
	result := baseTypeResult.(ast.Type)

	// reduce in reverse
	dimensions := ctx.AllTypeDimension()
	lastDimensionIndex := len(dimensions) - 1
	for i := range dimensions {
		dimensionContext := dimensions[lastDimensionIndex-i]
		dimension := dimensionContext.Accept(v).(*int)
		startPosition, endPosition := ast.PositionRangeFromContext(dimensionContext)
		if dimension == nil {
			result = &ast.VariableSizedType{
				Type:     result,
				StartPos: startPosition,
				EndPos:   endPosition,
			}
		} else {
			result = &ast.ConstantSizedType{
				Type:     result,
				Size:     *dimension,
				StartPos: startPosition,
				EndPos:   endPosition,
			}
		}
	}

	return result
}

func (v *ProgramVisitor) VisitTypeDimension(ctx *TypeDimensionContext) interface{} {
	var result *int

	literalContext := ctx.DecimalLiteral()
	if literalContext == nil {
		return result
	}

	value, err := strconv.Atoi(literalContext.GetText())
	if err != nil {
		return result
	}

	result = &value

	return result
}

func (v *ProgramVisitor) VisitBlock(ctx *BlockContext) interface{} {
	statements := ctx.Statements().Accept(v).([]ast.Statement)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.Block{
		Statements: statements,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitStatements(ctx *StatementsContext) interface{} {
	var statements []ast.Statement
	for _, statement := range ctx.AllStatement() {
		statements = append(
			statements,
			statement.Accept(v).(ast.Statement),
		)
	}
	return statements
}

func (v *ProgramVisitor) VisitChildren(node antlr.RuleNode) interface{} {
	for _, child := range node.GetChildren() {
		ruleChild, ok := child.(antlr.RuleNode)
		if !ok {
			continue
		}

		result := ruleChild.Accept(v)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *ProgramVisitor) VisitStatement(ctx *StatementContext) interface{} {
	result := v.VisitChildren(ctx.BaseParserRuleContext)
	if expression, ok := result.(ast.Expression); ok {
		return &ast.ExpressionStatement{
			Expression: expression,
		}
	}

	return result
}

func (v *ProgramVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
	expressionNode := ctx.Expression()
	var expression ast.Expression
	if expressionNode != nil {
		expression = expressionNode.Accept(v).(ast.Expression)
	}

	// TODO: get end position from expression
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.ReturnStatement{
		Expression: expression,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	isConstant := ctx.Let() != nil
	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()
	expressionResult := ctx.Expression().Accept(v)
	if expressionResult == nil {
		return nil
	}
	expression := expressionResult.(ast.Expression)
	var fullType ast.Type

	fullTypeContext := ctx.FullType()
	if fullTypeContext != nil {
		if x, ok := fullTypeContext.Accept(v).(ast.Type); ok {
			fullType = x
		}
	}

	// TODO: get end position from expression
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)
	identifierPosition := ast.PositionFromToken(identifierNode.GetSymbol())

	return &ast.VariableDeclaration{
		IsConstant:    isConstant,
		Identifier:    identifier,
		Value:         expression,
		Type:          fullType,
		StartPos:      startPosition,
		EndPos:        endPosition,
		IdentifierPos: identifierPosition,
	}
}

func (v *ProgramVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	test := ctx.test.Accept(v).(ast.Expression)
	then := ctx.then.Accept(v).(*ast.Block)

	var elseBlock *ast.Block
	if ctx.alt != nil {
		elseBlock = ctx.alt.Accept(v).(*ast.Block)
	} else {
		ifStatementContext := ctx.IfStatement()
		if ifStatementContext != nil {
			if ifStatement, ok := ifStatementContext.Accept(v).(*ast.IfStatement); ok {
				elseBlock = &ast.Block{
					Statements: []ast.Statement{ifStatement},
					StartPos:   ifStatement.StartPos,
					EndPos:     ifStatement.EndPos,
				}
			}
		}
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.IfStatement{
		Test:     test,
		Then:     then,
		Else:     elseBlock,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	test := ctx.Expression().Accept(v).(ast.Expression)
	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.WhileStatement{
		Test:     test,
		Block:    block,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()
	identifierSymbol := identifierNode.GetSymbol()

	targetStartPosition := ast.PositionFromToken(identifierSymbol)
	targetEndPosition := ast.EndPosition(targetStartPosition, identifierSymbol.GetStop())

	var target ast.Expression = &ast.IdentifierExpression{
		Identifier: identifier,
		StartPos:   targetStartPosition,
		EndPos:     targetEndPosition,
	}

	value := ctx.Expression().Accept(v).(ast.Expression)

	for _, accessExpressionContext := range ctx.AllExpressionAccess() {
		expression := accessExpressionContext.Accept(v)
		accessExpression := expression.(ast.AccessExpression)
		target = v.wrapPartialAccessExpression(target, accessExpression)
	}

	// TODO: get end position from expression
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.AssignmentStatement{
		Target:   target,
		Value:    value,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	element := ctx.OrExpression().Accept(v)
	if element == nil {
		return nil
	}
	expression := element.(ast.Expression)

	if ctx.then != nil && ctx.alt != nil {
		then := ctx.then.Accept(v).(ast.Expression)
		alt := ctx.alt.Accept(v).(ast.Expression)
		startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

		return &ast.ConditionalExpression{
			Test:     expression,
			Then:     then,
			Else:     alt,
			StartPos: startPosition,
			EndPos:   endPosition,
		}
	}

	return expression
}

func (v *ProgramVisitor) VisitOrExpression(ctx *OrExpressionContext) interface{} {
	right := ctx.AndExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.OrExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: ast.OperationOr,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	right := ctx.EqualityExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.AndExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: ast.OperationAnd,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	right := ctx.RelationalExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.EqualityExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.EqualityOp().Accept(v).(ast.Operation)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	right := ctx.AdditiveExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.RelationalExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.RelationalOp().Accept(v).(ast.Operation)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	right := ctx.MultiplicativeExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.AdditiveExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.AdditiveOp().Accept(v).(ast.Operation)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	right := ctx.UnaryExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.MultiplicativeExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.MultiplicativeOp().Accept(v).(ast.Operation)

	startPosition := leftExpression.StartPosition()
	endPosition := rightExpression.EndPosition()

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	unaryContext := ctx.UnaryExpression()
	if unaryContext == nil {
		primaryContext := ctx.PrimaryExpression()
		if primaryContext == nil {
			return nil
		}
		return primaryContext.Accept(v)
	}

	// ensure unary operators are not juxtaposed
	if ctx.GetChildCount() > 2 {
		position := ast.PositionFromToken(ctx.UnaryOp(0).GetStart())
		panic(&JuxtaposedUnaryOperatorsError{
			Pos: position,
		})
	}

	expression := unaryContext.Accept(v).(ast.Expression)
	operation := ctx.UnaryOp(0).Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.UnaryExpression{
		Operation:  operation,
		Expression: expression,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitUnaryOp(ctx *UnaryOpContext) interface{} {

	if ctx.Negate() != nil {
		return ast.OperationNegate
	}

	if ctx.Minus() != nil {
		return ast.OperationMinus
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	result := ctx.PrimaryExpressionStart().Accept(v).(ast.Expression)

	for _, suffix := range ctx.AllPrimaryExpressionSuffix() {
		switch partialExpression := suffix.Accept(v).(type) {
		case *ast.InvocationExpression:
			result = &ast.InvocationExpression{
				Expression: result,
				Arguments:  partialExpression.Arguments,
				StartPos:   partialExpression.StartPos,
				EndPos:     partialExpression.EndPos,
			}
		case ast.AccessExpression:
			result = v.wrapPartialAccessExpression(result, partialExpression)
		default:
			panic(&errors.UnreachableError{})
		}
	}

	return result
}

func (v *ProgramVisitor) wrapPartialAccessExpression(
	wrapped ast.Expression,
	partialAccessExpression ast.AccessExpression,
) ast.Expression {

	switch partialAccessExpression := partialAccessExpression.(type) {
	case *ast.IndexExpression:
		return &ast.IndexExpression{
			Expression: wrapped,
			Index:      partialAccessExpression.Index,
			StartPos:   partialAccessExpression.StartPos,
			EndPos:     partialAccessExpression.EndPos,
		}
	case *ast.MemberExpression:
		return &ast.MemberExpression{
			Expression: wrapped,
			Identifier: partialAccessExpression.Identifier,
			StartPos:   partialAccessExpression.StartPos,
			EndPos:     partialAccessExpression.EndPos,
		}
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitExpressionAccess(ctx *ExpressionAccessContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	identifier := ctx.Identifier().GetText()
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	// NOTE: partial, expression is filled later
	return &ast.MemberExpression{
		Identifier: identifier,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	index := ctx.Expression().Accept(v).(ast.Expression)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	// NOTE: partial, expression is filled later
	return &ast.IndexExpression{
		Index:    index,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return ctx.Literal().Accept(v)
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func parseIntExpression(token antlr.Token, text string, kind IntegerLiteralKind) *ast.IntExpression {
	startPosition := ast.PositionFromToken(token)
	endPosition := ast.EndPosition(startPosition, token.GetStop())

	// check literal has no leading underscore
	if strings.HasPrefix(text, "_") {
		panic(&InvalidIntegerLiteralError{
			IntegerLiteralKind:        kind,
			InvalidIntegerLiteralKind: InvalidIntegerLiteralKindLeadingUnderscore,
			// NOTE: not using text, because it has the base-prefix stripped
			Literal:  token.GetText(),
			StartPos: startPosition,
			EndPos:   endPosition,
		})
	}

	// check literal has no trailing underscore
	if strings.HasSuffix(text, "_") {
		panic(&InvalidIntegerLiteralError{
			IntegerLiteralKind:        kind,
			InvalidIntegerLiteralKind: InvalidIntegerLiteralKindTrailingUnderscore,
			// NOTE: not using text, because it has the base-prefix stripped
			Literal:  token.GetText(),
			StartPos: startPosition,
			EndPos:   endPosition,
		})
	}

	withoutUnderscores := strings.Replace(text, "_", "", -1)

	value, ok := big.NewInt(0).SetString(withoutUnderscores, kind.Base())
	if !ok {
		panic(fmt.Sprintf("invalid %s literal: %s", kind, text))
	}
	return &ast.IntExpression{
		Value: value,
		Pos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{} {
	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.GetStop().GetStop())

	panic(&InvalidIntegerLiteralError{
		IntegerLiteralKind:        IntegerLiteralKindUnknown,
		InvalidIntegerLiteralKind: InvalidIntegerLiteralKindUnknownPrefix,
		Literal:                   ctx.GetText(),
		StartPos:                  startPosition,
		EndPos:                    endPosition,
	})
}

func (v *ProgramVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText(),
		IntegerLiteralKindDecimal,
	)
}

func (v *ProgramVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindBinary,
	)
}

func (v *ProgramVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindOctal,
	)
}

func (v *ProgramVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindHexadecimal,
	)
}

func (v *ProgramVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return ctx.Expression().Accept(v)
}

func (v *ProgramVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	position := ast.PositionFromToken(ctx.GetStart())

	if ctx.True() != nil {
		return &ast.BoolExpression{
			Value: true,
			Pos:   position,
		}
	}

	if ctx.False() != nil {
		return &ast.BoolExpression{
			Value: false,
			Pos:   position,
		}
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	var expressions []ast.Expression
	for _, expression := range ctx.AllExpression() {
		expressions = append(
			expressions,
			expression.Accept(v).(ast.Expression),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return &ast.ArrayExpression{
		Values:   expressions,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	identifierNode := ctx.Identifier()

	identifier := identifierNode.GetText()
	identifierSymbol := identifierNode.GetSymbol()
	startPosition := ast.PositionFromToken(identifierSymbol)
	endPosition := ast.EndPosition(startPosition, identifierSymbol.GetStop())

	return &ast.IdentifierExpression{
		Identifier: identifier,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitInvocation(ctx *InvocationContext) interface{} {
	var arguments []*ast.Argument
	for _, argument := range ctx.AllArgument() {
		arguments = append(
			arguments,
			argument.Accept(v).(*ast.Argument),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	// NOTE: partial, argument is filled later
	return &ast.InvocationExpression{
		Arguments: arguments,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitArgument(ctx *ArgumentContext) interface{} {
	identifierNode := ctx.Identifier()
	label := ""
	if identifierNode != nil {
		label = identifierNode.GetText()
	}
	expression := ctx.Expression().Accept(v).(ast.Expression)
	return &ast.Argument{
		Label:      label,
		Expression: expression,
	}
}

func (v *ProgramVisitor) VisitEqualityOp(ctx *EqualityOpContext) interface{} {
	if ctx.Equal() != nil {
		return ast.OperationEqual
	}

	if ctx.Unequal() != nil {
		return ast.OperationUnequal
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitRelationalOp(ctx *RelationalOpContext) interface{} {
	if ctx.Less() != nil {
		return ast.OperationLess
	}

	if ctx.Greater() != nil {
		return ast.OperationGreater
	}

	if ctx.LessEqual() != nil {
		return ast.OperationLessEqual
	}

	if ctx.GreaterEqual() != nil {
		return ast.OperationGreaterEqual
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitAdditiveOp(ctx *AdditiveOpContext) interface{} {
	if ctx.Plus() != nil {
		return ast.OperationPlus
	}

	if ctx.Minus() != nil {
		return ast.OperationMinus
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{} {
	if ctx.Mul() != nil {
		return ast.OperationMul
	}

	if ctx.Div() != nil {
		return ast.OperationDiv
	}

	if ctx.Mod() != nil {
		return ast.OperationMod
	}

	panic(&errors.UnreachableError{})
}

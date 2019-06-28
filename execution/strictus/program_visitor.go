package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
	"bamboo-runtime/execution/strictus/errors"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"math/big"
	"strconv"
)

type ProgramVisitor struct {
	*BaseStrictusVisitor
}

func (v *ProgramVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	var allDeclarations []ast.Declaration

	for _, declarationContext := range ctx.AllDeclaration() {
		declaration := declarationContext.Accept(v).(ast.Declaration)
		allDeclarations = append(allDeclarations, declaration)
	}

	return ast.Program{
		Declarations: allDeclarations,
	}
}

func (v *ProgramVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	isPublic := ctx.Pub() != nil
	identifier := ctx.Identifier().GetText()
	closeParen := ctx.CloseParen().GetSymbol()
	returnType := v.visitReturnType(ctx.returnType, closeParen)
	var parameters []ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]ast.Parameter)
	}
	block := ctx.Block().Accept(v).(ast.Block)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)
	identifierPosition := ast.PositionFromToken(ctx.Identifier().GetSymbol())

	return ast.FunctionDeclaration{
		IsPublic:           isPublic,
		Identifier:         identifier,
		Parameters:         parameters,
		ReturnType:         returnType,
		Block:              block,
		StartPosition:      startPosition,
		EndPosition:        endPosition,
		IdentifierPosition: identifierPosition,
	}
}

// visitReturnType returns the return type.
// if none was given in the program, return an empty type with the position of tokenBefore
func (v *ProgramVisitor) visitReturnType(ctx ITypeNameContext, tokenBefore antlr.Token) ast.Type {
	if ctx == nil {
		positionBeforeMissingReturnType := ast.PositionFromToken(tokenBefore)
		return ast.BaseType{
			Position: positionBeforeMissingReturnType,
		}
	}
	return ctx.Accept(v).(ast.Type)
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	closeParen := ctx.CloseParen().GetSymbol()
	returnType := v.visitReturnType(ctx.returnType, closeParen)
	var parameters []ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]ast.Parameter)
	}
	block := ctx.Block().Accept(v).(ast.Block)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.FunctionExpression{
		Parameters:    parameters,
		ReturnType:    returnType,
		Block:         block,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	var parameters []ast.Parameter

	for _, parameter := range ctx.AllParameter() {
		parameters = append(
			parameters,
			parameter.Accept(v).(ast.Parameter),
		)
	}

	return parameters
}

func (v *ProgramVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	identifier := ctx.Identifier().GetText()
	typeName := ctx.TypeName().Accept(v).(ast.Type)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.Parameter{
		Identifier:    identifier,
		Type:          typeName,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	identifierNode := ctx.Identifier()
	// identifier?
	if identifierNode != nil {
		identifier := identifierNode.GetText()
		position := ast.PositionFromToken(identifierNode.GetSymbol())
		return ast.BaseType{
			Identifier: identifier,
			Position:   position,
		}
	}

	// alternative: function type
	var parameterTypes []ast.Type
	for _, typeName := range ctx.parameterTypes {
		parameterTypes = append(
			parameterTypes,
			typeName.Accept(v).(ast.Type),
		)
	}

	returnType := ctx.returnType.Accept(v).(ast.Type)

	startPosition := ast.PositionFromToken(ctx.OpenParen().GetSymbol())
	endPosition := returnType.GetEndPosition()

	return ast.FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     returnType,
		StartPosition:  startPosition,
		EndPosition:    endPosition,
	}
}

func (v *ProgramVisitor) VisitTypeName(ctx *TypeNameContext) interface{} {
	result := ctx.BaseType().Accept(v).(ast.Type)

	// reduce in reverse
	dimensions := ctx.AllTypeDimension()
	lastDimensionIndex := len(dimensions) - 1
	for i := range dimensions {
		dimensionContext := dimensions[lastDimensionIndex-i]
		dimension := dimensionContext.Accept(v).(*int)
		startPosition, endPosition := ast.PositionRangeFromContext(dimensionContext)
		if dimension == nil {
			result = ast.VariableSizedType{
				Type:          result,
				StartPosition: startPosition,
				EndPosition:   endPosition,
			}
		} else {
			result = ast.ConstantSizedType{
				Type:          result,
				Size:          *dimension,
				StartPosition: startPosition,
				EndPosition:   endPosition,
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
	var statements []ast.Statement
	for _, statement := range ctx.AllStatement() {
		statements = append(
			statements,
			statement.Accept(v).(ast.Statement),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.Block{
		Statements:    statements,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
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
		return ast.ExpressionStatement{
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

	return ast.ReturnStatement{
		Expression:    expression,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	isConst := ctx.Const() != nil
	identifier := ctx.Identifier().GetText()
	expression := ctx.Expression().Accept(v).(ast.Expression)
	var typeName ast.Type

	typeNameContext := ctx.TypeName()
	if typeNameContext != nil {
		if x, ok := typeNameContext.Accept(v).(ast.Type); ok {
			typeName = x
		}
	}

	// TODO: get end position from expression
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)
	identifierPosition := ast.PositionFromToken(ctx.Identifier().GetSymbol())

	return ast.VariableDeclaration{
		IsConst:            isConst,
		Identifier:         identifier,
		Value:              expression,
		Type:               typeName,
		StartPosition:      startPosition,
		EndPosition:        endPosition,
		IdentifierPosition: identifierPosition,
	}
}

func (v *ProgramVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	test := ctx.test.Accept(v).(ast.Expression)
	then := ctx.then.Accept(v).(ast.Block)

	var elseBlock ast.Block
	if ctx.alt != nil {
		elseBlock = ctx.alt.Accept(v).(ast.Block)
	} else {
		ifStatementContext := ctx.IfStatement()
		if ifStatementContext != nil {
			if ifStatement, ok := ifStatementContext.Accept(v).(ast.IfStatement); ok {
				elseBlock = ast.Block{
					Statements:    []ast.Statement{ifStatement},
					StartPosition: ifStatement.StartPosition,
					EndPosition:   ifStatement.EndPosition,
				}
			}
		}
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.IfStatement{
		Test:          test,
		Then:          then,
		Else:          elseBlock,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	test := ctx.Expression().Accept(v).(ast.Expression)
	block := ctx.Block().Accept(v).(ast.Block)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.WhileStatement{
		Test:          test,
		Block:         block,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	identifier := ctx.Identifier().GetText()
	value := ctx.Expression().Accept(v).(ast.Expression)

	targetPosition := ast.PositionFromToken(ctx.Identifier().GetSymbol())

	var target ast.Expression = ast.IdentifierExpression{
		Identifier: identifier,
		Position:   targetPosition,
	}

	for _, accessExpressionContext := range ctx.AllExpressionAccess() {
		expression := accessExpressionContext.Accept(v)
		accessExpression := expression.(ast.AccessExpression)
		target = v.wrapPartialAccessExpression(target, accessExpression)
	}

	// TODO: get end position from expression
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.AssignmentStatement{
		Target:        target,
		Value:         value,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	expression := ctx.OrExpression().Accept(v).(ast.Expression)

	if ctx.then != nil && ctx.alt != nil {
		then := ctx.then.Accept(v).(ast.Expression)
		alt := ctx.alt.Accept(v).(ast.Expression)
		startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

		return ast.ConditionalExpression{
			Test:          expression,
			Then:          then,
			Else:          alt,
			StartPosition: startPosition,
			EndPosition:   endPosition,
		}
	}

	return expression
}

func (v *ProgramVisitor) VisitOrExpression(ctx *OrExpressionContext) interface{} {
	right := ctx.AndExpression().Accept(v).(ast.Expression)
	leftContext := ctx.OrExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     ast.OperationOr,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	right := ctx.EqualityExpression().Accept(v).(ast.Expression)
	leftContext := ctx.AndExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     ast.OperationAnd,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	right := ctx.RelationalExpression().Accept(v).(ast.Expression)

	leftContext := ctx.EqualityExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	operation := ctx.EqualityOp().Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     operation,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	right := ctx.AdditiveExpression().Accept(v).(ast.Expression)

	leftContext := ctx.RelationalExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	operation := ctx.RelationalOp().Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     operation,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	right := ctx.MultiplicativeExpression().Accept(v).(ast.Expression)

	leftContext := ctx.AdditiveExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	operation := ctx.AdditiveOp().Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     operation,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	right := ctx.UnaryExpression().Accept(v).(ast.Expression)

	leftContext := ctx.MultiplicativeExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	operation := ctx.MultiplicativeOp().Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.BinaryExpression{
		Operation:     operation,
		Left:          left,
		Right:         right,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	unaryContext := ctx.UnaryExpression()
	if unaryContext == nil {
		return ctx.PrimaryExpression().Accept(v)
	}

	// ensure unary operators are not juxtaposed
	if ctx.GetChildCount() > 2 {
		position := ast.PositionFromToken(ctx.UnaryOp(0).GetStart())
		panic(&JuxtaposedUnaryOperatorsError{
			Position: position,
		})
	}

	expression := unaryContext.Accept(v).(ast.Expression)
	operation := ctx.UnaryOp(0).Accept(v).(ast.Operation)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	return ast.UnaryExpression{
		Operation:     operation,
		Expression:    expression,
		StartPosition: startPosition,
		EndPosition:   endPosition,
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
		case ast.InvocationExpression:
			result = ast.InvocationExpression{
				Expression:    result,
				Arguments:     partialExpression.Arguments,
				StartPosition: partialExpression.StartPosition,
				EndPosition:   partialExpression.EndPosition,
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
	case ast.IndexExpression:
		return ast.IndexExpression{
			Expression:    wrapped,
			Index:         partialAccessExpression.Index,
			StartPosition: partialAccessExpression.StartPosition,
			EndPosition:   partialAccessExpression.EndPosition,
		}
	case ast.MemberExpression:
		return ast.MemberExpression{
			Expression:    wrapped,
			Identifier:    partialAccessExpression.Identifier,
			StartPosition: partialAccessExpression.StartPosition,
			EndPosition:   partialAccessExpression.EndPosition,
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
	return ast.MemberExpression{
		Identifier:    identifier,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	index := ctx.Expression().Accept(v).(ast.Expression)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	// NOTE: partial, expression is filled later
	return ast.IndexExpression{
		Index:         index,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return ctx.Literal().Accept(v)
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func parseIntExpression(token antlr.Token, text string, kind string, base int) ast.IntExpression {
	value, ok := big.NewInt(0).SetString(text, base)
	if !ok {
		panic(fmt.Sprintf("invalid %s literal: %s", kind, text))
	}
	return ast.IntExpression{
		Value:    value,
		Position: ast.PositionFromToken(token),
	}
}

func (v *ProgramVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText(),
		"decimal",
		10,
	)
}

func (v *ProgramVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		"binary",
		2,
	)
}

func (v *ProgramVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		"octal",
		8,
	)
}

func (v *ProgramVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		"hexadecimal",
		16,
	)
}

func (v *ProgramVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return ctx.Expression().Accept(v)
}

func (v *ProgramVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	position := ast.PositionFromToken(ctx.GetStart())

	if ctx.True() != nil {
		return ast.BoolExpression{
			Value:    true,
			Position: position,
		}
	}

	if ctx.False() != nil {
		return ast.BoolExpression{
			Value:    false,
			Position: position,
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

	return ast.ArrayExpression{
		Values:        expressions,
		StartPosition: startPosition,
		EndPosition:   endPosition,
	}
}

func (v *ProgramVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	identifierNode := ctx.Identifier()

	identifier := identifierNode.GetText()
	position := ast.PositionFromToken(identifierNode.GetSymbol())

	return ast.IdentifierExpression{
		Identifier: identifier,
		Position:   position,
	}
}

func (v *ProgramVisitor) VisitInvocation(ctx *InvocationContext) interface{} {
	var expressions []ast.Expression
	for _, expression := range ctx.AllExpression() {
		expressions = append(
			expressions,
			expression.Accept(v).(ast.Expression),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx.BaseParserRuleContext)

	// NOTE: partial, expression is filled later
	return ast.InvocationExpression{
		Arguments:     expressions,
		StartPosition: startPosition,
		EndPosition:   endPosition,
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

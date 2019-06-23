package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"strconv"
)

type ProgramVisitor struct {
	*BaseStrictusVisitor
}

func (v *ProgramVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	declarations := map[string]ast.Declaration{}
	var allDeclarations []ast.Declaration

	for _, declarationContext := range ctx.AllDeclaration() {
		declaration := declarationContext.Accept(v).(ast.Declaration)
		name := declaration.DeclarationName()
		if _, exists := declarations[name]; exists {
			panic(fmt.Sprintf("can't redeclare %s", name))
		}
		declarations[name] = declaration
		allDeclarations = append(allDeclarations, declaration)
	}

	return ast.Program{
		Declarations:    declarations,
		AllDeclarations: allDeclarations,
	}
}

func (v *ProgramVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	isPublic := ctx.Pub() != nil
	identifier := ctx.Identifier().GetText()
	returnType := v.visitReturnType(ctx.returnType)
	var parameters []ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]ast.Parameter)
	}
	block := ctx.Block().Accept(v).(ast.Block)

	return ast.FunctionDeclaration{
		IsPublic:   isPublic,
		Identifier: identifier,
		Parameters: parameters,
		ReturnType: returnType,
		Block:      block,
	}
}

func (v *ProgramVisitor) visitReturnType(ctx ITypeNameContext) ast.Type {
	if ctx == nil {
		return ast.VoidType{}
	}
	return ctx.Accept(v).(ast.Type)
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	returnType := v.visitReturnType(ctx.returnType)
	var parameters []ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]ast.Parameter)
	}
	block := ctx.Block().Accept(v).(ast.Block)

	return ast.FunctionExpression{
		Parameters: parameters,
		ReturnType: returnType,
		Block:      block,
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
	return ast.Parameter{
		Identifier: identifier,
		Type:       typeName,
	}
}

func (v *ProgramVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	identifierNode := ctx.Identifier()
	// identifier?
	if identifierNode != nil {
		identifier := identifierNode.GetText()
		switch identifier {
		case "Int8":
			return ast.Int8Type{}
		case "Int16":
			return ast.Int16Type{}
		case "Int32":
			return ast.Int32Type{}
		case "Int64":
			return ast.Int64Type{}
		case "UInt8":
			return ast.UInt8Type{}
		case "UInt16":
			return ast.UInt16Type{}
		case "UInt32":
			return ast.UInt32Type{}
		case "UInt64":
			return ast.UInt64Type{}
		case "Void":
			return ast.VoidType{}
		case "Bool":
			return ast.BoolType{}
		default:
			panic(fmt.Sprintf("unknown type: %s", identifier))
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

	return ast.FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     returnType,
	}
}

func (v *ProgramVisitor) VisitTypeName(ctx *TypeNameContext) interface{} {
	result := ctx.BaseType().Accept(v).(ast.Type)

	// reduce in reverse
	dimensions := ctx.AllTypeDimension()
	lastDimensionIndex := len(dimensions) - 1
	for i := range dimensions {
		dimension := dimensions[lastDimensionIndex-i].Accept(v).(*int)
		if dimension == nil {
			result = ast.VariableSizedType{
				Type: result,
			}
		} else {
			result = ast.ConstantSizedType{
				Type: result,
				Size: *dimension,
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

	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.PositionFromToken(ctx.GetStop())

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

	return ast.ReturnStatement{
		Expression: expression,
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

	return ast.VariableDeclaration{
		IsConst:    isConst,
		Identifier: identifier,
		Value:      expression,
		Type:       typeName,
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

	return ast.IfStatement{
		Test: test,
		Then: then,
		Else: elseBlock,
	}
}

func (v *ProgramVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	test := ctx.Expression().Accept(v).(ast.Expression)
	block := ctx.Block().Accept(v).(ast.Block)

	return ast.WhileStatement{
		Test:  test,
		Block: block,
	}
}

func (v *ProgramVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	identifier := ctx.Identifier().GetText()
	value := ctx.Expression().Accept(v).(ast.Expression)

	var target ast.Expression = ast.IdentifierExpression{Identifier: identifier}

	for _, accessExpressionContext := range ctx.AllExpressionAccess() {
		expression := accessExpressionContext.Accept(v)
		accessExpression, ok := expression.(ast.AccessExpression)
		if !ok {
			panic(fmt.Sprintf("assignment to unknown access expression: %#+v", expression))
		}
		target = v.wrapPartialAccessExpression(target, accessExpression)
	}

	return ast.Assignment{
		Target: target,
		Value:  value,
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
		return ast.ConditionalExpression{
			Test: expression,
			Then: then,
			Else: alt,
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
	return ast.BinaryExpression{
		Operation: ast.OperationOr,
		Left:      left,
		Right:     right,
	}
}

func (v *ProgramVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	right := ctx.EqualityExpression().Accept(v).(ast.Expression)
	leftContext := ctx.AndExpression()
	if leftContext == nil {
		return right
	}

	left := leftContext.Accept(v).(ast.Expression)
	return ast.BinaryExpression{
		Operation: ast.OperationAnd,
		Left:      left,
		Right:     right,
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

	return ast.BinaryExpression{
		Operation: operation,
		Left:      left,
		Right:     right,
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

	return ast.BinaryExpression{
		Operation: operation,
		Left:      left,
		Right:     right,
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

	return ast.BinaryExpression{
		Operation: operation,
		Left:      left,
		Right:     right,
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

	return ast.BinaryExpression{
		Operation: operation,
		Left:      left,
		Right:     right,
	}
}
func (v *ProgramVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	unaryContext := ctx.UnaryExpression()
	if unaryContext == nil {
		return ctx.PrimaryExpression().Accept(v)
	}

	if ctx.GetChildCount() > 2 {
		panic(fmt.Sprintf("unary operators must not be juxtaposed; parenthesize inner expression"))
	}

	expression := unaryContext.Accept(v).(ast.Expression)
	operation := ctx.UnaryOp(0).Accept(v).(ast.Operation)

	return ast.UnaryExpression{
		Operation:  operation,
		Expression: expression,
	}
}

func (v *ProgramVisitor) VisitUnaryOp(ctx *UnaryOpContext) interface{} {

	if ctx.Negate() != nil {
		return ast.OperationNegate
	}

	if ctx.Minus() != nil {
		return ast.OperationMinus
	}

	panic("unreachable")
}

func (v *ProgramVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	result := ctx.PrimaryExpressionStart().Accept(v).(ast.Expression)

	for _, suffix := range ctx.AllPrimaryExpressionSuffix() {
		switch partialExpression := suffix.Accept(v).(type) {
		case ast.InvocationExpression:
			result = ast.InvocationExpression{
				Expression: result,
				Arguments:  partialExpression.Arguments,
			}
		case ast.AccessExpression:
			result = v.wrapPartialAccessExpression(result, partialExpression)
		default:
			panic(fmt.Sprintf("unknown primary expression suffix: %#+v", suffix))
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
			Expression: wrapped,
			Index:      partialAccessExpression.Index,
		}
	case ast.MemberExpression:
		return ast.MemberExpression{
			Expression: wrapped,
			Identifier: partialAccessExpression.Identifier,
		}
	}
	panic(fmt.Sprintf("invalid primary expression suffix: %#+v", partialAccessExpression))
}

func (v *ProgramVisitor) VisitPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitExpressionAccess(ctx *ExpressionAccessContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	access := ctx.Identifier().GetText()
	// NOTE: partial, expression is filled later
	return ast.MemberExpression{Identifier: access}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	access := ctx.Expression().Accept(v).(ast.Expression)
	// NOTE: partial, expression is filled later
	return ast.IndexExpression{Index: access}
}

func (v *ProgramVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return ctx.Literal().Accept(v)
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func parseIntExpression(text string, kind string, base int) ast.Int64Expression {
	value, err := strconv.ParseInt(text, base, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid %s literal: %s", kind, text))
	}
	return ast.Int64Expression(value)
}

func (v *ProgramVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return parseIntExpression(ctx.GetText(), "decimal", 10)
}

func (v *ProgramVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return parseIntExpression(ctx.GetText()[2:], "binary", 2)
}

func (v *ProgramVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return parseIntExpression(ctx.GetText()[2:], "octal", 8)
}

func (v *ProgramVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return parseIntExpression(ctx.GetText()[2:], "hexadecimal", 16)
}

func (v *ProgramVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return ctx.Expression().Accept(v)
}

func (v *ProgramVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	text := ctx.GetText()
	value, err := strconv.ParseBool(text)
	if err != nil {
		panic(fmt.Sprintf("invalid boolean literal: %s", text))
	}

	return ast.BoolExpression{
		Value:    value,
		Position: ast.PositionFromToken(ctx.GetStart()),
	}
}

func (v *ProgramVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	var expressions []ast.Expression
	for _, expression := range ctx.AllExpression() {
		expressions = append(
			expressions,
			expression.Accept(v).(ast.Expression),
		)
	}

	return ast.ArrayExpression{
		Values: expressions,
	}
}

func (v *ProgramVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	identifier := ctx.Identifier().GetText()
	return ast.IdentifierExpression{
		Identifier: identifier,
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

	// NOTE: partial, expression is filled later
	return ast.InvocationExpression{
		Arguments: expressions,
	}
}

func (v *ProgramVisitor) VisitEqualityOp(ctx *EqualityOpContext) interface{} {
	if ctx.Equal() != nil {
		return ast.OperationEqual
	}

	if ctx.Unequal() != nil {
		return ast.OperationUnequal
	}

	panic("unreachable")
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

	panic("unreachable")
}

func (v *ProgramVisitor) VisitAdditiveOp(ctx *AdditiveOpContext) interface{} {
	if ctx.Plus() != nil {
		return ast.OperationPlus
	}

	if ctx.Minus() != nil {
		return ast.OperationMinus
	}

	panic("unreachable")
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

	panic("unreachable")
}

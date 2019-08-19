package parser

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
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
	access := ctx.Access().Accept(v).(ast.Access)

	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()

	parameterListEnd := ctx.ParameterList().GetStop()
	returnType := v.visitReturnType(ctx.returnType, parameterListEnd)

	var parameters []*ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]*ast.Parameter)
	}

	// NOTE: in e.g interface declarations, function blocks are optional

	var functionBlock *ast.FunctionBlock
	functionBlockContext := ctx.FunctionBlock()
	if functionBlockContext != nil {
		functionBlock = functionBlockContext.Accept(v).(*ast.FunctionBlock)
	}

	startPosition := ast.PositionFromToken(ctx.GetStart())
	identifierPos := ast.PositionFromToken(identifierNode.GetSymbol())

	return &ast.FunctionDeclaration{
		Access:        access,
		Identifier:    identifier,
		Parameters:    parameters,
		ReturnType:    returnType,
		FunctionBlock: functionBlock,
		StartPos:      startPosition,
		IdentifierPos: identifierPos,
	}
}

// visitReturnType returns the return type.
// if none was given in the program, return an empty type with the position of tokenBefore
func (v *ProgramVisitor) visitReturnType(ctx IFullTypeContext, tokenBefore antlr.Token) ast.Type {
	if ctx == nil {
		positionBeforeMissingReturnType :=
			ast.PositionFromToken(tokenBefore)
		return &ast.NominalType{
			Pos: positionBeforeMissingReturnType,
		}
	}
	result := ctx.Accept(v)
	if result == nil {
		return nil
	}
	return result.(ast.Type)
}

func (v *ProgramVisitor) VisitAccess(ctx *AccessContext) interface{} {
	if ctx.Pub() != nil {
		return ast.AccessPublic
	}

	if ctx.PubSet() != nil {
		return ast.AccessPublicSettable
	}

	return ast.AccessNotSpecified
}

func (v *ProgramVisitor) VisitStructureDeclaration(ctx *StructureDeclarationContext) interface{} {
	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()

	var fields []*ast.FieldDeclaration
	for _, fieldCtx := range ctx.AllField() {
		field := fieldCtx.Accept(v).(*ast.FieldDeclaration)
		fields = append(fields, field)
	}

	var initializer *ast.InitializerDeclaration
	initializerNode := ctx.Initializer()
	if initializerNode != nil {
		initializer = initializerNode.Accept(v).(*ast.InitializerDeclaration)
	}

	var functions []*ast.FunctionDeclaration
	for _, functionDeclarationCtx := range ctx.AllFunctionDeclaration() {
		functionDeclaration :=
			functionDeclarationCtx.Accept(v).(*ast.FunctionDeclaration)
		functions = append(functions, functionDeclaration)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)
	identifierPos := ast.PositionFromToken(identifierNode.GetSymbol())

	return &ast.StructureDeclaration{
		Identifier:    identifier,
		Fields:        fields,
		Initializer:   initializer,
		Functions:     functions,
		IdentifierPos: identifierPos,
		StartPos:      startPosition,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitField(ctx *FieldContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	variableKindContext := ctx.VariableKind()
	variableKind := ast.VariableKindNotSpecified
	if variableKindContext != nil {
		variableKind = variableKindContext.Accept(v).(ast.VariableKind)
	}

	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()
	identifierPos := ast.PositionFromToken(identifierNode.GetSymbol())

	typeContext := ctx.FullType()
	fullType := typeContext.Accept(v).(ast.Type)

	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, typeContext.GetStop().GetStop())

	return &ast.FieldDeclaration{
		Access:        access,
		VariableKind:  variableKind,
		Identifier:    identifier,
		Type:          fullType,
		StartPos:      startPosition,
		EndPos:        endPosition,
		IdentifierPos: identifierPos,
	}
}

func (v *ProgramVisitor) VisitInitializer(ctx *InitializerContext) interface{} {
	identifier := ctx.Identifier().GetText()

	var parameters []*ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]*ast.Parameter)
	}

	functionBlock := ctx.FunctionBlock().Accept(v).(*ast.FunctionBlock)

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.InitializerDeclaration{
		Identifier:    identifier,
		Parameters:    parameters,
		FunctionBlock: functionBlock,
		StartPos:      startPosition,
	}
}

func (v *ProgramVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()

	var fields []*ast.FieldDeclaration
	for _, fieldCtx := range ctx.AllField() {
		field := fieldCtx.Accept(v).(*ast.FieldDeclaration)
		fields = append(fields, field)
	}

	var functions []*ast.FunctionDeclaration
	for _, functionDeclarationCtx := range ctx.AllFunctionDeclaration() {
		functionDeclaration :=
			functionDeclarationCtx.Accept(v).(*ast.FunctionDeclaration)
		functions = append(functions, functionDeclaration)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)
	identifierPos := ast.PositionFromToken(identifierNode.GetSymbol())

	return &ast.InterfaceDeclaration{
		Identifier:    identifier,
		Fields:        fields,
		Functions:     functions,
		IdentifierPos: identifierPos,
		StartPos:      startPosition,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	parameterListEnd := ctx.ParameterList().GetStop()
	returnType := v.visitReturnType(ctx.returnType, parameterListEnd)

	var parameters []*ast.Parameter
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).([]*ast.Parameter)
	}

	functionBlock := ctx.FunctionBlock().Accept(v).(*ast.FunctionBlock)

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.FunctionExpression{
		Parameters:    parameters,
		ReturnType:    returnType,
		FunctionBlock: functionBlock,
		StartPos:      startPosition,
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
		position := ast.PositionFromToken(ctx.argumentLabel)
		labelPos = &position
	}

	// identifier
	identifier := ctx.parameterName.GetText()
	identifierPos := ast.PositionFromToken(ctx.parameterName)

	fullType := ctx.FullType().Accept(v).(ast.Type)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

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
		return &ast.NominalType{
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

	for _, optional := range ctx.optionals {
		switch optional.GetTokenType() {
		case StrictusLexerOptional:
			endPos := ast.PositionFromToken(optional)
			result = &ast.OptionalType{
				Type:   result,
				EndPos: endPos,
			}
			continue
		case StrictusLexerNilCoalescing:
			endPosInner := ast.PositionFromToken(optional)
			endPosOuter := endPosInner.Shifted(1)
			result = &ast.OptionalType{
				Type: &ast.OptionalType{
					Type:   result,
					EndPos: endPosInner,
				},
				EndPos: endPosOuter,
			}
			continue
		}
	}

	return result
}

func (v *ProgramVisitor) VisitTypeDimension(ctx *TypeDimensionContext) interface{} {
	var result *int

	literalNode := ctx.DecimalLiteral()
	if literalNode == nil {
		return result
	}

	value, err := strconv.Atoi(literalNode.GetText())
	if err != nil {
		return result
	}

	result = &value

	return result
}

func (v *ProgramVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.visitBlock(ctx.BaseParserRuleContext, ctx.Statements())
}

func (v *ProgramVisitor) VisitFunctionBlock(ctx *FunctionBlockContext) interface{} {
	block := v.visitBlock(ctx.BaseParserRuleContext, ctx.Statements())

	var preConditions []*ast.Condition
	preConditionsCtx := ctx.PreConditions()
	if preConditionsCtx != nil {
		preConditions = preConditionsCtx.Accept(v).([]*ast.Condition)
	}

	var postConditions []*ast.Condition
	postConditionsCtx := ctx.PostConditions()
	if postConditionsCtx != nil {
		postConditions = postConditionsCtx.Accept(v).([]*ast.Condition)
	}

	return &ast.FunctionBlock{
		Block:          block,
		PreConditions:  preConditions,
		PostConditions: postConditions,
	}
}

func (v *ProgramVisitor) visitBlock(ctx antlr.ParserRuleContext, statementsCtx IStatementsContext) *ast.Block {
	statements := statementsCtx.Accept(v).([]ast.Statement)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx)
	return &ast.Block{
		Statements: statements,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitPreConditions(ctx *PreConditionsContext) interface{} {
	return ctx.Conditions().Accept(v)
}

func (v *ProgramVisitor) VisitPostConditions(ctx *PostConditionsContext) interface{} {
	return ctx.Conditions().Accept(v)
}

func (v *ProgramVisitor) VisitConditions(ctx *ConditionsContext) interface{} {
	var conditions []*ast.Condition
	for _, statement := range ctx.AllCondition() {
		conditions = append(
			conditions,
			statement.Accept(v).(*ast.Condition),
		)
	}
	return conditions
}

func (v *ProgramVisitor) VisitCondition(ctx *ConditionContext) interface{} {
	parentParent := ctx.GetParent().GetParent()

	_, isPreCondition := parentParent.(*PreConditionsContext)
	_, isPostCondition := parentParent.(*PostConditionsContext)

	var kind ast.ConditionKind
	if isPreCondition {
		kind = ast.ConditionKindPre
	} else if isPostCondition {
		kind = ast.ConditionKindPost
	} else {
		panic(errors.UnreachableError{})
	}

	test := ctx.test.Accept(v).(ast.Expression)

	var message ast.Expression
	if ctx.message != nil {
		message = ctx.message.Accept(v).(ast.Expression)
	}

	return &ast.Condition{
		Kind:    kind,
		Test:    test,
		Message: message,
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

	startPosition := ast.PositionFromToken(ctx.GetStart())

	var endPosition ast.Position
	if expression != nil {
		endPosition = expression.EndPosition()
	} else {
		returnEnd := ctx.Return().GetSymbol().GetStop()
		endPosition = ast.EndPosition(startPosition, returnEnd)
	}

	return &ast.ReturnStatement{
		Expression: expression,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitBreakStatement(ctx *BreakStatementContext) interface{} {
	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.Break().GetSymbol().GetStop())

	return &ast.BreakStatement{
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitContinueStatement(ctx *ContinueStatementContext) interface{} {
	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.Continue().GetSymbol().GetStop())

	return &ast.ContinueStatement{
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	variableKind := ctx.VariableKind().Accept(v).(ast.VariableKind)
	isConstant := variableKind == ast.VariableKindConstant

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

	startPosition := ast.PositionFromToken(ctx.GetStart())
	identifierPosition := ast.PositionFromToken(identifierNode.GetSymbol())

	return &ast.VariableDeclaration{
		IsConstant:    isConstant,
		Identifier:    identifier,
		Value:         expression,
		Type:          fullType,
		StartPos:      startPosition,
		IdentifierPos: identifierPosition,
	}
}

func (v *ProgramVisitor) VisitVariableKind(ctx *VariableKindContext) interface{} {
	if ctx.Let() != nil {
		return ast.VariableKindConstant
	}

	if ctx.Var() != nil {
		return ast.VariableKindVariable
	}

	return ast.VariableKindNotSpecified
}

func (v *ProgramVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	var test ast.IfStatementTest
	if ctx.testExpression != nil {
		test = ctx.testExpression.Accept(v).(ast.Expression)
	} else if ctx.testDeclaration != nil {
		test = ctx.testDeclaration.Accept(v).(*ast.VariableDeclaration)
	} else {
		panic(&errors.UnreachableError{})
	}

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
					StartPos:   ifStatement.StartPosition(),
					EndPos:     ifStatement.EndPosition(),
				}
			}
		}
	}

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.IfStatement{
		Test:     test,
		Then:     then,
		Else:     elseBlock,
		StartPos: startPosition,
	}
}

func (v *ProgramVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	test := ctx.Expression().Accept(v).(ast.Expression)
	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

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

	return &ast.AssignmentStatement{
		Target: target,
		Value:  value,
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

		return &ast.ConditionalExpression{
			Test: expression,
			Then: then,
			Else: alt,
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

	return &ast.BinaryExpression{
		Operation: ast.OperationOr,
		Left:      leftExpression,
		Right:     rightExpression,
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

	return &ast.BinaryExpression{
		Operation: ast.OperationAnd,
		Left:      leftExpression,
		Right:     rightExpression,
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

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	right := ctx.NilCoalescingExpression().Accept(v)
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

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{} {
	// NOTE: right associative

	left := ctx.AdditiveExpression().Accept(v)
	if left == nil {
		return nil
	}

	leftExpression := left.(ast.Expression)

	rightContext := ctx.NilCoalescingExpression()
	if rightContext == nil {
		return leftExpression
	}

	rightExpression := rightContext.Accept(v).(ast.Expression)
	return &ast.BinaryExpression{
		Operation: ast.OperationNilCoalesce,
		Left:      leftExpression,
		Right:     rightExpression,
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

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
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

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
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

	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := expression.EndPosition()

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
				InvokedExpression: result,
				Arguments:         partialExpression.Arguments,
				EndPos:            partialExpression.EndPos,
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
	identifierNode := ctx.Identifier()
	identifier := identifierNode.GetText()

	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, identifierNode.GetSymbol().GetStop())

	// NOTE: partial, expression is filled later
	return &ast.MemberExpression{
		Identifier: identifier,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	index := ctx.Expression().Accept(v).(ast.Expression)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

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
		Value:    value,
		StartPos: startPosition,
		EndPos:   endPosition,
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
	startPosition := ast.PositionFromToken(ctx.GetStart())

	trueNode := ctx.True()
	if trueNode != nil {
		endPosition := ast.EndPosition(startPosition, trueNode.GetSymbol().GetStop())

		return &ast.BoolExpression{
			Value:    true,
			StartPos: startPosition,
			EndPos:   endPosition,
		}
	}

	falseNode := ctx.False()
	if falseNode != nil {
		endPosition := ast.EndPosition(startPosition, falseNode.GetSymbol().GetStop())

		return &ast.BoolExpression{
			Value:    false,
			StartPos: startPosition,
			EndPos:   endPosition,
		}
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitNilLiteral(ctx *NilLiteralContext) interface{} {
	position := ast.PositionFromToken(ctx.GetStart())
	return &ast.NilExpression{
		Pos: position,
	}
}

func (v *ProgramVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.StringLiteral().GetSymbol().GetStop())

	stringLiteral := ctx.StringLiteral().GetText()

	// slice off leading and trailing quotes
	// and parse escape characters
	parsedString := parseStringLiteral(
		stringLiteral[1 : len(stringLiteral)-1],
	)

	return &ast.StringExpression{
		Value:    parsedString,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func parseStringLiteral(s string) string {
	var builder strings.Builder

	var c byte
	for len(s) > 0 {
		c, s = s[0], s[1:]

		if c != '\\' {
			builder.WriteByte(c)
			continue
		}

		c, s = s[0], s[1:]

		switch c {
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
			// skip `{`
			s = s[1:]

			j := 0
			var v rune
			for ; s[j] != '}' && j < 8; j++ {
				x := parseHex(s[j])
				v = v<<4 | x
			}

			builder.WriteRune(v)

			// skip hex characters and `}`

			s = s[j+1:]
		}
	}

	return builder.String()
}

func parseHex(b byte) rune {
	c := rune(b)
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}

	panic(errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	var expressions []ast.Expression
	for _, expression := range ctx.AllExpression() {
		expressions = append(
			expressions,
			expression.Accept(v).(ast.Expression),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

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

	endPosition := ast.PositionFromToken(ctx.GetStop())

	// NOTE: partial, argument is filled later
	return &ast.InvocationExpression{
		Arguments: arguments,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitArgument(ctx *ArgumentContext) interface{} {
	identifierNode := ctx.Identifier()
	label := ""
	var labelStartPos, labelEndPos *ast.Position
	if identifierNode != nil {
		label = identifierNode.GetText()
		symbol := identifierNode.GetSymbol()
		startPos := ast.PositionFromToken(symbol)
		endPos := ast.EndPosition(startPos, symbol.GetStop())
		labelStartPos = &startPos
		labelEndPos = &endPos
	}
	expression := ctx.Expression().Accept(v).(ast.Expression)
	return &ast.Argument{
		Label:         label,
		LabelStartPos: labelStartPos,
		LabelEndPos:   labelEndPos,
		Expression:    expression,
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

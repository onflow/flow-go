package parser

import (
	"encoding/hex"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"math/big"
	"strconv"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

type ProgramVisitor struct {
	*BaseStrictusVisitor
	parseErrors []error
}

func (v *ProgramVisitor) report(errs ...error) {
	v.parseErrors = append(v.parseErrors, errs...)
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

func (v *ProgramVisitor) VisitReplInput(ctx *ReplInputContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	parameterListEnd := ctx.ParameterList().GetStop()
	returnTypeAnnotation := v.visitReturnTypeAnnotation(ctx.returnType, parameterListEnd)

	var parameters ast.Parameters
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).(ast.Parameters)
	}

	// NOTE: in e.g interface declarations, function blocks are optional

	var functionBlock *ast.FunctionBlock
	functionBlockContext := ctx.FunctionBlock()
	if functionBlockContext != nil {
		functionBlock = functionBlockContext.Accept(v).(*ast.FunctionBlock)
	}

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.FunctionDeclaration{
		Access:               access,
		Identifier:           identifier,
		Parameters:           parameters,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             startPosition,
	}
}

// visitReturnTypeAnnotation returns the return type annotation.
// if none was given in the program, return a type annotation with an empty type, with the position of tokenBefore
func (v *ProgramVisitor) visitReturnTypeAnnotation(ctx ITypeAnnotationContext, tokenBefore antlr.Token) *ast.TypeAnnotation {
	if ctx == nil {
		positionBeforeMissingReturnType :=
			ast.PositionFromToken(tokenBefore)
		returnType := &ast.NominalType{
			Identifier: ast.Identifier{
				Pos: positionBeforeMissingReturnType,
			},
		}
		return &ast.TypeAnnotation{
			Move:     false,
			Type:     returnType,
			StartPos: positionBeforeMissingReturnType,
		}
	}
	result := ctx.Accept(v)
	if result == nil {
		return nil
	}
	return result.(*ast.TypeAnnotation)
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

func (v *ProgramVisitor) VisitImportDeclaration(ctx *ImportDeclarationContext) interface{} {
	startPosition := ast.PositionFromToken(ctx.GetStart())

	var location ast.ImportLocation
	var locationPos ast.Position
	var endPos ast.Position

	// string literal?
	stringLiteralContext := ctx.StringLiteral()
	if stringLiteralContext != nil {
		stringExpression := stringLiteralContext.Accept(v).(*ast.StringExpression)
		location = ast.StringImportLocation(stringExpression.Value)
		locationPos = stringExpression.StartPos
		endPos = stringExpression.EndPos
	} else {
		// hexadecimal literal (address)

		hexadecimalLiteralNode := ctx.HexadecimalLiteral()
		text := hexadecimalLiteralNode.GetText()[2:]
		bytes := []byte(strings.Replace(text, "_", "", -1))

		length := len(bytes)
		if length%2 == 1 {
			bytes = append([]byte{'0'}, bytes...)
			length += 1
		}

		address := make([]byte, hex.DecodedLen(length))
		_, err := hex.Decode(address, bytes)
		if err != nil {
			// unreachable, hex literal should always be valid
			panic(err)
		}
		location = ast.AddressImportLocation(address)
		symbol := hexadecimalLiteralNode.GetSymbol()
		locationPos = ast.PositionFromToken(symbol)
		endPos = ast.EndPosition(locationPos, symbol.GetStop())
	}

	allIdentifierNodes := ctx.AllIdentifier()
	identifiers := make([]ast.Identifier, len(allIdentifierNodes))
	for i, identifierNode := range allIdentifierNodes {
		identifiers[i] = identifierNode.Accept(v).(ast.Identifier)
	}

	return &ast.ImportDeclaration{
		Identifiers: identifiers,
		Location:    location,
		StartPos:    startPosition,
		EndPos:      endPos,
		LocationPos: locationPos,
	}
}

func (v *ProgramVisitor) VisitEventDeclaration(ctx *EventDeclarationContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	var parameters ast.Parameters
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).(ast.Parameters)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.EventDeclaration{
		Identifier: identifier,
		Parameters: parameters,
		StartPos:   startPosition,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitEmitStatement(ctx *EmitStatementContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	invocation := ctx.Invocation().Accept(v).(*ast.InvocationExpression)
	invocation.InvokedExpression =
		&ast.IdentifierExpression{
			Identifier: identifier,
		}

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.EmitStatement{
		InvocationExpression: invocation,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{} {
	kind := ctx.CompositeKind().Accept(v).(common.CompositeKind)
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	conformances := ctx.Conformances().Accept(v).([]*ast.NominalType)
	members := ctx.Members().Accept(v).(*ast.Members)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.CompositeDeclaration{
		CompositeKind: kind,
		Identifier:    identifier,
		Conformances:  conformances,
		Members:       members,
		StartPos:      startPosition,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitConformances(ctx *ConformancesContext) interface{} {
	identifierNodes := ctx.AllIdentifier
	conformances := make([]*ast.NominalType, len(identifierNodes()))
	for i, identifierNode := range identifierNodes() {
		identifier := identifierNode.Accept(v).(ast.Identifier)
		conformances[i] = &ast.NominalType{
			Identifier: identifier,
		}
	}
	return conformances
}

func (v *ProgramVisitor) VisitMember(ctx *MemberContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitMembers(ctx *MembersContext) interface{} {

	var fields []*ast.FieldDeclaration
	var initializers []*ast.InitializerDeclaration
	var functions []*ast.FunctionDeclaration
	var compositeDeclarations []*ast.CompositeDeclaration
	var interfaceDeclarations []*ast.InterfaceDeclaration

	for _, memberCtx := range ctx.AllMember() {
		member := memberCtx.Accept(v)

		switch member := member.(type) {
		case *ast.FieldDeclaration:
			fields = append(fields, member)

		case *ast.InitializerDeclaration:
			initializers = append(initializers, member)

		case *ast.FunctionDeclaration:
			functions = append(functions, member)

		case *ast.CompositeDeclaration:
			compositeDeclarations = append(compositeDeclarations, member)

		case *ast.InterfaceDeclaration:
			interfaceDeclarations = append(interfaceDeclarations, member)
		}
	}

	return &ast.Members{
		Fields:                fields,
		Initializers:          initializers,
		Functions:             functions,
		CompositeDeclarations: compositeDeclarations,
	}
}

func (v *ProgramVisitor) VisitField(ctx *FieldContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	variableKindContext := ctx.VariableKind()
	variableKind := ast.VariableKindNotSpecified
	if variableKindContext != nil {
		variableKind = variableKindContext.Accept(v).(ast.VariableKind)
	}

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	typeAnnotationContext := ctx.TypeAnnotation()
	typeAnnotation := typeAnnotationContext.Accept(v).(*ast.TypeAnnotation)

	startPosition := ast.PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, typeAnnotationContext.GetStop().GetStop())

	return &ast.FieldDeclaration{
		Access:         access,
		VariableKind:   variableKind,
		Identifier:     identifier,
		TypeAnnotation: typeAnnotation,
		StartPos:       startPosition,
		EndPos:         endPosition,
	}
}

func (v *ProgramVisitor) VisitInitializer(ctx *InitializerContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	var parameters ast.Parameters
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).(ast.Parameters)
	}

	// NOTE: in e.g interface declarations, function blocks are optional

	var functionBlock *ast.FunctionBlock
	functionBlockContext := ctx.FunctionBlock()
	if functionBlockContext != nil {
		functionBlock = functionBlockContext.Accept(v).(*ast.FunctionBlock)
	}

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.InitializerDeclaration{
		Identifier:    identifier,
		Parameters:    parameters,
		FunctionBlock: functionBlock,
		StartPos:      startPosition,
	}
}

func (v *ProgramVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	kind := ctx.CompositeKind().Accept(v).(common.CompositeKind)
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	members := ctx.Members().Accept(v).(*ast.Members)
	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.InterfaceDeclaration{
		CompositeKind: kind,
		Identifier:    identifier,
		Members:       members,
		StartPos:      startPosition,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitCompositeKind(ctx *CompositeKindContext) interface{} {
	if ctx.Struct() != nil {
		return common.CompositeKindStructure
	}

	if ctx.Resource() != nil {
		return common.CompositeKindResource
	}

	if ctx.Contract() != nil {
		return common.CompositeKindContract
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	parameterListEnd := ctx.ParameterList().GetStop()
	returnTypeAnnotation := v.visitReturnTypeAnnotation(ctx.returnType, parameterListEnd)

	var parameters ast.Parameters
	parameterList := ctx.ParameterList()
	if parameterList != nil {
		parameters = parameterList.Accept(v).(ast.Parameters)
	}

	functionBlock := ctx.FunctionBlock().Accept(v).(*ast.FunctionBlock)

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.FunctionExpression{
		Parameters:           parameters,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	var parameters ast.Parameters

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
	if ctx.argumentLabel != nil {
		label = ctx.argumentLabel.GetText()
	}

	identifier := ctx.parameterName.Accept(v).(ast.Identifier)

	typeAnnotation := ctx.TypeAnnotation().Accept(v).(*ast.TypeAnnotation)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.Parameter{
		Label:          label,
		Identifier:     identifier,
		TypeAnnotation: typeAnnotation,
		StartPos:       startPosition,
		EndPos:         endPosition,
	}
}

func (v *ProgramVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitNominalType(ctx *NominalTypeContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	return &ast.NominalType{
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {

	var parameterTypeAnnotations []*ast.TypeAnnotation
	for _, typeAnnotationContext := range ctx.parameterTypes {
		parameterTypeAnnotations = append(
			parameterTypeAnnotations,
			typeAnnotationContext.Accept(v).(*ast.TypeAnnotation),
		)
	}

	if ctx.returnType == nil {
		return nil
	}
	returnTypeAnnotation := ctx.returnType.Accept(v).(*ast.TypeAnnotation)

	startPosition := ast.PositionFromToken(ctx.OpenParen(0).GetSymbol())
	endPosition := returnTypeAnnotation.EndPosition()

	return &ast.FunctionType{
		ParameterTypeAnnotations: parameterTypeAnnotations,
		ReturnTypeAnnotation:     returnTypeAnnotation,
		StartPos:                 startPosition,
		EndPos:                   endPosition,
	}
}

func (v *ProgramVisitor) VisitVariableSizedType(ctx *VariableSizedTypeContext) interface{} {
	elementType := ctx.FullType().Accept(v).(ast.Type)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.VariableSizedType{
		Type:     elementType,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitConstantSizedType(ctx *ConstantSizedTypeContext) interface{} {
	elementType := ctx.FullType().Accept(v).(ast.Type)

	size, err := strconv.Atoi(ctx.size.GetText())
	if err != nil {
		return nil
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.ConstantSizedType{
		Type:     elementType,
		Size:     size,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitDictionaryType(ctx *DictionaryTypeContext) interface{} {
	keyType := ctx.keyType.Accept(v).(ast.Type)
	valueType := ctx.valueType.Accept(v).(ast.Type)

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.DictionaryType{
		KeyType:   keyType,
		ValueType: valueType,
		StartPos:  startPosition,
		EndPos:    endPosition,
	}
}

func (v *ProgramVisitor) VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{} {
	move := ctx.Move() != nil
	fullType := ctx.FullType().Accept(v).(ast.Type)
	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.TypeAnnotation{
		Move:     move,
		Type:     fullType,
		StartPos: startPosition,
	}
}

func (v *ProgramVisitor) VisitFullType(ctx *FullTypeContext) interface{} {
	baseTypeResult := ctx.BaseType().Accept(v)
	if baseTypeResult == nil {
		return nil
	}
	result := baseTypeResult.(ast.Type)

	for _, optional := range ctx.optionals {
		endPos := ast.PositionFromToken(optional)
		result = &ast.OptionalType{
			Type:   result,
			EndPos: endPos,
		}
	}

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

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	expressionResult := ctx.Expression().Accept(v)
	if expressionResult == nil {
		return nil
	}
	expression := expressionResult.(ast.Expression)
	var typeAnnotation *ast.TypeAnnotation

	typeAnnotationContext := ctx.TypeAnnotation()
	if typeAnnotationContext != nil {
		if x, ok := typeAnnotationContext.Accept(v).(*ast.TypeAnnotation); ok {
			typeAnnotation = x
		}
	}

	transfer := ctx.Transfer().Accept(v).(*ast.Transfer)

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.VariableDeclaration{
		IsConstant:     isConstant,
		Identifier:     identifier,
		Value:          expression,
		TypeAnnotation: typeAnnotation,
		Transfer:       transfer,
		StartPos:       startPosition,
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
	target := v.targetExpression(ctx.Identifier(), ctx.AllExpressionAccess())
	transfer := ctx.Transfer().Accept(v).(*ast.Transfer)
	value := ctx.Expression().Accept(v).(ast.Expression)

	return &ast.AssignmentStatement{
		Target:   target,
		Transfer: transfer,
		Value:    value,
	}
}

func (v *ProgramVisitor) targetExpression(
	identifierContext IIdentifierContext,
	expressionAccessContexts []IExpressionAccessContext,
) ast.Expression {
	identifier := identifierContext.Accept(v).(ast.Identifier)
	var target ast.Expression = &ast.IdentifierExpression{
		Identifier: identifier,
	}

	for _, accessExpressionContext := range expressionAccessContexts {
		expression := accessExpressionContext.Accept(v)
		accessExpression := expression.(ast.AccessExpression)
		target = v.wrapPartialAccessExpression(target, accessExpression)
	}

	return target
}

func (v *ProgramVisitor) VisitTransfer(ctx *TransferContext) interface{} {
	operation := ast.TransferOperationCopy
	if ctx.Move() != nil {
		operation = ast.TransferOperationMove
	}

	position := ast.PositionFromToken(ctx.GetStart())

	return &ast.Transfer{
		Operation: operation,
		Pos:       position,
	}
}

func (v *ProgramVisitor) VisitSwap(ctx *SwapContext) interface{} {
	left := ctx.left.Accept(v).(ast.Expression)
	right := ctx.right.Accept(v).(ast.Expression)

	return &ast.SwapStatement{
		Left:  left,
		Right: right,
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

	left := ctx.FailableDowncastingExpression().Accept(v)
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

func (v *ProgramVisitor) VisitFailableDowncastingExpression(ctx *FailableDowncastingExpressionContext) interface{} {
	typeAnnotationContext := ctx.TypeAnnotation()
	if typeAnnotationContext == nil {
		return ctx.ConcatenatingExpression().Accept(v)
	}

	expression := ctx.FailableDowncastingExpression().Accept(v).(ast.Expression)
	typeAnnotation := typeAnnotationContext.Accept(v).(*ast.TypeAnnotation)

	return &ast.FailableDowncastExpression{
		Expression:     expression,
		TypeAnnotation: typeAnnotation,
	}
}

func (v *ProgramVisitor) VisitConcatenatingExpression(ctx *ConcatenatingExpressionContext) interface{} {
	right := ctx.AdditiveExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.ConcatenatingExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationConcat,
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
		v.report(
			&JuxtaposedUnaryOperatorsError{
				Pos: position,
			},
		)
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

	if ctx.Move() != nil {
		return ast.OperationMove
	}

	panic(&errors.UnreachableError{})
}

func (v *ProgramVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitComposedExpression(ctx *ComposedExpressionContext) interface{} {
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
			TargetExpression:   wrapped,
			IndexingExpression: partialAccessExpression.IndexingExpression,
			IndexingType:       partialAccessExpression.IndexingType,
			StartPos:           partialAccessExpression.StartPos,
			EndPos:             partialAccessExpression.EndPos,
		}
	case *ast.MemberExpression:
		return &ast.MemberExpression{
			Expression: wrapped,
			Identifier: partialAccessExpression.Identifier,
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
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	// NOTE: partial, expression is filled later
	return &ast.MemberExpression{
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	var indexExpression ast.Expression
	var indexType ast.Type

	expressionContext := ctx.Expression()
	if expressionContext != nil {
		indexExpression = expressionContext.Accept(v).(ast.Expression)
	} else {
		indexType = ctx.FullType().Accept(v).(ast.Type)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	// NOTE: partial, expression is filled later
	return &ast.IndexExpression{
		IndexingExpression: indexExpression,
		IndexingType:       indexType,
		StartPos:           startPosition,
		EndPos:             endPosition,
	}
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitPrimaryExpressionStart(ctx *PrimaryExpressionStartContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitCreateExpression(ctx *CreateExpressionContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	invocation := ctx.Invocation().Accept(v).(*ast.InvocationExpression)
	invocation.InvokedExpression =
		&ast.IdentifierExpression{
			Identifier: identifier,
		}

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.CreateExpression{
		InvocationExpression: invocation,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitDestroyExpression(ctx *DestroyExpressionContext) interface{} {
	expression := ctx.Expression().Accept(v).(ast.Expression)

	startPosition := ast.PositionFromToken(ctx.GetStart())

	return &ast.DestroyExpression{
		Expression: expression,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return ctx.Literal().Accept(v)
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{} {
	intExpression := ctx.PositiveIntegerLiteral().Accept(v).(*ast.IntExpression)
	if ctx.Minus() != nil {
		intExpression.Value.Neg(intExpression.Value)
	}
	return intExpression
}

func (v *ProgramVisitor) parseIntExpression(token antlr.Token, text string, kind IntegerLiteralKind) *ast.IntExpression {
	startPosition := ast.PositionFromToken(token)
	endPosition := ast.EndPosition(startPosition, token.GetStop())

	// check literal has no leading underscore
	if strings.HasPrefix(text, "_") {
		v.report(
			&InvalidIntegerLiteralError{
				IntegerLiteralKind:        kind,
				InvalidIntegerLiteralKind: InvalidIntegerLiteralKindLeadingUnderscore,
				// NOTE: not using text, because it has the base-prefix stripped
				Literal:  token.GetText(),
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		)
	}

	// check literal has no trailing underscore
	if strings.HasSuffix(text, "_") {
		v.report(
			&InvalidIntegerLiteralError{
				IntegerLiteralKind:        kind,
				InvalidIntegerLiteralKind: InvalidIntegerLiteralKindTrailingUnderscore,
				// NOTE: not using text, because it has the base-prefix stripped
				Literal:  token.GetText(),
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		)
	}

	withoutUnderscores := strings.Replace(text, "_", "", -1)

	value, ok := big.NewInt(0).SetString(withoutUnderscores, kind.Base())
	if !ok {
		v.report(
			&InvalidIntegerLiteralError{
				IntegerLiteralKind:        kind,
				InvalidIntegerLiteralKind: InvalidIntegerLiteralKindUnknown,
				// NOTE: not using text, because it has the base-prefix stripped
				Literal:  token.GetText(),
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		)
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

	v.report(
		&InvalidIntegerLiteralError{
			IntegerLiteralKind:        IntegerLiteralKindUnknown,
			InvalidIntegerLiteralKind: InvalidIntegerLiteralKindUnknownPrefix,
			Literal:                   ctx.GetText(),
			StartPos:                  startPosition,
			EndPos:                    endPosition,
		},
	)

	return &ast.IntExpression{
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return v.parseIntExpression(
		ctx.GetStart(),
		ctx.GetText(),
		IntegerLiteralKindDecimal,
	)
}

func (v *ProgramVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return v.parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindBinary,
	)
}

func (v *ProgramVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return v.parseIntExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindOctal,
	)
}

func (v *ProgramVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return v.parseIntExpression(
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

func (v *ProgramVisitor) VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{} {
	var entries []ast.Entry
	for _, entry := range ctx.AllDictionaryEntry() {
		entries = append(
			entries,
			entry.Accept(v).(ast.Entry),
		)
	}

	startPosition, endPosition := ast.PositionRangeFromContext(ctx)

	return &ast.DictionaryExpression{
		Entries:  entries,
		StartPos: startPosition,
		EndPos:   endPosition,
	}
}

func (v *ProgramVisitor) VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{} {
	key := ctx.key.Accept(v).(ast.Expression)
	value := ctx.value.Accept(v).(ast.Expression)

	return ast.Entry{
		Key:   key,
		Value: value,
	}
}

func (v *ProgramVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	return &ast.IdentifierExpression{
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {

	text := ctx.GetText()
	pos := ast.PositionFromToken(ctx.GetStart())

	return ast.Identifier{
		Identifier: text,
		Pos:        pos,
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
		symbol := identifierNode.GetStart()
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

// Code generated from parser/Cadence.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Cadence
import "github.com/antlr/antlr4/runtime/Go/antlr"

// BaseCadenceListener is a complete listener for a parse tree produced by CadenceParser.
type BaseCadenceListener struct{}

var _ CadenceListener = &BaseCadenceListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseCadenceListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseCadenceListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseCadenceListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseCadenceListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterProgram is called when production program is entered.
func (s *BaseCadenceListener) EnterProgram(ctx *ProgramContext) {}

// ExitProgram is called when production program is exited.
func (s *BaseCadenceListener) ExitProgram(ctx *ProgramContext) {}

// EnterReplInput is called when production replInput is entered.
func (s *BaseCadenceListener) EnterReplInput(ctx *ReplInputContext) {}

// ExitReplInput is called when production replInput is exited.
func (s *BaseCadenceListener) ExitReplInput(ctx *ReplInputContext) {}

// EnterReplElement is called when production replElement is entered.
func (s *BaseCadenceListener) EnterReplElement(ctx *ReplElementContext) {}

// ExitReplElement is called when production replElement is exited.
func (s *BaseCadenceListener) ExitReplElement(ctx *ReplElementContext) {}

// EnterReplStatement is called when production replStatement is entered.
func (s *BaseCadenceListener) EnterReplStatement(ctx *ReplStatementContext) {}

// ExitReplStatement is called when production replStatement is exited.
func (s *BaseCadenceListener) ExitReplStatement(ctx *ReplStatementContext) {}

// EnterReplDeclaration is called when production replDeclaration is entered.
func (s *BaseCadenceListener) EnterReplDeclaration(ctx *ReplDeclarationContext) {}

// ExitReplDeclaration is called when production replDeclaration is exited.
func (s *BaseCadenceListener) ExitReplDeclaration(ctx *ReplDeclarationContext) {}

// EnterDeclaration is called when production declaration is entered.
func (s *BaseCadenceListener) EnterDeclaration(ctx *DeclarationContext) {}

// ExitDeclaration is called when production declaration is exited.
func (s *BaseCadenceListener) ExitDeclaration(ctx *DeclarationContext) {}

// EnterTransactionDeclaration is called when production transactionDeclaration is entered.
func (s *BaseCadenceListener) EnterTransactionDeclaration(ctx *TransactionDeclarationContext) {}

// ExitTransactionDeclaration is called when production transactionDeclaration is exited.
func (s *BaseCadenceListener) ExitTransactionDeclaration(ctx *TransactionDeclarationContext) {}

// EnterPrepare is called when production prepare is entered.
func (s *BaseCadenceListener) EnterPrepare(ctx *PrepareContext) {}

// ExitPrepare is called when production prepare is exited.
func (s *BaseCadenceListener) ExitPrepare(ctx *PrepareContext) {}

// EnterExecute is called when production execute is entered.
func (s *BaseCadenceListener) EnterExecute(ctx *ExecuteContext) {}

// ExitExecute is called when production execute is exited.
func (s *BaseCadenceListener) ExitExecute(ctx *ExecuteContext) {}

// EnterImportDeclaration is called when production importDeclaration is entered.
func (s *BaseCadenceListener) EnterImportDeclaration(ctx *ImportDeclarationContext) {}

// ExitImportDeclaration is called when production importDeclaration is exited.
func (s *BaseCadenceListener) ExitImportDeclaration(ctx *ImportDeclarationContext) {}

// EnterAccess is called when production access is entered.
func (s *BaseCadenceListener) EnterAccess(ctx *AccessContext) {}

// ExitAccess is called when production access is exited.
func (s *BaseCadenceListener) ExitAccess(ctx *AccessContext) {}

// EnterCompositeDeclaration is called when production compositeDeclaration is entered.
func (s *BaseCadenceListener) EnterCompositeDeclaration(ctx *CompositeDeclarationContext) {}

// ExitCompositeDeclaration is called when production compositeDeclaration is exited.
func (s *BaseCadenceListener) ExitCompositeDeclaration(ctx *CompositeDeclarationContext) {}

// EnterConformances is called when production conformances is entered.
func (s *BaseCadenceListener) EnterConformances(ctx *ConformancesContext) {}

// ExitConformances is called when production conformances is exited.
func (s *BaseCadenceListener) ExitConformances(ctx *ConformancesContext) {}

// EnterVariableKind is called when production variableKind is entered.
func (s *BaseCadenceListener) EnterVariableKind(ctx *VariableKindContext) {}

// ExitVariableKind is called when production variableKind is exited.
func (s *BaseCadenceListener) ExitVariableKind(ctx *VariableKindContext) {}

// EnterField is called when production field is entered.
func (s *BaseCadenceListener) EnterField(ctx *FieldContext) {}

// ExitField is called when production field is exited.
func (s *BaseCadenceListener) ExitField(ctx *FieldContext) {}

// EnterFields is called when production fields is entered.
func (s *BaseCadenceListener) EnterFields(ctx *FieldsContext) {}

// ExitFields is called when production fields is exited.
func (s *BaseCadenceListener) ExitFields(ctx *FieldsContext) {}

// EnterInterfaceDeclaration is called when production interfaceDeclaration is entered.
func (s *BaseCadenceListener) EnterInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// ExitInterfaceDeclaration is called when production interfaceDeclaration is exited.
func (s *BaseCadenceListener) ExitInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// EnterMembersAndNestedDeclarations is called when production membersAndNestedDeclarations is entered.
func (s *BaseCadenceListener) EnterMembersAndNestedDeclarations(ctx *MembersAndNestedDeclarationsContext) {
}

// ExitMembersAndNestedDeclarations is called when production membersAndNestedDeclarations is exited.
func (s *BaseCadenceListener) ExitMembersAndNestedDeclarations(ctx *MembersAndNestedDeclarationsContext) {
}

// EnterMemberOrNestedDeclaration is called when production memberOrNestedDeclaration is entered.
func (s *BaseCadenceListener) EnterMemberOrNestedDeclaration(ctx *MemberOrNestedDeclarationContext) {}

// ExitMemberOrNestedDeclaration is called when production memberOrNestedDeclaration is exited.
func (s *BaseCadenceListener) ExitMemberOrNestedDeclaration(ctx *MemberOrNestedDeclarationContext) {}

// EnterCompositeKind is called when production compositeKind is entered.
func (s *BaseCadenceListener) EnterCompositeKind(ctx *CompositeKindContext) {}

// ExitCompositeKind is called when production compositeKind is exited.
func (s *BaseCadenceListener) ExitCompositeKind(ctx *CompositeKindContext) {}

// EnterSpecialFunctionDeclaration is called when production specialFunctionDeclaration is entered.
func (s *BaseCadenceListener) EnterSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) {
}

// ExitSpecialFunctionDeclaration is called when production specialFunctionDeclaration is exited.
func (s *BaseCadenceListener) ExitSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) {
}

// EnterFunctionDeclaration is called when production functionDeclaration is entered.
func (s *BaseCadenceListener) EnterFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// ExitFunctionDeclaration is called when production functionDeclaration is exited.
func (s *BaseCadenceListener) ExitFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// EnterEventDeclaration is called when production eventDeclaration is entered.
func (s *BaseCadenceListener) EnterEventDeclaration(ctx *EventDeclarationContext) {}

// ExitEventDeclaration is called when production eventDeclaration is exited.
func (s *BaseCadenceListener) ExitEventDeclaration(ctx *EventDeclarationContext) {}

// EnterParameterList is called when production parameterList is entered.
func (s *BaseCadenceListener) EnterParameterList(ctx *ParameterListContext) {}

// ExitParameterList is called when production parameterList is exited.
func (s *BaseCadenceListener) ExitParameterList(ctx *ParameterListContext) {}

// EnterParameter is called when production parameter is entered.
func (s *BaseCadenceListener) EnterParameter(ctx *ParameterContext) {}

// ExitParameter is called when production parameter is exited.
func (s *BaseCadenceListener) ExitParameter(ctx *ParameterContext) {}

// EnterTypeAnnotation is called when production typeAnnotation is entered.
func (s *BaseCadenceListener) EnterTypeAnnotation(ctx *TypeAnnotationContext) {}

// ExitTypeAnnotation is called when production typeAnnotation is exited.
func (s *BaseCadenceListener) ExitTypeAnnotation(ctx *TypeAnnotationContext) {}

// EnterFullType is called when production fullType is entered.
func (s *BaseCadenceListener) EnterFullType(ctx *FullTypeContext) {}

// ExitFullType is called when production fullType is exited.
func (s *BaseCadenceListener) ExitFullType(ctx *FullTypeContext) {}

// EnterInnerType is called when production innerType is entered.
func (s *BaseCadenceListener) EnterInnerType(ctx *InnerTypeContext) {}

// ExitInnerType is called when production innerType is exited.
func (s *BaseCadenceListener) ExitInnerType(ctx *InnerTypeContext) {}

// EnterBaseType is called when production baseType is entered.
func (s *BaseCadenceListener) EnterBaseType(ctx *BaseTypeContext) {}

// ExitBaseType is called when production baseType is exited.
func (s *BaseCadenceListener) ExitBaseType(ctx *BaseTypeContext) {}

// EnterTypeRestrictions is called when production typeRestrictions is entered.
func (s *BaseCadenceListener) EnterTypeRestrictions(ctx *TypeRestrictionsContext) {}

// ExitTypeRestrictions is called when production typeRestrictions is exited.
func (s *BaseCadenceListener) ExitTypeRestrictions(ctx *TypeRestrictionsContext) {}

// EnterNominalType is called when production nominalType is entered.
func (s *BaseCadenceListener) EnterNominalType(ctx *NominalTypeContext) {}

// ExitNominalType is called when production nominalType is exited.
func (s *BaseCadenceListener) ExitNominalType(ctx *NominalTypeContext) {}

// EnterFunctionType is called when production functionType is entered.
func (s *BaseCadenceListener) EnterFunctionType(ctx *FunctionTypeContext) {}

// ExitFunctionType is called when production functionType is exited.
func (s *BaseCadenceListener) ExitFunctionType(ctx *FunctionTypeContext) {}

// EnterVariableSizedType is called when production variableSizedType is entered.
func (s *BaseCadenceListener) EnterVariableSizedType(ctx *VariableSizedTypeContext) {}

// ExitVariableSizedType is called when production variableSizedType is exited.
func (s *BaseCadenceListener) ExitVariableSizedType(ctx *VariableSizedTypeContext) {}

// EnterConstantSizedType is called when production constantSizedType is entered.
func (s *BaseCadenceListener) EnterConstantSizedType(ctx *ConstantSizedTypeContext) {}

// ExitConstantSizedType is called when production constantSizedType is exited.
func (s *BaseCadenceListener) ExitConstantSizedType(ctx *ConstantSizedTypeContext) {}

// EnterDictionaryType is called when production dictionaryType is entered.
func (s *BaseCadenceListener) EnterDictionaryType(ctx *DictionaryTypeContext) {}

// ExitDictionaryType is called when production dictionaryType is exited.
func (s *BaseCadenceListener) ExitDictionaryType(ctx *DictionaryTypeContext) {}

// EnterBlock is called when production block is entered.
func (s *BaseCadenceListener) EnterBlock(ctx *BlockContext) {}

// ExitBlock is called when production block is exited.
func (s *BaseCadenceListener) ExitBlock(ctx *BlockContext) {}

// EnterFunctionBlock is called when production functionBlock is entered.
func (s *BaseCadenceListener) EnterFunctionBlock(ctx *FunctionBlockContext) {}

// ExitFunctionBlock is called when production functionBlock is exited.
func (s *BaseCadenceListener) ExitFunctionBlock(ctx *FunctionBlockContext) {}

// EnterPreConditions is called when production preConditions is entered.
func (s *BaseCadenceListener) EnterPreConditions(ctx *PreConditionsContext) {}

// ExitPreConditions is called when production preConditions is exited.
func (s *BaseCadenceListener) ExitPreConditions(ctx *PreConditionsContext) {}

// EnterPostConditions is called when production postConditions is entered.
func (s *BaseCadenceListener) EnterPostConditions(ctx *PostConditionsContext) {}

// ExitPostConditions is called when production postConditions is exited.
func (s *BaseCadenceListener) ExitPostConditions(ctx *PostConditionsContext) {}

// EnterConditions is called when production conditions is entered.
func (s *BaseCadenceListener) EnterConditions(ctx *ConditionsContext) {}

// ExitConditions is called when production conditions is exited.
func (s *BaseCadenceListener) ExitConditions(ctx *ConditionsContext) {}

// EnterCondition is called when production condition is entered.
func (s *BaseCadenceListener) EnterCondition(ctx *ConditionContext) {}

// ExitCondition is called when production condition is exited.
func (s *BaseCadenceListener) ExitCondition(ctx *ConditionContext) {}

// EnterStatements is called when production statements is entered.
func (s *BaseCadenceListener) EnterStatements(ctx *StatementsContext) {}

// ExitStatements is called when production statements is exited.
func (s *BaseCadenceListener) ExitStatements(ctx *StatementsContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseCadenceListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseCadenceListener) ExitStatement(ctx *StatementContext) {}

// EnterReturnStatement is called when production returnStatement is entered.
func (s *BaseCadenceListener) EnterReturnStatement(ctx *ReturnStatementContext) {}

// ExitReturnStatement is called when production returnStatement is exited.
func (s *BaseCadenceListener) ExitReturnStatement(ctx *ReturnStatementContext) {}

// EnterBreakStatement is called when production breakStatement is entered.
func (s *BaseCadenceListener) EnterBreakStatement(ctx *BreakStatementContext) {}

// ExitBreakStatement is called when production breakStatement is exited.
func (s *BaseCadenceListener) ExitBreakStatement(ctx *BreakStatementContext) {}

// EnterContinueStatement is called when production continueStatement is entered.
func (s *BaseCadenceListener) EnterContinueStatement(ctx *ContinueStatementContext) {}

// ExitContinueStatement is called when production continueStatement is exited.
func (s *BaseCadenceListener) ExitContinueStatement(ctx *ContinueStatementContext) {}

// EnterIfStatement is called when production ifStatement is entered.
func (s *BaseCadenceListener) EnterIfStatement(ctx *IfStatementContext) {}

// ExitIfStatement is called when production ifStatement is exited.
func (s *BaseCadenceListener) ExitIfStatement(ctx *IfStatementContext) {}

// EnterWhileStatement is called when production whileStatement is entered.
func (s *BaseCadenceListener) EnterWhileStatement(ctx *WhileStatementContext) {}

// ExitWhileStatement is called when production whileStatement is exited.
func (s *BaseCadenceListener) ExitWhileStatement(ctx *WhileStatementContext) {}

// EnterForStatement is called when production forStatement is entered.
func (s *BaseCadenceListener) EnterForStatement(ctx *ForStatementContext) {}

// ExitForStatement is called when production forStatement is exited.
func (s *BaseCadenceListener) ExitForStatement(ctx *ForStatementContext) {}

// EnterEmitStatement is called when production emitStatement is entered.
func (s *BaseCadenceListener) EnterEmitStatement(ctx *EmitStatementContext) {}

// ExitEmitStatement is called when production emitStatement is exited.
func (s *BaseCadenceListener) ExitEmitStatement(ctx *EmitStatementContext) {}

// EnterVariableDeclaration is called when production variableDeclaration is entered.
func (s *BaseCadenceListener) EnterVariableDeclaration(ctx *VariableDeclarationContext) {}

// ExitVariableDeclaration is called when production variableDeclaration is exited.
func (s *BaseCadenceListener) ExitVariableDeclaration(ctx *VariableDeclarationContext) {}

// EnterAssignment is called when production assignment is entered.
func (s *BaseCadenceListener) EnterAssignment(ctx *AssignmentContext) {}

// ExitAssignment is called when production assignment is exited.
func (s *BaseCadenceListener) ExitAssignment(ctx *AssignmentContext) {}

// EnterSwap is called when production swap is entered.
func (s *BaseCadenceListener) EnterSwap(ctx *SwapContext) {}

// ExitSwap is called when production swap is exited.
func (s *BaseCadenceListener) ExitSwap(ctx *SwapContext) {}

// EnterTransfer is called when production transfer is entered.
func (s *BaseCadenceListener) EnterTransfer(ctx *TransferContext) {}

// ExitTransfer is called when production transfer is exited.
func (s *BaseCadenceListener) ExitTransfer(ctx *TransferContext) {}

// EnterExpression is called when production expression is entered.
func (s *BaseCadenceListener) EnterExpression(ctx *ExpressionContext) {}

// ExitExpression is called when production expression is exited.
func (s *BaseCadenceListener) ExitExpression(ctx *ExpressionContext) {}

// EnterConditionalExpression is called when production conditionalExpression is entered.
func (s *BaseCadenceListener) EnterConditionalExpression(ctx *ConditionalExpressionContext) {}

// ExitConditionalExpression is called when production conditionalExpression is exited.
func (s *BaseCadenceListener) ExitConditionalExpression(ctx *ConditionalExpressionContext) {}

// EnterOrExpression is called when production orExpression is entered.
func (s *BaseCadenceListener) EnterOrExpression(ctx *OrExpressionContext) {}

// ExitOrExpression is called when production orExpression is exited.
func (s *BaseCadenceListener) ExitOrExpression(ctx *OrExpressionContext) {}

// EnterAndExpression is called when production andExpression is entered.
func (s *BaseCadenceListener) EnterAndExpression(ctx *AndExpressionContext) {}

// ExitAndExpression is called when production andExpression is exited.
func (s *BaseCadenceListener) ExitAndExpression(ctx *AndExpressionContext) {}

// EnterEqualityExpression is called when production equalityExpression is entered.
func (s *BaseCadenceListener) EnterEqualityExpression(ctx *EqualityExpressionContext) {}

// ExitEqualityExpression is called when production equalityExpression is exited.
func (s *BaseCadenceListener) ExitEqualityExpression(ctx *EqualityExpressionContext) {}

// EnterRelationalExpression is called when production relationalExpression is entered.
func (s *BaseCadenceListener) EnterRelationalExpression(ctx *RelationalExpressionContext) {}

// ExitRelationalExpression is called when production relationalExpression is exited.
func (s *BaseCadenceListener) ExitRelationalExpression(ctx *RelationalExpressionContext) {}

// EnterNilCoalescingExpression is called when production nilCoalescingExpression is entered.
func (s *BaseCadenceListener) EnterNilCoalescingExpression(ctx *NilCoalescingExpressionContext) {}

// ExitNilCoalescingExpression is called when production nilCoalescingExpression is exited.
func (s *BaseCadenceListener) ExitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) {}

// EnterBitwiseOrExpression is called when production bitwiseOrExpression is entered.
func (s *BaseCadenceListener) EnterBitwiseOrExpression(ctx *BitwiseOrExpressionContext) {}

// ExitBitwiseOrExpression is called when production bitwiseOrExpression is exited.
func (s *BaseCadenceListener) ExitBitwiseOrExpression(ctx *BitwiseOrExpressionContext) {}

// EnterBitwiseXorExpression is called when production bitwiseXorExpression is entered.
func (s *BaseCadenceListener) EnterBitwiseXorExpression(ctx *BitwiseXorExpressionContext) {}

// ExitBitwiseXorExpression is called when production bitwiseXorExpression is exited.
func (s *BaseCadenceListener) ExitBitwiseXorExpression(ctx *BitwiseXorExpressionContext) {}

// EnterBitwiseAndExpression is called when production bitwiseAndExpression is entered.
func (s *BaseCadenceListener) EnterBitwiseAndExpression(ctx *BitwiseAndExpressionContext) {}

// ExitBitwiseAndExpression is called when production bitwiseAndExpression is exited.
func (s *BaseCadenceListener) ExitBitwiseAndExpression(ctx *BitwiseAndExpressionContext) {}

// EnterBitwiseShiftExpression is called when production bitwiseShiftExpression is entered.
func (s *BaseCadenceListener) EnterBitwiseShiftExpression(ctx *BitwiseShiftExpressionContext) {}

// ExitBitwiseShiftExpression is called when production bitwiseShiftExpression is exited.
func (s *BaseCadenceListener) ExitBitwiseShiftExpression(ctx *BitwiseShiftExpressionContext) {}

// EnterAdditiveExpression is called when production additiveExpression is entered.
func (s *BaseCadenceListener) EnterAdditiveExpression(ctx *AdditiveExpressionContext) {}

// ExitAdditiveExpression is called when production additiveExpression is exited.
func (s *BaseCadenceListener) ExitAdditiveExpression(ctx *AdditiveExpressionContext) {}

// EnterMultiplicativeExpression is called when production multiplicativeExpression is entered.
func (s *BaseCadenceListener) EnterMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// ExitMultiplicativeExpression is called when production multiplicativeExpression is exited.
func (s *BaseCadenceListener) ExitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// EnterCastingExpression is called when production castingExpression is entered.
func (s *BaseCadenceListener) EnterCastingExpression(ctx *CastingExpressionContext) {}

// ExitCastingExpression is called when production castingExpression is exited.
func (s *BaseCadenceListener) ExitCastingExpression(ctx *CastingExpressionContext) {}

// EnterUnaryExpression is called when production unaryExpression is entered.
func (s *BaseCadenceListener) EnterUnaryExpression(ctx *UnaryExpressionContext) {}

// ExitUnaryExpression is called when production unaryExpression is exited.
func (s *BaseCadenceListener) ExitUnaryExpression(ctx *UnaryExpressionContext) {}

// EnterPrimaryExpression is called when production primaryExpression is entered.
func (s *BaseCadenceListener) EnterPrimaryExpression(ctx *PrimaryExpressionContext) {}

// ExitPrimaryExpression is called when production primaryExpression is exited.
func (s *BaseCadenceListener) ExitPrimaryExpression(ctx *PrimaryExpressionContext) {}

// EnterAccessExpression is called when production accessExpression is entered.
func (s *BaseCadenceListener) EnterAccessExpression(ctx *AccessExpressionContext) {}

// ExitAccessExpression is called when production accessExpression is exited.
func (s *BaseCadenceListener) ExitAccessExpression(ctx *AccessExpressionContext) {}

// EnterInvocationExpression is called when production invocationExpression is entered.
func (s *BaseCadenceListener) EnterInvocationExpression(ctx *InvocationExpressionContext) {}

// ExitInvocationExpression is called when production invocationExpression is exited.
func (s *BaseCadenceListener) ExitInvocationExpression(ctx *InvocationExpressionContext) {}

// EnterNestedExpression is called when production nestedExpression is entered.
func (s *BaseCadenceListener) EnterNestedExpression(ctx *NestedExpressionContext) {}

// ExitNestedExpression is called when production nestedExpression is exited.
func (s *BaseCadenceListener) ExitNestedExpression(ctx *NestedExpressionContext) {}

// EnterIdentifierExpression is called when production identifierExpression is entered.
func (s *BaseCadenceListener) EnterIdentifierExpression(ctx *IdentifierExpressionContext) {}

// ExitIdentifierExpression is called when production identifierExpression is exited.
func (s *BaseCadenceListener) ExitIdentifierExpression(ctx *IdentifierExpressionContext) {}

// EnterForceExpression is called when production forceExpression is entered.
func (s *BaseCadenceListener) EnterForceExpression(ctx *ForceExpressionContext) {}

// ExitForceExpression is called when production forceExpression is exited.
func (s *BaseCadenceListener) ExitForceExpression(ctx *ForceExpressionContext) {}

// EnterLiteralExpression is called when production literalExpression is entered.
func (s *BaseCadenceListener) EnterLiteralExpression(ctx *LiteralExpressionContext) {}

// ExitLiteralExpression is called when production literalExpression is exited.
func (s *BaseCadenceListener) ExitLiteralExpression(ctx *LiteralExpressionContext) {}

// EnterFunctionExpression is called when production functionExpression is entered.
func (s *BaseCadenceListener) EnterFunctionExpression(ctx *FunctionExpressionContext) {}

// ExitFunctionExpression is called when production functionExpression is exited.
func (s *BaseCadenceListener) ExitFunctionExpression(ctx *FunctionExpressionContext) {}

// EnterEqualityOp is called when production equalityOp is entered.
func (s *BaseCadenceListener) EnterEqualityOp(ctx *EqualityOpContext) {}

// ExitEqualityOp is called when production equalityOp is exited.
func (s *BaseCadenceListener) ExitEqualityOp(ctx *EqualityOpContext) {}

// EnterRelationalOp is called when production relationalOp is entered.
func (s *BaseCadenceListener) EnterRelationalOp(ctx *RelationalOpContext) {}

// ExitRelationalOp is called when production relationalOp is exited.
func (s *BaseCadenceListener) ExitRelationalOp(ctx *RelationalOpContext) {}

// EnterBitwiseShiftOp is called when production bitwiseShiftOp is entered.
func (s *BaseCadenceListener) EnterBitwiseShiftOp(ctx *BitwiseShiftOpContext) {}

// ExitBitwiseShiftOp is called when production bitwiseShiftOp is exited.
func (s *BaseCadenceListener) ExitBitwiseShiftOp(ctx *BitwiseShiftOpContext) {}

// EnterAdditiveOp is called when production additiveOp is entered.
func (s *BaseCadenceListener) EnterAdditiveOp(ctx *AdditiveOpContext) {}

// ExitAdditiveOp is called when production additiveOp is exited.
func (s *BaseCadenceListener) ExitAdditiveOp(ctx *AdditiveOpContext) {}

// EnterMultiplicativeOp is called when production multiplicativeOp is entered.
func (s *BaseCadenceListener) EnterMultiplicativeOp(ctx *MultiplicativeOpContext) {}

// ExitMultiplicativeOp is called when production multiplicativeOp is exited.
func (s *BaseCadenceListener) ExitMultiplicativeOp(ctx *MultiplicativeOpContext) {}

// EnterUnaryOp is called when production unaryOp is entered.
func (s *BaseCadenceListener) EnterUnaryOp(ctx *UnaryOpContext) {}

// ExitUnaryOp is called when production unaryOp is exited.
func (s *BaseCadenceListener) ExitUnaryOp(ctx *UnaryOpContext) {}

// EnterCastingOp is called when production castingOp is entered.
func (s *BaseCadenceListener) EnterCastingOp(ctx *CastingOpContext) {}

// ExitCastingOp is called when production castingOp is exited.
func (s *BaseCadenceListener) ExitCastingOp(ctx *CastingOpContext) {}

// EnterCreateExpression is called when production createExpression is entered.
func (s *BaseCadenceListener) EnterCreateExpression(ctx *CreateExpressionContext) {}

// ExitCreateExpression is called when production createExpression is exited.
func (s *BaseCadenceListener) ExitCreateExpression(ctx *CreateExpressionContext) {}

// EnterDestroyExpression is called when production destroyExpression is entered.
func (s *BaseCadenceListener) EnterDestroyExpression(ctx *DestroyExpressionContext) {}

// ExitDestroyExpression is called when production destroyExpression is exited.
func (s *BaseCadenceListener) ExitDestroyExpression(ctx *DestroyExpressionContext) {}

// EnterReferenceExpression is called when production referenceExpression is entered.
func (s *BaseCadenceListener) EnterReferenceExpression(ctx *ReferenceExpressionContext) {}

// ExitReferenceExpression is called when production referenceExpression is exited.
func (s *BaseCadenceListener) ExitReferenceExpression(ctx *ReferenceExpressionContext) {}

// EnterExpressionAccess is called when production expressionAccess is entered.
func (s *BaseCadenceListener) EnterExpressionAccess(ctx *ExpressionAccessContext) {}

// ExitExpressionAccess is called when production expressionAccess is exited.
func (s *BaseCadenceListener) ExitExpressionAccess(ctx *ExpressionAccessContext) {}

// EnterMemberAccess is called when production memberAccess is entered.
func (s *BaseCadenceListener) EnterMemberAccess(ctx *MemberAccessContext) {}

// ExitMemberAccess is called when production memberAccess is exited.
func (s *BaseCadenceListener) ExitMemberAccess(ctx *MemberAccessContext) {}

// EnterBracketExpression is called when production bracketExpression is entered.
func (s *BaseCadenceListener) EnterBracketExpression(ctx *BracketExpressionContext) {}

// ExitBracketExpression is called when production bracketExpression is exited.
func (s *BaseCadenceListener) ExitBracketExpression(ctx *BracketExpressionContext) {}

// EnterInvocation is called when production invocation is entered.
func (s *BaseCadenceListener) EnterInvocation(ctx *InvocationContext) {}

// ExitInvocation is called when production invocation is exited.
func (s *BaseCadenceListener) ExitInvocation(ctx *InvocationContext) {}

// EnterArgument is called when production argument is entered.
func (s *BaseCadenceListener) EnterArgument(ctx *ArgumentContext) {}

// ExitArgument is called when production argument is exited.
func (s *BaseCadenceListener) ExitArgument(ctx *ArgumentContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseCadenceListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseCadenceListener) ExitLiteral(ctx *LiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseCadenceListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseCadenceListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterNilLiteral is called when production nilLiteral is entered.
func (s *BaseCadenceListener) EnterNilLiteral(ctx *NilLiteralContext) {}

// ExitNilLiteral is called when production nilLiteral is exited.
func (s *BaseCadenceListener) ExitNilLiteral(ctx *NilLiteralContext) {}

// EnterPathLiteral is called when production pathLiteral is entered.
func (s *BaseCadenceListener) EnterPathLiteral(ctx *PathLiteralContext) {}

// ExitPathLiteral is called when production pathLiteral is exited.
func (s *BaseCadenceListener) ExitPathLiteral(ctx *PathLiteralContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseCadenceListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseCadenceListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterFixedPointLiteral is called when production fixedPointLiteral is entered.
func (s *BaseCadenceListener) EnterFixedPointLiteral(ctx *FixedPointLiteralContext) {}

// ExitFixedPointLiteral is called when production fixedPointLiteral is exited.
func (s *BaseCadenceListener) ExitFixedPointLiteral(ctx *FixedPointLiteralContext) {}

// EnterIntegerLiteral is called when production integerLiteral is entered.
func (s *BaseCadenceListener) EnterIntegerLiteral(ctx *IntegerLiteralContext) {}

// ExitIntegerLiteral is called when production integerLiteral is exited.
func (s *BaseCadenceListener) ExitIntegerLiteral(ctx *IntegerLiteralContext) {}

// EnterDecimalLiteral is called when production DecimalLiteral is entered.
func (s *BaseCadenceListener) EnterDecimalLiteral(ctx *DecimalLiteralContext) {}

// ExitDecimalLiteral is called when production DecimalLiteral is exited.
func (s *BaseCadenceListener) ExitDecimalLiteral(ctx *DecimalLiteralContext) {}

// EnterBinaryLiteral is called when production BinaryLiteral is entered.
func (s *BaseCadenceListener) EnterBinaryLiteral(ctx *BinaryLiteralContext) {}

// ExitBinaryLiteral is called when production BinaryLiteral is exited.
func (s *BaseCadenceListener) ExitBinaryLiteral(ctx *BinaryLiteralContext) {}

// EnterOctalLiteral is called when production OctalLiteral is entered.
func (s *BaseCadenceListener) EnterOctalLiteral(ctx *OctalLiteralContext) {}

// ExitOctalLiteral is called when production OctalLiteral is exited.
func (s *BaseCadenceListener) ExitOctalLiteral(ctx *OctalLiteralContext) {}

// EnterHexadecimalLiteral is called when production HexadecimalLiteral is entered.
func (s *BaseCadenceListener) EnterHexadecimalLiteral(ctx *HexadecimalLiteralContext) {}

// ExitHexadecimalLiteral is called when production HexadecimalLiteral is exited.
func (s *BaseCadenceListener) ExitHexadecimalLiteral(ctx *HexadecimalLiteralContext) {}

// EnterInvalidNumberLiteral is called when production InvalidNumberLiteral is entered.
func (s *BaseCadenceListener) EnterInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) {}

// ExitInvalidNumberLiteral is called when production InvalidNumberLiteral is exited.
func (s *BaseCadenceListener) ExitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) {}

// EnterArrayLiteral is called when production arrayLiteral is entered.
func (s *BaseCadenceListener) EnterArrayLiteral(ctx *ArrayLiteralContext) {}

// ExitArrayLiteral is called when production arrayLiteral is exited.
func (s *BaseCadenceListener) ExitArrayLiteral(ctx *ArrayLiteralContext) {}

// EnterDictionaryLiteral is called when production dictionaryLiteral is entered.
func (s *BaseCadenceListener) EnterDictionaryLiteral(ctx *DictionaryLiteralContext) {}

// ExitDictionaryLiteral is called when production dictionaryLiteral is exited.
func (s *BaseCadenceListener) ExitDictionaryLiteral(ctx *DictionaryLiteralContext) {}

// EnterDictionaryEntry is called when production dictionaryEntry is entered.
func (s *BaseCadenceListener) EnterDictionaryEntry(ctx *DictionaryEntryContext) {}

// ExitDictionaryEntry is called when production dictionaryEntry is exited.
func (s *BaseCadenceListener) ExitDictionaryEntry(ctx *DictionaryEntryContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseCadenceListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseCadenceListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterEos is called when production eos is entered.
func (s *BaseCadenceListener) EnterEos(ctx *EosContext) {}

// ExitEos is called when production eos is exited.
func (s *BaseCadenceListener) ExitEos(ctx *EosContext) {}

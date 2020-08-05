// Code generated from parser/Cadence.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Cadence
import "github.com/antlr/antlr4/runtime/Go/antlr"

type BaseCadenceVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseCadenceVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReplInput(ctx *ReplInputContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReplElement(ctx *ReplElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReplStatement(ctx *ReplStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReplDeclaration(ctx *ReplDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitTransactionDeclaration(ctx *TransactionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitPrepare(ctx *PrepareContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitExecute(ctx *ExecuteContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitImportDeclaration(ctx *ImportDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAccess(ctx *AccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitConformances(ctx *ConformancesContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitVariableKind(ctx *VariableKindContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitField(ctx *FieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFields(ctx *FieldsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitMembersAndNestedDeclarations(ctx *MembersAndNestedDeclarationsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitMemberOrNestedDeclaration(ctx *MemberOrNestedDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCompositeKind(ctx *CompositeKindContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitEventDeclaration(ctx *EventDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFullType(ctx *FullTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitInnerType(ctx *InnerTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitTypeRestrictions(ctx *TypeRestrictionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitNominalType(ctx *NominalTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitVariableSizedType(ctx *VariableSizedTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitConstantSizedType(ctx *ConstantSizedTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDictionaryType(ctx *DictionaryTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFunctionBlock(ctx *FunctionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitPreConditions(ctx *PreConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitPostConditions(ctx *PostConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitConditions(ctx *ConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCondition(ctx *ConditionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitStatements(ctx *StatementsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitStatement(ctx *StatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBreakStatement(ctx *BreakStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitContinueStatement(ctx *ContinueStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitForStatement(ctx *ForStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitEmitStatement(ctx *EmitStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitSwap(ctx *SwapContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitTransfer(ctx *TransferContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitOrExpression(ctx *OrExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBitwiseOrExpression(ctx *BitwiseOrExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBitwiseXorExpression(ctx *BitwiseXorExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBitwiseAndExpression(ctx *BitwiseAndExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBitwiseShiftExpression(ctx *BitwiseShiftExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCastingExpression(ctx *CastingExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAccessExpression(ctx *AccessExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitInvocationExpression(ctx *InvocationExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitForceExpression(ctx *ForceExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitEqualityOp(ctx *EqualityOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitRelationalOp(ctx *RelationalOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBitwiseShiftOp(ctx *BitwiseShiftOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitAdditiveOp(ctx *AdditiveOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitUnaryOp(ctx *UnaryOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCastingOp(ctx *CastingOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitCreateExpression(ctx *CreateExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDestroyExpression(ctx *DestroyExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitReferenceExpression(ctx *ReferenceExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitExpressionAccess(ctx *ExpressionAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitInvocation(ctx *InvocationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitArgument(ctx *ArgumentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitNilLiteral(ctx *NilLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitPathLiteral(ctx *PathLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitFixedPointLiteral(ctx *FixedPointLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCadenceVisitor) VisitEos(ctx *EosContext) interface{} {
	return v.VisitChildren(ctx)
}

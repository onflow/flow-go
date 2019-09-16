// Code generated from parser/Strictus.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Strictus
import "github.com/antlr/antlr4/runtime/Go/antlr"

type BaseStrictusVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseStrictusVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitImportDeclaration(ctx *ImportDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitAccess(ctx *AccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitConformances(ctx *ConformancesContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitVariableKind(ctx *VariableKindContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitField(ctx *FieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMembers(ctx *MembersContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMember(ctx *MemberContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitCompositeKind(ctx *CompositeKindContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInitializer(ctx *InitializerContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFullType(ctx *FullTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitTypeIndex(ctx *TypeIndexContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFunctionBlock(ctx *FunctionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitPreConditions(ctx *PreConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitPostConditions(ctx *PostConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitConditions(ctx *ConditionsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitCondition(ctx *ConditionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitStatements(ctx *StatementsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitStatement(ctx *StatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBreakStatement(ctx *BreakStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitContinueStatement(ctx *ContinueStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitOrExpression(ctx *OrExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFailableDowncastingExpression(ctx *FailableDowncastingExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitConcatenatingExpression(ctx *ConcatenatingExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitEqualityOp(ctx *EqualityOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitRelationalOp(ctx *RelationalOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitAdditiveOp(ctx *AdditiveOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitUnaryOp(ctx *UnaryOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitExpressionAccess(ctx *ExpressionAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInvocation(ctx *InvocationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitArgument(ctx *ArgumentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitNilLiteral(ctx *NilLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitEos(ctx *EosContext) interface{} {
	return v.VisitChildren(ctx)
}

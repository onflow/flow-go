// Code generated from Strictus.g4 by ANTLR 4.7.2. DO NOT EDIT.

package strictus // Strictus
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

func (v *BaseStrictusVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitTypeName(ctx *TypeNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitTypeDimension(ctx *TypeDimensionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInt32Type(ctx *Int32TypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitInt64Type(ctx *Int64TypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitStatement(ctx *StatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
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

func (v *BaseStrictusVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
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

func (v *BaseStrictusVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
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

func (v *BaseStrictusVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseStrictusVisitor) VisitEos(ctx *EosContext) interface{} {
	return v.VisitChildren(ctx)
}

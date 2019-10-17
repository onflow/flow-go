// Code generated from parser/Strictus.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Strictus
import "github.com/antlr/antlr4/runtime/Go/antlr"

import "strings"

var _ = strings.Builder{}

// A complete Visitor for a parse tree produced by StrictusParser.
type StrictusVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by StrictusParser#program.
	VisitProgram(ctx *ProgramContext) interface{}

	// Visit a parse tree produced by StrictusParser#replInput.
	VisitReplInput(ctx *ReplInputContext) interface{}

	// Visit a parse tree produced by StrictusParser#declaration.
	VisitDeclaration(ctx *DeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#importDeclaration.
	VisitImportDeclaration(ctx *ImportDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#access.
	VisitAccess(ctx *AccessContext) interface{}

	// Visit a parse tree produced by StrictusParser#compositeDeclaration.
	VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#conformances.
	VisitConformances(ctx *ConformancesContext) interface{}

	// Visit a parse tree produced by StrictusParser#variableKind.
	VisitVariableKind(ctx *VariableKindContext) interface{}

	// Visit a parse tree produced by StrictusParser#field.
	VisitField(ctx *FieldContext) interface{}

	// Visit a parse tree produced by StrictusParser#interfaceDeclaration.
	VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#members.
	VisitMembers(ctx *MembersContext) interface{}

	// Visit a parse tree produced by StrictusParser#member.
	VisitMember(ctx *MemberContext) interface{}

	// Visit a parse tree produced by StrictusParser#compositeKind.
	VisitCompositeKind(ctx *CompositeKindContext) interface{}

	// Visit a parse tree produced by StrictusParser#specialFunctionDeclaration.
	VisitSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#functionDeclaration.
	VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#eventDeclaration.
	VisitEventDeclaration(ctx *EventDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#parameterList.
	VisitParameterList(ctx *ParameterListContext) interface{}

	// Visit a parse tree produced by StrictusParser#parameter.
	VisitParameter(ctx *ParameterContext) interface{}

	// Visit a parse tree produced by StrictusParser#typeAnnotation.
	VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{}

	// Visit a parse tree produced by StrictusParser#fullType.
	VisitFullType(ctx *FullTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#baseType.
	VisitBaseType(ctx *BaseTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#nominalType.
	VisitNominalType(ctx *NominalTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#functionType.
	VisitFunctionType(ctx *FunctionTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#variableSizedType.
	VisitVariableSizedType(ctx *VariableSizedTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#constantSizedType.
	VisitConstantSizedType(ctx *ConstantSizedTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#dictionaryType.
	VisitDictionaryType(ctx *DictionaryTypeContext) interface{}

	// Visit a parse tree produced by StrictusParser#block.
	VisitBlock(ctx *BlockContext) interface{}

	// Visit a parse tree produced by StrictusParser#functionBlock.
	VisitFunctionBlock(ctx *FunctionBlockContext) interface{}

	// Visit a parse tree produced by StrictusParser#preConditions.
	VisitPreConditions(ctx *PreConditionsContext) interface{}

	// Visit a parse tree produced by StrictusParser#postConditions.
	VisitPostConditions(ctx *PostConditionsContext) interface{}

	// Visit a parse tree produced by StrictusParser#conditions.
	VisitConditions(ctx *ConditionsContext) interface{}

	// Visit a parse tree produced by StrictusParser#condition.
	VisitCondition(ctx *ConditionContext) interface{}

	// Visit a parse tree produced by StrictusParser#statements.
	VisitStatements(ctx *StatementsContext) interface{}

	// Visit a parse tree produced by StrictusParser#statement.
	VisitStatement(ctx *StatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#returnStatement.
	VisitReturnStatement(ctx *ReturnStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#breakStatement.
	VisitBreakStatement(ctx *BreakStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#continueStatement.
	VisitContinueStatement(ctx *ContinueStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#ifStatement.
	VisitIfStatement(ctx *IfStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#whileStatement.
	VisitWhileStatement(ctx *WhileStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#emitStatement.
	VisitEmitStatement(ctx *EmitStatementContext) interface{}

	// Visit a parse tree produced by StrictusParser#variableDeclaration.
	VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{}

	// Visit a parse tree produced by StrictusParser#assignment.
	VisitAssignment(ctx *AssignmentContext) interface{}

	// Visit a parse tree produced by StrictusParser#swap.
	VisitSwap(ctx *SwapContext) interface{}

	// Visit a parse tree produced by StrictusParser#transfer.
	VisitTransfer(ctx *TransferContext) interface{}

	// Visit a parse tree produced by StrictusParser#expression.
	VisitExpression(ctx *ExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#conditionalExpression.
	VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#orExpression.
	VisitOrExpression(ctx *OrExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#andExpression.
	VisitAndExpression(ctx *AndExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#equalityExpression.
	VisitEqualityExpression(ctx *EqualityExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#relationalExpression.
	VisitRelationalExpression(ctx *RelationalExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#nilCoalescingExpression.
	VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#failableDowncastingExpression.
	VisitFailableDowncastingExpression(ctx *FailableDowncastingExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#concatenatingExpression.
	VisitConcatenatingExpression(ctx *ConcatenatingExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#additiveExpression.
	VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#multiplicativeExpression.
	VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#unaryExpression.
	VisitUnaryExpression(ctx *UnaryExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#primaryExpression.
	VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#composedExpression.
	VisitComposedExpression(ctx *ComposedExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#primaryExpressionSuffix.
	VisitPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) interface{}

	// Visit a parse tree produced by StrictusParser#equalityOp.
	VisitEqualityOp(ctx *EqualityOpContext) interface{}

	// Visit a parse tree produced by StrictusParser#relationalOp.
	VisitRelationalOp(ctx *RelationalOpContext) interface{}

	// Visit a parse tree produced by StrictusParser#additiveOp.
	VisitAdditiveOp(ctx *AdditiveOpContext) interface{}

	// Visit a parse tree produced by StrictusParser#multiplicativeOp.
	VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{}

	// Visit a parse tree produced by StrictusParser#unaryOp.
	VisitUnaryOp(ctx *UnaryOpContext) interface{}

	// Visit a parse tree produced by StrictusParser#primaryExpressionStart.
	VisitPrimaryExpressionStart(ctx *PrimaryExpressionStartContext) interface{}

	// Visit a parse tree produced by StrictusParser#createExpression.
	VisitCreateExpression(ctx *CreateExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#destroyExpression.
	VisitDestroyExpression(ctx *DestroyExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#identifierExpression.
	VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#literalExpression.
	VisitLiteralExpression(ctx *LiteralExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#functionExpression.
	VisitFunctionExpression(ctx *FunctionExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#nestedExpression.
	VisitNestedExpression(ctx *NestedExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#expressionAccess.
	VisitExpressionAccess(ctx *ExpressionAccessContext) interface{}

	// Visit a parse tree produced by StrictusParser#memberAccess.
	VisitMemberAccess(ctx *MemberAccessContext) interface{}

	// Visit a parse tree produced by StrictusParser#bracketExpression.
	VisitBracketExpression(ctx *BracketExpressionContext) interface{}

	// Visit a parse tree produced by StrictusParser#invocation.
	VisitInvocation(ctx *InvocationContext) interface{}

	// Visit a parse tree produced by StrictusParser#argument.
	VisitArgument(ctx *ArgumentContext) interface{}

	// Visit a parse tree produced by StrictusParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#booleanLiteral.
	VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#nilLiteral.
	VisitNilLiteral(ctx *NilLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#stringLiteral.
	VisitStringLiteral(ctx *StringLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#integerLiteral.
	VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#DecimalLiteral.
	VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#BinaryLiteral.
	VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#OctalLiteral.
	VisitOctalLiteral(ctx *OctalLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#HexadecimalLiteral.
	VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#InvalidNumberLiteral.
	VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#arrayLiteral.
	VisitArrayLiteral(ctx *ArrayLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#dictionaryLiteral.
	VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{}

	// Visit a parse tree produced by StrictusParser#dictionaryEntry.
	VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{}

	// Visit a parse tree produced by StrictusParser#identifier.
	VisitIdentifier(ctx *IdentifierContext) interface{}

	// Visit a parse tree produced by StrictusParser#eos.
	VisitEos(ctx *EosContext) interface{}
}

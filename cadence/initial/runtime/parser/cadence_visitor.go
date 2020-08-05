// Code generated from parser/Cadence.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Cadence
import "github.com/antlr/antlr4/runtime/Go/antlr"

import "strings"

var _ = strings.Builder{}

// A complete Visitor for a parse tree produced by CadenceParser.
type CadenceVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by CadenceParser#program.
	VisitProgram(ctx *ProgramContext) interface{}

	// Visit a parse tree produced by CadenceParser#replInput.
	VisitReplInput(ctx *ReplInputContext) interface{}

	// Visit a parse tree produced by CadenceParser#replElement.
	VisitReplElement(ctx *ReplElementContext) interface{}

	// Visit a parse tree produced by CadenceParser#replStatement.
	VisitReplStatement(ctx *ReplStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#replDeclaration.
	VisitReplDeclaration(ctx *ReplDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#declaration.
	VisitDeclaration(ctx *DeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#transactionDeclaration.
	VisitTransactionDeclaration(ctx *TransactionDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#prepare.
	VisitPrepare(ctx *PrepareContext) interface{}

	// Visit a parse tree produced by CadenceParser#execute.
	VisitExecute(ctx *ExecuteContext) interface{}

	// Visit a parse tree produced by CadenceParser#importDeclaration.
	VisitImportDeclaration(ctx *ImportDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#access.
	VisitAccess(ctx *AccessContext) interface{}

	// Visit a parse tree produced by CadenceParser#compositeDeclaration.
	VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#conformances.
	VisitConformances(ctx *ConformancesContext) interface{}

	// Visit a parse tree produced by CadenceParser#variableKind.
	VisitVariableKind(ctx *VariableKindContext) interface{}

	// Visit a parse tree produced by CadenceParser#field.
	VisitField(ctx *FieldContext) interface{}

	// Visit a parse tree produced by CadenceParser#fields.
	VisitFields(ctx *FieldsContext) interface{}

	// Visit a parse tree produced by CadenceParser#interfaceDeclaration.
	VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#membersAndNestedDeclarations.
	VisitMembersAndNestedDeclarations(ctx *MembersAndNestedDeclarationsContext) interface{}

	// Visit a parse tree produced by CadenceParser#memberOrNestedDeclaration.
	VisitMemberOrNestedDeclaration(ctx *MemberOrNestedDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#compositeKind.
	VisitCompositeKind(ctx *CompositeKindContext) interface{}

	// Visit a parse tree produced by CadenceParser#specialFunctionDeclaration.
	VisitSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#functionDeclaration.
	VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#eventDeclaration.
	VisitEventDeclaration(ctx *EventDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#parameterList.
	VisitParameterList(ctx *ParameterListContext) interface{}

	// Visit a parse tree produced by CadenceParser#parameter.
	VisitParameter(ctx *ParameterContext) interface{}

	// Visit a parse tree produced by CadenceParser#typeAnnotation.
	VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{}

	// Visit a parse tree produced by CadenceParser#fullType.
	VisitFullType(ctx *FullTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#innerType.
	VisitInnerType(ctx *InnerTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#baseType.
	VisitBaseType(ctx *BaseTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#typeRestrictions.
	VisitTypeRestrictions(ctx *TypeRestrictionsContext) interface{}

	// Visit a parse tree produced by CadenceParser#nominalType.
	VisitNominalType(ctx *NominalTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#functionType.
	VisitFunctionType(ctx *FunctionTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#variableSizedType.
	VisitVariableSizedType(ctx *VariableSizedTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#constantSizedType.
	VisitConstantSizedType(ctx *ConstantSizedTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#dictionaryType.
	VisitDictionaryType(ctx *DictionaryTypeContext) interface{}

	// Visit a parse tree produced by CadenceParser#block.
	VisitBlock(ctx *BlockContext) interface{}

	// Visit a parse tree produced by CadenceParser#functionBlock.
	VisitFunctionBlock(ctx *FunctionBlockContext) interface{}

	// Visit a parse tree produced by CadenceParser#preConditions.
	VisitPreConditions(ctx *PreConditionsContext) interface{}

	// Visit a parse tree produced by CadenceParser#postConditions.
	VisitPostConditions(ctx *PostConditionsContext) interface{}

	// Visit a parse tree produced by CadenceParser#conditions.
	VisitConditions(ctx *ConditionsContext) interface{}

	// Visit a parse tree produced by CadenceParser#condition.
	VisitCondition(ctx *ConditionContext) interface{}

	// Visit a parse tree produced by CadenceParser#statements.
	VisitStatements(ctx *StatementsContext) interface{}

	// Visit a parse tree produced by CadenceParser#statement.
	VisitStatement(ctx *StatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#returnStatement.
	VisitReturnStatement(ctx *ReturnStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#breakStatement.
	VisitBreakStatement(ctx *BreakStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#continueStatement.
	VisitContinueStatement(ctx *ContinueStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#ifStatement.
	VisitIfStatement(ctx *IfStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#whileStatement.
	VisitWhileStatement(ctx *WhileStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#forStatement.
	VisitForStatement(ctx *ForStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#emitStatement.
	VisitEmitStatement(ctx *EmitStatementContext) interface{}

	// Visit a parse tree produced by CadenceParser#variableDeclaration.
	VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{}

	// Visit a parse tree produced by CadenceParser#assignment.
	VisitAssignment(ctx *AssignmentContext) interface{}

	// Visit a parse tree produced by CadenceParser#swap.
	VisitSwap(ctx *SwapContext) interface{}

	// Visit a parse tree produced by CadenceParser#transfer.
	VisitTransfer(ctx *TransferContext) interface{}

	// Visit a parse tree produced by CadenceParser#expression.
	VisitExpression(ctx *ExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#conditionalExpression.
	VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#orExpression.
	VisitOrExpression(ctx *OrExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#andExpression.
	VisitAndExpression(ctx *AndExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#equalityExpression.
	VisitEqualityExpression(ctx *EqualityExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#relationalExpression.
	VisitRelationalExpression(ctx *RelationalExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#nilCoalescingExpression.
	VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#bitwiseOrExpression.
	VisitBitwiseOrExpression(ctx *BitwiseOrExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#bitwiseXorExpression.
	VisitBitwiseXorExpression(ctx *BitwiseXorExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#bitwiseAndExpression.
	VisitBitwiseAndExpression(ctx *BitwiseAndExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#bitwiseShiftExpression.
	VisitBitwiseShiftExpression(ctx *BitwiseShiftExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#additiveExpression.
	VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#multiplicativeExpression.
	VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#castingExpression.
	VisitCastingExpression(ctx *CastingExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#unaryExpression.
	VisitUnaryExpression(ctx *UnaryExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#primaryExpression.
	VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#accessExpression.
	VisitAccessExpression(ctx *AccessExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#invocationExpression.
	VisitInvocationExpression(ctx *InvocationExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#nestedExpression.
	VisitNestedExpression(ctx *NestedExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#identifierExpression.
	VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#forceExpression.
	VisitForceExpression(ctx *ForceExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#literalExpression.
	VisitLiteralExpression(ctx *LiteralExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#functionExpression.
	VisitFunctionExpression(ctx *FunctionExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#equalityOp.
	VisitEqualityOp(ctx *EqualityOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#relationalOp.
	VisitRelationalOp(ctx *RelationalOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#bitwiseShiftOp.
	VisitBitwiseShiftOp(ctx *BitwiseShiftOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#additiveOp.
	VisitAdditiveOp(ctx *AdditiveOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#multiplicativeOp.
	VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#unaryOp.
	VisitUnaryOp(ctx *UnaryOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#castingOp.
	VisitCastingOp(ctx *CastingOpContext) interface{}

	// Visit a parse tree produced by CadenceParser#createExpression.
	VisitCreateExpression(ctx *CreateExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#destroyExpression.
	VisitDestroyExpression(ctx *DestroyExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#referenceExpression.
	VisitReferenceExpression(ctx *ReferenceExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#expressionAccess.
	VisitExpressionAccess(ctx *ExpressionAccessContext) interface{}

	// Visit a parse tree produced by CadenceParser#memberAccess.
	VisitMemberAccess(ctx *MemberAccessContext) interface{}

	// Visit a parse tree produced by CadenceParser#bracketExpression.
	VisitBracketExpression(ctx *BracketExpressionContext) interface{}

	// Visit a parse tree produced by CadenceParser#invocation.
	VisitInvocation(ctx *InvocationContext) interface{}

	// Visit a parse tree produced by CadenceParser#argument.
	VisitArgument(ctx *ArgumentContext) interface{}

	// Visit a parse tree produced by CadenceParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#booleanLiteral.
	VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#nilLiteral.
	VisitNilLiteral(ctx *NilLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#pathLiteral.
	VisitPathLiteral(ctx *PathLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#stringLiteral.
	VisitStringLiteral(ctx *StringLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#fixedPointLiteral.
	VisitFixedPointLiteral(ctx *FixedPointLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#integerLiteral.
	VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#DecimalLiteral.
	VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#BinaryLiteral.
	VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#OctalLiteral.
	VisitOctalLiteral(ctx *OctalLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#HexadecimalLiteral.
	VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#InvalidNumberLiteral.
	VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#arrayLiteral.
	VisitArrayLiteral(ctx *ArrayLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#dictionaryLiteral.
	VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{}

	// Visit a parse tree produced by CadenceParser#dictionaryEntry.
	VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{}

	// Visit a parse tree produced by CadenceParser#identifier.
	VisitIdentifier(ctx *IdentifierContext) interface{}

	// Visit a parse tree produced by CadenceParser#eos.
	VisitEos(ctx *EosContext) interface{}
}

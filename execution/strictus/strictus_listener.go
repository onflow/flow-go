// Code generated from Strictus.g4 by ANTLR 4.7.2. DO NOT EDIT.

package strictus // Strictus
import "github.com/antlr/antlr4/runtime/Go/antlr"

// StrictusListener is a complete listener for a parse tree produced by StrictusParser.
type StrictusListener interface {
	antlr.ParseTreeListener

	// EnterProgram is called when entering the program production.
	EnterProgram(c *ProgramContext)

	// EnterDeclaration is called when entering the declaration production.
	EnterDeclaration(c *DeclarationContext)

	// EnterFunctionDeclaration is called when entering the functionDeclaration production.
	EnterFunctionDeclaration(c *FunctionDeclarationContext)

	// EnterParameterList is called when entering the parameterList production.
	EnterParameterList(c *ParameterListContext)

	// EnterParameter is called when entering the parameter production.
	EnterParameter(c *ParameterContext)

	// EnterTypeName is called when entering the typeName production.
	EnterTypeName(c *TypeNameContext)

	// EnterTypeDimension is called when entering the typeDimension production.
	EnterTypeDimension(c *TypeDimensionContext)

	// EnterInt32Type is called when entering the Int32Type production.
	EnterInt32Type(c *Int32TypeContext)

	// EnterInt64Type is called when entering the Int64Type production.
	EnterInt64Type(c *Int64TypeContext)

	// EnterBlock is called when entering the block production.
	EnterBlock(c *BlockContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterReturnStatement is called when entering the returnStatement production.
	EnterReturnStatement(c *ReturnStatementContext)

	// EnterIfStatement is called when entering the ifStatement production.
	EnterIfStatement(c *IfStatementContext)

	// EnterWhileStatement is called when entering the whileStatement production.
	EnterWhileStatement(c *WhileStatementContext)

	// EnterVariableDeclaration is called when entering the variableDeclaration production.
	EnterVariableDeclaration(c *VariableDeclarationContext)

	// EnterAssignment is called when entering the assignment production.
	EnterAssignment(c *AssignmentContext)

	// EnterExpression is called when entering the expression production.
	EnterExpression(c *ExpressionContext)

	// EnterConditionalExpression is called when entering the conditionalExpression production.
	EnterConditionalExpression(c *ConditionalExpressionContext)

	// EnterOrExpression is called when entering the orExpression production.
	EnterOrExpression(c *OrExpressionContext)

	// EnterAndExpression is called when entering the andExpression production.
	EnterAndExpression(c *AndExpressionContext)

	// EnterEqualityExpression is called when entering the equalityExpression production.
	EnterEqualityExpression(c *EqualityExpressionContext)

	// EnterRelationalExpression is called when entering the relationalExpression production.
	EnterRelationalExpression(c *RelationalExpressionContext)

	// EnterAdditiveExpression is called when entering the additiveExpression production.
	EnterAdditiveExpression(c *AdditiveExpressionContext)

	// EnterMultiplicativeExpression is called when entering the multiplicativeExpression production.
	EnterMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// EnterPrimaryExpression is called when entering the primaryExpression production.
	EnterPrimaryExpression(c *PrimaryExpressionContext)

	// EnterPrimaryExpressionSuffix is called when entering the primaryExpressionSuffix production.
	EnterPrimaryExpressionSuffix(c *PrimaryExpressionSuffixContext)

	// EnterEqualityOp is called when entering the equalityOp production.
	EnterEqualityOp(c *EqualityOpContext)

	// EnterRelationalOp is called when entering the relationalOp production.
	EnterRelationalOp(c *RelationalOpContext)

	// EnterAdditiveOp is called when entering the additiveOp production.
	EnterAdditiveOp(c *AdditiveOpContext)

	// EnterMultiplicativeOp is called when entering the multiplicativeOp production.
	EnterMultiplicativeOp(c *MultiplicativeOpContext)

	// EnterIdentifierExpression is called when entering the IdentifierExpression production.
	EnterIdentifierExpression(c *IdentifierExpressionContext)

	// EnterLiteralExpression is called when entering the LiteralExpression production.
	EnterLiteralExpression(c *LiteralExpressionContext)

	// EnterFunctionExpression is called when entering the FunctionExpression production.
	EnterFunctionExpression(c *FunctionExpressionContext)

	// EnterNestedExpression is called when entering the NestedExpression production.
	EnterNestedExpression(c *NestedExpressionContext)

	// EnterExpressionAccess is called when entering the expressionAccess production.
	EnterExpressionAccess(c *ExpressionAccessContext)

	// EnterMemberAccess is called when entering the memberAccess production.
	EnterMemberAccess(c *MemberAccessContext)

	// EnterBracketExpression is called when entering the bracketExpression production.
	EnterBracketExpression(c *BracketExpressionContext)

	// EnterInvocation is called when entering the invocation production.
	EnterInvocation(c *InvocationContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterDecimalLiteral is called when entering the DecimalLiteral production.
	EnterDecimalLiteral(c *DecimalLiteralContext)

	// EnterBinaryLiteral is called when entering the BinaryLiteral production.
	EnterBinaryLiteral(c *BinaryLiteralContext)

	// EnterOctalLiteral is called when entering the OctalLiteral production.
	EnterOctalLiteral(c *OctalLiteralContext)

	// EnterHexadecimalLiteral is called when entering the HexadecimalLiteral production.
	EnterHexadecimalLiteral(c *HexadecimalLiteralContext)

	// EnterArrayLiteral is called when entering the arrayLiteral production.
	EnterArrayLiteral(c *ArrayLiteralContext)

	// EnterEos is called when entering the eos production.
	EnterEos(c *EosContext)

	// ExitProgram is called when exiting the program production.
	ExitProgram(c *ProgramContext)

	// ExitDeclaration is called when exiting the declaration production.
	ExitDeclaration(c *DeclarationContext)

	// ExitFunctionDeclaration is called when exiting the functionDeclaration production.
	ExitFunctionDeclaration(c *FunctionDeclarationContext)

	// ExitParameterList is called when exiting the parameterList production.
	ExitParameterList(c *ParameterListContext)

	// ExitParameter is called when exiting the parameter production.
	ExitParameter(c *ParameterContext)

	// ExitTypeName is called when exiting the typeName production.
	ExitTypeName(c *TypeNameContext)

	// ExitTypeDimension is called when exiting the typeDimension production.
	ExitTypeDimension(c *TypeDimensionContext)

	// ExitInt32Type is called when exiting the Int32Type production.
	ExitInt32Type(c *Int32TypeContext)

	// ExitInt64Type is called when exiting the Int64Type production.
	ExitInt64Type(c *Int64TypeContext)

	// ExitBlock is called when exiting the block production.
	ExitBlock(c *BlockContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitReturnStatement is called when exiting the returnStatement production.
	ExitReturnStatement(c *ReturnStatementContext)

	// ExitIfStatement is called when exiting the ifStatement production.
	ExitIfStatement(c *IfStatementContext)

	// ExitWhileStatement is called when exiting the whileStatement production.
	ExitWhileStatement(c *WhileStatementContext)

	// ExitVariableDeclaration is called when exiting the variableDeclaration production.
	ExitVariableDeclaration(c *VariableDeclarationContext)

	// ExitAssignment is called when exiting the assignment production.
	ExitAssignment(c *AssignmentContext)

	// ExitExpression is called when exiting the expression production.
	ExitExpression(c *ExpressionContext)

	// ExitConditionalExpression is called when exiting the conditionalExpression production.
	ExitConditionalExpression(c *ConditionalExpressionContext)

	// ExitOrExpression is called when exiting the orExpression production.
	ExitOrExpression(c *OrExpressionContext)

	// ExitAndExpression is called when exiting the andExpression production.
	ExitAndExpression(c *AndExpressionContext)

	// ExitEqualityExpression is called when exiting the equalityExpression production.
	ExitEqualityExpression(c *EqualityExpressionContext)

	// ExitRelationalExpression is called when exiting the relationalExpression production.
	ExitRelationalExpression(c *RelationalExpressionContext)

	// ExitAdditiveExpression is called when exiting the additiveExpression production.
	ExitAdditiveExpression(c *AdditiveExpressionContext)

	// ExitMultiplicativeExpression is called when exiting the multiplicativeExpression production.
	ExitMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// ExitPrimaryExpression is called when exiting the primaryExpression production.
	ExitPrimaryExpression(c *PrimaryExpressionContext)

	// ExitPrimaryExpressionSuffix is called when exiting the primaryExpressionSuffix production.
	ExitPrimaryExpressionSuffix(c *PrimaryExpressionSuffixContext)

	// ExitEqualityOp is called when exiting the equalityOp production.
	ExitEqualityOp(c *EqualityOpContext)

	// ExitRelationalOp is called when exiting the relationalOp production.
	ExitRelationalOp(c *RelationalOpContext)

	// ExitAdditiveOp is called when exiting the additiveOp production.
	ExitAdditiveOp(c *AdditiveOpContext)

	// ExitMultiplicativeOp is called when exiting the multiplicativeOp production.
	ExitMultiplicativeOp(c *MultiplicativeOpContext)

	// ExitIdentifierExpression is called when exiting the IdentifierExpression production.
	ExitIdentifierExpression(c *IdentifierExpressionContext)

	// ExitLiteralExpression is called when exiting the LiteralExpression production.
	ExitLiteralExpression(c *LiteralExpressionContext)

	// ExitFunctionExpression is called when exiting the FunctionExpression production.
	ExitFunctionExpression(c *FunctionExpressionContext)

	// ExitNestedExpression is called when exiting the NestedExpression production.
	ExitNestedExpression(c *NestedExpressionContext)

	// ExitExpressionAccess is called when exiting the expressionAccess production.
	ExitExpressionAccess(c *ExpressionAccessContext)

	// ExitMemberAccess is called when exiting the memberAccess production.
	ExitMemberAccess(c *MemberAccessContext)

	// ExitBracketExpression is called when exiting the bracketExpression production.
	ExitBracketExpression(c *BracketExpressionContext)

	// ExitInvocation is called when exiting the invocation production.
	ExitInvocation(c *InvocationContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitDecimalLiteral is called when exiting the DecimalLiteral production.
	ExitDecimalLiteral(c *DecimalLiteralContext)

	// ExitBinaryLiteral is called when exiting the BinaryLiteral production.
	ExitBinaryLiteral(c *BinaryLiteralContext)

	// ExitOctalLiteral is called when exiting the OctalLiteral production.
	ExitOctalLiteral(c *OctalLiteralContext)

	// ExitHexadecimalLiteral is called when exiting the HexadecimalLiteral production.
	ExitHexadecimalLiteral(c *HexadecimalLiteralContext)

	// ExitArrayLiteral is called when exiting the arrayLiteral production.
	ExitArrayLiteral(c *ArrayLiteralContext)

	// ExitEos is called when exiting the eos production.
	ExitEos(c *EosContext)
}

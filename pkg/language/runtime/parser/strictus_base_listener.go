// Code generated from parser/Strictus.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Strictus
import "github.com/antlr/antlr4/runtime/Go/antlr"

// BaseStrictusListener is a complete listener for a parse tree produced by StrictusParser.
type BaseStrictusListener struct{}

var _ StrictusListener = &BaseStrictusListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseStrictusListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseStrictusListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseStrictusListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseStrictusListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterProgram is called when production program is entered.
func (s *BaseStrictusListener) EnterProgram(ctx *ProgramContext) {}

// ExitProgram is called when production program is exited.
func (s *BaseStrictusListener) ExitProgram(ctx *ProgramContext) {}

// EnterDeclaration is called when production declaration is entered.
func (s *BaseStrictusListener) EnterDeclaration(ctx *DeclarationContext) {}

// ExitDeclaration is called when production declaration is exited.
func (s *BaseStrictusListener) ExitDeclaration(ctx *DeclarationContext) {}

// EnterImportDeclaration is called when production importDeclaration is entered.
func (s *BaseStrictusListener) EnterImportDeclaration(ctx *ImportDeclarationContext) {}

// ExitImportDeclaration is called when production importDeclaration is exited.
func (s *BaseStrictusListener) ExitImportDeclaration(ctx *ImportDeclarationContext) {}

// EnterAccess is called when production access is entered.
func (s *BaseStrictusListener) EnterAccess(ctx *AccessContext) {}

// ExitAccess is called when production access is exited.
func (s *BaseStrictusListener) ExitAccess(ctx *AccessContext) {}

// EnterCompositeDeclaration is called when production compositeDeclaration is entered.
func (s *BaseStrictusListener) EnterCompositeDeclaration(ctx *CompositeDeclarationContext) {}

// ExitCompositeDeclaration is called when production compositeDeclaration is exited.
func (s *BaseStrictusListener) ExitCompositeDeclaration(ctx *CompositeDeclarationContext) {}

// EnterConformances is called when production conformances is entered.
func (s *BaseStrictusListener) EnterConformances(ctx *ConformancesContext) {}

// ExitConformances is called when production conformances is exited.
func (s *BaseStrictusListener) ExitConformances(ctx *ConformancesContext) {}

// EnterVariableKind is called when production variableKind is entered.
func (s *BaseStrictusListener) EnterVariableKind(ctx *VariableKindContext) {}

// ExitVariableKind is called when production variableKind is exited.
func (s *BaseStrictusListener) ExitVariableKind(ctx *VariableKindContext) {}

// EnterField is called when production field is entered.
func (s *BaseStrictusListener) EnterField(ctx *FieldContext) {}

// ExitField is called when production field is exited.
func (s *BaseStrictusListener) ExitField(ctx *FieldContext) {}

// EnterInterfaceDeclaration is called when production interfaceDeclaration is entered.
func (s *BaseStrictusListener) EnterInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// ExitInterfaceDeclaration is called when production interfaceDeclaration is exited.
func (s *BaseStrictusListener) ExitInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// EnterMembers is called when production members is entered.
func (s *BaseStrictusListener) EnterMembers(ctx *MembersContext) {}

// ExitMembers is called when production members is exited.
func (s *BaseStrictusListener) ExitMembers(ctx *MembersContext) {}

// EnterMember is called when production member is entered.
func (s *BaseStrictusListener) EnterMember(ctx *MemberContext) {}

// ExitMember is called when production member is exited.
func (s *BaseStrictusListener) ExitMember(ctx *MemberContext) {}

// EnterCompositeKind is called when production compositeKind is entered.
func (s *BaseStrictusListener) EnterCompositeKind(ctx *CompositeKindContext) {}

// ExitCompositeKind is called when production compositeKind is exited.
func (s *BaseStrictusListener) ExitCompositeKind(ctx *CompositeKindContext) {}

// EnterInitializer is called when production initializer is entered.
func (s *BaseStrictusListener) EnterInitializer(ctx *InitializerContext) {}

// ExitInitializer is called when production initializer is exited.
func (s *BaseStrictusListener) ExitInitializer(ctx *InitializerContext) {}

// EnterFunctionDeclaration is called when production functionDeclaration is entered.
func (s *BaseStrictusListener) EnterFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// ExitFunctionDeclaration is called when production functionDeclaration is exited.
func (s *BaseStrictusListener) ExitFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// EnterParameterList is called when production parameterList is entered.
func (s *BaseStrictusListener) EnterParameterList(ctx *ParameterListContext) {}

// ExitParameterList is called when production parameterList is exited.
func (s *BaseStrictusListener) ExitParameterList(ctx *ParameterListContext) {}

// EnterParameter is called when production parameter is entered.
func (s *BaseStrictusListener) EnterParameter(ctx *ParameterContext) {}

// ExitParameter is called when production parameter is exited.
func (s *BaseStrictusListener) ExitParameter(ctx *ParameterContext) {}

// EnterTypeAnnotation is called when production typeAnnotation is entered.
func (s *BaseStrictusListener) EnterTypeAnnotation(ctx *TypeAnnotationContext) {}

// ExitTypeAnnotation is called when production typeAnnotation is exited.
func (s *BaseStrictusListener) ExitTypeAnnotation(ctx *TypeAnnotationContext) {}

// EnterFullType is called when production fullType is entered.
func (s *BaseStrictusListener) EnterFullType(ctx *FullTypeContext) {}

// ExitFullType is called when production fullType is exited.
func (s *BaseStrictusListener) ExitFullType(ctx *FullTypeContext) {}

// EnterBaseType is called when production baseType is entered.
func (s *BaseStrictusListener) EnterBaseType(ctx *BaseTypeContext) {}

// ExitBaseType is called when production baseType is exited.
func (s *BaseStrictusListener) ExitBaseType(ctx *BaseTypeContext) {}

// EnterNominalType is called when production nominalType is entered.
func (s *BaseStrictusListener) EnterNominalType(ctx *NominalTypeContext) {}

// ExitNominalType is called when production nominalType is exited.
func (s *BaseStrictusListener) ExitNominalType(ctx *NominalTypeContext) {}

// EnterFunctionType is called when production functionType is entered.
func (s *BaseStrictusListener) EnterFunctionType(ctx *FunctionTypeContext) {}

// ExitFunctionType is called when production functionType is exited.
func (s *BaseStrictusListener) ExitFunctionType(ctx *FunctionTypeContext) {}

// EnterVariableSizedType is called when production variableSizedType is entered.
func (s *BaseStrictusListener) EnterVariableSizedType(ctx *VariableSizedTypeContext) {}

// ExitVariableSizedType is called when production variableSizedType is exited.
func (s *BaseStrictusListener) ExitVariableSizedType(ctx *VariableSizedTypeContext) {}

// EnterConstantSizedType is called when production constantSizedType is entered.
func (s *BaseStrictusListener) EnterConstantSizedType(ctx *ConstantSizedTypeContext) {}

// ExitConstantSizedType is called when production constantSizedType is exited.
func (s *BaseStrictusListener) ExitConstantSizedType(ctx *ConstantSizedTypeContext) {}

// EnterDictionaryType is called when production dictionaryType is entered.
func (s *BaseStrictusListener) EnterDictionaryType(ctx *DictionaryTypeContext) {}

// ExitDictionaryType is called when production dictionaryType is exited.
func (s *BaseStrictusListener) ExitDictionaryType(ctx *DictionaryTypeContext) {}

// EnterBlock is called when production block is entered.
func (s *BaseStrictusListener) EnterBlock(ctx *BlockContext) {}

// ExitBlock is called when production block is exited.
func (s *BaseStrictusListener) ExitBlock(ctx *BlockContext) {}

// EnterFunctionBlock is called when production functionBlock is entered.
func (s *BaseStrictusListener) EnterFunctionBlock(ctx *FunctionBlockContext) {}

// ExitFunctionBlock is called when production functionBlock is exited.
func (s *BaseStrictusListener) ExitFunctionBlock(ctx *FunctionBlockContext) {}

// EnterPreConditions is called when production preConditions is entered.
func (s *BaseStrictusListener) EnterPreConditions(ctx *PreConditionsContext) {}

// ExitPreConditions is called when production preConditions is exited.
func (s *BaseStrictusListener) ExitPreConditions(ctx *PreConditionsContext) {}

// EnterPostConditions is called when production postConditions is entered.
func (s *BaseStrictusListener) EnterPostConditions(ctx *PostConditionsContext) {}

// ExitPostConditions is called when production postConditions is exited.
func (s *BaseStrictusListener) ExitPostConditions(ctx *PostConditionsContext) {}

// EnterConditions is called when production conditions is entered.
func (s *BaseStrictusListener) EnterConditions(ctx *ConditionsContext) {}

// ExitConditions is called when production conditions is exited.
func (s *BaseStrictusListener) ExitConditions(ctx *ConditionsContext) {}

// EnterCondition is called when production condition is entered.
func (s *BaseStrictusListener) EnterCondition(ctx *ConditionContext) {}

// ExitCondition is called when production condition is exited.
func (s *BaseStrictusListener) ExitCondition(ctx *ConditionContext) {}

// EnterStatements is called when production statements is entered.
func (s *BaseStrictusListener) EnterStatements(ctx *StatementsContext) {}

// ExitStatements is called when production statements is exited.
func (s *BaseStrictusListener) ExitStatements(ctx *StatementsContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseStrictusListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseStrictusListener) ExitStatement(ctx *StatementContext) {}

// EnterReturnStatement is called when production returnStatement is entered.
func (s *BaseStrictusListener) EnterReturnStatement(ctx *ReturnStatementContext) {}

// ExitReturnStatement is called when production returnStatement is exited.
func (s *BaseStrictusListener) ExitReturnStatement(ctx *ReturnStatementContext) {}

// EnterBreakStatement is called when production breakStatement is entered.
func (s *BaseStrictusListener) EnterBreakStatement(ctx *BreakStatementContext) {}

// ExitBreakStatement is called when production breakStatement is exited.
func (s *BaseStrictusListener) ExitBreakStatement(ctx *BreakStatementContext) {}

// EnterContinueStatement is called when production continueStatement is entered.
func (s *BaseStrictusListener) EnterContinueStatement(ctx *ContinueStatementContext) {}

// ExitContinueStatement is called when production continueStatement is exited.
func (s *BaseStrictusListener) ExitContinueStatement(ctx *ContinueStatementContext) {}

// EnterIfStatement is called when production ifStatement is entered.
func (s *BaseStrictusListener) EnterIfStatement(ctx *IfStatementContext) {}

// ExitIfStatement is called when production ifStatement is exited.
func (s *BaseStrictusListener) ExitIfStatement(ctx *IfStatementContext) {}

// EnterWhileStatement is called when production whileStatement is entered.
func (s *BaseStrictusListener) EnterWhileStatement(ctx *WhileStatementContext) {}

// ExitWhileStatement is called when production whileStatement is exited.
func (s *BaseStrictusListener) ExitWhileStatement(ctx *WhileStatementContext) {}

// EnterVariableDeclaration is called when production variableDeclaration is entered.
func (s *BaseStrictusListener) EnterVariableDeclaration(ctx *VariableDeclarationContext) {}

// ExitVariableDeclaration is called when production variableDeclaration is exited.
func (s *BaseStrictusListener) ExitVariableDeclaration(ctx *VariableDeclarationContext) {}

// EnterAssignment is called when production assignment is entered.
func (s *BaseStrictusListener) EnterAssignment(ctx *AssignmentContext) {}

// ExitAssignment is called when production assignment is exited.
func (s *BaseStrictusListener) ExitAssignment(ctx *AssignmentContext) {}

// EnterTransfer is called when production transfer is entered.
func (s *BaseStrictusListener) EnterTransfer(ctx *TransferContext) {}

// ExitTransfer is called when production transfer is exited.
func (s *BaseStrictusListener) ExitTransfer(ctx *TransferContext) {}

// EnterExpression is called when production expression is entered.
func (s *BaseStrictusListener) EnterExpression(ctx *ExpressionContext) {}

// ExitExpression is called when production expression is exited.
func (s *BaseStrictusListener) ExitExpression(ctx *ExpressionContext) {}

// EnterConditionalExpression is called when production conditionalExpression is entered.
func (s *BaseStrictusListener) EnterConditionalExpression(ctx *ConditionalExpressionContext) {}

// ExitConditionalExpression is called when production conditionalExpression is exited.
func (s *BaseStrictusListener) ExitConditionalExpression(ctx *ConditionalExpressionContext) {}

// EnterOrExpression is called when production orExpression is entered.
func (s *BaseStrictusListener) EnterOrExpression(ctx *OrExpressionContext) {}

// ExitOrExpression is called when production orExpression is exited.
func (s *BaseStrictusListener) ExitOrExpression(ctx *OrExpressionContext) {}

// EnterAndExpression is called when production andExpression is entered.
func (s *BaseStrictusListener) EnterAndExpression(ctx *AndExpressionContext) {}

// ExitAndExpression is called when production andExpression is exited.
func (s *BaseStrictusListener) ExitAndExpression(ctx *AndExpressionContext) {}

// EnterEqualityExpression is called when production equalityExpression is entered.
func (s *BaseStrictusListener) EnterEqualityExpression(ctx *EqualityExpressionContext) {}

// ExitEqualityExpression is called when production equalityExpression is exited.
func (s *BaseStrictusListener) ExitEqualityExpression(ctx *EqualityExpressionContext) {}

// EnterRelationalExpression is called when production relationalExpression is entered.
func (s *BaseStrictusListener) EnterRelationalExpression(ctx *RelationalExpressionContext) {}

// ExitRelationalExpression is called when production relationalExpression is exited.
func (s *BaseStrictusListener) ExitRelationalExpression(ctx *RelationalExpressionContext) {}

// EnterNilCoalescingExpression is called when production nilCoalescingExpression is entered.
func (s *BaseStrictusListener) EnterNilCoalescingExpression(ctx *NilCoalescingExpressionContext) {}

// ExitNilCoalescingExpression is called when production nilCoalescingExpression is exited.
func (s *BaseStrictusListener) ExitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) {}

// EnterFailableDowncastingExpression is called when production failableDowncastingExpression is entered.
func (s *BaseStrictusListener) EnterFailableDowncastingExpression(ctx *FailableDowncastingExpressionContext) {
}

// ExitFailableDowncastingExpression is called when production failableDowncastingExpression is exited.
func (s *BaseStrictusListener) ExitFailableDowncastingExpression(ctx *FailableDowncastingExpressionContext) {
}

// EnterConcatenatingExpression is called when production concatenatingExpression is entered.
func (s *BaseStrictusListener) EnterConcatenatingExpression(ctx *ConcatenatingExpressionContext) {}

// ExitConcatenatingExpression is called when production concatenatingExpression is exited.
func (s *BaseStrictusListener) ExitConcatenatingExpression(ctx *ConcatenatingExpressionContext) {}

// EnterAdditiveExpression is called when production additiveExpression is entered.
func (s *BaseStrictusListener) EnterAdditiveExpression(ctx *AdditiveExpressionContext) {}

// ExitAdditiveExpression is called when production additiveExpression is exited.
func (s *BaseStrictusListener) ExitAdditiveExpression(ctx *AdditiveExpressionContext) {}

// EnterMultiplicativeExpression is called when production multiplicativeExpression is entered.
func (s *BaseStrictusListener) EnterMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// ExitMultiplicativeExpression is called when production multiplicativeExpression is exited.
func (s *BaseStrictusListener) ExitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// EnterUnaryExpression is called when production unaryExpression is entered.
func (s *BaseStrictusListener) EnterUnaryExpression(ctx *UnaryExpressionContext) {}

// ExitUnaryExpression is called when production unaryExpression is exited.
func (s *BaseStrictusListener) ExitUnaryExpression(ctx *UnaryExpressionContext) {}

// EnterPrimaryExpression is called when production primaryExpression is entered.
func (s *BaseStrictusListener) EnterPrimaryExpression(ctx *PrimaryExpressionContext) {}

// ExitPrimaryExpression is called when production primaryExpression is exited.
func (s *BaseStrictusListener) ExitPrimaryExpression(ctx *PrimaryExpressionContext) {}

// EnterPrimaryExpressionSuffix is called when production primaryExpressionSuffix is entered.
func (s *BaseStrictusListener) EnterPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) {}

// ExitPrimaryExpressionSuffix is called when production primaryExpressionSuffix is exited.
func (s *BaseStrictusListener) ExitPrimaryExpressionSuffix(ctx *PrimaryExpressionSuffixContext) {}

// EnterEqualityOp is called when production equalityOp is entered.
func (s *BaseStrictusListener) EnterEqualityOp(ctx *EqualityOpContext) {}

// ExitEqualityOp is called when production equalityOp is exited.
func (s *BaseStrictusListener) ExitEqualityOp(ctx *EqualityOpContext) {}

// EnterRelationalOp is called when production relationalOp is entered.
func (s *BaseStrictusListener) EnterRelationalOp(ctx *RelationalOpContext) {}

// ExitRelationalOp is called when production relationalOp is exited.
func (s *BaseStrictusListener) ExitRelationalOp(ctx *RelationalOpContext) {}

// EnterAdditiveOp is called when production additiveOp is entered.
func (s *BaseStrictusListener) EnterAdditiveOp(ctx *AdditiveOpContext) {}

// ExitAdditiveOp is called when production additiveOp is exited.
func (s *BaseStrictusListener) ExitAdditiveOp(ctx *AdditiveOpContext) {}

// EnterMultiplicativeOp is called when production multiplicativeOp is entered.
func (s *BaseStrictusListener) EnterMultiplicativeOp(ctx *MultiplicativeOpContext) {}

// ExitMultiplicativeOp is called when production multiplicativeOp is exited.
func (s *BaseStrictusListener) ExitMultiplicativeOp(ctx *MultiplicativeOpContext) {}

// EnterUnaryOp is called when production unaryOp is entered.
func (s *BaseStrictusListener) EnterUnaryOp(ctx *UnaryOpContext) {}

// ExitUnaryOp is called when production unaryOp is exited.
func (s *BaseStrictusListener) ExitUnaryOp(ctx *UnaryOpContext) {}

// EnterCreateExpression is called when production CreateExpression is entered.
func (s *BaseStrictusListener) EnterCreateExpression(ctx *CreateExpressionContext) {}

// ExitCreateExpression is called when production CreateExpression is exited.
func (s *BaseStrictusListener) ExitCreateExpression(ctx *CreateExpressionContext) {}

// EnterDestroyExpression is called when production DestroyExpression is entered.
func (s *BaseStrictusListener) EnterDestroyExpression(ctx *DestroyExpressionContext) {}

// ExitDestroyExpression is called when production DestroyExpression is exited.
func (s *BaseStrictusListener) ExitDestroyExpression(ctx *DestroyExpressionContext) {}

// EnterIdentifierExpression is called when production IdentifierExpression is entered.
func (s *BaseStrictusListener) EnterIdentifierExpression(ctx *IdentifierExpressionContext) {}

// ExitIdentifierExpression is called when production IdentifierExpression is exited.
func (s *BaseStrictusListener) ExitIdentifierExpression(ctx *IdentifierExpressionContext) {}

// EnterLiteralExpression is called when production LiteralExpression is entered.
func (s *BaseStrictusListener) EnterLiteralExpression(ctx *LiteralExpressionContext) {}

// ExitLiteralExpression is called when production LiteralExpression is exited.
func (s *BaseStrictusListener) ExitLiteralExpression(ctx *LiteralExpressionContext) {}

// EnterFunctionExpression is called when production FunctionExpression is entered.
func (s *BaseStrictusListener) EnterFunctionExpression(ctx *FunctionExpressionContext) {}

// ExitFunctionExpression is called when production FunctionExpression is exited.
func (s *BaseStrictusListener) ExitFunctionExpression(ctx *FunctionExpressionContext) {}

// EnterNestedExpression is called when production NestedExpression is entered.
func (s *BaseStrictusListener) EnterNestedExpression(ctx *NestedExpressionContext) {}

// ExitNestedExpression is called when production NestedExpression is exited.
func (s *BaseStrictusListener) ExitNestedExpression(ctx *NestedExpressionContext) {}

// EnterExpressionAccess is called when production expressionAccess is entered.
func (s *BaseStrictusListener) EnterExpressionAccess(ctx *ExpressionAccessContext) {}

// ExitExpressionAccess is called when production expressionAccess is exited.
func (s *BaseStrictusListener) ExitExpressionAccess(ctx *ExpressionAccessContext) {}

// EnterMemberAccess is called when production memberAccess is entered.
func (s *BaseStrictusListener) EnterMemberAccess(ctx *MemberAccessContext) {}

// ExitMemberAccess is called when production memberAccess is exited.
func (s *BaseStrictusListener) ExitMemberAccess(ctx *MemberAccessContext) {}

// EnterBracketExpression is called when production bracketExpression is entered.
func (s *BaseStrictusListener) EnterBracketExpression(ctx *BracketExpressionContext) {}

// ExitBracketExpression is called when production bracketExpression is exited.
func (s *BaseStrictusListener) ExitBracketExpression(ctx *BracketExpressionContext) {}

// EnterInvocation is called when production invocation is entered.
func (s *BaseStrictusListener) EnterInvocation(ctx *InvocationContext) {}

// ExitInvocation is called when production invocation is exited.
func (s *BaseStrictusListener) ExitInvocation(ctx *InvocationContext) {}

// EnterArgument is called when production argument is entered.
func (s *BaseStrictusListener) EnterArgument(ctx *ArgumentContext) {}

// ExitArgument is called when production argument is exited.
func (s *BaseStrictusListener) ExitArgument(ctx *ArgumentContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseStrictusListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseStrictusListener) ExitLiteral(ctx *LiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseStrictusListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseStrictusListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterNilLiteral is called when production nilLiteral is entered.
func (s *BaseStrictusListener) EnterNilLiteral(ctx *NilLiteralContext) {}

// ExitNilLiteral is called when production nilLiteral is exited.
func (s *BaseStrictusListener) ExitNilLiteral(ctx *NilLiteralContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseStrictusListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseStrictusListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterIntegerLiteral is called when production integerLiteral is entered.
func (s *BaseStrictusListener) EnterIntegerLiteral(ctx *IntegerLiteralContext) {}

// ExitIntegerLiteral is called when production integerLiteral is exited.
func (s *BaseStrictusListener) ExitIntegerLiteral(ctx *IntegerLiteralContext) {}

// EnterDecimalLiteral is called when production DecimalLiteral is entered.
func (s *BaseStrictusListener) EnterDecimalLiteral(ctx *DecimalLiteralContext) {}

// ExitDecimalLiteral is called when production DecimalLiteral is exited.
func (s *BaseStrictusListener) ExitDecimalLiteral(ctx *DecimalLiteralContext) {}

// EnterBinaryLiteral is called when production BinaryLiteral is entered.
func (s *BaseStrictusListener) EnterBinaryLiteral(ctx *BinaryLiteralContext) {}

// ExitBinaryLiteral is called when production BinaryLiteral is exited.
func (s *BaseStrictusListener) ExitBinaryLiteral(ctx *BinaryLiteralContext) {}

// EnterOctalLiteral is called when production OctalLiteral is entered.
func (s *BaseStrictusListener) EnterOctalLiteral(ctx *OctalLiteralContext) {}

// ExitOctalLiteral is called when production OctalLiteral is exited.
func (s *BaseStrictusListener) ExitOctalLiteral(ctx *OctalLiteralContext) {}

// EnterHexadecimalLiteral is called when production HexadecimalLiteral is entered.
func (s *BaseStrictusListener) EnterHexadecimalLiteral(ctx *HexadecimalLiteralContext) {}

// ExitHexadecimalLiteral is called when production HexadecimalLiteral is exited.
func (s *BaseStrictusListener) ExitHexadecimalLiteral(ctx *HexadecimalLiteralContext) {}

// EnterInvalidNumberLiteral is called when production InvalidNumberLiteral is entered.
func (s *BaseStrictusListener) EnterInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) {}

// ExitInvalidNumberLiteral is called when production InvalidNumberLiteral is exited.
func (s *BaseStrictusListener) ExitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) {}

// EnterArrayLiteral is called when production arrayLiteral is entered.
func (s *BaseStrictusListener) EnterArrayLiteral(ctx *ArrayLiteralContext) {}

// ExitArrayLiteral is called when production arrayLiteral is exited.
func (s *BaseStrictusListener) ExitArrayLiteral(ctx *ArrayLiteralContext) {}

// EnterDictionaryLiteral is called when production dictionaryLiteral is entered.
func (s *BaseStrictusListener) EnterDictionaryLiteral(ctx *DictionaryLiteralContext) {}

// ExitDictionaryLiteral is called when production dictionaryLiteral is exited.
func (s *BaseStrictusListener) ExitDictionaryLiteral(ctx *DictionaryLiteralContext) {}

// EnterDictionaryEntry is called when production dictionaryEntry is entered.
func (s *BaseStrictusListener) EnterDictionaryEntry(ctx *DictionaryEntryContext) {}

// ExitDictionaryEntry is called when production dictionaryEntry is exited.
func (s *BaseStrictusListener) ExitDictionaryEntry(ctx *DictionaryEntryContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseStrictusListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseStrictusListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterEos is called when production eos is entered.
func (s *BaseStrictusListener) EnterEos(ctx *EosContext) {}

// ExitEos is called when production eos is exited.
func (s *BaseStrictusListener) ExitEos(ctx *EosContext) {}

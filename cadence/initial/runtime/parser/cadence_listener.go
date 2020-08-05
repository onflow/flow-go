// Code generated from parser/Cadence.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // Cadence
import "github.com/antlr/antlr4/runtime/Go/antlr"

// CadenceListener is a complete listener for a parse tree produced by CadenceParser.
type CadenceListener interface {
	antlr.ParseTreeListener

	// EnterProgram is called when entering the program production.
	EnterProgram(c *ProgramContext)

	// EnterReplInput is called when entering the replInput production.
	EnterReplInput(c *ReplInputContext)

	// EnterReplElement is called when entering the replElement production.
	EnterReplElement(c *ReplElementContext)

	// EnterReplStatement is called when entering the replStatement production.
	EnterReplStatement(c *ReplStatementContext)

	// EnterReplDeclaration is called when entering the replDeclaration production.
	EnterReplDeclaration(c *ReplDeclarationContext)

	// EnterDeclaration is called when entering the declaration production.
	EnterDeclaration(c *DeclarationContext)

	// EnterTransactionDeclaration is called when entering the transactionDeclaration production.
	EnterTransactionDeclaration(c *TransactionDeclarationContext)

	// EnterPrepare is called when entering the prepare production.
	EnterPrepare(c *PrepareContext)

	// EnterExecute is called when entering the execute production.
	EnterExecute(c *ExecuteContext)

	// EnterImportDeclaration is called when entering the importDeclaration production.
	EnterImportDeclaration(c *ImportDeclarationContext)

	// EnterAccess is called when entering the access production.
	EnterAccess(c *AccessContext)

	// EnterCompositeDeclaration is called when entering the compositeDeclaration production.
	EnterCompositeDeclaration(c *CompositeDeclarationContext)

	// EnterConformances is called when entering the conformances production.
	EnterConformances(c *ConformancesContext)

	// EnterVariableKind is called when entering the variableKind production.
	EnterVariableKind(c *VariableKindContext)

	// EnterField is called when entering the field production.
	EnterField(c *FieldContext)

	// EnterFields is called when entering the fields production.
	EnterFields(c *FieldsContext)

	// EnterInterfaceDeclaration is called when entering the interfaceDeclaration production.
	EnterInterfaceDeclaration(c *InterfaceDeclarationContext)

	// EnterMembersAndNestedDeclarations is called when entering the membersAndNestedDeclarations production.
	EnterMembersAndNestedDeclarations(c *MembersAndNestedDeclarationsContext)

	// EnterMemberOrNestedDeclaration is called when entering the memberOrNestedDeclaration production.
	EnterMemberOrNestedDeclaration(c *MemberOrNestedDeclarationContext)

	// EnterCompositeKind is called when entering the compositeKind production.
	EnterCompositeKind(c *CompositeKindContext)

	// EnterSpecialFunctionDeclaration is called when entering the specialFunctionDeclaration production.
	EnterSpecialFunctionDeclaration(c *SpecialFunctionDeclarationContext)

	// EnterFunctionDeclaration is called when entering the functionDeclaration production.
	EnterFunctionDeclaration(c *FunctionDeclarationContext)

	// EnterEventDeclaration is called when entering the eventDeclaration production.
	EnterEventDeclaration(c *EventDeclarationContext)

	// EnterParameterList is called when entering the parameterList production.
	EnterParameterList(c *ParameterListContext)

	// EnterParameter is called when entering the parameter production.
	EnterParameter(c *ParameterContext)

	// EnterTypeAnnotation is called when entering the typeAnnotation production.
	EnterTypeAnnotation(c *TypeAnnotationContext)

	// EnterFullType is called when entering the fullType production.
	EnterFullType(c *FullTypeContext)

	// EnterInnerType is called when entering the innerType production.
	EnterInnerType(c *InnerTypeContext)

	// EnterBaseType is called when entering the baseType production.
	EnterBaseType(c *BaseTypeContext)

	// EnterTypeRestrictions is called when entering the typeRestrictions production.
	EnterTypeRestrictions(c *TypeRestrictionsContext)

	// EnterNominalType is called when entering the nominalType production.
	EnterNominalType(c *NominalTypeContext)

	// EnterFunctionType is called when entering the functionType production.
	EnterFunctionType(c *FunctionTypeContext)

	// EnterVariableSizedType is called when entering the variableSizedType production.
	EnterVariableSizedType(c *VariableSizedTypeContext)

	// EnterConstantSizedType is called when entering the constantSizedType production.
	EnterConstantSizedType(c *ConstantSizedTypeContext)

	// EnterDictionaryType is called when entering the dictionaryType production.
	EnterDictionaryType(c *DictionaryTypeContext)

	// EnterBlock is called when entering the block production.
	EnterBlock(c *BlockContext)

	// EnterFunctionBlock is called when entering the functionBlock production.
	EnterFunctionBlock(c *FunctionBlockContext)

	// EnterPreConditions is called when entering the preConditions production.
	EnterPreConditions(c *PreConditionsContext)

	// EnterPostConditions is called when entering the postConditions production.
	EnterPostConditions(c *PostConditionsContext)

	// EnterConditions is called when entering the conditions production.
	EnterConditions(c *ConditionsContext)

	// EnterCondition is called when entering the condition production.
	EnterCondition(c *ConditionContext)

	// EnterStatements is called when entering the statements production.
	EnterStatements(c *StatementsContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterReturnStatement is called when entering the returnStatement production.
	EnterReturnStatement(c *ReturnStatementContext)

	// EnterBreakStatement is called when entering the breakStatement production.
	EnterBreakStatement(c *BreakStatementContext)

	// EnterContinueStatement is called when entering the continueStatement production.
	EnterContinueStatement(c *ContinueStatementContext)

	// EnterIfStatement is called when entering the ifStatement production.
	EnterIfStatement(c *IfStatementContext)

	// EnterWhileStatement is called when entering the whileStatement production.
	EnterWhileStatement(c *WhileStatementContext)

	// EnterForStatement is called when entering the forStatement production.
	EnterForStatement(c *ForStatementContext)

	// EnterEmitStatement is called when entering the emitStatement production.
	EnterEmitStatement(c *EmitStatementContext)

	// EnterVariableDeclaration is called when entering the variableDeclaration production.
	EnterVariableDeclaration(c *VariableDeclarationContext)

	// EnterAssignment is called when entering the assignment production.
	EnterAssignment(c *AssignmentContext)

	// EnterSwap is called when entering the swap production.
	EnterSwap(c *SwapContext)

	// EnterTransfer is called when entering the transfer production.
	EnterTransfer(c *TransferContext)

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

	// EnterNilCoalescingExpression is called when entering the nilCoalescingExpression production.
	EnterNilCoalescingExpression(c *NilCoalescingExpressionContext)

	// EnterBitwiseOrExpression is called when entering the bitwiseOrExpression production.
	EnterBitwiseOrExpression(c *BitwiseOrExpressionContext)

	// EnterBitwiseXorExpression is called when entering the bitwiseXorExpression production.
	EnterBitwiseXorExpression(c *BitwiseXorExpressionContext)

	// EnterBitwiseAndExpression is called when entering the bitwiseAndExpression production.
	EnterBitwiseAndExpression(c *BitwiseAndExpressionContext)

	// EnterBitwiseShiftExpression is called when entering the bitwiseShiftExpression production.
	EnterBitwiseShiftExpression(c *BitwiseShiftExpressionContext)

	// EnterAdditiveExpression is called when entering the additiveExpression production.
	EnterAdditiveExpression(c *AdditiveExpressionContext)

	// EnterMultiplicativeExpression is called when entering the multiplicativeExpression production.
	EnterMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// EnterCastingExpression is called when entering the castingExpression production.
	EnterCastingExpression(c *CastingExpressionContext)

	// EnterUnaryExpression is called when entering the unaryExpression production.
	EnterUnaryExpression(c *UnaryExpressionContext)

	// EnterPrimaryExpression is called when entering the primaryExpression production.
	EnterPrimaryExpression(c *PrimaryExpressionContext)

	// EnterAccessExpression is called when entering the accessExpression production.
	EnterAccessExpression(c *AccessExpressionContext)

	// EnterInvocationExpression is called when entering the invocationExpression production.
	EnterInvocationExpression(c *InvocationExpressionContext)

	// EnterNestedExpression is called when entering the nestedExpression production.
	EnterNestedExpression(c *NestedExpressionContext)

	// EnterIdentifierExpression is called when entering the identifierExpression production.
	EnterIdentifierExpression(c *IdentifierExpressionContext)

	// EnterForceExpression is called when entering the forceExpression production.
	EnterForceExpression(c *ForceExpressionContext)

	// EnterLiteralExpression is called when entering the literalExpression production.
	EnterLiteralExpression(c *LiteralExpressionContext)

	// EnterFunctionExpression is called when entering the functionExpression production.
	EnterFunctionExpression(c *FunctionExpressionContext)

	// EnterEqualityOp is called when entering the equalityOp production.
	EnterEqualityOp(c *EqualityOpContext)

	// EnterRelationalOp is called when entering the relationalOp production.
	EnterRelationalOp(c *RelationalOpContext)

	// EnterBitwiseShiftOp is called when entering the bitwiseShiftOp production.
	EnterBitwiseShiftOp(c *BitwiseShiftOpContext)

	// EnterAdditiveOp is called when entering the additiveOp production.
	EnterAdditiveOp(c *AdditiveOpContext)

	// EnterMultiplicativeOp is called when entering the multiplicativeOp production.
	EnterMultiplicativeOp(c *MultiplicativeOpContext)

	// EnterUnaryOp is called when entering the unaryOp production.
	EnterUnaryOp(c *UnaryOpContext)

	// EnterCastingOp is called when entering the castingOp production.
	EnterCastingOp(c *CastingOpContext)

	// EnterCreateExpression is called when entering the createExpression production.
	EnterCreateExpression(c *CreateExpressionContext)

	// EnterDestroyExpression is called when entering the destroyExpression production.
	EnterDestroyExpression(c *DestroyExpressionContext)

	// EnterReferenceExpression is called when entering the referenceExpression production.
	EnterReferenceExpression(c *ReferenceExpressionContext)

	// EnterExpressionAccess is called when entering the expressionAccess production.
	EnterExpressionAccess(c *ExpressionAccessContext)

	// EnterMemberAccess is called when entering the memberAccess production.
	EnterMemberAccess(c *MemberAccessContext)

	// EnterBracketExpression is called when entering the bracketExpression production.
	EnterBracketExpression(c *BracketExpressionContext)

	// EnterInvocation is called when entering the invocation production.
	EnterInvocation(c *InvocationContext)

	// EnterArgument is called when entering the argument production.
	EnterArgument(c *ArgumentContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterNilLiteral is called when entering the nilLiteral production.
	EnterNilLiteral(c *NilLiteralContext)

	// EnterPathLiteral is called when entering the pathLiteral production.
	EnterPathLiteral(c *PathLiteralContext)

	// EnterStringLiteral is called when entering the stringLiteral production.
	EnterStringLiteral(c *StringLiteralContext)

	// EnterFixedPointLiteral is called when entering the fixedPointLiteral production.
	EnterFixedPointLiteral(c *FixedPointLiteralContext)

	// EnterIntegerLiteral is called when entering the integerLiteral production.
	EnterIntegerLiteral(c *IntegerLiteralContext)

	// EnterDecimalLiteral is called when entering the DecimalLiteral production.
	EnterDecimalLiteral(c *DecimalLiteralContext)

	// EnterBinaryLiteral is called when entering the BinaryLiteral production.
	EnterBinaryLiteral(c *BinaryLiteralContext)

	// EnterOctalLiteral is called when entering the OctalLiteral production.
	EnterOctalLiteral(c *OctalLiteralContext)

	// EnterHexadecimalLiteral is called when entering the HexadecimalLiteral production.
	EnterHexadecimalLiteral(c *HexadecimalLiteralContext)

	// EnterInvalidNumberLiteral is called when entering the InvalidNumberLiteral production.
	EnterInvalidNumberLiteral(c *InvalidNumberLiteralContext)

	// EnterArrayLiteral is called when entering the arrayLiteral production.
	EnterArrayLiteral(c *ArrayLiteralContext)

	// EnterDictionaryLiteral is called when entering the dictionaryLiteral production.
	EnterDictionaryLiteral(c *DictionaryLiteralContext)

	// EnterDictionaryEntry is called when entering the dictionaryEntry production.
	EnterDictionaryEntry(c *DictionaryEntryContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterEos is called when entering the eos production.
	EnterEos(c *EosContext)

	// ExitProgram is called when exiting the program production.
	ExitProgram(c *ProgramContext)

	// ExitReplInput is called when exiting the replInput production.
	ExitReplInput(c *ReplInputContext)

	// ExitReplElement is called when exiting the replElement production.
	ExitReplElement(c *ReplElementContext)

	// ExitReplStatement is called when exiting the replStatement production.
	ExitReplStatement(c *ReplStatementContext)

	// ExitReplDeclaration is called when exiting the replDeclaration production.
	ExitReplDeclaration(c *ReplDeclarationContext)

	// ExitDeclaration is called when exiting the declaration production.
	ExitDeclaration(c *DeclarationContext)

	// ExitTransactionDeclaration is called when exiting the transactionDeclaration production.
	ExitTransactionDeclaration(c *TransactionDeclarationContext)

	// ExitPrepare is called when exiting the prepare production.
	ExitPrepare(c *PrepareContext)

	// ExitExecute is called when exiting the execute production.
	ExitExecute(c *ExecuteContext)

	// ExitImportDeclaration is called when exiting the importDeclaration production.
	ExitImportDeclaration(c *ImportDeclarationContext)

	// ExitAccess is called when exiting the access production.
	ExitAccess(c *AccessContext)

	// ExitCompositeDeclaration is called when exiting the compositeDeclaration production.
	ExitCompositeDeclaration(c *CompositeDeclarationContext)

	// ExitConformances is called when exiting the conformances production.
	ExitConformances(c *ConformancesContext)

	// ExitVariableKind is called when exiting the variableKind production.
	ExitVariableKind(c *VariableKindContext)

	// ExitField is called when exiting the field production.
	ExitField(c *FieldContext)

	// ExitFields is called when exiting the fields production.
	ExitFields(c *FieldsContext)

	// ExitInterfaceDeclaration is called when exiting the interfaceDeclaration production.
	ExitInterfaceDeclaration(c *InterfaceDeclarationContext)

	// ExitMembersAndNestedDeclarations is called when exiting the membersAndNestedDeclarations production.
	ExitMembersAndNestedDeclarations(c *MembersAndNestedDeclarationsContext)

	// ExitMemberOrNestedDeclaration is called when exiting the memberOrNestedDeclaration production.
	ExitMemberOrNestedDeclaration(c *MemberOrNestedDeclarationContext)

	// ExitCompositeKind is called when exiting the compositeKind production.
	ExitCompositeKind(c *CompositeKindContext)

	// ExitSpecialFunctionDeclaration is called when exiting the specialFunctionDeclaration production.
	ExitSpecialFunctionDeclaration(c *SpecialFunctionDeclarationContext)

	// ExitFunctionDeclaration is called when exiting the functionDeclaration production.
	ExitFunctionDeclaration(c *FunctionDeclarationContext)

	// ExitEventDeclaration is called when exiting the eventDeclaration production.
	ExitEventDeclaration(c *EventDeclarationContext)

	// ExitParameterList is called when exiting the parameterList production.
	ExitParameterList(c *ParameterListContext)

	// ExitParameter is called when exiting the parameter production.
	ExitParameter(c *ParameterContext)

	// ExitTypeAnnotation is called when exiting the typeAnnotation production.
	ExitTypeAnnotation(c *TypeAnnotationContext)

	// ExitFullType is called when exiting the fullType production.
	ExitFullType(c *FullTypeContext)

	// ExitInnerType is called when exiting the innerType production.
	ExitInnerType(c *InnerTypeContext)

	// ExitBaseType is called when exiting the baseType production.
	ExitBaseType(c *BaseTypeContext)

	// ExitTypeRestrictions is called when exiting the typeRestrictions production.
	ExitTypeRestrictions(c *TypeRestrictionsContext)

	// ExitNominalType is called when exiting the nominalType production.
	ExitNominalType(c *NominalTypeContext)

	// ExitFunctionType is called when exiting the functionType production.
	ExitFunctionType(c *FunctionTypeContext)

	// ExitVariableSizedType is called when exiting the variableSizedType production.
	ExitVariableSizedType(c *VariableSizedTypeContext)

	// ExitConstantSizedType is called when exiting the constantSizedType production.
	ExitConstantSizedType(c *ConstantSizedTypeContext)

	// ExitDictionaryType is called when exiting the dictionaryType production.
	ExitDictionaryType(c *DictionaryTypeContext)

	// ExitBlock is called when exiting the block production.
	ExitBlock(c *BlockContext)

	// ExitFunctionBlock is called when exiting the functionBlock production.
	ExitFunctionBlock(c *FunctionBlockContext)

	// ExitPreConditions is called when exiting the preConditions production.
	ExitPreConditions(c *PreConditionsContext)

	// ExitPostConditions is called when exiting the postConditions production.
	ExitPostConditions(c *PostConditionsContext)

	// ExitConditions is called when exiting the conditions production.
	ExitConditions(c *ConditionsContext)

	// ExitCondition is called when exiting the condition production.
	ExitCondition(c *ConditionContext)

	// ExitStatements is called when exiting the statements production.
	ExitStatements(c *StatementsContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitReturnStatement is called when exiting the returnStatement production.
	ExitReturnStatement(c *ReturnStatementContext)

	// ExitBreakStatement is called when exiting the breakStatement production.
	ExitBreakStatement(c *BreakStatementContext)

	// ExitContinueStatement is called when exiting the continueStatement production.
	ExitContinueStatement(c *ContinueStatementContext)

	// ExitIfStatement is called when exiting the ifStatement production.
	ExitIfStatement(c *IfStatementContext)

	// ExitWhileStatement is called when exiting the whileStatement production.
	ExitWhileStatement(c *WhileStatementContext)

	// ExitForStatement is called when exiting the forStatement production.
	ExitForStatement(c *ForStatementContext)

	// ExitEmitStatement is called when exiting the emitStatement production.
	ExitEmitStatement(c *EmitStatementContext)

	// ExitVariableDeclaration is called when exiting the variableDeclaration production.
	ExitVariableDeclaration(c *VariableDeclarationContext)

	// ExitAssignment is called when exiting the assignment production.
	ExitAssignment(c *AssignmentContext)

	// ExitSwap is called when exiting the swap production.
	ExitSwap(c *SwapContext)

	// ExitTransfer is called when exiting the transfer production.
	ExitTransfer(c *TransferContext)

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

	// ExitNilCoalescingExpression is called when exiting the nilCoalescingExpression production.
	ExitNilCoalescingExpression(c *NilCoalescingExpressionContext)

	// ExitBitwiseOrExpression is called when exiting the bitwiseOrExpression production.
	ExitBitwiseOrExpression(c *BitwiseOrExpressionContext)

	// ExitBitwiseXorExpression is called when exiting the bitwiseXorExpression production.
	ExitBitwiseXorExpression(c *BitwiseXorExpressionContext)

	// ExitBitwiseAndExpression is called when exiting the bitwiseAndExpression production.
	ExitBitwiseAndExpression(c *BitwiseAndExpressionContext)

	// ExitBitwiseShiftExpression is called when exiting the bitwiseShiftExpression production.
	ExitBitwiseShiftExpression(c *BitwiseShiftExpressionContext)

	// ExitAdditiveExpression is called when exiting the additiveExpression production.
	ExitAdditiveExpression(c *AdditiveExpressionContext)

	// ExitMultiplicativeExpression is called when exiting the multiplicativeExpression production.
	ExitMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// ExitCastingExpression is called when exiting the castingExpression production.
	ExitCastingExpression(c *CastingExpressionContext)

	// ExitUnaryExpression is called when exiting the unaryExpression production.
	ExitUnaryExpression(c *UnaryExpressionContext)

	// ExitPrimaryExpression is called when exiting the primaryExpression production.
	ExitPrimaryExpression(c *PrimaryExpressionContext)

	// ExitAccessExpression is called when exiting the accessExpression production.
	ExitAccessExpression(c *AccessExpressionContext)

	// ExitInvocationExpression is called when exiting the invocationExpression production.
	ExitInvocationExpression(c *InvocationExpressionContext)

	// ExitNestedExpression is called when exiting the nestedExpression production.
	ExitNestedExpression(c *NestedExpressionContext)

	// ExitIdentifierExpression is called when exiting the identifierExpression production.
	ExitIdentifierExpression(c *IdentifierExpressionContext)

	// ExitForceExpression is called when exiting the forceExpression production.
	ExitForceExpression(c *ForceExpressionContext)

	// ExitLiteralExpression is called when exiting the literalExpression production.
	ExitLiteralExpression(c *LiteralExpressionContext)

	// ExitFunctionExpression is called when exiting the functionExpression production.
	ExitFunctionExpression(c *FunctionExpressionContext)

	// ExitEqualityOp is called when exiting the equalityOp production.
	ExitEqualityOp(c *EqualityOpContext)

	// ExitRelationalOp is called when exiting the relationalOp production.
	ExitRelationalOp(c *RelationalOpContext)

	// ExitBitwiseShiftOp is called when exiting the bitwiseShiftOp production.
	ExitBitwiseShiftOp(c *BitwiseShiftOpContext)

	// ExitAdditiveOp is called when exiting the additiveOp production.
	ExitAdditiveOp(c *AdditiveOpContext)

	// ExitMultiplicativeOp is called when exiting the multiplicativeOp production.
	ExitMultiplicativeOp(c *MultiplicativeOpContext)

	// ExitUnaryOp is called when exiting the unaryOp production.
	ExitUnaryOp(c *UnaryOpContext)

	// ExitCastingOp is called when exiting the castingOp production.
	ExitCastingOp(c *CastingOpContext)

	// ExitCreateExpression is called when exiting the createExpression production.
	ExitCreateExpression(c *CreateExpressionContext)

	// ExitDestroyExpression is called when exiting the destroyExpression production.
	ExitDestroyExpression(c *DestroyExpressionContext)

	// ExitReferenceExpression is called when exiting the referenceExpression production.
	ExitReferenceExpression(c *ReferenceExpressionContext)

	// ExitExpressionAccess is called when exiting the expressionAccess production.
	ExitExpressionAccess(c *ExpressionAccessContext)

	// ExitMemberAccess is called when exiting the memberAccess production.
	ExitMemberAccess(c *MemberAccessContext)

	// ExitBracketExpression is called when exiting the bracketExpression production.
	ExitBracketExpression(c *BracketExpressionContext)

	// ExitInvocation is called when exiting the invocation production.
	ExitInvocation(c *InvocationContext)

	// ExitArgument is called when exiting the argument production.
	ExitArgument(c *ArgumentContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitNilLiteral is called when exiting the nilLiteral production.
	ExitNilLiteral(c *NilLiteralContext)

	// ExitPathLiteral is called when exiting the pathLiteral production.
	ExitPathLiteral(c *PathLiteralContext)

	// ExitStringLiteral is called when exiting the stringLiteral production.
	ExitStringLiteral(c *StringLiteralContext)

	// ExitFixedPointLiteral is called when exiting the fixedPointLiteral production.
	ExitFixedPointLiteral(c *FixedPointLiteralContext)

	// ExitIntegerLiteral is called when exiting the integerLiteral production.
	ExitIntegerLiteral(c *IntegerLiteralContext)

	// ExitDecimalLiteral is called when exiting the DecimalLiteral production.
	ExitDecimalLiteral(c *DecimalLiteralContext)

	// ExitBinaryLiteral is called when exiting the BinaryLiteral production.
	ExitBinaryLiteral(c *BinaryLiteralContext)

	// ExitOctalLiteral is called when exiting the OctalLiteral production.
	ExitOctalLiteral(c *OctalLiteralContext)

	// ExitHexadecimalLiteral is called when exiting the HexadecimalLiteral production.
	ExitHexadecimalLiteral(c *HexadecimalLiteralContext)

	// ExitInvalidNumberLiteral is called when exiting the InvalidNumberLiteral production.
	ExitInvalidNumberLiteral(c *InvalidNumberLiteralContext)

	// ExitArrayLiteral is called when exiting the arrayLiteral production.
	ExitArrayLiteral(c *ArrayLiteralContext)

	// ExitDictionaryLiteral is called when exiting the dictionaryLiteral production.
	ExitDictionaryLiteral(c *DictionaryLiteralContext)

	// ExitDictionaryEntry is called when exiting the dictionaryEntry production.
	ExitDictionaryEntry(c *DictionaryEntryContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitEos is called when exiting the eos production.
	ExitEos(c *EosContext)
}

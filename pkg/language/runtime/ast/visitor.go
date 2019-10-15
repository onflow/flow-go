package ast

type Repr interface{}

type Element interface {
	HasPosition
	Accept(Visitor) Repr
}

type NotAnElement struct{}

func (NotAnElement) Accept(Visitor) Repr {
	// NO-OP
	return nil
}

func (NotAnElement) StartPosition() Position {
	return Position{}
}

func (NotAnElement) EndPosition() Position {
	return Position{}
}

type StatementVisitor interface {
	VisitReturnStatement(*ReturnStatement) Repr
	VisitBreakStatement(*BreakStatement) Repr
	VisitContinueStatement(*ContinueStatement) Repr
	VisitIfStatement(*IfStatement) Repr
	VisitWhileStatement(*WhileStatement) Repr
	VisitEmitStatement(*EmitStatement) Repr
	VisitVariableDeclaration(*VariableDeclaration) Repr
	VisitAssignmentStatement(*AssignmentStatement) Repr
	VisitSwapStatement(*SwapStatement) Repr
	VisitExpressionStatement(*ExpressionStatement) Repr
}

type ExpressionVisitor interface {
	VisitBoolExpression(*BoolExpression) Repr
	VisitNilExpression(*NilExpression) Repr
	VisitIntExpression(*IntExpression) Repr
	VisitArrayExpression(*ArrayExpression) Repr
	VisitDictionaryExpression(*DictionaryExpression) Repr
	VisitIdentifierExpression(*IdentifierExpression) Repr
	VisitInvocationExpression(*InvocationExpression) Repr
	VisitMemberExpression(*MemberExpression) Repr
	VisitIndexExpression(*IndexExpression) Repr
	VisitConditionalExpression(*ConditionalExpression) Repr
	VisitUnaryExpression(*UnaryExpression) Repr
	VisitBinaryExpression(*BinaryExpression) Repr
	VisitFunctionExpression(*FunctionExpression) Repr
	VisitStringExpression(*StringExpression) Repr
	VisitFailableDowncastExpression(*FailableDowncastExpression) Repr
	VisitCreateExpression(*CreateExpression) Repr
	VisitDestroyExpression(*DestroyExpression) Repr
}

type Visitor interface {
	StatementVisitor
	ExpressionVisitor
	VisitProgram(*Program) Repr
	VisitFunctionDeclaration(*FunctionDeclaration) Repr
	VisitBlock(*Block) Repr
	VisitFunctionBlock(*FunctionBlock) Repr
	VisitCompositeDeclaration(*CompositeDeclaration) Repr
	VisitInterfaceDeclaration(*InterfaceDeclaration) Repr
	VisitFieldDeclaration(*FieldDeclaration) Repr
	VisitInitializerDeclaration(*InitializerDeclaration) Repr
	VisitCondition(*Condition) Repr
	VisitImportDeclaration(*ImportDeclaration) Repr
	VisitEventDeclaration(*EventDeclaration) Repr
}

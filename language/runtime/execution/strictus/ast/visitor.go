package ast

type Repr interface{}

type Element interface {
	HasPosition
	Accept(Visitor) Repr
}

type Visitor interface {
	VisitProgram(*Program) Repr
	VisitFunctionDeclaration(*FunctionDeclaration) Repr
	VisitBlock(*Block) Repr
	// statements
	VisitReturnStatement(*ReturnStatement) Repr
	VisitIfStatement(*IfStatement) Repr
	VisitWhileStatement(*WhileStatement) Repr
	VisitVariableDeclaration(*VariableDeclaration) Repr
	VisitAssignment(*AssignmentStatement) Repr
	VisitExpressionStatement(*ExpressionStatement) Repr
	/// expressions
	VisitBoolExpression(*BoolExpression) Repr
	VisitIntExpression(*IntExpression) Repr
	VisitArrayExpression(*ArrayExpression) Repr
	VisitIdentifierExpression(*IdentifierExpression) Repr
	VisitInvocationExpression(*InvocationExpression) Repr
	VisitMemberExpression(*MemberExpression) Repr
	VisitIndexExpression(*IndexExpression) Repr
	VisitConditionalExpression(*ConditionalExpression) Repr
	VisitUnaryExpression(*UnaryExpression) Repr
	VisitBinaryExpression(*BinaryExpression) Repr
	VisitFunctionExpression(*FunctionExpression) Repr
}

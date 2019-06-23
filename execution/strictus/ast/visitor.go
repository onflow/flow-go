package ast

type Repr interface{}

type Element interface {
	Accept(Visitor) Repr
}

type Visitor interface {
	VisitProgram(Program) Repr
	VisitFunctionDeclaration(FunctionDeclaration) Repr
	VisitBlock(Block) Repr
	// statements
	VisitReturnStatement(ReturnStatement) Repr
	VisitIfStatement(IfStatement) Repr
	VisitWhileStatement(WhileStatement) Repr
	VisitVariableDeclaration(VariableDeclaration) Repr
	VisitAssignment(Assignment) Repr
	VisitExpressionStatement(ExpressionStatement) Repr
	/// expressions
	VisitBoolExpression(BoolExpression) Repr
	VisitInt8Expression(Int8Expression) Repr
	VisitInt16Expression(Int16Expression) Repr
	VisitInt32Expression(Int32Expression) Repr
	VisitInt64Expression(Int64Expression) Repr
	VisitUInt8Expression(UInt8Expression) Repr
	VisitUInt16Expression(UInt16Expression) Repr
	VisitUInt32Expression(UInt32Expression) Repr
	VisitUInt64Expression(UInt64Expression) Repr
	VisitArrayExpression(ArrayExpression) Repr
	VisitIdentifierExpression(IdentifierExpression) Repr
	VisitInvocationExpression(InvocationExpression) Repr
	VisitMemberExpression(MemberExpression) Repr
	VisitIndexExpression(IndexExpression) Repr
	VisitConditionalExpression(ConditionalExpression) Repr
	VisitUnaryExpression(UnaryExpression) Repr
	VisitBinaryExpression(BinaryExpression) Repr
	VisitFunctionExpression(FunctionExpression) Repr
}

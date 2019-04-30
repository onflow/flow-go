package ast

type Repr interface{}

type Element interface {
	Accept(Visitor) Repr
}

type Visitor interface {
	VisitProgram(Program) Repr
	VisitFunction(Function) Repr
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
	VisitIntExpression(IntExpression) Repr
	VisitArrayExpression(ArrayExpression) Repr
	VisitIdentifierExpression(IdentifierExpression) Repr
	VisitInvocationExpression(InvocationExpression) Repr
	VisitMemberExpression(MemberExpression) Repr
	VisitIndexExpression(IndexExpression) Repr
	VisitConditionalExpression(ConditionalExpression) Repr
	VisitBinaryExpression(BinaryExpression) Repr
}

type DefaultVisitor struct{}

func (v DefaultVisitor) VisitProgram(program Program) Repr {
	for _, function := range program.Functions {
		function.Accept(v)
	}

	return nil
}

func (v DefaultVisitor) VisitFunction(function Function) Repr {
	return function.Block.Accept(v)
}

func (v DefaultVisitor) VisitBlock(block Block) Repr {
	for _, statement := range block.Statements {
		statement.Accept(v)
	}

	return nil
}

func (v DefaultVisitor) VisitReturnStatement(s ReturnStatement) Repr {
	return s.Expression.Accept(v)
}

func (v DefaultVisitor) VisitIfStatement(s IfStatement) Repr {
	s.Test.Accept(v)
	s.Then.Accept(v)
	s.Else.Accept(v)
	return nil
}

func (v DefaultVisitor) VisitWhileStatement(s WhileStatement) Repr {
	s.Test.Accept(v)
	s.Block.Accept(v)
	return nil
}

func (v DefaultVisitor) VisitVariableDeclaration(d VariableDeclaration) Repr {
	return d.Value.Accept(v)
}

func (v DefaultVisitor) VisitAssignment(a Assignment) Repr {
	return a.Value.Accept(v)
}

func (v DefaultVisitor) VisitExpressionStatement(s ExpressionStatement) Repr {
	return s.Expression.Accept(v)
}

func (v DefaultVisitor) VisitBoolExpression(e BoolExpression) Repr {
	return nil
}

func (v DefaultVisitor) VisitIntExpression(IntExpression) Repr {
	return nil
}

func (v DefaultVisitor) VisitArrayExpression(e ArrayExpression) Repr {
	for _, value := range e.Values {
		value.Accept(v)
	}

	return nil
}

func (v DefaultVisitor) VisitIdentifierExpression(IdentifierExpression) Repr {
	return nil
}

func (v DefaultVisitor) VisitInvocationExpression(e InvocationExpression) Repr {
	for _, argument := range e.Arguments {
		argument.Accept(v)
	}

	return nil
}

func (v DefaultVisitor) VisitMemberExpression(e MemberExpression) Repr {
	return e.Expression.Accept(v)
}

func (v DefaultVisitor) VisitIndexExpression(e IndexExpression) Repr {
	e.Index.Accept(v)
	e.Expression.Accept(v)
	return nil
}

func (v DefaultVisitor) VisitConditionalExpression(e ConditionalExpression) Repr {
	e.Test.Accept(v)
	e.Then.Accept(v)
	e.Else.Accept(v)
	return nil
}

func (v DefaultVisitor) VisitBinaryExpression(e BinaryExpression) Repr {
	e.Left.Accept(v)
	e.Right.Accept(v)
	return nil
}

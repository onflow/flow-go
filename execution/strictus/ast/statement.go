package ast

type Statement interface {
	Element
	isStatement()
}

// ReturnStatement

type ReturnStatement struct {
	Expression    Expression
	StartPosition *Position
	EndPosition   *Position
}

func (s *ReturnStatement) GetStartPosition() *Position {
	return s.StartPosition
}

func (s *ReturnStatement) GetEndPosition() *Position {
	return s.EndPosition
}

func (*ReturnStatement) isStatement() {}

func (s *ReturnStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitReturnStatement(s)
}

// IfStatement

type IfStatement struct {
	Test          Expression
	Then          *Block
	Else          *Block
	StartPosition *Position
	EndPosition   *Position
}

func (s *IfStatement) GetStartPosition() *Position {
	return s.StartPosition
}

func (s *IfStatement) GetEndPosition() *Position {
	return s.EndPosition
}

func (*IfStatement) isStatement() {}

func (s *IfStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitIfStatement(s)
}

// WhileStatement

type WhileStatement struct {
	Test          Expression
	Block         *Block
	StartPosition *Position
	EndPosition   *Position
}

func (s *WhileStatement) GetStartPosition() *Position {
	return s.StartPosition
}

func (s *WhileStatement) GetEndPosition() *Position {
	return s.EndPosition
}

func (*WhileStatement) isStatement() {}

func (s *WhileStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitWhileStatement(s)
}

// AssignmentStatement

type AssignmentStatement struct {
	Target        Expression
	Value         Expression
	StartPosition *Position
	EndPosition   *Position
}

func (s *AssignmentStatement) GetStartPosition() *Position {
	return s.StartPosition
}

func (s *AssignmentStatement) GetEndPosition() *Position {
	return s.EndPosition
}

func (*AssignmentStatement) isStatement() {}

func (s *AssignmentStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitAssignment(s)
}

// ExpressionStatement

type ExpressionStatement struct {
	Expression Expression
}

func (s *ExpressionStatement) GetStartPosition() *Position {
	return s.Expression.GetStartPosition()
}

func (s *ExpressionStatement) GetEndPosition() *Position {
	return s.Expression.GetEndPosition()
}

func (*ExpressionStatement) isStatement() {}

func (s *ExpressionStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitExpressionStatement(s)
}

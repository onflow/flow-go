package ast

type Statement interface {
	Element
	isStatement()
}

// ReturnStatement

type ReturnStatement struct {
	Expression Expression
	StartPos   Position
	EndPos     Position
}

func (s *ReturnStatement) StartPosition() Position {
	return s.StartPos
}

func (s *ReturnStatement) EndPosition() Position {
	return s.EndPos
}

func (*ReturnStatement) isStatement() {}

func (s *ReturnStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitReturnStatement(s)
}

// BreakStatement

type BreakStatement struct {
	StartPos Position
	EndPos   Position
}

func (s *BreakStatement) StartPosition() Position {
	return s.StartPos
}

func (s *BreakStatement) EndPosition() Position {
	return s.EndPos
}

func (*BreakStatement) isStatement() {}

func (s *BreakStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitBreakStatement(s)
}

// ContinueStatement

type ContinueStatement struct {
	StartPos Position
	EndPos   Position
}

func (s *ContinueStatement) StartPosition() Position {
	return s.StartPos
}

func (s *ContinueStatement) EndPosition() Position {
	return s.EndPos
}

func (*ContinueStatement) isStatement() {}

func (s *ContinueStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitContinueStatement(s)
}

// IfStatement

type IfStatement struct {
	Test     Expression
	Then     *Block
	Else     *Block
	StartPos Position
}

func (s *IfStatement) StartPosition() Position {
	return s.StartPos
}

func (s *IfStatement) EndPosition() Position {
	if s.Else != nil {
		return s.Else.EndPosition()
	} else {
		return s.Then.EndPosition()
	}
}

func (*IfStatement) isStatement() {}

func (s *IfStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitIfStatement(s)
}

// WhileStatement

type WhileStatement struct {
	Test     Expression
	Block    *Block
	StartPos Position
	EndPos   Position
}

func (s *WhileStatement) StartPosition() Position {
	return s.StartPos
}

func (s *WhileStatement) EndPosition() Position {
	return s.EndPos
}

func (*WhileStatement) isStatement() {}

func (s *WhileStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitWhileStatement(s)
}

// AssignmentStatement

type AssignmentStatement struct {
	Target Expression
	Value  Expression
}

func (s *AssignmentStatement) StartPosition() Position {
	return s.Target.StartPosition()
}

func (s *AssignmentStatement) EndPosition() Position {
	return s.Value.EndPosition()
}

func (*AssignmentStatement) isStatement() {}

func (s *AssignmentStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitAssignment(s)
}

// ExpressionStatement

type ExpressionStatement struct {
	Expression Expression
}

func (s *ExpressionStatement) StartPosition() Position {
	return s.Expression.StartPosition()
}

func (s *ExpressionStatement) EndPosition() Position {
	return s.Expression.EndPosition()
}

func (*ExpressionStatement) isStatement() {}

func (s *ExpressionStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitExpressionStatement(s)
}

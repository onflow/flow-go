package ast

type Program struct {
	// all declarations, in the order they are defined
	Declarations []Declaration
}

func (p *Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

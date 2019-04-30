package ast

type Program struct {
	Functions []Function
}

func (p Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

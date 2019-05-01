package ast

type Program struct {
	Functions map[string]Function
}

func (p Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

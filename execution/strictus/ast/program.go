package ast

type Program struct {
	// all declarations, indexed by name
	Declarations map[string]Declaration
	// all declarations, in the order they are defined
	AllDeclarations []Declaration
}

func (p Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

package ast

type Function struct {
	IsPublic   bool
	Identifier string
	Parameters []Parameter
	ReturnType Type
	Block      Block
}

func (f Function) Accept(visitor Visitor) Repr {
	return visitor.VisitFunction(f)
}

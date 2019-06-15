package ast

type VariableDeclaration struct {
	IsConst    bool
	Identifier string
	Type       Type
	Value      Expression
}

func (VariableDeclaration) isDeclaration() {}
func (VariableDeclaration) isStatement()   {}

func (v VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(v)
}

func (v VariableDeclaration) DeclarationName() string {
	return v.Identifier
}

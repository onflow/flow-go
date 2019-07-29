package ast

type VariableDeclaration struct {
	IsConstant    bool
	Identifier    string
	Type          Type
	Value         Expression
	StartPos      *Position
	EndPos        *Position
	IdentifierPos *Position
}

func (v *VariableDeclaration) StartPosition() *Position {
	return v.StartPos
}

func (v *VariableDeclaration) EndPosition() *Position {
	return v.EndPos
}

func (v *VariableDeclaration) GetIdentifierPosition() *Position {
	return v.IdentifierPos
}

func (*VariableDeclaration) isDeclaration() {}
func (*VariableDeclaration) isStatement()   {}

func (v *VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(v)
}

func (v *VariableDeclaration) DeclarationName() string {
	return v.Identifier
}

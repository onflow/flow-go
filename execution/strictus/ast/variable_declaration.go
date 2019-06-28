package ast

type VariableDeclaration struct {
	IsConst            bool
	Identifier         string
	Type               Type
	Value              Expression
	StartPosition      Position
	EndPosition        Position
	IdentifierPosition Position
}

func (v VariableDeclaration) GetStartPosition() Position {
	return v.StartPosition
}

func (v VariableDeclaration) GetEndPosition() Position {
	return v.EndPosition
}

func (v VariableDeclaration) GetIdentifierPosition() Position {
	return v.IdentifierPosition
}

func (VariableDeclaration) isDeclaration() {}
func (VariableDeclaration) isStatement()   {}

func (v VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(v)
}

func (v VariableDeclaration) DeclarationName() string {
	return v.Identifier
}

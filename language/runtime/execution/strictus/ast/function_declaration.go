package ast

type FunctionDeclaration struct {
	IsPublic      bool
	Identifier    string
	Parameters    []*Parameter
	ReturnType    Type
	Block         *Block
	StartPos      *Position
	EndPos        *Position
	IdentifierPos *Position
}

func (f *FunctionDeclaration) StartPosition() *Position {
	return f.StartPos
}

func (f *FunctionDeclaration) EndPosition() *Position {
	return f.EndPos
}

func (f *FunctionDeclaration) GetIdentifierPosition() *Position {
	return f.IdentifierPos
}

func (f *FunctionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionDeclaration(f)
}

func (*FunctionDeclaration) isDeclaration() {}
func (*FunctionDeclaration) isStatement()   {}

func (f *FunctionDeclaration) DeclarationName() string {
	return f.Identifier
}

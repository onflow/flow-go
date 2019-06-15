package ast

type FunctionDeclaration struct {
	IsPublic   bool
	Identifier string
	Parameters []Parameter
	ReturnType Type
	Block      Block
}

func (f FunctionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionDeclaration(f)
}

func (FunctionDeclaration) isDeclaration() {}
func (FunctionDeclaration) isStatement()   {}

func (f FunctionDeclaration) DeclarationName() string {
	return f.Identifier
}

package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

type FunctionDeclaration struct {
	Access        Access
	Identifier    string
	Parameters    []*Parameter
	ReturnType    Type
	Block         *Block
	StartPos      *Position
	IdentifierPos *Position
}

func (f *FunctionDeclaration) StartPosition() *Position {
	return f.StartPos
}

func (f *FunctionDeclaration) EndPosition() *Position {
	return f.Block.EndPosition()
}

func (f *FunctionDeclaration) IdentifierPosition() *Position {
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

func (f *FunctionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (f *FunctionDeclaration) ToExpression() *FunctionExpression {
	return &FunctionExpression{
		Parameters: f.Parameters,
		ReturnType: f.ReturnType,
		Block:      f.Block,
		StartPos:   f.StartPos,
	}
}

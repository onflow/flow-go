package ast

import "github.com/dapperlabs/flow-go/pkg/language/runtime/common"

type FunctionDeclaration struct {
	Access               Access
	Identifier           Identifier
	Parameters           []*Parameter
	ReturnTypeAnnotation *TypeAnnotation
	FunctionBlock        *FunctionBlock
	StartPos             Position
}

func (f *FunctionDeclaration) StartPosition() Position {
	return f.StartPos
}

func (f *FunctionDeclaration) EndPosition() Position {
	return f.FunctionBlock.EndPosition()
}

func (f *FunctionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionDeclaration(f)
}

func (*FunctionDeclaration) isDeclaration() {}
func (*FunctionDeclaration) isStatement()   {}

func (f *FunctionDeclaration) DeclarationName() string {
	return f.Identifier.Identifier
}

func (f *FunctionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (f *FunctionDeclaration) ToExpression() *FunctionExpression {
	return &FunctionExpression{
		Parameters:           f.Parameters,
		ReturnTypeAnnotation: f.ReturnTypeAnnotation,
		FunctionBlock:        f.FunctionBlock,
		StartPos:             f.StartPos,
	}
}

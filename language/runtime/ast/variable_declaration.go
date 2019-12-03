package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type VariableDeclaration struct {
	Access            Access
	IsConstant        bool
	Identifier        Identifier
	TypeAnnotation    *TypeAnnotation
	Value             Expression
	Transfer          *Transfer
	StartPos          Position
	SecondTransfer    *Transfer
	SecondValue       Expression
	ParentIfStatement *IfStatement
}

func (d *VariableDeclaration) StartPosition() Position {
	return d.StartPos
}

func (d *VariableDeclaration) EndPosition() Position {
	return d.Value.EndPosition()
}

func (*VariableDeclaration) isIfStatementTest() {}

func (*VariableDeclaration) isDeclaration() {}

func (*VariableDeclaration) isStatement() {}

func (d *VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(d)
}

func (d *VariableDeclaration) DeclarationIdentifier() Identifier {
	return d.Identifier
}

func (d *VariableDeclaration) DeclarationKind() common.DeclarationKind {
	if d.IsConstant {
		return common.DeclarationKindConstant
	}
	return common.DeclarationKindVariable
}

func (d *VariableDeclaration) DeclarationAccess() Access {
	return d.Access
}

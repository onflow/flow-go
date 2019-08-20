package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

type VariableDeclaration struct {
	IsConstant    bool
	Identifier    string
	Type          Type
	Value         Expression
	StartPos      Position
	IdentifierPos Position
}

func (v *VariableDeclaration) StartPosition() Position {
	return v.StartPos
}

func (v *VariableDeclaration) EndPosition() Position {
	return v.Value.EndPosition()
}

func (v *VariableDeclaration) IdentifierPosition() Position {
	return v.IdentifierPos
}

func (*VariableDeclaration) isIfStatementTest() {}

func (*VariableDeclaration) isDeclaration() {}

func (*VariableDeclaration) isStatement() {}

func (v *VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(v)
}

func (v *VariableDeclaration) DeclarationName() string {
	return v.Identifier
}

func (v *VariableDeclaration) DeclarationKind() common.DeclarationKind {
	if v.IsConstant {
		return common.DeclarationKindConstant
	} else {
		return common.DeclarationKindVariable
	}
}

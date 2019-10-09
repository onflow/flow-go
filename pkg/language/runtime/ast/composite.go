package ast

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

// CompositeDeclaration

type CompositeDeclaration struct {
	CompositeKind common.CompositeKind
	Identifier    Identifier
	Conformances  []*NominalType
	Members       *Members
	StartPos      Position
	EndPos        Position
}

func (d *CompositeDeclaration) StartPosition() Position {
	return d.StartPos
}

func (d *CompositeDeclaration) EndPosition() Position {
	return d.EndPos
}

func (d *CompositeDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitCompositeDeclaration(d)
}

func (*CompositeDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*CompositeDeclaration) isStatement() {}

func (d *CompositeDeclaration) DeclarationName() string {
	return d.Identifier.Identifier
}

func (d *CompositeDeclaration) DeclarationKind() common.DeclarationKind {
	switch d.CompositeKind {
	case common.CompositeKindStructure:
		return common.DeclarationKindStructure
	case common.CompositeKindResource:
		return common.DeclarationKindResource
	case common.CompositeKindContract:
		return common.DeclarationKindContract
	case common.CompositeKindEvent:
		return common.DeclarationKindEvent
	}

	panic(&errors.UnreachableError{})
}

// FieldDeclaration

type FieldDeclaration struct {
	Access         Access
	VariableKind   VariableKind
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	StartPos       Position
	EndPos         Position
}

func (f *FieldDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFieldDeclaration(f)
}

func (f *FieldDeclaration) StartPosition() Position {
	return f.StartPos
}

func (f *FieldDeclaration) EndPosition() Position {
	return f.EndPos
}

func (*FieldDeclaration) isDeclaration() {}

func (f *FieldDeclaration) DeclarationName() string {
	return f.Identifier.Identifier
}

func (f *FieldDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindField
}

// InitializerDeclaration

type InitializerDeclaration struct {
	Identifier    Identifier
	Parameters    Parameters
	FunctionBlock *FunctionBlock
	StartPos      Position
}

func (i *InitializerDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInitializerDeclaration(i)
}

func (i *InitializerDeclaration) StartPosition() Position {
	return i.StartPos
}

func (i *InitializerDeclaration) EndPosition() Position {
	return i.FunctionBlock.EndPos
}

func (*InitializerDeclaration) isDeclaration() {}

func (i *InitializerDeclaration) DeclarationName() string {
	return "init"
}

func (i *InitializerDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindInitializer
}

func (i *InitializerDeclaration) ToFunctionExpression() *FunctionExpression {
	return &FunctionExpression{
		Parameters:    i.Parameters,
		FunctionBlock: i.FunctionBlock,
		StartPos:      i.StartPos,
	}
}

package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

// CompositeDeclaration

type CompositeDeclaration struct {
	CompositeKind common.CompositeKind
	Identifier    Identifier
	Conformances  []*NominalType
	Members       *Members
	Range
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
	}

	panic(&errors.UnreachableError{})
}

// FieldDeclaration

type FieldDeclaration struct {
	Access         Access
	VariableKind   VariableKind
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	Range
}

func (f *FieldDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFieldDeclaration(f)
}

func (*FieldDeclaration) isDeclaration() {}

func (f *FieldDeclaration) DeclarationName() string {
	return f.Identifier.Identifier
}

func (f *FieldDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindField
}

// SpecialFunctionDeclaration

type SpecialFunctionDeclaration struct {
	DeclarationKind common.DeclarationKind
	*FunctionDeclaration
}

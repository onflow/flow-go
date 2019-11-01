package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

// InterfaceDeclaration

type InterfaceDeclaration struct {
	CompositeKind common.CompositeKind
	Identifier    Identifier
	Members       *Members
	Range
}

func (d *InterfaceDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInterfaceDeclaration(d)
}

func (*InterfaceDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*InterfaceDeclaration) isStatement() {}

func (d *InterfaceDeclaration) DeclarationName() string {
	return d.Identifier.Identifier
}

func (d *InterfaceDeclaration) DeclarationKind() common.DeclarationKind {
	switch d.CompositeKind {
	case common.CompositeKindStructure:
		return common.DeclarationKindStructureInterface
	case common.CompositeKindResource:
		return common.DeclarationKindResourceInterface
	case common.CompositeKindContract:
		return common.DeclarationKindContractInterface
	}

	panic(&errors.UnreachableError{})
}

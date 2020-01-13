package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

// InterfaceDeclaration

type InterfaceDeclaration struct {
	Access                Access
	CompositeKind         common.CompositeKind
	Identifier            Identifier
	Members               *Members
	CompositeDeclarations []*CompositeDeclaration
	InterfaceDeclarations []*InterfaceDeclaration
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

func (d *InterfaceDeclaration) DeclarationIdentifier() *Identifier {
	return &d.Identifier
}

func (d *InterfaceDeclaration) DeclarationAccess() Access {
	return d.Access
}

func (d *InterfaceDeclaration) DeclarationKind() common.DeclarationKind {
	return d.CompositeKind.DeclarationKind(true)
}

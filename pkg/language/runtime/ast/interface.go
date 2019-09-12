package ast

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

// InterfaceDeclaration

type InterfaceDeclaration struct {
	Kind                  common.CompositeKind
	Identifier            Identifier
	Fields                []*FieldDeclaration
	Initializers          []*InitializerDeclaration
	Functions             []*FunctionDeclaration
	functionsByIdentifier map[string]*FunctionDeclaration
	StartPos              Position
	EndPos                Position
}

func (d *InterfaceDeclaration) FunctionsByIdentifier() map[string]*FunctionDeclaration {
	if d.functionsByIdentifier == nil {
		functionsByIdentifier := make(map[string]*FunctionDeclaration, len(d.Functions))
		for _, function := range d.Functions {
			functionsByIdentifier[function.Identifier.Identifier] = function
		}
		d.functionsByIdentifier = functionsByIdentifier
	}
	return d.functionsByIdentifier
}

func (d *InterfaceDeclaration) StartPosition() Position {
	return d.StartPos
}

func (d *InterfaceDeclaration) EndPosition() Position {
	return d.EndPos
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
	switch d.Kind {
	case common.CompositeKindStructure:
		return common.DeclarationKindStructureInterface
	case common.CompositeKindResource:
		return common.DeclarationKindResourceInterface
	case common.CompositeKindContract:
		return common.DeclarationKindContractInterface
	}

	panic(&errors.UnreachableError{})
}

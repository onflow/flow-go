package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

// InterfaceDeclaration

type InterfaceDeclaration struct {
	Identifier            string
	Fields                []*FieldDeclaration
	Initializer           *InitializerDeclaration
	Functions             []*FunctionDeclaration
	functionsByIdentifier map[string]*FunctionDeclaration
	IdentifierPos         Position
	StartPos              Position
	EndPos                Position
}

func (s *InterfaceDeclaration) FunctionsByIdentifier() map[string]*FunctionDeclaration {
	if s.functionsByIdentifier == nil {
		functionsByIdentifier := make(map[string]*FunctionDeclaration, len(s.Functions))
		for _, function := range s.Functions {
			functionsByIdentifier[function.Identifier] = function
		}
		s.functionsByIdentifier = functionsByIdentifier
	}
	return s.functionsByIdentifier
}

func (s *InterfaceDeclaration) StartPosition() Position {
	return s.StartPos
}

func (s *InterfaceDeclaration) EndPosition() Position {
	return s.EndPos
}

func (s *InterfaceDeclaration) IdentifierPosition() Position {
	return s.IdentifierPos
}

func (s *InterfaceDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInterfaceDeclaration(s)
}

func (*InterfaceDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*InterfaceDeclaration) isStatement() {}

func (s *InterfaceDeclaration) DeclarationName() string {
	return s.Identifier
}

func (s *InterfaceDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindInterface
}

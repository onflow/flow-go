package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

// EventDeclaration

type EventDeclaration struct {
	Identifier    Identifier
	ParameterList *ParameterList
	Range
}

func (e *EventDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitEventDeclaration(e)
}

func (*EventDeclaration) isDeclaration() {}
func (*EventDeclaration) isStatement()   {}

func (e *EventDeclaration) DeclarationName() string {
	return e.Identifier.Identifier
}

func (e *EventDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindEvent
}

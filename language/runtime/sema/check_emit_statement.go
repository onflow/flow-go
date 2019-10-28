package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func (checker *Checker) VisitEmitStatement(statement *ast.EmitStatement) ast.Repr {
	typ := checker.checkInvocationExpression(statement.InvocationExpression)

	if typ.IsInvalidType() {
		return nil
	}

	// check that emitted expression is an event
	eventType, isEventType := typ.(*EventType)
	if !isEventType {
		checker.report(&EmitNonEventError{
			Type:  typ,
			Range: ast.NewRangeFromPositioned(statement),
		})
		return nil
	}

	// check that the event isn't imported
	if eventType.Location.ID() != checker.Location.ID() {
		checker.report(&EmitImportedEventError{
			Type:  typ,
			Range: ast.NewRangeFromPositioned(eventType),
		})
	}

	return nil
}

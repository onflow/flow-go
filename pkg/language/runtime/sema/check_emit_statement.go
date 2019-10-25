package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
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

	// check the the event isn't imported
	if eventType.ImportLocation.ID() != checker.Location.ID() {
		checker.report(&EmitImportedEventError{
			Type: typ,
			Range: ast.Range{
				StartPos: statement.StartPosition(),
				EndPos:   statement.EndPosition(),
			},
		})
	}

	return nil
}

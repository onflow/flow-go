package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func (checker *Checker) VisitEmitStatement(statement *ast.EmitStatement) ast.Repr {
	invocation := statement.InvocationExpression

	ty := checker.checkInvocationExpression(invocation)

	if ty.IsInvalidType() {
		return nil
	}

	// Check that emitted expression is an event

	eventType, isEventType := ty.(*EventType)
	if !isEventType {
		checker.report(
			&EmitNonEventError{
				Type:  ty,
				Range: ast.NewRangeFromPositioned(statement.InvocationExpression),
			},
		)
		return nil
	}

	// Check that the emitted event is declared in the same location

	if eventType.Location.ID() != checker.Location.ID() {
		checker.report(
			&EmitImportedEventError{
				Type:  ty,
				Range: ast.NewRangeFromPositioned(statement.InvocationExpression),
			},
		)
	}

	return nil
}

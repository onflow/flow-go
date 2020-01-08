package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitEmitStatement(statement *ast.EmitStatement) ast.Repr {
	invocation := statement.InvocationExpression

	ty := checker.checkInvocationExpression(invocation)

	if ty.IsInvalidType() {
		return nil
	}

	// Check that emitted expression is an event

	compositeType, isCompositeType := ty.(*CompositeType)
	if !isCompositeType || compositeType.Kind != common.CompositeKindEvent {
		checker.report(
			&EmitNonEventError{
				Type:  ty,
				Range: ast.NewRangeFromPositioned(statement.InvocationExpression),
			},
		)
		return nil
	}

	// Check that the emitted event is declared in the same location

	if !ast.LocationsMatch(compositeType.Location, checker.Location) {

		checker.report(
			&EmitImportedEventError{
				Type:  ty,
				Range: ast.NewRangeFromPositioned(statement.InvocationExpression),
			},
		)
	}

	return nil
}

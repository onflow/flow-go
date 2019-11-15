package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitCreateExpression(expression *ast.CreateExpression) ast.Repr {

	checker.inCreate = true
	defer func() {
		checker.inCreate = false
	}()

	invocation := expression.InvocationExpression

	ty := invocation.Accept(checker).(Type)

	// Check that the created expression is a resource

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed

	compositeType, isCompositeType := ty.(*CompositeType)

	if !ty.IsInvalidType() &&
		(!isCompositeType || compositeType.Kind != common.CompositeKindResource) {

		checker.report(
			&InvalidConstructionError{
				Range: ast.NewRangeFromPositioned(invocation),
			},
		)

		return ty
	}

	// Check that the created resource is declared in the same location

	if !ast.LocationsMatch(compositeType.Location, checker.Location) {

		checker.report(
			&CreateImportedResourceError{
				Type:  compositeType,
				Range: ast.NewRangeFromPositioned(invocation),
			},
		)
	}

	return ty
}

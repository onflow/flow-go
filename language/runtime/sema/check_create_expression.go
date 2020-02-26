package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitCreateExpression(expression *ast.CreateExpression) ast.Repr {
	inCreate := checker.inCreate
	checker.inCreate = true
	defer func() {
		checker.inCreate = inCreate
	}()

	// TODO: maybe check that invoked expression is a composite constructor

	invocation := expression.InvocationExpression

	ty := invocation.Accept(checker).(Type)

	if ty.IsInvalidType() {
		return ty
	}

	// Check that the created expression is a resource

	compositeType, isCompositeType := ty.(*CompositeType)

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed

	if !isCompositeType || compositeType.Kind != common.CompositeKindResource {

		checker.report(
			&InvalidConstructionError{
				Range: ast.NewRangeFromPositioned(invocation),
			},
		)

		return ty
	}

	checker.checkResourceCreationOrDestruction(compositeType, invocation)

	return ty
}

// checkResourceCreationOrDestruction checks that the create or destroy expression occurs
// in the same contract that declares the composite, or if not contained in a contract,
// it occurs at least in the same location

func (checker *Checker) checkResourceCreationOrDestruction(compositeType *CompositeType, positioned ast.HasPosition) {

	contractType := containingContractKindedType(compositeType)

	if contractType == nil {
		if ast.LocationsMatch(compositeType.Location, checker.Location) {
			return
		}
	} else {
		if checker.containerTypes[contractType] {
			return
		}
	}

	checker.report(
		&InvalidResourceCreationError{
			Type:  compositeType,
			Range: ast.NewRangeFromPositioned(positioned),
		},
	)
}

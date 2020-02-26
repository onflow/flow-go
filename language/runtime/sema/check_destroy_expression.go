package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func (checker *Checker) VisitDestroyExpression(expression *ast.DestroyExpression) (resultType ast.Repr) {
	resultType = &VoidType{}

	valueType := expression.Expression.Accept(checker).(Type)

	checker.recordResourceInvalidation(
		expression.Expression,
		valueType,
		ResourceInvalidationKindDestroy,
	)

	// The destruction of any resource type (even compound resource types)

	if valueType.IsInvalidType() {
		return
	}

	if !valueType.IsResourceType() {

		checker.report(
			&InvalidDestructionError{
				Range: ast.NewRangeFromPositioned(expression.Expression),
			},
		)

		return
	}

	return
}

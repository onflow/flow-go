package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

func (checker *Checker) VisitDestroyExpression(expression *ast.DestroyExpression) (resultType ast.Repr) {
	resultType = &VoidType{}

	valueType := expression.Expression.Accept(checker).(Type)

	checker.recordResourceInvalidation(
		expression.Expression,
		valueType,
		ResourceInvalidationKindDestroy,
	)

	// destruction of any resource type (even compound resource types) is allowed:
	// the destructor of the resource type will be invoked

	if !valueType.IsResourceType() {

		checker.report(
			&InvalidDestructionError{
				StartPos: expression.Expression.StartPosition(),
				EndPos:   expression.Expression.EndPosition(),
			},
		)

		return
	}

	return
}

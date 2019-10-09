package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitDestroyExpression(expression *ast.DestroyExpression) (resultType ast.Repr) {
	resultType = &VoidType{}

	valueType := expression.Expression.Accept(checker).(Type)

	checker.recordResourceInvalidation(
		expression.Expression,
		valueType,
		ResourceInvalidationKindDestroy,
	)

	// TODO: allow destruction of any resource type (even compound resource types):
	//  Simply check `isResourceType`.
	//  See https://github.com/dapperlabs/flow-go/issues/816

	// NOTE: not using `isResourceType`,
	// as only direct resources and resource interfaces can be destructed (for now)

	compositeType, isCompositeType := valueType.(*CompositeType)
	isResource := isCompositeType &&
		compositeType.Kind == common.CompositeKindResource

	interfaceType, isInterfaceType := valueType.(*InterfaceType)
	isResourceInterface := isInterfaceType &&
		interfaceType.CompositeKind == common.CompositeKindResource

	if !isResource && !isResourceInterface {

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

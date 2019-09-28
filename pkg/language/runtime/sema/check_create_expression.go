package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitCreateExpression(expression *ast.CreateExpression) ast.Repr {

	checker.inCreate = true
	defer func() {
		checker.inCreate = false
	}()

	ty := expression.InvocationExpression.Accept(checker)

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed
	if compositeType, ok := ty.(*CompositeType); !ok ||
		compositeType.Kind != common.CompositeKindResource {

		checker.report(
			&InvalidConstructionError{
				StartPos: expression.InvocationExpression.StartPosition(),
				EndPos:   expression.InvocationExpression.EndPosition(),
			},
		)
	}

	return ty
}

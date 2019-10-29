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

	ty := expression.InvocationExpression.Accept(checker)

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed
	if compositeType, ok := ty.(*CompositeType); !ok ||
		compositeType.Kind != common.CompositeKindResource {

		checker.report(
			&InvalidConstructionError{
				Range: ast.NewRangeFromPositioned(expression.InvocationExpression),
			},
		)
	}

	return ty
}

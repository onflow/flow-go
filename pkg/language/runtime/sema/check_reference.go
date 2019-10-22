package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) VisitReferenceExpression(expression *ast.ReferenceExpression) (resultType ast.Repr) {
	// TODO:
	panic(&errors.UnreachableError{})
}

package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) VisitEmitStatement(*ast.EmitStatement) ast.Repr {
	panic(errors.UnreachableError{})
}

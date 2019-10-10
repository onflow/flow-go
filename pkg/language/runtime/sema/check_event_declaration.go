package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) VisitEventDeclaration(*ast.EventDeclaration) ast.Repr {
	// TODO: implement events
	panic(errors.UnreachableError{})
}

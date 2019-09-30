package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

type Variable struct {
	Kind common.DeclarationKind
	// Type is the type of the variable
	Type Type
	// IsConstant indicates if the variable is read-only
	IsConstant bool
	// Depth is the depth of scopes in which the variable was declared
	Depth int
	// ArgumentLabels are the argument labels that must be used in an invocation of the variable
	ArgumentLabels []string
	// Pos is the position where the variable was declared
	Pos *ast.Position
	// MovePos is the position where the resource-typed variable was moved
	MovePos *ast.Position
	// DestroyPos is the position where the resource-typed variable was destroyed
	DestroyPos *ast.Position
}

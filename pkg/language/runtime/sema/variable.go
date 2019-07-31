package sema

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type Variable struct {
	Declaration *ast.VariableDeclaration
	Depth       int
	Type        Type
}

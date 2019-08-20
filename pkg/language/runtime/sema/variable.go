package sema

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

type Variable struct {
	Kind           common.DeclarationKind
	IsConstant     bool
	Depth          int
	Type           Type
	ArgumentLabels []string
	Pos            *ast.Position
}

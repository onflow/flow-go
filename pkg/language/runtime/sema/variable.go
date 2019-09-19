package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

type Variable struct {
	Kind           common.DeclarationKind
	IsConstant     bool
	Depth          int
	Type           Type
	ArgumentLabels []string
	Pos            *ast.Position
}

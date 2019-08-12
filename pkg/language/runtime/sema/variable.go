package sema

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"

type Variable struct {
	IsConstant     bool
	Depth          int
	Type           Type
	ArgumentLabels []string
	Pos            *ast.Position
}

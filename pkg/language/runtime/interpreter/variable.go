package interpreter

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type Variable struct {
	Declaration *ast.VariableDeclaration
	Value       Value
}

func (v *Variable) Set(newValue Value) bool {
	// TODO: move to sema/checker
	if v.Declaration.IsConstant {
		return false
	}

	v.Value = newValue

	return true
}

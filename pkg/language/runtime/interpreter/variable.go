package interpreter

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type Variable struct {
	Declaration *ast.VariableDeclaration
	Depth       int
	Value       Value
}

func newVariable(declaration *ast.VariableDeclaration, depth int, value Value) *Variable {
	return &Variable{
		Declaration: declaration,
		Depth:       depth,
		Value:       value,
	}
}

func (v *Variable) Set(newValue Value) bool {
	if v.Declaration.IsConstant {
		return false
	}

	v.Value = newValue

	return true
}

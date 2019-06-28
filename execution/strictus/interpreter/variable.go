package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
)

type Variable struct {
	Declaration ast.VariableDeclaration
	Depth       int
	Type        Type
	Value       Value
}

func newVariable(declaration ast.VariableDeclaration, depth int, value Value) *Variable {
	var variableType Type
	if declaration.Type != nil {
		variableType = convertType(declaration.Type)
	}

	return &Variable{
		Declaration: declaration,
		Depth:       depth,
		Value:       value,
		Type:        variableType,
	}
}

func (v *Variable) Set(newValue Value) bool {
	if v.Declaration.IsConst {
		return false
	}

	// TODO: check type

	v.Value = newValue

	return true
}

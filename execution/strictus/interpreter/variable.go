package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
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

func (v *Variable) Set(newValue Value) {
	if v.Declaration.IsConst {
		// TODO: improve errors
		panic(fmt.Sprintf("can't assign to constant variable: %s", v.Declaration.Identifier))
	}

	// TODO: check type

	v.Value = newValue
}

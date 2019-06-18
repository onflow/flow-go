package interpreter

import (
	"bamboo-emulator/execution/strictus/ast"
	"github.com/raviqqe/hamt"
)

type Function struct {
	Expression ast.FunctionExpression
	Activation hamt.Map
}

func newFunction(expression ast.FunctionExpression, activation hamt.Map) *Function {
	return &Function{
		Expression: expression,
		Activation: activation,
	}
}

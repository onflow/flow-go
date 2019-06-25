package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"github.com/raviqqe/hamt"
)

type FunctionValue struct {
	Expression ast.FunctionExpression
	Activation hamt.Map
}

func (FunctionValue) isValue() {}

func newFunction(expression ast.FunctionExpression, activation hamt.Map) *FunctionValue {
	return &FunctionValue{
		Expression: expression,
		Activation: activation,
	}
}

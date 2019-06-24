package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"github.com/raviqqe/hamt"
)

type FunctionValue interface {
	Value
	isFunctionValue()
	invoke(interpreter *Interpreter, arguments []Value) Value
}

type InterpretedFunctionValue struct {
	Expression ast.FunctionExpression
	Activation hamt.Map
}

func (InterpretedFunctionValue) isValue()         {}
func (InterpretedFunctionValue) isFunctionValue() {}

func newInterpretedFunction(expression ast.FunctionExpression, activation hamt.Map) *InterpretedFunctionValue {
	return &InterpretedFunctionValue{
		Expression: expression,
		Activation: activation,
	}
}
func (f *InterpretedFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Value {
	return interpreter.invokeFunction(f, arguments)
}

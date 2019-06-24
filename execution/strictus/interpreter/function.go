package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"github.com/raviqqe/hamt"
)

// FunctionValue

type FunctionValue interface {
	Value
	isFunctionValue()
	invoke(interpreter *Interpreter, arguments []Value) Value
}

// InterpretedFunctionValue

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

// HostFunctionValue

type HostFunctionValue func(*Interpreter, []Value) Value

func (HostFunctionValue) isValue()           {}
func (f HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Value {
	return f(interpreter, arguments)
}

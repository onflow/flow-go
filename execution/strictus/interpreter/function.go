package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	. "bamboo-runtime/execution/strictus/trampoline"
	"github.com/raviqqe/hamt"
)

// FunctionValue

type FunctionValue interface {
	Value
	isFunctionValue()
	invoke(interpreter *Interpreter, arguments []Value) Trampoline
	parameterCount() int
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

func (f *InterpretedFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Trampoline {
	return interpreter.invokeInterpretedFunction(f, arguments)
}

func (f *InterpretedFunctionValue) parameterCount() int {
	return len(f.Expression.Parameters)
}

// HostFunctionValue

type HostFunctionValue struct {
	functionType FunctionType
	function     func(*Interpreter, []Value) Value
}

func (HostFunctionValue) isValue()           {}
func (f HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Trampoline {
	result := f.function(interpreter, arguments)
	return Done{Result: result}
}

func (f *HostFunctionValue) parameterCount() int {
	return len(f.functionType.ParameterTypes)
}

func NewHostFunction(
	functionType FunctionType,
	function func(*Interpreter, []Value) Value,
) *HostFunctionValue {
	return &HostFunctionValue{
		functionType: functionType,
		function:     function,
	}
}

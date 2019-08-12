package interpreter

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	. "github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
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
	Expression *ast.FunctionExpression
	Activation hamt.Map
}

func (InterpretedFunctionValue) isValue() {}

func (f InterpretedFunctionValue) Copy() Value {
	return f
}

func (InterpretedFunctionValue) isFunctionValue() {}

func newInterpretedFunction(expression *ast.FunctionExpression, activation hamt.Map) InterpretedFunctionValue {
	return InterpretedFunctionValue{
		Expression: expression,
		Activation: activation,
	}
}

func (f InterpretedFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Trampoline {
	return interpreter.invokeInterpretedFunction(f, arguments)
}

func (f InterpretedFunctionValue) parameterCount() int {
	return len(f.Expression.Parameters)
}

// HostFunctionValue

type HostFunctionValue struct {
	functionType *sema.FunctionType
	function     func(*Interpreter, []Value) Trampoline
}

func (HostFunctionValue) isValue() {}

func (f HostFunctionValue) Copy() Value {
	return f
}

func (HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) invoke(interpreter *Interpreter, arguments []Value) Trampoline {
	return f.function(interpreter, arguments)
}

func (f HostFunctionValue) parameterCount() int {
	return len(f.functionType.ParameterTypes)
}

func NewHostFunction(
	functionType *sema.FunctionType,
	function func(*Interpreter, []Value) Trampoline,
) HostFunctionValue {
	return HostFunctionValue{
		functionType: functionType,
		function:     function,
	}
}

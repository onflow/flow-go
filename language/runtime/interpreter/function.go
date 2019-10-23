package interpreter

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/trampoline"
)

// FunctionValue

type FunctionValue interface {
	Value
	isFunctionValue()
	invoke(arguments []Value, location Location) Trampoline
}

// InterpretedFunctionValue

type InterpretedFunctionValue struct {
	Interpreter *Interpreter
	Expression  *ast.FunctionExpression
	Type        *sema.FunctionType
	Activation  hamt.Map
}

func (InterpretedFunctionValue) isValue() {}

func (f InterpretedFunctionValue) Copy() Value {
	return f
}

func (InterpretedFunctionValue) isFunctionValue() {}

func newInterpretedFunction(
	interpreter *Interpreter,
	expression *ast.FunctionExpression,
	functionType *sema.FunctionType,
	activation hamt.Map,
) InterpretedFunctionValue {
	return InterpretedFunctionValue{
		Interpreter: interpreter,
		Expression:  expression,
		Type:        functionType,
		Activation:  activation,
	}
}

func (f InterpretedFunctionValue) invoke(arguments []Value, _ Location) Trampoline {
	return f.Interpreter.invokeInterpretedFunction(f, arguments)
}

// HostFunctionValue

type HostFunction func(arguments []Value, location Location) Trampoline

type HostFunctionValue struct {
	Function HostFunction
}

func (HostFunctionValue) isValue() {}

func (f HostFunctionValue) Copy() Value {
	return f
}

func (HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) invoke(arguments []Value, location Location) Trampoline {
	return f.Function(arguments, location)
}

func NewHostFunctionValue(
	function HostFunction,
) HostFunctionValue {
	return HostFunctionValue{
		Function: function,
	}
}

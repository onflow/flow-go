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
	invoke(interpreter *Interpreter, arguments []Value, position ast.Position) Trampoline
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

func (f InterpretedFunctionValue) invoke(interpreter *Interpreter, arguments []Value, _ ast.Position) Trampoline {
	return interpreter.invokeInterpretedFunction(f, arguments)
}

func (f InterpretedFunctionValue) parameterCount() int {
	return len(f.Expression.Parameters)
}

// HostFunctionValue

type HostFunction func(interpreter *Interpreter, arguments []Value, position ast.Position) Trampoline

type HostFunctionValue struct {
	Type     *sema.FunctionType
	Function HostFunction
}

func (HostFunctionValue) isValue() {}

func (f HostFunctionValue) Copy() Value {
	return f
}

func (HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) invoke(interpreter *Interpreter, arguments []Value, position ast.Position) Trampoline {
	return f.Function(interpreter, arguments, position)
}

func (f HostFunctionValue) parameterCount() int {
	return len(f.Type.ParameterTypes)
}

func NewHostFunctionValue(
	functionType *sema.FunctionType,
	function HostFunction,
) HostFunctionValue {
	return HostFunctionValue{
		Type:     functionType,
		Function: function,
	}
}

// StructureFunctionValue

type StructureFunctionValue struct {
	function  InterpretedFunctionValue
	structure StructureValue
}

func (*StructureFunctionValue) isValue() {}

func (*StructureFunctionValue) isFunctionValue() {}

func (f *StructureFunctionValue) parameterCount() int {
	// TODO:
	return 0
}

func (f *StructureFunctionValue) Copy() Value {
	functionCopy := *f
	return &functionCopy
}

func (f *StructureFunctionValue) CopyWithStructure(structure StructureValue) *StructureFunctionValue {
	functionCopy := *f
	functionCopy.structure = structure
	return &functionCopy
}

func (f *StructureFunctionValue) invoke(interpreter *Interpreter, arguments []Value, _ ast.Position) Trampoline {
	return interpreter.invokeStructureFunction(
		f.function,
		arguments,
		f.structure,
	)
}

func NewStructFunction(
	function InterpretedFunctionValue,
	structure StructureValue,
) *StructureFunctionValue {
	return &StructureFunctionValue{
		function,
		structure,
	}
}

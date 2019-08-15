package stdlib

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
)

type StandardLibraryFunction struct {
	Name     string
	Function interpreter.HostFunctionValue
}

func NewStandardLibraryFunction(
	name string,
	functionType *sema.FunctionType,
	function interpreter.HostFunction,
) StandardLibraryFunction {
	functionValue := interpreter.NewHostFunctionValue(
		functionType,
		function,
	)
	return StandardLibraryFunction{
		Name:     name,
		Function: functionValue,
	}
}

// AssertionError

type AssertionError struct {
	Message  string
	Position ast.Position
}

func (e AssertionError) StartPosition() ast.Position {
	return e.Position
}

func (e AssertionError) EndPosition() ast.Position {
	return e.Position
}

func (e AssertionError) Error() string {
	return fmt.Sprintf("assertion failed: %s", e.Message)
}

// Assertion

var Assert = NewStandardLibraryFunction(
	"assert",
	&sema.FunctionType{
		ParameterTypes: []sema.Type{
			&sema.BoolType{},
			&sema.StringType{},
		},
		ReturnType: &sema.VoidType{},
	},
	func(inter *interpreter.Interpreter, arguments []interpreter.Value, position ast.Position) trampoline.Trampoline {
		result := arguments[0].(interpreter.BoolValue)
		if !result {
			message := arguments[1].(interpreter.StringValue)
			panic(AssertionError{
				Message:  string(message),
				Position: position,
			})
		}
		return trampoline.Done{}
	},
)

var BuiltIns = []StandardLibraryFunction{
	Assert,
}

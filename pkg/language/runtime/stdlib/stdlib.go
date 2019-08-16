package stdlib

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
)

type StandardLibraryFunction struct {
	Name           string
	Function       interpreter.HostFunctionValue
	ArgumentLabels []string
}

func NewStandardLibraryFunction(
	name string,
	functionType *sema.FunctionType,
	function interpreter.HostFunction,
	argumentLabels []string,
) StandardLibraryFunction {
	functionValue := interpreter.NewHostFunctionValue(
		functionType,
		function,
	)
	return StandardLibraryFunction{
		Name:           name,
		Function:       functionValue,
		ArgumentLabels: argumentLabels,
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
	const message = "assertion failed"
	if e.Message == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, e.Message)
}

// Assertion

var assertRequiredArgumentCount = 1

var AssertFunction = NewStandardLibraryFunction(
	"assert",
	&sema.FunctionType{
		ParameterTypes: []sema.Type{
			&sema.BoolType{},
			&sema.StringType{},
		},
		ReturnType:            &sema.VoidType{},
		RequiredArgumentCount: &assertRequiredArgumentCount,
	},
	func(inter *interpreter.Interpreter, arguments []interpreter.Value, position ast.Position) trampoline.Trampoline {
		result := arguments[0].(interpreter.BoolValue)
		if !result {
			var message string
			if len(arguments) > 1 {
				message = string(arguments[1].(interpreter.StringValue))
			}
			panic(AssertionError{
				Message:  message,
				Position: position,
			})
		}
		return trampoline.Done{}
	},
	[]string{"", "message"},
)

// PanicError

type PanicError struct {
	Message  string
	Position ast.Position
}

func (e PanicError) StartPosition() ast.Position {
	return e.Position
}

func (e PanicError) EndPosition() ast.Position {
	return e.Position
}

func (e PanicError) Error() string {
	return fmt.Sprintf("panic: %s", e.Message)
}

// PanicFunction

var PanicFunction = NewStandardLibraryFunction(
	"panic",
	&sema.FunctionType{
		ParameterTypes: []sema.Type{
			&sema.StringType{},
		},
		ReturnType: &sema.VoidType{},
	},
	func(inter *interpreter.Interpreter, arguments []interpreter.Value, position ast.Position) trampoline.Trampoline {
		message := arguments[0].(interpreter.StringValue)
		panic(PanicError{
			Message:  string(message),
			Position: position,
		})
		return trampoline.Done{}
	},
	nil,
)

var BuiltIns = []StandardLibraryFunction{
	AssertFunction,
	PanicFunction,
}

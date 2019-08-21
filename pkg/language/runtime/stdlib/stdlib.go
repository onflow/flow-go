package stdlib

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
)

type StandardLibraryFunction struct {
	Name           string
	Function       interpreter.HostFunctionValue
	ArgumentLabels []string
}

func (f StandardLibraryFunction) DeclarationName() string {
	return f.Name
}

func (f StandardLibraryFunction) DeclarationType() sema.Type {
	return f.Function.Type
}

func (StandardLibraryFunction) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (StandardLibraryFunction) DeclarationPosition() ast.Position {
	return ast.Position{}
}

func (StandardLibraryFunction) DeclarationIsConstant() bool {
	return true
}

func (StandardLibraryFunction) DeclarationArgumentLabels() []string {
	return nil
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
		ReturnType: &sema.NeverType{},
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

// BuiltIns

var BuiltIns = []StandardLibraryFunction{
	AssertFunction,
	PanicFunction,
}

// Log

var Log = NewStandardLibraryFunction(
	"log",
	&sema.FunctionType{
		ParameterTypes: []sema.Type{&sema.AnyType{}},
		ReturnType:     &sema.VoidType{},
	},
	func(_ *interpreter.Interpreter, arguments []interpreter.Value, _ ast.Position) trampoline.Trampoline {
		fmt.Printf("%v\n", arguments[0])
		return trampoline.Done{Result: &interpreter.VoidValue{}}
	},
	nil,
)

// Helpers

var Helpers = []StandardLibraryFunction{
	Log,
}

// ToValueDeclarations

func ToValueDeclarations(functions []StandardLibraryFunction) []sema.ValueDeclaration {
	valueDeclarations := make([]sema.ValueDeclaration, len(functions))
	for i, function := range functions {
		valueDeclarations[i] = function
	}
	return valueDeclarations
}

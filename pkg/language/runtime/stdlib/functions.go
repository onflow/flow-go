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
	Type           *sema.FunctionType
	Function       interpreter.HostFunctionValue
	ArgumentLabels []string
}

func (f StandardLibraryFunction) ValueDeclarationType() sema.Type {
	return f.Type
}

func (StandardLibraryFunction) ValueDeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (StandardLibraryFunction) ValueDeclarationPosition() ast.Position {
	return ast.Position{}
}

func (StandardLibraryFunction) ValueDeclarationIsConstant() bool {
	return true
}

func (f StandardLibraryFunction) ValueDeclarationArgumentLabels() []string {
	return f.ArgumentLabels
}

func NewStandardLibraryFunction(
	name string,
	functionType *sema.FunctionType,
	function interpreter.HostFunction,
	argumentLabels []string,
) StandardLibraryFunction {
	functionValue := interpreter.NewHostFunctionValue(function)
	return StandardLibraryFunction{
		Name:           name,
		Type:           functionType,
		Function:       functionValue,
		ArgumentLabels: argumentLabels,
	}
}

// AssertionError

type AssertionError struct {
	Message  string
	Location interpreter.Location
}

func (e AssertionError) StartPosition() ast.Position {
	return e.Location.Position
}

func (e AssertionError) EndPosition() ast.Position {
	return e.Location.Position
}

func (e AssertionError) Error() string {
	const message = "assertion failed"
	if e.Message == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, e.Message)
}

func (e AssertionError) ImportLocation() ast.ImportLocation {
	return e.Location.ImportLocation
}

// AssertFunction

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
	func(arguments []interpreter.Value, location interpreter.Location) trampoline.Trampoline {
		result := arguments[0].(interpreter.BoolValue)
		if !result {
			var message string
			if len(arguments) > 1 {
				message = string(arguments[1].(interpreter.StringValue))
			}
			panic(AssertionError{
				Message:  message,
				Location: location,
			})
		}
		return trampoline.Done{}
	},
	[]string{"", "message"},
)

// PanicError

type PanicError struct {
	Message  string
	Location interpreter.Location
}

func (e PanicError) StartPosition() ast.Position {
	return e.Location.Position
}

func (e PanicError) EndPosition() ast.Position {
	return e.Location.Position
}

func (e PanicError) Error() string {
	return fmt.Sprintf("panic: %s", e.Message)
}

func (e PanicError) ImportLocation() ast.ImportLocation {
	return e.Location.ImportLocation
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
	func(arguments []interpreter.Value, location interpreter.Location) trampoline.Trampoline {
		message := arguments[0].(interpreter.StringValue)
		panic(PanicError{
			Message:  string(message),
			Location: location,
		})
		return trampoline.Done{}
	},
	nil,
)

// BuiltinFunctions

var BuiltinFunctions = []StandardLibraryFunction{
	AssertFunction,
	PanicFunction,
}

// LogFunction

var LogFunction = NewStandardLibraryFunction(
	"log",
	&sema.FunctionType{
		ParameterTypes: []sema.Type{&sema.AnyType{}},
		ReturnType:     &sema.VoidType{},
	},
	func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		fmt.Printf("%v\n", arguments[0])
		return trampoline.Done{Result: &interpreter.VoidValue{}}
	},
	nil,
)

// HelperFunctions

var HelperFunctions = []StandardLibraryFunction{
	LogFunction,
}

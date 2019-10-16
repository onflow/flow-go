package stdlib

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/trampoline"
)

type StandardLibraryFunction struct {
	Name           string
	Type           *sema.FunctionType
	Function       interpreter.HostFunctionValue
	ArgumentLabels []string
	IsDeclared     bool
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
	isDeclared bool,
) StandardLibraryFunction {
	functionValue := interpreter.NewHostFunctionValue(function)
	return StandardLibraryFunction{
		Name:           name,
		Type:           functionType,
		Function:       functionValue,
		ArgumentLabels: argumentLabels,
		IsDeclared:     isDeclared,
	}
}

// StandardLibraryFunctions

type StandardLibraryFunctions []StandardLibraryFunction

func (functions StandardLibraryFunctions) ToValueDeclarations() map[string]sema.ValueDeclaration {
	valueDeclarations := make(map[string]sema.ValueDeclaration, len(functions))
	for _, function := range functions {
		if function.IsDeclared {
			valueDeclarations[function.Name] = function
		}
	}
	return valueDeclarations
}

func (functions StandardLibraryFunctions) ToValues() map[string]interpreter.Value {
	values := make(map[string]interpreter.Value, len(functions))
	for _, function := range functions {
		values[function.Name] = function.Function
	}
	return values
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
		ParameterTypeAnnotations: sema.NewTypeAnnotations(
			&sema.BoolType{},
			&sema.StringType{},
		),
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.VoidType{},
		),
		RequiredArgumentCount: &assertRequiredArgumentCount,
	},
	func(arguments []interpreter.Value, location interpreter.Location) trampoline.Trampoline {
		result := arguments[0].(interpreter.BoolValue)
		if !result {
			var message string
			if len(arguments) > 1 {
				message = arguments[1].(interpreter.StringValue).StrValue()
			}
			panic(AssertionError{
				Message:  message,
				Location: location,
			})
		}
		return trampoline.Done{}
	},
	[]string{
		sema.ArgumentLabelNotRequired,
		"message",
	},
	false,
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
		ParameterTypeAnnotations: sema.NewTypeAnnotations(
			&sema.StringType{},
		),
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.NeverType{},
		),
	},
	func(arguments []interpreter.Value, location interpreter.Location) trampoline.Trampoline {
		message := arguments[0].(interpreter.StringValue)
		panic(PanicError{
			Message:  message.StrValue(),
			Location: location,
		})
		return trampoline.Done{}
	},
	nil,
	true,
)

// EmitEventFunction

var EmitEventFunction = NewStandardLibraryFunction(
	"emitEvent",
	&sema.FunctionType{
		ParameterTypeAnnotations: sema.NewTypeAnnotations(
			&sema.EventType{},
		),
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.VoidType{},
		),
	},
	func(arguments []interpreter.Value, location interpreter.Location) trampoline.Trampoline {
		eventValue := arguments[0].(interpreter.EventValue)

		fmt.Printf("Event emitted: %s\n", eventValue)

		return trampoline.Done{}
	},
	nil,
	false,
)

// BuiltinFunctions

var BuiltinFunctions = StandardLibraryFunctions{
	AssertFunction,
	PanicFunction,
	EmitEventFunction,
}

// LogFunction

var LogFunction = NewStandardLibraryFunction(
	"log",
	&sema.FunctionType{
		ParameterTypeAnnotations: sema.NewTypeAnnotations(
			&sema.AnyType{},
		),
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.VoidType{},
		),
	},
	func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		fmt.Printf("%v\n", arguments[0])
		return trampoline.Done{Result: &interpreter.VoidValue{}}
	},
	nil,
	true,
)

// HelperFunctions

var HelperFunctions = StandardLibraryFunctions{
	LogFunction,
}

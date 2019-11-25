package stdlib

import (
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
)

// This file defines functions built-in to Cadence.

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
	func(arguments []interpreter.Value, location interpreter.LocationPosition) trampoline.Trampoline {
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
)

// PanicError

type PanicError struct {
	Message  string
	Location interpreter.LocationPosition
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

func (e PanicError) ImportLocation() ast.Location {
	return e.Location.Location
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
	func(arguments []interpreter.Value, location interpreter.LocationPosition) trampoline.Trampoline {
		message := arguments[0].(interpreter.StringValue)
		panic(PanicError{
			Message:  message.StrValue(),
			Location: location,
		})
		return trampoline.Done{}
	},
	nil,
)

// BuiltinFunctions

var BuiltinFunctions = StandardLibraryFunctions{
	AssertFunction,
	PanicFunction,
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
	func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		fmt.Printf("%v\n", arguments[0])
		return trampoline.Done{Result: &interpreter.VoidValue{}}
	},
	nil,
)

// HelperFunctions

var HelperFunctions = StandardLibraryFunctions{
	LogFunction,
}

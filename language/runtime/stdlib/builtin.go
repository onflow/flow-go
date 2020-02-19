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

var AssertFunction = NewStandardLibraryFunction(
	"assert",
	&sema.FunctionType{
		Parameters: []*sema.Parameter{
			{
				Label:          sema.ArgumentLabelNotRequired,
				Identifier:     "condition",
				TypeAnnotation: sema.NewTypeAnnotation(&sema.BoolType{}),
			},
			{
				Identifier:     "message",
				TypeAnnotation: sema.NewTypeAnnotation(&sema.StringType{}),
			},
		},
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.VoidType{},
		),
		RequiredArgumentCount: (func() *int {
			var count = 1
			return &count
		})(),
	},
	func(invocation interpreter.Invocation) trampoline.Trampoline {
		result := invocation.Arguments[0].(interpreter.BoolValue)
		if !result {
			var message string
			if len(invocation.Arguments) > 1 {
				message = invocation.Arguments[1].(*interpreter.StringValue).Str
			}
			panic(AssertionError{
				Message:  message,
				Location: invocation.Location,
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
		Parameters: []*sema.Parameter{
			{
				Label:          sema.ArgumentLabelNotRequired,
				Identifier:     "message",
				TypeAnnotation: sema.NewTypeAnnotation(&sema.StringType{}),
			},
		},
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.NeverType{},
		),
	},
	func(invocation interpreter.Invocation) trampoline.Trampoline {
		message := invocation.Arguments[0].(*interpreter.StringValue)
		panic(PanicError{
			Message:  message.Str,
			Location: invocation.Location,
		})
	},
	nil,
)

var ArrayFunction = NewStandardLibraryFunction(
	"Array",
	&sema.FunctionType{
		Parameters: []*sema.Parameter{
			{
				Identifier:     "size",
				TypeAnnotation: sema.NewTypeAnnotation(&sema.IntType{}),
			},
			{
				Identifier: "generate",
				TypeAnnotation: sema.NewTypeAnnotation(
					&sema.FunctionType{
						Parameters: []*sema.Parameter{
							{
								Identifier:     "index",
								TypeAnnotation: sema.NewTypeAnnotation(&sema.IntType{}),
							},
						},
						ReturnTypeAnnotation: sema.NewTypeAnnotation(&sema.AnyType{}),
					},
				),
			},
		},
		ReturnTypeGetter: func(argumentTypes []sema.Type) sema.Type {
			generateFunctionType := argumentTypes[1].(*sema.FunctionType)
			elementType := generateFunctionType.ReturnTypeAnnotation.Type
			return &sema.VariableSizedType{
				Type: elementType,
			}
		},
	},
	func(invocation interpreter.Invocation) trampoline.Trampoline {
		count := invocation.Arguments[0].(interpreter.NumberValue).IntValue()
		generate := invocation.Arguments[1].(interpreter.FunctionValue)

		var elements []interpreter.Value

		var step func(i int) trampoline.Trampoline
		step = func(i int) trampoline.Trampoline {
			if i >= count {
				array := interpreter.NewArrayValueUnownedNonCopying(elements...)
				return trampoline.Done{Result: array}
			}

			generateInvocation := invocation
			generateInvocation.Arguments = []interpreter.Value{interpreter.NewIntValue(int64(i))}
			generateInvocation.ArgumentTypes = []sema.Type{&sema.IntType{}}

			return generate.Invoke(generateInvocation).
				FlatMap(func(value interface{}) trampoline.Trampoline {
					elements = append(elements, value.(interpreter.Value))

					return step(i + 1)
				})
		}

		return step(0)
	},
	nil,
)

// BuiltinFunctions

var BuiltinFunctions = StandardLibraryFunctions{
	AssertFunction,
	PanicFunction,
	ArrayFunction,
}

// LogFunction

var LogFunction = NewStandardLibraryFunction(
	"log",
	&sema.FunctionType{
		Parameters: []*sema.Parameter{
			{
				Label:          sema.ArgumentLabelNotRequired,
				Identifier:     "value",
				TypeAnnotation: sema.NewTypeAnnotation(&sema.AnyStructType{}),
			},
		},
		ReturnTypeAnnotation: sema.NewTypeAnnotation(
			&sema.VoidType{},
		),
	},
	func(invocation interpreter.Invocation) trampoline.Trampoline {
		fmt.Printf("%v\n", invocation.Arguments[0])
		result := interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	},
	nil,
)

// HelperFunctions

var HelperFunctions = StandardLibraryFunctions{
	LogFunction,
}

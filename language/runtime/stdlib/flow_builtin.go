package stdlib

import (
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
)

// This file defines functions built in to the Flow runtime.
// TODO: improve types
var setValueFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// owner
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// controller
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// value
		// TODO: add proper type
		&sema.IntType{},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var getValueFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// owner
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// controller
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	),
	// value
	// TODO: add proper type
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.IntType{},
	),
}

// TODO: improve types
var createAccountFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// publicKeys
		&sema.VariableSizedType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
		// code
		&sema.OptionalType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	),
	// value
	// TODO: add proper type
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.IntType{},
	),
}

// TODO: improve types
var addAccountKeyFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var removeAccountKeyFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
		// index
		&sema.IntType{},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var updateAccountCodeFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
		// code
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

var getAccountFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// TODO:
		// address
		&sema.StringType{},
	),
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		AccountType.Type,
	),
}

var logFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		&sema.AnyType{},
	),
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// FlowBuiltinImpls defines the set of functions needed to implement the Flow
// built-in functions.
type FlowBuiltinImpls struct {
	CreateAccount     func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
	AddAccountKey     func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
	RemoveAccountKey  func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
	UpdateAccountCode func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
	GetAccount        func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
	Log               func([]interpreter.Value, interpreter.LocationPosition) trampoline.Trampoline
}

// FlowBuiltInFunctions returns a list of standard library functions, bound to
// the provided implementation.
func FlowBuiltInFunctions(impls FlowBuiltinImpls) StandardLibraryFunctions {
	return StandardLibraryFunctions{
		NewStandardLibraryFunction(
			"createAccount",
			&createAccountFunctionType,
			impls.CreateAccount,
			nil,
		),
		NewStandardLibraryFunction(
			"addAccountKey",
			&addAccountKeyFunctionType,
			impls.AddAccountKey,
			nil,
		),
		NewStandardLibraryFunction(
			"removeAccountKey",
			&removeAccountKeyFunctionType,
			impls.RemoveAccountKey,
			nil,
		),
		NewStandardLibraryFunction(
			"updateAccountCode",
			&updateAccountCodeFunctionType,
			impls.UpdateAccountCode,
			nil,
		),
		NewStandardLibraryFunction(
			"getAccount",
			&getAccountFunctionType,
			impls.GetAccount,
			nil,
		),
		NewStandardLibraryFunction(
			"log",
			&logFunctionType,
			impls.Log,
			nil,
		),
	}
}

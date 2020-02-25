package stdlib

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

// This file defines functions built in to the Flow runtime.

var flowLocation = ast.StringLocation("flow")

// built-in function types

var accountFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "publicKeys",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.VariableSizedType{
					Type: &sema.VariableSizedType{
						Type: &sema.IntType{},
					},
				},
			),
		},
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "code",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.VariableSizedType{
					Type: &sema.IntType{},
				},
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.AccountType{},
	),
	// additional arguments are passed to the contract initializer
	RequiredArgumentCount: (func() *int {
		var count = 2
		return &count
	})(),
}

var getAccountFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "address",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.AddressType{},
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.PublicAccountType{},
	),
}

var logFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "value",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.AnyStructType{},
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

var getCurrentBlockFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(&BlockType{}),
}

// FlowBuiltinImpls defines the set of functions needed to implement the Flow
// built-in functions.
type FlowBuiltinImpls struct {
	CreateAccount   interpreter.HostFunction
	GetAccount      interpreter.HostFunction
	Log             interpreter.HostFunction
	GetCurrentBlock interpreter.HostFunction
}

// FlowBuiltInFunctions returns a list of standard library functions, bound to
// the provided implementation.
func FlowBuiltInFunctions(impls FlowBuiltinImpls) StandardLibraryFunctions {
	return StandardLibraryFunctions{
		NewStandardLibraryFunction(
			"Account",
			accountFunctionType,
			impls.CreateAccount,
			nil,
		),
		NewStandardLibraryFunction(
			"getAccount",
			getAccountFunctionType,
			impls.GetAccount,
			nil,
		),
		NewStandardLibraryFunction(
			"log",
			logFunctionType,
			impls.Log,
			nil,
		),
		NewStandardLibraryFunction(
			"getCurrentBlock",
			getCurrentBlockFunctionType,
			impls.GetCurrentBlock,
			nil,
		),
	}
}

// built-in event types

func newFlowEventType(identifier string, parameters ...*sema.Parameter) *sema.CompositeType {

	eventType := &sema.CompositeType{
		Kind:       common.CompositeKindEvent,
		Location:   flowLocation,
		Identifier: identifier,
		Members:    map[string]*sema.Member{},
	}

	for _, parameter := range parameters {

		eventType.Members[parameter.Identifier] =
			sema.NewPublicConstantFieldMember(
				eventType,
				parameter.Identifier,
				parameter.TypeAnnotation.Type,
			)

		eventType.ConstructorParameters = append(
			eventType.ConstructorParameters,
			parameter,
		)
	}

	return eventType
}

var AccountCreatedEventType = newFlowEventType(
	"AccountCreated",
	&sema.Parameter{
		Identifier: "address",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.StringType{},
		),
	},
)

var AccountKeyAddedEventType = newFlowEventType(
	"AccountKeyAdded",
	&sema.Parameter{
		Identifier: "address",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.StringType{},
		),
	},
	&sema.Parameter{
		Identifier: "publicKey",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		),
	},
)

var AccountKeyRemovedEventType = newFlowEventType(
	"AccountKeyRemoved",
	&sema.Parameter{
		Identifier: "address",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.StringType{},
		),
	},
	&sema.Parameter{
		Identifier: "publicKey",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		),
	},
)

var AccountCodeUpdatedEventType = newFlowEventType(
	"AccountCodeUpdated",
	&sema.Parameter{
		Identifier: "address",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.StringType{},
		),
	},
	&sema.Parameter{
		Identifier: "codeHash",
		TypeAnnotation: sema.NewTypeAnnotation(
			&sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		),
	},
)

// BlockType

type BlockType struct{}

func (*BlockType) IsType() {}

func (*BlockType) String() string {
	return "Block"
}

func (*BlockType) ID() sema.TypeID {
	return "Block"
}

func (*BlockType) Equal(other sema.Type) bool {
	_, ok := other.(*BlockType)
	return ok
}

func (*BlockType) IsResourceType() bool {
	return false
}

func (*BlockType) IsInvalidType() bool {
	return false
}

func (*BlockType) CanHaveMembers() bool {
	return true
}

const BlockIDSize = 32

func (t *BlockType) GetMember(identifier string, _ ast.Range, _ func(error)) *sema.Member {
	newField := func(fieldType sema.Type) *sema.Member {
		return sema.NewPublicConstantFieldMember(t, identifier, fieldType)
	}

	switch identifier {
	case "number":
		return newField(&sema.UInt64Type{})

	case "id":
		return newField(
			&sema.ConstantSizedType{
				Type: &sema.UInt8Type{},
				Size: BlockIDSize,
			},
		)

	case "previousBlock", "nextBlock":
		return newField(
			&sema.OptionalType{
				Type: &BlockType{},
			},
		)

	default:
		return nil
	}
}

var FlowBuiltInTypes = StandardLibraryTypes{
	StandardLibraryType{
		Name: "Block",
		Type: &BlockType{},
		Kind: common.DeclarationKindType,
	},
}

/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stdlib

import (
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/interpreter"
	"example.com/cadence-initial/runtime/sema"
)

// This file defines functions built in to the Flow runtime.

var flowLocation = ast.StringLocation("flow")

// built-in function types

var accountFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "payer",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.AuthAccountType{},
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.AuthAccountType{},
	),
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

var getBlockFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:      "at",
			Identifier: "height",
			TypeAnnotation: sema.NewTypeAnnotation(
				&sema.UInt64Type{},
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.OptionalType{
			Type: &BlockType{},
		},
	),
}

var unsafeRandomFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.UInt64Type{},
	),
}

// FlowBuiltinImpls defines the set of functions needed to implement the Flow
// built-in functions.
type FlowBuiltinImpls struct {
	CreateAccount   interpreter.HostFunction
	GetAccount      interpreter.HostFunction
	Log             interpreter.HostFunction
	GetCurrentBlock interpreter.HostFunction
	GetBlock        interpreter.HostFunction
	UnsafeRandom    interpreter.HostFunction
}

// FlowBuiltInFunctions returns a list of standard library functions, bound to
// the provided implementation.
func FlowBuiltInFunctions(impls FlowBuiltinImpls) StandardLibraryFunctions {
	return StandardLibraryFunctions{
		NewStandardLibraryFunction(
			"AuthAccount",
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
		NewStandardLibraryFunction(
			"getBlock",
			getBlockFunctionType,
			impls.GetBlock,
			nil,
		),
		NewStandardLibraryFunction(
			"unsafeRandom",
			unsafeRandomFunctionType,
			impls.UnsafeRandom,
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

const HashSize = 32

var HashType = &sema.ConstantSizedType{
	Size: HashSize,
	Type: &sema.UInt8Type{},
}

var TypeIDsType = &sema.VariableSizedType{
	Type: &sema.StringType{},
}

var AccountEventAddressParameter = &sema.Parameter{
	Identifier:     "address",
	TypeAnnotation: sema.NewTypeAnnotation(&sema.AddressType{}),
}

var AccountEventCodeHashParameter = &sema.Parameter{
	Identifier:     "codeHash",
	TypeAnnotation: sema.NewTypeAnnotation(HashType),
}

var AccountEventPublicKeyParameter = &sema.Parameter{
	Identifier: "publicKey",
	TypeAnnotation: sema.NewTypeAnnotation(
		&sema.VariableSizedType{
			Type: &sema.UInt8Type{},
		},
	),
}

var AccountEventContractsParameter = &sema.Parameter{
	Identifier:     "contracts",
	TypeAnnotation: sema.NewTypeAnnotation(TypeIDsType),
}

var AccountCreatedEventType = newFlowEventType(
	"AccountCreated",
	AccountEventAddressParameter,
)

var AccountKeyAddedEventType = newFlowEventType(
	"AccountKeyAdded",
	AccountEventAddressParameter,
	AccountEventPublicKeyParameter,
)

var AccountKeyRemovedEventType = newFlowEventType(
	"AccountKeyRemoved",
	AccountEventAddressParameter,
	AccountEventPublicKeyParameter,
)

var AccountCodeUpdatedEventType = newFlowEventType(
	"AccountCodeUpdated",
	AccountEventAddressParameter,
	AccountEventCodeHashParameter,
	AccountEventContractsParameter,
)

// BlockType

type BlockType struct{}

func (*BlockType) IsType() {}

func (*BlockType) String() string {
	return "Block"
}

func (*BlockType) QualifiedString() string {
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

func (*BlockType) TypeAnnotationState() sema.TypeAnnotationState {
	return sema.TypeAnnotationStateValid
}

func (*BlockType) IsInvalidType() bool {
	return false
}

func (*BlockType) ContainsFirstLevelInterfaceType() bool {
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
	case "height":
		return newField(&sema.UInt64Type{})

	case "timestamp":
		return newField(&sema.UFix64Type{})

	case "id":
		return newField(
			&sema.ConstantSizedType{
				Type: &sema.UInt8Type{},
				Size: BlockIDSize,
			},
		)

	default:
		return nil
	}
}

func (t *BlockType) Unify(_ sema.Type, _ map[*sema.TypeParameter]sema.Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *BlockType) Resolve(_ map[*sema.TypeParameter]sema.Type) sema.Type {
	return t
}

var FlowBuiltInTypes = StandardLibraryTypes{
	StandardLibraryType{
		Name: "Block",
		Type: &BlockType{},
		Kind: common.DeclarationKindType,
	},
}

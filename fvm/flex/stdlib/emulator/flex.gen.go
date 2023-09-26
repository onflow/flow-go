// Code generated from ../flex.cdc. DO NOT EDIT.
/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright Dapper Labs, Inc.
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

package emulator

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

var Flex_FlexAddressTypeConstructorType = &sema.FunctionType{
	IsConstructor: true,
	Parameters: []sema.Parameter{
		{
			Identifier: "bytes",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.ConstantSizedType{
				Type: sema.UInt8Type,
				Size: 20,
			}),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		Flex_FlexAddressType,
	),
}

const Flex_FlexAddressTypeConstructorDocString = `
Constructs a new Flex address from the given byte representation.
`

const Flex_FlexAddressTypeBytesFieldName = "bytes"

var Flex_FlexAddressTypeBytesFieldType = &sema.ConstantSizedType{
	Type: sema.UInt8Type,
	Size: 20,
}

const Flex_FlexAddressTypeBytesFieldDocString = `
Bytes of the address
`

const Flex_FlexAddressTypeName = "FlexAddress"

var Flex_FlexAddressType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier:         Flex_FlexAddressTypeName,
		Kind:               common.CompositeKindStructure,
	}

	return t
}()

func init() {
	var members = []*sema.Member{
		sema.NewUnmeteredFieldMember(
			Flex_FlexAddressType,
			ast.AccessPublic,
			ast.VariableKindConstant,
			Flex_FlexAddressTypeBytesFieldName,
			Flex_FlexAddressTypeBytesFieldType,
			Flex_FlexAddressTypeBytesFieldDocString,
		),
	}

	Flex_FlexAddressType.Members = sema.MembersAsMap(members)
	Flex_FlexAddressType.Fields = sema.MembersFieldNames(members)
	Flex_FlexAddressType.ConstructorParameters = Flex_FlexAddressTypeConstructorType.Parameters
}

const FlexTypeRunFunctionName = "run"

var FlexTypeRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier: "tx",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.VariableSizedType{
				Type: sema.UInt8Type,
			}),
		},
		{
			Identifier:     "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_FlexAddressType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.BoolType,
	),
}

const FlexTypeRunFunctionDocString = `
Run runs a flex transaction, deducts the gas fees and deposits them into the
provided coinbase address
`

const FlexTypeName = "Flex"

var FlexType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier:         FlexTypeName,
		Kind:               common.CompositeKindContract,
	}

	t.SetNestedType(Flex_FlexAddressTypeName, Flex_FlexAddressType)
	return t
}()

func init() {
	var members = []*sema.Member{
		// TODO: use NewUnmeteredConstructorMember
		sema.NewUnmeteredFunctionMember(
			FlexType,
			ast.AccessPublic,
			Flex_FlexAddressTypeName,
			Flex_FlexAddressTypeConstructorType,
			Flex_FlexAddressTypeConstructorDocString,
		),
		sema.NewUnmeteredFunctionMember(
			FlexType,
			ast.AccessPublic,
			FlexTypeRunFunctionName,
			FlexTypeRunFunctionType,
			FlexTypeRunFunctionDocString,
		),
	}

	FlexType.Members = sema.MembersAsMap(members)
	FlexType.Fields = sema.MembersFieldNames(members)
}

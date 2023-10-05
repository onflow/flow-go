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

var Flex_BalanceTypeConstructorType = &sema.FunctionType{
	IsConstructor: true,
	Parameters: []sema.Parameter{
		{
			Identifier:     "flow",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		Flex_BalanceType,
	),
}

const Flex_BalanceTypeConstructorDocString = `
Constructs a new balance, given the balance in FLOW.
`

const Flex_BalanceTypeFlowFieldName = "flow"

var Flex_BalanceTypeFlowFieldType = sema.UFix64Type

const Flex_BalanceTypeFlowFieldDocString = `
The balance in FLOW.
`

const Flex_BalanceTypeToAttoFlowFunctionName = "toAttoFlow"

var Flex_BalanceTypeToAttoFlowFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.UInt64Type,
	),
}

const Flex_BalanceTypeToAttoFlowFunctionDocString = `
Returns the balance in terms of atto-FLOW.
Atto-FLOW is the smallest denomination of FLOW inside Flex.
`

const Flex_BalanceTypeName = "Balance"

var Flex_BalanceType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier:         Flex_BalanceTypeName,
		Kind:               common.CompositeKindStructure,
	}

	return t
}()

func init() {
	var members = []*sema.Member{
		sema.NewUnmeteredFieldMember(
			Flex_BalanceType,
			ast.AccessPublic,
			ast.VariableKindConstant,
			Flex_BalanceTypeFlowFieldName,
			Flex_BalanceTypeFlowFieldType,
			Flex_BalanceTypeFlowFieldDocString,
		),
		sema.NewUnmeteredFunctionMember(
			Flex_BalanceType,
			ast.AccessPublic,
			Flex_BalanceTypeToAttoFlowFunctionName,
			Flex_BalanceTypeToAttoFlowFunctionType,
			Flex_BalanceTypeToAttoFlowFunctionDocString,
		),
	}

	Flex_BalanceType.Members = sema.MembersAsMap(members)
	Flex_BalanceType.Fields = sema.MembersFieldNames(members)
	Flex_BalanceType.ConstructorParameters = Flex_BalanceTypeConstructorType.Parameters
}

const Flex_FlowOwnedAccountTypeAddressFunctionName = "address"

var Flex_FlowOwnedAccountTypeAddressFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		Flex_FlexAddressType,
	),
}

const Flex_FlowOwnedAccountTypeAddressFunctionDocString = `
The address of the owned Flex account
`

const Flex_FlowOwnedAccountTypeDepositFunctionName = "deposit"

var Flex_FlowOwnedAccountTypeDepositFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "from",
			TypeAnnotation: sema.NewTypeAnnotation(FlowToken_VaultType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.VoidType,
	),
}

const Flex_FlowOwnedAccountTypeDepositFunctionDocString = `
Deposits the given vault into the Flex account's balance
`

const Flex_FlowOwnedAccountTypeWithdrawFunctionName = "withdraw"

var Flex_FlowOwnedAccountTypeWithdrawFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "balance",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_BalanceType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		FlowToken_VaultType,
	),
}

const Flex_FlowOwnedAccountTypeWithdrawFunctionDocString = `
Withdraws the balance from the Flex account's balance
`

const Flex_FlowOwnedAccountTypeDeployFunctionName = "deploy"

var Flex_FlowOwnedAccountTypeDeployFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier: "code",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.VariableSizedType{
				Type: sema.UInt8Type,
			}),
		},
		{
			Identifier:     "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Identifier:     "value",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_BalanceType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		Flex_FlexAddressType,
	),
}

const Flex_FlowOwnedAccountTypeDeployFunctionDocString = `
Deploys a contract to the Flex environment.
Returns the address of the newly deployed contract.
`

const Flex_FlowOwnedAccountTypeCallFunctionName = "call"

var Flex_FlowOwnedAccountTypeCallFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "to",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_FlexAddressType),
		},
		{
			Identifier: "data",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.VariableSizedType{
				Type: sema.UInt8Type,
			}),
		},
		{
			Identifier:     "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Identifier:     "value",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_BalanceType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VariableSizedType{
			Type: sema.UInt8Type,
		},
	),
}

const Flex_FlowOwnedAccountTypeCallFunctionDocString = `
Calls a function with the given data.
The execution is limited by the given amount of gas.
`

const Flex_FlowOwnedAccountTypeName = "FlowOwnedAccount"

var Flex_FlowOwnedAccountType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier:         Flex_FlowOwnedAccountTypeName,
		Kind:               common.CompositeKindResource,
	}

	return t
}()

func init() {
	var members = []*sema.Member{
		sema.NewUnmeteredFunctionMember(
			Flex_FlowOwnedAccountType,
			ast.AccessPublic,
			Flex_FlowOwnedAccountTypeAddressFunctionName,
			Flex_FlowOwnedAccountTypeAddressFunctionType,
			Flex_FlowOwnedAccountTypeAddressFunctionDocString,
		),
		sema.NewUnmeteredFunctionMember(
			Flex_FlowOwnedAccountType,
			ast.AccessPublic,
			Flex_FlowOwnedAccountTypeDepositFunctionName,
			Flex_FlowOwnedAccountTypeDepositFunctionType,
			Flex_FlowOwnedAccountTypeDepositFunctionDocString,
		),
		sema.NewUnmeteredFunctionMember(
			Flex_FlowOwnedAccountType,
			ast.AccessPublic,
			Flex_FlowOwnedAccountTypeWithdrawFunctionName,
			Flex_FlowOwnedAccountTypeWithdrawFunctionType,
			Flex_FlowOwnedAccountTypeWithdrawFunctionDocString,
		),
		sema.NewUnmeteredFunctionMember(
			Flex_FlowOwnedAccountType,
			ast.AccessPublic,
			Flex_FlowOwnedAccountTypeDeployFunctionName,
			Flex_FlowOwnedAccountTypeDeployFunctionType,
			Flex_FlowOwnedAccountTypeDeployFunctionDocString,
		),
		sema.NewUnmeteredFunctionMember(
			Flex_FlowOwnedAccountType,
			ast.AccessPublic,
			Flex_FlowOwnedAccountTypeCallFunctionName,
			Flex_FlowOwnedAccountTypeCallFunctionType,
			Flex_FlowOwnedAccountTypeCallFunctionDocString,
		),
	}

	Flex_FlowOwnedAccountType.Members = sema.MembersAsMap(members)
	Flex_FlowOwnedAccountType.Fields = sema.MembersFieldNames(members)
}

const FlexTypeCreateFlowOwnedAccountFunctionName = "createFlowOwnedAccount"

var FlexTypeCreateFlowOwnedAccountFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		Flex_FlowOwnedAccountType,
	),
}

const FlexTypeCreateFlowOwnedAccountFunctionDocString = `
Creates a new Flex account
`

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
Runs an a RLP-encoded Flex transaction,
deducts the gas fees and deposits them into the
provided coinbase address.

Returns true if the transaction was successful,
and returns false otherwise.
`

const FlexTypeName = "Flex"

var FlexType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier:         FlexTypeName,
		Kind:               common.CompositeKindContract,
	}

	t.SetNestedType(Flex_FlexAddressTypeName, Flex_FlexAddressType)
	t.SetNestedType(Flex_BalanceTypeName, Flex_BalanceType)
	t.SetNestedType(Flex_FlowOwnedAccountTypeName, Flex_FlowOwnedAccountType)
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
		// TODO: use NewUnmeteredConstructorMember
		sema.NewUnmeteredFunctionMember(
			FlexType,
			ast.AccessPublic,
			Flex_BalanceTypeName,
			Flex_BalanceTypeConstructorType,
			Flex_BalanceTypeConstructorDocString,
		),
		sema.NewUnmeteredFunctionMember(
			FlexType,
			ast.AccessPublic,
			FlexTypeCreateFlowOwnedAccountFunctionName,
			FlexTypeCreateFlowOwnedAccountFunctionType,
			FlexTypeCreateFlowOwnedAccountFunctionDocString,
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

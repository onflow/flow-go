package stdlib

import (
	_ "embed"
	"fmt"
	"regexp"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed contract.cdc
var contractCode string

//go:embed contract_minimal.cdc
var ContractMinimalCode string

var nftImportPattern = regexp.MustCompile(`(?m)^import "NonFungibleToken"`)
var fungibleTokenImportPattern = regexp.MustCompile(`(?m)^import "FungibleToken"`)
var flowTokenImportPattern = regexp.MustCompile(`(?m)^import "FlowToken"`)

func ContractCode(nonFungibleTokenAddress, fungibleTokenAddress, flowTokenAddress flow.Address) []byte {
	evmContract := nftImportPattern.ReplaceAllString(
		contractCode,
		fmt.Sprintf("import NonFungibleToken from %s", nonFungibleTokenAddress.HexWithPrefix()),
	)
	evmContract = fungibleTokenImportPattern.ReplaceAllString(
		evmContract,
		fmt.Sprintf("import FungibleToken from %s", fungibleTokenAddress.HexWithPrefix()),
	)
	evmContract = flowTokenImportPattern.ReplaceAllString(
		evmContract,
		fmt.Sprintf("import FlowToken from %s", flowTokenAddress.HexWithPrefix()),
	)
	return []byte(evmContract)
}

const (
	ContractName = "EVM"

	EVMAddressTypeBytesFieldName = "bytes"

	EVMBytesTypeValueFieldName = "value"

	EVMAddressTypeQualifiedIdentifier = "EVM.EVMAddress"

	EVMBalanceTypeQualifiedIdentifier = "EVM.Balance"

	EVMBytesTypeQualifiedIdentifier = "EVM.EVMBytes"

	EVMBytes4TypeQualifiedIdentifier = "EVM.EVMBytes4"

	EVMBytes32TypeQualifiedIdentifier = "EVM.EVMBytes32"

	EVMResultTypeQualifiedIdentifier        = "EVM.Result"
	EVMResultDecodedTypeQualifiedIdentifier = "EVM.ResultDecoded"
	EVMResultTypeStatusFieldName            = "status"
	EVMResultTypeErrorCodeFieldName         = "errorCode"
	EVMResultTypeErrorMessageFieldName      = "errorMessage"
	EVMResultTypeGasUsedFieldName           = "gasUsed"
	EVMResultTypeResultsFieldName           = "results"
	EVMResultTypeDataFieldName              = "data"
	EVMResultTypeDeployedContractFieldName  = "deployedContract"

	EVMStatusTypeQualifiedIdentifier = "EVM.Status"

	EVMBlockTypeQualifiedIdentifier = "EVM.EVMBlock"
)

const (
	EVMAddressLength = 20
	EVMBytes4Length  = 4
	EVMBytes32Length = 32
)

var (
	EVMTransactionBytesCadenceType = cadence.NewVariableSizedArrayType(cadence.UInt8Type)

	EVMTransactionBytesType       = sema.NewVariableSizedType(nil, sema.UInt8Type)
	EVMTransactionsBatchBytesType = sema.NewVariableSizedType(nil, EVMTransactionBytesType)
	EVMAddressBytesType           = sema.NewConstantSizedType(nil, sema.UInt8Type, EVMAddressLength)

	EVMAddressBytesStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, EVMAddressBytesType)

	EVMBytesValueStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, EVMTransactionBytesType)

	EVMBytes4ValueStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(
		nil,
		sema.NewConstantSizedType(nil, sema.UInt8Type, EVMBytes4Length),
	)

	EVMBytes32ValueStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(
		nil,
		sema.NewConstantSizedType(nil, sema.UInt8Type, EVMBytes32Length),
	)

	EVMAddressBytesCadenceType = cadence.NewConstantSizedArrayType(EVMAddressLength, cadence.UInt8Type)
)

// InternalEVM.encodeABI

const InternalEVMTypeEncodeABIFunctionName = "encodeABI"

var InternalEVMTypeEncodeABIFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "values",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewVariableSizedType(nil, sema.AnyStructType),
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// InternalEVM.decodeABI

const InternalEVMTypeDecodeABIFunctionName = "decodeABI"

var InternalEVMTypeDecodeABIFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier: "types",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewVariableSizedType(nil, sema.MetaType),
			),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.NewVariableSizedType(nil, sema.AnyStructType),
	),
}

// InternalEVM.run

const InternalEVMTypeRunFunctionName = "run"

var InternalEVMTypeRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "tx",
			TypeAnnotation: sema.NewTypeAnnotation(EVMTransactionBytesType),
		},
		{
			Label:          "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.dryRun

const InternalEVMTypeDryRunFunctionName = "dryRun"

var InternalEVMTypeDryRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "tx",
			TypeAnnotation: sema.NewTypeAnnotation(EVMTransactionBytesType),
		},
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.batchRun

const InternalEVMTypeBatchRunFunctionName = "batchRun"

var InternalEVMTypeBatchRunFunctionType *sema.FunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "txs",
			TypeAnnotation: sema.NewTypeAnnotation(EVMTransactionsBatchBytesType),
		},
		{
			Label:          "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	// Actually [EVM.Result], but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.NewVariableSizedType(nil, sema.AnyStructType)),
}

// InternalEVM.call

const InternalEVMTypeCallFunctionName = "call"

var InternalEVMTypeCallFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.callWithSigAndArgs

const InternalEVMTypeCallWithSigAndArgsFunctionName = "callWithSigAndArgs"

var InternalEVMTypeCallWithSigAndArgsFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "signature",
			TypeAnnotation: sema.NewTypeAnnotation(sema.StringType),
		},
		{
			Label:          "args",
			TypeAnnotation: sema.NewTypeAnnotation(sema.NewVariableSizedType(nil, sema.AnyStructType)),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
		{
			Label: "resultTypes",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewOptionalType(
					nil,
					sema.NewVariableSizedType(nil, sema.MetaType),
				),
			),
		},
	},
	// Actually EVM.ResultDecoded, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.dryCall

const InternalEVMTypeDryCallFunctionName = "dryCall"

var InternalEVMTypeDryCallFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.dryCallWithSigAndArgs

const InternalEVMTypeDryCallWithSigAndArgsFunctionName = "dryCallWithSigAndArgs"

var InternalEVMTypeDryCallWithSigAndArgsFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "signature",
			TypeAnnotation: sema.NewTypeAnnotation(sema.StringType),
		},
		{
			Label:          "args",
			TypeAnnotation: sema.NewTypeAnnotation(sema.NewVariableSizedType(nil, sema.AnyStructType)),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
		{
			Label: "resultTypes",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewOptionalType(
					nil,
					sema.NewVariableSizedType(nil, sema.MetaType),
				),
			),
		},
	},
	// Actually EVM.ResultDecoded, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.createCadenceOwnedAccount

const InternalEVMTypeCreateCadenceOwnedAccountFunctionName = "createCadenceOwnedAccount"

var InternalEVMTypeCreateCadenceOwnedAccountFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "uuid",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
}

// InternalEVM.deposit

const InternalEVMTypeDepositFunctionName = "deposit"

var InternalEVMTypeDepositFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(sema.AnyResourceType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.VoidType),
}

// InternalEVM.balance

const InternalEVMTypeBalanceFunctionName = "balance"

var InternalEVMTypeBalanceFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "address",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
}

// InternalEVM.nonce

const InternalEVMTypeNonceFunctionName = "nonce"

var InternalEVMTypeNonceFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "address",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
}

// InternalEVM.code

const InternalEVMTypeCodeFunctionName = "code"

var InternalEVMTypeCodeFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "address",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// InternalEVM.codeHash

const InternalEVMTypeCodeHashFunctionName = "codeHash"

var InternalEVMTypeCodeHashFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "address",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// InternalEVM.withdraw

const InternalEVMTypeWithdrawFunctionName = "withdraw"

var InternalEVMTypeWithdrawFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "amount",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyResourceType),
}

// InternalEVM.deploy

const InternalEVMTypeDeployFunctionName = "deploy"

var InternalEVMTypeDeployFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(EVMAddressBytesType),
		},
		{
			Label:          "code",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM.castToAttoFLOW

const InternalEVMTypeCastToAttoFLOWFunctionName = "castToAttoFLOW"

var InternalEVMTypeCastToAttoFLOWFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "balance",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
}

// InternalEVM.castToFLOW

const InternalEVMTypeCastToFLOWFunctionName = "castToFLOW"

var InternalEVMTypeCastToFLOWFunctionType = &sema.FunctionType{
	Purity: sema.FunctionPurityView,
	Parameters: []sema.Parameter{
		{
			Label:          "balance",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
}

// InternalEVM.commitBlockProposal

const InternalEVMTypeCommitBlockProposalFunctionName = "commitBlockProposal"

var InternalEVMTypeCommitBlockProposalFunctionType = &sema.FunctionType{
	Parameters:           []sema.Parameter{},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.VoidType),
}

// InternalEVM.getLatestBlock

const InternalEVMTypeGetLatestBlockFunctionName = "getLatestBlock"

var InternalEVMTypeGetLatestBlockFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{},
	// Actually EVM.Block, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
}

// InternalEVM

const InternalEVMContractName = "InternalEVM"

var InternalEVMContractType = func() *sema.CompositeType {
	ty := &sema.CompositeType{
		Identifier: InternalEVMContractName,
		Kind:       common.CompositeKindContract,
	}

	ty.Members = sema.MembersAsMap([]*sema.Member{
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeRunFunctionName,
			InternalEVMTypeRunFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDryRunFunctionName,
			InternalEVMTypeDryRunFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeBatchRunFunctionName,
			InternalEVMTypeBatchRunFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCreateCadenceOwnedAccountFunctionName,
			InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCallFunctionName,
			InternalEVMTypeCallFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCallWithSigAndArgsFunctionName,
			InternalEVMTypeCallWithSigAndArgsFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDryCallFunctionName,
			InternalEVMTypeDryCallFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDryCallWithSigAndArgsFunctionName,
			InternalEVMTypeDryCallWithSigAndArgsFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDepositFunctionName,
			InternalEVMTypeDepositFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeWithdrawFunctionName,
			InternalEVMTypeWithdrawFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDeployFunctionName,
			InternalEVMTypeDeployFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCastToAttoFLOWFunctionName,
			InternalEVMTypeCastToAttoFLOWFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCastToFLOWFunctionName,
			InternalEVMTypeCastToFLOWFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeBalanceFunctionName,
			InternalEVMTypeBalanceFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeNonceFunctionName,
			InternalEVMTypeNonceFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCodeFunctionName,
			InternalEVMTypeCodeFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCodeHashFunctionName,
			InternalEVMTypeCodeHashFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeEncodeABIFunctionName,
			InternalEVMTypeEncodeABIFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeDecodeABIFunctionName,
			InternalEVMTypeDecodeABIFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeGetLatestBlockFunctionName,
			InternalEVMTypeGetLatestBlockFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			InternalEVMTypeCommitBlockProposalFunctionName,
			InternalEVMTypeCommitBlockProposalFunctionType,
			"",
		),
	})
	return ty
}()

func newInternalEVMStandardLibraryValue(
	value interpreter.Value,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  InternalEVMContractName,
		Type:  InternalEVMContractType,
		Value: value,
		Kind:  common.DeclarationKindContract,
	}
}

var internalEVMStandardLibraryType = stdlib.StandardLibraryType{
	Name: InternalEVMContractName,
	Type: InternalEVMContractType,
	Kind: common.DeclarationKindContract,
}

func SetupEnvironment(
	env runtime.Environment,
	internalEVMValue interpreter.Value,
	contractAddress flow.Address,
) {
	location := common.NewAddressLocation(nil, common.Address(contractAddress), ContractName)

	env.DeclareType(
		internalEVMStandardLibraryType,
		location,
	)
	env.DeclareValue(
		newInternalEVMStandardLibraryValue(internalEVMValue),
		location,
	)
}

func NewEVMAddressCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		EVMAddressTypeQualifiedIdentifier,
		[]cadence.Field{
			{
				Identifier: "bytes",
				Type:       EVMAddressBytesCadenceType,
			},
		},
		nil,
	)
}

func NewBalanceCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		EVMBalanceTypeQualifiedIdentifier,
		[]cadence.Field{
			{
				Identifier: "attoflow",
				Type:       cadence.UIntType,
			},
		},
		nil,
	)
}

func NewEVMBlockCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		EVMBlockTypeQualifiedIdentifier,
		[]cadence.Field{
			{
				Identifier: "height",
				Type:       cadence.UInt64Type,
			},
			{
				Identifier: "hash",
				Type:       cadence.StringType,
			},
			{
				Identifier: "totalSupply",
				Type:       cadence.IntType,
			},
			{
				Identifier: "timestamp",
				Type:       cadence.UInt64Type,
			},
		},
		nil,
	)
}

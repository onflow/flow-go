package fvm

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func DeductTransactionFeesInvoker(ctx Context, payer flow.Address) *TransactionContractFunctionInvoker {
	return NewTransactionContractFunctionInvoker(
		common.AddressLocation{
			Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()),
			Name:    systemcontracts.ContractServiceAccount,
		},
		systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(payer.Bytes())),
		},
		[]sema.Type{
			sema.AuthAccountType,
		},
		ctx.Logger,
	)
}

func SetupNewAccountInvoker(ctx Context, flowAddress flow.Address, payer common.Address) *TransactionContractFunctionInvoker {
	// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
	return NewTransactionContractFunctionInvoker(
		common.AddressLocation{Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()), Name: systemcontracts.ContractServiceAccount},
		systemcontracts.ContractServiceAccountFunction_setupNewAccount,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(flowAddress.Bytes())),
			interpreter.NewAddressValue(common.BytesToAddress(payer.Bytes())),
		},
		[]sema.Type{
			sema.AuthAccountType,
			sema.AuthAccountType,
		},
		ctx.Logger,
	)
}

func AccountAvailableBalanceInvoker(ctx Context, address common.Address) *TransactionContractFunctionInvoker {
	return NewTransactionContractFunctionInvoker(
		common.AddressLocation{Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()), Name: systemcontracts.ContractStorageFees},
		systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(address.Bytes())),
		},
		[]sema.Type{
			&sema.AddressType{},
		},
		ctx.Logger,
	)
}

func AccountBalanceInvoker(ctx Context, address common.Address) *TransactionContractFunctionInvoker {
	return NewTransactionContractFunctionInvoker(
		common.AddressLocation{Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()), Name: systemcontracts.ContractServiceAccount},
		systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(address.Bytes())),
		},
		[]sema.Type{
			sema.PublicAccountType,
		},
		ctx.Logger,
	)
}

func AccountStorageCapacityInvoker(ctx Context, address common.Address) *TransactionContractFunctionInvoker {
	return NewTransactionContractFunctionInvoker(
		common.AddressLocation{Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()), Name: systemcontracts.ContractStorageFees},
		systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(address.Bytes())),
		},
		[]sema.Type{
			&sema.AddressType{},
		},
		ctx.Logger,
	)
}

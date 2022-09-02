package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

var deductTransactionFeeSpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractNameFlowFees,
	FunctionName: systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// InvokeDeductTransactionFeesContract executes the fee deduction contract on
// the service account.
func InvokeDeductTransactionFeesContract(
	env Environment,
	payer flow.Address,
	inclusionEffort uint64,
	executionEffort uint64,
) (cadence.Value, error) {
	return NewContractFunctionInvoker(env).Invoke(
		deductTransactionFeeSpec,
		FlowFeesAddress(env.Chain()),
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(executionEffort),
		},
	)
}

// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
var setupNewAccountSpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractServiceAccount,
	FunctionName: systemcontracts.ContractServiceAccountFunction_setupNewAccount,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.AuthAccountType,
	},
}

// InvokeSetupNewAccountContract executes the new account setup contract on
// the service account.
func InvokeSetupNewAccountContract(
	env Environment,
	flowAddress flow.Address,
	payer common.Address,
) (cadence.Value, error) {
	return NewContractFunctionInvoker(env).Invoke(
		setupNewAccountSpec,
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.BytesToAddress(flowAddress.Bytes()),
			cadence.BytesToAddress(payer.Bytes()),
		},
	)
}

var accountAvailableBalanceSpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractStorageFees,
	FunctionName: systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// InvokeAccountAvailableBalanceContract executes the get available balance
// contract on the storage fees contract.
func InvokeAccountAvailableBalanceContract(
	env Environment,
	address common.Address,
) (cadence.Value, error) {
	return NewContractFunctionInvoker(env).Invoke(
		accountAvailableBalanceSpec,
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountBalanceInvocationSpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractServiceAccount,
	FunctionName: systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
	ArgumentTypes: []sema.Type{
		sema.PublicAccountType,
	},
}

// InvokeAccountBalanceContract executes the get available balance contract
// on the service account.
func InvokeAccountBalanceContract(
	env Environment,
	address common.Address,
) (cadence.Value, error) {
	return NewContractFunctionInvoker(env).Invoke(
		accountBalanceInvocationSpec,
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountStorageCapacitySpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractStorageFees,
	FunctionName: systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// InvokeAccountStorageCapacityContract executes the get storage capacity
// contract on the storage fees contract.
func InvokeAccountStorageCapacityContract(
	env Environment,
	address common.Address,
) (cadence.Value, error) {
	return NewContractFunctionInvoker(env).Invoke(
		accountStorageCapacitySpec,
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

// InvokeAccountsStorageCapacity prepares a function that calls get storage capacity on the storage fees contract
// for multiple accounts at once
func InvokeAccountsStorageCapacity(
	env Environment,
	addresses []common.Address,
) (cadence.Value, error) {
	arrayValues := make([]cadence.Value, len(addresses))
	for i, address := range addresses {
		arrayValues[i] = cadence.BytesToAddress(address.Bytes())
	}

	return NewContractFunctionInvoker(env).Invoke(
		ContractFunctionSpec{
			LocationName: systemcontracts.ContractStorageFees,
			FunctionName: systemcontracts.ContractStorageFeesFunction_calculateAccountsCapacity,
			ArgumentTypes: []sema.Type{
				sema.NewConstantSizedType(
					nil,
					&sema.AddressType{},
					int64(len(arrayValues)),
				),
			},
		},
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.NewArray(arrayValues),
		},
	)
}

var useContractAuditVoucherSpec = ContractFunctionSpec{
	LocationName: systemcontracts.ContractDeploymentAudits,
	FunctionName: systemcontracts.ContractDeploymentAuditsFunction_useVoucherForDeploy,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
		sema.StringType,
	},
}

// InvokeUseContractAuditVoucherContract executes the use a contract
// deployment audit voucher contract.
func InvokeUseContractAuditVoucherContract(
	env Environment,
	address common.Address,
	code string,
) (bool, error) {
	resultCdc, err := NewContractFunctionInvoker(env).Invoke(
		useContractAuditVoucherSpec,
		env.Chain().ServiceAddress(),
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
			cadence.String(code),
		},
	)
	if err != nil {
		return false, err
	}
	result := resultCdc.(cadence.Bool).ToGoValue().(bool)
	return result, nil
}

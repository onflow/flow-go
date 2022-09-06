package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func FlowFeesAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(flowFeesAccountIndex)
	return address
}

func ServiceAddress(chain flow.Chain) flow.Address {
	return chain.ServiceAddress()
}

var deductTransactionFeeSpec = ContractFunctionSpec{
	AddressFromChain: FlowFeesAddress,
	LocationName:     systemcontracts.ContractNameFlowFees,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// DeductTransactionFees executes the fee deduction contract on the service
// account.
func (sys *SystemContracts) DeductTransactionFees(
	payer flow.Address,
	inclusionEffort uint64,
	executionEffort uint64,
) (cadence.Value, error) {
	return sys.Invoke(
		deductTransactionFeeSpec,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(executionEffort),
		},
	)
}

// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
var setupNewAccountSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_setupNewAccount,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.AuthAccountType,
	},
}

// SetupNewAccount executes the new account setup contract on the service
// account.
func (sys *SystemContracts) SetupNewAccount(
	flowAddress flow.Address,
	payer common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		setupNewAccountSpec,
		[]cadence.Value{
			cadence.BytesToAddress(flowAddress.Bytes()),
			cadence.BytesToAddress(payer.Bytes()),
		},
	)
}

var accountAvailableBalanceSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountAvailableBalance executes the get available balance contract on the
// storage fees contract.
func (sys *SystemContracts) AccountAvailableBalance(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountAvailableBalanceSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountBalanceInvocationSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
	ArgumentTypes: []sema.Type{
		sema.PublicAccountType,
	},
}

// AccountBalance executes the get available balance contract on the service
// account.
func (sys *SystemContracts) AccountBalance(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountBalanceInvocationSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountStorageCapacitySpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountStorageCapacity executes the get storage capacity contract on the
// service account.
func (sys *SystemContracts) AccountStorageCapacity(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountStorageCapacitySpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

// AccountsStorageCapacity gets storage capacity for multiple accounts at once.
func (sys *SystemContracts) AccountsStorageCapacity(
	addresses []common.Address,
) (cadence.Value, error) {
	arrayValues := make([]cadence.Value, len(addresses))
	for i, address := range addresses {
		arrayValues[i] = cadence.BytesToAddress(address.Bytes())
	}

	return sys.Invoke(
		ContractFunctionSpec{
			AddressFromChain: ServiceAddress,
			LocationName:     systemcontracts.ContractStorageFees,
			FunctionName:     systemcontracts.ContractStorageFeesFunction_calculateAccountsCapacity,
			ArgumentTypes: []sema.Type{
				sema.NewConstantSizedType(
					nil,
					&sema.AddressType{},
					int64(len(arrayValues)),
				),
			},
		},
		[]cadence.Value{
			cadence.NewArray(arrayValues),
		},
	)
}

var useContractAuditVoucherSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractDeploymentAudits,
	FunctionName:     systemcontracts.ContractDeploymentAuditsFunction_useVoucherForDeploy,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
		sema.StringType,
	},
}

// UseContractAuditVoucher executes the use a contract deployment audit voucher
// contract.
func (sys *SystemContracts) UseContractAuditVoucher(
	address common.Address,
	code string,
) (bool, error) {
	resultCdc, err := sys.Invoke(
		useContractAuditVoucherSpec,
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

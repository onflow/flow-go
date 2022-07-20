package fvm

import (
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

var deductTransactionFeesInvocationArgumentTypes = []sema.Type{
	sema.AuthAccountType,
	sema.UInt64Type,
	sema.UInt64Type,
}

// InvokeDeductTransactionFeesContract executes the fee deduction contract on
// the service account.
func InvokeDeductTransactionFeesContract(
	env Environment,
	traceSpan opentracing.Span,
	payer flow.Address,
	inclusionEffort uint64,
	executionEffort uint64,
) (cadence.Value, error) {

	feesAddress := FlowFeesAddress(env.Context().Chain)

	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(feesAddress),
			Name:    systemcontracts.ContractNameFlowFees,
		},
		systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(executionEffort),
		},
		deductTransactionFeesInvocationArgumentTypes,
		env.Context().Logger,
	)
	return invoker.Invoke(env, traceSpan)
}

var setupNewAccountInvocationArgumentTypes = []sema.Type{
	sema.AuthAccountType,
	sema.AuthAccountType,
}

// InvokeSetupNewAccountContract executes the new account setup contract on
// the service account.
func InvokeSetupNewAccountContract(
	env Environment,
	traceSpan opentracing.Span,
	flowAddress flow.Address,
	payer common.Address,
) (cadence.Value, error) {

	// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(env.Context().Chain.ServiceAddress()),
			Name:    systemcontracts.ContractServiceAccount,
		},
		systemcontracts.ContractServiceAccountFunction_setupNewAccount,
		[]cadence.Value{
			cadence.BytesToAddress(flowAddress.Bytes()),
			cadence.BytesToAddress(payer.Bytes()),
		},
		setupNewAccountInvocationArgumentTypes,
		env.Context().Logger,
	)
	return invoker.Invoke(env, traceSpan)
}

var accountAvailableBalanceInvocationArgumentTypes = []sema.Type{
	&sema.AddressType{},
}

// InvokeAccountAvailableBalanceContract executes the get available balance
// contract on the storage fees contract.
func InvokeAccountAvailableBalanceContract(
	env Environment,
	traceSpan opentracing.Span,
	address common.Address,
) (cadence.Value, error) {

	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(env.Context().Chain.ServiceAddress()),
			Name:    systemcontracts.ContractStorageFees,
		},
		systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
		accountAvailableBalanceInvocationArgumentTypes,
		env.Context().Logger,
	)
	return invoker.Invoke(env, traceSpan)
}

var accountBalanceInvocationArgumentTypes = []sema.Type{
	sema.PublicAccountType,
}

// InvokeAccountBalanceContract executes the get available balance contract
// on the service account.
func InvokeAccountBalanceContract(
	env Environment,
	traceSpan opentracing.Span,
	address common.Address,
) (cadence.Value, error) {

	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(env.Context().Chain.ServiceAddress()),
			Name:    systemcontracts.ContractServiceAccount},
		systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
		accountBalanceInvocationArgumentTypes,
		env.Context().Logger,
	)
	return invoker.Invoke(env, traceSpan)
}

var accountStorageCapacityInvocationArgumentTypes = []sema.Type{
	&sema.AddressType{},
}

// InvokeAccountStorageCapacityContract executes the get storage capacity
// contract on the storage fees contract.
func InvokeAccountStorageCapacityContract(
	env Environment,
	traceSpan opentracing.Span,
	address common.Address,
) (cadence.Value, error) {

	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(env.Context().Chain.ServiceAddress()),
			Name:    systemcontracts.ContractStorageFees,
		},
		systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
		accountStorageCapacityInvocationArgumentTypes,
		env.Context().Logger,
	)
	return invoker.Invoke(env, traceSpan)
}

// AccountsStorageCapacityInvocation prepares a function that calls get storage capacity on the storage fees contract
// for multiple accounts at once
func AccountsStorageCapacityInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(addresses []common.Address) (cadence.Value, error) {
	return func(addresses []common.Address) (cadence.Value, error) {
		arrayValues := make([]cadence.Value, len(addresses))
		for i, address := range addresses {
			arrayValues[i] = cadence.BytesToAddress(address.Bytes())
		}
		invoker := NewContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractStorageFees,
			},
			systemcontracts.ContractStorageFeesFunction_calculateAccountsCapacity,
			[]cadence.Value{
				cadence.NewArray(arrayValues),
			},
			[]sema.Type{
				sema.NewConstantSizedType(
					nil,
					&sema.AddressType{},
					int64(len(arrayValues)),
				),
			},
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

var useContractAuditVoucherInvocationArgumentTypes = []sema.Type{
	&sema.AddressType{},
	sema.StringType,
}

// InvokeUseContractAuditVoucherContract executes the use a contract
// deployment audit voucher contract.
func InvokeUseContractAuditVoucherContract(
	env Environment,
	traceSpan opentracing.Span,
	address common.Address,
	code string) (bool, error) {

	invoker := NewContractFunctionInvoker(
		common.AddressLocation{
			Address: common.Address(env.Context().Chain.ServiceAddress()),
			Name:    systemcontracts.ContractDeploymentAudits,
		},
		systemcontracts.ContractDeploymentAuditsFunction_useVoucherForDeploy,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
			cadence.String(code),
		},
		useContractAuditVoucherInvocationArgumentTypes,
		env.Context().Logger,
	)
	resultCdc, err := invoker.Invoke(env, traceSpan)
	if err != nil {
		return false, err
	}
	result := resultCdc.(cadence.Bool).ToGoValue().(bool)
	return result, nil
}

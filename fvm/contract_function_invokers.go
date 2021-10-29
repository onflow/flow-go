package fvm

import (
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

var deductTransactionFeesInvocationArgumentTypes = []sema.Type{
	sema.AuthAccountType,
}

// DeductTransactionFeesInvocation prepares a function that calls fee deduction on the service account
func DeductTransactionFeesInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(payer flow.Address) (cadence.Value, error) {
	return func(payer flow.Address) (cadence.Value, error) {
		invoker := NewTransactionContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractServiceAccount,
			},
			systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
			[]interpreter.Value{
				interpreter.NewAddressValue(common.Address(payer)),
			},
			deductTransactionFeesInvocationArgumentTypes,
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

var setupNewAccountInvocationArgumentTypes = []sema.Type{
	sema.AuthAccountType,
	sema.AuthAccountType,
}

// SetupNewAccountInvocation prepares a function that calls new account setup on the service account
func SetupNewAccountInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(flowAddress flow.Address, payer common.Address) (cadence.Value, error) {
	return func(flowAddress flow.Address, payer common.Address) (cadence.Value, error) {
		// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
		invoker := NewTransactionContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractServiceAccount,
			},
			systemcontracts.ContractServiceAccountFunction_setupNewAccount,
			[]interpreter.Value{
				interpreter.NewAddressValue(common.Address(flowAddress)),
				interpreter.NewAddressValue(payer),
			},
			setupNewAccountInvocationArgumentTypes,
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

var accountAvailableBalanceInvocationArgumentTypes = []sema.Type{
	&sema.AddressType{},
}

// AccountAvailableBalanceInvocation prepares a function that calls get available balance on the storage fees contract
func AccountAvailableBalanceInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(address common.Address) (cadence.Value, error) {
	return func(address common.Address) (cadence.Value, error) {
		invoker := NewTransactionContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractStorageFees,
			},
			systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
			[]interpreter.Value{
				interpreter.NewAddressValue(address),
			},
			accountAvailableBalanceInvocationArgumentTypes,
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

var accountBalanceInvocationArgumentTypes = []sema.Type{
	sema.PublicAccountType,
}

// AccountBalanceInvocation prepares a function that calls get available balance on the service account
func AccountBalanceInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(address common.Address) (cadence.Value, error) {
	return func(address common.Address) (cadence.Value, error) {
		invoker := NewTransactionContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractServiceAccount},
			systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
			[]interpreter.Value{
				interpreter.NewAddressValue(address),
			},
			accountBalanceInvocationArgumentTypes,
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

var accountStorageCapacityInvocationArgumentTypes = []sema.Type{
	&sema.AddressType{},
}

// AccountStorageCapacityInvocation prepares a function that calls get storage capacity on the storage fees contract
func AccountStorageCapacityInvocation(
	env Environment,
	traceSpan opentracing.Span,
) func(address common.Address) (cadence.Value, error) {
	return func(address common.Address) (cadence.Value, error) {
		invoker := NewTransactionContractFunctionInvoker(
			common.AddressLocation{
				Address: common.Address(env.Context().Chain.ServiceAddress()),
				Name:    systemcontracts.ContractStorageFees,
			},
			systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
			[]interpreter.Value{
				interpreter.NewAddressValue(address),
			},
			accountStorageCapacityInvocationArgumentTypes,
			env.Context().Logger,
		)
		return invoker.Invoke(env, traceSpan)
	}
}

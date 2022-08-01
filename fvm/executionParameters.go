package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/utils"
)

// GetExecutionEffortWeights reads stored execution effort weights from the service account
func GetExecutionEffortWeights(
	env Environment,
	service runtime.Address,
) (
	computationWeights meter.ExecutionEffortWeights,
	err error,
) {
	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionExecutionParametersPathDomain,
			Identifier: blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier,
		},
		runtime.Context{Interface: env},
	)
	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}

	computationWeightsRaw, ok := utils.CadenceValueToWeights(value)
	if !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionExecutionParametersPathDomain,
			blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier)
	}

	// Merge the default weights with the weights from the state.
	// This allows for weights that are not set in the state, to be set by default.
	// In case the network is stuck because of a transaction using an FVM feature that has 0 weight
	// (or is not metered at all), the defaults can be changed and the network restarted
	// instead of trying to change the weights with a transaction.
	computationWeights = make(meter.ExecutionEffortWeights, len(meter.DefaultComputationWeights))
	for k, v := range meter.DefaultComputationWeights {
		computationWeights[k] = v
	}
	for k, v := range computationWeightsRaw {
		computationWeights[common.ComputationKind(k)] = v
	}

	return computationWeights, nil
}

// GetExecutionMemoryWeights reads stored execution memory weights from the service account
func GetExecutionMemoryWeights(
	env Environment,
	service runtime.Address,
) (
	memoryWeights meter.ExecutionMemoryWeights,
	err error,
) {
	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionExecutionParametersPathDomain,
			Identifier: blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier,
		},
		runtime.Context{Interface: env},
	)
	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}

	memoryWeightsRaw, ok := utils.CadenceValueToWeights(value)
	if !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionExecutionParametersPathDomain,
			blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier)
	}

	// Merge the default weights with the weights from the state.
	// This allows for weights that are not set in the state, to be set by default.
	// In case the network is stuck because of a transaction using an FVM feature that has 0 weight
	// (or is not metered at all), the defaults can be changed and the network restarted
	// instead of trying to change the weights with a transaction.
	memoryWeights = make(meter.ExecutionMemoryWeights, len(meter.DefaultMemoryWeights))
	for k, v := range meter.DefaultMemoryWeights {
		memoryWeights[k] = v
	}
	for k, v := range memoryWeightsRaw {
		memoryWeights[common.MemoryKind(k)] = v
	}

	return memoryWeights, nil
}

// GetExecutionMemoryLimit reads the stored execution memory limit from the service account
func GetExecutionMemoryLimit(
	env Environment,
	service runtime.Address,
) (
	memoryLimit uint64,
	err error,
) {
	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionExecutionParametersPathDomain,
			Identifier: blueprints.TransactionFeesExecutionMemoryLimitPathIdentifier,
		},
		runtime.Context{Interface: env},
	)
	if err != nil {
		// this might be fatal, return as is
		return 0, err
	}

	memoryLimitRaw, ok := value.(cadence.UInt64)
	if value == nil || !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return 0, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionExecutionParametersPathDomain,
			blueprints.TransactionFeesExecutionMemoryLimitPathIdentifier)
	}

	return memoryLimitRaw.ToGoValue().(uint64), nil
}

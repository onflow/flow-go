package fvm

import (
	"context"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func getEnvironmentMeterParameters(
	vm *VirtualMachine,
	ctx Context,
	view state.View,
	programs *programs.Programs,
	procComputationLimit uint,
	procMemoryLimit uint64,
) (
	meter.MeterParameters,
	error,
) {
	params := meter.DefaultParameters().
		WithComputationLimit(procComputationLimit).
		WithMemoryLimit(procMemoryLimit)

	var err error
	if ctx.AllowContextOverrideByExecutionState {
		sth := state.NewStateHolder(
			state.NewState(
				view,
				meter.NewMeter(meter.DefaultParameters()),
				state.DefaultParameters().
					WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
					WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
					WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize)))

		sth.DisableAllLimitEnforcements()

		env := NewScriptEnvironment(context.Background(), ctx, vm, sth, programs)

		params, err = fillEnvironmentMeterParameters(ctx, env, params)
	}

	// TODO(patrick): disable memory/interaction limits for service account

	return params, err
}

func fillEnvironmentMeterParameters(
	ctx Context,
	env Environment,
	params meter.MeterParameters,
) (
	meter.MeterParameters,
	error,
) {

	// Check that the service account exists because all the settings are
	// stored in it
	serviceAddress := ctx.Chain.ServiceAddress()
	service := runtime.Address(serviceAddress)

	// set the property if no error, but if the error is a fatal error then
	// return it
	setIfOk := func(prop string, err error, setter func()) (fatal error) {
		err, fatal = errors.SplitErrorTypes(err)
		if fatal != nil {
			// this is a fatal error. return it
			ctx.Logger.
				Error().
				Err(fatal).
				Msgf("error getting %s", prop)
			return fatal
		}
		if err != nil {
			// this is a general error.
			// could be that no setting was present in the state,
			// or that the setting was not parseable,
			// or some other deterministic thing.
			ctx.Logger.
				Debug().
				Err(err).
				Msgf("could not set %s. Using defaults", prop)
			return nil
		}
		// everything is ok. do the setting
		setter()
		return nil
	}

	computationWeights, err := GetExecutionEffortWeights(env, service)
	err = setIfOk(
		"execution effort weights",
		err,
		func() { params = params.WithComputationWeights(computationWeights) })
	if err != nil {
		return params, err
	}

	memoryWeights, err := GetExecutionMemoryWeights(env, service)
	err = setIfOk(
		"execution memory weights",
		err,
		func() { params = params.WithMemoryWeights(memoryWeights) })
	if err != nil {
		return params, err
	}

	memoryLimit, err := GetExecutionMemoryLimit(env, service)
	err = setIfOk(
		"execution memory limit",
		err,
		func() { params = params.WithMemoryLimit(memoryLimit) })
	if err != nil {
		return params, err
	}

	return params, nil
}

func getExecutionWeights[KindType common.ComputationKind | common.MemoryKind](
	env Environment,
	service runtime.Address,
	path cadence.Path,
	defaultWeights map[KindType]uint64,
) (
	map[KindType]uint64,
	error,
) {
	value, err := env.VM().Runtime.ReadStored(
		service,
		path,
		runtime.Context{Interface: env},
	)

	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}

	weightsRaw, ok := utils.CadenceValueToWeights(value)
	if !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			path.Domain,
			path.Identifier)
	}

	// Merge the default weights with the weights from the state.
	// This allows for weights that are not set in the state, to be set by default.
	// In case the network is stuck because of a transaction using an FVM feature that has 0 weight
	// (or is not metered at all), the defaults can be changed and the network restarted
	// instead of trying to change the weights with a transaction.
	weights := make(map[KindType]uint64, len(defaultWeights))
	for k, v := range defaultWeights {
		weights[k] = v
	}
	for k, v := range weightsRaw {
		weights[KindType(k)] = v
	}

	return weights, nil
}

// GetExecutionEffortWeights reads stored execution effort weights from the service account
func GetExecutionEffortWeights(
	env Environment,
	service runtime.Address,
) (
	computationWeights meter.ExecutionEffortWeights,
	err error,
) {
	return getExecutionWeights(
		env,
		service,
		cadence.Path{
			Domain:     blueprints.TransactionExecutionParametersPathDomain,
			Identifier: blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier,
		},
		meter.DefaultComputationWeights)
}

// GetExecutionMemoryWeights reads stored execution memory weights from the service account
func GetExecutionMemoryWeights(
	env Environment,
	service runtime.Address,
) (
	memoryWeights meter.ExecutionMemoryWeights,
	err error,
) {
	return getExecutionWeights(
		env,
		service,
		cadence.Path{
			Domain:     blueprints.TransactionExecutionParametersPathDomain,
			Identifier: blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier,
		},
		meter.DefaultMemoryWeights)
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

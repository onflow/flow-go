package fvm

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/meter"
	basicMeter "github.com/onflow/flow-go/fvm/meter/basic"
	weightedMeter "github.com/onflow/flow-go/fvm/meter/weighted"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func createMeterFromEnv(env Environment, sth *state.StateHolder, l zerolog.Logger) (meter.Meter, error) {
	// do not meter getting execution weights
	sth.DisableAllLimitEnforcements()
	defer sth.EnableAllLimitEnforcements()

	// Get the meter to set the weights for
	m := sth.State().Meter()

	// Get the computation weights
	computationWeights, err := getExecutionEffortWeights(env)

	if err != nil {
		// This could be a reason to error the transaction in the future, but for now we just log it
		l.Info().
			Err(err).
			Msg("failed to get execution effort weights")

		return basicMeter.NewMeter(
			m.TotalComputationLimit(),
			m.TotalMemoryLimit()), nil
	}

	// Get the memory weights
	memoryWeights, err := getExecutionMemoryWeights(env)

	if err != nil {
		// This could be a reason to error the transaction in the future, but for now we just log it
		l.Info().Err(err).
			Msg("failed to get execution memory weights")

		return basicMeter.NewMeter(
			m.TotalComputationLimit(),
			m.TotalMemoryLimit()), nil
	}

	return weightedMeter.NewMeter(
		m.TotalComputationLimit(),
		m.TotalMemoryLimit(),
		computationWeights,
		memoryWeights), nil
}

// getExecutionEffortWeights reads stored execution effort weights from the service account
func getExecutionEffortWeights(env Environment) (map[uint]uint64, error) {
	service := runtime.Address(env.Context().Chain.ServiceAddress())

	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionFeesExecutionEffortWeightsPathDomain,
			Identifier: blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier,
		},
		runtime.Context{Interface: env},
	)

	if err != nil {
		return map[uint]uint64{}, err
	}
	weights, ok := utils.CadenceValueToUintUintMap(value)
	if !ok {
		return map[uint]uint64{}, fmt.Errorf("could not decode stored execution effort weights")
	}

	return weights, nil
}

// getExecutionMemoryWeights reads stored execution memory weights from the service account
func getExecutionMemoryWeights(_ Environment) (map[uint]uint64, error) {
	// TODO: implement when memory metering is ready
	return map[uint]uint64{}, nil
}

package fvm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	fvmmock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/runtime/testutil"
)

func TestGetExecutionMemoryWeights(t *testing.T) {
	address := common.Address{}

	setupEnvMock := func(readStored func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)) environment.Environment {
		envMock := &fvmmock.Environment{}
		envMock.On("BorrowCadenceRuntime", mock.Anything).Return(
			reusableRuntime.NewReusableCadenceRuntime(
				&testutil.TestInterpreterRuntime{
					ReadStoredFunc: readStored,
				},
				runtime.Config{},
			),
		)
		envMock.On("ReturnCadenceRuntime", mock.Anything).Return()
		return envMock
	}

	t.Run("return error if nothing is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, nil
				})
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				blueprints.TransactionFeesExecutionMemoryWeightsPath.String()).Error())
		},
	)
	t.Run("return error if can't parse stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewBool(false), nil
				})
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				blueprints.TransactionFeesExecutionMemoryWeightsPath.String()).Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, someErr.Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, someErr.Error())
		},
	)
	t.Run("no error if a dictionary is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.NoError(t, err)
		},
	)
	t.Run("return defaults if empty dict is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			weights, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, meter.DefaultMemoryWeights, weights, 0)
		},
	)
	t.Run("return merged if some dict is stored",
		func(t *testing.T) {
			expectedWeights := meter.ExecutionMemoryWeights{}
			var existingWeightKey common.MemoryKind
			var existingWeightValue uint64
			for k, v := range meter.DefaultMemoryWeights {
				expectedWeights[k] = v
			}
			// change one existing value
			for kind, u := range meter.DefaultMemoryWeights {
				existingWeightKey = kind
				existingWeightValue = u
				expectedWeights[kind] = u + 1
				break
			}
			expectedWeights[0] = 0

			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{
						{
							Value: cadence.UInt64(0),
							Key:   cadence.UInt64(0),
						}, // a new key
						{
							Value: cadence.UInt64(existingWeightValue + 1),
							Key:   cadence.UInt64(existingWeightKey),
						}, // existing key with new value
					}), nil
				})

			weights, err := fvm.GetExecutionMemoryWeights(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, expectedWeights, weights, 0)
		},
	)
}

func TestGetExecutionEffortWeights(t *testing.T) {
	address := common.Address{}

	setupEnvMock := func(readStored func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)) environment.Environment {
		envMock := &fvmmock.Environment{}
		envMock.On("BorrowCadenceRuntime", mock.Anything).Return(
			reusableRuntime.NewReusableCadenceRuntime(
				&testutil.TestInterpreterRuntime{
					ReadStoredFunc: readStored,
				},
				runtime.Config{},
			),
		)
		envMock.On("ReturnCadenceRuntime", mock.Anything).Return()
		return envMock
	}

	t.Run("return error if nothing is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, nil
				})
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				blueprints.TransactionFeesExecutionEffortWeightsPath.String()).Error())
		},
	)
	t.Run("return error if can't parse stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewBool(false), nil
				})
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				blueprints.TransactionFeesExecutionEffortWeightsPath.String()).Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, someErr.Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, someErr.Error())
		},
	)
	t.Run("no error if a dictionary is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.NoError(t, err)
		},
	)
	t.Run("return defaults if empty dict is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			weights, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, meter.DefaultComputationWeights, weights, 0)
		},
	)
	t.Run("return merged if some dict is stored",
		func(t *testing.T) {
			expectedWeights := meter.ExecutionEffortWeights{}
			var existingWeightKey common.ComputationKind
			var existingWeightValue uint64
			for k, v := range meter.DefaultComputationWeights {
				expectedWeights[k] = v
			}
			// change one existing value
			for kind, u := range meter.DefaultComputationWeights {
				existingWeightKey = kind
				existingWeightValue = u
				expectedWeights[kind] = u + 1
				break
			}
			expectedWeights[0] = 0

			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{
						{
							Value: cadence.UInt64(0),
							Key:   cadence.UInt64(0),
						}, // a new key
						{
							Value: cadence.UInt64(existingWeightValue + 1),
							Key:   cadence.UInt64(existingWeightKey),
						}, // existing key with new value
					}), nil
				})

			weights, err := fvm.GetExecutionEffortWeights(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, expectedWeights, weights, 0)
		},
	)
}

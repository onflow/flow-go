package fvm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	fvmmock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/runtime/testutil"
	"github.com/onflow/flow-go/model/flow"
)

func TestGetWeights(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		runTests[common.MemoryKind](
			t,
			func(env environment.Environment, service common.Address) (map[common.MemoryKind]uint64, error) {
				return fvm.GetExecutionMemoryWeights(env, service)
			},
			blueprints.TransactionFeesExecutionMemoryWeightsPath,
			meter.DefaultMemoryWeights,
		)
	})
	t.Run("execution effort", func(t *testing.T) {
		runTests[common.ComputationKind](
			t,
			func(env environment.Environment, service common.Address) (map[common.ComputationKind]uint64, error) {
				return fvm.GetExecutionEffortWeights(env, service)
			},
			blueprints.TransactionFeesExecutionEffortWeightsPath,
			meter.DefaultComputationWeights,
		)
	})
}

func runTests[T common.ComputationKind | common.MemoryKind](
	t *testing.T,
	f func(env environment.Environment, service common.Address) (map[T]uint64, error),
	path cadence.Path,
	defaultWeights map[T]uint64) {
	address := common.Address{}

	setupEnvMock := func(readStored func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)) environment.Environment {
		pool := reusableRuntime.NewCustomReusableCadenceRuntimePool(
			0,
			flow.Mainnet.Chain(),
			runtime.Config{},
			func(config runtime.Config) runtime.Runtime {
				return &testutil.TestRuntime{
					ReadStoredFunc: readStored,
				}
			},
		)

		envMock := &fvmmock.Environment{}
		envMock.On("BorrowCadenceRuntime", mock.Anything).Return(
			pool.Borrow(envMock, environment.CadenceTransactionRuntime),
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
			_, err := f(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				path.String()).Error())
		},
	)
	t.Run("return error if can't parse stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewBool(false), nil
				})
			_, err := f(envMock, address)
			require.Error(t, err)
			require.EqualError(t, err, errors.NewCouldNotGetExecutionParameterFromStateError(
				address.Hex(),
				path.String()).Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := f(envMock, address)
			require.Error(t, err)
			require.ErrorContains(t, err, someErr.Error())
		},
	)
	t.Run("return error if get stored returns error",
		func(t *testing.T) {
			someErr := fmt.Errorf("some error")
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return nil, someErr
				})
			_, err := f(envMock, address)
			require.Error(t, err)
			require.ErrorContains(t, err, someErr.Error())
		},
	)
	t.Run("no error if a dictionary is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			_, err := f(envMock, address)
			require.NoError(t, err)
		},
	)
	t.Run("return defaults if empty dict is stored",
		func(t *testing.T) {
			envMock := setupEnvMock(
				func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
					return cadence.NewDictionary([]cadence.KeyValuePair{}), nil
				})
			weights, err := f(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, defaultWeights, weights, 0)
		},
	)
	t.Run("return merged if some dict is stored",
		func(t *testing.T) {
			expectedWeights := make(map[T]uint64)
			var existingWeightKey T
			var existingWeightValue uint64
			for k, v := range defaultWeights {
				expectedWeights[k] = v
			}
			// change one existing value
			for kind, u := range defaultWeights {
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

			weights, err := f(envMock, address)
			require.NoError(t, err)
			require.InDeltaMapValues(t, expectedWeights, weights, 0)
		},
	)
}

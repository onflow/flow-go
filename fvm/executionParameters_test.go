package fvm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter/weighted"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
)

func TestGetExecutionMemoryWeights(t *testing.T) {
	address := common.Address{}

	setupEnvMock := func(readStored func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)) fvm.Environment {
		r := &TestReadStoredRuntime{
			readStored: readStored,
		}
		vm := fvm.VirtualMachine{
			Runtime: r,
		}
		envMock := &fvmmock.Environment{}
		envMock.On("VM").
			Return(&vm).
			Once()
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
				blueprints.TransactionExecutionParametersPathDomain,
				blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier).Error())
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
				blueprints.TransactionExecutionParametersPathDomain,
				blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier).Error())
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
			require.InDeltaMapValues(t, weighted.DefaultMemoryWeights, weights, 0)
		},
	)
	t.Run("return merged if some dict is stored",
		func(t *testing.T) {
			expectedWeights := weighted.ExecutionMemoryWeights{}
			var existingWeightKey common.MemoryKind
			var existingWeightValue uint64
			for k, v := range weighted.DefaultMemoryWeights {
				expectedWeights[k] = v
			}
			// change one existing value
			for kind, u := range weighted.DefaultMemoryWeights {
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
	) (cadence.Value, error)) fvm.Environment {
		r := &TestReadStoredRuntime{
			readStored: readStored,
		}
		vm := fvm.VirtualMachine{
			Runtime: r,
		}
		envMock := &fvmmock.Environment{}
		envMock.On("VM").
			Return(&vm).
			Once()
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
				blueprints.TransactionExecutionParametersPathDomain,
				blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier).Error())
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
				blueprints.TransactionExecutionParametersPathDomain,
				blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier).Error())
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
			require.InDeltaMapValues(t, weighted.DefaultComputationWeights, weights, 0)
		},
	)
	t.Run("return merged if some dict is stored",
		func(t *testing.T) {
			expectedWeights := weighted.ExecutionEffortWeights{}
			var existingWeightKey common.ComputationKind
			var existingWeightValue uint64
			for k, v := range weighted.DefaultComputationWeights {
				expectedWeights[k] = v
			}
			// change one existing value
			for kind, u := range weighted.DefaultComputationWeights {
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

var _ runtime.Runtime = &TestReadStoredRuntime{}

type TestReadStoredRuntime struct {
	readStored func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error)
}

func (t *TestReadStoredRuntime) SetDebugger(debugger *interpreter.Debugger) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) ExecuteScript(runtime.Script, runtime.Context) (cadence.Value, error) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) ExecuteTransaction(runtime.Script, runtime.Context) error {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) InvokeContractFunction(common.AddressLocation, string, []interpreter.Value, []sema.Type, runtime.Context) (cadence.Value, error) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) ParseAndCheckProgram([]byte, runtime.Context) (*interpreter.Program, error) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetCoverageReport(*runtime.CoverageReport) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetContractUpdateValidationEnabled(bool) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetAtreeValidationEnabled(bool) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetTracingEnabled(bool) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetInvalidatedResourceValidationEnabled(bool) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) SetResourceOwnerChangeHandlerEnabled(bool) {
	panic("not implemented")
}

func (t *TestReadStoredRuntime) ReadStored(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
	return t.readStored(address, path, context)
}

func (t *TestReadStoredRuntime) ReadLinked(common.Address, cadence.Path, runtime.Context) (cadence.Value, error) {
	panic("not implemented")
}

func (*TestReadStoredRuntime) Storage(runtime.Context) (*runtime.Storage, *interpreter.Interpreter, error) {
	panic("not implemented")
}

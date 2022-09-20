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
	"github.com/onflow/flow-go/fvm/meter"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
)

func TestGetExecutionMemoryWeights(t *testing.T) {
	address := common.Address{}

	setupEnvMock := func(readStored func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)) fvm.Environment {
		r := &TestInterpreterRuntime{
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
			_, err := fvm.GetExecutionMemoryWeights(envMock, address)
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
	) (cadence.Value, error)) fvm.Environment {
		r := &TestInterpreterRuntime{
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
			_, err := fvm.GetExecutionEffortWeights(envMock, address)
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

var _ runtime.Runtime = &TestInterpreterRuntime{}

type TestInterpreterRuntime struct {
	readStored             func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error)
	invokeContractFunction func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error)
}

func (t *TestInterpreterRuntime) NewScriptExecutor(script runtime.Script, context runtime.Context) runtime.Executor {
	panic("NewScriptExecutor not defined")
}

func (t *TestInterpreterRuntime) NewTransactionExecutor(script runtime.Script, context runtime.Context) runtime.Executor {
	panic("NewTransactionExecutor not defined")
}

func (t *TestInterpreterRuntime) NewContractFunctionExecutor(contractLocation common.AddressLocation, functionName string, arguments []cadence.Value, argumentTypes []sema.Type, context runtime.Context) runtime.Executor {
	panic("NewContractFunctionExecutor not defined")
}

func (t *TestInterpreterRuntime) SetDebugger(debugger *interpreter.Debugger) {
	panic("SetDebugger not defined")
}

func (t *TestInterpreterRuntime) ExecuteScript(runtime.Script, runtime.Context) (cadence.Value, error) {
	panic("ExecuteScript not defined")
}

func (t *TestInterpreterRuntime) ExecuteTransaction(runtime.Script, runtime.Context) error {
	panic("ExecuteTransaction not defined")
}

func (t *TestInterpreterRuntime) InvokeContractFunction(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
	if t.invokeContractFunction == nil {
		panic("InvokeContractFunction not defined")
	}
	return t.invokeContractFunction(a, s, values, types, ctx)
}

func (t *TestInterpreterRuntime) ParseAndCheckProgram([]byte, runtime.Context) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not defined")
}

func (t *TestInterpreterRuntime) SetCoverageReport(*runtime.CoverageReport) {
	panic("SetCoverageReport not defined")
}

func (t *TestInterpreterRuntime) SetContractUpdateValidationEnabled(bool) {
	panic("SetContractUpdateValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetAtreeValidationEnabled(bool) {
	panic("SetAtreeValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetTracingEnabled(bool) {
	panic("SetTracingEnabled not defined")
}

func (t *TestInterpreterRuntime) SetInvalidatedResourceValidationEnabled(bool) {
	panic("SetInvalidatedResourceValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetResourceOwnerChangeHandlerEnabled(bool) {
	panic("SetResourceOwnerChangeHandlerEnabled not defined")
}

func (t *TestInterpreterRuntime) ReadStored(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
	if t.readStored == nil {
		panic("ReadStored not defined")
	}
	return t.readStored(address, path, context)
}

func (t *TestInterpreterRuntime) ReadLinked(common.Address, cadence.Path, runtime.Context) (cadence.Value, error) {
	panic("ReadLinked not defined")
}

func (*TestInterpreterRuntime) Storage(runtime.Context) (*runtime.Storage, *interpreter.Interpreter, error) {
	panic("not implemented")
}

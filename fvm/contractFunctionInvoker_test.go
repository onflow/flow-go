package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	runtimeerrors "github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	fvmMock "github.com/onflow/flow-go/fvm/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

func TestSystemContractsInvoke(t *testing.T) {

	type testCase struct {
		name             string
		contractFunction func(
			a common.AddressLocation,
			s string,
			values []cadence.Value,
			types []sema.Type,
			ctx runtime.Context,
		) (cadence.Value, error)
		require func(t *testing.T, v cadence.Value, err error)
	}

	testCases := []testCase{
		{
			name: "noop",
			contractFunction: func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
				return nil, nil
			},
			require: func(t *testing.T, v cadence.Value, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "value gets returned as is",
			contractFunction: func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
				return cadence.String("value"), nil
			},
			require: func(t *testing.T, v cadence.Value, err error) {
				require.NoError(t, err)
				require.Equal(t, cadence.String("value"), v)
			},
		},
		{
			name: "unknown error from contract function is handled",
			contractFunction: func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
				return nil, fmt.Errorf("non-runtime error")
			},
			require: func(t *testing.T, v cadence.Value, err error) {
				require.Error(t, err)

				var failure errors.Failure
				require.ErrorAs(t, err, &failure)

				errors.As(err, &failure)

				require.Equal(t, errors.FailureCodeUnknownFailure, failure.FailureCode())
			},
		},
		{
			name: "known error from contract function is handled",
			contractFunction: func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
				return nil, runtime.Error{
					Err:      fmt.Errorf("some error"),
					Location: common.AddressLocation{},
				}
			},
			require: func(t *testing.T, v cadence.Value, err error) {
				require.Error(t, err)

				var rterr *errors.CadenceRuntimeError
				require.ErrorAs(t, err, &rterr)
			},
		},
		{
			name: "external error from contract function is unwrapped",
			contractFunction: func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
				return nil, runtime.Error{
					Err: runtimeerrors.ExternalError{
						Recovered: &errors.ContractNotFoundError{},
					},
					Location: common.AddressLocation{},
				}
			},
			require: func(t *testing.T, v cadence.Value, err error) {
				require.Error(t, err)

				var someFVMerr *errors.ContractNotFoundError
				require.ErrorAs(t, err, &someFVMerr)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := &fvmMock.Environment{}
			vm := &fvm.VirtualMachine{
				Runtime: &TestInterpreterRuntime{
					invokeContractFunction: tc.contractFunction,
				},
			}
			logger := zerolog.Logger{}
			chain := flow.Mainnet.Chain()

			env.On("StartSpanFromRoot", mock.Anything).Return(trace.NoopSpan)
			env.On("VM").Return(vm)
			env.On("Chain").Return(chain)
			env.On("Logger").Return(&logger)
			env.On("BorrowCadenceRuntime", mock.Anything).Return(
				fvm.NewReusableCadenceRuntime())
			env.On("ReturnCadenceRuntime", mock.Anything).Return()

			invoker := fvm.NewSystemContracts()
			invoker.SetEnvironment(env)
			value, err := invoker.Invoke(
				fvm.ContractFunctionSpec{
					AddressFromChain: func(_ flow.Chain) flow.Address {
						return flow.Address{}
					},
					FunctionName: "functionName",
				},
				[]cadence.Value{})

			tc.require(t, value, err)
		})
	}
}

package environment_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	fvmMock "github.com/onflow/flow-go/fvm/environment/mock"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/runtime/testutil"
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := &fvmMock.Environment{}
			logger := zerolog.Logger{}
			chain := flow.Mainnet.Chain()

			env.On("StartSpanFromRoot", mock.Anything).Return(trace.NoopSpan)
			env.On("Chain").Return(chain)
			env.On("Logger").Return(&logger)
			env.On("BorrowCadenceRuntime", mock.Anything).Return(
				reusableRuntime.NewReusableCadenceRuntime(
					&testutil.TestInterpreterRuntime{
						InvokeContractFunc: tc.contractFunction,
					}))
			env.On("ReturnCadenceRuntime", mock.Anything).Return()

			invoker := environment.NewSystemContracts()
			invoker.SetEnvironment(env)
			value, err := invoker.Invoke(
				environment.ContractFunctionSpec{
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

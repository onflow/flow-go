package environment_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/runtime/testutil"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
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
			const chainID = flow.Mainnet

			tracer := tracing.NewTracerSpan()
			runtimePool := reusableRuntime.NewCustomReusableCadenceRuntimePool(
				0,
				chainID.Chain(),
				runtime.Config{},
				func(_ runtime.Config) runtime.Runtime {
					return &testutil.TestRuntime{
						InvokeContractFunc: tc.contractFunction,
					}
				},
			)
			runtime := environment.NewRuntime(
				environment.RuntimeParams{
					ReusableCadenceRuntimePool: runtimePool,
				},
				environment.CadenceTransactionRuntime,
			)
			invoker := environment.NewSystemContracts(
				chainID.Chain(),
				tracer,
				environment.NewProgramLogger(
					tracer,
					environment.DefaultProgramLoggerParams()),
				runtime)
			value, err := invoker.Invoke(
				environment.ContractFunctionSpec{
					AddressFromChain: func(_ flow.Chain) flow.Address {
						return flow.EmptyAddress
					},
					FunctionName: "functionName",
				},
				[]cadence.Value{})

			tc.require(t, value, err)
		})
	}
}

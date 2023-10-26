package evm_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestEVMRun(t *testing.T) {

	t.Parallel()

	RunWithTestBackend(t, func(backend types.Backend) {
		RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithDeployedContract(t, backend, rootAddr, func(testContract *TestContract) {
				RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {
					num := int64(12)

					interEnv := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

					chainID := flow.Emulator
					service := chainID.Chain().ServiceAddress()

					err := evm.SetupEnvironment(chainID, backend, interEnv, service)
					require.NoError(t, err)

					inter := runtime.NewInterpreterRuntime(runtime.Config{})

					script := []byte(fmt.Sprintf(
						`
                          import EVM from %s

                          access(all)
                          fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): Bool {
                              let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
                              return EVM.run(tx: tx, coinbase: coinbase)
                          }
                        `,
						service.HexWithPrefix(),
					))

					gasLimit := uint64(100_000)

					txBytes := testAccount.PrepareSignAndEncodeTx(t,
						testContract.DeployedAt.ToCommon(),
						testContract.MakeStoreCallData(t, big.NewInt(num)),
						big.NewInt(0),
						gasLimit,
						big.NewInt(1),
					)

					tx := cadence.NewArray(
						ConvertToCadence(txBytes),
					).WithType(stdlib.EVMTransactionBytesCadenceType)

					coinbase := cadence.NewArray(
						ConvertToCadence(testAccount.Address().Bytes()),
					).WithType(stdlib.EVMAddressBytesCadenceType)

					accountCodes := map[common.Location][]byte{}
					var events []cadence.Event

					runtimeInterface := &TestRuntimeInterface{
						Storage:           backend,
						OnResolveLocation: SingleIdentifierLocationResolver(t),
						OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
							accountCodes[location] = code
							return nil
						},
						OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
							code = accountCodes[location]
							return code, nil
						},
						OnEmitEvent: func(event cadence.Event) error {
							events = append(events, event)
							return nil
						},
						OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
							return json.Decode(nil, b)
						},
					}

					result, err := inter.ExecuteScript(
						runtime.Script{
							Source:    script,
							Arguments: EncodeArgs([]cadence.Value{tx, coinbase}),
						},
						runtime.Context{
							Interface:   runtimeInterface,
							Environment: interEnv,
							Location:    common.ScriptLocation{},
						},
					)
					require.NoError(t, err)

					assert.Equal(t, cadence.Bool(true), result)
				})
			})
		})
	})
}

package query_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/offchain/query"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestView(t *testing.T) {

	const chainID = flow.Emulator
	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithDeployedContract(t,
				GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
					RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {

						h := SetupHandler(chainID, backend, rootAddr)

						view := query.NewView(
							chainID,
							rootAddr,
							storage.NewEphemeralStorage(
								backend,
							),
						)

						// test make balance query
						expectedBalance := h.AccountByAddress(testAccount.Address(), false).Balance()
						bal, err := view.GetBalance(testAccount.Address().ToCommon())
						require.NoError(t, err)
						require.True(t, types.BalancesAreEqual(expectedBalance, bal))

						// test make nonce query
						expectedNonce := h.AccountByAddress(testAccount.Address(), false).Nonce()
						nonce, err := view.GetNonce(testAccount.Address().ToCommon())
						require.NoError(t, err)
						require.Equal(t, expectedNonce, nonce)

						// test make code query
						expectedCode := h.AccountByAddress(testContract.DeployedAt, false).Code()
						code, err := view.GetCode(testContract.DeployedAt.ToCommon())
						require.NoError(t, err)
						require.True(t, bytes.Equal(expectedCode[:], code[:]))

						// test make code hash query
						expectedCodeHash := h.AccountByAddress(testContract.DeployedAt, false).CodeHash()
						codeHash, err := view.GetCodeHash(testContract.DeployedAt.ToCommon())
						require.NoError(t, err)
						require.True(t, bytes.Equal(expectedCodeHash[:], codeHash[:]))

						// test dry call
						// make call to test contract - set
						expectedFlowHeight := uint64(3)
						pc := &TestPrecompiledContract{
							RequiredGasFunc: func(input []byte) uint64 {
								return 1
							},
							RunFunc: func(input []byte) ([]byte, error) {
								output := make([]byte, 32)
								err := precompiles.EncodeUint64(expectedFlowHeight, output, 0)
								return output, err
							},
							AddressFunc: func() types.Address {
								// cadence arch address
								return handler.NewAddressAllocator().AllocatePrecompileAddress(1)
							},
						}
						res, err := view.DryCall(
							testAccount.Address().ToCommon(),
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t, "verifyArchCallToFlowBlockHeight", expectedFlowHeight),
							big.NewInt(0),
							uint64(1_000_000),
							big.NewInt(0),
							query.NewDryRunWithExtraPrecompiledContracts(
								[]types.PrecompiledContract{pc}),
						)
						require.NoError(t, err)
						require.NoError(t, res.ValidationError)
						require.NoError(t, res.VMError)

						// test dry call with overrides
						newBalance := big.NewInt(3000)
						res, err = view.DryCall(
							testAccount.Address().ToCommon(),
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t,
								"checkBalance",
								testAccount.Address().ToCommon(),
								newBalance),
							big.NewInt(0),
							uint64(1_000_000),
							big.NewInt(0),
							query.NewDryRunStorageOverrideBalance(
								testAccount.Address().ToCommon(),
								newBalance,
							),
						)
						require.NoError(t, err)
						require.NoError(t, res.ValidationError)
						require.NoError(t, res.VMError)

					})
				})
		})
	})
}

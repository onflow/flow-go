package query_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/query"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/testutils/contracts"
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

						blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
						require.NoError(t, err)

						maxCallGasLimit := params.MaxTxGas + 10_000_000
						view := query.NewView(
							chainID,
							rootAddr,
							storage.NewEphemeralStorage(
								backend,
							),
							blks,
							maxCallGasLimit,
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
							query.WithExtraPrecompiledContracts(
								[]types.PrecompiledContract{pc}),
						)
						require.NoError(t, err)
						require.NoError(t, res.ValidationError)
						require.NoError(t, res.VMError)

						// test dry call with balance state overrides
						newBalance := big.NewInt(3000)
						res, err = view.DryCall(
							testAccount.Address().ToCommon(),
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t,
								"checkBalance",
								testAccount.Address().ToCommon(),
								newBalance,
							),
							big.NewInt(0),
							uint64(1_000_000),
							query.WithStateOverrideBalance(
								testAccount.Address().ToCommon(),
								newBalance,
							),
						)
						require.NoError(t, err)
						require.NoError(t, res.ValidationError)
						require.NoError(t, res.VMError)

						// test we can go above the EIP-7825 max tx gas
						_, err = view.DryCall(
							testAccount.Address().ToCommon(),
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t,
								"store",
								big.NewInt(2),
							),
							big.NewInt(0),
							params.MaxTxGas+1_000,
						)
						require.NoError(t, err)

						// test we cannot go above the configured `maxCallGasLimit`
						_, err = view.DryCall(
							testAccount.Address().ToCommon(),
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t,
								"store",
								big.NewInt(2),
							),
							big.NewInt(0),
							maxCallGasLimit+1,
						)
						require.Error(t, err)
						require.ErrorContains(
							t,
							err,
							"gas limit is bigger than max gas limit allowed 26777217 > 26777216",
						)
					})
				})
		})
	})
}

func TestViewStateOverrides(t *testing.T) {

	const chainID = flow.Emulator
	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithDeployedContract(t,
				GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
					RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {

						t.Run("DryCall with WithStateOverrideState for existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newNumberValue := common.HexToHash("0x32") // 50 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideState(
									testContract.DeployedAt.ToCommon(),
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								newNumberValue,
								common.BytesToHash(res.ReturnedData),
							)
						})

						t.Run("DryCall with WithStateOverrideStateDiff for existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newNumberValue := common.HexToHash("0x64") // 100 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideStateDiff(
									testContract.DeployedAt.ToCommon(),
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								newNumberValue,
								common.BytesToHash(res.ReturnedData),
							)
						})

						t.Run("DryCall with WithStateOverrideState for non-existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newNumberValue := common.HexToHash("0x32") // 50 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideState(
									// this is a random address, without any code in it
									common.HexToAddress("0xD370975A6257fE8CeF93101799D602D30838BAad"),
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								common.Hash{0x0},
								common.BytesToHash(res.ReturnedData),
							)
						})

						t.Run("DryCall with WithStateOverrideStateDiff for existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newNumberValue := common.HexToHash("0x64") // 100 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideStateDiff(
									// this is a random address, without any code in it
									common.HexToAddress("0xCebE1e78Db8C757fe18E7EdfE5a3D99B5ca45c8d"),
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								common.Hash{0x0},
								common.BytesToHash(res.ReturnedData),
							)
						})

						t.Run("DryCall with WithStateOverrideCode and WithStateOverrideState for non-existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newContractAddress := common.HexToAddress("0xD370975A6257fE8CeF93101799D602D30838BAad")

							newNumberValue := common.HexToHash("0x32") // 50 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								newContractAddress,
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideCode(
									newContractAddress,
									contracts.TestContractBytes[17:], // we need the deployed byte-code
								),
								query.WithStateOverrideState(
									newContractAddress,
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								newNumberValue,
								common.BytesToHash(res.ReturnedData),
							)
						})

						t.Run("DryCall with WithStateOverrideCode and WithStateOverrideStateDiff for non-existing account", func(t *testing.T) {
							blks, err := blocks.NewBlocks(chainID, rootAddr, backend)
							require.NoError(t, err)

							maxCallGasLimit := uint64(5_000_000)
							view := query.NewView(
								chainID,
								rootAddr,
								storage.NewEphemeralStorage(
									backend,
								),
								blks,
								maxCallGasLimit,
							)

							newContractAddress := common.HexToAddress("0xD370975A6257fE8CeF93101799D602D30838BAad")

							newNumberValue := common.HexToHash("0x332") // 818 in hex
							res, err := view.DryCall(
								testAccount.Address().ToCommon(),
								newContractAddress,
								testContract.MakeCallData(t, "retrieve"),
								big.NewInt(0),
								uint64(1_000_000),
								query.WithStateOverrideCode(
									newContractAddress,
									contracts.TestContractBytes[17:], // we need the deployed byte-code
								),
								query.WithStateOverrideStateDiff(
									newContractAddress,
									map[common.Hash]common.Hash{
										{0x0}: newNumberValue,
									},
								),
							)
							require.NoError(t, err)
							require.NoError(t, res.ValidationError)
							require.NoError(t, res.VMError)
							require.Equal(
								t,
								newNumberValue,
								common.BytesToHash(res.ReturnedData),
							)
						})
					})
				})
		})
	})
}

package emulator_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var blockNumber = big.NewInt(10)
var defaultCtx = types.NewDefaultBlockContext(blockNumber.Uint64())

func RunWithNewEmulator(t testing.TB, backend *testutils.TestBackend, rootAddr flow.Address, f func(*emulator.Emulator)) {
	env := emulator.NewEmulator(backend, rootAddr)
	f(env)
}

func RunWithNewBlockView(t testing.TB, em *emulator.Emulator, f func(blk types.BlockView)) {
	blk, err := em.NewBlockView(defaultCtx)
	require.NoError(t, err)
	f(blk)
}

func RunWithNewReadOnlyBlockView(t testing.TB, em *emulator.Emulator, f func(blk types.ReadOnlyBlockView)) {
	blk, err := em.NewReadOnlyBlockView(defaultCtx)
	require.NoError(t, err)
	f(blk)
}

func TestNativeTokenBridging(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			originalBalance := big.NewInt(10000)
			testAccount := types.NewAddressFromString("test")
			bridgeAccount := types.NewAddressFromString("bridge")
			nonce := uint64(0)

			t.Run("mint tokens to the first account", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						call := types.NewDepositCall(bridgeAccount, testAccount, originalBalance, nonce)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						expectedHash, err := call.Hash()
						require.NoError(t, err)
						require.Equal(t, expectedHash, res.TxHash)
						nonce += 1
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						retBalance, err := blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, originalBalance, retBalance)
						// check balance of bridgeAccount to be zero

						retBalance, err = blk.BalanceOf(bridgeAccount)
						require.NoError(t, err)
						require.Equal(t, big.NewInt(0), retBalance)
					})
				})
			})
			t.Run("tokens withdraw", func(t *testing.T) {
				amount := big.NewInt(1000)
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						retBalance, err := blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, originalBalance, retBalance)
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						call := types.NewWithdrawCall(bridgeAccount, testAccount, amount, nonce)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						expectedHash, err := call.Hash()
						require.NoError(t, err)
						require.Equal(t, expectedHash, res.TxHash)
						nonce += 1
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						retBalance, err := blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(originalBalance, amount), retBalance)
						// check balance of bridgeAccount to be zero

						retBalance, err = blk.BalanceOf(bridgeAccount)
						require.NoError(t, err)
						require.Equal(t, big.NewInt(0), retBalance)
					})
				})
			})
		})
	})
}

func TestContractInteraction(t *testing.T) {
	t.Parallel()
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {

			testContract := testutils.GetStorageTestContract(t)

			testAccount := types.NewAddressFromString("test")
			bridgeAccount := types.NewAddressFromString("bridge")
			nonce := uint64(0)

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			// fund test account
			RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, nonce))
					require.NoError(t, err)
					nonce += 1
				})
			})

			var contractAddr types.Address

			t.Run("deploy contract", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						call := types.NewDeployCall(
							testAccount,
							testContract.ByteCode,
							math.MaxUint64,
							amountToBeTransfered,
							nonce)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						contractAddr = res.DeployedContractAddress
						expectedHash, err := call.Hash()
						require.NoError(t, err)
						require.Equal(t, expectedHash, res.TxHash)
						nonce += 1
					})
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						require.NotNil(t, contractAddr)
						retCode, err := blk.CodeOf(contractAddr)
						require.NoError(t, err)
						require.NotEmpty(t, retCode)

						retBalance, err := blk.BalanceOf(contractAddr)
						require.NoError(t, err)
						require.Equal(t, amountToBeTransfered, retBalance)

						retBalance, err = blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(amount, amountToBeTransfered), retBalance)
					})
				})
			})

			t.Run("call contract", func(t *testing.T) {
				num := big.NewInt(10)
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "store", num),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						require.GreaterOrEqual(t, res.GasConsumed, uint64(40_000))
						nonce += 1
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "retrieve"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						nonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedValue)
						require.Equal(t, num, ret)
						require.GreaterOrEqual(t, res.GasConsumed, uint64(23_000))
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "blockNumber"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						nonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedValue)
						require.Equal(t, blockNumber, ret)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					blk, err := em.NewBlockView(ctx)
					require.NoError(t, err)
					res, err := blk.DirectCall(
						types.NewContractCall(
							testAccount,
							contractAddr,
							testContract.MakeCallData(t, "chainID"),
							1_000_000,
							big.NewInt(0), // this should be zero because the contract doesn't have receiver
							nonce,
						),
					)
					require.NoError(t, err)
					nonce += 1

					ret := new(big.Int).SetBytes(res.ReturnedValue)
					require.Equal(t, types.FlowEVMPreviewNetChainID, ret)
				})
			})

			t.Run("test sending transactions (happy case)", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					ctx.GasFeeCollector = types.NewAddressFromString("coinbase")
					coinbaseOrgBalance := gethCommon.Big1
					// small amount of money to create account
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, ctx.GasFeeCollector, coinbaseOrgBalance, 0))
						require.NoError(t, err)
					})

					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := account.PrepareAndSignTx(
						t,
						testAccount.ToCommon(), // to
						nil,                    // data
						big.NewInt(1000),       // amount
						gethParams.TxGas,       // gas limit
						gethCommon.Big1,        // gas fee

					)
					res, err := blk.RunTransaction(tx)
					require.NoError(t, err)
					require.NoError(t, res.VMError)
					require.Greater(t, res.GasConsumed, uint64(0))

					// check the balance of coinbase
					RunWithNewReadOnlyBlockView(t, env, func(blk2 types.ReadOnlyBlockView) {
						bal, err := blk2.BalanceOf(ctx.GasFeeCollector)
						require.NoError(t, err)
						expected := gethParams.TxGas*gethCommon.Big1.Uint64() + gethCommon.Big1.Uint64()
						require.Equal(t, expected, bal.Uint64())

						nonce, err := blk2.NonceOf(fAddr)
						require.NoError(t, err)
						require.Equal(t, 1, int(nonce))
					})
				})
			})

			t.Run("test runing transactions with dynamic fees (happy case)", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})
				account.SetNonce(account.Nonce() + 1)

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					ctx.GasFeeCollector = types.NewAddressFromString("coinbase")
					coinbaseOrgBalance := gethCommon.Big1
					// small amount of money to create account
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, ctx.GasFeeCollector, coinbaseOrgBalance, 1))
						require.NoError(t, err)
					})

					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := account.SignTx(
						t,
						gethTypes.NewTx(&gethTypes.DynamicFeeTx{
							ChainID:   types.FlowEVMPreviewNetChainID,
							Nonce:     account.Nonce(),
							GasTipCap: big.NewInt(2),
							GasFeeCap: big.NewInt(3),
							Gas:       gethParams.TxGas,
							To:        &gethCommon.Address{},
							Value:     big.NewInt(1),
						}),
					)
					account.SetNonce(account.Nonce() + 1)

					res, err := blk.RunTransaction(tx)
					require.NoError(t, err)
					require.NoError(t, res.VMError)
					require.Greater(t, res.GasConsumed, uint64(0))
				})
			})

			t.Run("test sending transactions (invalid nonce)", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := account.SignTx(t,
						gethTypes.NewTransaction(
							100,                    // nonce
							testAccount.ToCommon(), // to
							big.NewInt(1000),       // amount
							gethParams.TxGas,       // gas limit
							gethCommon.Big1,        // gas fee
							nil,                    // data
						),
					)
					_, err = blk.RunTransaction(tx)
					require.Error(t, err)
					require.True(t, types.IsEVMValidationError(err))
				})
			})

			t.Run("test sending transactions (bad signature)", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := gethTypes.NewTx(&gethTypes.LegacyTx{
						Nonce:    0,
						GasPrice: gethCommon.Big1,
						Gas:      gethParams.TxGas, // gas limit
						To:       nil,              // to
						Value:    big.NewInt(1000), // amount
						Data:     nil,              // data
						V:        big.NewInt(1),
						R:        big.NewInt(2),
						S:        big.NewInt(3),
					})
					_, err = blk.RunTransaction(tx)
					require.Error(t, err)
					require.True(t, types.IsEVMValidationError(err))
				})
			})
		})
	})
}

func TestDeployAtFunctionality(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			testContract := testutils.GetStorageTestContract(t)
			testAccount := types.NewAddressFromString("test")
			bridgeAccount := types.NewAddressFromString("bridge")

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			// fund test account
			RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, 0))
					require.NoError(t, err)
				})
			})

			t.Run("deploy contract at target address", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					target := types.Address{1, 2, 3}
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								target,
								testContract.ByteCode,
								math.MaxUint64,
								amountToBeTransfered,
								0,
							),
						)
						require.NoError(t, err)
						require.Equal(t, target, res.DeployedContractAddress)
					})
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						require.NotNil(t, target)
						retCode, err := blk.CodeOf(target)
						require.NoError(t, err)
						require.NotEmpty(t, retCode)

						retBalance, err := blk.BalanceOf(target)
						require.NoError(t, err)
						require.Equal(t, amountToBeTransfered, retBalance)

						retBalance, err = blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(amount, amountToBeTransfered), retBalance)
					})
					// test deployment to an address that is already exist
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								target,
								testContract.ByteCode,
								math.MaxUint64,
								amountToBeTransfered,
								0),
						)
						require.NoError(t, err)
						require.Equal(t, gethVM.ErrContractAddressCollision, res.VMError)
					})
					// test deployment with not enough gas
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								types.Address{3, 4, 5},
								testContract.ByteCode,
								100,
								new(big.Int),
								0),
						)
						require.NoError(t, err)
						require.Equal(t, fmt.Errorf("out of gas"), res.VMError)
					})
				})
			})
		})
	})
}

// Self destruct test deploys a contract with a selfdestruct function
// this function is called and we make sure the balance the contract had
// is returned to the address provided, and the contract data stays according to the
// EIP 6780 https://eips.ethereum.org/EIPS/eip-6780 in case where the selfdestruct
// is not caleld in the same transaction as deployment.
func TestSelfdestruct(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			testutils.RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *testutils.EOATestAccount) {

				testContract := testutils.GetStorageTestContract(t)
				testAddress := types.NewAddressFromString("testaddr")
				bridgeAccount := types.NewAddressFromString("bridge")

				startBalance := big.NewInt(0).Mul(big.NewInt(1000), big.NewInt(gethParams.Ether))
				deployBalance := big.NewInt(0).Mul(big.NewInt(10), big.NewInt(gethParams.Ether))
				var contractAddr types.Address

				// setup the test with funded account and deploying a selfdestruct contract.
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAddress, startBalance, 0))
						require.NoError(t, err)
					})

					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCall(
								testAddress,
								testContract.ByteCode,
								math.MaxUint64,
								deployBalance,
								0),
						)
						require.NoError(t, err)
						contractAddr = res.DeployedContractAddress
					})

					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						bal, err := blk.BalanceOf(testAddress)
						require.NoError(t, err)
						require.Equal(t, big.NewInt(0).Sub(startBalance, deployBalance), bal)

						bal, err = blk.BalanceOf(contractAddr)
						require.NoError(t, err)
						require.Equal(t, deployBalance, bal)
					})

					// call the destroy method which executes selfdestruct call.
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(&types.DirectCall{
							Type:     types.DirectCallTxType,
							From:     testAddress,
							To:       contractAddr,
							Data:     testContract.MakeCallData(t, "destroy"),
							Value:    big.NewInt(0),
							GasLimit: 100_000,
						})
						require.NoError(t, err)
						require.False(t, res.Failed())
					})

					// after calling selfdestruct the balance should be returned to the caller and
					// equal initial funded balance of the caller.
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						bal, err := blk.BalanceOf(testAddress)
						require.NoError(t, err)
						require.Equal(t, startBalance, bal)

						bal, err = blk.BalanceOf(contractAddr)
						require.NoError(t, err)
						require.Equal(t, big.NewInt(0), bal)

						nonce, err := blk.NonceOf(contractAddr)
						require.NoError(t, err)
						require.Equal(t, uint64(1), nonce)

						code, err := blk.CodeOf(contractAddr)
						require.NoError(t, err)
						require.True(t, len(code) > 0)
					})
				})
			})
		})
	})
}

func TestTransfers(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {

			testAccount1 := types.NewAddressFromString("test1")
			testAccount2 := types.NewAddressFromString("test2")
			bridgeAccount := types.NewAddressFromString("bridge")

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount1, amount, 0))
					require.NoError(t, err)
				})
			})

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewTransferCall(testAccount1, testAccount2, amountToBeTransfered, 0))
					require.NoError(t, err)
				})
			})

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewReadOnlyBlockView(t, em, func(blk types.ReadOnlyBlockView) {
					bal, err := blk.BalanceOf(testAccount2)
					require.NoError(t, err)
					require.Equal(t, amountToBeTransfered.Uint64(), bal.Uint64())

					bal, err = blk.BalanceOf(testAccount1)
					require.NoError(t, err)
					require.Equal(t, new(big.Int).Sub(amount, amountToBeTransfered).Uint64(), bal.Uint64())
				})
			})
		})
	})
}

func TestStorageNoSideEffect(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
			var err error
			em := emulator.NewEmulator(backend, flowEVMRoot)
			testAccount := types.NewAddressFromString("test")
			bridgeAccount := types.NewAddressFromString("bridge")

			amount := big.NewInt(10)
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err = blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, 0))
				require.NoError(t, err)
			})

			orgSize := backend.TotalStorageSize()
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err = blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, 0))
				require.NoError(t, err)
			})
			require.Equal(t, orgSize, backend.TotalStorageSize())
		})
	})
}

func TestCallingExtraPrecompiles(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
			RunWithNewEmulator(t, backend, flowEVMRoot, func(em *emulator.Emulator) {

				testAccount := types.NewAddressFromString("test")
				bridgeAccount := types.NewAddressFromString("bridge")
				amount := big.NewInt(10_000_000)
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, 0))
					require.NoError(t, err)
				})

				input := []byte{1, 2}
				output := []byte{3, 4}
				addr := testutils.RandomAddress(t)
				pc := &MockedPrecompile{
					AddressFunc: func() types.Address {
						return addr
					},
					RequiredGasFunc: func(input []byte) uint64 {
						return uint64(10)
					},
					RunFunc: func(inp []byte) ([]byte, error) {
						require.Equal(t, input, inp)
						return output, nil
					},
				}

				ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
				ctx.ExtraPrecompiles = []types.Precompile{pc}

				blk, err := em.NewBlockView(ctx)
				require.NoError(t, err)

				res, err := blk.DirectCall(
					types.NewContractCall(
						testAccount,
						types.NewAddress(addr.ToCommon()),
						input,
						1_000_000,
						big.NewInt(0), // this should be zero because the contract doesn't have receiver
						0,
					),
				)
				require.NoError(t, err)
				require.Equal(t, output, res.ReturnedValue)
			})
		})
	})
}

type MockedPrecompile struct {
	AddressFunc     func() types.Address
	RequiredGasFunc func(input []byte) uint64
	RunFunc         func(input []byte) ([]byte, error)
}

func (mp *MockedPrecompile) Address() types.Address {
	if mp.AddressFunc == nil {
		panic("Address not set for the mocked precompile")
	}
	return mp.AddressFunc()
}

func (mp *MockedPrecompile) RequiredGas(input []byte) uint64 {
	if mp.RequiredGasFunc == nil {
		panic("RequiredGas not set for the mocked precompile")
	}
	return mp.RequiredGasFunc(input)
}

func (mp *MockedPrecompile) Run(input []byte) ([]byte, error) {
	if mp.RunFunc == nil {
		panic("Run not set for the mocked precompile")
	}
	return mp.RunFunc(input)
}

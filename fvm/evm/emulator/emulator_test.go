package emulator_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/debug"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"

	_ "github.com/onflow/go-ethereum/eth/tracers/native" // imported so callTracers is registered in init
)

var blockNumber = big.NewInt(10)
var defaultCtx = types.NewDefaultBlockContext(blockNumber.Uint64())

func RunWithNewEmulator(t testing.TB, backend *testutils.TestBackend, rootAddr flow.Address, f func(*emulator.Emulator)) {
	env := emulator.NewEmulator(zerolog.Nop(), backend, rootAddr)
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
			testAccountNonce := uint64(0)

			t.Run("mint tokens to the first account", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						call := types.NewDepositCall(bridgeAccount, testAccount, originalBalance, 0)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						require.Equal(t, call.Hash(), res.TxHash)
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
						require.Equal(t, big.NewInt(0).Uint64(), retBalance.Uint64())
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
						retNonce, err := blk.NonceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, testAccountNonce, retNonce)
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						call := types.NewWithdrawCall(bridgeAccount, testAccount, amount, testAccountNonce)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						require.Equal(t, call.Hash(), res.TxHash)
						testAccountNonce += 1
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
						require.Equal(t, big.NewInt(0).Uint64(), retBalance.Uint64())

						retNonce, err := blk.NonceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, testAccountNonce, retNonce)
					})
				})
			})
			// TODO: add test for deploying to an empty contract (error on deposit call)
			// and a test case for when not enough balance on account for withdraw
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
			testAccountNonce := uint64(0)

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			// fund test account
			RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, testAccount, amount, 0))
					require.NoError(t, err)
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
							testAccountNonce)
						res, err := blk.DirectCall(call)
						require.NoError(t, err)
						require.NotNil(t, res.DeployedContractAddress)
						contractAddr = *res.DeployedContractAddress
						require.Equal(t, call.Hash(), res.TxHash)
						testAccountNonce += 1
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

						retNonce, err := blk.NonceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, testAccountNonce, retNonce)
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
								testAccountNonce,
							),
						)
						require.NoError(t, err)
						require.GreaterOrEqual(t, res.GasConsumed, uint64(40_000))
						testAccountNonce += 1
						require.Empty(t, res.PrecompiledCalls)
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
								testAccountNonce,
							),
						)
						require.NoError(t, err)
						testAccountNonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedData)
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
								testAccountNonce,
							),
						)
						require.NoError(t, err)
						testAccountNonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedData)
						require.Equal(t, blockNumber, ret)

					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "assertError"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								testAccountNonce,
							),
						)
						require.NoError(t, err)
						testAccountNonce += 1
						require.Error(t, res.VMError)
						strings.Contains(string(res.ReturnedData), "Assert Error Message")
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "customError"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								testAccountNonce,
							),
						)
						require.NoError(t, err)
						testAccountNonce += 1
						require.Error(t, res.VMError)
						strings.Contains(string(res.ReturnedData), "Value is too low")
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
							testAccountNonce,
						),
					)
					require.NoError(t, err)
					testAccountNonce += 1

					ret := new(big.Int).SetBytes(res.ReturnedData)
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
					require.NoError(t, res.ValidationError)
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

			t.Run("test batch running transactions", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				account.SetNonce(account.Nonce() + 1)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					ctx.GasFeeCollector = types.NewAddressFromString("coinbase-collector")
					coinbaseOrgBalance := gethCommon.Big1
					// small amount of money to create account
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(bridgeAccount, ctx.GasFeeCollector, coinbaseOrgBalance, 0))
						require.NoError(t, err)
					})

					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)

					const batchSize = 3
					txs := make([]*gethTypes.Transaction, batchSize)
					for i := range txs {
						txs[i] = account.PrepareAndSignTx(
							t,
							testAccount.ToCommon(), // to
							nil,                    // data
							big.NewInt(1000),       // amount
							gethParams.TxGas,       // gas limit
							gethCommon.Big1,        // gas fee

						)
					}

					results, err := blk.BatchRunTransactions(txs)
					require.NoError(t, err)
					for _, res := range results {
						require.NoError(t, res.VMError)
						require.NoError(t, res.ValidationError)
						require.Greater(t, res.GasConsumed, uint64(0))
					}

					// check the balance of coinbase
					RunWithNewReadOnlyBlockView(t, env, func(blk2 types.ReadOnlyBlockView) {
						bal, err := blk2.BalanceOf(ctx.GasFeeCollector)
						require.NoError(t, err)
						expected := gethParams.TxGas*batchSize + gethCommon.Big1.Uint64()
						require.Equal(t, expected, bal.Uint64())

						nonce, err := blk2.NonceOf(fAddr)
						require.NoError(t, err)
						require.Equal(t, batchSize+1, int(nonce))
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
				account.SetNonce(account.Nonce() + 4)

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
					require.NoError(t, res.ValidationError)
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
					res, err := blk.RunTransaction(tx)
					require.NoError(t, err)
					require.Error(t, res.ValidationError)
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
					res, err := blk.RunTransaction(tx)
					require.NoError(t, err)
					require.Error(t, res.ValidationError)
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
						require.NotNil(t, res.DeployedContractAddress)
						require.Equal(t, target, *res.DeployedContractAddress)
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
// is not called in the same transaction as deployment.
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
						require.NotNil(t, res.DeployedContractAddress)
						contractAddr = *res.DeployedContractAddress
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
						require.Equal(t, big.NewInt(0).Uint64(), bal.Uint64())

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
			em := emulator.NewEmulator(zerolog.Nop(), backend, flowEVMRoot)
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
				capturedCall := &types.PrecompiledCalls{
					Address: addr,
					RequiredGasCalls: []types.RequiredGasCall{{
						Input:  input,
						Output: uint64(10),
					}},
					RunCalls: []types.RunCall{{
						Input:    input,
						Output:   output,
						ErrorMsg: "",
					}},
				}
				pc := &MockedPrecompiled{
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
				ctx.ExtraPrecompiledContracts = []types.PrecompiledContract{pc}

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
				require.Equal(t, output, res.ReturnedData)
				require.NotEmpty(t, res.PrecompiledCalls)

				apc, err := types.AggregatedPrecompileCallsFromEncoded(res.PrecompiledCalls)
				require.NoError(t, err)
				require.Len(t, apc, 1)
				require.Equal(t, *capturedCall, apc[0])
			})
		})
	})
}

func TestTransactionTracing(t *testing.T) {

	// runWithDeployedContract is a helper function that will deploy a contract we can use to interact with
	runWithDeployedContract := func(t *testing.T, f func(*testutils.TestContract, *testutils.EOATestAccount, *emulator.Emulator)) {
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *testutils.EOATestAccount) {
					RunWithNewEmulator(t, backend, rootAddr, func(emu *emulator.Emulator) {
						testContract := testutils.GetStorageTestContract(t)
						nonce := testAccount.Nonce()

						// deploy contract
						RunWithNewBlockView(t, emu, func(blk types.BlockView) {
							call := types.NewDeployCall(
								testAccount.Address(),
								testContract.ByteCode,
								math.MaxUint64,
								big.NewInt(0),
								nonce)

							res, err := blk.DirectCall(call)
							require.NoError(t, err)
							require.NotNil(t, res.DeployedContractAddress)
							testAccount.SetNonce(testAccount.Nonce() + 1)
							testContract.DeployedAt = *res.DeployedContractAddress
							f(testContract, testAccount, emu)
						})
					})
				})
			})
		})
	}

	// manually create block with the provided tracer injected
	blockWithTracer := func(t *testing.T, emu *emulator.Emulator) (types.BlockView, *testutils.MockUploader, *debug.CallTracer) {
		uploader := &testutils.MockUploader{}
		tracer, err := debug.NewEVMCallTracer(uploader, zerolog.Nop())
		require.NoError(t, err)

		// manually create block with provided tracer
		ctx := types.NewDefaultBlockContext(1)
		ctx.Tracer = tracer.TxTracer()
		blk, err := emu.NewBlockView(ctx)
		require.NoError(t, err)

		return blk, uploader, tracer
	}

	t.Run("contract interaction using direct call", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, uploader, tracer := blockWithTracer(t, emu)

			var txID gethCommon.Hash
			var trace json.RawMessage

			blockID := flow.Identifier{0x01}
			uploaded := make(chan struct{})

			uploader.UploadFunc = func(id string, message json.RawMessage) error {
				uploaded <- struct{}{}
				require.Equal(t, debug.TraceID(txID, blockID), id)
				require.Equal(t, trace, message)
				require.Greater(t, len(message), 0)
				return nil
			}

			// interact and record trace
			res, err := blk.DirectCall(
				types.NewContractCall(
					testAccount.Address(),
					testContract.DeployedAt,
					testContract.MakeCallData(t, "store", big.NewInt(2)),
					1_000_000,
					big.NewInt(0),
					testAccount.Nonce(),
				),
			)
			require.NoError(t, err)
			txID = res.TxHash
			trace, err = tracer.TxTracer().GetResult()
			require.NoError(t, err)
			tracer.WithBlockID(blockID)

			tracer.Collect(txID)

			testAccount.SetNonce(testAccount.Nonce() + 1)
			require.Eventuallyf(t, func() bool {
				<-uploaded
				return true
			}, time.Second, time.Millisecond*100, "upload did not execute")
		})
	})

	t.Run("contract deploy at using direct call", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, uploader, tracer := blockWithTracer(t, emu)

			var txID gethCommon.Hash
			var trace json.RawMessage

			uploaded := make(chan struct{})
			blockID := flow.Identifier{0x01}

			uploader.UploadFunc = func(id string, message json.RawMessage) error {
				uploaded <- struct{}{}
				require.Equal(t, debug.TraceID(txID, blockID), id)
				require.Equal(t, trace, message)
				require.Greater(t, len(message), 0)
				return nil
			}

			// interact and record trace
			res, err := blk.DirectCall(
				types.NewDeployCallWithTargetAddress(
					testAccount.Address(),
					types.Address{0x01, 0x02},
					testContract.ByteCode,
					1_000_000,
					big.NewInt(0),
					testAccount.Nonce(),
				),
			)
			require.NoError(t, err)
			require.NoError(t, res.ValidationError)
			require.NoError(t, res.VMError)
			txID = res.TxHash
			trace, err = tracer.TxTracer().GetResult()
			require.NoError(t, err)
			tracer.WithBlockID(blockID)

			tracer.Collect(txID)

			testAccount.SetNonce(testAccount.Nonce() + 1)
			require.Eventuallyf(t, func() bool {
				<-uploaded
				return true
			}, time.Second, time.Millisecond*100, "upload did not execute")
		})
	})

	t.Run("contract interaction using run transaction", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, uploader, tracer := blockWithTracer(t, emu)

			var txID gethCommon.Hash
			var trace json.RawMessage

			blockID := flow.Identifier{0x02}
			uploaded := make(chan struct{})

			uploader.UploadFunc = func(id string, message json.RawMessage) error {
				uploaded <- struct{}{}
				require.Equal(t, debug.TraceID(txID, blockID), id)
				require.Equal(t, trace, message)
				require.Greater(t, len(message), 0)
				return nil
			}

			tx := testAccount.PrepareAndSignTx(
				t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "store", big.NewInt(2)),
				big.NewInt(0),
				1_000_000,
				big.NewInt(0),
			)

			// interact and record trace
			res, err := blk.RunTransaction(tx)
			require.NoError(t, err)
			require.NoError(t, res.ValidationError)
			require.NoError(t, res.VMError)
			txID = res.TxHash
			trace, err = tracer.TxTracer().GetResult()
			require.NoError(t, err)
			tracer.WithBlockID(blockID)

			tracer.Collect(txID)
			testAccount.SetNonce(testAccount.Nonce() + 1)
			require.Eventuallyf(t, func() bool {
				<-uploaded
				return true
			}, time.Second, time.Millisecond*100, "upload did not execute")
		})

	})

	t.Run("contract interaction run failed transaction", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, _, tracer := blockWithTracer(t, emu)
			var txID gethCommon.Hash

			tx := testAccount.PrepareAndSignTx(
				t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "store", big.NewInt(2)),
				big.NewInt(0),
				21210,
				big.NewInt(100),
			)

			// interact and record trace
			res, err := blk.RunTransaction(tx)
			require.NoError(t, err)
			require.EqualError(t, res.VMError, "out of gas")

			tracer.Collect(txID)
		})

	})

	t.Run("contract interaction run invalid transaction", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, _, tracer := blockWithTracer(t, emu)
			var txID gethCommon.Hash

			tx := testAccount.PrepareAndSignTx(
				t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "store", big.NewInt(2)),
				big.NewInt(0),
				1_000_000,
				big.NewInt(-1),
			)

			// interact and record trace
			res, err := blk.RunTransaction(tx)
			require.NoError(t, err)
			require.EqualError(t, res.ValidationError, "max fee per gas less than block base fee: address 0x658Bdf435d810C91414eC09147DAA6DB62406379, maxFeePerGas: -1, baseFee: 0")

			tracer.Collect(txID)
		})

	})

	t.Run("contract interaction using run batch transaction", func(t *testing.T) {
		runWithDeployedContract(t, func(testContract *testutils.TestContract, testAccount *testutils.EOATestAccount, emu *emulator.Emulator) {
			blk, uploader, tracer := blockWithTracer(t, emu)

			var txID gethCommon.Hash
			var trace json.RawMessage

			blockID := flow.Identifier{0x02}
			uploaded := make(chan struct{})

			uploader.UploadFunc = func(id string, message json.RawMessage) error {
				uploaded <- struct{}{}
				require.Equal(t, debug.TraceID(txID, blockID), id)
				require.Equal(t, trace, message)
				require.Greater(t, len(message), 0)
				return nil
			}

			tx := testAccount.PrepareAndSignTx(
				t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "store", big.NewInt(2)),
				big.NewInt(0),
				1_000_000,
				big.NewInt(0),
			)

			// interact and record trace
			results, err := blk.BatchRunTransactions([]*gethTypes.Transaction{tx})
			require.NoError(t, err)
			require.Len(t, results, 1)
			txID = results[0].TxHash
			trace, err = tracer.TxTracer().GetResult()
			require.NoError(t, err)
			tracer.WithBlockID(blockID)

			tracer.Collect(txID)

			require.Eventuallyf(t, func() bool {
				<-uploaded
				return true
			}, time.Second, time.Millisecond*100, "upload did not execute")
		})

	})

}

func TestTxIndex(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
				expectedTxIndex := uint16(1)
				ctx.TxCountSoFar = 1
				testAccount1 := types.NewAddressFromString("test")
				testAccount2 := types.NewAddressFromString("test")

				blk, err := em.NewBlockView(ctx)
				require.NoError(t, err)

				res, err := blk.DirectCall(
					types.NewContractCall(
						testAccount1,
						testAccount2,
						nil,
						1_000_000,
						big.NewInt(0),
						0,
					),
				)

				require.NoError(t, err)
				require.Equal(t, expectedTxIndex, res.Index)
				expectedTxIndex += 1
				ctx.TxCountSoFar = 2

				// create a test eoa account
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()

				blk, err = em.NewBlockView(ctx)
				require.NoError(t, err)
				res, err = blk.DirectCall(
					types.NewDepositCall(
						types.EmptyAddress,
						fAddr,
						types.OneFlow,
						account.Nonce(),
					))
				require.NoError(t, err)
				require.NoError(t, res.VMError)
				require.NoError(t, res.ValidationError)
				require.Equal(t, expectedTxIndex, res.Index)
				expectedTxIndex += 1
				ctx.TxCountSoFar = 3

				blk, err = em.NewBlockView(ctx)
				require.NoError(t, err)

				tx := account.PrepareAndSignTx(
					t,
					testAccount1.ToCommon(), // to
					nil,                     // data
					big.NewInt(1000),        // amount
					gethParams.TxGas,        // gas limit
					big.NewInt(0),
				)

				res, err = blk.RunTransaction(tx)
				require.NoError(t, err)
				require.NoError(t, res.VMError)
				require.NoError(t, res.ValidationError)
				require.Equal(t, expectedTxIndex, res.Index)
				expectedTxIndex += 1
				ctx.TxCountSoFar = 4

				blk, err = em.NewBlockView(ctx)
				require.NoError(t, err)

				const batchSize = 3
				txs := make([]*gethTypes.Transaction, batchSize)
				for i := range txs {
					txs[i] = account.PrepareAndSignTx(
						t,
						testAccount1.ToCommon(), // to
						nil,                     // data
						big.NewInt(1000),        // amount
						gethParams.TxGas,        // gas limit
						big.NewInt(0),
					)
				}
				results, err := blk.BatchRunTransactions(txs)
				require.NoError(t, err)
				for i, res := range results {
					require.NoError(t, res.VMError)
					require.NoError(t, res.ValidationError)
					require.Equal(t, expectedTxIndex+uint16(i), res.Index)
				}
			})
		})
	})
}

type MockedPrecompiled struct {
	AddressFunc     func() types.Address
	RequiredGasFunc func(input []byte) uint64
	RunFunc         func(input []byte) ([]byte, error)
}

var _ types.PrecompiledContract = &MockedPrecompiled{}

func (mp *MockedPrecompiled) Address() types.Address {
	if mp.AddressFunc == nil {
		panic("Address not set for the mocked precompiled contract")
	}
	return mp.AddressFunc()
}

func (mp *MockedPrecompiled) RequiredGas(input []byte) uint64 {
	if mp.RequiredGasFunc == nil {
		panic("RequiredGas not set for the mocked precompiled contract")
	}
	return mp.RequiredGasFunc(input)
}

func (mp *MockedPrecompiled) Run(input []byte) ([]byte, error) {
	if mp.RunFunc == nil {
		panic("Run not set for the mocked precompiled contract")
	}
	return mp.RunFunc(input)
}

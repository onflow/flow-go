package emulator_test

import (
	"math"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var blockNumber = big.NewInt(10)
var defaultCtx = types.NewDefaultBlockContext(blockNumber.Uint64())

func RunWithTestDB(t testing.TB, f func(types.Database)) {
	testutils.RunWithTestBackend(t, func(backend types.Backend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
			db, err := database.NewDatabase(backend, flowEVMRoot)
			require.NoError(t, err)
			f(db)
		})
	})
}

func RunWithNewEmulator(t testing.TB, db types.Database, f func(*emulator.Emulator)) {
	env := emulator.NewEmulator(db)
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
	RunWithTestDB(t, func(db types.Database) {
		originalBalance := big.NewInt(10000)
		testAccount := types.NewAddressFromString("test")

		t.Run("mint tokens to the first account", func(t *testing.T) {
			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					amount := big.NewInt(10000)
					res, err := blk.MintTo(testAccount, amount)
					require.NoError(t, err)
					require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
				})
			})
		})
		t.Run("mint tokens withdraw", func(t *testing.T) {
			amount := big.NewInt(1000)
			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
					retBalance, err := blk.BalanceOf(testAccount)
					require.NoError(t, err)
					require.Equal(t, originalBalance, retBalance)
				})
			})
			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					res, err := blk.WithdrawFrom(testAccount, amount)
					require.NoError(t, err)
					require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
				})
			})
			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
					retBalance, err := blk.BalanceOf(testAccount)
					require.NoError(t, err)
					require.Equal(t, amount.Sub(originalBalance, amount), retBalance)
				})
			})
		})
	})

}

func TestContractInteraction(t *testing.T) {
	RunWithTestDB(t, func(db types.Database) {

		testContract := testutils.GetTestContract(t)

		testAccount := types.NewAddressFromString("test")
		amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
		amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

		// fund test account
		RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
			RunWithNewBlockView(t, env, func(blk types.BlockView) {
				_, err := blk.MintTo(testAccount, amount)
				require.NoError(t, err)
			})
		})

		var contractAddr types.Address

		t.Run("deploy contract", func(t *testing.T) {
			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					res, err := blk.Deploy(testAccount, testContract.ByteCode, math.MaxUint64, amountToBeTransfered)
					require.NoError(t, err)
					contractAddr = res.DeployedContractAddress
				})
				RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
					require.NotNil(t, contractAddr)
					retCode, err := blk.CodeOf(contractAddr)
					require.NoError(t, err)
					require.True(t, len(retCode) > 0)

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

			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					res, err := blk.Call(
						testAccount,
						contractAddr,
						testContract.MakeStoreCallData(t, num),
						1_000_000,
						// this should be zero because the contract doesn't have receiver
						big.NewInt(0),
					)
					require.NoError(t, err)
					require.GreaterOrEqual(t, res.GasConsumed, uint64(40_000))
				})
			})

			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					res, err := blk.Call(
						testAccount,
						contractAddr,
						testContract.MakeRetrieveCallData(t),
						1_000_000,
						// this should be zero because the contract doesn't have receiver
						big.NewInt(0),
					)
					require.NoError(t, err)

					ret := new(big.Int).SetBytes(res.ReturnedValue)
					require.Equal(t, num, ret)
					require.GreaterOrEqual(t, res.GasConsumed, uint64(23_000))
				})
			})

			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					res, err := blk.Call(
						testAccount,
						contractAddr,
						testContract.MakeBlockNumberCallData(t),
						1_000_000,
						// this should be zero because the contract doesn't have receiver
						big.NewInt(0),
					)
					require.NoError(t, err)

					ret := new(big.Int).SetBytes(res.ReturnedValue)
					require.Equal(t, blockNumber, ret)
				})
			})

		})

		t.Run("test sending transactions (happy case)", func(t *testing.T) {
			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := gethCrypto.HexToECDSA(keyHex)
			address := gethCrypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379
			fAddr := types.NewAddress(address)

			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.MintTo(fAddr, amount)
					require.NoError(t, err)
				})
			})

			RunWithNewEmulator(t, db, func(env *emulator.Emulator) {
				ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
				ctx.GasFeeCollector = types.NewAddressFromString("coinbase")
				coinbaseOrgBalance := gethCommon.Big1
				// small amount of money to create account
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.MintTo(ctx.GasFeeCollector, coinbaseOrgBalance)
					require.NoError(t, err)
				})

				blk, err := env.NewBlockView(ctx)
				require.NoError(t, err)
				signer := emulator.GetDefaultSigner()
				tx, _ := gethTypes.SignTx(
					gethTypes.NewTransaction(
						0,                      // nonce
						testAccount.ToCommon(), // to
						big.NewInt(1000),       // amount
						gethParams.TxGas,       // gas limit
						gethCommon.Big1,        // gas price
						nil,                    // data
					), signer, key)
				res, err := blk.RunTransaction(tx)
				require.NoError(t, err)
				require.Empty(t, res.ErrorMessage)

				// TODO: check why nonce of zero is valid for tx but on check is 4 ?

				// check the balance of coinbase
				RunWithNewReadOnlyBlockView(t, env, func(blk2 types.ReadOnlyBlockView) {
					bal, err := blk2.BalanceOf(ctx.GasFeeCollector)
					require.NoError(t, err)
					require.Greater(t, bal.Uint64(), coinbaseOrgBalance.Uint64())

					nonce, err := blk2.NonceOf(testAccount)
					require.NoError(t, err)
					require.Equal(t, 1, int(nonce))
				})
			})
		})
	})
}

func TestTransfers(t *testing.T) {
	RunWithTestDB(t, func(db types.Database) {
		testAccount1 := types.NewAddressFromString("test1")
		testAccount2 := types.NewAddressFromString("test2")

		amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
		amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

		RunWithNewEmulator(t, db, func(em *emulator.Emulator) {
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err := blk.MintTo(testAccount1, amount)
				require.NoError(t, err)
			})
		})

		RunWithNewEmulator(t, db, func(em *emulator.Emulator) {
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err := blk.Transfer(testAccount1, testAccount2, amountToBeTransfered)
				require.NoError(t, err)
			})
		})

		RunWithNewEmulator(t, db, func(em *emulator.Emulator) {
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
}

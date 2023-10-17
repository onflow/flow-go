package evm_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/flow-go/fvm/flex/evm"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/require"
)

var blockNumber = big.NewInt(10)

func RunWithTestDB(t testing.TB, f func(*storage.Database)) {
	testutils.RunWithTestBackend(t, func(backend models.Backend) {
		testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
			db, err := storage.NewDatabase(backend, testutils.TestFlexRootAddress)
			require.NoError(t, err)
			f(db)
		})
	})
}

func RunWithNewEmulator(t testing.TB, db *storage.Database, f func(*evm.Emulator)) {
	cfg := evm.NewConfig(evm.WithBlockNumber(blockNumber))
	env := evm.NewEmulator(cfg, db)
	f(env)
}

func TestNativeTokenBridging(t *testing.T) {
	RunWithTestDB(t, func(db *storage.Database) {
		originalBalance := big.NewInt(10000)
		testAccount := models.NewFlexAddressFromString("test")

		t.Run("mint tokens to the first account", func(t *testing.T) {
			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				amount := big.NewInt(10000)
				res, err := env.MintTo(testAccount, amount)
				require.NoError(t, err)
				require.Equal(t, evm.TransferGasUsage, res.GasConsumed)
			})
		})
		t.Run("mint tokens withdraw", func(t *testing.T) {
			amount := big.NewInt(1000)
			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				retBalance, err := env.BalanceOf(testAccount)
				require.NoError(t, err)
				require.Equal(t, originalBalance, retBalance)
			})
			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				res, err := env.WithdrawFrom(testAccount, amount)
				require.NoError(t, err)
				require.Equal(t, evm.TransferGasUsage, res.GasConsumed)
			})
			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				retBalance, err := env.BalanceOf(testAccount)
				require.NoError(t, err)
				require.Equal(t, amount.Sub(originalBalance, amount), retBalance)
			})
		})
	})

}

func TestContractInteraction(t *testing.T) {
	RunWithTestDB(t, func(db *storage.Database) {

		testContract := testutils.GetTestContract(t)

		testAccount := models.NewFlexAddressFromString("test")
		amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(params.Ether))
		amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Ether))

		// fund test account
		RunWithNewEmulator(t, db, func(env *evm.Emulator) {
			_, err := env.MintTo(testAccount, amount)
			require.NoError(t, err)
		})

		var contractAddr models.FlexAddress

		t.Run("deploy contract", func(t *testing.T) {
			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				res, err := env.Deploy(testAccount, testContract.ByteCode, math.MaxUint64, amountToBeTransfered)
				require.NoError(t, err)

				contractAddr = res.DeployedContractAddress
				require.NotNil(t, contractAddr)
				retCode, err := env.CodeOf(contractAddr)
				require.NoError(t, err)
				require.True(t, len(retCode) > 0)

				retBalance, err := env.BalanceOf(contractAddr)
				require.NoError(t, err)
				require.Equal(t, amountToBeTransfered, retBalance)

				retBalance, err = env.BalanceOf(testAccount)
				require.NoError(t, err)
				require.Equal(t, amount.Sub(amount, amountToBeTransfered), retBalance)

			})
		})

		t.Run("call contract", func(t *testing.T) {
			num := big.NewInt(10)

			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				res, err := env.Call(testAccount,
					contractAddr,
					testContract.MakeStoreCallData(t, num),
					1_000_000,
					// this should be zero because the contract doesn't have receiver
					big.NewInt(0),
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t, res.GasConsumed, uint64(40_000))
			})

			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				res, err := env.Call(testAccount,
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

			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				res, err := env.Call(testAccount,
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

		t.Run("test sending transactions", func(t *testing.T) {
			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := crypto.HexToECDSA(keyHex)
			address := crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379
			fAddr := models.NewFlexAddress(address)

			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				_, err := env.MintTo(fAddr, amount)
				require.NoError(t, err)
			})

			RunWithNewEmulator(t, db, func(env *evm.Emulator) {
				signer := evm.GetDefaultSigner()
				tx, _ := types.SignTx(types.NewTransaction(0, testAccount.ToCommon(), big.NewInt(1000), params.TxGas, new(big.Int).Add(big.NewInt(0), common.Big1), nil), signer, key)
				coinbase := models.NewFlexAddressFromString("coinbase")
				_, err := env.RunTransaction(tx, coinbase)
				// TODO check the balance of coinbase
				require.NoError(t, err)
			})
		})
	})
}

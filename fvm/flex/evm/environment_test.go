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

func RunWithTestDB(t testing.TB, f func(*storage.Database)) {
	testutils.RunWithTestBackend(t, func(backend models.Backend) {
		testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
			db, err := storage.NewDatabase(backend, testutils.TestFlexRootAddress)
			require.NoError(t, err)
			f(db)
		})
	})
}

func RunWithNewEnv(t testing.TB, db *storage.Database, f func(*evm.Environment)) {
	coinbase := common.BytesToAddress([]byte("coinbase"))
	config := evm.NewFlexConfig((evm.WithCoinbase(coinbase)),
		evm.WithBlockNumber(evm.BlockNumberForEVMRules))
	env, err := evm.NewEnvironment(config, db)
	require.NoError(t, err)
	f(env)
}

func TestNativeTokenBridging(t *testing.T) {
	RunWithTestDB(t, func(db *storage.Database) {
		originalBalance := big.NewInt(10000)
		testAccount := models.NewFlexAddressFromString("test")

		t.Run("mint tokens to the first account", func(t *testing.T) {
			RunWithNewEnv(t, db, func(env *evm.Environment) {
				amount := big.NewInt(10000)
				res, err := env.MintTo(testAccount, amount)
				require.NoError(t, err)
				require.Equal(t, evm.TransferGasUsage, res.GasConsumed)
			})
		})
		t.Run("mint tokens withdraw", func(t *testing.T) {
			amount := big.NewInt(1000)
			RunWithNewEnv(t, db, func(env *evm.Environment) {
				require.Equal(t, originalBalance, env.State.GetBalance(testAccount.ToCommon()))
			})
			RunWithNewEnv(t, db, func(env *evm.Environment) {
				res, err := env.WithdrawFrom(testAccount, amount)
				require.NoError(t, err)
				require.Equal(t, evm.TransferGasUsage, res.GasConsumed)
			})
			RunWithNewEnv(t, db, func(env *evm.Environment) {
				require.Equal(t, amount.Sub(originalBalance, amount), env.State.GetBalance(testAccount.ToCommon()))
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
		RunWithNewEnv(t, db, func(env *evm.Environment) {
			_, err := env.MintTo(testAccount, amount)
			require.NoError(t, err)
		})

		var contractAddr models.FlexAddress

		t.Run("deploy contract", func(t *testing.T) {
			RunWithNewEnv(t, db, func(env *evm.Environment) {
				res, err := env.Deploy(testAccount, testContract.ByteCode, math.MaxUint64, amountToBeTransfered)
				require.NoError(t, err)

				contractAddr = res.DeployedContractAddress
				require.NotNil(t, contractAddr)
				require.True(t, len(env.State.GetCode(contractAddr.ToCommon())) > 0)
				require.Equal(t, amountToBeTransfered, env.State.GetBalance(contractAddr.ToCommon()))
				require.Equal(t, amount.Sub(amount, amountToBeTransfered), env.State.GetBalance(testAccount.ToCommon()))
			})
		})

		t.Run("call contract", func(t *testing.T) {
			num := big.NewInt(10)

			RunWithNewEnv(t, db, func(env *evm.Environment) {
				res, err := env.Call(testAccount,
					contractAddr,
					testContract.MakeStoreCallData(t, num),
					1_000_000,
					// this should be zero because the contract doesn't have receiver
					big.NewInt(0),
				)
				require.NoError(t, err)
				require.Equal(t, uint64(1000), res.GasConsumed)
			})

			RunWithNewEnv(t, db, func(env *evm.Environment) {
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
				require.Equal(t, uint64(1000), res.GasConsumed)
			})

		})

		t.Run("test sending transactions", func(t *testing.T) {

			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := crypto.HexToECDSA(keyHex)
			address := crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379
			fAddr := models.NewFlexAddress(address)

			RunWithNewEnv(t, db, func(env *evm.Environment) {
				_, err := env.MintTo(fAddr, amount)
				require.NoError(t, err)
			})

			RunWithNewEnv(t, db, func(env *evm.Environment) {
				signer := types.MakeSigner(env.Config.ChainConfig, evm.BlockNumberForEVMRules, env.Config.BlockContext.Time)
				tx, _ := types.SignTx(types.NewTransaction(0, testAccount.ToCommon(), big.NewInt(1000), params.TxGas, new(big.Int).Add(big.NewInt(0), common.Big1), nil), signer, key)

				_, err := env.RunTransaction(tx)
				require.NoError(t, err)
			})
		})
	})
}

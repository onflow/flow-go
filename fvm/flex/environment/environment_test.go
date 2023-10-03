package env_test

import (
	"math"

	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	fenv "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/require"
)

func RunWithTestDB(t testing.TB, f func(*storage.Database)) {
	testutils.RunWithTestBackend(t, func(backend models.Backend) {
		testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
			db := storage.NewDatabase(backend, testutils.TestFlexRootAddress)
			f(db)
		})
	})
}

func RunWithNewEnv(t testing.TB, db *storage.Database, f func(*fenv.Environment)) {
	coinbase := common.BytesToAddress([]byte("coinbase"))
	config := fenv.NewFlexConfig((fenv.WithCoinbase(coinbase)),
		fenv.WithBlockNumber(fenv.BlockNumberForEVMRules))
	env, err := fenv.NewEnvironment(config, db)
	require.NoError(t, err)
	f(env)
}

func TestNativeTokenBridging(t *testing.T) {
	RunWithTestDB(t, func(db *storage.Database) {
		originalBalance := big.NewInt(10000)
		testAccount := common.BytesToAddress([]byte("test"))

		t.Run("mint tokens to the first account", func(t *testing.T) {
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				amount := big.NewInt(10000)
				testAccount := common.BytesToAddress([]byte("test"))
				err := env.MintTo(amount, testAccount)
				require.NoError(t, err)
			})
		})
		t.Run("mint tokens withdraw", func(t *testing.T) {
			amount := big.NewInt(1000)
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				require.Equal(t, originalBalance, env.State.GetBalance(testAccount))
			})
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err := env.WithdrawFrom(amount, testAccount)
				require.NoError(t, err)
			})
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				require.Equal(t, amount.Sub(originalBalance, amount), env.State.GetBalance(testAccount))
			})
		})
	})

}

func TestContractInteraction(t *testing.T) {
	RunWithTestDB(t, func(db *storage.Database) {

		testContract := testutils.GetTestContract(t)

		testAccount := common.BytesToAddress([]byte("test"))
		amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(params.Ether))
		amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Ether))

		// fund test account
		RunWithNewEnv(t, db, func(env *fenv.Environment) {
			err := env.MintTo(amount, testAccount)
			require.NoError(t, err)
		})

		var contractAddr common.Address

		t.Run("deploy contract", func(t *testing.T) {
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err := env.Deploy(testAccount, testContract.ByteCode, math.MaxUint64, amountToBeTransfered)
				require.NoError(t, err)

				contractAddr = env.Result.DeployedContractAddress
				require.NotNil(t, contractAddr)

				require.True(t, len(env.State.GetCode(contractAddr)) > 0)
				require.Equal(t, amountToBeTransfered, env.State.GetBalance(contractAddr))
				require.Equal(t, amount.Sub(amount, amountToBeTransfered), env.State.GetBalance(testAccount))
			})
		})

		t.Run("call contract", func(t *testing.T) {
			num := big.NewInt(10)

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err := env.Call(testAccount,
					contractAddr,
					testContract.MakeStoreCallData(t, num),
					1_000_000,
					// this should be zero because the contract doesn't have receiver
					big.NewInt(0),
				)
				require.NoError(t, err)
				require.False(t, env.Result.Failed)
			})

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err := env.Call(testAccount,
					contractAddr,
					testContract.MakeRetrieveCallData(t),
					1_000_000,
					// this should be zero because the contract doesn't have receiver
					big.NewInt(0),
				)
				require.NoError(t, err)
				require.False(t, env.Result.Failed)

				ret := env.Result.RetValue
				retNum := new(big.Int).SetBytes(ret)
				require.Equal(t, num, retNum)
			})

		})

		t.Run("test sending transactions", func(t *testing.T) {

			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := crypto.HexToECDSA(keyHex)
			address := crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err := env.MintTo(amount, address)
				require.NoError(t, err)
			})

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				signer := types.MakeSigner(env.Config.ChainConfig, fenv.BlockNumberForEVMRules, env.Config.BlockContext.Time)
				tx, _ := types.SignTx(types.NewTransaction(0, testAccount, big.NewInt(1000), params.TxGas, new(big.Int).Add(big.NewInt(0), common.Big1), nil), signer, key)

				err := env.RunTransaction(tx, math.MaxUint64)
				require.NoError(t, err)
				require.False(t, env.Result.Failed)
			})
		})
	})
}

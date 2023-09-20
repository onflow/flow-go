package env_test

import (
	"bytes"
	"encoding/hex"
	"io"
	"math"

	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/flow-go/fvm/environment"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	fenv "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/stretchr/testify/require"
)

func RunWithTempDB(t testing.TB, f func(*storage.Database)) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	led := storage.NewLedger(accounts)
	db := storage.NewDatabase(led)
	f(db)
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
	RunWithTempDB(t, func(db *storage.Database) {
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
	RunWithTempDB(t, func(db *storage.Database) {
		////  contract code:
		// contract Storage {
		//     uint256 number;
		//     constructor() payable {
		//     }
		//     function store(uint256 num) public {
		//         number = num;
		//     }
		//     function retrieve() public view returns (uint256){
		//         return number;
		//     }
		// }

		definition := `
			[
				{
					"inputs": [],
					"stateMutability": "payable",
					"type": "constructor"
				},
				{
					"inputs": [],
					"name": "retrieve",
					"outputs": [
						{
							"internalType": "uint256",
							"name": "",
							"type": "uint256"
						}
					],
					"stateMutability": "view",
					"type": "function"
				},
				{
					"inputs": [
						{
							"internalType": "uint256",
							"name": "num",
							"type": "uint256"
						}
					],
					"name": "store",
					"outputs": [],
					"stateMutability": "nonpayable",
					"type": "function"
				}
			]
			`

		byteCodes, err := hex.DecodeString("6080604052610150806100136000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80632e64cec11461003b5780636057361d14610059575b600080fd5b610043610075565b60405161005091906100a1565b60405180910390f35b610073600480360381019061006e91906100ed565b61007e565b005b60008054905090565b8060008190555050565b6000819050919050565b61009b81610088565b82525050565b60006020820190506100b66000830184610092565b92915050565b600080fd5b6100ca81610088565b81146100d557600080fd5b50565b6000813590506100e7816100c1565b92915050565b600060208284031215610103576101026100bc565b5b6000610111848285016100d8565b9150509291505056fea2646970667358221220029e22143e146846aff5dd684a6d627d0bec77c78e5b7ce77674d91c25d7e22264736f6c63430008120033")
		require.NoError(t, err)

		testAccount := common.BytesToAddress([]byte("test"))
		amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(params.Ether))
		amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Ether))

		// fund test account
		RunWithNewEnv(t, db, func(env *fenv.Environment) {
			err = env.MintTo(amount, testAccount)
			require.NoError(t, err)
		})

		var contractAddr common.Address

		t.Run("deploy contract", func(t *testing.T) {
			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				err = env.Deploy(testAccount, byteCodes, math.MaxUint64, amountToBeTransfered)
				require.NoError(t, err)

				contractAddr = env.Result.DeployedContractAddress
				require.NotNil(t, contractAddr)

				require.True(t, len(env.State.GetCode(contractAddr)) > 0)
				require.Equal(t, amountToBeTransfered, env.State.GetBalance(contractAddr))
				require.Equal(t, amount.Sub(amount, amountToBeTransfered), env.State.GetBalance(testAccount))
			})
		})

		t.Run("call contract", func(t *testing.T) {
			abi, err := abi.JSON(strings.NewReader(definition))
			require.NoError(t, err)

			num := big.NewInt(10)

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				store, err := abi.Pack("store", num)
				require.NoError(t, err)

				err = env.Call(&testAccount,
					&contractAddr,
					store,
					1_000_000,
					// this should be zero because the contract doesn't have receiver
					big.NewInt(0),
				)
				require.NoError(t, err)
				require.False(t, env.Result.Failed)
			})

			RunWithNewEnv(t, db, func(env *fenv.Environment) {
				retrieve, err := abi.Pack("retrieve")
				require.NoError(t, err)

				err = env.Call(&testAccount,
					&contractAddr,
					retrieve,
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

			RunWithNewEnv(t, db, func(env *env.Environment) {
				err = env.MintTo(amount, address)
				require.NoError(t, err)
			})

			RunWithNewEnv(t, db, func(env *env.Environment) {
				signer := types.MakeSigner(env.Config.ChainConfig, fenv.BlockNumberForEVMRules, env.Config.BlockContext.Time)
				tx, _ := types.SignTx(types.NewTransaction(0, testAccount, big.NewInt(1000), params.TxGas, new(big.Int).Add(big.NewInt(0), common.Big1), nil), signer, key)

				var b bytes.Buffer
				writer := io.Writer(&b)
				tx.EncodeRLP(writer)

				err = env.RunTransaction(b.Bytes())
				require.NoError(t, err)
				require.False(t, env.Result.Failed)
			})
		})
	})
}

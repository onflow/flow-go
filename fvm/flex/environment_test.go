package flex_test

import (
	"encoding/hex"
	"fmt"

	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/stretchr/testify/require"
)

func RunWithTempDB(t testing.TB, f func(*storage.Database)) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	db := storage.NewDatabase(accounts)
	f(db)
}

func TestNativeTokenBridging(t *testing.T) {
	RunWithTempDB(t, func(db *storage.Database) {
		coinbase := common.BytesToAddress([]byte("coinbase"))
		config := flex.NewFlexConfig((flex.WithCoinbase(coinbase)),
			flex.WithBlockNumber(big.NewInt(1)))

		amount := big.NewInt(10000)
		testAccount := common.BytesToAddress([]byte("test"))

		t.Run("mint tokens to the first account", func(t *testing.T) {
			env, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)

			err = env.MintTo(amount, testAccount)
			require.NoError(t, err)
		})
		t.Run("mint tokens withdraw", func(t *testing.T) {
			env2, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)
			require.Equal(t, amount, env2.State.GetBalance(testAccount))

			amount2 := big.NewInt(3000)
			err = env2.WithdrawFrom(amount2, testAccount)
			require.NoError(t, err)

			env3, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)
			require.Equal(t, amount.Sub(amount, amount2), env3.State.GetBalance(testAccount))
		})
	})

}

func TestContractInteraction(t *testing.T) {
	RunWithTempDB(t, func(db *storage.Database) {
		coinbase := common.BytesToAddress([]byte("coinbase"))
		config := flex.NewFlexConfig((flex.WithCoinbase(coinbase)),
			flex.WithBlockNumber(big.NewInt(1)))

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

		byteCodes, err := hex.DecodeString("6080604052610150806100136000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80632e64cec11461003b5780636057361d14610059575b600080fd5b610043610075565b60405161005091906100a1565b60405180910390f35b610073600480360381019061006e91906100ed565b61007e565b005b60008054905090565b8060008190555050565b6000819050919050565b61009b81610088565b82525050565b60006020820190506100b66000830184610092565b92915050565b600080fd5b6100ca81610088565b81146100d557600080fd5b50565b6000813590506100e7816100c1565b92915050565b600060208284031215610103576101026100bc565b5b6000610111848285016100d8565b9150509291505056fea264697066735822122000554714e02795fc835a96df86a765b93852d8f1401a1bba8b9ea8049a31746764736f6c63430008120033")
		require.NoError(t, err)

		// setup and fund the test account
		testAccount := common.BytesToAddress([]byte("test"))
		amount := big.NewInt(10000)
		amountToBeTransfered := big.NewInt(4000)

		// fund test account
		env, err := flex.NewEnvironment(config, db)
		require.NoError(t, err)

		err = env.MintTo(amount, testAccount)
		require.NoError(t, err)

		var contractAddr common.Address

		t.Run("deploy contract", func(t *testing.T) {
			env, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)

			err = env.Deploy(testAccount, byteCodes, amountToBeTransfered)
			require.NoError(t, err)

			contractAddr = env.Result.DeployedContractAddress
			require.NotNil(t, contractAddr)

			env.State.GetBalance(testAccount)
			require.True(t, len(env.State.GetCode(contractAddr)) > 0)
			require.Equal(t, amountToBeTransfered, env.State.GetBalance(contractAddr))
			require.Equal(t, amount.Sub(amount, amountToBeTransfered), env.State.GetBalance(testAccount))
		})

		t.Run("call contract", func(t *testing.T) {
			env, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)

			abi, err := abi.JSON(strings.NewReader(definition))
			require.NoError(t, err)

			store, err := abi.Pack("store", big.NewInt(256))
			require.NoError(t, err)

			err = env.Call(testAccount, contractAddr, store, big.NewInt(0))
			require.NoError(t, err)

			env2, err := flex.NewEnvironment(config, db)
			require.NoError(t, err)

			retrieve, err := abi.Pack("retrieve")
			require.NoError(t, err)

			err = env2.Call(testAccount, contractAddr, retrieve, big.NewInt(0))
			require.NoError(t, err)

			ret := env2.Result.RetValue
			num := new(big.Int).SetBytes(ret)
			fmt.Println(num)

			require.Equal(t, "XXXX", ret)
		})

	})

}

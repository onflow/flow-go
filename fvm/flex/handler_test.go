package flex_test

import (
	"bytes"
	"io"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/flow-go/fvm/flex"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/utils"
	"github.com/stretchr/testify/require"
)

func TestFlexContractHandler(t *testing.T) {
	t.Parallel()
	t.Run("test last executed block call", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			handler := flex.NewFlexContractHandler(backend)
			// test call last executed block without initialization
			b := handler.LastExecutedBlock()
			require.NotNil(t, b)
			require.Equal(t, models.GenesisFlexBlock, b)

			// some opeartion
			foa := handler.NewFlowOwnedAccount()
			require.NotNil(t, foa)

			// check if uuid index and block height has been incremented
			b = handler.LastExecutedBlock()
			require.Equal(t, uint64(1), b.Height)
			require.Equal(t, uint64(2), b.UUIDIndex)
		})
	})

	t.Run("test foa creation", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			handler := flex.NewFlexContractHandler(backend)
			foa := handler.NewFlowOwnedAccount()
			require.NotNil(t, foa)

			expectedAddress := models.NewFlexAddress(common.HexToAddress("0x00000000000000000001"))
			require.Equal(t, expectedAddress, foa.Address())
		})
	})

	t.Run("test running transaction", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			handler := flex.NewFlexContractHandler(backend)
			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := crypto.HexToECDSA(keyHex)
			address := crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379

			// deposit 1 Flow to the foa account
			foa := handler.NewFlowOwnedAccount()
			orgBalance, err := models.NewBalanceFromAttoFlow(big.NewInt(1e18))
			require.NoError(t, err)
			vault := models.NewFlowTokenVault(orgBalance)
			foa.Deposit(vault)

			// transfer 0.1 flow to the non-foa address
			deduction, err := models.NewBalanceFromAttoFlow(big.NewInt(1e17))
			require.NoError(t, err)
			foa.Call(models.NewFlexAddress(address), nil, 400000, deduction)
			require.Equal(t, orgBalance.Sub(deduction), foa.Balance())

			// transfer 0.01 flow back to the foa through
			addition, err := models.NewBalanceFromAttoFlow(big.NewInt(1e16))
			require.NoError(t, err)
			flexConf := env.NewFlexConfig()
			signer := types.MakeSigner(flexConf.ChainConfig, env.BlockNumberForEVMRules, flexConf.BlockContext.Time)
			tx, _ := types.SignTx(types.NewTransaction(
				0,
				foa.Address().ToCommon(),
				addition.ToAttoFlow(),
				params.TxGas*10,
				big.NewInt(1e8), // high gas fee to test coinbase collection
				nil),
				signer, key)

			var b bytes.Buffer
			writer := io.Writer(&b)
			tx.EncodeRLP(writer)

			// setup coinbase
			foa2 := handler.NewFlowOwnedAccount()
			require.Equal(t, models.Balance(0), foa2.Balance())

			success := handler.Run(b.Bytes(), foa2.Address())
			require.True(t, success)

			require.Equal(t, orgBalance.Sub(deduction).Add(addition), foa.Balance())

			// fees has been collected to the coinbase
			require.NotEqual(t, models.Balance(0), foa2.Balance())

		})
	})
}

func TestFOA(t *testing.T) {
	t.Run("test deposit/withdraw", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			handler := flex.NewFlexContractHandler(backend)
			foa := handler.NewFlowOwnedAccount()
			require.NotNil(t, foa)

			zeroBalance, err := models.NewBalanceFromAttoFlow(big.NewInt(0))
			require.NoError(t, err)
			require.Equal(t, zeroBalance, foa.Balance())

			balance, err := models.NewBalanceFromAttoFlow(big.NewInt(100))
			require.NoError(t, err)
			vault := models.NewFlowTokenVault(balance)

			foa.Deposit(vault)

			require.NoError(t, err)
			require.Equal(t, balance, foa.Balance())

			v := foa.Withdraw(balance)
			require.NoError(t, err)
			require.Equal(t, balance, v.Balance())

			require.NoError(t, err)
			require.Equal(t, zeroBalance, foa.Balance())
		})
	})

	t.Run("test deploy/call", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			handler := flex.NewFlexContractHandler(backend)
			foa := handler.NewFlowOwnedAccount()
			require.NotNil(t, foa)

			// deposit 100 flow
			orgBalance, err := models.NewBalanceFromAttoFlow(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(100)))
			require.NoError(t, err)
			vault := models.NewFlowTokenVault(orgBalance)
			foa.Deposit(vault)

			testContract := utils.GetTestContract(t)
			addr := foa.Deploy(testContract.ByteCode, math.MaxUint64, models.Balance(0))
			require.NotNil(t, addr)

			num := big.NewInt(22)

			_ = foa.Call(
				addr,
				testContract.MakeStoreCallData(t, num),
				math.MaxUint64,
				models.Balance(0))

			ret := foa.Call(
				addr,
				testContract.MakeRetrieveCallData(t),
				math.MaxUint64,
				models.Balance(0))

			require.Equal(t, num, new(big.Int).SetBytes(ret))
		})
	})
}

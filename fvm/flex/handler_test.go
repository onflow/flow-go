package flex_test

import (
	"bytes"
	"io"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/flex"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/stretchr/testify/require"
)

func TestFlexContractHandler(t *testing.T) {
	t.Parallel()
	t.Run("test last executed block call", func(t *testing.T) {
		flex.RunWithTempLedger(t, func(led atree.Ledger) {
			handler := flex.NewFlexContractHandler(led)
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
		flex.RunWithTempLedger(t, func(led atree.Ledger) {
			handler := flex.NewFlexContractHandler(led)
			foa := handler.NewFlowOwnedAccount()
			require.NotNil(t, foa)

			expectedAddress := models.NewFlexAddress(common.HexToAddress("0x00000000000000000001"))
			require.Equal(t, expectedAddress, foa.Address())
		})
	})

	t.Run("test running transaction", func(t *testing.T) {
		flex.RunWithTempLedger(t, func(led atree.Ledger) {
			handler := flex.NewFlexContractHandler(led)
			keyHex := "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
			key, _ := crypto.HexToECDSA(keyHex)
			address := crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379

			// deposit 200_000_000_000 to the foa account
			foa := handler.NewFlowOwnedAccount()
			vault := models.NewFlowTokenVault(models.Balance(200_000_000_000))
			foa.Deposit(vault)

			// transfer 80_000_000_000 of the money to the non-foa address
			foa.Call(*models.NewFlexAddress(address), nil, 400000, models.Balance(80_000_000_000))
			require.Equal(t, models.Balance(120_000_000_000), foa.Balance())

			// transfer 20_000_000 back to the foa through
			flexConf := env.NewFlexConfig()
			signer := types.MakeSigner(flexConf.ChainConfig, env.BlockNumberForEVMRules, flexConf.BlockContext.Time)
			tx, _ := types.SignTx(types.NewTransaction(
				0,
				foa.Address().ToCommon(),
				models.Balance(20_000_000).ToAttoFlow(),
				params.TxGas,
				common.Big1,
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

			require.Equal(t, models.Balance(140_000_000_000), foa.Balance())

			// fees has been collected
			require.NotEqual(t, models.Balance(0), foa2.Balance())

		})
	})
}

func TestFOA(t *testing.T) {
	flex.RunWithTempLedger(t, func(led atree.Ledger) {
		handler := flex.NewFlexContractHandler(led)
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
}

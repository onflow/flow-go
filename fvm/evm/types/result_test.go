package types

import (
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	"github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func Test_ResultErrors(t *testing.T) {
	t.Run("invalid result summary", func(t *testing.T) {
		res := Result{
			ValidationError: gethCore.ErrGasLimitReached,
			TxType:          0,
			GasConsumed:     InvalidTransactionGasCost,
			ReturnedValue:   []byte{0x01},
			TxHash:          gethCommon.Hash{0x01, 0x02},
			Index:           0,
		}

		require.True(t, res.Invalid())
		require.False(t, res.Failed())
		require.Empty(t, res.VMErrorString())

		sum := res.ResultSummary()
		require.Equal(t, StatusInvalid, sum.Status)
		require.Equal(t, ValidationErrCodeGasLimitReached, sum.ErrorCode)
		require.Equal(t, Data([]byte{0x01}), sum.ReturnedValue)
	})

	t.Run("setting invalid result", func(t *testing.T) {
		res := Result{}
		res.SetValidationError(gethCore.ErrNonceMax)

		require.True(t, res.Invalid())
		require.False(t, res.Failed())
		sum := res.ResultSummary()
		require.Equal(t, ValidationErrCodeNonceMax, sum.ErrorCode)
		require.Equal(t, StatusInvalid, sum.Status)
	})

	t.Run("successful result", func(t *testing.T) {
		const gas = uint64(1000)
		addr := NewAddress(gethCommon.Address{0x01})
		res := Result{
			TxType:                  1,
			GasConsumed:             gas,
			DeployedContractAddress: &addr,
			ReturnedValue:           []byte{0x01},
			TxHash:                  gethCommon.Hash{0x02},
			Index:                   1,
		}

		sum := res.ResultSummary()
		require.Equal(t, ErrCodeNoError, sum.ErrorCode)
		require.Equal(t, gas, sum.GasConsumed)
		require.Equal(t, StatusSuccessful, sum.Status)
		require.Equal(t, &addr, sum.DeployedContractAddress)
		require.Equal(t, Data([]byte{0x01}), sum.ReturnedValue)
	})

	t.Run("failed result", func(t *testing.T) {
		res := Result{
			VMError: gethVM.ErrGasUintOverflow,
		}

		require.True(t, res.Failed())
		require.False(t, res.Invalid())
		require.Equal(t, "gas uint64 overflow", res.VMErrorString())

		sum := res.ResultSummary()
		require.Equal(t, StatusFailed, sum.Status)
		require.Equal(t, ExecutionErrCodeGasUintOverflow, sum.ErrorCode)
	})

	t.Run("receipt", func(t *testing.T) {
		const gas = uint64(2000)
		deploy := NewAddress(gethCommon.Address{0x02, 0x03})
		res := Result{
			TxType:                  1,
			GasConsumed:             gas,
			DeployedContractAddress: &deploy,
		}

		rec := res.Receipt()
		require.Equal(t, deploy.ToCommon(), rec.ContractAddress)
		require.Equal(t, types.ReceiptStatusSuccessful, rec.Status)

		res = Result{
			GasConsumed: gas,
			TxType:      1,
			VMError:     gethCore.ErrGasUintOverflow,
		}

		rec = res.Receipt()
		require.Equal(t, types.ReceiptStatusFailed, rec.Status)
		require.Equal(t, gas, rec.CumulativeGasUsed)

		// if invalid we should return nil
		res = Result{
			ValidationError: gethCore.ErrNonceMax,
		}
		require.Nil(t, res.Receipt())
	})

	t.Run("invalid result construct", func(t *testing.T) {
		res := NewInvalidResult(
			types.NewTransaction(1, gethCommon.Address{}, nil, 100, nil, nil),
			gethCore.ErrGasLimitReached,
		)

		require.True(t, res.Invalid())
		require.False(t, res.Failed())
	})
}

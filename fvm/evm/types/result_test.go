package types

import (
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	"github.com/onflow/go-ethereum/core/types"
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

	t.Run("receipt", func(t *testing.T) {
		const gas = uint64(2000)
		res := Result{
			GasConsumed: gas,
			TxType:      1,
			VMError:     gethCore.ErrGasUintOverflow,
		}

		rec := res.Receipt()
		require.Equal(t, types.ReceiptStatusFailed, rec.Status)
		require.Equal(t, gas, rec.CumulativeGasUsed)

		// if invalid we should return nil
		res = Result{
			ValidationError: gethCore.ErrNonceMax,
		}
		require.Nil(t, res.Receipt())
	})
}

package types

import (
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	"github.com/stretchr/testify/require"
)

func Test_ResultInvalid(t *testing.T) {
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
}

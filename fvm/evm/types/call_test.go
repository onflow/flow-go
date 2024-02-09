package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectCall(t *testing.T) {
	dc := &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DepositCallSubType,
		From:     Address{0x1, 0x2},
		To:       Address{0x3, 0x4},
		Data:     []byte{0xf, 0xa, 0xb},
		Value:    big.NewInt(5),
		GasLimit: 100,
	}

	t.Run("calculate hash", func(t *testing.T) {
		h, err := dc.Hash()
		require.NoError(t, err)
		assert.Equal(t, "0xe28ff08eca95608646d765e3007b3710f7f2a8ac5e297431da1962c33487e7b6", h.Hex())
	})

	t.Run("construct transaction", func(t *testing.T) {
		tx := dc.Transaction()
		h, err := dc.Hash()
		require.NoError(t, err)
		assert.Equal(t, dc.Value, tx.Value())
		assert.Equal(t, dc.To.ToCommon(), *tx.To())
		assert.Equal(t, h, tx.Hash())
		assert.Equal(t, dc.GasLimit, tx.Gas())
		assert.Equal(t, dc.Data, tx.Data())
		assert.Equal(t, uint64(0), tx.Nonce()) // no nonce exists for direct call
	})
}

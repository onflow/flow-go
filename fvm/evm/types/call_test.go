package types

import (
	"bytes"
	"io"
	"math/big"
	"testing"

	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/rlp"
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
		assert.Equal(t, "0xed76124cc3c59f13e1113f5c380e2a67dab9bf616afc645073d2491fe3aecb62", h.Hex())

		// the hash should stay the same after RLP encoding and decoding
		var b bytes.Buffer
		writer := io.Writer(&b)
		err = dc.Transaction().EncodeRLP(writer)
		require.NoError(t, err)

		reconstructedTx := &gethTypes.Transaction{}
		err = reconstructedTx.DecodeRLP(rlp.NewStream(io.Reader(&b), 1000))
		require.NoError(t, err)

		h = reconstructedTx.Hash()
		assert.Equal(t, "0xed76124cc3c59f13e1113f5c380e2a67dab9bf616afc645073d2491fe3aecb62", h.Hex())
	})

	t.Run("same content except `from` should result in different hashes", func(t *testing.T) {
		h, err := dc.Hash()
		require.NoError(t, err)

		dc.From = Address{0x4, 0x5}
		h2, err := dc.Hash()
		require.NoError(t, err)

		assert.NotEqual(t, h2.Hex(), h.Hex())
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

		v, r, s := tx.RawSignatureValues()
		require.Equal(t, dc.From.Bytes(), r.Bytes())
		require.Equal(t, []byte{dc.SubType}, s.Bytes())
		require.Equal(t, []byte{DirectCallTxType}, v.Bytes())
	})
}

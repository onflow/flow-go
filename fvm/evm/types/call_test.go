package types

import (
	"encoding/hex"
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

func TestDirectCallHashCollision(t *testing.T) {
	// 	[
	//   "0xff",
	//   "0x05",
	//   "0x0000000000000000000000024e2b7e2c92f95905", from address
	//   "0x2b7e32bb7f9ba35ea1a0d8181c8d163b3b0d5ea2",
	//   "0x",
	//   "0x016345785d8a0000",
	//   "0x5b05",
	//   "0x05"
	// ]
	payload1, err := hex.DecodeString("fff83b81ff05940000000000000000000000024b89c195c8fc2b7f942b7e32bb7f9ba35ea1a0d8181c8d163b3b0d5ea28088016345785d8a0000825b0505")
	require.NoError(t, err)

	// 	[
	//   "0xff",
	//   "0x05",
	//   "0x0000000000000000000000024b89c195c8fc2b7f", from address
	//   "0x2b7e32bb7f9ba35ea1a0d8181c8d163b3b0d5ea2",
	//   "0x",
	//   "0x016345785d8a0000",
	//   "0x5b05",
	//   "0x05"
	// ]
	payload2, err := hex.DecodeString("fff83b81ff05940000000000000000000000024e2b7e2c92f95905942b7e32bb7f9ba35ea1a0d8181c8d163b3b0d5ea28088016345785d8a0000825b0505")
	require.NoError(t, err)

	// From the above 2 different direct calls, only the `from` is
	// different. That should affect the generated hash of the two
	// direct calls, but it doesn't currently. Hence, we have a hash
	// collision.
	// The reason being:
	// We use geth transaction hash calculation since direct call hash is included in the
	// block transaction hashes, and thus observed as any other transaction
	// return dc.Transaction().Hash(), nil
	// But when we convert a direct call to a `LegacyTx` transaction,
	// the `from` field is not taken into account.

	dc1, err := DirectCallFromEncoded(payload1)
	require.NoError(t, err)

	dc2, err := DirectCallFromEncoded(payload2)
	require.NoError(t, err)

	assert.Equal(t, byte(5), dc1.SubType)

	assert.Equal(t, byte(5), dc2.SubType)

	hash1, err := dc1.Hash()
	require.NoError(t, err)

	hash2, err := dc2.Hash()
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

package flow_test

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

type legacyAccountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
	SeqNumber uint64
	// "Revoked" field intentionally omitted
}

func TestDecodeAccountPublicKey_Legacy(t *testing.T) {
	publicKeyBytes, err := hex.DecodeString("98fd858d00b995cc9d67ec6c140f0fc26fcf7895621b3bf2661ea7b99e54f8cebd9143b50d21e6854187f895db7b6c7ec3032be5047b3652a0c2686e5b97de48")
	require.NoError(t, err)

	publicKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, publicKeyBytes)
	require.NoError(t, err)

	sigAlgo := crypto.ECDSAP256
	hashAlgo := hash.SHA3_256
	weight := 1000
	sequenceNumber := uint64(42)

	// manually encode key in legacy format
	w := legacyAccountPublicKeyWrapper{
		PublicKey: publicKey.Encode(),
		SignAlgo:  uint(crypto.ECDSAP256),
		HashAlgo:  uint(hash.SHA3_256),
		Weight:    uint(weight),
		SeqNumber: sequenceNumber,
	}

	b, err := rlp.EncodeToBytes(&w)
	require.NoError(t, err)

	accountKey, err := flow.DecodeAccountPublicKey(b, 1)
	require.NoError(t, err)

	assert.Equal(t, 1, accountKey.Index)
	assert.Equal(t, publicKey, accountKey.PublicKey)
	assert.Equal(t, sigAlgo, accountKey.SignAlgo)
	assert.Equal(t, hashAlgo, accountKey.HashAlgo)
	assert.Equal(t, weight, accountKey.Weight)
	assert.Equal(t, sequenceNumber, accountKey.SeqNumber)

	// legacy account key should not be revoked
	assert.False(t, accountKey.Revoked)
}

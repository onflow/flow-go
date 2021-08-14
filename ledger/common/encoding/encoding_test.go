package encoding_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// TODO add tests for raw byte values (useful for versioning)

// Test_KeyPartEncodingDecoding tests encoding decoding functionality of a ledger key part
func Test_KeyPartEncodingDecoding(t *testing.T) {

	kp := utils.KeyPartFixture(1, "key part 1")
	encoded := encoding.EncodeKeyPart(&kp)
	newkp, err := encoding.DecodeKeyPart(encoded)
	require.NoError(t, err)
	require.True(t, kp.Equals(newkp))

	// wrong type decoding
	_, err = encoding.DecodeKey(encoded)
	require.Error(t, err)

	// test wrong version decoding
	encoded[0] = uint8(1)
	_, err = encoding.DecodeKeyPart(encoded)
	require.Error(t, err)
}

// Test_KeyEncodingDecoding tests encoding decoding functionality of a ledger key
func Test_KeyEncodingDecoding(t *testing.T) {
	kp1 := utils.KeyPartFixture(1, "key part 1")
	kp2 := utils.KeyPartFixture(22, "key part 2")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	encoded := encoding.EncodeKey(&k)
	newk, err := encoding.DecodeKey(encoded)
	require.NoError(t, err)
	require.True(t, newk.Equals(&k))
}

// Test_ValueEncodingDecoding tests encoding decoding functionality of a ledger value
func Test_ValueEncodingDecoding(t *testing.T) {
	v := ledger.Value("value")
	encoded := encoding.EncodeValue(v)
	newV, err := encoding.DecodeValue(encoded)
	require.NoError(t, err)
	require.Equal(t, v, newV)
}

// Test_PayloadEncodingDecoding tests encoding decoding functionality of a payload
func Test_PayloadEncodingDecoding(t *testing.T) {
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	encoded := encoding.EncodePayload(p)
	newp, err := encoding.DecodePayload(encoded)
	require.NoError(t, err)
	require.True(t, newp.Equals(p))
}

// Test_ProofEncodingDecoding tests encoding decoding functionality of a proof
func Test_TrieProofEncodingDecoding(t *testing.T) {
	p, _ := utils.TrieProofFixture()
	encoded := encoding.EncodeTrieProof(p)
	newp, err := encoding.DecodeTrieProof(encoded)
	require.NoError(t, err)
	require.True(t, newp.Equals(p))
}

// Test_BatchProofEncodingDecoding tests encoding decoding functionality of a batch proof
func Test_BatchProofEncodingDecoding(t *testing.T) {
	bp, _ := utils.TrieBatchProofFixture()
	encoded := encoding.EncodeTrieBatchProof(bp)
	newbp, err := encoding.DecodeTrieBatchProof(encoded)
	require.NoError(t, err)
	require.True(t, newbp.Equals(bp))
}

// Test_TrieUpdateEncodingDecoding tests encoding decoding functionality of a trie update
func Test_TrieUpdateEncodingDecoding(t *testing.T) {

	p1 := utils.PathByUint16(2)
	kp1 := ledger.NewKeyPart(uint16(1), []byte("key 1 part 1"))
	kp2 := ledger.NewKeyPart(uint16(22), []byte("key 1 part 2"))
	k1 := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	pl1 := ledger.NewPayload(k1, []byte{'A'})

	p2 := utils.PathByUint16(2)
	kp3 := ledger.NewKeyPart(uint16(1), []byte("key2 part 1"))
	k2 := ledger.NewKey([]ledger.KeyPart{kp3})
	pl2 := ledger.NewPayload(k2, []byte{'B'})

	tu := &ledger.TrieUpdate{
		RootHash: utils.RootHashFixture(),
		Paths:    []ledger.Path{p1, p2},
		Payloads: []*ledger.Payload{pl1, pl2},
	}

	encoded := encoding.EncodeTrieUpdate(tu)
	newtu, err := encoding.DecodeTrieUpdate(encoded)
	require.NoError(t, err)
	require.True(t, newtu.Equals(tu))
}

package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
)

// TODO add tests for raw byte values (useful for versioning)

// Test_KeyPartEncodingDecoding tests encoding decoding functionality of a ledger key part
func Test_KeyPartEncodingDecoding(t *testing.T) {

	kp := common.KeyPartFixture(1, "key part 1")
	encoded := common.EncodeKeyPart(kp)
	newkp, err := common.DecodeKeyPart(encoded)
	require.NoError(t, err)
	require.True(t, kp.Equals(newkp))

	// wrong type decoding
	_, err = common.DecodeKey(encoded)
	require.Error(t, err)

	// test wrong version decoding
	encoded[0] = byte(uint8(1))
	_, err = common.DecodeKeyPart(encoded)
	require.Error(t, err)
}

// Test_KeyEncodingDecoding tests encoding decoding functionality of a ledger key
func Test_KeyEncodingDecoding(t *testing.T) {
	kp1 := common.KeyPartFixture(1, "key part 1")
	kp2 := common.KeyPartFixture(22, "key part 2")
	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})
	encoded := common.EncodeKey(k)
	newk, err := common.DecodeKey(encoded)
	require.NoError(t, err)
	require.True(t, newk.Equals(k))
}

// Test_ValueEncodingDecoding tests encoding decoding functionality of a ledger value
func Test_ValueEncodingDecoding(t *testing.T) {
	v := ledger.Value("value")
	encoded := common.EncodeValue(v)
	newV, err := common.DecodeValue(encoded)
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

	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(*k, v)

	encoded := common.EncodePayload(p)
	newp, err := common.DecodePayload(encoded)
	require.NoError(t, err)
	require.True(t, newp.Equals(p))
}

// Test_ProofEncodingDecoding tests encoding decoding functionality of a proof
func Test_ProofEncodingDecoding(t *testing.T) {
	p, _ := common.ProofFixture()
	encoded := common.EncodeProof(p)
	newp, err := common.DecodeProof(encoded)
	require.NoError(t, err)
	require.True(t, newp.Equals(p))
}

// Test_BatchProofEncodingDecoding tests encoding decoding functionality of a batch proof
func Test_BatchProofEncodingDecoding(t *testing.T) {
	bp, _ := common.BatchProofFixture()
	encoded := common.EncodeBatchProof(bp)
	newbp, err := common.DecodeBatchProof(encoded)
	require.NoError(t, err)
	require.True(t, newbp.Equals(bp))
}

// Test_TrieUpdateEncodingDecoding tests encoding decoding functionality of a trie update
func Test_TrieUpdateEncodingDecoding(t *testing.T) {

	p1 := common.TwoBytesPath(2)
	kp1 := ledger.NewKeyPart(uint16(1), []byte("key 1 part 1"))
	kp2 := ledger.NewKeyPart(uint16(22), []byte("key 1 part 2"))
	k1 := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})
	pl1 := ledger.NewPayload(*k1, ledger.Value([]byte{'A'}))

	p2 := common.TwoBytesPath(2)
	kp3 := ledger.NewKeyPart(uint16(1), []byte("key2 part 1"))
	k2 := ledger.NewKey([]ledger.KeyPart{*kp3})
	pl2 := ledger.NewPayload(*k2, ledger.Value([]byte{'B'}))

	tu := &ledger.TrieUpdate{
		StateCommitment: common.StateCommitmentFixture(),
		Paths:           []ledger.Path{p1, p2},
		Payloads:        []*ledger.Payload{pl1, pl2},
	}

	encoded := common.EncodeTrieUpdate(tu)
	newtu, err := common.DecodeTrieUpdate(encoded)
	require.NoError(t, err)
	require.True(t, newtu.Equals(tu))
}

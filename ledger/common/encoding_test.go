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
	kpt := uint16(1)
	kpv := []byte("key part")
	kp := ledger.NewKeyPart(kpt, kpv)

	encoded := common.EncodeKeyPart(kp)
	newkp, err := common.DecodeKeyPart(encoded)
	require.NoError(t, err)
	require.Equal(t, newkp.Type, kpt)
	require.Equal(t, newkp.Value, kpv)

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
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})

	encoded := common.EncodeKey(k)
	newk, err := common.DecodeKey(encoded)
	require.NoError(t, err)

	require.True(t, newk.Equal(k))
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
	require.True(t, newp.Equal(p))
}

// Test_ProofEncodingDecoding tests encoding decoding functionality of a proof
func Test_ProofEncodingDecoding(t *testing.T) {
	p := common.ProofFixture()
	encoded := common.EncodeProof(p)
	newp, err := common.DecodeProof(encoded)
	require.NoError(t, err)
	require.True(t, newp.Equal(p))
}

// Test_BatchProofEncodingDecoding tests encoding decoding functionality of a batch proof
func Test_BatchProofEncodingDecoding(t *testing.T) {
	bp := common.BatchProofFixture()
	encoded := common.EncodeBatchProof(bp)
	newbp, err := common.DecodeBatchProof(encoded)
	require.NoError(t, err)
	require.True(t, newbp.Equal(bp))
}

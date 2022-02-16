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

func Test_NilPayloadWithoutPrefixEncodingDecoding(t *testing.T) {

	buf := []byte{1, 2, 3}
	bufLen := len(buf)

	// Test encoded payload data length
	encodedPayloadLen := encoding.EncodedPayloadLengthWithoutPrefix(nil)
	require.Equal(t, 0, encodedPayloadLen)

	// Encode payload and append to buffer
	encoded := encoding.EncodeAndAppendPayloadWithoutPrefix(buf, nil)
	// Test encoded data size
	require.Equal(t, bufLen, len(encoded))
	// Test original input data isn't modified
	require.Equal(t, buf, encoded)
	// Test returned encoded data reuses input data
	require.True(t, &buf[0] == &encoded[0])

	// Decode and copy payload (excluding prefix)
	newp, err := encoding.DecodePayloadWithoutPrefix(encoded[bufLen:], false)
	require.NoError(t, err)
	require.Nil(t, newp)

	// Zerocopy option has no effect for nil payload, but test it anyway.
	// Decode payload (excluding prefix) with zero copy
	newp, err = encoding.DecodePayloadWithoutPrefix(encoded[bufLen:], true)
	require.NoError(t, err)
	require.Nil(t, newp)
}

func Test_PayloadWithoutPrefixEncodingDecoding(t *testing.T) {

	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	const encodedPayloadSize = 47 // size of encoded payload p without prefix (version + type)

	testCases := []struct {
		name     string
		payload  *ledger.Payload
		bufCap   int
		zeroCopy bool
	}{
		// full cap means no capacity for appending payload (new alloc)
		{"full cap zerocopy", p, 0, true},
		{"full cap", p, 0, false},
		// small cap means not enough capacity for appending payload (new alloc)
		{"small cap zerocopy", p, encodedPayloadSize - 1, true},
		{"small cap", p, encodedPayloadSize - 1, false},
		// exact cap means exact capacity for appending payload (no alloc)
		{"exact cap zerocopy", p, encodedPayloadSize, true},
		{"exact cap", p, encodedPayloadSize, false},
		// large cap means extra capacity than is needed for appending payload (no alloc)
		{"large cap zerocopy", p, encodedPayloadSize + 1, true},
		{"large cap", p, encodedPayloadSize + 1, false},
	}

	bufPrefix := []byte{1, 2, 3}
	bufPrefixLen := len(bufPrefix)

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {

			// Create a buffer of specified cap + prefix length
			buffer := make([]byte, bufPrefixLen, bufPrefixLen+tc.bufCap)
			copy(buffer, bufPrefix)

			// Encode payload and append to buffer
			encoded := encoding.EncodeAndAppendPayloadWithoutPrefix(buffer, tc.payload)
			encodedPayloadLen := encoding.EncodedPayloadLengthWithoutPrefix(tc.payload)
			// Test encoded data size
			require.Equal(t, len(encoded), bufPrefixLen+encodedPayloadLen)
			// Test if original input data is modified
			require.Equal(t, bufPrefix, encoded[:bufPrefixLen])
			// Test if input buffer is reused if it fits
			if tc.bufCap >= encodedPayloadLen {
				require.True(t, &buffer[0] == &encoded[0])
			} else {
				// new alloc
				require.True(t, &buffer[0] != &encoded[0])
			}

			// Decode payload (excluding prefix)
			newp, err := encoding.DecodePayloadWithoutPrefix(encoded[bufPrefixLen:], tc.zeroCopy)
			require.NoError(t, err)
			require.True(t, newp.Equals(tc.payload))

			// Reset encoded payload
			for i := 0; i < len(encoded); i++ {
				encoded[i] = 0
			}

			if tc.zeroCopy {
				// Test if decoded payload is changed after source data is modified
				// because data is shared.
				require.False(t, newp.Equals(tc.payload))
			} else {
				// Test if decoded payload is unchanged after source data is modified.
				require.True(t, newp.Equals(tc.payload))
			}
		})
	}
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

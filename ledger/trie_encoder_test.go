package ledger_test

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// TestKeyPartSerialization tests encoding and decoding functionality of a ledger key part
func TestKeyPartSerialization(t *testing.T) {
	kp := testutils.KeyPartFixture(1, "key part 1")

	encodedV0 := []byte{
		0x00, 0x00, // version 0
		0x02,       // type
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
	}

	encodedV1 := []byte{
		0x00, 0x01, // version 1
		0x02,       // type
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeKeyPart(&kp)
		require.Equal(t, encodedV1, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		// decode key part encoded in version 0
		decodedkp, err := ledger.DecodeKeyPart(encodedV0)
		require.NoError(t, err)
		require.Equal(t, kp, *decodedkp)

		// decode key part encoded in version 1
		decodedkp, err = ledger.DecodeKeyPart(encodedV1)
		require.NoError(t, err)
		require.Equal(t, kp, *decodedkp)
	})

	t.Run("roundtrip", func(t *testing.T) {
		encoded := ledger.EncodeKeyPart(&kp)
		newkp, err := ledger.DecodeKeyPart(encoded)
		require.NoError(t, err)
		require.Equal(t, kp, *newkp)

		// wrong type decoding
		_, err = ledger.DecodeKey(encoded)
		require.Error(t, err)

		// test wrong version decoding
		binary.BigEndian.PutUint16(encoded, ledger.PayloadVersion+1)
		_, err = ledger.DecodeKeyPart(encoded)
		require.Error(t, err)
	})
}

// TestKeySerialization tests encoding and decoding functionality of a ledger key
func TestKeySerialization(t *testing.T) {
	kp1 := testutils.KeyPartFixture(1, "key part 1")
	kp2 := testutils.KeyPartFixture(22, "key part 2")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})

	encodedV0 := []byte{
		0x00, 0x00, // version 0
		0x03,       // type
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 2
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
	}

	encodedV1 := []byte{
		0x00, 0x01, // version 1
		0x03,       // type
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 2
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeKey(&k)
		require.Equal(t, encodedV1, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		// decode key encoded in version 0
		decodedk, err := ledger.DecodeKey(encodedV0)
		require.NoError(t, err)
		require.Equal(t, k, *decodedk)

		// decode key encoded in version 1
		decodedk, err = ledger.DecodeKey(encodedV1)
		require.NoError(t, err)
		require.Equal(t, k, *decodedk)
	})

	t.Run("roundtrip", func(t *testing.T) {
		encoded := ledger.EncodeKey(&k)
		newk, err := ledger.DecodeKey(encoded)
		require.NoError(t, err)
		require.Equal(t, k, *newk)
	})
}

// TestValueSerialization tests encoding and decoding functionality of a ledger value
func TestValueSerialization(t *testing.T) {
	v := ledger.Value("value")

	encodedV0 := []byte{
		0x00, 0x01, // version 1
		0x04,                         // type
		0x76, 0x61, 0x6c, 0x75, 0x65, // value
	}

	encodedV1 := []byte{
		0x00, 0x01, // version 1
		0x04,                         // type
		0x76, 0x61, 0x6c, 0x75, 0x65, // value
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeValue(v)
		require.Equal(t, encodedV1, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		// decode key encoded in version 0
		decodedv, err := ledger.DecodeValue(encodedV0)
		require.NoError(t, err)
		require.Equal(t, v, decodedv)

		// decode key encoded in version 1
		decodedv, err = ledger.DecodeValue(encodedV1)
		require.NoError(t, err)
		require.Equal(t, v, decodedv)
	})

	t.Run("roundtrip", func(t *testing.T) {
		encoded := ledger.EncodeValue(v)
		newV, err := ledger.DecodeValue(encoded)
		require.NoError(t, err)
		require.Equal(t, v, newV)
	})
}

// TestPayloadSerialization tests encoding and decoding functionality of a payload
func TestPayloadSerialization(t *testing.T) {
	kp1 := ledger.NewKeyPart(1, []byte("key part 1"))
	kp2 := ledger.NewKeyPart(uint16(22), []byte("key part 2"))
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	encodedV0 := []byte{
		0x00, 0x00, // version 0
		0x06,                   // type
		0x00, 0x00, 0x00, 0x22, // length of encoded key
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 2
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x41, // value
	}

	encodedV1 := []byte{
		0x00, 0x01, // version 1
		0x06,                   // type
		0x00, 0x00, 0x00, 0x22, // length of encoded key
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 2
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
		0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x41, // value
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodePayload(p)
		require.Equal(t, encodedV1, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		// decode payload encoded in version 0
		decodedp, err := ledger.DecodePayload(encodedV0)
		require.NoError(t, err)
		require.Equal(t, p, decodedp)

		// decode payload encoded in version 1
		decodedp, err = ledger.DecodePayload(encodedV1)
		require.NoError(t, err)
		require.Equal(t, p, decodedp)
	})

	t.Run("roundtrip", func(t *testing.T) {
		encoded := ledger.EncodePayload(p)
		newp, err := ledger.DecodePayload(encoded)
		require.NoError(t, err)
		require.Equal(t, p, newp)
	})
}

// TestNilPayloadWithoutPrefixSerialization tests encoding and decoding
// nil payload without prefix (version and type).
func TestNilPayloadWithoutPrefixSerialization(t *testing.T) {

	t.Run("encoding", func(t *testing.T) {
		buf := []byte{1, 2, 3}

		// Test encoded payload data length
		encodedPayloadLen := ledger.EncodedPayloadLengthWithoutPrefix(nil, ledger.PayloadVersion)
		require.Equal(t, 0, encodedPayloadLen)

		// Encode payload and append to buffer
		encoded := ledger.EncodeAndAppendPayloadWithoutPrefix(buf, nil, ledger.PayloadVersion)
		// Test original input data isn't modified
		require.Equal(t, buf, encoded)
		// Test returned encoded data reuses input data
		require.True(t, &buf[0] == &encoded[0])
	})

	t.Run("decoding", func(t *testing.T) {
		// Decode and copy payload (excluding prefix)
		newp, err := ledger.DecodePayloadWithoutPrefix([]byte{}, false, ledger.PayloadVersion)
		require.NoError(t, err)
		require.Nil(t, newp)

		// Zerocopy option has no effect for nil payload, but test it anyway.
		// Decode payload (excluding prefix) with zero copy
		newp, err = ledger.DecodePayloadWithoutPrefix([]byte{}, true, ledger.PayloadVersion)
		require.NoError(t, err)
		require.Nil(t, newp)
	})
}

// TestPayloadWithoutPrefixSerialization tests encoding and decoding payload without prefix (version and type).
func TestPayloadWithoutPrefixSerialization(t *testing.T) {
	kp1 := ledger.NewKeyPart(1, []byte("key part 1"))
	kp2 := ledger.NewKeyPart(22, []byte("key part 2"))
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	encodedV0 := []byte{
		0x00, 0x00, 0x00, 0x22, // length of encoded key
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 0
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x41, // value
	}

	encodedV1 := []byte{
		0x00, 0x00, 0x00, 0x22, // length of encoded key
		0x00, 0x02, // number of key parts
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 0
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value
		0x00, 0x00, 0x00, 0x0c, // length of encoded key part 1
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value
		0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x41, // value
	}

	t.Run("encoding", func(t *testing.T) {
		// encode payload without prefix using version 0
		encoded := ledger.EncodeAndAppendPayloadWithoutPrefix(nil, p, 0)
		encodedPayloadLen := ledger.EncodedPayloadLengthWithoutPrefix(p, 0)
		require.Equal(t, len(encodedV0), encodedPayloadLen)
		require.Equal(t, encodedV0, encoded)

		// encode payload without prefix using version 1
		encoded = ledger.EncodeAndAppendPayloadWithoutPrefix(nil, p, 1)
		encodedPayloadLen = ledger.EncodedPayloadLengthWithoutPrefix(p, 1)
		require.Equal(t, len(encodedV1), encodedPayloadLen)
		require.Equal(t, encodedV1, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		// decode payload without prefix encoding in verison 0
		decodedp, err := ledger.DecodePayloadWithoutPrefix(encodedV0, true, 0)
		require.NoError(t, err)
		require.Equal(t, p, decodedp)

		// decode payload without prefix encoding in verison 1
		decodedp, err = ledger.DecodePayloadWithoutPrefix(encodedV1, true, 1)
		require.NoError(t, err)
		require.Equal(t, p, decodedp)
	})

	const encodedPayloadSize = 43 // size of encoded payload p without prefix (version + type)

	testCases := []struct {
		name     string
		bufCap   int
		zeroCopy bool
	}{
		// full cap means no capacity for appending payload (new alloc)
		{"full cap zerocopy", 0, true},
		{"full cap", 0, false},
		// small cap means not enough capacity for appending payload (new alloc)
		{"small cap zerocopy", encodedPayloadSize - 1, true},
		{"small cap", encodedPayloadSize - 1, false},
		// exact cap means exact capacity for appending payload (no alloc)
		{"exact cap zerocopy", encodedPayloadSize, true},
		{"exact cap", encodedPayloadSize, false},
		// large cap means extra capacity than is needed for appending payload (no alloc)
		{"large cap zerocopy", encodedPayloadSize + 1, true},
		{"large cap", encodedPayloadSize + 1, false},
	}

	bufPrefix := []byte{1, 2, 3}
	bufPrefixLen := len(bufPrefix)

	for _, tc := range testCases {

		t.Run("roundtrip "+tc.name, func(t *testing.T) {
			// Create a buffer of specified cap + prefix length
			buffer := make([]byte, bufPrefixLen, bufPrefixLen+tc.bufCap)
			copy(buffer, bufPrefix)

			// Encode payload and append to buffer
			encoded := ledger.EncodeAndAppendPayloadWithoutPrefix(buffer, p, ledger.PayloadVersion)
			encodedPayloadLen := ledger.EncodedPayloadLengthWithoutPrefix(p, ledger.PayloadVersion)
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
			decodedp, err := ledger.DecodePayloadWithoutPrefix(encoded[bufPrefixLen:], tc.zeroCopy, ledger.PayloadVersion)
			require.NoError(t, err)
			require.Equal(t, p, decodedp)

			// Reset encoded payload
			for i := 0; i < len(encoded); i++ {
				encoded[i] = 0
			}

			if tc.zeroCopy {
				// Test if decoded payload is changed after source data is modified
				// because data is shared.
				require.NotEqual(t, p, decodedp)
			} else {
				// Test if decoded payload is unchanged after source data is modified.
				require.Equal(t, p, decodedp)
			}
		})
	}
}

// TestTrieProofSerialization tests encoding and decoding functionality of a proof
func TestTrieProofSerialization(t *testing.T) {

	interim1Bytes, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	interim2Bytes, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")

	var interim1, interim2 hash.Hash
	copy(interim1[:], interim1Bytes)
	copy(interim2[:], interim2Bytes)

	p := &ledger.TrieProof{
		Payload:   testutils.LightPayload8('A', 'A'),
		Interims:  []hash.Hash{interim1, interim2},
		Inclusion: true,
		Flags:     []byte{byte(130), byte(0)},
		Steps:     7,
		Path:      testutils.PathByUint16(330),
	}

	encodedV0 := []byte{
		0x00, 0x00, // version 0
		0x07,       // type
		0x80,       // inclusion
		0x07,       // step
		0x02,       // length of flag
		0x82, 0x00, // flag
		0x00, 0x20, // length of path
		0x01, 0x4a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x16, // length of encoded payload
		0x00, 0x00, 0x00, 0x09, // length of encoded payload key
		0x00, 0x01, // number of payload key parts
		0x00, 0x00, 0x00, 0x03, // length of encoded payload key part 0
		0x00, 0x00, // payload key part type
		0x41,                                           // payload key part value
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded payload value
		0x41,        // payload value
		0x02,        // number of interims
		0x00, 0x020, // length of encoded interim
		0xac, 0xcb, 0x03, 0x99, 0xdd, 0x2b, 0x3a, 0x7a,
		0x48, 0x61, 0x8b, 0x23, 0x76, 0xf5, 0xe6, 0x1d,
		0x82, 0x2e, 0x0c, 0x77, 0x36, 0xb0, 0x44, 0xc3,
		0x64, 0xa0, 0x5c, 0x29, 0x04, 0xa2, 0xf3, 0x15, // interim
		0x00, 0x020, // length of encoded interim
		0xf3, 0xfb, 0xa4, 0x26, 0xa2, 0xf0, 0x1c, 0x34,
		0x23, 0x04, 0xe3, 0xca, 0x77, 0x96, 0xc3, 0x98,
		0x0c, 0x62, 0xc6, 0x25, 0xf7, 0xfd, 0x43, 0x10,
		0x5a, 0xd5, 0xaf, 0xd9, 0x2b, 0x16, 0x55, 0x42, // interim
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeTrieProof(p)
		require.Equal(t, encodedV0, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		decodedp, err := ledger.DecodeTrieProof(encodedV0)
		require.NoError(t, err)
		require.True(t, decodedp.Equals(p))
	})

	t.Run("roundtrip", func(t *testing.T) {
		p, _ := testutils.TrieProofFixture()
		encoded := ledger.EncodeTrieProof(p)
		newp, err := ledger.DecodeTrieProof(encoded)
		require.NoError(t, err)
		require.True(t, newp.Equals(p))
	})
}

// TestBatchProofSerialization tests encoding and decoding functionality of a batch proof
func TestBatchProofSerialization(t *testing.T) {
	interim1Bytes, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	interim2Bytes, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")

	var interim1, interim2 hash.Hash
	copy(interim1[:], interim1Bytes)
	copy(interim2[:], interim2Bytes)

	p := &ledger.TrieProof{
		Payload:   testutils.LightPayload8('A', 'A'),
		Interims:  []hash.Hash{interim1, interim2},
		Inclusion: true,
		Flags:     []byte{byte(130), byte(0)},
		Steps:     7,
		Path:      testutils.PathByUint16(330),
	}

	bp := &ledger.TrieBatchProof{
		Proofs: []*ledger.TrieProof{p, p},
	}

	encodedProofV0 := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8a, // length of encoded proof
		0x80,       // inclusion
		0x07,       // step
		0x02,       // length of flag
		0x82, 0x00, // flag
		0x00, 0x20, // length of path
		0x01, 0x4a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x16, // length of encoded payload
		0x00, 0x00, 0x00, 0x09, // length of encoded payload key
		0x00, 0x01, // number of payload key parts
		0x00, 0x00, 0x00, 0x03, // length of encoded payload key part 0
		0x00, 0x00, // payload key part type
		0x41,                                           // payload key part value
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded payload value
		0x41,        // payload value
		0x02,        // number of interims
		0x00, 0x020, // length of encoded interim
		0xac, 0xcb, 0x03, 0x99, 0xdd, 0x2b, 0x3a, 0x7a,
		0x48, 0x61, 0x8b, 0x23, 0x76, 0xf5, 0xe6, 0x1d,
		0x82, 0x2e, 0x0c, 0x77, 0x36, 0xb0, 0x44, 0xc3,
		0x64, 0xa0, 0x5c, 0x29, 0x04, 0xa2, 0xf3, 0x15, // interim
		0x00, 0x020, // length of encoded interim
		0xf3, 0xfb, 0xa4, 0x26, 0xa2, 0xf0, 0x1c, 0x34,
		0x23, 0x04, 0xe3, 0xca, 0x77, 0x96, 0xc3, 0x98,
		0x0c, 0x62, 0xc6, 0x25, 0xf7, 0xfd, 0x43, 0x10,
		0x5a, 0xd5, 0xaf, 0xd9, 0x2b, 0x16, 0x55, 0x42, // interim
	}

	encodedBatchProofHead := []byte{
		0x00, 0x00, // version 0
		0x08,                   // type
		0x00, 0x00, 0x00, 0x02, // number of proofs
	}
	encodedV0 := append([]byte{}, encodedBatchProofHead...)
	encodedV0 = append(encodedV0, encodedProofV0...)
	encodedV0 = append(encodedV0, encodedProofV0...)

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeTrieBatchProof(bp)
		require.Equal(t, encodedV0, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		decodedbp, err := ledger.DecodeTrieBatchProof(encodedV0)
		require.NoError(t, err)
		require.True(t, decodedbp.Equals(bp))
	})

	t.Run("roundtrip", func(t *testing.T) {
		bp, _ = testutils.TrieBatchProofFixture()
		encoded := ledger.EncodeTrieBatchProof(bp)
		newbp, err := ledger.DecodeTrieBatchProof(encoded)
		require.NoError(t, err)
		require.True(t, newbp.Equals(bp))
	})
}

// TestTrieUpdateSerialization tests encoding and decoding functionality of a trie update
func TestTrieUpdateSerialization(t *testing.T) {

	p1 := testutils.PathByUint16(1)
	kp1 := ledger.NewKeyPart(uint16(1), []byte("key 1 part 1"))
	kp2 := ledger.NewKeyPart(uint16(22), []byte("key 1 part 2"))
	k1 := ledger.NewKey([]ledger.KeyPart{kp1, kp2})
	pl1 := ledger.NewPayload(k1, []byte{'A'})

	p2 := testutils.PathByUint16(2)
	kp3 := ledger.NewKeyPart(uint16(1), []byte("key 2 part 1"))
	k2 := ledger.NewKey([]ledger.KeyPart{kp3})
	pl2 := ledger.NewPayload(k2, []byte{'B'})

	tu := &ledger.TrieUpdate{
		RootHash: testutils.RootHashFixture(),
		Paths:    []ledger.Path{p1, p2},
		Payloads: []*ledger.Payload{pl1, pl2},
	}

	encodedV0 := []byte{
		0x00, 0x00, // version 0
		0x0b,        // type
		0x00, 0x020, // length of hash
		0x6a, 0x7a, 0x56, 0x5a, 0xdd, 0x94, 0xfb, 0x36,
		0x06, 0x9d, 0x79, 0xe8, 0x72, 0x5c, 0x22, 0x1c,
		0xd1, 0xe5, 0x74, 0x07, 0x42, 0x50, 0x1e, 0xf0,
		0x14, 0xea, 0x6d, 0xb9, 0x99, 0xfd, 0x98, 0xad, // root hash
		0x00, 0x00, 0x00, 0x02, // number of paths
		0x00, 0x20, // length of path
		0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path 1
		0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path 2
		0x00, 0x00, 0x00, 0x33, // length of encoded payload
		0x00, 0x00, 0x00, 0x26, // length of encoded key
		0x00, 0x02, // number of key parts: 2
		0x00, 0x00, 0x00, 0x0e, // length of encoded key part 0
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x31, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value: key 1 part 1
		0x00, 0x00, 0x00, 0x0e, // length of encoded key part 1
		0x00, 0x16, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x31, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x32, // key part value: key 1 part 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x41,                   // value
		0x00, 0x00, 0x00, 0x21, // length of encoded payload
		0x00, 0x00, 0x00, 0x14, // length of encoded key
		0x00, 0x01, // number of key parts
		0x00, 0x00, 0x00, 0x0e, // length of encoded key part 0
		0x00, 0x01, // key part type
		0x6b, 0x65, 0x79, 0x20, 0x32, 0x20, 0x70, 0x61, 0x72, 0x74, 0x20, 0x31, // key part value: key 2 part 1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // length of encoded value
		0x42, // value
	}

	t.Run("encoding", func(t *testing.T) {
		encoded := ledger.EncodeTrieUpdate(tu)
		require.Equal(t, encodedV0, encoded)
	})

	t.Run("decoding", func(t *testing.T) {
		decodedtu, err := ledger.DecodeTrieUpdate(encodedV0)
		require.NoError(t, err)
		require.True(t, decodedtu.Equals(tu))
	})

	t.Run("roundtrip", func(t *testing.T) {
		encoded := ledger.EncodeTrieUpdate(tu)
		decodedtu, err := ledger.DecodeTrieUpdate(encoded)
		require.NoError(t, err)
		require.True(t, decodedtu.Equals(tu))
	})
}

package accountkeymetadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindDuplicateKey(t *testing.T) {
	testcases := []struct {
		name                      string
		deduplicated              bool
		data                      []byte
		encodedKey                []byte
		getKeyDigest              func([]byte) uint64
		getStoredKey              func(uint32) ([]byte, error)
		expectedDigest            uint64
		expectedFound             bool
		expectedDuplicateKeyIndex uint32
		expectError               bool
	}{
		{
			name:         "no duplicate digest",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			encodedKey: []byte{0x01}, // not used in this test case
			getKeyDigest: func(encodedKey []byte) uint64 {
				require.Equal(t, []byte{0x01}, encodedKey)
				return 1
			},
			getStoredKey:   nil, // not used in this test case
			expectedDigest: 1,
			expectedFound:  false,
		},
		{
			name:         "digest collision",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			encodedKey: []byte{0x01},
			getKeyDigest: func(encodedKey []byte) uint64 {
				require.Equal(t, []byte{0x01}, encodedKey)
				return 2
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Equal(t, uint32(1), keyIndex)
				return []byte{0x02}, nil
			},
			expectedDigest: SentinelFastDigest64,
			expectedFound:  false,
			expectError:    false,
		},
		{
			name:         "duplicate key",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			encodedKey: []byte{0x01},
			getKeyDigest: func(encodedKey []byte) uint64 {
				require.Equal(t, []byte{0x01}, encodedKey)
				return 3
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Equal(t, uint32(2), keyIndex)
				return []byte{0x01}, nil
			},
			expectedDigest:            3,
			expectedFound:             true,
			expectedDuplicateKeyIndex: 2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			keyMetadata, err := NewKeyMetadataAppenderFromBytes(tc.data, tc.deduplicated, maxStoredDigests)
			require.NoError(t, err)

			digest, found, duplicateStoredKeyIndex, err := FindDuplicateKey(
				keyMetadata,
				tc.encodedKey,
				tc.getKeyDigest,
				tc.getStoredKey,
			)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedDigest, digest)
			require.Equal(t, tc.expectedFound, found)
			require.Equal(t, tc.expectedDuplicateKeyIndex, duplicateStoredKeyIndex)
		})
	}
}

func TestDecodeDigests(t *testing.T) {
	testcases := []struct {
		name           string
		encodedDigests []byte
		digests        []uint64
		hasError       bool
	}{
		{
			name:           "nil encoded digests",
			encodedDigests: nil,
			digests:        []uint64{},
		},
		{
			name:           "empty encoded digests",
			encodedDigests: []byte{},
			digests:        []uint64{},
		},
		{
			name:           "truncated encoded digests",
			encodedDigests: []byte{0},
			hasError:       true,
		},
		{
			name:           "1 digests",
			encodedDigests: []byte{0, 0, 0, 0, 0, 0, 0, 1},
			digests:        []uint64{1},
		},
		{
			name: "2 digests",
			encodedDigests: []byte{
				0, 0, 0, 0, 0, 0, 0, 1,
				0, 0, 0, 0, 0, 0, 0, 2,
			},
			digests: []uint64{1, 2},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeDigests(tc.encodedDigests)
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.digests, decoded)
		})
	}
}

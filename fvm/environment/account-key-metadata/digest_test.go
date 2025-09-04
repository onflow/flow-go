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
		digest                    uint64
		encodedKey                []byte
		getStoredKey              func(uint32) ([]byte, error)
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
			digest:        1,
			encodedKey:    []byte{0x01}, // not used in this test case
			getStoredKey:  nil,          // not used in this test case
			expectedFound: false,
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
			digest:     2,
			encodedKey: []byte{0x01},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Equal(t, uint32(1), keyIndex)
				return []byte{0x02}, nil
			},
			expectedFound: false,
			expectError:   true,
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
			digest:     3,
			encodedKey: []byte{0x01},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Equal(t, uint32(2), keyIndex)
				return []byte{0x01}, nil
			},
			expectedFound:             true,
			expectedDuplicateKeyIndex: 2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			keyMetadata, err := NewKeyMetadataAppenderFromBytes(tc.data, tc.deduplicated, maxStoredDigests)
			require.NoError(t, err)

			found, duplicateStoredKeyIndex, err := FindDuplicateKey(
				keyMetadata,
				tc.encodedKey,
				tc.digest,
				tc.getStoredKey,
			)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedFound, found)
			require.Equal(t, tc.expectedDuplicateKeyIndex, duplicateStoredKeyIndex)
		})
	}
}

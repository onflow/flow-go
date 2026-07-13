package accountkeymetadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const maxStoredDigests = 2

func TestGetRevokedStatusFromKeyMetadataBytes(t *testing.T) {
	testcases := []struct {
		name                  string
		data                  []byte
		expectedRevokedStatus map[uint32]bool
	}{
		{
			name: "1 public key (1 revoked group)",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedRevokedStatus: map[uint32]bool{
				1: false,
			},
		},
		{
			name: "2 public keys (1 revoked group)",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: false,
				2: false,
			},
		},
		{
			name: "2 public keys (2 revoked groups)",
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: false,
				2: true,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			for keyIndex, expected := range tc.expectedRevokedStatus {
				revoked, err := GetRevokedStatus(tc.data, keyIndex)
				require.NoError(t, err)
				require.Equal(t, expected, revoked)
			}
		})
	}
}

func TestSetRevokedStatusInKeyMetadata(t *testing.T) {
	testcases := []struct {
		name             string
		data             []byte
		keyIndexToRevoke uint32
		expected         []byte
	}{
		{
			name: "revoke key in run length 1 group",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
		},
		{
			name: "revoke first key in run length 2 group",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "revoke second key in run length 2 group",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 2,
			expected: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "revoke key in run length 1 group (cannot merge with next group)",
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "no-op revoke",
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 2,
			expected: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "revoke first key in run length 3 group (no previous group)",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "revoke second key in run length 3 group",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 2,
			expected: []byte{
				0, 0, 0, 0x0c, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name: "revoke last key in run length 3 group",
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 3,
			expected: []byte{
				0, 0, 0, 0x8, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			newData, err := SetRevokedStatus(tc.data, tc.keyIndexToRevoke)
			require.NoError(t, err)
			require.Equal(t, tc.expected, newData)
		})
	}
}

func TestGetKeyMetadata(t *testing.T) {
	type keyMetadata struct {
		weight         uint16
		revoked        bool
		storedKeyIndex uint32
	}

	testcases := []struct {
		name                string
		deduplicated        bool
		data                []byte
		expectedKeyMetadata map[uint32]keyMetadata
	}{
		{
			name:         "not deduplicated, 2 account public keys",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedKeyMetadata: map[uint32]keyMetadata{
				1: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 1,
				},
			},
		},
		{
			name:         "not deduplicated, 3 account public keys",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0x80, 0x01, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedKeyMetadata: map[uint32]keyMetadata{
				1: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 1,
				},
				2: {
					weight:         1,
					revoked:        true,
					storedKeyIndex: 2,
				},
			},
		},
		{
			name:         "deduplicated, 2 account public keys, 1 stored key, deduplication from key at index 1",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
			},
			expectedKeyMetadata: map[uint32]keyMetadata{
				1: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 0,
				},
			},
		},
		{
			name:         "deduplicated, 3 account public keys, 2 stored keys, deduplication from key at index 1",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0x80, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedKeyMetadata: map[uint32]keyMetadata{
				1: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 0,
				},
				2: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 1,
				},
			},
		},
		{
			name:         "deduplicated, 4 account public keys, 2 stored keys, deduplication from key at index 2",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 1, 0x80, 0x01, // weight and revoked group
				0, 0, 0, 2, // start index for mapping
				0, 0, 0, 0x06, // length prefix for mapping
				0, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 2, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
				0, 0, 0, 0, 0, 0, 0, 4, // digest 4
			},
			expectedKeyMetadata: map[uint32]keyMetadata{
				1: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 1,
				},
				2: {
					weight:         1000,
					revoked:        false,
					storedKeyIndex: 0,
				},
				3: {
					weight:         1,
					revoked:        true,
					storedKeyIndex: 0,
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			for keyIndex, expected := range tc.expectedKeyMetadata {
				weight, revoked, storedKeyIndex, err := GetKeyMetadata(tc.data, keyIndex, tc.deduplicated)
				require.NoError(t, err)
				require.Equal(t, expected.weight, weight)
				require.Equal(t, expected.revoked, revoked)
				require.Equal(t, expected.storedKeyIndex, storedKeyIndex)
			}
		})
	}
}

func TestAppendUniqueKeyMetadata(t *testing.T) {
	testcases := []struct {
		name                   string
		deduplicated           bool
		data                   []byte
		key0Digest             uint64
		revoked                bool
		weight                 uint16
		digest                 uint64
		expectedData           []byte
		expectedStoredKeyIndex uint32
		expectedDeduplicated   bool
	}{
		{
			name:         "not deduplicated, append new key_1 to {key_0}",
			deduplicated: false,
			data:         []byte{},
			key0Digest:   1,
			revoked:      false,
			weight:       1000,
			digest:       2,
			expectedData: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			expectedStoredKeyIndex: 1,
			expectedDeduplicated:   false,
		},
		{
			name:         "not deduplicated, append new key_2 to {key_0, key_1}",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			revoked: false,
			weight:  1,
			digest:  3,
			expectedData: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0, 1, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
				0, 0, 0, 0, 0, 0, 0, 3, // key_2 digest
			},
			expectedStoredKeyIndex: 2,
			expectedDeduplicated:   false,
		},
		{
			name:         "deduplicated, append new key_1 to {key_0, key_0}",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
			},
			revoked: false,
			weight:  1000,
			digest:  3,
			expectedData: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0x80, 2, 0, 0, 0, 0, // mapping group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 3, // key_1 digest
			},
			expectedStoredKeyIndex: 1,
			expectedDeduplicated:   true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var km *KeyMetadataAppender
			var err error

			if len(tc.data) == 0 {
				km = NewKeyMetadataAppender(tc.key0Digest, maxStoredDigests)
			} else {
				km, err = NewKeyMetadataAppenderFromBytes(tc.data, tc.deduplicated, maxStoredDigests)
				require.NoError(t, err)
			}

			storedKeyIndex, err := km.AppendUniqueKeyMetadata(tc.revoked, tc.weight, tc.digest)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStoredKeyIndex, storedKeyIndex)

			newData, deduplicated := km.ToBytes()
			require.Equal(t, tc.expectedData, newData)
			require.Equal(t, tc.expectedDeduplicated, deduplicated)
		})
	}
}

func TestAppendDuplicateKeyMetadata(t *testing.T) {
	testcases := []struct {
		name                    string
		deduplicated            bool
		data                    []byte
		key0Digest              uint64
		revoked                 bool
		weight                  uint16
		keyIndex                uint32
		duplicateStoredKeyIndex uint32
		expectedData            []byte
		expectedDeduplicated    bool
	}{
		{
			name:                    "not deduplicated, append key_0 to {key_0}",
			deduplicated:            false,
			data:                    []byte{},
			key0Digest:              1,
			revoked:                 false,
			weight:                  1000,
			keyIndex:                1,
			duplicateStoredKeyIndex: 0,
			expectedData: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
			},
			expectedDeduplicated: true,
		},
		{
			name:         "not deduplicated, append key_0 to {key_0, key_1}",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			revoked:                 false,
			weight:                  1,
			keyIndex:                2,
			duplicateStoredKeyIndex: 0,
			expectedData: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0, 1, // weight and revoked group 2
				0, 0, 0, 2, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			expectedDeduplicated: true,
		},
		{
			name:         "deduplicated, append key_0 to {key_0, key_0}",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
			},
			revoked:                 false,
			weight:                  1000,
			keyIndex:                2,
			duplicateStoredKeyIndex: 0,
			expectedData: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
			},
			expectedDeduplicated: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var km *KeyMetadataAppender
			var err error

			if len(tc.data) == 0 {
				km = NewKeyMetadataAppender(tc.key0Digest, maxStoredDigests)
			} else {
				km, err = NewKeyMetadataAppenderFromBytes(tc.data, tc.deduplicated, maxStoredDigests)
				require.NoError(t, err)
			}

			err = km.AppendDuplicateKeyMetadata(tc.keyIndex, tc.duplicateStoredKeyIndex, tc.revoked, tc.weight)
			require.NoError(t, err)

			newData, deduplicated := km.ToBytes()
			require.Equal(t, tc.expectedData, newData)
			require.Equal(t, tc.expectedDeduplicated, deduplicated)
		})
	}
}

func TestDecodeKeyMetadata(t *testing.T) {
	testcases := []struct {
		name                             string
		deduplicated                     bool
		data                             []byte
		expectedWeightAndRevokedStatuses []WeightAndRevokedStatus
		expectedStartKeyIndexForMappings uint32
		expectedMappings                 []uint32
		expectedStartKeyIndexForDigests  uint32
		expectedDigests                  []uint64
	}{
		{
			name:         "not deduplicated, 2 account public keys",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedWeightAndRevokedStatuses: []WeightAndRevokedStatus{
				{Weight: 1000, Revoked: false},
			},
			expectedStartKeyIndexForMappings: 0,
			expectedMappings:                 nil,
			expectedStartKeyIndexForDigests:  0,
			expectedDigests:                  []uint64{1, 2},
		},
		{
			name:         "not deduplicated, 3 account public keys",
			deduplicated: false,
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0x80, 0x01, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedWeightAndRevokedStatuses: []WeightAndRevokedStatus{
				{Weight: 1000, Revoked: false},
				{Weight: 1, Revoked: true},
			},
			expectedStartKeyIndexForMappings: 0,
			expectedMappings:                 nil,
			expectedStartKeyIndexForDigests:  1,
			expectedDigests:                  []uint64{2, 3},
		},
		{
			name:         "deduplicated, 2 account public keys, 1 stored key, deduplication from key at index 1",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
			},
			expectedWeightAndRevokedStatuses: []WeightAndRevokedStatus{
				{Weight: 1000, Revoked: false},
			},
			expectedStartKeyIndexForMappings: 1,
			expectedMappings:                 []uint32{0},
			expectedStartKeyIndexForDigests:  0,
			expectedDigests:                  []uint64{1},
		},
		{
			name:         "deduplicated, 3 account public keys, 2 stored keys, deduplication from key at index 1",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0x80, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedWeightAndRevokedStatuses: []WeightAndRevokedStatus{
				{Weight: 1000, Revoked: false},
				{Weight: 1000, Revoked: false},
			},
			expectedStartKeyIndexForMappings: 1,
			expectedMappings:                 []uint32{0, 1},
			expectedStartKeyIndexForDigests:  0,
			expectedDigests:                  []uint64{1, 2},
		},
		{
			name:         "deduplicated, 4 account public keys, 2 stored keys, deduplication from key at index 2",
			deduplicated: true,
			data: []byte{
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 1, 0x80, 0x01, // weight and revoked group
				0, 0, 0, 2, // start index for mapping
				0, 0, 0, 0x06, // length prefix for mapping
				0, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 2, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
				0, 0, 0, 0, 0, 0, 0, 4, // digest 4
			},
			expectedWeightAndRevokedStatuses: []WeightAndRevokedStatus{
				{Weight: 1000, Revoked: false},
				{Weight: 1000, Revoked: false},
				{Weight: 1, Revoked: true},
			},
			expectedStartKeyIndexForMappings: 2,
			expectedMappings:                 []uint32{0, 0},
			expectedStartKeyIndexForDigests:  2,
			expectedDigests:                  []uint64{3, 4},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			weightAndRevokedStatuses, startKeyIndexForMappings, mappings, startKeyIndexForDigests, digests, err := DecodeKeyMetadata(tc.data, tc.deduplicated)
			require.NoError(t, err)
			require.Equal(t, tc.expectedWeightAndRevokedStatuses, weightAndRevokedStatuses)
			require.Equal(t, tc.expectedStartKeyIndexForMappings, startKeyIndexForMappings)
			require.Equal(t, tc.expectedMappings, mappings)
			require.Equal(t, tc.expectedStartKeyIndexForDigests, startKeyIndexForDigests)
			require.Equal(t, tc.expectedDigests, digests)
		})
	}
}

package environment_test

import (
	"bytes"
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
)

func TestAccountStatus(t *testing.T) {

	s := environment.NewAccountStatus()

	t.Run("test setting values", func(t *testing.T) {
		index := atree.SlabIndex{1, 2, 3, 4, 5, 6, 7, 8}
		s.SetStorageIndex(index)
		s.SetAccountPublicKeyCount(34)
		s.SetStorageUsed(56)
		s.SetAccountIdCounter(78)

		require.Equal(t, uint64(56), s.StorageUsed())
		returnedIndex := s.SlabIndex()
		require.True(t, bytes.Equal(index[:], returnedIndex[:]))
		require.Equal(t, uint32(34), s.AccountPublicKeyCount())
		require.Equal(t, uint64(78), s.AccountIdCounter())

	})

	t.Run("test serialization", func(t *testing.T) {
		b := append([]byte(nil), s.ToBytes()...)
		clone, err := environment.AccountStatusFromBytes(b)
		require.NoError(t, err)
		require.Equal(t, s.SlabIndex(), clone.SlabIndex())
		require.Equal(t, s.AccountPublicKeyCount(), clone.AccountPublicKeyCount())
		require.Equal(t, s.StorageUsed(), clone.StorageUsed())
		require.Equal(t, s.AccountIdCounter(), clone.AccountIdCounter())

		// invalid size bytes
		_, err = environment.AccountStatusFromBytes([]byte{1, 2})
		require.Error(t, err)
	})
}

func TestAccountStatusV4AppendAndGetKeyMetadata(t *testing.T) {

	newAccountStatusBytes := []byte{
		0x40,                   // version + flag
		0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
		0, 0, 0, 0, // value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
	}

	accountStatusWithOneKeyBytes := []byte{
		0x40,                   // version + flag
		0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
		0, 0, 0, 1, // value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
	}

	accountStatusWithTwoKeyBytes := []byte{
		0x40,                   // version + flag
		0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
		0, 0, 0, 2, // value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
		// key metadata
		0, 0, 0, 4, // length prefix for weight and revoked list
		0, 1, 3, 0xe8, // weight and revoked group
		0, 0, 0, 0, // start index for digests
		0, 0, 0, 0x10, // length prefix for digests
		0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
		0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
	}

	accountStatusWithOneKeyAndOneDuplicateKeyBytes := []byte{
		0x41,                   // version + flag
		0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
		0, 0, 0, 2, // value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
		// key metadata
		0, 0, 0, 4, // length prefix for weight and revoked list
		0, 1, 3, 0xe8, // weight and revoked group
		0, 0, 0, 1, // start index for mapping
		0, 0, 0, 6, // length prefix for mapping
		0, 1, 0, 0, 0, 0, // mapping group 1
		0, 0, 0, 0, // start index for digests
		0, 0, 0, 8, // length prefix for digests
		0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
	}

	t.Run("new account status", func(t *testing.T) {
		s := environment.NewAccountStatus()
		require.Equal(t, uint8(4), s.Version())
		require.False(t, s.IsAccountKeyDeduplicated())
		require.Equal(t, uint32(0), s.AccountPublicKeyCount())

		require.Equal(t, newAccountStatusBytes, s.ToBytes())

		decoded, err := environment.AccountStatusFromBytes(newAccountStatusBytes)
		require.NoError(t, err)
		require.Equal(t, s, decoded)
	})

	type keyMetadata struct {
		revoked        bool
		weight         uint16
		storedKeyIndex uint32
	}

	testcases := []struct {
		name                      string
		accountStatusData         []byte
		revoked                   bool
		weight                    uint16
		encodedKey                []byte
		getKeyDigest              func([]byte) uint64
		getStoredKey              func(uint32) ([]byte, error)
		expectedStoredKeyIndex    uint32
		expectedSaveKey           bool
		expectedPublicKeyCount    uint32
		expectedDeduplication     bool
		expectedAccountStatusData []byte
		keyMetadata               map[uint32]keyMetadata
	}{
		{
			name:              "append key_0",
			accountStatusData: newAccountStatusBytes,
			revoked:           false,
			weight:            1000,
			encodedKey:        []byte{1},
			getKeyDigest: func([]byte) uint64 {
				require.Fail(t, "getKeyDigest shouldn't be called when appending first key")
				return 0
			},
			getStoredKey: func(uint32) ([]byte, error) {
				require.Fail(t, "getStoredKey shouldn't be called when appending first key")
				return nil, nil
			},
			expectedStoredKeyIndex:    uint32(0),
			expectedSaveKey:           true,
			expectedPublicKeyCount:    uint32(1),
			expectedDeduplication:     false,
			expectedAccountStatusData: accountStatusWithOneKeyBytes,
		},
		{
			name:              "append key_1 to {key_0}",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 1
				}
				if bytes.Equal(encodedKey, []byte{2}) {
					return 2
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex:    uint32(1),
			expectedSaveKey:           true,
			expectedPublicKeyCount:    uint32(2),
			expectedDeduplication:     false,
			expectedAccountStatusData: accountStatusWithTwoKeyBytes,
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name:              "append key_0 to {key_0}",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{1},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 1
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex:    uint32(0),
			expectedSaveKey:           false,
			expectedPublicKeyCount:    uint32(2),
			expectedDeduplication:     true,
			expectedAccountStatusData: accountStatusWithOneKeyAndOneDuplicateKeyBytes,
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(0),
				},
			},
		},
		{
			name:              "append key_0 to {key_0, key_1}",
			accountStatusData: accountStatusWithTwoKeyBytes,
			revoked:           false,
			weight:            uint16(1),
			encodedKey:        []byte{1},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 1
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(0),
			expectedSaveKey:        false,
			expectedPublicKeyCount: uint32(3),
			expectedDeduplication:  true,
			expectedAccountStatusData: []byte{
				0x41,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0, 1, // weight and revoked group
				0, 0, 0, 2, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
				2: {
					revoked:        false,
					weight:         uint16(1),
					storedKeyIndex: uint32(0),
				},
			},
		},
		{
			name:              "append key_1 to {key_0, key_0}",
			accountStatusData: accountStatusWithOneKeyAndOneDuplicateKeyBytes,
			revoked:           false,
			weight:            uint16(1),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{2}) {
					return 2
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(1),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(3),
			expectedDeduplication:  true,
			expectedAccountStatusData: []byte{
				0x41,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0, 1, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0x80, 2, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(0),
				},
				2: {
					revoked:        false,
					weight:         uint16(1),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name:              "append key_1 to {key_0}, key_0 has empty hash",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 0
				}
				if bytes.Equal(encodedKey, []byte{2}) {
					return 2
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(1),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(2),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 0, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name:              "append key_1 to {key_0}, key_1 has empty hash",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 1
				}
				if bytes.Equal(encodedKey, []byte{2}) {
					return 0
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(1),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(2),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name:              "append key_1 to {key_0}, key_0 and key_1 have empty hash",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 0
				}
				if bytes.Equal(encodedKey, []byte{2}) {
					return 0
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(1),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(2),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 0, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name:              "append key_1 to {key_0}, key_0 and key_1 have hash collision",
			accountStatusData: accountStatusWithOneKeyBytes,
			revoked:           false,
			weight:            uint16(1000),
			encodedKey:        []byte{2},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{1}) {
					return 1
				}
				if bytes.Equal(encodedKey, []byte{2}) {
					return 1
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 0 {
					return []byte{1}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(1),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(2),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_1 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
			},
		},
		{
			name: "append key_2 to {key_0, key_1}, key_1 has empty hash",
			accountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_1 digest
			},
			revoked:    false,
			weight:     uint16(1),
			encodedKey: []byte{3},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{3}) {
					return 3
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(2),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(3),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0, 1, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 0, // key_1 digest
				0, 0, 0, 0, 0, 0, 0, 3, // key_2 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
				2: {
					revoked:        false,
					weight:         uint16(1),
					storedKeyIndex: uint32(2),
				},
			},
		},
		{
			name: "append key_2 to {key_0, key_1}, key_2 has empty hash",
			accountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			revoked:    false,
			weight:     uint16(1),
			encodedKey: []byte{3},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{3}) {
					return 0
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(2),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(3),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0, 1, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_2 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
				2: {
					revoked:        false,
					weight:         uint16(1),
					storedKeyIndex: uint32(2),
				},
			},
		},
		{
			name: "append key_2 to {key_0, key_1}, key_1 and key_2 have hash collision",
			accountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // key_0 digest
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
			},
			revoked:    false,
			weight:     uint16(1),
			encodedKey: []byte{3},
			getKeyDigest: func(encodedKey []byte) uint64 {
				if bytes.Equal(encodedKey, []byte{3}) {
					return 2
				}
				require.Fail(t, "getKeyDigest(%x) isn't expected", encodedKey)
				return 0
			},
			getStoredKey: func(keyIndex uint32) ([]byte, error) {
				if keyIndex == 1 {
					return []byte{2}, nil
				}
				require.Fail(t, "getStoredKey(%d) isn't expected", keyIndex)
				return nil, nil
			},
			expectedStoredKeyIndex: uint32(2),
			expectedSaveKey:        true,
			expectedPublicKeyCount: uint32(3),
			expectedDeduplication:  false,
			expectedAccountStatusData: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 1, 0, 1, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // key_1 digest
				0, 0, 0, 0, 0, 0, 0, 0, // key_2 digest
			},
			keyMetadata: map[uint32]keyMetadata{
				1: {
					revoked:        false,
					weight:         uint16(1000),
					storedKeyIndex: uint32(1),
				},
				2: {
					revoked:        false,
					weight:         uint16(1),
					storedKeyIndex: uint32(2),
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := environment.AccountStatusFromBytes(tc.accountStatusData)
			require.NoError(t, err)

			storedKeyIndex, saveKey, err := s.AppendAccountPublicKeyMetadata(
				tc.revoked,
				tc.weight,
				tc.encodedKey,
				tc.getKeyDigest,
				tc.getStoredKey,
			)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStoredKeyIndex, storedKeyIndex)
			require.Equal(t, tc.expectedSaveKey, saveKey)
			require.Equal(t, tc.expectedPublicKeyCount, s.AccountPublicKeyCount())
			require.Equal(t, tc.expectedDeduplication, s.IsAccountKeyDeduplicated())

			require.Equal(t, tc.expectedAccountStatusData, s.ToBytes())

			decoded, err := environment.AccountStatusFromBytes(tc.expectedAccountStatusData)
			require.NoError(t, err)
			require.Equal(t, s, decoded)
			require.Equal(t, tc.expectedPublicKeyCount, s.AccountPublicKeyCount())
			require.Equal(t, tc.expectedDeduplication, s.IsAccountKeyDeduplicated())

			for keyIndex, expected := range tc.keyMetadata {
				// Get revoked status
				revoked, err := s.AccountPublicKeyRevokedStatus(keyIndex)
				require.NoError(t, err)
				require.Equal(t, expected.revoked, revoked)

				// Get key metadata
				weight, revoked, storedKeyIndex, err := s.AccountPublicKeyMetadata(keyIndex)
				require.NoError(t, err)
				require.Equal(t, expected.revoked, revoked)
				require.Equal(t, expected.weight, weight)
				require.Equal(t, expected.storedKeyIndex, storedKeyIndex)
			}
		})
	}
}

func TestAccountStatusV4RevokeKey(t *testing.T) {

	testcases := []struct {
		name                  string
		data                  []byte
		keyIndexToRevoke      uint32
		expected              []byte
		expectedRevokedStatus map[uint32]bool
	}{
		{
			name: "revoke only key in a revoked group of size 1",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 2, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
			expectedRevokedStatus: map[uint32]bool{
				1: true,
			},
		},
		{
			name: "revoke first key in a revoked group of size 2 (no prev group)",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: true,
				2: false,
			},
		},
		{
			name: "revoke second key in a revoked group of size 2 (no next group)",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 2,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 2
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
		{
			name: "revoke first key in a revoke group of size 1 (no prev group)",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
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
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 0x80, 0x01, // weight and revoked group 2
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: true,
				2: true,
			},
		},
		{
			name: "revoke first key in a revoke group of size 1 (has prev group)",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
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
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 3, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
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
		{
			name: "revoke first key in a revoke group of size 3 (no prev group)",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 1,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 8, // length prefix for weight and revoked list
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: true,
				2: false,
				3: false,
			},
		},
		{
			name: "revoke second key in a revoke group of size 3",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 2,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 0x0c, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 1, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: false,
				2: true,
				3: false,
			},
		},
		{
			name: "revoke third key in a revoke group of size 3",
			data: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 3, 3, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			keyIndexToRevoke: 3,
			expected: []byte{
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // value for storage index
				0, 0, 0, 4, // value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // value for address id counter
				// key metadata
				0, 0, 0, 0x8, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group 1
				0, 1, 0x83, 0xe8, // weight and revoked group 1
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
			expectedRevokedStatus: map[uint32]bool{
				1: false,
				2: false,
				3: true,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := environment.AccountStatusFromBytes(tc.data)
			require.NoError(t, err)

			err = s.RevokeAccountPublicKey(tc.keyIndexToRevoke)
			require.NoError(t, err)

			require.Equal(t, tc.expected, s.ToBytes())

			decoded, err := environment.AccountStatusFromBytes(tc.expected)
			require.NoError(t, err)
			require.Equal(t, s, decoded)

			for keyIndex, expected := range tc.expectedRevokedStatus {
				revoked, err := decoded.AccountPublicKeyRevokedStatus(keyIndex)
				require.NoError(t, err)
				require.Equal(t, expected, revoked)
			}
		})
	}
}

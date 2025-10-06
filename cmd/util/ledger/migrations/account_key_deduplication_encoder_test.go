package migrations

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
)

func TestAccountPublicKeyWeightsAndRevokedStatusSerizliation(t *testing.T) {
	testcases := []struct {
		name     string
		status   []accountPublicKeyWeightAndRevokedStatus
		expected []byte
	}{
		{
			name:     "empty",
			status:   nil,
			expected: nil,
		},
		{
			name: "one status",
			status: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8},
		},
		{
			name: "multiple identical status",
			status: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
		},
		{
			name: "different status",
			status: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 2, revoked: false},
				{weight: 2, revoked: true},
			},
			expected: []byte{
				0, 1, 0, 1,
				0, 1, 0, 2,
				0, 1, 0x80, 2,
			},
		},
		{
			name: "different status",
			status: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 2, revoked: true},
			},
			expected: []byte{
				0, 2, 0, 1,
				0, 1, 0x80, 2,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := encodeAccountPublicKeyWeightsAndRevokedStatus(tc.status)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			decodedStatus, err := decodeAccountPublicKeyWeightAndRevokedStatusGroups(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.status, decodedStatus)
		})
	}

	t.Run("run length around max group count", func(t *testing.T) {
		testcases := []struct {
			name     string
			status   accountPublicKeyWeightAndRevokedStatus
			count    uint32
			expected []byte
		}{
			{
				name:   "run length maxRunLengthInEncodedStatusGroup - 1",
				status: accountPublicKeyWeightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInEncodedStatusGroup - 1,
				expected: []byte{
					0xff, 0xfe, 0x83, 0xe8,
				},
			},
			{
				name:   "run length maxRunLengthInEncodedStatusGroup ",
				status: accountPublicKeyWeightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInEncodedStatusGroup,
				expected: []byte{
					0xff, 0xff, 0x83, 0xe8,
				},
			},
			{
				name:   "run length maxRunLengthInEncodedStatusGroup + 1",
				status: accountPublicKeyWeightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInEncodedStatusGroup + 1,
				expected: []byte{
					0xff, 0xff, 0x83, 0xe8,
					0x00, 0x01, 0x83, 0xe8,
				},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				status := make([]accountPublicKeyWeightAndRevokedStatus, tc.count)
				for i := range len(status) {
					status[i] = tc.status
				}

				b, err := encodeAccountPublicKeyWeightsAndRevokedStatus(status)
				require.NoError(t, err)
				require.Equal(t, tc.expected, b)

				decodedStatus, err := decodeAccountPublicKeyWeightAndRevokedStatusGroups(b)
				require.NoError(t, err)
				require.ElementsMatch(t, status, decodedStatus)
			})
		}
	})
}

func TestMappingGroupSerialization(t *testing.T) {
	testcases := []struct {
		name     string
		mappings []uint32
		expected []byte
	}{
		{
			name:     "1 group with run length 1",
			mappings: []uint32{1},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
			},
		},
		{
			name:     "2 groups with different run length",
			mappings: []uint32{1, 1, 2},
			expected: []byte{
				0, 2, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 2,
			},
		},
		{
			name:     "consecutive group count followed by regular group",
			mappings: []uint32{1, 2, 2},
			expected: []byte{
				0x80, 2, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 2,
			},
		},
		{
			name:     "group value not consecutive",
			mappings: []uint32{1, 3},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 3,
			},
		},
		{
			name:     "consecutive group with run length 2",
			mappings: []uint32{1, 2},
			expected: []byte{
				0x80, 2, 0, 0, 0, 1,
			},
		},
		{
			name:     "consecutive group with run length 3",
			mappings: []uint32{1, 2, 3},
			expected: []byte{
				0x80, 3, 0, 0, 0, 1,
			},
		},
		{
			name:     "consecutive group followed by non-consecutive group",
			mappings: []uint32{1, 2, 2},
			expected: []byte{
				0x80, 2, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 2,
			},
		},
		{
			name:     "consecutive group followed by consecutive group",
			mappings: []uint32{1, 2, 2, 3},
			expected: []byte{
				0x80, 2, 0, 0, 0, 1,
				0x80, 2, 0, 0, 0, 2,
			},
		},
		{
			name:     "consecutive groups mixed with non-consecutive groups",
			mappings: []uint32{1, 3, 4, 5, 5, 5, 5, 6, 7, 7},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
				0x80, 3, 0, 0, 0, 3,
				0, 3, 0, 0, 0, 5,
				0x80, 2, 0, 0, 0, 6,
				0, 1, 0, 0, 0, 7,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := encodeAccountPublicKeyMapping(tc.mappings)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			decodedMappings, err := decodeAccountPublicKeyMapping(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.mappings, decodedMappings)
		})
	}
}

func TestDigestListSerialization(t *testing.T) {
	testcases := []struct {
		name     string
		digests  []uint64
		expected []byte
	}{
		{
			name:     "empty",
			digests:  nil,
			expected: nil,
		},
		{
			name:    "1 digest",
			digests: []uint64{1},
			expected: []byte{
				0, 0, 0, 0, 0, 0, 0, 1,
			},
		},
		{
			name:    "2 digests",
			digests: []uint64{1, 2},
			expected: []byte{
				0, 0, 0, 0, 0, 0, 0, 1,
				0, 0, 0, 0, 0, 0, 0, 2,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			b := encodeDigestList(tc.digests)
			require.Equal(t, tc.expected, b)

			decodedDigests, err := decodeDigestList(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.digests, decodedDigests)
		})
	}
}

func TestBatchPublicKeySerialization(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		b, err := encodePublicKeysInBatches(nil, maxPublicKeyCountInBatch)
		require.NoError(t, err)
		require.Empty(t, b)

		decodedPublicKeys, err := decodeBatchPublicKey(nil)
		require.NoError(t, err)
		require.Empty(t, decodedPublicKeys)
	})

	t.Run("1 public key", func(t *testing.T) {
		encodedPublicKey := make([]byte, 73)
		_, _ = rand.Read(encodedPublicKey)

		b, err := encodePublicKeysInBatches([][]byte{encodedPublicKey}, maxPublicKeyCountInBatch)
		require.NoError(t, err)
		require.Empty(t, b)
	})

	t.Run("2 public key", func(t *testing.T) {
		encodedPublicKey1 := make([]byte, 73)
		_, _ = rand.Read(encodedPublicKey1)

		encodedPublicKey2 := make([]byte, 80)
		_, _ = rand.Read(encodedPublicKey2)

		encodedPublicKeys := [][]byte{encodedPublicKey1, encodedPublicKey2}

		b, err := encodePublicKeysInBatches(encodedPublicKeys, maxPublicKeyCountInBatch)
		require.NoError(t, err)
		require.True(t, len(b) == 1)

		decodedPublicKeys, err := decodeBatchPublicKey(b[0])
		require.NoError(t, err)
		require.True(t, len(decodedPublicKeys) == 2)
		require.Empty(t, decodedPublicKeys[0])
		require.Equal(t, encodedPublicKey2, decodedPublicKeys[1])
	})

	t.Run("2 batches of public key", func(t *testing.T) {
		encodedPublicKeys := make([][]byte, maxPublicKeyCountInBatch*1.5)

		for i := range len(encodedPublicKeys) {
			encodedPublicKeys[i] = make([]byte, 70+i)
			_, _ = rand.Read(encodedPublicKeys[i])
		}

		b, err := encodePublicKeysInBatches(encodedPublicKeys, maxPublicKeyCountInBatch)
		require.NoError(t, err)
		require.True(t, len(b) == 2)

		// Decode first batch
		decodedPublicKeys, err := decodeBatchPublicKey(b[0])
		require.NoError(t, err)
		require.True(t, len(decodedPublicKeys) == maxPublicKeyCountInBatch)
		require.Empty(t, decodedPublicKeys[0])
		for i := 1; i < maxPublicKeyCountInBatch; i++ {
			require.Equal(t, encodedPublicKeys[i], decodedPublicKeys[i])
		}

		// Decode second batch
		decodedPublicKeys, err = decodeBatchPublicKey(b[1])
		require.NoError(t, err)
		require.True(t, len(decodedPublicKeys) == len(encodedPublicKeys)-maxPublicKeyCountInBatch)
		for i := range len(decodedPublicKeys) {
			require.Equal(t, encodedPublicKeys[i+maxPublicKeyCountInBatch], decodedPublicKeys[i])
		}
	})
}

func TestAccountStatusV4Serialization(t *testing.T) {
	// NOTE: account status only contains key metadata
	// if there are at least 2 account public keys.

	testcases := []struct {
		name                     string
		deduplicated             bool
		accountPublicKeyCount    uint32
		weightAndRevokedStatus   []accountPublicKeyWeightAndRevokedStatus
		startIndexForDigests     uint32
		digests                  []uint64
		startIndexForMappings    uint32
		accountPublicKeyMappings []uint32
		expected                 []byte
	}{
		{
			name:                  "not deduplicated with 2 account public key",
			deduplicated:          false,
			accountPublicKeyCount: uint32(2),
			weightAndRevokedStatus: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			startIndexForDigests: uint32(0),
			digests:              []uint64{1, 2},
			expected: []byte{
				// Required Fields
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
				0, 0, 0, 2, // init value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
				// Optional Fields
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
			},
		},
		{
			name:                  "not deduplicated with 3 account public key",
			deduplicated:          false,
			accountPublicKeyCount: uint32(3),
			weightAndRevokedStatus: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			startIndexForDigests: uint32(1),
			digests:              []uint64{2, 3},
			expected: []byte{
				// Required Fields
				0x40,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
				0, 0, 0, 3, // init value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
				// Optional Fields
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 2, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for digests
				0, 0, 0, 0x10, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 2, // digest 2
				0, 0, 0, 0, 0, 0, 0, 3, // digest 3
			},
		},
		{
			name:                  "deduplicated with 2 account public key (1 stored key)",
			deduplicated:          true,
			accountPublicKeyCount: uint32(2),
			weightAndRevokedStatus: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			startIndexForDigests:     uint32(0),
			digests:                  []uint64{1},
			startIndexForMappings:    uint32(1),
			accountPublicKeyMappings: []uint32{0},
			expected: []byte{
				// Required Fields
				0x41,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
				0, 0, 0, 2, // init value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
				// Optional Fields
				0, 0, 0, 4, // length prefix for weight and revoked list
				0, 1, 3, 0xe8, // weight and revoked group
				0, 0, 0, 1, // start index for mapping
				0, 0, 0, 6, // length prefix for mapping
				0, 1, 0, 0, 0, 0, // mapping group 1
				0, 0, 0, 0, // start index for digests
				0, 0, 0, 8, // length prefix for digests
				0, 0, 0, 0, 0, 0, 0, 1, // digest 1
			},
		},
		{
			name:                  "deduplicated with 3 account public key (2 stored keys)",
			deduplicated:          true,
			accountPublicKeyCount: uint32(3),
			weightAndRevokedStatus: []accountPublicKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			startIndexForDigests:     uint32(0),
			digests:                  []uint64{1, 2},
			startIndexForMappings:    uint32(1),
			accountPublicKeyMappings: []uint32{0, 1},
			expected: []byte{
				// Required Fields
				0x41,                   // version + flag
				0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
				0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
				0, 0, 0, 3, // init value for public key counts
				0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
				// Optional Fields
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
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			s := environment.NewAccountStatus()
			s.SetAccountPublicKeyCount(tc.accountPublicKeyCount)

			b, err := encodeAccountStatusV4WithPublicKeyMetadata(
				s.ToBytes(),
				tc.weightAndRevokedStatus,
				tc.startIndexForDigests,
				tc.digests,
				tc.startIndexForMappings,
				tc.accountPublicKeyMappings,
				tc.deduplicated,
			)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			_, decodedWeightAndRevokedStatus, decodedStartIndexForDigests, decodedDigests, decodedStartIndexForMappings, decodedAccountPublicKeyMappings, err := decodeAccountStatusV4(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.weightAndRevokedStatus, decodedWeightAndRevokedStatus)
			require.Equal(t, tc.startIndexForDigests, decodedStartIndexForDigests)
			require.ElementsMatch(t, tc.digests, decodedDigests)
			require.Equal(t, tc.startIndexForMappings, decodedStartIndexForMappings)
			require.ElementsMatch(t, tc.accountPublicKeyMappings, decodedAccountPublicKeyMappings)

			err = validateKeyMetadata(
				tc.deduplicated,
				tc.accountPublicKeyCount,
				decodedWeightAndRevokedStatus,
				decodedStartIndexForDigests,
				decodedDigests,
				decodedStartIndexForMappings,
				decodedAccountPublicKeyMappings)
			require.NoError(t, err)
		})
	}
}

func decodeAccountStatusV4(b []byte) (
	requiredFields []byte,
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForDigests uint32,
	digests []uint64,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
	err error,
) {
	if len(b) < accountStatusV4MinimumSize {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: expect at least %d byte, got %d bytes", accountStatusV4MinimumSize, len(b))
	}

	version, flag := b[0]&versionMask>>4, b[0]&flagMask

	if version != 4 {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: expect version 4, got %d", version)
	}

	if flag != 0 && flag != 1 {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: expect flag 0 or 1, got %d", flag)
	}

	deduplicated := flag == 1

	requiredFields = append([]byte(nil), b[:accountStatusV4MinimumSize]...)
	optionalFields := append([]byte(nil), b[accountStatusV4MinimumSize:]...)

	accountStatus, err := environment.AccountStatusFromBytes(requiredFields)
	if err != nil {
		return nil, nil, 0, nil, 0, nil, err
	}

	accountPublicKeyCount := accountStatus.AccountPublicKeyCount()

	if accountPublicKeyCount <= 1 {
		if len(optionalFields) > 0 {
			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: found optional fields when account public key count is %d", accountPublicKeyCount)
		}

		if deduplicated {
			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to create AccountStatusV4: deduplication flag should be off when account public key is less than 2")
		}

		return requiredFields, nil, 0, nil, 0, nil, err
	}

	weightAndRevokedStatus, startKeyIndexForMapping, accountPublicKeyMappings, startKeyIndexForDigests, digests, err = decodeAccountStatusKeyMetadata(optionalFields, deduplicated)

	return
}

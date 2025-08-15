package migrations

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
)

func TestAccountKeyWeightsAndRevokedStatusSerizliation(t *testing.T) {
	testcases := []struct {
		name     string
		status   []accountKeyWeightAndRevokedStatus
		expected []byte
	}{
		{
			name:     "empty",
			status:   nil,
			expected: nil,
		},
		{
			name: "one status",
			status: []accountKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8},
		},
		{
			name: "multiple identical status",
			status: []accountKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
		},
		{
			name: "different status",
			status: []accountKeyWeightAndRevokedStatus{
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
			status: []accountKeyWeightAndRevokedStatus{
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
			b, err := encodeAccountKeyWeightsAndRevokedStatus(tc.status)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			decodedStatus, err := decodeAccountKeyWeightAndRevokedStatusGroups(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.status, decodedStatus)
		})
	}

	t.Run("same status exceeding max group count", func(t *testing.T) {
		count := maxRunLengthInEncodedStatusGroup + 10
		status := make([]accountKeyWeightAndRevokedStatus, count)
		for i := range len(status) {
			status[i] = accountKeyWeightAndRevokedStatus{weight: 1000, revoked: true}
		}

		expected := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x0a, 0x83, 0xe8}

		b, err := encodeAccountKeyWeightsAndRevokedStatus(status)
		require.NoError(t, err)
		require.Equal(t, expected, b)

		decodedStatus, err := decodeAccountKeyWeightAndRevokedStatusGroups(b)
		require.NoError(t, err)
		require.ElementsMatch(t, status, decodedStatus)
	})
}

func TestMappingGroupSerialization(t *testing.T) {
	testcases := []struct {
		name     string
		groups   []mappingGroup
		mappings []uint32
		expected []byte
	}{
		{
			name: "1 group",
			groups: []mappingGroup{
				{1, 1},
			},
			mappings: []uint32{1},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
			},
		},
		{
			name: "2 groups",
			groups: []mappingGroup{
				{2, 1},
				{1, 2},
			},
			mappings: []uint32{1, 1, 2},
			expected: []byte{
				0, 2, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 2,
			},
		},
		{
			name: "group count not consecutive",
			groups: []mappingGroup{
				{1, 1},
				{2, 2},
			},
			mappings: []uint32{1, 2, 2},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
				0, 2, 0, 0, 0, 2,
			},
		},
		{
			name: "group value not consecutive",
			groups: []mappingGroup{
				{1, 1},
				{1, 3},
			},
			mappings: []uint32{1, 3},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
				0, 1, 0, 0, 0, 3,
			},
		},
		{
			name: "2 consecutive groups",
			groups: []mappingGroup{
				{1, 1},
				{1, 2},
			},
			mappings: []uint32{1, 2},
			expected: []byte{
				0x80, 2, 0, 0, 0, 1,
			},
		},
		{
			name: ">2 consecutive groups",
			groups: []mappingGroup{
				{1, 1},
				{1, 2},
				{1, 3},
			},
			mappings: []uint32{1, 2, 3},
			expected: []byte{
				0x80, 3, 0, 0, 0, 1,
			},
		},
		{
			name: "consecutive groups mixed with non-consecutive groups",
			groups: []mappingGroup{
				{1, 1},
				{1, 3},
				{1, 4},
				{2, 5},
				{1, 6},
				{1, 7},
			},
			mappings: []uint32{1, 3, 4, 5, 5, 6, 7},
			expected: []byte{
				0, 1, 0, 0, 0, 1,
				0x80, 2, 0, 0, 0, 3,
				0, 2, 0, 0, 0, 5,
				0x80, 2, 0, 0, 0, 6,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := encodeAccountKeyMapping(tc.mappings)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			decodedMappings, err := decodeAccountKeyMapping(b)
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
		b, err := encodePublicKeysInBatches(nil)
		require.NoError(t, err)
		require.Empty(t, b)

		decodedPublicKeys, err := decodeBatchPublicKey(nil)
		require.NoError(t, err)
		require.Empty(t, decodedPublicKeys)
	})

	t.Run("1 public key", func(t *testing.T) {
		encodedPublicKey := make([]byte, 73)
		_, _ = rand.Read(encodedPublicKey)

		b, err := encodePublicKeysInBatches([][]byte{encodedPublicKey})
		require.NoError(t, err)
		require.Empty(t, b)
	})

	t.Run("2 public key", func(t *testing.T) {
		encodedPublicKey1 := make([]byte, 73)
		_, _ = rand.Read(encodedPublicKey1)

		encodedPublicKey2 := make([]byte, 80)
		_, _ = rand.Read(encodedPublicKey2)

		encodedPublicKeys := [][]byte{encodedPublicKey1, encodedPublicKey2}

		b, err := encodePublicKeysInBatches(encodedPublicKeys)
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

		b, err := encodePublicKeysInBatches(encodedPublicKeys)
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
	// if there are at least 2 account keys.

	testcases := []struct {
		name                   string
		deduplicated           bool
		accountKeyCount        uint32
		weightAndRevokedStatus []accountKeyWeightAndRevokedStatus
		startIndexForDigests   uint32
		digests                []uint64
		startIndexForMappings  uint32
		accountKeyMappings     []uint32
		expected               []byte
	}{
		{
			name:            "not deduplicated with 2 account key",
			deduplicated:    false,
			accountKeyCount: uint32(2),
			weightAndRevokedStatus: []accountKeyWeightAndRevokedStatus{
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
			name:            "not deduplicated with 3 account key",
			deduplicated:    false,
			accountKeyCount: uint32(3),
			weightAndRevokedStatus: []accountKeyWeightAndRevokedStatus{
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
			name:            "deduplicated with 2 account key (1 stored key)",
			deduplicated:    true,
			accountKeyCount: uint32(2),
			weightAndRevokedStatus: []accountKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			startIndexForDigests:  uint32(0),
			digests:               []uint64{1},
			startIndexForMappings: uint32(1),
			accountKeyMappings:    []uint32{0},
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
			name:            "deduplicated with 3 account key (2 stored keys)",
			deduplicated:    true,
			accountKeyCount: uint32(3),
			weightAndRevokedStatus: []accountKeyWeightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			startIndexForDigests:  uint32(0),
			digests:               []uint64{1, 2},
			startIndexForMappings: uint32(1),
			accountKeyMappings:    []uint32{0, 1},
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
			s.SetPublicKeyCount(tc.accountKeyCount)

			b, err := encodeAccountStatusV4WithPublicKeyMetadata(
				s.ToBytes(),
				tc.weightAndRevokedStatus,
				tc.startIndexForDigests,
				tc.digests,
				tc.startIndexForMappings,
				tc.accountKeyMappings,
				tc.deduplicated,
			)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			_, decodedWeightAndRevokedStatus, decodedStartIndexForDigests, decodedDigests, decodedStartIndexForMappings, decodedAccountKeyMappings, err := decodeAccountStatusV4(b)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.weightAndRevokedStatus, decodedWeightAndRevokedStatus)
			require.Equal(t, tc.startIndexForDigests, decodedStartIndexForDigests)
			require.ElementsMatch(t, tc.digests, decodedDigests)
			require.Equal(t, tc.startIndexForMappings, decodedStartIndexForMappings)
			require.ElementsMatch(t, tc.accountKeyMappings, decodedAccountKeyMappings)

			err = validateKeyMetadata(
				tc.deduplicated,
				tc.accountKeyCount,
				decodedWeightAndRevokedStatus,
				decodedStartIndexForDigests,
				decodedDigests,
				decodedStartIndexForMappings,
				decodedAccountKeyMappings)
			require.NoError(t, err)
		})
	}
}

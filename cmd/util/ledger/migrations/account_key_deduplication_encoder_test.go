package migrations

import (
	"crypto/rand"
	"encoding/binary"
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

func decodeAccountPublicKeyWeightAndRevokedStatusGroups(b []byte) ([]accountPublicKeyWeightAndRevokedStatus, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode weight and revoked status: expect multiple of %d bytes, got %d", weightAndRevokedStatusGroupSize, len(b))
	}

	statuses := make([]accountPublicKeyWeightAndRevokedStatus, 0, len(b)/weightAndRevokedStatusGroupSize)

	for i := 0; i < len(b); i += weightAndRevokedStatusGroupSize {
		runLength := uint32(binary.BigEndian.Uint16(b[i:]))
		weightAndRevoked := binary.BigEndian.Uint16(b[i+2 : i+4])

		status := accountPublicKeyWeightAndRevokedStatus{
			weight:  weightAndRevoked & weightMask,
			revoked: (weightAndRevoked & revokedMask) > 0,
		}

		for range runLength {
			statuses = append(statuses, status)
		}
	}

	return statuses, nil
}

func decodeAccountPublicKeyMapping(b []byte) ([]uint32, error) {
	if len(b)%mappingGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode mappings: expect multiple of %d bytes, got %d", mappingGroupSize, len(b))
	}

	mapping := make([]uint32, 0, len(b)/mappingGroupSize)

	for i := 0; i < len(b); i += mappingGroupSize {
		runLength := binary.BigEndian.Uint16(b[i:])
		storedKeyIndex := binary.BigEndian.Uint32(b[i+runLengthSize:])

		if highBit := (runLength & consecutiveGroupFlagMask) >> 15; highBit == 1 {
			runLength &= lengthMask

			for i := range runLength {
				mapping = append(mapping, storedKeyIndex+uint32(i))
			}
		} else {
			for range runLength {
				mapping = append(mapping, storedKeyIndex)
			}
		}
	}

	return mapping, nil
}

func decodeDigestList(b []byte) ([]uint64, error) {
	if len(b)%digestSize != 0 {
		return nil, fmt.Errorf("failed to decode digest list: expect multiple of %d byte, got %d", digestSize, len(b))
	}

	storedDigestCount := len(b) / digestSize

	digests := make([]uint64, 0, storedDigestCount)

	for i := 0; i < len(b); i += digestSize {
		digests = append(digests, binary.BigEndian.Uint64(b[i:]))
	}

	return digests, nil
}

func decodeBatchPublicKey(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}

	encodedPublicKeys := make([][]byte, 0, maxPublicKeyCountInBatch)

	off := 0
	for off < len(b) {
		size := int(b[off])
		off++

		if off+size > len(b) {
			return nil, fmt.Errorf("failed to decode batch public key: off %d + size %d out of bounds %d: %x", off, size, len(b), b)
		}

		encodedPublicKey := b[off : off+size]
		off += size

		encodedPublicKeys = append(encodedPublicKeys, encodedPublicKey)
	}

	if off != len(b) {
		return nil, fmt.Errorf("failed to decode batch public key: trailing data (%d bytes): %x", len(b)-off, b)
	}

	return encodedPublicKeys, nil
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

	// Decode weight and revoked list

	var weightAndRevokedGroupsData []byte
	weightAndRevokedGroupsData, optionalFields, err = parseNextLengthPrefixedData(optionalFields)
	if err != nil {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: %w", err)
	}

	weightAndRevokedStatus, err = decodeAccountPublicKeyWeightAndRevokedStatusGroups(weightAndRevokedGroupsData)
	if err != nil {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode weight and revoked status list: %w", err)
	}

	// Decode account public key mapping if deduplication is on

	if deduplicated {
		if len(optionalFields) < 4 {
			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start key index for mapping, got %d bytes", len(optionalFields))
		}

		startKeyIndexForMapping = binary.BigEndian.Uint32(optionalFields)

		optionalFields = optionalFields[4:]

		var mappingData []byte
		mappingData, optionalFields, err = parseNextLengthPrefixedData(optionalFields)
		if err != nil {
			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: %w", err)
		}

		accountPublicKeyMappings, err = decodeAccountPublicKeyMapping(mappingData)
		if err != nil {
			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode account public key mappings: %w", err)
		}
	}

	// Decode digests list

	if len(optionalFields) < 4 {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start stored key index for digests, got %d bytes", len(optionalFields))
	}

	startKeyIndexForDigests = binary.BigEndian.Uint32(optionalFields)
	optionalFields = optionalFields[4:]

	var digestsData []byte
	digestsData, optionalFields, err = parseNextLengthPrefixedData(optionalFields)
	if err != nil {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: %w", err)
	}

	digests, err = decodeDigestList(digestsData)
	if err != nil {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode digests: %w", err)
	}

	// Check trailing data

	if len(optionalFields) != 0 {
		return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to decode AccountStatusV4: got %d extra bytes", len(optionalFields))
	}

	return
}

func parseNextLengthPrefixedData(b []byte) (next []byte, rest []byte, err error) {
	if len(b) < lengthPrefixSize {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least 4 bytes, got %d bytes", len(b))
	}

	length := binary.BigEndian.Uint32(b[:lengthPrefixSize])

	if len(b) < lengthPrefixSize+int(length) {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least %d bytes, got %d bytes", lengthPrefixSize+int(length), len(b))
	}

	b = b[lengthPrefixSize:]
	return b[:length], b[length:], nil
}

func validateKeyMetadata(
	deduplicated bool,
	accountPublicKeyCount uint32,
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForDigests uint32,
	digests []uint64,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
) error {
	if len(weightAndRevokedStatus) != int(accountPublicKeyCount)-1 {
		return fmt.Errorf("found %d weight and revoked status, expect %d", len(weightAndRevokedStatus), accountPublicKeyCount-1)
	}

	if len(digests) > maxStoredDigests {
		return fmt.Errorf("found %d digests, expect max %d digests", len(digests), maxStoredDigests)
	}

	if len(digests) > int(accountPublicKeyCount) {
		return fmt.Errorf("found %d digest, expect fewer digests than account public key count %d", len(digests), accountPublicKeyCount)
	}

	if int(startKeyIndexForDigests)+len(digests) > int(accountPublicKeyCount) {
		return fmt.Errorf("found %d digest at start index %d, expect fewer digests than account public key count %d", len(digests), startKeyIndexForDigests, accountPublicKeyCount)
	}

	if deduplicated {
		if int(startKeyIndexForMapping)+len(accountPublicKeyMappings) != int(accountPublicKeyCount) {
			return fmt.Errorf("found %d mappings at start index %d, expect %d",
				len(accountPublicKeyMappings),
				startKeyIndexForMapping,
				accountPublicKeyCount,
			)
		}
	} else {
		if len(accountPublicKeyMappings) > 0 {
			return fmt.Errorf("found %d account public key mappings for non-deduplicated account, expect 0", len(accountPublicKeyMappings))
		}
	}

	return nil
}

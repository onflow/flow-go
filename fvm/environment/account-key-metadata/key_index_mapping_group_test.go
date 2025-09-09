package accountkeymetadata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
)

func TestAppendAndGetStoredKeyIndexFromMapping(t *testing.T) {
	t.Run("get from empty data", func(t *testing.T) {
		_, err := getStoredKeyIndexFromMappings(nil, 0)
		require.True(t, errors.IsKeyMetadataNotFoundError(err))
	})

	t.Run("get from truncated data", func(t *testing.T) {
		b := []byte{1}

		_, err := getStoredKeyIndexFromMappings(b, 1)
		require.True(t, errors.IsKeyMetadataDecodingError(err))
	})

	t.Run("append to truncated data", func(t *testing.T) {
		b := []byte{1}

		_, err := appendStoredKeyIndexToMappings(b, 1)
		require.True(t, errors.IsKeyMetadataDecodingError(err))
	})

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
			var b []byte
			var err error

			for _, storedKeyIndex := range tc.mappings {
				b, err = appendStoredKeyIndexToMappings(b, storedKeyIndex)
				require.NoError(t, err)
			}
			require.Equal(t, tc.expected, b)

			for keyIndex, expectedStoredKeyIndex := range tc.mappings {
				storedKeyIndex, err := getStoredKeyIndexFromMappings(b, uint32(keyIndex))
				require.NoError(t, err)
				require.Equal(t, expectedStoredKeyIndex, storedKeyIndex)
			}
		})
	}

	t.Run("run length around max group count", func(t *testing.T) {
		testcases := []struct {
			name                    string
			encodedExistingMappings []byte
			mapping                 uint32
			expected                []byte
			expectedCount           uint32
			expectedMapping         uint32
			expectedStartMapping    uint32
			isConsecutiveGroup      bool
		}{
			{
				name: "regular group, run length maxRunLengthInMappingGroup - 1",
				encodedExistingMappings: []byte{
					0x7f, 0xfe, 0x00, 0x00, 0x00, 0x01,
				},
				mapping: 1,
				expected: []byte{
					0x7f, 0xff, 0x00, 0x00, 0x00, 0x01,
				},
				expectedCount:   maxRunLengthInMappingGroup,
				expectedMapping: 1,
			},
			{
				name: "regular group, run length maxRunLengthInMappingGroup",
				encodedExistingMappings: []byte{
					0x7f, 0xff, 0x00, 0x00, 0x00, 0x01,
				},
				mapping: 1,
				expected: []byte{
					0x7f, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
				},
				expectedCount:   maxRunLengthInMappingGroup + 1,
				expectedMapping: 1,
			},
			{
				name: "regular group, run length maxRunLengthInMappingGroup + 1",
				encodedExistingMappings: []byte{
					0x7f, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
				},
				mapping: 1,
				expected: []byte{
					0x7f, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x02, 0x00, 0x00, 0x00, 0x01,
				},
				expectedCount:   maxRunLengthInMappingGroup + 2,
				expectedMapping: 1,
			},
			{
				name: "consecutive group, run length maxRunLengthInMappingGroup - 1",
				encodedExistingMappings: []byte{
					0xff, 0xfe, 0x00, 0x00, 0x00, 0x01,
				},
				mapping: maxRunLengthInMappingGroup,
				expected: []byte{
					0xff, 0xff, 0x00, 0x00, 0x00, 0x01,
				},
				expectedCount:        maxRunLengthInMappingGroup,
				expectedStartMapping: 1,
				isConsecutiveGroup:   true,
			},
			{
				name: "consecutive group, run length maxRunLengthInMappingGroup",
				encodedExistingMappings: []byte{
					0xff, 0xff, 0x00, 0x00, 0x00, 0x01,
				},
				mapping: maxRunLengthInMappingGroup + 1,
				expected: []byte{
					0xff, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x01, 0x00, 0x00, 0x80, 0x00,
				},
				expectedCount:        maxRunLengthInMappingGroup + 1,
				expectedStartMapping: 1,
				isConsecutiveGroup:   true,
			},
			{
				name: "consecutive group, run length maxRunLengthInMappingGroup + 1",
				encodedExistingMappings: []byte{
					0xff, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x01, 0x00, 0x00, 0x80, 0x00,
				},
				mapping: maxRunLengthInMappingGroup + 2,
				expected: []byte{
					0xff, 0xff, 0x00, 0x00, 0x00, 0x01,
					0x80, 0x02, 0x00, 0x00, 0x80, 0x00,
				},
				expectedCount:        maxRunLengthInMappingGroup + 2,
				expectedStartMapping: 1,
				isConsecutiveGroup:   true,
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {

				// Encode and append stored key index
				b, err := appendStoredKeyIndexToMappings(tc.encodedExistingMappings, tc.mapping)
				require.NoError(t, err)
				require.Equal(t, tc.expected, b)

				// Get stored key index from mappings
				if tc.isConsecutiveGroup {
					for i := range tc.expectedCount {
						retrievedStoredKeyIndex, err := getStoredKeyIndexFromMappings(b, i)
						require.NoError(t, err)
						require.Equal(t, tc.expectedStartMapping+i, retrievedStoredKeyIndex)
					}
				} else {
					for i := range tc.expectedCount {
						retrievedStoredKeyIndex, err := getStoredKeyIndexFromMappings(b, i)
						require.NoError(t, err)
						require.Equal(t, tc.expectedMapping, retrievedStoredKeyIndex)
					}
				}
			})
		}
	})
}

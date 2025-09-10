package accountkeymetadata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
)

type weightAndRevokedStatus struct {
	weight  uint16
	revoked bool
}

func TestAppendAndGetWeightAndRevokedStatus(t *testing.T) {
	t.Run("get from empty data", func(t *testing.T) {
		_, _, err := getWeightAndRevokedStatus(nil, 0)
		require.True(t, errors.IsKeyMetadataNotFoundError(err))
	})

	t.Run("get from truncated data", func(t *testing.T) {
		b := []byte{1}

		_, _, err := getWeightAndRevokedStatus(b, 1)
		require.True(t, errors.IsKeyMetadataDecodingError(err))
	})

	t.Run("append to truncated data", func(t *testing.T) {
		b := []byte{1}

		_, err := appendWeightAndRevokedStatus(b, false, 1)
		require.True(t, errors.IsKeyMetadataDecodingError(err))
	})

	// Some of the test cases are from migration test TestAccountPublicKeyWeightsAndRevokedStatusSerizliation
	// in cmd/util/ledger/migrations/account_key_deduplication_encoder_test.go
	testcases := []struct {
		name     string
		status   []weightAndRevokedStatus
		expected []byte
	}{
		{
			name: "one group, run length 1",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8},
		},
		{
			name: "one group, run length 1",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 1, 0x83, 0xe8},
		},
		{
			name: "one group, run length 3",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
		},
		{
			name: "three groups, run length 1",
			status: []weightAndRevokedStatus{
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
			name: "three groups, different run length",
			status: []weightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 2, revoked: true},
				{weight: 3, revoked: true},
				{weight: 3, revoked: true},
			},
			expected: []byte{
				0, 2, 0, 1,
				0, 1, 0x80, 2,
				0, 2, 0x80, 3,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var b []byte
			var err error

			// Encode and append status
			for _, s := range tc.status {
				b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
				require.NoError(t, err)
			}
			require.Equal(t, tc.expected, b)

			// Get revoked and weight status
			for i, s := range tc.status {
				revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
				require.NoError(t, err)
				require.Equal(t, s.revoked, revoked)
				require.Equal(t, s.weight, weight)
			}

			_, _, err = getWeightAndRevokedStatus(b, uint32(len(tc.status)))
			require.True(t, errors.IsKeyMetadataNotFoundError(err))
		})
	}

	t.Run("run length around max group count", func(t *testing.T) {
		testcases := []struct {
			name     string
			status   weightAndRevokedStatus
			count    uint32
			expected []byte
		}{
			{
				name:   "run length maxRunLengthInEncodedStatusGroup - 1",
				status: weightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInWeightAndRevokedStatusGroup - 1,
				expected: []byte{
					0xff, 0xfe, 0x83, 0xe8,
				},
			},
			{
				name:   "run length maxRunLengthInEncodedStatusGroup ",
				status: weightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInWeightAndRevokedStatusGroup,
				expected: []byte{
					0xff, 0xff, 0x83, 0xe8,
				},
			},
			{
				name:   "run length maxRunLengthInEncodedStatusGroup + 1",
				status: weightAndRevokedStatus{weight: 1000, revoked: true},
				count:  maxRunLengthInWeightAndRevokedStatusGroup + 1,
				expected: []byte{
					0xff, 0xff, 0x83, 0xe8,
					0x00, 0x01, 0x83, 0xe8,
				},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				status := make([]weightAndRevokedStatus, tc.count)
				for i := range len(status) {
					status[i] = tc.status
				}

				var b []byte
				var err error

				// Encode and append status
				for _, s := range status {
					b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
					require.NoError(t, err)
				}
				require.Equal(t, tc.expected, b)

				// Get revoked and weight status
				for i, s := range status {
					revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
					require.NoError(t, err)
					require.Equal(t, s.revoked, revoked)
					require.Equal(t, s.weight, weight)
				}
			})
		}
	})
}

func TestSetRevokeInWeightAndRevokedStatus(t *testing.T) {
	testcases := []struct {
		name     string
		status   []weightAndRevokedStatus
		expected []byte
		index    uint32
	}{
		{
			name: "revoke in run-length 1 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x83, 0xe8},
			index:    0,
		},
		{
			name: "no-op revoke in run-length 1 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 1, 0x83, 0xe8},
			index:    0,
		},
		{
			name: "revoke first of run-length 2 group (no prev group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x83, 0xe8, 0, 1, 0x03, 0xe8},
			index:    0,
		},
		{
			name: "revoke second of run-length 2 group (no next group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8, 0, 1, 0x83, 0xe8},
			index:    1,
		},
		{
			name: "no-op revoke first of run-length 2 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 2, 0x83, 0xe8},
			index:    0,
		},
		{
			name: "no-op revoke second of run-length 2 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 2, 0x83, 0xe8},
			index:    1,
		},
		{
			name: "revoke first of run-length 3 group (no prev group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x83, 0xe8, 0, 2, 0x03, 0xe8},
			index:    0,
		},
		{
			name: "revoke second of run-length 3 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8, 0, 1, 0x83, 0xe8, 0, 1, 0x03, 0xe8},
			index:    1,
		},
		{
			name: "revoke third of run-length 3 group (no next group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 2, 0x03, 0xe8, 0, 1, 0x83, 0xe8},
			index:    2,
		},
		{
			name: "no-op revoke first of run-length 3 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
			index:    0,
		},
		{
			name: "no-op revoke second of run-length 3 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
			index:    1,
		},
		{
			name: "no-op revoke last of run-length 3 group",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 3, 0x83, 0xe8},
			index:    2,
		},
		{
			name: "revoke first of run-length 2 group (cannot merge with previous group)",
			status: []weightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 1, 0, 1, 0, 1, 0x83, 0xe8, 0, 1, 0x03, 0xe8},
			index:    1,
		},
		{
			name: "revoke first of run-length 2 group (merge with previous group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
			},
			expected: []byte{0, 2, 0x83, 0xe8, 0, 1, 0x03, 0xe8},
			index:    1,
		},
		{
			name: "revoke second of run-length 2 group (cann't merge with next group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1, revoked: false},
			},
			expected: []byte{0, 1, 0x03, 0xe8, 0, 1, 0x83, 0xe8, 0, 1, 0, 1},
			index:    1,
		},
		{
			name: "revoke second of run-length 2 group (merge with next group)",
			status: []weightAndRevokedStatus{
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: true},
			},
			expected: []byte{0, 1, 0x03, 0xe8, 0, 2, 0x83, 0xe8},
			index:    1,
		},
		{
			name: "revoke middle of a large group",
			status: []weightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
			},
			expected: []byte{0, 3, 0, 1, 0, 2, 0x03, 0xe8, 0, 1, 0x83, 0xe8, 0, 1, 0x03, 0xe8, 0, 3, 0, 1},
			index:    5,
		},
		{
			name: "revoke in run-length 1 group (cannot merge with previous and next groups)",
			status: []weightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1000, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
				{weight: 1, revoked: false},
			},
			expected: []byte{0, 3, 0, 1, 0, 1, 0x83, 0xe8, 0, 3, 0, 1},
			index:    3,
		},
		{
			name: "revoke in run-length 1 group (merge with both previous and next groups)",
			status: []weightAndRevokedStatus{
				{weight: 1, revoked: false},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: false},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1000, revoked: true},
				{weight: 1, revoked: false},
			},
			expected: []byte{0, 1, 0, 1, 0, 7, 0x83, 0xe8, 0, 1, 0, 1},
			index:    4,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var b []byte
			var err error

			// Encode and append status
			for _, s := range tc.status {
				b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
				require.NoError(t, err)
			}

			// Revoke at given index
			b, err = setRevokedStatus(b, tc.index)
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)

			// Get revoked and weight status
			for i, s := range tc.status {
				revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
				require.NoError(t, err)
				if uint32(i) == tc.index {
					require.Equal(t, true, revoked)
				} else {
					require.Equal(t, s.revoked, revoked)
				}
				require.Equal(t, s.weight, weight)
			}
		})
	}

	t.Run("can't merge with previous group due to run length limit", func(t *testing.T) {
		status := make([]weightAndRevokedStatus, maxRunLengthInWeightAndRevokedStatusGroup)
		for i := range len(status) {
			status[i] = weightAndRevokedStatus{weight: 1000, revoked: true}
		}
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})

		revokeIndex := uint32(maxRunLengthInWeightAndRevokedStatusGroup)

		expected := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x02, 0x03, 0xe8,
		}

		expectedAfterRevoke := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x01, 0x83, 0xe8,
			0x00, 0x01, 0x03, 0xe8,
		}

		var b []byte
		var err error

		// Encode and append status
		for _, s := range status {
			b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
			require.NoError(t, err)
		}
		require.Equal(t, expected, b)

		// Revoke at given index
		b, err = setRevokedStatus(b, revokeIndex)
		require.NoError(t, err)
		require.Equal(t, expectedAfterRevoke, b)

		// Get revoked and weight status
		for i, s := range status {
			revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
			require.NoError(t, err)
			if uint32(i) == revokeIndex {
				require.Equal(t, true, revoked)
			} else {
				require.Equal(t, s.revoked, revoked)
			}
			require.Equal(t, s.weight, weight)
		}
	})

	t.Run("merge with previous group at run length limit", func(t *testing.T) {
		status := make([]weightAndRevokedStatus, maxRunLengthInWeightAndRevokedStatusGroup-1)
		for i := range len(status) {
			status[i] = weightAndRevokedStatus{weight: 1000, revoked: true}
		}
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})

		revokeIndex := uint32(maxRunLengthInWeightAndRevokedStatusGroup - 1)

		expected := []byte{
			0xff, 0xfe, 0x83, 0xe8,
			0x00, 0x02, 0x03, 0xe8,
		}

		expectedAfterRevoke := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x01, 0x03, 0xe8,
		}

		var b []byte
		var err error

		// Encode and append status
		for _, s := range status {
			b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
			require.NoError(t, err)
		}
		require.Equal(t, expected, b)

		// Revoke at given index
		b, err = setRevokedStatus(b, revokeIndex)
		require.NoError(t, err)
		require.Equal(t, expectedAfterRevoke, b)

		// Get revoked and weight status
		for i, s := range status {
			revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
			require.NoError(t, err)
			if uint32(i) == revokeIndex {
				require.Equal(t, true, revoked)
			} else {
				require.Equal(t, s.revoked, revoked)
			}
			require.Equal(t, s.weight, weight)
		}
	})

	t.Run("partially merge with next group", func(t *testing.T) {
		status := make([]weightAndRevokedStatus, maxRunLengthInWeightAndRevokedStatusGroup+2)
		status[0] = weightAndRevokedStatus{weight: 1000, revoked: false}
		status[1] = weightAndRevokedStatus{weight: 1000, revoked: false}
		for i := 2; i < len(status); i++ {
			status[i] = weightAndRevokedStatus{weight: 1000, revoked: true}
		}

		revokeIndex := uint32(1)

		expected := []byte{
			0x00, 0x02, 0x03, 0xe8,
			0xff, 0xff, 0x83, 0xe8,
		}

		expectedAfterRevoke := []byte{
			0x00, 0x01, 0x03, 0xe8,
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x01, 0x83, 0xe8,
		}

		var b []byte
		var err error

		// Encode and append status
		for _, s := range status {
			b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
			require.NoError(t, err)
		}
		require.Equal(t, expected, b)

		// Revoke at given index
		b, err = setRevokedStatus(b, revokeIndex)
		require.NoError(t, err)
		require.Equal(t, expectedAfterRevoke, b)

		// Get revoked and weight status
		for i, s := range status {
			revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
			require.NoError(t, err)
			if uint32(i) == revokeIndex {
				require.Equal(t, true, revoked)
			} else {
				require.Equal(t, s.revoked, revoked)
			}
			require.Equal(t, s.weight, weight)
		}
	})

	t.Run("cannot merge with previous group due to run length limit, partially merge with next group", func(t *testing.T) {
		status := make([]weightAndRevokedStatus, 0, 2*maxRunLengthInWeightAndRevokedStatusGroup+1)
		for range maxRunLengthInWeightAndRevokedStatusGroup {
			status = append(status, weightAndRevokedStatus{weight: 1000, revoked: true})
		}
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})
		for range maxRunLengthInWeightAndRevokedStatusGroup {
			status = append(status, weightAndRevokedStatus{weight: 1000, revoked: true})
		}

		revokeIndex := uint32(maxRunLengthInWeightAndRevokedStatusGroup)

		expected := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x01, 0x03, 0xe8,
			0xff, 0xff, 0x83, 0xe8,
		}

		expectedAfterRevoke := []byte{
			0xff, 0xff, 0x83, 0xe8,
			0xff, 0xff, 0x83, 0xe8,
			0x00, 0x01, 0x83, 0xe8,
		}

		var b []byte
		var err error

		// Encode and append status
		for _, s := range status {
			b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
			require.NoError(t, err)
		}
		require.Equal(t, expected, b)

		// Revoke at given index
		b, err = setRevokedStatus(b, revokeIndex)
		require.NoError(t, err)
		require.Equal(t, expectedAfterRevoke, b)

		// Get revoked and weight status
		for i, s := range status {
			revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
			require.NoError(t, err)
			if uint32(i) == revokeIndex {
				require.Equal(t, true, revoked)
			} else {
				require.Equal(t, s.revoked, revoked)
			}
			require.Equal(t, s.weight, weight)
		}
	})

	t.Run("merge with previous group and next group", func(t *testing.T) {
		status := make([]weightAndRevokedStatus, maxRunLengthInWeightAndRevokedStatusGroup-15)
		for i := range len(status) {
			status[i] = weightAndRevokedStatus{weight: 1000, revoked: true}
		}
		status = append(status, weightAndRevokedStatus{weight: 1000, revoked: false})
		for range 14 {
			status = append(status, weightAndRevokedStatus{weight: 1000, revoked: true})
		}

		revokeIndex := uint32(maxRunLengthInWeightAndRevokedStatusGroup - 15)

		expected := []byte{
			0xff, 0xf0, 0x83, 0xe8,
			0x00, 0x01, 0x03, 0xe8,
			0x00, 0x0e, 0x83, 0xe8,
		}

		expectedAfterRevoke := []byte{
			0xff, 0xff, 0x83, 0xe8,
		}

		var b []byte
		var err error

		// Encode and append status
		for _, s := range status {
			b, err = appendWeightAndRevokedStatus(b, s.revoked, s.weight)
			require.NoError(t, err)
		}
		require.Equal(t, expected, b)

		// Revoke at given index
		b, err = setRevokedStatus(b, revokeIndex)
		require.NoError(t, err)
		require.Equal(t, expectedAfterRevoke, b)

		// Get revoked and weight status
		for i, s := range status {
			revoked, weight, err := getWeightAndRevokedStatus(b, uint32(i))
			require.NoError(t, err)
			if uint32(i) == revokeIndex {
				require.Equal(t, true, revoked)
			} else {
				require.Equal(t, s.revoked, revoked)
			}
			require.Equal(t, s.weight, weight)
		}
	})
}

package flow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestEqualTo(t *testing.T) {
	testCases := []struct {
		name   string
		vb1    flow.VersionBeacon
		vb2    flow.VersionBeacon
		result bool
	}{
		{
			name: "Equal version beacons",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 1,
			},
			result: true,
		},
		{
			name: "Different sequence",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 2,
			},
			result: false,
		},
		{
			name: "Equal sequence, but invalid version",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
				},
				Sequence: 1,
			},
			result: false,
		},
		{
			name: "Different version boundaries",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.2.0"},
				},
				Sequence: 1,
			},
			result: false,
		},
		{
			name: "Different length of version boundaries",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
					{BlockHeight: 2, Version: "1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "1.0.0"},
				},
				Sequence: 1,
			},
			result: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.result, tc.vb1.EqualTo(&tc.vb2))
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name     string
		vb       *flow.VersionBeacon
		expected bool
	}{
		{
			name: "empty requirements table is invalid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{},
				Sequence:          1,
			},
			expected: false,
		},
		{
			name: "single version required requirement must be valid semver",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v0.21.37"},
				},
				Sequence: 1,
			},
			expected: false,
		},
		{
			name: "ordered by height ascending is valid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "0.21.37"},
					{BlockHeight: 100, Version: "0.21.37"},
					{BlockHeight: 200, Version: "0.21.37"},
					{BlockHeight: 300, Version: "0.21.37"},
				},
				Sequence: 1,
			},
			expected: true,
		},
		{
			name: "decreasing height is invalid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "0.21.37"},
					{BlockHeight: 200, Version: "0.21.37"},
					{BlockHeight: 180, Version: "0.21.37"},
					{BlockHeight: 300, Version: "0.21.37"},
				},
				Sequence: 1,
			},
			expected: false,
		},
		{
			name: "version higher or equal to the previous one is valid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "0.21.37"},
					{BlockHeight: 200, Version: "0.21.37"},
					{BlockHeight: 300, Version: "0.21.38"},
					{BlockHeight: 400, Version: "1.0.0"},
				},
				Sequence: 1,
			},
			expected: true,
		},
		{
			name: "any version lower than previous one is invalid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "0.21.37"},
					{BlockHeight: 200, Version: "1.2.3"},
					{BlockHeight: 300, Version: "1.2.4"},
					{BlockHeight: 400, Version: "1.2.3"},
				},
				Sequence: 1,
			},
			expected: false,
		},
		{
			name: "all version must be valid semver string to be valid",
			vb: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "0.21.37"},
					{BlockHeight: 200, Version: "0.21.37"},
					{BlockHeight: 300, Version: "0.21.38"},
					{BlockHeight: 400, Version: "v0.21.39"},
				},
				Sequence: 1,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vb.Validate()
			if tc.expected {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestVersionBeaconString(t *testing.T) {
	vb := &flow.VersionBeacon{
		VersionBoundaries: []flow.VersionBoundary{
			{BlockHeight: 1, Version: "0.21.37"},
			{BlockHeight: 200, Version: "0.21.37-patch.1"},
		},
		Sequence: 1,
	}
	require.Equal(t, "1:0.21.37 200:0.21.37-patch.1 ", vb.String())
}

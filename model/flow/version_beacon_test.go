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
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
				},
				Sequence: 1,
			},
			result: true,
		},
		{
			name: "Different sequence",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
				},
				Sequence: 2,
			},
			result: false,
		},
		{
			name: "Different version boundaries",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
				},
				Sequence: 1,
			},
			vb2: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.2.0"},
				},
				Sequence: 1,
			},
			result: false,
		},
		{
			name: "Different length of version boundaries",
			vb1: flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{BlockHeight: 1, Version: "v1.0.0"},
					{BlockHeight: 2, Version: "v1.1.0"},
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.result, tc.vb1.EqualTo(&tc.vb2))
		})
	}
}

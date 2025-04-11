package environment

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func Test_MapToCadenceVersion(t *testing.T) {
	flowV0 := semver.Version{}
	cadenceV0 := semver.Version{}
	flowV1 := semver.Version{
		Major: 0,
		Minor: 37,
		Patch: 0,
	}
	cadenceV1 := semver.Version{
		Major: 1,
		Minor: 0,
		Patch: 0,
	}

	mapping := FlowGoToCadenceVersionMapping{
		FlowGoVersion:  flowV1,
		CadenceVersion: cadenceV1,
	}

	t.Run("no mapping, v0", func(t *testing.T) {
		version := mapToCadenceVersion(flowV0, FlowGoToCadenceVersionMapping{})

		require.Equal(t, cadenceV0, version)
	})

	t.Run("v0", func(t *testing.T) {
		version := mapToCadenceVersion(flowV0, mapping)

		require.Equal(t, semver.Version{}, version)
	})
	t.Run("v1 - delta", func(t *testing.T) {

		v := flowV1
		v.Patch -= 1

		version := mapToCadenceVersion(v, mapping)

		require.Equal(t, cadenceV0, version)
	})
	t.Run("v1", func(t *testing.T) {
		version := mapToCadenceVersion(flowV1, mapping)

		require.Equal(t, cadenceV1, version)
	})
	t.Run("v1 + delta", func(t *testing.T) {

		v := flowV1
		v.BumpPatch()

		version := mapToCadenceVersion(v, mapping)

		require.Equal(t, cadenceV1, version)
	})
}

func Test_VersionBeaconAsDataSource(t *testing.T) {
	t.Run("no version beacon", func(t *testing.T) {
		versionBeacon := VersionBeaconExecutionVersionProvider{
			getVersionBeacon: func() (*flow.SealedVersionBeacon, error) {
				return nil, nil
			},
		}
		version, err := versionBeacon.ExecutionVersion()
		require.NoError(t, err)
		require.Equal(t, semver.Version{}, version)
	})

	t.Run("version beacon", func(t *testing.T) {
		versionBeacon := NewVersionBeaconExecutionVersionProvider(
			func() (*flow.SealedVersionBeacon, error) {
				return &flow.SealedVersionBeacon{
					VersionBeacon: &flow.VersionBeacon{
						VersionBoundaries: []flow.VersionBoundary{
							{
								BlockHeight: 10,
								Version:     semver.Version{Major: 0, Minor: 37, Patch: 0}.String(),
							},
						},
					},
				}, nil
			},
		)
		version, err := versionBeacon.ExecutionVersion()
		require.NoError(t, err)
		require.Equal(t, semver.Version{Major: 0, Minor: 37, Patch: 0}, version)
	})

	t.Run("version beacon, multiple boundaries", func(t *testing.T) {
		versionBeacon := NewVersionBeaconExecutionVersionProvider(
			func() (*flow.SealedVersionBeacon, error) {
				return &flow.SealedVersionBeacon{
					VersionBeacon: &flow.VersionBeacon{
						VersionBoundaries: []flow.VersionBoundary{
							{
								BlockHeight: 10,
								Version:     semver.Version{Major: 0, Minor: 37, Patch: 0}.String(),
							},
							{
								BlockHeight: 20,
								Version:     semver.Version{Major: 1, Minor: 0, Patch: 0}.String(),
							},
						},
					},
				}, nil
			},
		)

		version, err := versionBeacon.ExecutionVersion()
		require.NoError(t, err)
		// the first boundary is by definition the newest past one and defines the version
		require.Equal(t, semver.Version{Major: 0, Minor: 37, Patch: 0}, version)
	})
}

func Test_MinimumCadenceRequiredVersion(t *testing.T) {
	t.Run("no version beacon", func(t *testing.T) {
		getCadenceVersion := func(executionVersion string) (string, error) {
			versionBeacon := NewVersionBeaconExecutionVersionProvider(
				func() (*flow.SealedVersionBeacon, error) {
					return &flow.SealedVersionBeacon{
						VersionBeacon: &flow.VersionBeacon{
							VersionBoundaries: []flow.VersionBoundary{
								{
									BlockHeight: 10,
									Version:     executionVersion,
								},
							},
						},
					}, nil
				},
			)
			cadenceVersion := NewMinimumCadenceRequiredVersion(versionBeacon)
			return cadenceVersion.MinimumRequiredVersion()
		}

		setFVMToCadenceVersionMappingForTestingOnly(FlowGoToCadenceVersionMapping{
			FlowGoVersion:  semver.Version{Major: 0, Minor: 37, Patch: 0},
			CadenceVersion: semver.Version{Major: 1, Minor: 0, Patch: 0},
		})

		requireExpectedSemver := func(t *testing.T, executionVersion semver.Version, expectedCadenceVersion semver.Version) {
			t.Helper()
			actualCadenceVersion, err := getCadenceVersion(executionVersion.String())
			require.NoError(t, err)
			require.Equal(t, expectedCadenceVersion.String(), actualCadenceVersion)
		}

		requireExpectedSemver(t, semver.Version{Major: 0, Minor: 36, Patch: 9}, semver.Version{Major: 0, Minor: 0, Patch: 0})
		requireExpectedSemver(t, semver.Version{Major: 0, Minor: 37, Patch: 0}, semver.Version{Major: 1, Minor: 0, Patch: 0})
		requireExpectedSemver(t, semver.Version{Major: 0, Minor: 37, Patch: 1}, semver.Version{Major: 1, Minor: 0, Patch: 0})
	})
}

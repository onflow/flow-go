package environment

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

func Test_MapToCadenceVersion(t *testing.T) {
	v0 := semver.Version{}
	flowV1 := semver.Version{
		Major:      1,
		Minor:      2,
		Patch:      3,
		PreRelease: "rc.1",
	}
	flowV2 := semver.Version{
		Major:      2,
		Minor:      2,
		Patch:      3,
		PreRelease: "rc.1",
	}

	cadenceV1 := semver.Version{
		Major:      2,
		Minor:      1,
		Patch:      3,
		PreRelease: "rc.2",
	}
	cadenceV2 := semver.Version{
		Major:      12,
		Minor:      0,
		Patch:      0,
		PreRelease: "",
	}

	mapping := []VersionMapEntry{
		{
			FlowGoVersion:  flowV1,
			CadenceVersion: cadenceV1,
		},
		{
			FlowGoVersion:  flowV2,
			CadenceVersion: cadenceV2,
		},
	}

	mappingWith2Versions := []VersionMapEntry{
		{
			FlowGoVersion:  flowV1,
			CadenceVersion: cadenceV1,
		},
		{
			FlowGoVersion:  flowV2,
			CadenceVersion: cadenceV2,
		},
	}

	t.Run("no mapping, v0", func(t *testing.T) {
		version := mapToCadenceVersion(v0, nil)

		require.Equal(t, v0, version)
	})

	t.Run("v0", func(t *testing.T) {
		version := mapToCadenceVersion(v0, mappingWith2Versions)

		require.Equal(t, semver.Version{}, version)
	})
	t.Run("v1 - delta", func(t *testing.T) {

		v := flowV1
		v.Patch -= 1

		version := mapToCadenceVersion(v, mappingWith2Versions)

		require.Equal(t, v0, version)
	})
	t.Run("v1", func(t *testing.T) {
		version := mapToCadenceVersion(flowV1, mappingWith2Versions)

		require.Equal(t, cadenceV1, version)
	})
	t.Run("v1 + delta", func(t *testing.T) {

		v := flowV1
		v.BumpPatch()

		version := mapToCadenceVersion(v, mappingWith2Versions)

		require.Equal(t, cadenceV1, version)
	})
	t.Run("v2 - delta", func(t *testing.T) {

		v := flowV2
		v.Patch -= 1

		version := mapToCadenceVersion(v, mappingWith2Versions)

		require.Equal(t, cadenceV1, version)
	})
	t.Run("v2", func(t *testing.T) {
		version := mapToCadenceVersion(flowV2, mappingWith2Versions)

		require.Equal(t, cadenceV2, version)
	})
	t.Run("v2 + delta", func(t *testing.T) {

		v := flowV2
		v.BumpPatch()

		version := mapToCadenceVersion(v, mappingWith2Versions)

		require.Equal(t, cadenceV2, version)
	})

	t.Run("v1 - delta, single version in mapping", func(t *testing.T) {

		v := flowV1
		v.Patch -= 1

		version := mapToCadenceVersion(v, mapping)

		require.Equal(t, v0, version)
	})
	t.Run("v1, single version in mapping", func(t *testing.T) {
		version := mapToCadenceVersion(flowV1, mapping)

		require.Equal(t, cadenceV1, version)
	})
	t.Run("v1 + delta, single version in mapping", func(t *testing.T) {

		v := flowV1
		v.BumpPatch()

		version := mapToCadenceVersion(v, mapping)

		require.Equal(t, cadenceV1, version)
	})
}

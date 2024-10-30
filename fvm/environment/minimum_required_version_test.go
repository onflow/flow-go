package environment

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
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

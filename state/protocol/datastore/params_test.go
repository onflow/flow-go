package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewVersionedInstanceParams verifies that NewVersionedInstanceParams
// correctly constructs instance params for supported versions and
// returns an error for unsupported versions.
// Test cases:
// 1) If version is 0, it constructs InstanceParamsV0 with the provided data.
// 2) If version is unsupported, it returns an error.
func TestNewVersionedInstanceParams(t *testing.T) {
	finalizedRootID := unittest.IdentifierFixture()
	sealedRootID := unittest.IdentifierFixture()
	sporkRootID := unittest.IdentifierFixture()

	t.Run("valid version 0", func(t *testing.T) {
		versioned, err := NewVersionedInstanceParams(0, finalizedRootID, sealedRootID, sporkRootID)
		require.NoError(t, err)
		require.Equal(t, uint64(0), versioned.Version)

		var v0 InstanceParamsV0
		err = msgpack.Unmarshal(versioned.Data, &v0)
		require.NoError(t, err)

		require.Equal(t, finalizedRootID, v0.FinalizedRootID)
		require.Equal(t, sealedRootID, v0.SealedRootID)
		require.Equal(t, sporkRootID, v0.SporkRootBlockID)
	})

	t.Run("unsupported version", func(t *testing.T) {
		versioned, err := NewVersionedInstanceParams(99, finalizedRootID, sealedRootID, sporkRootID)
		require.Nil(t, versioned)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported instance params version")
	})
}

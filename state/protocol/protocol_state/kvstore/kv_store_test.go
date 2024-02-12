package kvstore

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncodeDecode tests encoding and decoding all supported model versions.
//   - VersionedEncode should return the correct version
//   - instances should be equal after encoding, then decoding
func TestEncodeDecode(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := &modelv0{}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(0), version)

		decoded, err := VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})

	t.Run("v1", func(t *testing.T) {
		model := &modelv1{
			InvalidEpochTransitionAttempted: rand.Int()%2 == 0,
		}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), version)

		decoded, err := VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})
}

// TestAPI tests that all supported model versions satisfy the public interfaces.
//   - should be able to read/write supported keys
//   - should return the appropriate sentinel for unsupported keys
func TestAPI(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := modelv0{}

		// v0
		version := model.GetProtocolStateVersion()
		assert.Equal(t, uint64(0), version)

		// v1
		err := model.SetInvalidEpochTransitionAttempted(true)
		assert.ErrorIs(t, err, ErrKeyNotSupported)

		_, err = model.GetInvalidEpochTransitionAttempted()
		assert.ErrorIs(t, err, ErrKeyNotSupported)
	})

	t.Run("v1", func(t *testing.T) {
		model := modelv1{}

		// v0
		version := model.GetProtocolStateVersion()
		assert.Equal(t, uint64(1), version)

		// v1
		err := model.SetInvalidEpochTransitionAttempted(true)
		assert.NoError(t, err)

		invalidEpochTransitionAttempted, err := model.GetInvalidEpochTransitionAttempted()
		assert.NoError(t, err)
		assert.Equal(t, true, invalidEpochTransitionAttempted)
	})
}

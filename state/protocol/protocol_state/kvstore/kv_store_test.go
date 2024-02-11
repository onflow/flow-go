package kvstore

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := modelv0{}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(0), version)

		decoded, err := VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})

	t.Run("v1", func(t *testing.T) {
		model := modelv1{
			InvalidEpochTransitionAttempted: rand.Int()%2 == 0,
		}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(0), version)

		decoded, err := VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})
}

func TestAPI(t *testing.T) {
	t.Run("")
}

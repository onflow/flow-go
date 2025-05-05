package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightCollectionFingerprint(t *testing.T) {
	col := unittest.CollectionFixture(2)
	colID := col.ID()
	data := fingerprint.Fingerprint(col.Light())
	var decoded flow.LightCollection
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	decodedID := decoded.ID()
	assert.Equal(t, colID, decodedID)
	assert.Equal(t, col.Light(), decoded)
}

// TestLightCollectionID is a basic check that the ID function is deterministic.
func TestLightCollectionID(t *testing.T) {
	col := unittest.CollectionFixture(2).Light()
	uncached := col.UncachedID()
	id := col.ID()
	id2 := col.ID()
	assert.Equal(t, uncached, id)
	assert.Equal(t, id, id2)
}

// BenchmarkLightCollectionID compares the cached and uncached ID functions.
func BenchmarkLightCollectionID(b *testing.B) {
	col := unittest.CollectionFixture(20).Light()

	b.Run("cached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = col.ID()
		}
	})
	b.Run("uncached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = col.UncachedID()
		}
	})
}

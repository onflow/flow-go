package pebble

import (
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBootstrap_NewBootstrap(t *testing.T) {
	sampleDir := path.Join(unittest.TempDir(t), "checkpoint.checkpoint")
	rootHeight := uint64(1)
	cache := pebble.NewCache(1 << 20)
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	unittest.RunWithConfiguredPebbleInstance(t, opts, func(p *pebble.DB) {
		// no issues when pebble instance is blank
		_, err := NewBootstrap(p, sampleDir, rootHeight)
		require.NoError(t, err)
		// set heights
		require.NoError(t, p.Set(firstHeightKey(), EncodedUint64(rootHeight), nil))
		require.NoError(t, p.Set(latestHeightKey(), EncodedUint64(rootHeight), nil))
		// errors if FirstHeight or LastHeight are populated
		_, err = NewBootstrap(p, sampleDir, rootHeight)
		require.ErrorContains(t, err, "cannot bootstrap populated DB")
	})
}

func TestBootstrap_IndexCheckpointFile(t *testing.T) {

}

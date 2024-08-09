package pruner

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type coreBase struct {
	chunkDataPacks storage.ChunkDataPacks

	throttleDelay time.Duration
}

func newCoreBase(chunkDataPacks storage.ChunkDataPacks, throttleDelay time.Duration) *coreBase {
	return &coreBase{
		chunkDataPacks: chunkDataPacks,
		throttleDelay:  throttleDelay,
	}
}

// pruneBlock removes all data associated with a block
// this method contains all the logic to perform the actual pruning.
// No errors are expected during normal operation.
func (c *coreBase) pruneBlock(blockID flow.Identifier, batch storage.BatchStorage) error {
	time.Sleep(c.throttleDelay)

	err := c.chunkDataPacks.Prune(blockID, batch)
	if err != nil {
		return fmt.Errorf("could not prune chunk data packs: %w", err)
	}

	return nil
}

package storage

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type ChunkLocatorQueue interface {
	StoreChunkLocator(chunkID flow.Identifier, locator *chunks.ChunkLocator) (bool, error)

	LatestIndex() (int64, error)

	AtIndex(index int64) (*chunks.ChunkLocator, error)
}

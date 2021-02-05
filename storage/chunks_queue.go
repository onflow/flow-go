package storage

import (
	"github.com/onflow/flow-go/model/chunks"
)

type ChunkLocatorQueue interface {
	StoreChunkLocator(locator *chunks.ChunkLocator) (bool, error)

	LatestIndex() (int64, error)

	AtIndex(index int64) (*chunks.ChunkLocator, error)
}

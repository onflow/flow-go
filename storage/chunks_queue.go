package storage

import (
	"github.com/onflow/flow-go/model/chunks"
)

type ChunksQueue interface {
	StoreChunkLocator(locator *chunks.Locator) (bool, error)

	LatestIndex() (uint64, error)

	AtIndex(index uint64) (*chunks.Locator, error)
}

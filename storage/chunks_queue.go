package storage

import (
	"github.com/onflow/flow-go/model/chunks"
)

type ChunksQueue interface {
	StoreChunkLocator(locator *chunks.Locator) (bool, error)

	LatestIndex() (int64, error)

	AtIndex(index int64) (*chunks.Locator, error)
}

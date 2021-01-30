package storage

import "github.com/onflow/flow-go/model/flow"

type ChunksQueue interface {
	StoreChunk(*flow.Chunk) (bool, error)

	LatestIndex() (int64, error)

	AtIndex(index int64) (*flow.Chunk, error)
}

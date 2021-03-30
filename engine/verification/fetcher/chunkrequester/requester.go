package chunkrequester

import (
	"time"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
)

type ChunkDataPackRequester struct {
	handler       fetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	retryInterval time.Duration                // determines time in milliseconds for retrying chunk data requests.
}

func New(retryInterval time.Duration,
	handler fetcher.ChunkDataPackHandler) *ChunkDataPackRequester {
	return &ChunkDataPackRequester{
		retryInterval: retryInterval,
		handler:       handler,
	}
}

func (c *ChunkDataPackRequester) Request(chunkID flow.Identifier, blockID flow.Identifier, targetIDs flow.IdentifierList) error {
	return nil
}

package utils

import (
	"context"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
)

// block reference is valid for 600 blocks, which is around 10 minutes at 1 second block time.
const maxAge = 5 * time.Second

// BlockRef stores information about the block reference to allow cached fetching
type BlockRef struct {
	c              *client.Client
	cachedBlockRef flowsdk.Identifier
	validUntil     time.Time
	mux            sync.Mutex
}

func NewBlockRef(c *client.Client) *BlockRef {
	return &BlockRef{
		c: c,
	}
}

// Get gets the cached block reference if it is younger than 1 minute, otherwise it fetches and caches the latest ref
func (b *BlockRef) Get() (flowsdk.Identifier, error) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if b.validUntil.After(time.Now()) {
		// still valid, return it
		return b.cachedBlockRef, nil
	}

	ref, err := b.c.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return flowsdk.Identifier{}, err
	}

	b.cachedBlockRef = ref.ID
	b.validUntil = time.Now().Add(maxAge)

	return b.cachedBlockRef, nil
}

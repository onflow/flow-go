package blob

import (
	"context"
	"crypto/rand"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	rateLimitedBs := newRateLimitedBlockStore(bs, "rate_test", 4, 4) // 4 bytes per second, burst of 4

	data := make([]byte, 4)
	rand.Read(data)
	blk := blocks.NewBlock(data)
	rateLimitedBs.Put(context.Background(), blk)

	_, err := rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.ErrorIs(t, err, rateLimitedError) // second request should be rate limited
}

func TestBurstLimit(t *testing.T) {
	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	rateLimitedBs := newRateLimitedBlockStore(bs, "burst_test", 4, 8) // 4 bytes per second, burst of 8

	data := make([]byte, 4)
	rand.Read(data)
	blk := blocks.NewBlock(data)
	rateLimitedBs.Put(context.Background(), blk)

	_, err := rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err) // second request is allowed due to burst limit
}

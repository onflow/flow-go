package blob

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimit(t *testing.T) {
	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	rateLimitedBs := newRateLimitedBlockStore(bs, "rate_test", 4, 4) // 4 bytes per second, burst of 4

	data := make([]byte, 4)
	_, _ = rand.Read(data)
	blk := blocks.NewBlock(data)
	err := rateLimitedBs.Put(context.Background(), blk)
	require.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.ErrorIs(t, err, rateLimitedError) // second request should be rate limited
}

func TestBurstLimit(t *testing.T) {
	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	rateLimitedBs := newRateLimitedBlockStore(bs, "burst_test", 4, 8) // 4 bytes per second, burst of 8

	data := make([]byte, 4)
	_, _ = rand.Read(data)
	blk := blocks.NewBlock(data)
	err := rateLimitedBs.Put(context.Background(), blk)
	require.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err) // second request is allowed due to burst limit
}

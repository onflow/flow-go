package blob

func TestRateLimit(t *testing.T) {
	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	rateLimitedBs := newRateLimitedBlockstore(bs, 1, 1) // 1 event per second, burst of 1

	data := make([]byte, 4)
	rand.Read(data)
	blk := blocks.NewBlock(data)
	rateLimitedBs.Put(context.Background(), blk)

	_, err := rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.NoError(t, err)

	_, err = rateLimitedBs.Get(context.Background(), blk.Cid())
	assert.ErrorIs(t, err, rateLimitedError) // second request should be rate limited
}

package ledger

// An ExecutionCache is a multi-level cache used to save execution state
// for different stages of block execution.
type ExecutionCache struct {
	blockView *View
	chunkView *View
}

// NewExecutionCache returns an empty execution cache.
func NewExecutionCache() *ExecutionCache {
	// TODO: connect execution cache to storage
	block := NewView(
		func(key string) ([]byte, error) { return nil, nil },
	)

	chunk := NewView(block.Get)

	return &ExecutionCache{
		blockView: block,
		chunkView: chunk,
	}
}

// StartNewChunk prepares the cache to execute a new chunk.
//
// This function applies the current chunk delta to the block cache and
// empties the chunk cache.
func (ec *ExecutionCache) StartNewChunk() {
	// apply current chunk delta to block cache
	ec.blockView.ApplyDelta(ec.chunkView.Delta())

	// create new chunk cache
	ec.chunkView = NewView(ec.blockView.Get)
}

// NewTransactionView creates a new ledger view that can be used to execute a transaction.
func (ec *ExecutionCache) NewTransactionView() *View {
	return NewView(ec.chunkView.Get)
}

// ApplyTransactionDelta applies a transaction ledger delta to the current chunk cache.
func (ec *ExecutionCache) ApplyTransactionDelta(delta Delta) {
	ec.chunkView.ApplyDelta(delta)
}

// ChunkDelta returns the ledger delta for the current chunk.
func (ec *ExecutionCache) ChunkDelta() Delta {
	return ec.chunkView.Delta()
}

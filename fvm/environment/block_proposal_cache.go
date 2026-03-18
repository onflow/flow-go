package environment

// BlockProposalCache provides in-memory caching for the active EVM block proposal,
// scoped to a single Cadence transaction. The value is stored as any to avoid an
// import cycle with the fvm/evm/types package.
type BlockProposalCache interface {
	// CachedBlockProposal returns the currently cached block proposal, or nil if none is cached.
	CachedBlockProposal() any
	// CacheBlockProposal stores the given block proposal in the in-memory cache.
	CacheBlockProposal(any)
	// SetBlockProposalFlusher registers a function to persist the cached proposal to storage.
	// Called by the EVM block store when it first stages a proposal.
	SetBlockProposalFlusher(func() error)
	// FlushBlockProposal calls the registered flusher to persist the cached proposal.
	// It is a no-op if no flusher has been registered.
	FlushBlockProposal() error
}

// blockProposalCache is a simple in-memory cache for a block proposal value.
type blockProposalCache struct {
	value   any
	flusher func() error
}

func (c *blockProposalCache) CachedBlockProposal() any {
	return c.value
}

func (c *blockProposalCache) CacheBlockProposal(v any) {
	c.value = v
}

func (c *blockProposalCache) SetBlockProposalFlusher(f func() error) {
	c.flusher = f
}

func (c *blockProposalCache) FlushBlockProposal() error {
	if c.flusher == nil {
		return nil
	}
	return c.flusher()
}

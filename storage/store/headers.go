package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Headers implements a simple read-only header storage around a DB.
type Headers struct {
	db storage.DB
	// cache is essentially an in-memory map from `Block.ID()` -> `Header`
	cache       *Cache[flow.Identifier, *flow.Header]
	heightCache *Cache[uint64, flow.Identifier]
	viewCache   *Cache[uint64, flow.Identifier]
	sigs        *proposalSignatures
	chainID     flow.ChainID
}

var _ storage.Headers = (*Headers)(nil)

// NewHeaders creates a Headers instance, which manages block headers of the main consensus (not cluster consensus).
// It supports storing, caching and retrieving by block ID, and additionally indexes by header height and view.
// Must be initialized with a non-cluster chainID; see [flow.AllChainIDs] and [cluster.IsCanonicalClusterID].
// No errors are expected during normal operations.
func NewHeaders(collector module.CacheMetrics, db storage.DB, chainID flow.ChainID) (*Headers, error) {
	if cluster.IsCanonicalClusterID(chainID) {
		return nil, irrecoverable.NewExceptionf("NewHeaders called on cluster chain ID %s - use NewClusterHeaders instead", chainID)
	}
	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, header *flow.Header) error {
		if header.ChainID != chainID {
			return fmt.Errorf("expected chain ID %v, got %v: %w", chainID, header.ChainID, storage.ErrWrongChain)
		}
		if !lctx.HoldsLock(storage.LockInsertBlock) {
			return fmt.Errorf("missing lock: %v", storage.LockInsertBlock)
		}
		return operation.InsertHeader(lctx, rw, blockID, header)
	}
	retrieveHeight := func(r storage.Reader, height uint64) (flow.Identifier, error) {
		var id flow.Identifier
		err := operation.LookupBlockHeight(r, height, &id)
		return id, err
	}
	retrieveView := func(r storage.Reader, view uint64) (flow.Identifier, error) {
		var id flow.Identifier
		err := operation.LookupCertifiedBlockByView(r, view, &id)
		return id, err
	}
	return newHeaders(collector, db, chainID, storeWithLock, retrieveHeight, retrieveView), nil
}

// NewClusterHeaders creates a Headers instance for a collection cluster chain, which stores block headers for cluster blocks.
// It supports storing, caching and retrieving by block ID, and additionally an index by header height.
// It does NOT support retrieving by view.
// Must be initialized with a valid cluster chain ID; see [cluster.IsCanonicalClusterID]
// No errors are expected during normal operations.
func NewClusterHeaders(collector module.CacheMetrics, db storage.DB, chainID flow.ChainID) (*Headers, error) {
	if !cluster.IsCanonicalClusterID(chainID) {
		return nil, irrecoverable.NewExceptionf("NewClusterHeaders called on non-cluster chain ID %s - use NewHeaders instead", chainID)
	}
	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, header *flow.Header) error {
		if header.ChainID != chainID {
			return fmt.Errorf("expected chain ID %v, got %v: %w", chainID, header.ChainID, storage.ErrWrongChain)
		}
		if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
			return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
		}
		return operation.InsertClusterHeader(lctx, rw, blockID, header)
	}
	retrieveHeight := func(r storage.Reader, height uint64) (flow.Identifier, error) {
		var id flow.Identifier
		err := operation.LookupClusterBlockHeight(r, chainID, height, &id)
		return id, err
	}
	retrieveView := func(r storage.Reader, view uint64) (flow.Identifier, error) {
		return flow.ZeroID, storage.ErrNotAvailableForClusterConsensus
	}
	return newHeaders(collector, db, chainID, storeWithLock, retrieveHeight, retrieveView), nil
}

// newHeaders contains shared logic for Header storage, including storing and retrieving by block ID
func newHeaders(collector module.CacheMetrics,
	db storage.DB,
	chainID flow.ChainID,
	storeWithLock storeWithLockFunc[flow.Identifier, *flow.Header],
	retrieveHeight retrieveFunc[uint64, flow.Identifier],
	retrieveView retrieveFunc[uint64, flow.Identifier],
) *Headers {
	retrieve := func(r storage.Reader, blockID flow.Identifier) (*flow.Header, error) {
		var header flow.Header
		err := operation.RetrieveHeader(r, blockID, &header)
		if err != nil {
			return nil, err
		}
		// raise an error when the retrieved header is for a different chain than expected
		if header.ChainID != chainID {
			return nil, fmt.Errorf("expected chain ID %v, got %v: %w", chainID, header.ChainID, storage.ErrWrongChain)
		}
		return &header, nil
	}

	h := &Headers{
		db: db,
		cache: newCache(collector, metrics.ResourceHeader,
			withLimit[flow.Identifier, *flow.Header](4*flow.DefaultTransactionExpiry),
			withStoreWithLock(storeWithLock),
			withRetrieve(retrieve)),

		heightCache: newCache(collector, metrics.ResourceFinalizedHeight,
			withLimit[uint64, flow.Identifier](4*flow.DefaultTransactionExpiry),
			withRetrieve(retrieveHeight)),

		viewCache: newCache(collector, metrics.ResourceCertifiedView,
			withLimit[uint64, flow.Identifier](4*flow.DefaultTransactionExpiry),
			withRetrieve(retrieveView)),

		sigs:    newProposalSignatures(collector, db),
		chainID: chainID,
	}

	return h
}

// storeTx adds storage operations for the given block header and auxiliary signature data to the provided database batch.
//
// CAUTION:
//   - The caller must ensure that `blockID` is a collision-resistant hash of the provided header!
//     Otherwise, data corruption may occur.
//   - The caller must acquire one (but not both) of the following locks and hold it until the database
//     write has been committed: either [storage.LockInsertBlock] or [storage.LockInsertOrFinalizeClusterBlock]
//     (depending on whether this is the header of the main consensus or a cluster consensus).
//
// It returns [storage.ErrAlreadyExists] if the header already exists, i.e. we only insert a new header once.
// This error allows the caller to detect duplicate inserts. If the header is stored along with other parts
// of the block in the same batch, similar duplication checks can be skipped for storing other parts of the block.
// Returns [storage.ErrWrongChain] if the header's ChainID does not match the one used when initializing the storage.
// No other errors are expected during normal operation.
func (h *Headers) storeTx(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockID flow.Identifier,
	header *flow.Header,
	proposalSig []byte,
) error {
	err := h.cache.PutWithLockTx(lctx, rw, blockID, header)
	if err != nil {
		return err
	}

	return h.sigs.storeTx(lctx, rw, blockID, proposalSig)
}

// retrieveTx returns the header with the given ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no block header with the given ID exists
//   - [storage.ErrWrongChain] if the block header exists in the database but is part of a different chain than expected
func (h *Headers) retrieveTx(blockID flow.Identifier) (*flow.Header, error) {
	val, err := h.cache.Get(h.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// retrieveProposalTx returns a proposal header of the block with the given ID.
// Essentially, this is the header, along with the proposer's signature.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no block header with the given ID exists
//   - [storage.ErrWrongChain] if the block header exists in the database but is part of a different chain than expected
func (h *Headers) retrieveProposalTx(blockID flow.Identifier) (*flow.ProposalHeader, error) {
	header, err := h.cache.Get(h.db.Reader(), blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}
	sig, err := h.sigs.retrieveTx(blockID)
	if err != nil {
		// a missing proposer signature implies state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve proposer signature for id %x: %w", blockID, err)
	}
	return &flow.ProposalHeader{Header: header, ProposerSigData: sig}, nil
}

// retrieveIdByHeightTx returns the block ID for the given finalized height.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no finalized block is known (for the chain this Headers instance is bound to)
func (h *Headers) retrieveIdByHeightTx(height uint64) (flow.Identifier, error) {
	// This method can only return IDs for the desired chain. This is because the height cache is populated
	// on retrieval, using a chain-specific database index `height` -> `ID`. Only blocks of the respective chain
	// are added to the index. Hence, only blocks of the respective chain are put into the cache on retrieval.
	blockID, err := h.heightCache.Get(h.db.Reader(), height)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("failed to retrieve block ID for height %d: %w", height, err)
	}
	return blockID, nil
}

// ByBlockID returns the header with the given ID. It is available for finalized blocks and those pending finalization.
// Error returns:
//   - [storage.ErrNotFound] if no block header with the given ID exists
//   - [storage.ErrWrongChain] if the block header exists in the database but is part of a different chain than expected
func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	return h.retrieveTx(blockID)
}

// ProposalByBlockID returns the header with the given ID, along with the corresponding proposer signature.
// It is available for finalized blocks and those pending finalization.
// Error returns:
//   - [storage.ErrNotFound] if no block header or proposer signature with the given blockID exists
//   - [storage.ErrWrongChain] if the block header exists in the database but is part of a different chain than expected
func (h *Headers) ProposalByBlockID(blockID flow.Identifier) (*flow.ProposalHeader, error) {
	return h.retrieveProposalTx(blockID)
}

// ByHeight returns the block with the given number. It is only available for finalized blocks.
// Error returns:
//   - [storage.ErrNotFound] if no finalized block is known at the given height
func (h *Headers) ByHeight(height uint64) (*flow.Header, error) {
	blockID, err := h.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header for height %d: %w", height, err)
	}
	header, err := h.retrieveTx(blockID)
	if err != nil {
		// any error here implies state corruption, since the block indicated by the height index was unavailable
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed header %x for height %d: %w", blockID, height, err)
	}
	return header, nil
}

// ByView returns the block with the given view. It is only available for certified blocks on a consensus chain.
// Certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
// even for non-finalized blocks.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block is known at given view.
//   - [storage.ErrNotAvailableForClusterConsensus] if called on a cluster Headers instance (created by [NewClusterHeaders])
func (h *Headers) ByView(view uint64) (*flow.Header, error) {
	blockID, err := h.viewCache.Get(h.db.Reader(), view)
	if err != nil {
		return nil, err
	}
	header, err := h.retrieveTx(blockID)
	if err != nil {
		// any error here implies state corruption, since the block indicated by the view index was unavailable
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed header %x for view %d: %w", blockID, view, err)
	}
	return header, nil
}

// Exists returns true if a header with the given ID has been stored on the appropriate chain.
// No errors are expected during normal operation.
func (h *Headers) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true (blocks on a different chain are never cached)
	if ok := h.cache.IsCached(blockID); ok {
		return ok, nil
	}
	// otherwise, try retrieve the header and check the ChainID is correct
	_, err := h.retrieveTx(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrWrongChain) {
			return false, nil
		}
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return true, nil
}

// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
// version of `ByHeight` that skips retrieving the block. Expected errors during normal operations:
//   - [storage.ErrNotFound] if no finalized block is known at given height
func (h *Headers) BlockIDByHeight(height uint64) (flow.Identifier, error) {
	blockID, err := h.retrieveIdByHeightTx(height)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not lookup block id by height %d: %w", height, err)
	}
	return blockID, nil
}

// ByParentID finds all children for the given parent block. The returned headers
// might be unfinalized; if there is more than one, at least one of them has to
// be unfinalized.
// CAUTION: this method is not backed by a cache and therefore comparatively slow!
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no block with the given parentID is known
//   - [storage.ErrWrongChain] if the parent is part of a different chain than expected
func (h *Headers) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	// first check the parent exists on the correct chain
	_, err := h.retrieveTx(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not check existence of parent %x: %w", parentID, err)
	}
	var blockIDs flow.IdentifierList
	err = operation.RetrieveBlockChildren(h.db.Reader(), parentID, &blockIDs)
	if err != nil {
		// if not found error is returned, there are two possible reasons:
		// 1. the parent block does not exist - has already been ruled out above
		// 2. the parent block exists but has no children, in which case we should return empty list
		if errors.Is(err, storage.ErrNotFound) {
			// parent exists but has no children
			return []*flow.Header{}, nil
		}
		return nil, fmt.Errorf("could not look up children: %w", err)
	}
	headers := make([]*flow.Header, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		header, err := h.ByBlockID(blockID)
		if err != nil {
			// failure to retrieve an indexed child indicates state corruption
			return nil, irrecoverable.NewExceptionf("could not retrieve indexed block %x: %w", blockID, err)
		}
		headers = append(headers, header)
	}
	return headers, nil
}

// BlockIDByView returns the block ID that is certified at the given view. It is an optimized
// version of `ByView` that skips retrieving the block. Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block is known at given view.
//   - [storage.ErrNotAvailableForClusterConsensus] if called on a cluster Headers instance (created by [NewClusterHeaders])
//
// NOTE: this method is not available until next spork (mainnet27) or a migration that builds the index.
func (h *Headers) BlockIDByView(view uint64) (flow.Identifier, error) {
	blockID, err := h.viewCache.Get(h.db.Reader(), view)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not lookup block id by view %d: %w", view, err)
	}
	return blockID, nil
}

// Deprecated: Undocumented, hence unsafe for public use.
func (h *Headers) FindHeaders(filter func(header *flow.Header) bool) ([]flow.Header, error) {
	blocks := make([]flow.Header, 0, 1)
	err := operation.FindHeaders(h.db.Reader(), filter, &blocks)
	return blocks, err
}

// RollbackExecutedBlock update the executed block header to the given header.
// Intended to be used by Execution Nodes only, to roll back executed block height.
// This method is NOT CONCURRENT SAFE, the caller should make sure to call
// this method in a single thread.
func (h *Headers) RollbackExecutedBlock(header *flow.Header) error {
	var blockID flow.Identifier
	err := operation.RetrieveExecutedBlock(h.db.Reader(), &blockID)
	if err != nil {
		return fmt.Errorf("cannot lookup executed block: %w", err)
	}

	var highest flow.Header
	err = operation.RetrieveHeader(h.db.Reader(), blockID, &highest)
	if err != nil {
		return fmt.Errorf("cannot retrieve executed header: %w", err)
	}

	// only rollback if the given height is below the current executed height
	if header.Height >= highest.Height {
		return fmt.Errorf("cannot roolback. expect the target height %v to be lower than highest executed height %v, but actually is not",
			header.Height, highest.Height,
		)
	}

	return h.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err = operation.UpdateExecutedBlock(rw.Writer(), header.ID())
		if err != nil {
			return fmt.Errorf("cannot update highest executed block: %w", err)
		}

		return nil
	})
}

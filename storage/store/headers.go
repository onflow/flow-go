package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
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

// NewHeaders creates a Headers instance, which stores block headers.
// It supports storing, caching and retrieving by block ID and the additionally indexed by header height.
func NewHeaders(collector module.CacheMetrics, db storage.DB, chainID flow.ChainID) *Headers {
	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, header *flow.Header) error {
		if header.ChainID != chainID {
			return fmt.Errorf("expected chain ID %v, got %v", chainID, header.ChainID) // TODO(4204) error sentinel
		}
		return operation.InsertHeader(lctx, rw, blockID, header)
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) (*flow.Header, error) {
		var header flow.Header
		err := operation.RetrieveHeader(r, blockID, &header)
		if err != nil {
			return nil, err
		}
		if header.ChainID != chainID {
			return nil, fmt.Errorf("expected chain ID '%v', got '%v'", chainID, header.ChainID) // TODO(4204) error sentinel
		}
		return &header, err
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

func (h *Headers) retrieveTx(blockID flow.Identifier) (*flow.Header, error) {
	val, err := h.cache.Get(h.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (h *Headers) retrieveProposalTx(blockID flow.Identifier) (*flow.ProposalHeader, error) {
	header, err := h.cache.Get(h.db.Reader(), blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}
	sig, err := h.sigs.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve proposer signature for id %x: %w", blockID, err)
	}
	return &flow.ProposalHeader{Header: header, ProposerSigData: sig}, nil
}

// results in [storage.ErrNotFound] for unknown height
func (h *Headers) retrieveIdByHeightTx(height uint64) (flow.Identifier, error) {
	blockID, err := h.heightCache.Get(h.db.Reader(), height)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("failed to retrieve block ID for height %d: %w", height, err)
	}
	return blockID, nil
}

// ByBlockID returns the header with the given ID. It is available for finalized blocks and those pending finalization.
// Error returns:
//   - [storage.ErrNotFound] if no block header with the given ID exists
func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	return h.retrieveTx(blockID)
}

// ProposalByBlockID returns the header with the given ID, along with the corresponding proposer signature.
// It is available for finalized blocks and those pending finalization.
// Error returns:
//   - [storage.ErrNotFound] if no block header or proposer signature with the given blockID exists
func (h *Headers) ProposalByBlockID(blockID flow.Identifier) (*flow.ProposalHeader, error) {
	return h.retrieveProposalTx(blockID)
}

// ByHeight returns the block with the given number. It is only available for finalized blocks.
// Error returns:
//   - [storage.ErrNotFound] if no finalized block is known at the given height
func (h *Headers) ByHeight(height uint64) (*flow.Header, error) {
	blockID, err := h.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return h.retrieveTx(blockID)
}

// ByView returns the block with the given view. It is only available for certified blocks.
// Certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
// even for non-finalized blocks.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block is known at given view.
func (h *Headers) ByView(view uint64) (*flow.Header, error) {
	blockID, err := h.viewCache.Get(h.db.Reader(), view)
	if err != nil {
		return nil, err
	}
	return h.retrieveTx(blockID)
}

// Exists returns true if a header with the given ID has been stored.
// No errors are expected during normal operation.
func (h *Headers) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true
	if ok := h.cache.IsCached(blockID); ok {
		return ok, nil
	}
	// otherwise, check badger store
	exists, err := operation.BlockExists(h.db.Reader(), blockID)
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
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
func (h *Headers) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	var blockIDs flow.IdentifierList
	err := operation.RetrieveBlockChildren(h.db.Reader(), parentID, &blockIDs)
	if err != nil {
		// if not found error is returned, there are two possible reasons:
		// 1. the parent block does not exist, in which case we should return not found error
		// 2. the parent block exists but has no children, in which case we should return empty list
		if errors.Is(err, storage.ErrNotFound) {
			exists, err := h.Exists(parentID)
			if err != nil {
				return nil, fmt.Errorf("could not check existence of parent %x: %w", parentID, err)
			}
			if !exists {
				return nil, fmt.Errorf("cannot retrieve children of unknown block %x: %w", parentID, storage.ErrNotFound)
			}
			// parent exists but has no children
			return []*flow.Header{}, nil
		}
		return nil, fmt.Errorf("could not look up children: %w", err)
	}
	headers := make([]*flow.Header, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		header, err := h.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve child (%x): %w", blockID, err)
		}
		headers = append(headers, header)
	}
	return headers, nil
}

// BlockIDByView returns the block ID that is certified at the given view. It is an optimized
// version of `ByView` that skips retrieving the block. Expected errors during normal operations:
//   - `[storage.ErrNotFound] if no certified block is known at given view.
//
// NOTE: this method is not available until next spork (mainnet27) or a migration that builds the index.
func (h *Headers) BlockIDByView(view uint64) (flow.Identifier, error) {
	blockID, err := h.viewCache.Get(h.db.Reader(), view)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not lookup block id by view %d: %w", view, err)
	}
	return blockID, nil
}

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

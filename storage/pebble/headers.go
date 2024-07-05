package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
)

type Headers struct {
	db          *pebble.DB
	cache       *Cache[flow.Identifier, *flow.Header]
	heightCache *Cache[uint64, flow.Identifier]
}

func NewHeaders(collector module.CacheMetrics, db *pebble.DB) *Headers {

	// CAUTION: should only be used to index FINALIZED blocks by their
	// respective height
	storeHeight := func(height uint64, id flow.Identifier) func(pebble.Writer) error {
		return operation.IndexBlockHeight(height, id)
	}

	retrieve := func(blockID flow.Identifier) func(pebble.Reader) (*flow.Header, error) {
		var header flow.Header
		return func(r pebble.Reader) (*flow.Header, error) {
			err := operation.RetrieveHeader(blockID, &header)(r)
			return &header, err
		}
	}

	retrieveHeight := func(height uint64) func(pebble.Reader) (flow.Identifier, error) {
		return func(r pebble.Reader) (flow.Identifier, error) {
			var id flow.Identifier
			err := operation.LookupBlockHeight(height, &id)(r)
			return id, err
		}
	}

	h := &Headers{
		db: db,
		cache: newCache(collector, metrics.ResourceHeader,
			withLimit[flow.Identifier, *flow.Header](4*flow.DefaultTransactionExpiry),
			withRetrieve(retrieve)),

		heightCache: newCache(collector, metrics.ResourceFinalizedHeight,
			withLimit[uint64, flow.Identifier](4*flow.DefaultTransactionExpiry),
			withStore(storeHeight),
			withRetrieve(retrieveHeight)),
	}

	return h
}

func (h *Headers) storePebble(blockID flow.Identifier, header *flow.Header) func(storage.PebbleReaderBatchWriter) error {
	return func(rw storage.PebbleReaderBatchWriter) error {
		rw.AddCallback(func() {
			h.cache.Insert(blockID, header)
		})

		_, tx := rw.ReaderWriter()
		err := operation.InsertHeader(blockID, header)(tx)
		if err != nil {
			return fmt.Errorf("could not store header %v: %w", blockID, err)
		}

		return nil
	}
}

func (h *Headers) retrieveTx(blockID flow.Identifier) func(pebble.Reader) (*flow.Header, error) {
	return h.cache.Get(blockID)
}

// results in `storage.ErrNotFound` for unknown height
func (h *Headers) retrieveIdByHeightTx(height uint64) func(pebble.Reader) (flow.Identifier, error) {
	return h.heightCache.Get(height)
}

func (h *Headers) Store(header *flow.Header) error {
	return operation.WithReaderBatchWriter(h.db, h.storePebble(header.ID(), header))
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	return h.retrieveTx(blockID)(h.db)
}

func (h *Headers) ByHeight(height uint64) (*flow.Header, error) {
	blockID, err := h.retrieveIdByHeightTx(height)(h.db)
	if err != nil {
		return nil, err
	}
	return h.retrieveTx(blockID)(h.db)
}

// Exists returns true if a header with the given ID has been stored.
// No errors are expected during normal operation.
func (h *Headers) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true
	if ok := h.cache.IsCached(blockID); ok {
		return ok, nil
	}
	// otherwise, check badger store
	var exists bool
	err := operation.BlockExists(blockID, &exists)(h.db)
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
}

// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
// version of `ByHeight` that skips retrieving the block. Expected errors during normal operation:
//   - `storage.ErrNotFound` if no finalized block is known at given height.
func (h *Headers) BlockIDByHeight(height uint64) (flow.Identifier, error) {
	blockID, err := h.retrieveIdByHeightTx(height)(h.db)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not lookup block id by height %d: %w", height, err)
	}
	return blockID, nil
}

func (h *Headers) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	var blockIDs flow.IdentifierList
	err := procedure.LookupBlockChildren(parentID, &blockIDs)(h.db)
	if err != nil {
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

func (h *Headers) FindHeaders(filter func(header *flow.Header) bool) ([]flow.Header, error) {
	blocks := make([]flow.Header, 0, 1)
	err := operation.FindHeaders(filter, &blocks)(h.db)
	return blocks, err
}

// RollbackExecutedBlock update the executed block header to the given header.
// only useful for execution node to roll back executed block height
// not concurrent safe
func (h *Headers) RollbackExecutedBlock(header *flow.Header) error {
	var blockID flow.Identifier

	err := operation.RetrieveExecutedBlock(&blockID)(h.db)
	if err != nil {
		return fmt.Errorf("cannot lookup executed block: %w", err)
	}

	var highest flow.Header
	err = operation.RetrieveHeader(blockID, &highest)(h.db)
	if err != nil {
		return fmt.Errorf("cannot retrieve executed header: %w", err)
	}

	// only rollback if the given height is below the current executed height
	if header.Height >= highest.Height {
		return fmt.Errorf("cannot roolback. expect the target height %v to be lower than highest executed height %v, but actually is not",
			header.Height, highest.Height,
		)
	}

	err = operation.InsertExecutedBlock(header.ID())(h.db)
	if err != nil {
		return fmt.Errorf("cannot update highest executed block: %w", err)
	}

	return nil
}

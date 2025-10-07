package store

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// proposalSignatures implements a proposal signature storage around a storage DB.
// The proposer's signature is only transiently important conceptually (until the block obtains a QC),
// but our current business logic validates it even in cases where it is not strictly necessary.
// For simplicity, we require it be stored for all blocks, however it is stored separately to
// make it easier to remove in the future if/when we update the syncing and block ingestion logic.
type proposalSignatures struct {
	db    storage.DB
	cache *Cache[flow.Identifier, []byte]
}

// newProposalSignatures creates a proposalSignatures instance which is a database of block proposal signatures
// which supports storing, caching and retrieving by block ID.
func newProposalSignatures(collector module.CacheMetrics, db storage.DB) *proposalSignatures {
	store := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, sig []byte) error {
		return operation.InsertProposalSignature(lctx, rw.Writer(), blockID, &sig)
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) ([]byte, error) {
		var sig []byte
		err := operation.RetrieveProposalSignature(r, blockID, &sig)
		return sig, err
	}

	return &proposalSignatures{
		db: db,
		cache: newCache(collector, metrics.ResourceProposalSignature,
			withLimit[flow.Identifier, []byte](4*flow.DefaultTransactionExpiry),
			withStoreWithLock(store),
			withRetrieve(retrieve)),
	}
}

// storeTx persists the given `sig` as the proposer's signature for the specified block.
//
// CAUTION:
//   - The caller must acquire either the lock [storage.LockInsertBlock] or [storage.LockInsertOrFinalizeClusterBlock]
//     but not both and hold the lock until the database write has been committed.
//   - The lock proof serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK
//     is done elsewhere ATOMICALLY with this write operation. It is intended that this function is called only for new
//     blocks, i.e. no signature was previously persisted for it.
//
// No error returns expected during normal operations.
func (h *proposalSignatures) storeTx(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, sig []byte) error {
	return h.cache.PutWithLockTx(lctx, rw, blockID, sig)
}

func (h *proposalSignatures) retrieveTx(blockID flow.Identifier) ([]byte, error) {
	return h.cache.Get(h.db.Reader(), blockID)
}

// ByBlockID returns the proposer signature for the specified block.
// Currently, we store the proposer signature for all blocks, even though this is only strictly
// necessary for blocks that do not have a QC yet. However, this might change in the future.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block with the specified ID is known
func (h *proposalSignatures) ByBlockID(blockID flow.Identifier) ([]byte, error) {
	return h.retrieveTx(blockID)
}

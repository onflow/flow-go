package store

import (
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
	store := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, sig []byte) error {
		return operation.InsertProposalSignature(rw.Writer(), blockID, &sig)
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
			withStore(store),
			withRetrieve(retrieve)),
	}
}

func (h *proposalSignatures) storeTx(rw storage.ReaderBatchWriter, blockID flow.Identifier, sig []byte) error {
	return h.cache.PutTx(rw, blockID, sig)
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

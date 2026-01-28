package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertQuorumCertificate atomically performs the following storage operations for the given QuorumCertificate [QC]:
//  1. Check if a QC certifying the same block is already stored.
//  2. Only if no QC exists for the block, append the storage operations for indexing the QC by the block ID it certifies.
//
// CAUTION:
//   - For the same block, different QCs can easily be constructed by selecting different sub-sets
//     of the received votes. In most cases, it is only important that a block has been certified,
//     but it is irrelevant who specifically contributed to the QC. Therefore, we only store the first QC.
//   - In order to make sure only one QC is stored per block, _all calls_ to
//     `InsertQuorumCertificate` must be synchronized by the higher-logic. Currently, we have the
//     lockctx.Proof to prove the higher logic is holding the [storage.LockInsertBlock] when
//     inserting the QC after checking that no QC is already stored.
//
// Expected error returns:
//   - [storage.ErrAlreadyExists] if any QuorumCertificate certifying the same block already exists
func InsertQuorumCertificate(lctx lockctx.Proof, rw storage.ReaderBatchWriter, qc *flow.QuorumCertificate) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot insert quorum certificate without holding lock %s", storage.LockInsertBlock)
	}

	key := MakePrefix(codeBlockIDToQuorumCertificate, qc.BlockID)
	exist, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return fmt.Errorf("failed to check if quorum certificate exists for block %s: %w", qc.BlockID, err)
	}
	if exist {
		return fmt.Errorf("quorum certificate for block %s already exists: %w", qc.BlockID, storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, qc)
}

// RetrieveQuorumCertificate retrieves the QuorumCertificate for the specified block.
// For every block that has been certified, this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a certified block
func RetrieveQuorumCertificate(r storage.Reader, blockID flow.Identifier, qc *flow.QuorumCertificate) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToQuorumCertificate, blockID), qc)
}

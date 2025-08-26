package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertQuorumCertificate upserts a quorum certificate by block ID.
// Note: we index the QC by the block it certifies. Hence, the key is not derived from the value via a
// collision-resistant hash function. For the same block, different QCs can easily be constructed by selecting
// different sub-sets of the received votes (provided more than the minimal number of consensus participants voted,
// which is typically the case). In most cases, it is only important that a block has been certified, but it is
// irrelevant who specifically contributed to the QC. Therefore, we only store the first QC.
//
// If *any* quorum certificate for the block already exists, it returns [storage.ErrAlreadyExists] (typically benign).
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

// RetrieveQuorumCertificate retrieves a quorum certificate by blockID.
// Returns [storage.ErrNotFound] if no QC is stored for the block.
func RetrieveQuorumCertificate(r storage.Reader, blockID flow.Identifier, qc *flow.QuorumCertificate) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToQuorumCertificate, blockID), qc)
}

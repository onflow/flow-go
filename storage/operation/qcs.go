package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertQuorumCertificate upserts a quorum certificate by block ID
// If a quorum certificate for the block already exists, it returns storage.ErrAlreadyExists.
func InsertQuorumCertificate(lctx lockctx.Proof, rw storage.ReaderBatchWriter, qc *flow.QuorumCertificate) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot upsert quorum certificate without holding lock %s", storage.LockInsertBlock)
	}

	key := MakePrefix(codeBlockIDToQuorumCertificate, qc.BlockID)
	exist, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return fmt.Errorf("failed to check if quorum certificate exists for block %s: %w", qc.BlockID, err)
	}

	if exist {
		return fmt.Errorf("quorum certificate for block %s already exists: %w", qc.BlockID,
			storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, qc)
}

// RetrieveQuorumCertificate retrieves a quorum certificate by blockID.
// Returns storage.ErrNotFound if no QC is stored for the block.
func RetrieveQuorumCertificate(r storage.Reader, blockID flow.Identifier, qc *flow.QuorumCertificate) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToQuorumCertificate, blockID), qc)
}

package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// QuorumCertificates represents storage for Quorum Certificates.
// Quorum Certificates are distributed using blocks, where a block incorporates a QC for its parent.
// When stored, QCs are indexed by the ID of the block they certify (not the block they are included within).
// In the example below, `QC_1` is indexed by `Block_1.ID()`
// Block_1 <- Block_2(QC_1)
type QuorumCertificates interface {
	// BatchStore stores a Quorum Certificate as part of database batch update. QC is indexed by QC.BlockID.
	//
	// Note: For the same block, different QCs can easily be constructed by selecting different sub-sets of the received votes
	// (provided more than the minimal number of consensus participants voted, which is typically the case). In most cases, it
	// is only important that a block has been certified, but irrelevant who specifically contributed to the QC. Therefore, we
	// only store the first QC.
	//
	// CAUTION: Persisting the QC only if none is already stored for the block, requires an atomic database read and write.
	// Therefore, the caller must acquire [storage.LockInsertBlock] and hold it until the database write has been committed.
	//
	// If *any* quorum certificate for QC.BlockID has already been stored, a [storage.ErrAlreadyExists] is returned (typically benign).
	BatchStore(lockctx.Proof, ReaderBatchWriter, *flow.QuorumCertificate) error

	// ByBlockID returns QC that certifies block referred by blockID.
	// * [storage.ErrNotFound] if no QC for blockID is known.
	ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error)
}

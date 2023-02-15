package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// QuorumCertificates represents storage for Quorum Certificates.
// Quorum Certificates are distributed using blocks, each block incorporates a QC for its parent.
// This specific storage allows to store and query QCs discovered from both consensus participants and followers.
// When stored, QCs are indexed by the ID of the block they reference (not the block they are included within).
// In the example below, `QC_1` is indexed by `Block_1.ID()`
//   Block_1 <- Block_2(QC_1)
type QuorumCertificates interface {
	// StoreTx stores a Quorum Certificate as part of database transaction QC is indexed by QC.BlockID.
	// * storage.ErrAlreadyExists if QC for blockID already exists
	StoreTx(qc *flow.QuorumCertificate) func(*transaction.Tx) error
	// ByBlockID returns QC that certifies block referred by blockID.
	// * storage.ErrNotFound if no QC for blockID doesn't exist.
	ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error)
}

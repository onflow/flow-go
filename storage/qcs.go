package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// QuorumCertificates represents storage for Quorum Certificates.
// Quorum Certificates are distributed using blocks, each block incorporates a QC for its parent.
// This specific storage allows to store and query QCs discovered during following the protocol or participating in.
type QuorumCertificates interface {
	// StoreTx stores a Quorum Certificate as part of database transaction QC is indexed by QC.BlockID.
	// No errors are expected during normal operations.
	StoreTx(qc *flow.QuorumCertificate) func(*transaction.Tx) error
	// ByBlockID returns QC that certifies block referred by blockID.
	// * storage.ErrNotFound if no QC for blockID doesn't exist.
	ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error)
}

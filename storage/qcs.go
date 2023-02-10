package storage

import "github.com/onflow/flow-go/model/flow"

// QuorumCertificates represents storage for Quorum Certificates.
// Quorum Certificates are distributed using blocks, each block incorporates a QC for its parent.
// This specific storage allows to store and query QCs discovered during following the protocol or participating in.
type QuorumCertificates interface {
	// Store stores a quorum certificate, QC is indexed by QC.BlockID.
	// No errors are expected during normal operations.
	Store(qc *flow.QuorumCertificate) error
	// ByBlockID returns QC that certifies block referred by blockID.
	// * storage.ErrNotFound if no QC for blockID doesn't exist.
	ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error)
}

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// QuorumCertificates represents storage for Quorum Certificates.
// Quorum Certificates are distributed using blocks, where a block incorporates a QC for its parent.
// When stored, QCs are indexed by the ID of the block they certify (not the block they are included within).
// In the example below, `QC_1` is indexed by `Block_1.ID()`
// Block_1 <- Block_2(QC_1)
type QuorumCertificates interface {
	// StoreTx stores a Quorum Certificate as part of database transaction QC is indexed by QC.BlockID.
	// * storage.ErrAlreadyExists if any QC for blockID is already stored
	StoreTx2(qc *flow.QuorumCertificate, ops TxOps) error
	// ByBlockID returns QC that certifies block referred by blockID.
	// * storage.ErrNotFound if no QC for blockID doesn't exist.
	ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error)
}

// StoreTx(qc) func(Transaction) error {
// 		return operation.InsertQuorumCertificate(qc)
// }

// func InsertQuorumCertificate(qc, ops) func(Transaction) error {
// 		key := makePrefix(codeBlockIDToQuorumCertificate, qc.BlockID)
//		val := qc
//		return ops.Insert(key, val)
// }

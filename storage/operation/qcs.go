package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UnsafeUpsertQuorumCertificate upserts a quorum certificate by block ID and
// overwrite any existing QC indexed by the same block ID.
func UnsafeUpsertQuorumCertificate(w storage.Writer, qc *flow.QuorumCertificate) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToQuorumCertificate, qc.BlockID), qc)
}

// RetrieveQuorumCertificate retrieves a quorum certificate by blockID.
// Returns storage.ErrNotFound if no QC is stored for the block.
func RetrieveQuorumCertificate(r storage.Reader, blockID flow.Identifier, qc *flow.QuorumCertificate) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToQuorumCertificate, blockID), qc)
}

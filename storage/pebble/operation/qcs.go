package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// InsertQuorumCertificate inserts a quorum certificate by block ID.
// Returns storage.ErrAlreadyExists if a QC has already been inserted for the block.
func InsertQuorumCertificate(qc *flow.QuorumCertificate) func(pebble.Writer) error {
	return insert(makePrefix(codeBlockIDToQuorumCertificate, qc.BlockID), qc)
}

// RetrieveQuorumCertificate retrieves a quorum certificate by blockID.
// Returns storage.ErrNotFound if no QC is stored for the block.
func RetrieveQuorumCertificate(blockID flow.Identifier, qc *flow.QuorumCertificate) func(pebble.Reader) error {
	return retrieve(makePrefix(codeBlockIDToQuorumCertificate, blockID), qc)
}

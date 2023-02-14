package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertQuorumCertificate inserts a quorum certificate by block ID.
func InsertQuorumCertificate(qc *flow.QuorumCertificate) func(*badger.Txn) error {
	return insert(makePrefix(codeQuorumCertificate, qc.BlockID), qc)
}

// RetrieveQuorumCertificate retrieves a quorum certificate by blockID.
func RetrieveQuorumCertificate(blockID flow.Identifier, qc *flow.QuorumCertificate) func(*badger.Txn) error {
	return retrieve(makePrefix(codeQuorumCertificate, blockID), qc)
}

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertRootQuorumCertificate inserts the root quorum certificate for the
// local blockchain state. The root quorum certificate certifies the root
// block and is used to bootstrap HotStuff.
//
// Only the root quorum certificate must be explicitly stored in this way!
// All other quorum certificates are implicitly included in the child of
// block they certify in the ParentSigs field.
func InsertRootQuorumCertificate(qc *flow.QuorumCertificate) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		return insert(makePrefix(codeRootQuorumCertificate), qc)(txn)
	}
}

func RetrieveRootQuorumCertificate(qc *flow.QuorumCertificate) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		return retrieve(makePrefix(codeRootQuorumCertificate), qc)(txn)
	}
}

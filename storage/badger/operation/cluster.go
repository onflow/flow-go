package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// This file implements storage functions for chain state book-keeping of
// collection node cluster consensus. In contrast to the corresponding functions
// for regular consensus, these functions include the cluster ID in order to
// support storing multiple chains, for example during epoch switchover.

// IndexClusterBlockHeight inserts a block number to block ID mapping for
// the given cluster.
func IndexClusterBlockHeight(clusterID string, number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedCluster, clusterID, number), blockID)
}

// LookupClusterBlockHeight retrieves a block ID by number for the given cluster
func LookupClusterBlockHeight(clusterID string, number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedCluster, clusterID, number), blockID)
}

// InsertClusterFinalizedHeight inserts the finalized boundary for the given cluster.
func InsertClusterFinalizedHeight(clusterID string, number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeClusterHeight, clusterID), number)
}

// UpdateClusterFinalizedHeight updates the finalized boundary for the given cluster.
func UpdateClusterFinalizedHeight(clusterID string, number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeClusterHeight, clusterID), number)
}

// RetrieveClusterFinalizedHeight retrieves the finalized boundary for the given cluster.
func RetrieveClusterFinalizedHeight(clusterID string, number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeClusterHeight, clusterID), number)
}

// IndexCollectionReference inserts the reference block ID for a cluster
// block payload (ie. collection) keyed by the cluster block ID
func IndexCollectionReference(clusterBlockID, refID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeCollectionReference, clusterBlockID), refID)
}

// LookupCollectionReference looks up the reference block ID for a cluster
// block payload (ie. collection) keyed by the cluster block ID.
func LookupCollectionReference(clusterBlockID flow.Identifier, refID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollectionReference, clusterBlockID), refID)
}

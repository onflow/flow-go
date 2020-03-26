package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// This file implements storage functions for chain state book-keeping of
// collection node cluster consensus. In contrast to the corresponding functions
// for regular consensus, these functions include the cluster ID in order to
// support storing multiple chains, for example during epoch switchover.

// InsertNumberForCluster inserts a block number to block ID mapping for
// the given cluster.
func InsertNumberForCluster(clusterID string, number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeNumber, clusterID, number), blockID)
}

// RetrieveNumberForCluster retrieves a block ID by number for the given cluster
func RetrieveNumberForCluster(clusterID string, number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeNumber, clusterID, number), blockID)
}

// InsertBoundaryForCluster inserts the finalized boundary for the given cluster.
func InsertBoundaryForCluster(clusterID string, number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeBoundary, clusterID), number)
}

// UpdateBoundaryForCluster updates the finalized boundary for the given cluster.
func UpdateBoundaryForCluster(clusterID string, number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeBoundary, clusterID), number)
}

// RetrieveBoundaryForCluster retrieves the finalized boundary for the given cluster.
func RetrieveBoundaryForCluster(clusterID string, number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBoundary, clusterID), number)
}

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// This file implements storage functions for chain state book-keeping of
// collection node cluster consensus. In contrast to the corresponding functions
// for regular consensus, these functions include the cluster ID in order to
// support storing multiple chains, for example during epoch switchover.

// IndexClusterBlockHeight inserts a block number to block ID mapping for
// the given cluster.
func IndexClusterBlockHeight(clusterID flow.ChainID, number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedCluster, clusterID, number), blockID)
}

// LookupClusterBlockHeight retrieves a block ID by number for the given cluster
func LookupClusterBlockHeight(clusterID flow.ChainID, number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedCluster, clusterID, number), blockID)
}

// InsertClusterFinalizedHeight inserts the finalized boundary for the given cluster.
func InsertClusterFinalizedHeight(clusterID flow.ChainID, number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeClusterHeight, clusterID), number)
}

// UpdateClusterFinalizedHeight updates the finalized boundary for the given cluster.
func UpdateClusterFinalizedHeight(clusterID flow.ChainID, number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeClusterHeight, clusterID), number)
}

// RetrieveClusterFinalizedHeight retrieves the finalized boundary for the given cluster.
func RetrieveClusterFinalizedHeight(clusterID flow.ChainID, number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeClusterHeight, clusterID), number)
}

// IndexReferenceBlockByClusterBlock inserts the reference block ID for the given
// cluster block ID. While each cluster block specifies a reference block in its
// payload, we maintain this additional lookup for performance reasons.
func IndexReferenceBlockByClusterBlock(clusterBlockID, refID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeClusterBlockToRefBlock, clusterBlockID), refID)
}

// LookupReferenceBlockByClusterBlock looks up the reference block ID for the given
// cluster block ID. While each cluster block specifies a reference block in its
// payload, we maintain this additional lookup for performance reasons.
func LookupReferenceBlockByClusterBlock(clusterBlockID flow.Identifier, refID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeClusterBlockToRefBlock, clusterBlockID), refID)
}

// IndexClusterBlockByReferenceHeight indexes a cluster block ID by its reference
// block height. The cluster block ID is included in the key for more efficient
// traversal. Only finalized cluster blocks should be included in this index.
// The key looks like: <prefix 0:1><ref_height 1:9><cluster_block_id 9:41>
func IndexClusterBlockByReferenceHeight(refHeight uint64, clusterBlockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeRefHeightToClusterBlock, refHeight, clusterBlockID), nil)
}

// LookupClusterBlocksByReferenceHeightRange traverses the ref_height->cluster_block
// index and returns any finalized cluster blocks which have a reference block with
// height in the given range. This is used to avoid including duplicate transaction
// when building or validating a new collection.
func LookupClusterBlocksByReferenceHeightRange(start, end uint64, clusterBlockIDs *[]flow.Identifier) func(*badger.Txn) error {
	startPrefix := makePrefix(codeRefHeightToClusterBlock, start)
	endPrefix := makePrefix(codeRefHeightToClusterBlock, end)

	return iterate(startPrefix, endPrefix, func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			clusterBlockIDBytes := key[9:]
			var clusterBlockID flow.Identifier
			copy(clusterBlockID[:], clusterBlockIDBytes)
			*clusterBlockIDs = append(*clusterBlockIDs, clusterBlockID)

			// the info we need is stored in the key, never process the value
			return false
		}
		return check, nil, nil
	}, withPrefetchValuesFalse)
}

// ClusterBlocksByReferenceHeightIndexExists checks whether the cluster block by reference
// height exists.
//
// TODO this method should not be included in the master branch.
func ClusterBlocksByReferenceHeightIndexExists(exists *bool) func(*badger.Txn) error {
	prefix := makePrefix(codeRefHeightToClusterBlock)
	return func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := tx.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); {
			*exists = true
			return nil
		}
		return nil
	}
}

// BatchIndexClusterBlockByReferenceHeight is IndexReferenceBlockByClusterBlock
// for use in a write batch.
//
// TODO this method should not be included in the master branch.
func BatchIndexClusterBlockByReferenceHeight(refHeight uint64, clusterBlockID flow.Identifier) func(*badger.WriteBatch) error {
	return batchInsert(makePrefix(codeRefHeightToClusterBlock, refHeight, clusterBlockID), nil)
}

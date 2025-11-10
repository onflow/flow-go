package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// ClusterForTransaction determines the Collection Node cluster responsible for the given transaction.
// Transactions specify a finalized reference block, which defines their reference epoch.
// Each transaction is assigned to one cluster in their reference epoch, based on the transaction ID/hash.
// See [flow/ClusterList.ByTxID] for the definition of this mapping.
// This function returns the transaction's reference epoch and the assigned cluster within that epoch.
//
// Error returns:
//   - [state.ErrUnknownSnapshotReference] if the transaction has a ReferenceBlockID unknown by this node
//   - exception in case of unexpected failure
func ClusterForTransaction(state protocol.State, tx *flow.TransactionBody) (protocol.CommittedEpoch, flow.IdentitySkeletonList, error) {
	refSnapshot := state.AtBlockID(tx.ReferenceBlockID)
	// using the transaction's reference block, determine which cluster we're in.
	// if we don't know the reference block, we will fail when attempting to query the epoch.
	refEpoch, err := refSnapshot.Epochs().Current()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get current epoch for reference block: %w", err)
	}
	clusters, err := refEpoch.Clustering()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get clusters for reference epoch: %w", err)
	}
	txCluster, ok := clusters.ByTxID(tx.ID())
	if !ok {
		return nil, nil, fmt.Errorf("could not get cluster responsible for tx: %x", tx.ID())
	}
	return refEpoch, txCluster, nil
}

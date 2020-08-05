package protocol

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
)

// ChainIDForCluster returns the canonical chain ID for a collection node cluster.
// TODO remove
func ChainIDForCluster(cluster flow.IdentityList) flow.ChainID {
	return flow.ChainID(cluster.Fingerprint().String())
}

// ClusterAssignments returns the assignments for clusters based on the given
// number and the provided collector identities.
// TODO remove or move to unittest
func ClusterAssignments(num uint, collectors flow.IdentityList) flow.AssignmentList {

	// double-check we only have collectors
	collectors = collectors.Filter(filter.HasRole(flow.RoleCollection))

	// order the identities by node ID
	sort.Slice(collectors, func(i, j int) bool {
		return order.ByNodeIDAsc(collectors[i], collectors[j])
	})

	// create the desired number of clusters and assign nodes
	var assignments flow.AssignmentList
	for i, collector := range collectors {
		index := uint(i) % num
		assignments[index] = append(assignments[index], collector.NodeID)
	}

	return assignments
}

// CanonicalClusterRootBlock returns the canonical root block for the given
// cluster in the given epoch.
func CanonicalClusterRootBlock(epoch uint64, participants flow.IdentityList) *cluster.Block {

	chainID := fmt.Sprintf("cluster-%d-%s", epoch, participants.Fingerprint())
	payload := cluster.EmptyPayload(flow.ZeroID)
	header := &flow.Header{
		ChainID:        flow.ChainID(chainID),
		ParentID:       flow.ZeroID,
		Height:         0,
		PayloadHash:    payload.Hash(),
		Timestamp:      flow.GenesisTime,
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	return &cluster.Block{
		Header:  header,
		Payload: &payload,
	}
}

package flow

import (
	"fmt"
	"math/big"
)

// AssignmentList is a list of identifier lists. Each list of identifiers lists the
// identities that are part of the given cluster.
type AssignmentList []IdentifierList

// ClusterList is a list of identity lists. Each `IdentityList` represents the
// nodes assigned to a specific cluster.
type ClusterList []IdentitySkeletonList

func (al AssignmentList) EqualTo(other AssignmentList) bool {
	if len(al) != len(other) {
		return false
	}
	for i, a := range al {
		if len(a) != len(other[i]) {
			return false
		}
		for j, identifier := range a {
			if identifier != other[i][j] {
				return false
			}
		}
	}
	return true
}

// Assignments returns the assignment list for a cluster.
func (cl ClusterList) Assignments() AssignmentList {
	assignments := make(AssignmentList, 0, len(cl))
	for _, cluster := range cl {
		assignment := make([]Identifier, 0, len(cluster))
		for _, collector := range cluster {
			assignment = append(assignment, collector.NodeID)
		}
		assignments = append(assignments, assignment)
	}
	return assignments
}

// NewClusterList creates a new cluster list based on the given cluster assignment
// and the provided list of identities.
func NewClusterList(assignments AssignmentList, collectors IdentitySkeletonList) (ClusterList, error) {

	// build a lookup for all the identities by node identifier
	lookup := make(map[Identifier]*IdentitySkeleton)
	for _, collector := range collectors {
		_, ok := lookup[collector.NodeID]
		if ok {
			return nil, fmt.Errorf("duplicate collector in list %v", collector.NodeID)
		}
		lookup[collector.NodeID] = collector
	}

	// replicate the identifier list but use identities instead
	clusters := make(ClusterList, 0, len(assignments))
	for _, participants := range assignments {
		cluster := make(IdentitySkeletonList, 0, len(participants))
		for _, participantID := range participants {
			participant, found := lookup[participantID]
			if !found {
				return nil, fmt.Errorf("could not find collector identity (%x)", participantID)
			}
			cluster = append(cluster, participant)
			delete(lookup, participantID)
		}
		clusters = append(clusters, cluster)
	}

	// check that every collector was assigned
	if len(lookup) != 0 {
		return nil, fmt.Errorf("missing collector assignments (%s)", lookup)
	}

	return clusters, nil
}

// ByIndex retrieves the list of identities that are part of the given cluster.
func (cl ClusterList) ByIndex(index uint) (IdentitySkeletonList, bool) {
	if index >= uint(len(cl)) {
		return nil, false
	}
	return cl[int(index)], true
}

// ByTxID selects the cluster that should receive the transaction with the given
// transaction ID.
//
// For evenly distributed transaction IDs, this will evenly distribute
// transactions between clusters.
func (cl ClusterList) ByTxID(txID Identifier) (IdentitySkeletonList, bool) {
	bigTxID := new(big.Int).SetBytes(txID[:])
	bigIndex := new(big.Int).Mod(bigTxID, big.NewInt(int64(len(cl))))
	return cl.ByIndex(uint(bigIndex.Uint64()))
}

// ByNodeID select the cluster that the node with the given ID is part of.
//
// Nodes will be divided into equally sized clusters as far as possible.
// The last return value will indicate if the look up was successful
func (cl ClusterList) ByNodeID(nodeID Identifier) (IdentitySkeletonList, uint, bool) {
	for index, cluster := range cl {
		for _, participant := range cluster {
			if participant.NodeID == nodeID {
				return cluster, uint(index), true
			}
		}
	}
	return nil, 0, false
}

// IndexOf returns the index of the given cluster.
func (cl ClusterList) IndexOf(cluster IdentitySkeletonList) (uint, bool) {
	clusterFingerprint := cluster.ID()
	for index, other := range cl {
		if other.ID() == clusterFingerprint {
			return uint(index), true
		}
	}
	return 0, false
}

package factory

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// NewClusterList creates a new cluster list based on the given cluster assignment and the provided list of identities.
// The implementation enforces the following protocol rules and errors in case they are violated:
//
//	(a) input `collectors` only contains collector nodes with positive weight
//	(b) collectors have unique node IDs
//	(c) each collector is assigned exactly to one cluster and is only listed once within that cluster
//
// Furthermore, for each cluster (i.e. element in `assignments`) we enforce:
//
//	(d) cluster contains at least one collector (i.e. is not empty)
//	(e) cluster is composed of known nodes, i.e. for each nodeID in `assignments` an IdentitySkeleton is given in `collectors`
//	(f) cluster assignment lists the nodes in canonical ordering
//
// The caller must ensure each assignment contains identities ordered in canonical order, so that
// each cluster in the returned cluster list is ordered in canonical order as well. If not,
// an error will be returned.
// This is a side-effect-free function. Any error return indicates that the input violate protocol rules.
func NewClusterList(assignments flow.AssignmentList, collectors flow.IdentitySkeletonList) (flow.ClusterList, error) {
	// build a lookup for all the identities by node identifier
	lookup := collectors.Lookup()
	for _, collector := range collectors { // enforce (a): `collectors` only contains collector nodes with positive weight
		if collector.Role != flow.RoleCollection {
			return nil, fmt.Errorf("node %v is not a collector", collector.NodeID)
		}
		if collector.InitialWeight == 0 {
			return nil, fmt.Errorf("node %v has zero weight", collector.NodeID)
		}
		lookup[collector.NodeID] = collector
	}
	if len(lookup) != len(collectors) { // enforce (b): collectors have unique node IDs
		return nil, fmt.Errorf("duplicate collector in list")
	}

	// assignments only contains the NodeIDs for each cluster. In the following, we substitute them with the respective IdentitySkeletons.
	clusters := make(flow.ClusterList, 0, len(assignments))
	for i, participants := range assignments {
		cluster := make(flow.IdentitySkeletonList, 0, len(participants))
		if len(participants) == 0 { // enforce (d): each cluster contains at least one collector (i.e. is not empty)
			return nil, fmt.Errorf("particpants in assignment list is empty, cluster index %v", i)
		}

		prev := participants[0] // for checking that cluster participants are listed in canonical order
		for i, participantID := range participants {
			participant, found := lookup[participantID] // enforce (e): for each nodeID in assignments an IdentitySkeleton is given in `collectors`
			if !found {
				return nil, fmt.Errorf("could not find collector identity (%x)", participantID)
			}
			cluster = append(cluster, participant)
			delete(lookup, participantID) // enforce (c) part 1: reject repeated assignment of the same node

			if i > 0 { // enforce (f): canonical ordering
				if !flow.IsIdentifierCanonical(prev, participantID) {
					return nil, fmt.Errorf("the assignments is not sorted in canonical order in cluster index %v, prev %v, next %v",
						i, prev, participantID)
				}
			}
			prev = participantID
		}

		clusters = append(clusters, cluster)
	}

	if len(lookup) != 0 { // enforce (c) part 2: every collector was assigned
		return nil, fmt.Errorf("missing collector assignments (%s)", lookup)
	}

	return clusters, nil
}

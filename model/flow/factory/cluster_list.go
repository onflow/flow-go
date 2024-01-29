package factory

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// NewClusterList creates a new cluster list based on the given cluster assignment
// and the provided list of identities.
//
// The caller must ensure the following prerequisites:
//   - each assignment contains identities ordered in canonical order
//   - every collector has a unique NodeID, i.e. there are no two elements in `collectors` with the same NodeID
//
// These prerequisites ensures that each cluster in the returned cluster list is ordered in canonical order as well.
// This function checks that the prerequisites are satisfied and errors otherwise.
func NewClusterList(assignments flow.AssignmentList, collectors flow.IdentityList) (flow.ClusterList, error) {

	// build a lookup for all the identities by node identifier
	lookup := make(map[flow.Identifier]*flow.Identity)
	for _, collector := range collectors {
		lookup[collector.NodeID] = collector
	}
	if len(lookup) != len(collectors) {
		return nil, fmt.Errorf("duplicate collector in list")
	}

	// replicate the identifier list but use identities instead
	clusters := make(flow.ClusterList, 0, len(assignments))
	for i, participants := range assignments {
		cluster := make(flow.IdentityList, 0, len(participants))
		if len(participants) == 0 {
			return nil, fmt.Errorf("participants in assignment list is empty, cluster index %v", i)
		}

		// Check assignments is sorted in canonical order
		prev := participants[0]

		for i, participantID := range participants {
			participant, found := lookup[participantID]
			if !found {
				return nil, fmt.Errorf("could not find collector identity (%x)", participantID)
			}
			cluster = append(cluster, participant)
			delete(lookup, participantID)

			if i > 0 {
				if !flow.IsIdentifierCanonical(prev, participantID) {
					return nil, fmt.Errorf("the assignments is not sorted in canonical order or there are duplicates in cluster index %v, prev %v, next %v",
						i, prev, participantID)
				}
			}
			prev = participantID
		}

		clusters = append(clusters, cluster)
	}

	// check that every collector was assigned
	if len(lookup) != 0 {
		return nil, fmt.Errorf("missing collector assignments (%s)", lookup)
	}

	return clusters, nil
}

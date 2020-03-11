package protocol

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// ClusterFor returns the cluster that the node with given ID belongs to.
func ClusterFor(sn Snapshot, id flow.Identifier) (flow.IdentityList, error) {

	clusters, err := sn.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, err := clusters.ByNodeID(id)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster for node: %w", err)
	}
	participants, err := sn.Identities(filter.In(cluster))
	if err != nil {
		return nil, fmt.Errorf("could not get nodes in cluster: %w", err)
	}

	return participants, nil
}

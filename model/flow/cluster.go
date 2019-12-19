package flow

// ClusterID is the unique ID of a given collection node cluster.
type ClusterID int

// ClusterList is a set of clusters, keyed by ID. Each cluster must contain at
// least one node. All nodes are collection nodes.
type ClusterList []IdentityList

// Size returns the number of clusters.
func (cl ClusterList) Size() int {
	return len(cl)
}

// Get returns the cluster with the given ID. Returns an empty cluster if
// no cluster with the given ID exists.
func (cl ClusterList) Get(id ClusterID) IdentityList {

	if id < 0 || int(id) >= len(cl) {
		return IdentityList{}
	}

	return cl[id]
}

// ClusterIDFor returns the cluster ID for the given node ID, if the node exists
// in any cluster. Otherwise returns -1.
//
// TODO this is O(n), optimize if # of collection nodes gets large
func (cl ClusterList) ClusterIDFor(nodeID Identifier) ClusterID {

	for clusterID, cluster := range cl {
		for _, identity := range cluster {
			if identity.NodeID == nodeID {
				return ClusterID(clusterID)
			}
		}
	}

	return -1
}

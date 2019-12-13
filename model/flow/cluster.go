package flow

// ClusterID is the unique ID of a given collection node cluster.
type ClusterID uint

// ClusterList is a set of clusters, keyed by ID. Each cluster must contain at
// least one node. All nodes are collection nodes.
type ClusterList []IdentityList

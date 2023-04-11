package module

// ClusterIDSProvider provides an interface to the current canonical cluster ID of the cluster an LN is assigned to.
type ClusterIDSProvider interface {
	// ActiveClusterIDS returns the active canonical cluster ID's for the assigned collection clusters.
	ActiveClusterIDS() ([]string, error)
}

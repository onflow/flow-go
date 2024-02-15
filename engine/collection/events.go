package collection

import "github.com/onflow/flow-go/model/flow"

// EngineEvents set of methods used to distribute and consume events related to collection node engine components.
type EngineEvents interface {
	ClusterEvents
}

// ClusterEvents defines methods used to disseminate cluster ID update events.
// Cluster IDs are updated when a new set of epoch components start and the old set of epoch components stops.
// A new list of cluster IDs will be assigned when the new set of epoch components are started, and the old set of cluster
// IDs are removed when the current set of epoch components are stopped. The implementation must be concurrency safe and  non-blocking.
type ClusterEvents interface {
	// ActiveClustersChanged is called when a new cluster ID update event is distributed.
	// Any error encountered on consuming event must handle internally by the implementation.
	// The implementation must be concurrency safe, but can be blocking.
	ActiveClustersChanged(flow.ChainIDList)
}

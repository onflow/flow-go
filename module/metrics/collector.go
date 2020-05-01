package metrics

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/trace"
)

// Collector implements the module.Metrics interface.
// It provides methods for collecting metrics data for monitoring purposes.
//
// This implementation takes a SHORT CUT:
// [CONTEXT] Collector implements the module.Metrics interface. Hence, it needs to provide consumers
// for the metrics for ALL nodes. For Collector Nodes, we have the need to separate the metrics by
// collector cluster. We do this by adding a label to identify the cluster (by its ID). However,
// this implies that the Collector implementation used for the collector Nodes must know the
// node's respective cluster ID. In contrast, for other node types, there is no Cluster ID.
// [CURRENT SOLUTION]
// We use the same Collector implementation for all nodes, with a variable to for the clusterID.
// Only collector nodes will generate metrics, which require the cluster ID as Label.
// For all other node types, the clusterID is not used and not set.
// ToDo: Clean up Tech Dept: Split up the module.Metrics interface and make one dedicated
//       Interface for each node type. Then, implementations can be (partially) separated
//       such that only the implementation for the Collector Nodes need to contain a clusterID.
type Collector struct {
	tracer    trace.Tracer
	clusterID string
}

// NewCollector instantiates a new metrics Collector. NOT SUITABLE for COLLECTOR NODES!
func NewCollector(log zerolog.Logger) (*Collector, error) {
	tracer, err := trace.NewTracer(log)
	if err != nil {
		return nil, err
	}

	return &Collector{
		tracer:    tracer,
		clusterID: "",
	}, nil
}

// NewCollector instantiates a new metrics suitable ONLY FOR COLLECTOR NODES.
func NewClusterCollector(log zerolog.Logger, clusterID string) (*Collector, error) {
	tracer, err := trace.NewTracer(log)
	if err != nil {
		return nil, err
	}

	return &Collector{
		tracer:    tracer,
		clusterID: clusterID,
	}, nil
}

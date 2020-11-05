package protocol

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// Cluster represents the detailed information for a particular cluster,
// for a given epoch. This information represents the INITIAL state of the
// cluster, as defined by the Epoch Preparation Protocol. It DOES NOT take
// into account state changes over the course of the epoch (ie. slashing).
type Cluster interface {

	// Index returns the index for this cluster.
	Index() uint

	// ID returns ID for the cluster.
	ID() flow.Identifier

	// EpochCounter returns the epoch counter for this cluster.
	EpochCounter() uint64

	// Members returns the initial set of collector nodes in this cluster.
	Members() flow.IdentifierList

	// RootBlock returns the root block for this cluster.
	RootBlock() *cluster.Block

	// RootQC returns the quorum certificate for this cluster.
	RootQC() *flow.QuorumCertificate
}

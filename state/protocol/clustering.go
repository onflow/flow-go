package protocol

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ClusterInformation interface {
	// Index returns the index for this cluster
	Index() uint32

	// Index returns the Epoch counter for this cluster
	EpochCounter() uint64

	// Members returns the Identities of the collector nodes in this cluster
	Members() flow.IdentityList

	// RootBlock returns the root block for this cluster
	RootBlock() *cluster.Block

	// RootQC returns the quorum certificate for for this cluster
	RootQC() *flow.QuorumCertificate
}

package protocol

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ClusterInformation interface {
	Index() uint32
	Counter() (uint64, error)

	// ClusterRootBlock returns the root block for this cluster
	Members() flow.IdentityList

	// RootBlock returns the root block for this cluster
	RootBlock() *cluster.Block

	// RootQC returns the quorum certificate for for this cluster
	RootQC() *flow.QuorumCertificate
}

package encodable

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// Cluster is the encoding format for protocol.Cluster
type Cluster struct {
	Index     uint
	Counter   uint64
	Members   flow.IdentityList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}

package encodable

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type Cluster struct {
	Index     uint
	Counter   uint64
	Members   flow.IdentityList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}

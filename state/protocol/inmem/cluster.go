package inmem

import (
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type Cluster struct {
	enc EncodableCluster
}

var _ protocol.Cluster = (*Cluster)(nil)

func (c Cluster) Index() uint                        { return c.enc.Index }
func (c Cluster) ChainID() flow.ChainID              { return c.enc.RootBlock.Header.ChainID }
func (c Cluster) EpochCounter() uint64               { return c.enc.Counter }
func (c Cluster) Members() flow.IdentitySkeletonList { return c.enc.Members }
func (c Cluster) RootBlock() *clustermodel.Block     { return c.enc.RootBlock }
func (c Cluster) RootQC() *flow.QuorumCertificate    { return c.enc.RootQC }

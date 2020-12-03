package inmem

import (
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

type Cluster struct {
	encodable.Cluster
}

func (c Cluster) Index() uint                     { return c.Cluster.Index }
func (c Cluster) ChainID() flow.ChainID           { return c.Cluster.RootBlock.Header.ChainID }
func (c Cluster) EpochCounter() uint64            { return c.Cluster.Counter }
func (c Cluster) Members() flow.IdentityList      { return c.Cluster.Members }
func (c Cluster) RootBlock() *clustermodel.Block  { return c.Cluster.RootBlock }
func (c Cluster) RootQC() *flow.QuorumCertificate { return c.RootQC() }

package inmem

import (
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type Cluster struct {
	enc EncodableCluster
}

func (c Cluster) Index() uint                     { return c.enc.Index }
func (c Cluster) ChainID() flow.ChainID           { return c.enc.RootBlock.Header.ChainID }
func (c Cluster) EpochCounter() uint64            { return c.enc.Counter }
func (c Cluster) Members() flow.IdentityList      { return c.enc.Members }
func (c Cluster) RootBlock() *clustermodel.Block  { return c.enc.RootBlock }
func (c Cluster) RootQC() *flow.QuorumCertificate { return c.enc.RootQC }

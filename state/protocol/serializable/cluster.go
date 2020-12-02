package serializable

import (
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type cluster struct {
	index     uint
	counter   uint64
	members   flow.IdentityList
	rootBlock *clustermodel.Block
	rootQC    *flow.QuorumCertificate
}

func (c cluster) Index() uint                     { return c.index }
func (c cluster) ChainID() flow.ChainID           { return c.rootBlock.Header.ChainID }
func (c cluster) EpochCounter() uint64            { return c.counter }
func (c cluster) Members() flow.IdentityList      { return c.members }
func (c cluster) RootBlock() *clustermodel.Block  { return c.rootBlock }
func (c cluster) RootQC() *flow.QuorumCertificate { return c.rootQC }

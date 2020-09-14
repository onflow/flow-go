package badger

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Cluster implements interface protocol.Cluster
// It is a container with all information about a specific cluster.
type Cluster struct {
	index     uint
	counter   uint64
	members   flow.IdentityList
	rootBlock *cluster.Block
	rootQC    *flow.QuorumCertificate
}

func (c Cluster) Index() uint                     { return c.index }
func (c Cluster) ChainID() flow.ChainID           { return c.rootBlock.Header.ChainID }
func (c Cluster) EpochCounter() uint64            { return c.counter }
func (c Cluster) Members() flow.IdentityList      { return c.members }
func (c Cluster) RootBlock() *cluster.Block       { return c.rootBlock }
func (c Cluster) RootQC() *flow.QuorumCertificate { return c.rootQC }

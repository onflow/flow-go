package badger

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ClusterInformation implements interface protocol.ClusterInformation
// It is a container with all information about a specific cluster.
type ClusterInformation struct {
	index     uint32
	counter   uint64
	members   flow.IdentityList
	rootBlock *cluster.Block
	rootQC    *flow.QuorumCertificate
}

func (c ClusterInformation) Index() uint32                   { return c.index }
func (c ClusterInformation) EpochCounter() uint64            { return c.counter }
func (c ClusterInformation) Members() flow.IdentityList      { return c.members }
func (c ClusterInformation) RootBlock() *cluster.Block       { return c.rootBlock }
func (c ClusterInformation) RootQC() *flow.QuorumCertificate { return c.rootQC }

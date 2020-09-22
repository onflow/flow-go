package libp2p

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
)

type collectionTopology struct {
	nodeID flow.Identifier
	state protocol.State
}

func (c collectionTopology) Subset(idList flow.IdentityList, size int, seed string) (map[flow.Identifier]flow.Identity, error) {
	clusterList, err := c.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, err
	}

	_, myClusterIndex, found := clusterList.ByNodeID(c.nodeID)
	if !found {
		return nil, fmt.Errorf("failed to find cluster for node ID %s", c.nodeID.String())
	}

	myCluster, err := c.state.Final().Epochs().Current().Cluster(myClusterIndex)
	if err != nil {
		return nil, err
	}

	myCluster.Members().

}


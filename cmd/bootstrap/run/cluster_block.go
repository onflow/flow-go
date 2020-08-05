package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
)

func GenerateRootClusterBlocks(epoch uint64, clusters flow.ClusterList) []*cluster.Block {
	clusterBlocks := make([]*cluster.Block, len(clusters))
	for i := range clusterBlocks {
		cluster, ok := clusters.ByIndex(uint(i))
		if !ok {
			panic(fmt.Sprintf("failed to get cluster by index: %v", i))
		}

		clusterBlocks[i] = protocol.CanonicalClusterRootBlock(epoch, cluster)
	}
	return clusterBlocks
}

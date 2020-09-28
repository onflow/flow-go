package run

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	clusterstate "github.com/onflow/flow-go/state/cluster"
)

func GenerateRootClusterBlocks(epoch uint64, clusters flow.ClusterList) []*cluster.Block {
	clusterBlocks := make([]*cluster.Block, len(clusters))
	for i := range clusterBlocks {
		cluster, ok := clusters.ByIndex(uint(i))
		if !ok {
			panic(fmt.Sprintf("failed to get cluster by index: %v", i))
		}

		clusterBlocks[i] = clusterstate.CanonicalRootBlock(epoch, cluster)
	}
	return clusterBlocks
}

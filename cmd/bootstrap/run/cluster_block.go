package run

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	clusterstate "github.com/onflow/flow-go/state/cluster"
)

func GenerateRootClusterBlocks(epoch uint64, clusters flow.ClusterList) []*cluster.UnsignedBlock {
	clusterBlocks := make([]*cluster.UnsignedBlock, len(clusters))
	for i := range clusterBlocks {
		cluster, ok := clusters.ByIndex(uint(i))
		if !ok {
			panic(fmt.Sprintf("failed to get cluster by index: %v", i))
		}

		rootBlock, err := clusterstate.CanonicalRootBlock(epoch, cluster)
		if err != nil {
			panic(fmt.Errorf("failed to get canonical root block: %w", err))
		}
		clusterBlocks[i] = rootBlock
	}
	return clusterBlocks
}

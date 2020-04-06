package cmd

import (
	"fmt"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/order"
)

func computeCollectorClusters(stakingNodes []model.NodeInfo) *flow.ClusterList {

	identities := flow.IdentityList{}
	for _, node := range stakingNodes {
		if node.Role != flow.RoleCollection {
			continue
		}

		identities = append(identities, node.Identity())
	}

	// order the identities by node ID
	sort.Slice(identities, func(i, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	// create the desired number of clusters and assign nodes
	clusters := flow.NewClusterList(uint(flagCollectionClusters))
	for i, identity := range identities {
		index := uint(i) % uint(flagCollectionClusters)
		clusters.Add(index, identity)
	}

	return clusters
}

func constructGenesisBlocksForCollectorClusters(clusters *flow.ClusterList) []cluster.Block {
	clusterBlocks := run.GenerateGenesisClusterBlocks(clusters)

	for i, clusterBlock := range clusterBlocks {
		writeJSON(fmt.Sprintf(model.FilenameGenesisClusterBlock, i), clusterBlock)
	}

	return clusterBlocks
}

func constructGenesisQCsForCollectorClusters(clusterList *flow.ClusterList, nodeInfos []model.NodeInfo, block flow.Block, clusterBlocks []cluster.Block) {

	if len(clusterBlocks) != clusterList.Size() {
		log.Fatal().Int("len(clusterBlocks)", len(clusterBlocks)).Int("clusterList.Size()", clusterList.Size()).
			Msg("number of clusters needs to equal number of cluster blocks")
	}

	for i := 0; i < clusterList.Size(); i++ {
		signers := filterClusterSigners(nodeInfos)

		qc, err := run.GenerateClusterGenesisQC(signers, &block, &clusterBlocks[i])
		if err != nil {
			log.Fatal().Err(err).Int("cluster index", i).Msg("generating collector cluster genesis QC failed")
		}

		writeJSON(fmt.Sprintf(model.FilenameGenesisClusterQC, i), qc)
	}
}

// Filters a list of nodes to omit nodes for which we don't have private
// staking key information (ie. partner nodes).
func filterClusterSigners(nodeInfos []model.NodeInfo) []model.NodeInfo {

	var filtered []model.NodeInfo
	for _, node := range nodeInfos {
		if node.Type() == model.NodeInfoTypePrivate {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

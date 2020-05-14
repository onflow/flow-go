package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructGenesisBlocksForCollectorClusters(clusters *flow.ClusterList) []*cluster.Block {
	clusterBlocks := run.GenerateGenesisClusterBlocks(clusters)

	for _, clusterBlock := range clusterBlocks {
		// cluster ID is equivalent to chain ID
		clusterID := clusterBlock.Header.ChainID
		writeJSON(fmt.Sprintf(model.PathGenesisClusterBlock, clusterID), clusterBlock)
	}

	return clusterBlocks
}

func constructGenesisQCsForCollectorClusters(clusterList *flow.ClusterList, nodeInfos []model.NodeInfo, block *flow.Block, clusterBlocks []*cluster.Block) {

	if len(clusterBlocks) != clusterList.Size() {
		log.Fatal().Int("len(clusterBlocks)", len(clusterBlocks)).Int("clusterList.Size()", clusterList.Size()).
			Msg("number of clusters needs to equal number of cluster blocks")
	}

	for i, cluster := range clusterList.All() {
		signers := filterClusterSigners(cluster, nodeInfos)

		qc, err := run.GenerateClusterGenesisQC(signers, block, clusterBlocks[i])
		if err != nil {
			log.Fatal().Err(err).Int("cluster index", i).Msg("generating collector cluster genesis QC failed")
		}

		// cluster ID is equivalent to chain ID
		clusterID := clusterBlocks[i].Header.ChainID
		writeJSON(fmt.Sprintf(model.PathGenesisClusterQC, clusterID), qc)
	}
}

// Filters a list of nodes to include only nodes that will sign the QC for the
// given cluster. The resulting list of nodes is only nodes that are in the
// given cluster AND are not partner nodes (ie. we have the private keys).
func filterClusterSigners(cluster flow.IdentityList, nodeInfos []model.NodeInfo) []model.NodeInfo {

	var filtered []model.NodeInfo
	for _, node := range nodeInfos {
		_, isInCluster := cluster.ByNodeID(node.NodeID)
		isNotPartner := node.Type() == model.NodeInfoTypePrivate

		if isInCluster && isNotPartner {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

package cmd

import (
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// Checks constraints about the number of partner and internal nodes.
// Internal nodes must comprise >2/3 of each collector cluster.
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {

	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)
	all := append(partners, internals...)

	// ensure all nodes of the same role have equal stake/weight
	for _, role := range flow.Roles() {
		withRole := all.Filter(filter.HasRole(role))
		expectedStake := withRole[0].Stake
		for _, node := range withRole {
			if node.Stake != expectedStake {
				log.Fatal().Msgf(
					"will not bootstrap configuration with non-equal stakes\n"+
						"found nodes with role %s and stake1=%d, stake2=%d",
					role, expectedStake, node.Stake)
			}
		}
	}

	// check collection committee Byzantine threshold for each cluster
	// for checking Byzantine constraints, the seed doesn't matter
	_, clusters := constructClusterAssignment(partnerNodes, internalNodes, 0)
	partnerCOLCount := uint(0)
	internalCOLCount := uint(0)
	for _, cluster := range clusters {
		clusterPartnerCount := uint(0)
		clusterInternalCount := uint(0)
		for _, node := range cluster {
			if _, exists := partners.ByNodeID(node.NodeID); exists {
				clusterPartnerCount++
			}
			if _, exists := internals.ByNodeID(node.NodeID); exists {
				clusterInternalCount++
			}
			if clusterInternalCount <= clusterPartnerCount*2 {
				log.Fatal().Msgf(
					"will not bootstrap configuration without Byzantine majority within cluster: "+
						"(partners=%d, internals=%d, min_internals=%d)",
					clusterPartnerCount, clusterInternalCount, clusterPartnerCount*2+1)
			}
		}
		partnerCOLCount += clusterPartnerCount
		internalCOLCount += clusterInternalCount
	}

	// ensure we have enough total collectors
	totalCollectors := partnerCOLCount + internalCOLCount
	if totalCollectors < flagCollectionClusters*minNodesPerCluster {
		log.Fatal().Msgf(
			"will not bootstrap configuration with insufficient # of collectors for cluster count: "+
				"(total_collectors=%d, clusters=%d, min_total_collectors=%d)",
			totalCollectors, flagCollectionClusters, flagCollectionClusters*minNodesPerCluster)
	}
}

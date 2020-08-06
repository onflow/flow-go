package cmd

import (
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// Checks constraints about the number of partner and internal nodes.
// Internal nodes must comprise >2/3 of consensus committee.
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

	// check consensus committee Byzantine threshold
	partnerCount := partners.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	internalCount := internals.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	if internalCount <= partnerCount*2 {
		log.Fatal().Msgf(
			"will not bootstrap configuration without Byzantine majority of consensus nodes: "+
				"(partners=%d, internals=%d, min_internals=%d)",
			partnerCount, internalCount, partnerCount*2+1)
	}

	// check collection committee Byzantine threshold for each cluster
	_, clusters := constructClusterAssignment(partnerNodes, internalNodes)
	for _, cluster := range clusters {
		partnerCount = 0
		internalCount = 0
		for _, node := range cluster {
			if _, exists := partners.ByNodeID(node.NodeID); exists {
				partnerCount++
			}
			if _, exists := internals.ByNodeID(node.NodeID); exists {
				internalCount++
			}
			if internalCount <= partnerCount*2 {
				log.Fatal().Msgf(
					"will not bootstrap configuration without Byzantine majority of cluster: "+
						"(partners=%d, internals=%d, min_internals=%d)",
					partnerCount, internalCount, partnerCount*2+1)
			}
		}
	}

	// ensure we have enough total collectors
	totalCollectors := partnerCount + internalCount
	if totalCollectors < flagCollectionClusters*minNodesPerCluster {
		log.Fatal().Msgf(
			"will not bootstrap configuration with insufficient # of collectors for cluster count: "+
				"(total_collectors=%d, clusters=%d, min_total_collectors=%d)",
			totalCollectors, flagCollectionClusters, flagCollectionClusters*minNodesPerCluster)
	}
}

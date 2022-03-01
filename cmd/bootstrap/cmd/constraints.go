package cmd

import (
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ensureUniformNodeWeightsPerRole verifies that the following condition is satisfied for each role R:
// * all node with role R must have the same weight
func ensureUniformNodeWeightsPerRole(allNodes flow.IdentityList) {
	// ensure all nodes of the same role have equal weight
	for _, role := range flow.Roles() {
		withRole := allNodes.Filter(filter.HasRole(role))
		expectedWeight := withRole[0].Weight
		for _, node := range withRole {
			if node.Weight != expectedWeight {
				log.Fatal().Msgf(
					"will not bootstrap configuration with non-equal weights\n"+
						"found nodes with role %s and weight1=%d, weight2=%d",
					role, expectedWeight, node.Weight)
			}
		}
	}
}

// Checks constraints about the number of partner and internal nodes.
// * Internal nodes must comprise >2/3 of each collector cluster.
// * for all roles R:
//   all node with role R must have the same weight
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {
	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)
	all := append(partners, internals...)

	ensureUniformNodeWeightsPerRole(all)

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

package cmd

import (
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ensureUniformNodeWeightsPerRole verifies that the following condition is satisfied for each role R:
// * all node with role R must have the same weight
// The function assumes there is at least one node for each role.
func ensureUniformNodeWeightsPerRole(allNodes flow.IdentityList) {
	// ensure all nodes of the same role have equal weight
	for _, role := range flow.Roles() {
		withRole := allNodes.Filter(filter.HasRole[flow.Identity](role))
		// each role has at least one node so it's safe to access withRole[0]
		expectedWeight := withRole[0].InitialWeight
		for _, node := range withRole {
			if node.InitialWeight != expectedWeight {
				log.Fatal().Msgf(
					"will not bootstrap configuration with non-equal weights\n"+
						"found nodes with role %s and weight1=%d, weight2=%d",
					role, expectedWeight, node.InitialWeight)
			}
		}
	}
}

// Checks constraints about the weights of partner and internal nodes.
//   - for all roles R:
//     all node with role R must have the same weight
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {
	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)
	all := append(partners, internals...)

	ensureUniformNodeWeightsPerRole(all)
}

// Check the ratio of internal/partner nodes in each cluster. The identities
// in each cluster do not matter for this check.
// Internal nodes must comprise >1/3 of each collector cluster.
func checkClusterConstraint(clusters flow.ClusterList, partnersInfo []model.NodeInfo, internalsInfo []model.NodeInfo) bool {
	partners := model.ToIdentityList(partnersInfo)
	internals := model.ToIdentityList(internalsInfo)
	for _, cluster := range clusters {
		var clusterPartnerCount, clusterInternalCount int
		for _, node := range cluster {
			if _, exists := partners.ByNodeID(node.NodeID); exists {
				clusterPartnerCount++
			}
			if _, exists := internals.ByNodeID(node.NodeID); exists {
				clusterInternalCount++
			}
		}
		if clusterInternalCount*2 <= clusterPartnerCount {
			return false
		}
	}
	return true
}

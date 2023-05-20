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
//   - Internal nodes must comprise >2/3 of each collector cluster.
//   - for all roles R:
//     all node with role R must have the same weight
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {
	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)
	all := append(partners, internals...)

	ensureUniformNodeWeightsPerRole(all)

	// check collection committee threshold of internal nodes in each cluster
	// although the assignmment is non-deterministic, the number of internal/partner
	// nodes in each cluster is deterministic. The following check is only a sanity
	// check about the number of internal/partner nodes in each cluster. The identites
	// in each cluster do not matter for this sanity check.
	_, clusters, err := constructClusterAssignment(partnerNodes, internalNodes)
	if err != nil {
		log.Fatal().Msgf("can't bootstrap because the cluster assignment failed: %s", err)
	}

	for i, cluster := range clusters {
		var clusterPartnerCount, clusterInternalCount int
		for _, node := range cluster {
			if _, exists := partners.ByNodeID(node.NodeID); exists {
				clusterPartnerCount++
			}
			if _, exists := internals.ByNodeID(node.NodeID); exists {
				clusterInternalCount++
			}
		}
		if clusterInternalCount <= clusterPartnerCount*2 {
			log.Fatal().Msgf(
				"can't bootstrap because cluster %d doesn't have enough internal nodes: "+
					"(partners=%d, internals=%d, min_internals=%d)",
				i, clusterPartnerCount, clusterInternalCount, clusterPartnerCount*2+1)
		}
	}
}

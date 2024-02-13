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

// Checks constraints about the number of partner and internal nodes.
//   - Internal nodes must comprise >2/3 of each collector cluster.
//   - for all roles R:
//     all node with role R must have the same weight
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {
	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)
	all := append(partners, internals...)

	ensureUniformNodeWeightsPerRole(all)
}

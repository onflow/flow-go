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

	// check consensus committee Byzantine threshold
	partnerCount := partners.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	internalCount := internals.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	if partnerCount*2 >= internalCount {
		log.Fatal().Msgf(
			"will not bootstrap configuration without Byzantine majority of consensus nodes: "+
				"(partners=%d, internals=%d, min_internals=%d)",
			partnerCount, internalCount, partnerCount*2+1)
	}

	// check collection committee Byzantine threshold
	partnerCount = partners.Filter(filter.HasRole(flow.RoleCollection)).Count()
	internalCount = internals.Filter(filter.HasRole(flow.RoleCollection)).Count()
	if internalCount < partnerCount*2+flagCollectionClusters {
		log.Fatal().Msgf(
			"will not bootstrap configuration without Byzantine majority of collection nodes: "+
				"(partner_nodes=%d, internal_nodes=%d, min_internal_nodes=%d)",
			partnerCount, internalCount, partnerCount*2+flagCollectionClusters)
	}

	// ensure we have enough total collectors
	totalCollectors := partnerCount + internalCount
	if totalCollectors < flagCollectionClusters*minNodesPerCluster {
		log.Fatal().Msgf(
			"will not bootstrap configuration with insufficient # of collectors for cluster count: "+
				"(total_collectors=%d, clusters=%d, min_total_collectors=%d)",
			totalCollectors, flagCollectionClusters, minNodesPerCluster)
	}
}

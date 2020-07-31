package cmd

import (
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// Checks constraints about the number of partner and internal nodes.
// Internal nodes must comprise >2/3 of consensus committee.
// Internal nodes must comprise >2/3 of each collector cluster.
// NOTE: Since we assign the initial collector clustering as part of
// generating the EpochSetup event, we only need to ensure that internal
// nodes comprise >2/3 of all collectors.
func checkConstraints(partnerNodes, internalNodes []model.NodeInfo) {

	partners := model.ToIdentityList(partnerNodes)
	internals := model.ToIdentityList(internalNodes)

	// check consensus committee
	parterCount := partners.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	internalCount := internals.Filter(filter.HasRole(flow.RoleConsensus)).Count()
	if parterCount*2 >= internalCount {
		log.Fatal().Msgf(
			"will not bootstrap configuration without Byzantine majority of consensus nodes: "+
				"(partners=%d, internals=%d, min_internals=%d)",
			parterCount, internalCount, parterCount*2+1)
	}

	// check collection committee
	parterCount = partners.Filter(filter.HasRole(flow.RoleCollection)).Count()
	internalCount = internals.Filter(filter.HasRole(flow.RoleCollection)).Count()
	if parterCount*2 >= internalCount {
		log.Fatal().Msgf(
			"will not bootstrap configuration without Byzantine majority of collection nodes: "+
				"(partners=%d, internals=%d, min_internals=%d)",
			parterCount, internalCount, parterCount*2+1)
	}
}

package leader

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/indices"
	"github.com/dapperlabs/flow-go/state/protocol"
)

const EstimatedSixMonthOfViews = 15000000 // 1 sec block time * 60 secs * 60 mins * 24 hours * 30 days * 6 months

func NewSelectionForConsensus(count int, rootHeader *flow.Header, rootQC *model.QuorumCertificate, st protocol.State) (*committee.LeaderSelection, error) {
	seed, err := ReadSeed(indices.ProtocolConsensusLeaderSelection, rootHeader, rootQC, st)
	if err != nil {
		return nil, fmt.Errorf("could not read seed: %w", err)
	}

	// find all consensus nodes identities which contain the stake info
	identities, err := st.AtBlockID(rootHeader.ID()).Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("could not get consensus identities: %w", err)
	}

	selection, err := committee.ComputeLeaderSelectionFromSeed(rootHeader.View, seed, count, identities)
	if err != nil {
		return nil, fmt.Errorf("could not compute leader selection from seed: %w", err)
	}

	return selection, nil
}

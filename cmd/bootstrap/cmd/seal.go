package cmd

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee/leader"
	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participantNodes []model.NodeInfo,
	assignments flow.AssignmentList,
	clusterQCs []*hotstuff.QuorumCertificate,
	dkgData model.DKGData,
) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := model.ToIdentityList(participantNodes)
	blockID := block.ID()

	epochSetup := epoch.Setup{
		Counter:      1, // TODO flag
		FinalView:    block.Header.View + leader.EstimatedSixMonthOfViews,
		Participants: participants,
		Assignments:  assignments,
		Seed:         blockID[:],
	}

	dkgParticipants := make(map[flow.Identifier]epoch.DKGParticipant)
	dkgParticipantIdentities := participants.Filter(filter.HasRole(flow.RoleConsensus))
	for i, keyShare := range dkgData.PubKeyShares {
		identity := dkgParticipantIdentities[i]
		dkgParticipants[identity.NodeID] = epoch.DKGParticipant{
			Index:    uint(i),
			KeyShare: keyShare,
		}
	}

	epochCommit := epoch.Commit{
		Counter:         1, // TODO flag
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgParticipants,
	}

	result := run.GenerateRootResult(block, stateCommit)
	seal := run.GenerateRootSeal(result) // TODO include service events

	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
}

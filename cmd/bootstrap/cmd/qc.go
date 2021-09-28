package cmd

import (
	"fmt"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// constructRootQC constructs root QC based on root block, votes and dkg info
func constructRootQC(block *flow.Block, votes []*model.Vote, internalNodes []bootstrap.NodeInfo, signer dkg.DKGParticipantPriv, dkgData inmem.EncodableDKG) *flow.QuorumCertificate {
	participantData, err := run.GenerateQCSignerParticipantData(internalNodes, signer, dkgData)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to generate QC participant data")
	}

	qc, err := run.GenerateRootQC(block, votes, participantData)
	if err != nil {
		log.Fatal().Err(err).Msg("generating root QC failed")
	}

	return qc
}

// NOTE: allNodes must be in the same order as when generating the DKG
func constructRootVotes(block *flow.Block, allNodes, internalNodes []bootstrap.NodeInfo, dkgData dkg.DKGData) {
	participantData, err := run.GenerateQCParticipantData(allNodes, internalNodes, dkgData)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to generate QC participant data")
	}

	votes, err := run.GenerateRootBlockVotes(block, participantData)
	if err != nil {
		log.Fatal().Err(err).Msg("generating votes for root block failed")
	}

	for _, vote := range votes {
		writeJSON(fmt.Sprintf(bootstrap.PathNodeRootBlockVote, vote.SignerID), vote)
	}
}

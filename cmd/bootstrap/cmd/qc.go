package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/dkg/wrapper"
)

func constructGenesisQC(block *flow.Block, allNodes, internalNodes []model.NodeInfo, dkgData model.DKGData) {
	participantData := GenerateQCParticipantData(allNodes, internalNodes, dkgData)

	qc, err := run.GenerateGenesisQC(participantData, block)
	if err != nil {
		log.Fatal().Err(err).Msg("generating genesis QC failed")
	}

	writeJSON(model.FilenameGenesisQC, qc)
}

func GenerateQCParticipantData(allNodes, internalNodes []model.NodeInfo, dkg model.DKGData) run.ParticipantData {

	// stakingNodes can include external validators, so it can be longer than internalNodes
	if len(allNodes) < len(internalNodes) {
		log.Fatal().Int("len(stakingNodes)", len(allNodes)).Int("len(internalNodes)", len(internalNodes)).
			Msg("need at least as many staking public keys as staking private keys")
	}

	// length of DKG participants needs to match stakingNodes, since we run DKG for external and internal validators
	if len(allNodes) != len(dkg.Participants) {
		log.Fatal().Int("len(stakingNodes)", len(allNodes)).Int("len(dkg.Participants)", len(dkg.Participants)).
			Msg("need exactly the same number of staking public keys as DKG private participants")
	}

	sd := run.ParticipantData{}

	// the QC will be signed by everyone in internalNodes
	for _, node := range internalNodes {
		// find the corresponding entry in dkg
		part := findDKGParticipant(dkg, node.NodeID)

		if node.NodeID == flow.ZeroID {
			log.Fatal().Str("Address", node.Address).Msg("NodeID must not be zero")
		}

		if node.Stake == 0 {
			log.Fatal().Str("NodeID", node.NodeID.String()).Msg("Stake must not be 0")
		}

		sd.Participants = append(sd.Participants, run.Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: part.KeyShare,
		})
	}

	dkgPubData := dkg.ForHotStuff()
	sd.DKGState = wrapper.NewState(dkgPubData)

	return sd
}

func findDKGParticipant(dkg model.DKGData, nodeID flow.Identifier) model.DKGParticipant {
	for _, part := range dkg.Participants {
		if part.NodeID == nodeID {
			return part
		}
	}
	log.Fatal().Str("nodeID", nodeID.String()).Msg("could not find nodeID in public DKG data")
	return model.DKGParticipant{}
}

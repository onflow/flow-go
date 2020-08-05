package cmd

import (
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootQC(block *flow.Block, allNodes, internalNodes []model.NodeInfo, dkgData model.DKGData) {
	participantData := GenerateQCParticipantData(allNodes, internalNodes, dkgData)

	qc, err := run.GenerateRootQC(block, participantData)
	if err != nil {
		log.Fatal().Err(err).Msg("generating root QC failed")
	}

	writeJSON(model.PathRootQC, qc)
}

func GenerateQCParticipantData(allNodes, internalNodes []model.NodeInfo, dkgData model.DKGData) run.ParticipantData {

	// stakingNodes can include external validators, so it can be longer than internalNodes
	if len(allNodes) < len(internalNodes) {
		log.Fatal().Int("len(stakingNodes)", len(allNodes)).Int("len(internalNodes)", len(internalNodes)).
			Msg("need at least as many staking public keys as staking private keys")
	}

	// length of DKG participants needs to match stakingNodes, since we run DKG for external and internal validators
	if len(allNodes) != len(dkgData.PrivKeyShares) {
		log.Fatal().Int("len(stakingNodes)", len(allNodes)).Int("len(dkgData.PrivKeyShares)", len(dkgData.PrivKeyShares)).
			Msg("need exactly the same number of staking public keys as DKG private participants")
	}

	qcData := run.ParticipantData{}

	participantLookup := make(map[flow.Identifier]epoch.DKGParticipant)

	// the QC will be signed by everyone in internalNodes
	for i, node := range internalNodes {
		// assign a node to a DGKdata entry, using the canonical ordering
		participantLookup[node.NodeID] = epoch.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}

		if node.NodeID == flow.ZeroID {
			log.Fatal().Str("Address", node.Address).Msg("NodeID must not be zero")
		}

		if node.Stake == 0 {
			log.Fatal().Str("NodeID", node.NodeID.String()).Msg("Stake must not be 0")
		}

		qcData.Participants = append(qcData.Participants, run.Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: dkgData.PrivKeyShares[i],
		})
	}

	for i := len(internalNodes); i < len(allNodes); i++ {
		// assign a node to a DGKdata entry, using the canonical ordering
		node := allNodes[i]
		participantLookup[node.NodeID] = epoch.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}
	}

	qcData.Lookup = participantLookup
	qcData.GroupKey = dkgData.PubGroupKey

	return qcData
}

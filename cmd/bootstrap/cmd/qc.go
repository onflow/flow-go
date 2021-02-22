package cmd

import (
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootQC(block *flow.Block, allNodes, internalNodes []model.NodeInfo, dkgData dkg.DKGData) {
	participantData, err := run.GenerateQCParticipantData(allNodes, internalNodes, dkgData)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to generate QC participant data")
	}

	qc, err := run.GenerateRootQC(block, participantData)
	if err != nil {
		log.Fatal().Err(err).Msg("generating root QC failed")
	}

	writeJSON(model.PathRootQC, qc)
}

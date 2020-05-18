package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
)

func runDKG(nodes []model.NodeInfo) model.DKGData {
	n := len(nodes)

	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	dkgData, err := run.RunDKG(n, generateRandomSeeds(n))
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	for i, participant := range dkgData.Participants {
		nodeID := nodes[i].NodeID
		dkgData.Participants[i].NodeID = nodeID

		log.Debug().Int("i", i).Str("nodeId", nodeID.String()).Msg("assembling dkg data")

		writeJSON(fmt.Sprintf(model.PathRandomBeaconPriv, nodeID), participant.Private())
	}

	writeJSON(model.PathDKGDataPub, dkgData.Public())

	return dkgData
}

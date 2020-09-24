package cmd

import (
	"fmt"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
)

func runDKG(nodes []model.NodeInfo) model.DKGData {
	n := len(nodes)

	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	var dkgData model.DKGData
	var err error
	if flagFastKG {
		dkgData, err = run.RunFastKG(n, generateRandomSeed())
	} else {
		dkgData, err = run.RunDKG(n, generateRandomSeeds(n))
	}
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	for i, privKey := range dkgData.PrivKeyShares {
		nodeID := nodes[i].NodeID

		log.Debug().Int("i", i).Str("nodeId", nodeID.String()).Msg("assembling dkg data")

		encKey := encodable.RandomBeaconPrivKey{PrivateKey: privKey}
		privParticpant := model.DKGParticipantPriv{
			NodeID:              nodeID,
			RandomBeaconPrivKey: encKey,
			GroupIndex:          i,
		}

		writeJSON(fmt.Sprintf(model.PathRandomBeaconPriv, nodeID), privParticpant)
	}

	return dkgData
}

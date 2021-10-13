package cmd

import (
	"fmt"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

func runDKG(nodes []model.NodeInfo) dkg.DKGData {
	n := len(nodes)

	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	var dkgData dkg.DKGData
	var err error
	if flagFastKG {
		dkgData, err = run.RunFastKG(n, flagBootstrapRandomSeed)
	} else {
		dkgData, err = run.RunDKG(n, GenerateRandomSeeds(n))
	}
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	pubKeyShares := make([]encodable.RandomBeaconPubKey, 0, len(dkgData.PubKeyShares))
	for _, pubKey := range dkgData.PubKeyShares {
		pubKeyShares = append(pubKeyShares, encodable.RandomBeaconPubKey{PublicKey: pubKey})
	}

	privKeyShares := make([]encodable.RandomBeaconPrivKey, 0, len(dkgData.PrivKeyShares))
	for i, privKey := range dkgData.PrivKeyShares {
		nodeID := nodes[i].NodeID

		encKey := encodable.RandomBeaconPrivKey{PrivateKey: privKey}
		privParticpant := dkg.DKGParticipantPriv{
			NodeID:              nodeID,
			RandomBeaconPrivKey: encKey,
			GroupIndex:          i,
		}

		privKeyShares = append(privKeyShares, encKey)

		writeJSON(fmt.Sprintf(model.PathRandomBeaconPriv, nodeID), privParticpant)
	}

	// write full DKG info that will be used to construct QC
	writeJSON(model.PathRootDKGData, inmem.EncodableFullDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: dkgData.PubGroupKey,
		},
		PubKeyShares:  pubKeyShares,
		PrivKeyShares: privKeyShares,
	})

	return dkgData
}

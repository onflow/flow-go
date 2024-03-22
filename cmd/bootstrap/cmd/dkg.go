package cmd

import (
	"fmt"

	"github.com/onflow/crypto"

	bootstrapDKG "github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

func runBeaconKG(nodes []model.NodeInfo) dkg.DKGData {
	n := len(nodes)

	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	var dkgData dkg.DKGData
	var err error
	dkgData, err = bootstrapDKG.RandomBeaconKG(n, GenerateRandomSeed(crypto.KeyGenSeedMinLen))
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
		privKeyShares = append(privKeyShares, encKey)

		err = common.WriteJSON(fmt.Sprintf(model.PathRandomBeaconPriv, nodeID), flagOutdir, encKey)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to write json")
		}
	}

	// write full DKG info that will be used to construct QC
	err = common.WriteJSON(model.PathRootDKGData, flagOutdir, inmem.EncodableFullDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: dkgData.PubGroupKey,
		},
		PubKeyShares:  pubKeyShares,
		PrivKeyShares: privKeyShares,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}

	return dkgData
}

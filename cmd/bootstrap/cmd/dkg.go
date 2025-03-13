package cmd

import (
	"fmt"

	"github.com/onflow/crypto"

	bootstrapDKG "github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

func runBeaconKG(nodes []model.NodeInfo) (dkg.ThresholdKeySet, flow.DKGIndexMap) {
	n := len(nodes)
	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	randomBeaconData, err := bootstrapDKG.RandomBeaconKG(n, GenerateRandomSeed(crypto.KeyGenSeedMinLen))
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	encodableParticipants := make([]inmem.ThresholdParticipant, 0, len(nodes))
	for i, privKey := range randomBeaconData.PrivKeyShares {
		nodeID := nodes[i].NodeID

		encKey := encodable.RandomBeaconPrivKey{PrivateKey: privKey}
		encodableParticipants = append(encodableParticipants, inmem.ThresholdParticipant{
			PrivKeyShare: encKey,
			PubKeyShare:  encodable.RandomBeaconPubKey{PublicKey: randomBeaconData.PubKeyShares[i]},
			NodeID:       nodeID,
		})

		err = common.WriteJSON(fmt.Sprintf(model.PathRandomBeaconPriv, nodeID), flagOutdir, encKey)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to write json")
		}
		log.Info().Msgf("wrote file %s/%s", flagOutdir, fmt.Sprintf(model.PathRandomBeaconPriv, nodeID))
	}

	indexMap := make(flow.DKGIndexMap, len(nodes))
	for i, node := range nodes {
		indexMap[node.NodeID] = i
	}

	// write full DKG info that will be used to construct QC
	err = common.WriteJSON(model.PathRootDKGData, flagOutdir, inmem.ThresholdKeySet{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: randomBeaconData.PubGroupKey,
		},
		Participants: encodableParticipants,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}
	log.Info().Msgf("wrote file %s/%s", flagOutdir, model.PathRootDKGData)

	return randomBeaconData, indexMap
}

package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
)

type DKGParticipantPriv struct {
	NodeID              flow.Identifier
	RandomBeaconPrivKey EncodableRandomBeaconPrivKey
	GroupIndex          int
}

type DKGParticipantPub struct {
	NodeID             flow.Identifier
	RandomBeaconPubKey EncodableRandomBeaconPubKey
	GroupIndex         int
}

type DKGDataPriv struct {
	PubGroupKey  EncodableRandomBeaconPubKey
	Participants []DKGParticipantPriv
}

type DKGDataPub struct {
	PubGroupKey  EncodableRandomBeaconPubKey
	Participants []DKGParticipantPub
}

func runDKG(nodes []NodeInfoPub) (DKGDataPub, DKGDataPriv) {
	n := len(nodes)

	log.Info().Msgf("read %v node infos for DKG", n)

	log.Debug().Msgf("will run DKG")
	dkgData, err := run.RunDKG(n, generateRandomSeeds(n))
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	dkgDataPriv := DKGDataPriv{
		Participants: make([]DKGParticipantPriv, 0, n),
		PubGroupKey:  EncodableRandomBeaconPubKey{dkgData.PubGroupKey},
	}
	dkgDataPub := DKGDataPub{
		Participants: make([]DKGParticipantPub, 0, n),
		PubGroupKey:  EncodableRandomBeaconPubKey{dkgData.PubGroupKey},
	}
	for i, nodeInfo := range nodes {
		log.Debug().Int("i", i).Str("nodeId", nodeInfo.NodeID.String()).Msg("assembling dkg data")
		partPriv, partPub := assembleDKGParticipant(nodeInfo, dkgData.Participants[i])
		dkgDataPriv.Participants = append(dkgDataPriv.Participants, partPriv)
		dkgDataPub.Participants = append(dkgDataPub.Participants, partPub)
		writeJSON(fmt.Sprintf(FilenameRandomBeaconPriv, partPriv.NodeID), partPriv)
	}

	writeJSON(FilenameDKGDataPub, dkgDataPub)

	return dkgDataPub, dkgDataPriv
}

func assembleDKGParticipant(info NodeInfoPub, part run.DKGParticipant) (DKGParticipantPriv, DKGParticipantPub) {
	pub := pubKeyToString(part.Priv.PublicKey())

	log.Debug().
		Str("pub", pub).
		Msg("participant's encoded public DKG key")

	partPriv := DKGParticipantPriv{
		NodeID:              info.NodeID,
		RandomBeaconPrivKey: EncodableRandomBeaconPrivKey{part.Priv},
		GroupIndex:          part.GroupIndex,
	}

	partPub := DKGParticipantPub{
		NodeID:             info.NodeID,
		RandomBeaconPubKey: EncodableRandomBeaconPubKey{part.Priv.PublicKey()},
		GroupIndex:         part.GroupIndex,
	}

	return partPriv, partPub
}

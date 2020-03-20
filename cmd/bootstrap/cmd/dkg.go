package cmd

import (
	"fmt"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
)

type DKGParticipantPriv struct {
	NodeID              flow.Identifier     `yaml:"nodeId"`
	RandomBeaconPrivKey RandomBeaconPrivKey `yaml:"randomBeaconPrivKey"`
	GroupIndex          int                 `yaml:"groupIndex"`
}

type DKGParticipantPub struct {
	NodeID             flow.Identifier    `yaml:"nodeId"`
	RandomBeaconPubKey RandomBeaconPubKey `yaml:"randomBeaconPubKey"`
	GroupIndex         int                `yaml:"groupIndex"`
}

type DKGDataPriv struct {
	PubGroupKey  RandomBeaconPubKey   `yaml:"pubGroupKey"`
	Participants []DKGParticipantPriv `yaml:"participants"`
}

type DKGDataPub struct {
	PubGroupKey  RandomBeaconPubKey  `yaml:"pubGroupKey"`
	Participants []DKGParticipantPub `yaml:"participants"`
}

func runDKG(nodes []NodeInfoPub) (DKGDataPub, DKGDataPriv) {
	n := len(nodes)

	log.Info().Msgf("read %v node infos", n)

	log.Debug().Msgf("will run DKG")
	dkgData, err := run.RunDKG(n, generateRandomSeeds(n))
	if err != nil {
		log.Fatal().Err(err).Msg("error running DKG")
	}
	log.Info().Msgf("finished running DKG")

	dkgDataPriv := DKGDataPriv{
		Participants: make([]DKGParticipantPriv, 0, n),
		PubGroupKey:  RandomBeaconPubKey{dkgData.PubGroupKey},
	}
	dkgDataPub := DKGDataPub{
		Participants: make([]DKGParticipantPub, 0, n),
		PubGroupKey:  RandomBeaconPubKey{dkgData.PubGroupKey},
	}
	for i, nodeInfo := range nodes {
		log.Debug().Int("i", i).Str("nodeId", nodeInfo.NodeID.String()).Msg("assembling dkg data")
		partPriv, partPub := assembleDKGParticipant(nodeInfo, dkgData.Participants[i])
		dkgDataPriv.Participants = append(dkgDataPriv.Participants, partPriv)
		dkgDataPub.Participants = append(dkgDataPub.Participants, partPub)
		writeYaml(fmt.Sprintf("%v.random-beacon.priv.yml", partPriv.NodeID), partPriv)
	}

	writeYaml("dkg-data.pub.yml", dkgDataPub)

	return dkgDataPub, dkgDataPriv
}

func assembleDKGParticipant(info NodeInfoPub, part run.DKGParticipant) (DKGParticipantPriv, DKGParticipantPub) {
	pub := pubKeyToString(part.Pub)

	log.Debug().
		Str("pub", pub).
		Msg("participant's encoded public DKG key")

	partPriv := DKGParticipantPriv{
		NodeID:              info.NodeID,
		RandomBeaconPrivKey: RandomBeaconPrivKey{part.Priv},
		GroupIndex:          part.GroupIndex,
	}

	partPub := DKGParticipantPub{
		NodeID:             info.NodeID,
		RandomBeaconPubKey: RandomBeaconPubKey{part.Pub},
		GroupIndex:         part.GroupIndex,
	}

	return partPriv, partPub
}

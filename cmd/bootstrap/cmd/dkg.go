package cmd

import (
	"fmt"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var stakingInfosFile string

type DKGParticipantPriv struct {
	NodeID              string `yaml:"nodeId"`
	RandomBeaconPrivKey string `yaml:"randomBeaconPrivKey"`
	GroupIndex          int    `yaml:"groupIndex"`
}

type DKGParticipantPub struct {
	NodeID             string `yaml:"nodeId"`
	RandomBeaconPubKey string `yaml:"randomBeaconPubKey"`
	GroupIndex         int    `yaml:"groupIndex"`
}

type DKGDataPriv struct {
	PubGroupKey  string               `yaml:"pubGroupKey"`
	Participants []DKGParticipantPriv `yaml:"participants"`
}

type DKGDataPub struct {
	PubGroupKey  string              `yaml:"pubGroupKey"`
	Participants []DKGParticipantPub `yaml:"participants"`
}

// dkgCmd represents the dkg command
var dkgCmd = &cobra.Command{
	Use:   "dkg",
	Short: "Generate random beacon keys for all nodes through DKG",
	Run: func(cmd *cobra.Command, args []string) {
		var nodeInfos []NodeInfoPub
		readYaml(stakingInfosFile, &nodeInfos)
		nodes := len(nodeInfos)

		log.Info().Msgf("read %v node infos", nodes)

		log.Debug().Msgf("will run DKG")
		dkgData, err := run.RunDKG(nodes, generateRandomSeeds(nodes))
		if err != nil {
			log.Fatal().Err(err).Msg("eror running DKG")
		}
		log.Info().Msgf("finished running DKG")

		dkgDataPriv := DKGDataPriv{
			Participants: make([]DKGParticipantPriv, 0, nodes),
			PubGroupKey:  pubKeyToString(dkgData.PubGroupKey),
		}
		dkgDataPub := DKGDataPub{
			Participants: make([]DKGParticipantPub, 0, nodes),
			PubGroupKey:  pubKeyToString(dkgData.PubGroupKey),
		}
		for i, nodeInfo := range nodeInfos {
			log.Debug().Int("i", i).Str("nodeId", nodeInfo.NodeID).Msg("assembling dkg data")
			partPriv, partPub := assembleDKGParticipant(nodeInfo, dkgData.Participants[i])
			dkgDataPriv.Participants = append(dkgDataPriv.Participants, partPriv)
			dkgDataPub.Participants = append(dkgDataPub.Participants, partPub)
			writeYaml(fmt.Sprintf("%v.random-beacon.priv.yml", partPriv.NodeID), partPriv)
		}

		writeYaml("dkg-data.pub.yml", dkgDataPub)
		writeYaml("dkg-data.priv.yml", dkgDataPriv)
	},
}

func init() {
	rootCmd.AddCommand(dkgCmd)

	dkgCmd.Flags().StringVarP(&stakingInfosFile, "staking-infos", "s", "", "Path to a yml file containing staking information for all genesis nodes [required]")
	dkgCmd.MarkFlagRequired("staking-infos")
}

func assembleDKGParticipant(info NodeInfoPub, part run.DKGParticipant) (DKGParticipantPriv, DKGParticipantPub) {
	pub := pubKeyToString(part.Pub)
	priv := privKeyToString(part.Priv)

	log.Debug().
		Str("pub", pub).
		Msg("participant's encoded public DKG key")

	partPriv := DKGParticipantPriv{
		NodeID:              info.NodeID,
		RandomBeaconPrivKey: priv,
		GroupIndex:          part.GroupIndex,
	}

	partPub := DKGParticipantPub{
		NodeID:             info.NodeID,
		RandomBeaconPubKey: pub,
		GroupIndex:         part.GroupIndex,
	}

	return partPriv, partPub
}

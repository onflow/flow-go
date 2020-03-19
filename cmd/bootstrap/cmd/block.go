package cmd

import (
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var stateCommitmentFile string
var nodeInfosFile string
var dkgDataPubFile string

// blockCmd represents the block command
var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Construct genesis block",
	Run: func(cmd *cobra.Command, args []string) {
		var sc StateCommitment
		readYaml(stateCommitmentFile, &sc)
		var nodeInfos []NodeInfoPub
		readYaml(nodeInfosFile, &nodeInfos)
		var dkgDataPub DKGDataPub
		readYaml(dkgDataPubFile, &dkgDataPub)

		if len(nodeInfos) != len(dkgDataPub.Participants) {
			log.Fatal().Int("len(nodeInfos)", len(nodeInfos)).
				Int("len(dkgDataPub.Participants", len(dkgDataPub.Participants)).
				Msg("node info and public DKG data participants do not have equal lenght")
		}

		seal := run.GenerateRootSeal(sc.StateCommitment)
		identityList := generateIdentityList(nodeInfos, dkgDataPub)
		block := run.GenerateRootBlock(identityList, seal)

		writeYaml("genesis-block.yml", block)
	},
}

func init() {
	rootCmd.AddCommand(blockCmd)

	blockCmd.Flags().StringVarP(&stateCommitmentFile, "state-commitment", "s", "",
		"Path to a yml file containing a state-commitment [required]")
	blockCmd.MarkFlagRequired("state-commitment")
	blockCmd.Flags().StringVarP(&nodeInfosFile, "node-infos", "n", "",
		"Path to a yml file containing staking information for all genesis nodes [required]")
	blockCmd.MarkFlagRequired("node-infos")
	blockCmd.Flags().StringVarP(&dkgDataPubFile, "dkg-data-pub", "d", "",
		"Path to a yml file containing public dkg data for all genesis nodes [required]")
	blockCmd.MarkFlagRequired("dkg-data-pub")
}

func generateIdentityList(infos []NodeInfoPub, dkgDataPub DKGDataPub) flow.IdentityList {
	var list flow.IdentityList
	list = make([]*flow.Identity, 0, len(infos))

	for i, info := range infos {
		part := dkgDataPub.Participants[i]
		if info.NodeID != part.NodeID {
			log.Fatal().Int("i", i).Str("node info NodeID", info.NodeID.String()).
				Str("DKG data participant NodeID", part.NodeID.String()).
				Msg("NodeID in node info and public DKG data participants does not match")
		}

		list = append(list, &flow.Identity{
			NodeID:             info.NodeID,
			Address:            info.Address,
			Role:               info.Role,
			Stake:              info.Stake,
			StakingPubKey:      info.StakingPubKey,
			RandomBeaconPubKey: part.RandomBeaconPubKey,
			NetworkPubKey:      info.NetworkPubKey,
		})
	}

	return list
}

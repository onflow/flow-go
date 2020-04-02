package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	flagRole        string
	flagAddress     string
	flagNetworkSeed []byte
	flagStakingSeed []byte
)

type PartnerNodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey model.EncodableNetworkPrivKey
	StakingPrivKey model.EncodableStakingPrivKey
}

type PartnerNodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	NetworkPubKey model.EncodableNetworkPubKey
	StakingPubKey model.EncodableStakingPubKey
}

// keyCmd represents the key command
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generate networking and staking keys for a partner node and write them to files",
	Run: func(cmd *cobra.Command, args []string) {
		// validate inputs
		role := validateRole(flagRole)
		networkSeed := validateSeed(flagNetworkSeed)
		stakingSeed := validateSeed(flagStakingSeed)

		log.Debug().Msg("will generate networking key")
		networkKeys, err := run.GenerateNetworkingKeys(1, [][]byte{networkSeed})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking key")
		}
		log.Info().Msg("generated networking key")

		log.Debug().Msg("will generate staking key")
		stakingKeys, err := run.GenerateStakingKeys(1, [][]byte{stakingSeed})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate staking key")
		}
		log.Info().Msg("generated staking key")

		log.Debug().Str("address", flagAddress).Msg("assembling node information")
		nodeInfo := assembleNodeInfo(model.NodeConfig{role, flagAddress, 0}, networkKeys[0],
			stakingKeys[0])

		//TODO is this ok to not have partner-specific models? seems they are the same
		// only difference is partnerpublic does not have stake
		writeJSON(fmt.Sprintf(model.FilenameNodeInfoPriv, nodeInfo.NodeID), nodeInfo.Private())
		writeJSON(fmt.Sprintf(model.FilenameNodeInfoPub, nodeInfo.NodeID), nodeInfo.Public())
	},
}

func init() {
	rootCmd.AddCommand(keyCmd)

	keyCmd.Flags().StringVar(&flagRole, "role", "",
		"node role (can be \"collection\", \"consensus\", \"execution\", \"verification\" or \"observation\")")
	_ = keyCmd.MarkFlagRequired("role")
	keyCmd.Flags().StringVar(&flagAddress, "address", "", "network address")
	_ = keyCmd.MarkFlagRequired("address")
	keyCmd.Flags().BytesHexVar(&flagNetworkSeed, "networking-seed", []byte{}, "networking seed")
	_ = keyCmd.MarkFlagRequired("networking-seed")
	keyCmd.Flags().BytesHexVar(&flagStakingSeed, "staking-seed", []byte{}, "staking seed")
	_ = keyCmd.MarkFlagRequired("staking-seed")
}

func validateRole(role string) flow.Role {
	parsed, err := flow.ParseRole(role)
	if err != nil {
		log.Fatal().Err(err).Msg("unsupported role, allowed values: \"collection\", \"consensus\", \"execution\", " +
			"\"verification\" or \"observation\"")
	}
	return parsed
}

func validateSeed(seed []byte) []byte {
	if len(seed) < minSeedBytes {
		log.Fatal().Int("len(seed)", len(seed)).Msgf("seed too short, needs to be at least %v bytes long", minSeedBytes)
	}
	return seed
}

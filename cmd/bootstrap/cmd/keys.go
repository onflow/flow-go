package cmd

import (
	"fmt"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var configFile string

type NodeConfig struct {
	Role    string `yaml:"role"`
	Address string `yaml:"address"`
	Stake   uint64 `yaml:"stake"`
}

type NodeInfoPriv struct {
	Role           string `yaml:"role"`
	Address        string `yaml:"address"`
	NodeID         string `yaml:"nodeId"`
	NetworkPrivKey string `yaml:"networkPrivKey"`
	StakingPrivKey string `yaml:"stakingPrivKey"`
}

type NodeInfoPub struct {
	Role          string `yaml:"role"`
	Address       string `yaml:"address"`
	NodeID        string `yaml:"nodeId"`
	NetworkPubKey string `yaml:"networkPubKey"`
	StakingPubKey string `yaml:"stakingPubKey"`
	Stake         uint64 `yaml:"stake"`
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Generate staking and networking keys for multiple nodes",
	Run: func(cmd *cobra.Command, args []string) {
		var nodeConfigs []NodeConfig
		readYaml(configFile, &nodeConfigs)
		nodes := len(nodeConfigs)
		log.Info().Msgf("read %v node configurations", nodes)

		log.Debug().Msgf("will generate %v networking keys", nodes)
		// TODO replace with user provided seeds (through flag or file)
		networkKeys, err := run.GenerateNetworkingKeys(nodes, generateRandomSeeds(nodes))
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking keys")
		}
		log.Info().Msgf("generated %v networking keys", nodes)

		log.Debug().Msgf("will generate %v staking keys", nodes)
		// TODO replace with user provided seeds (through flag or file)
		stakingKeys, err := run.GenerateStakingKeys(nodes, generateRandomSeeds(nodes))
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking keys")
		}
		log.Info().Msgf("generated %v staking keys", nodes)

		nodeInfosPub := make([]NodeInfoPub, 0, nodes)
		for i, nodeConfig := range nodeConfigs {
			log.Debug().Int("i", i).Str("address", nodeConfig.Address).Msg("assembling node information")
			nodeInfoPriv, nodeInfoPub := assembleNodeInfo(nodeConfig, networkKeys[i], stakingKeys[i])
			nodeInfosPub = append(nodeInfosPub, nodeInfoPub)
			writeYaml(fmt.Sprintf("%v.node-info.priv.yml", nodeInfoPriv.NodeID), nodeInfoPriv)
		}

		writeYaml("node-infos.pub.yml", nodeInfosPub)
	},
}

func init() {
	rootCmd.AddCommand(keysCmd)

	keysCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to a yml file containing multiple node configurations (node_role, network_address, stake) [required]")
	keysCmd.MarkFlagRequired("config")
}

func assembleNodeInfo(nodeConfig NodeConfig, networkKey, stakingKey crypto.PrivateKey) (NodeInfoPriv, NodeInfoPub) {
	networkPubKey := pubKeyToString(networkKey.PublicKey())
	networkPrivKey := privKeyToString(networkKey)
	stakingPubKey := pubKeyToString(stakingKey.PublicKey())
	stakingPrivKey := privKeyToString(stakingKey)

	log.Debug().
		Str("networkPubKey", networkPubKey).
		Str("stakingPubKey", stakingPubKey).
		Msg("encoded public staking and network keys")

	nodeInfoPriv := NodeInfoPriv{
		Role:           nodeConfig.Role,
		Address:        nodeConfig.Address,
		NodeID:         flow.MakeID(stakingPubKey).String(),
		NetworkPrivKey: networkPrivKey,
		StakingPrivKey: stakingPrivKey,
	}

	nodeInfoPub := NodeInfoPub{
		Role:          nodeConfig.Role,
		Address:       nodeConfig.Address,
		NodeID:        flow.MakeID(stakingPubKey).String(),
		NetworkPubKey: networkPubKey,
		StakingPubKey: stakingPubKey,
		Stake:         nodeConfig.Stake,
	}

	return nodeInfoPriv, nodeInfoPub
}

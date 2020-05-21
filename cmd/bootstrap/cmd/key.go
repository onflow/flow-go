package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

var (
	flagRole        string
	flagAddress     string
	flagNetworkSeed []byte
	flagStakingSeed []byte
)

// keyCmd represents the key command
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generate networking and staking keys for a partner node and write them to files",
	Run: func(cmd *cobra.Command, args []string) {
		// validate inputs
		role := validateRole(flagRole)
		validateAddressFormat(flagAddress)
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
		conf := model.NodeConfig{
			Role:    role,
			Address: flagAddress,
			Stake:   0,
		}
		nodeInfo := assembleNodeInfo(conf, networkKeys[0], stakingKeys[0])

		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			log.Fatal().Err(err).Msg("could not access private keys")
		}

		writeText(model.PathNodeId, []byte(nodeInfo.NodeID.String()))
		writeJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), private)
		writeJSON(fmt.Sprintf(model.PathNodeInfoPub, nodeInfo.NodeID), nodeInfo.Public())
	},
}

func init() {
	rootCmd.AddCommand(keyCmd)

	keyCmd.Flags().StringVar(&flagRole, "role", "",
		"node role (can be \"collection\", \"consensus\", \"execution\", \"verification\" or \"access\")")
	_ = keyCmd.MarkFlagRequired("role")
	keyCmd.Flags().StringVar(&flagAddress, "address", "", "network address")
	_ = keyCmd.MarkFlagRequired("address")
	keyCmd.Flags().BytesHexVar(&flagNetworkSeed, "networking-seed", generateRandomSeed(),
		fmt.Sprintf("hex encoded networking seed (min %v bytes)", minSeedBytes))
	keyCmd.Flags().BytesHexVar(&flagStakingSeed, "staking-seed", generateRandomSeed(),
		fmt.Sprintf("hex encoded staking seed (min %v bytes)", minSeedBytes))
}

func validateRole(role string) flow.Role {
	parsed, err := flow.ParseRole(role)
	if err != nil {
		log.Fatal().Err(err).Msg("unsupported role, allowed values: \"collection\", \"consensus\", \"execution\", " +
			"\"verification\" or \"access\"")
	}
	return parsed
}

func validateSeed(seed []byte) []byte {
	if len(seed) < minSeedBytes {
		log.Fatal().Int("len(seed)", len(seed)).Msgf("seed too short, needs to be at least %v bytes long", minSeedBytes)
	}
	return seed
}

// validateAddressFormat validates the address provided by pretty much doing what the network layer would do before
// starting the node
func validateAddressFormat(address string) {
	checkErr := func(err error) {
		if err != nil {
			log.Fatal().Err(err).Str("address", address).Msg("invalid address format.\n" +
				`Address needs to be in the format hostname:port or ip:port e.g. "flow.com:3569"`)
		}
	}

	// split address into ip/hostname and port
	ip, port, err := net.SplitHostPort(address)
	checkErr(err)

	// check that port number is indeed a number
	_, err = strconv.Atoi(port)
	checkErr(err)

	nodeAddrs := libp2p.NodeAddress{
		IP:   ip,
		Port: port,
	}

	// create a libp2p address from the ip and port
	lp2pAddr := libp2p.MultiaddressStr(nodeAddrs)
	_, err = multiaddr.NewMultiaddr(lp2pAddr)
	checkErr(err)
}

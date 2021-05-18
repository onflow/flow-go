package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

var (
	flagRole    string
	flagAddress string

	// bools
	flagNetworkKey bool
	flagStakingKey bool
	flagMachineKey bool

	// seed flags
	flagNetworkSeed []byte
	flagStakingSeed []byte
	flagMachineSeed []byte
)

// keyCmd represents the key command
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generate networking and staking keys for a partner node and write them to files",
	Run:   keyCmdRun,
}

func init() {
	rootCmd.AddCommand(keyCmd)

	// required flags
	keyCmd.Flags().StringVar(&flagRole, "role", "", "node role (can be \"collection\", \"consensus\", \"execution\", \"verification\" or \"access\")")
	_ = keyCmd.MarkFlagRequired("role")
	keyCmd.Flags().StringVar(&flagAddress, "address", "", "network address")
	_ = keyCmd.MarkFlagRequired("address")

	keyCmd.Flags().BoolVar(&flagNetworkKey, "networking", false, "generate networking key only")
	keyCmd.Flags().BoolVar(&flagNetworkKey, "staking", false, "generate staking key only")
	keyCmd.Flags().BoolVar(&flagNetworkKey, "machine", false, "generate machine key only")

	keyCmd.Flags().BytesHexVar(&flagNetworkSeed, "networking-seed", generateRandomSeed(), fmt.Sprintf("hex encoded networking seed (min %v bytes)", minSeedBytes))
	keyCmd.Flags().BytesHexVar(&flagStakingSeed, "staking-seed", generateRandomSeed(), fmt.Sprintf("hex encoded staking seed (min %v bytes)", minSeedBytes))
	keyCmd.Flags().BytesHexVar(&flagMachineSeed, "machine-seed", generateRandomSeed(), fmt.Sprintf("hex encoded machine account seed (min %v bytes)", minSeedBytes))
}

// keyCmdRun generate the node staking key, networking key and node information
func keyCmdRun(_ *cobra.Command, _ []string) {
	// validate inputs
	role := validateRole(flagRole)
	validateAddressFormat(flagAddress)

	var networkKey crypto.PrivateKey
	var stakingKey crypto.PrivateKey
	var machineKey crypto.PrivateKey

	if !flagNetworkKey && !flagStakingKey && !flagMachineKey {
		flagNetworkKey = true
		flagStakingKey = true
		flagMachineKey = true
	}

	if flagNetworkKey {
		log.Debug().Msg("will generate networking key")
		networkSeed := validateSeed(flagNetworkSeed)
		networkKeys, err := run.GenerateNetworkingKeys(1, [][]byte{networkSeed})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking key")
		}
		networkKey = networkKeys[0]
		log.Info().Msg("generated networking key")
	}

	if flagStakingKey {
		log.Debug().Msg("will generate staking key")
		stakingSeed := validateSeed(flagStakingSeed)
		stakingKeys, err := run.GenerateStakingKeys(1, [][]byte{stakingSeed})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate staking key")
		}
		stakingKey = stakingKeys[0]
		log.Info().Msg("generated staking key")
	}

	if flagMachineKey {
		log.Debug().Msg("will generate machine account key")
		machineSeed := validateSeed(flagMachineSeed)
		machineKeys, err := run.GenerateNetworkingKeys(1, [][]byte{machineSeed})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate machine account key")
		}
		machineKey = machineKeys[0]
		log.Info().Msg("generated machine account key")
	}

	log.Debug().Str("address", flagAddress).Msg("assembling node information")
	conf := model.NodeConfig{
		Role:    role,
		Address: flagAddress,
		Stake:   0,
	}
	nodeInfo := assembleNodeInfo(conf, networkKey, stakingKey, machineKey)

	// retrieve private representation of the node
	private, err := nodeInfo.Private()
	if err != nil {
		log.Fatal().Err(err).Msg("could not access private keys")
	}

	writeText(model.PathNodeID, []byte(nodeInfo.NodeID.String()))
	writeJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), private)
	writeJSON(fmt.Sprintf(model.PathNodeInfoPub, nodeInfo.NodeID), nodeInfo.Public())
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

	// create a libp2p address from the ip and port
	lp2pAddr := p2p.MultiAddressStr(ip, port)
	_, err = multiaddr.NewMultiaddr(lp2pAddr)
	checkErr(err)
}

package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

var (
	flagRole    string
	flagAddress string

	// seed flags
	flagNetworkSeed []byte
	flagStakingSeed []byte
	flagMachineSeed []byte
)

// keyCmd represents the key command
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generate networking, staking, and machine account keys for a partner node and write them to files.",
	Run:   keyCmdRun,
}

func init() {
	rootCmd.AddCommand(keyCmd)

	// required flags
	keyCmd.Flags().StringVar(&flagRole, "role", "", "node role (can be \"collection\", \"consensus\", \"execution\", \"verification\" or \"access\")")
	cmd.MarkFlagRequired(keyCmd, "role")
	keyCmd.Flags().StringVar(&flagAddress, "address", "", "network address")
	cmd.MarkFlagRequired(keyCmd, "address")

	keyCmd.Flags().BytesHexVar(
		&flagNetworkSeed,
		"networking-seed",
		[]byte{},
		fmt.Sprintf("hex encoded networking seed (min %d bytes)", crypto.KeyGenSeedMinLenECDSAP256))
	keyCmd.Flags().BytesHexVar(
		&flagStakingSeed,
		"staking-seed",
		[]byte{},
		fmt.Sprintf("hex encoded staking seed (min %d bytes)", crypto.KeyGenSeedMinLenBLSBLS12381))
	keyCmd.Flags().BytesHexVar(
		&flagMachineSeed,
		"machine-seed",
		[]byte{},
		fmt.Sprintf("hex encoded machine account seed (min %d bytes)", crypto.KeyGenSeedMinLenECDSAP256))
}

// keyCmdRun generate the node staking key, networking key and node information
func keyCmdRun(_ *cobra.Command, _ []string) {

	// generate private key seeds if not specified via flag
	if len(flagNetworkSeed) == 0 {
		flagNetworkSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLenECDSAP256)
	}
	if len(flagStakingSeed) == 0 {
		flagStakingSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLenBLSBLS12381)
	}
	if len(flagMachineSeed) == 0 {
		flagMachineSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLenECDSAP256)
	}

	// validate inputs
	role := validateRole(flagRole)
	validateAddressFormat(flagAddress)

	// generate staking and network keys
	networkKey, stakingKey, secretsDBKey, err := generateKeys()
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate staking or network keys")
	}

	log.Debug().Str("address", flagAddress).Msg("assembling node information")
	conf := model.NodeConfig{
		Role:    role,
		Address: flagAddress,
		Weight:  0,
	}
	nodeInfo := assembleNodeInfo(conf, networkKey, stakingKey)

	private, err := nodeInfo.Private()
	if err != nil {
		log.Fatal().Err(err).Msg("could not access private keys")
	}

	// write files
	writeText(model.PathNodeID, []byte(nodeInfo.NodeID.String()))
	writeJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), private)
	writeText(fmt.Sprintf(model.PathSecretsEncryptionKey, nodeInfo.NodeID), secretsDBKey)
	writeJSON(fmt.Sprintf(model.PathNodeInfoPub, nodeInfo.NodeID), nodeInfo.Public())

	// write machine account info
	if role == flow.RoleCollection || role == flow.RoleConsensus {

		// generate machine account priv key
		machineKey, err := generateMachineAccountKey()
		if err != nil {
			log.Fatal().Err(err).Msg("could not generate machine account key")
		}

		log.Debug().Str("address", flagAddress).Msg("assembling machine account information")
		// write the public key to terminal for entry in Flow Port
		machineAccountPriv := assembleNodeMachineAccountKey(machineKey)
		writeJSON(fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeInfo.NodeID), machineAccountPriv)
	}
}

func generateKeys() (crypto.PrivateKey, crypto.PrivateKey, []byte, error) {

	log.Debug().Msg("will generate networking key")
	networkKey, err := utils.GenerateNetworkingKey(flagNetworkSeed)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not generate networking key: %w", err)
	}
	log.Info().Msg("generated networking key")

	log.Debug().Msg("will generate staking key")
	stakingKey, err := utils.GenerateStakingKey(flagStakingSeed)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not generate staking key: %w", err)
	}
	log.Info().Msg("generated staking key")

	log.Debug().Msg("will generate db encryption key")
	secretsDBKey, err := utils.GenerateSecretsDBEncryptionKey()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not generate secrets db encryption key: %w", err)
	}
	log.Info().Msg("generated db encryption key")

	return networkKey, stakingKey, secretsDBKey, nil
}

func generateMachineAccountKey() (crypto.PrivateKey, error) {

	log.Debug().Msg("will generate machine account key")
	machineKey, err := utils.GenerateMachineAccountKey(flagMachineSeed)
	if err != nil {
		return nil, fmt.Errorf("could not generate machine key: %w", err)
	}
	log.Info().Msg("generated machine account key")

	return machineKey, nil
}

func validateRole(role string) flow.Role {
	parsed, err := flow.ParseRole(role)
	if err != nil {
		log.Fatal().Err(err).Msg("unsupported role, allowed values: \"collection\", \"consensus\", \"execution\", " +
			"\"verification\" or \"access\"")
	}
	return parsed
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

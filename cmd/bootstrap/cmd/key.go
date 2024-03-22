package cmd

import (
	"fmt"
	"github.com/onflow/crypto"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
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
		fmt.Sprintf("hex encoded networking seed (min %d bytes)", crypto.KeyGenSeedMinLen))
	keyCmd.Flags().BytesHexVar(
		&flagStakingSeed,
		"staking-seed",
		[]byte{},
		fmt.Sprintf("hex encoded staking seed (min %d bytes)", crypto.KeyGenSeedMinLen))
	keyCmd.Flags().BytesHexVar(
		&flagMachineSeed,
		"machine-seed",
		[]byte{},
		fmt.Sprintf("hex encoded machine account seed (min %d bytes)", crypto.KeyGenSeedMinLen))
}

// keyCmdRun generate the node staking key, networking key and node information
func keyCmdRun(_ *cobra.Command, _ []string) {

	// generate private key seeds if not specified via flag
	if len(flagNetworkSeed) == 0 {
		flagNetworkSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	}
	if len(flagStakingSeed) == 0 {
		flagStakingSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	}
	if len(flagMachineSeed) == 0 {
		flagMachineSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	}

	// validate inputs
	role := validateRole(flagRole)
	common.ValidateAddressFormat(log, flagAddress)

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
	err = common.WriteText(model.PathNodeID, flagOutdir, []byte(nodeInfo.NodeID.String()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write file")
	}
	log.Info().Msgf("wrote file %v", model.PathNodeID)

	err = common.WriteJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), flagOutdir, private)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}

	err = common.WriteText(fmt.Sprintf(model.PathSecretsEncryptionKey, nodeInfo.NodeID), flagOutdir, secretsDBKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write file")
	}
	log.Info().Msgf("wrote file %v", model.PathSecretsEncryptionKey)

	err = common.WriteJSON(fmt.Sprintf(model.PathNodeInfoPub, nodeInfo.NodeID), flagOutdir, nodeInfo.Public())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}
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
		err = common.WriteJSON(fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeInfo.NodeID), flagOutdir, machineAccountPriv)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to write json")
		}
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

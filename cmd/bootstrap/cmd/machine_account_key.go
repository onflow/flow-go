package cmd

import (
	"fmt"
	"path"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"

	"github.com/spf13/cobra"

	model "github.com/onflow/flow-go/model/bootstrap"
)

// machineAccountKeyCmd represents the `machine-account-key` command which generates required machine account key
// and writes it to the default path within the bootstrap directory. Used by existing operators to create the
// machine account key only
var machineAccountKeyCmd = &cobra.Command{
	Use:   "machine-account-key",
	Short: "Generates machine account key and writes it to the default path within the bootstrap directory",
	Run:   machineAccountKeyRun,
}

func init() {
	rootCmd.AddCommand(machineAccountKeyCmd)

	machineAccountKeyCmd.Flags().BytesHexVar(&flagMachineSeed, "seed", GenerateRandomSeed(crypto.KeyGenSeedMinLenECDSAP256), fmt.Sprintf("hex encoded machine account seed (min %d bytes)", crypto.KeyGenSeedMinLenECDSAP256))
}

// machineAccountKeyRun generate a machine account key and writes it to a default file path.
func machineAccountKeyRun(_ *cobra.Command, _ []string) {

	// read nodeID written to boostrap dir by `bootstrap key`
	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node id")
	}

	// check if node-machine-account-key.priv.json path exists
	machineAccountKeyPath := fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID)
	keyExists, err := pathExists(path.Join(flagOutdir, machineAccountKeyPath))
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if node-machine-account-key.priv.json exists")
	}
	if keyExists {
		log.Warn().Msg("machine account private key already exists, exiting...")
		return
	}

	machineKey, err := utils.GenerateMachineAccountKey(flagMachineSeed)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate machine key")
	}
	log.Info().Msg("generated machine account private key")

	// construct object to write to file
	// also write the public key to terminal for entry in Flow Port
	machineAccountPriv := assembleNodeMachineAccountKey(machineKey)

	writeJSON(machineAccountKeyPath, machineAccountPriv)
}

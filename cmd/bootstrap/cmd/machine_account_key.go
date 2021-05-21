package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
)

var (
	flagNodeID string
)

// machineAccountKeyCmd represents the `machine-account-key` command which generates required machine account key
// file and writes it to the default path within the boostrap directory. Used by existing operators to create  the
// machine account key only
var machineAccountKeyCmd = &cobra.Command{
	Use:   "machine-account-key",
	Short: "",
	Run:   machineAccountKeyRun,
}

func init() {
	rootCmd.AddCommand(machineAccountKeyCmd)

	machineAccountKeyCmd.Flags().BytesHexVar(&flagMachineSeed, "seed", generateRandomSeed(), fmt.Sprintf("hex encoded machine account seed (min %v bytes)", minSeedBytes))
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
	keyExists, err := pathExists(machineAccountKeyPath)
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if node-machine-account-key.priv.json exists")
	}
	if keyExists {
		log.Info().Msg("machine account private key already exists")
		return
	}

	machineSeed := validateSeed(flagMachineSeed)
	machineKey, err := run.GenerateMachineAccountKey(machineSeed)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate machine key")
	}
	log.Info().Msg("generated machine account private key")

	// construct object to write to file
	machineAccountPriv := assembleNodeMachineAccountPriv(machineKey)

	writeJSON(machineAccountKeyPath, machineAccountPriv)
	log.Info().Str("path", machineAccountKeyPath).Msg("wrote machine account private key")
}

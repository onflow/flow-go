package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/io"
)

const deployArgsFileName = "deploy-epoch-args.json"

// deployCmd represents a command to ...
var deployCmd = &cobra.Command{
	Use:   "deploy-tx-args",
	Short: "",
	Long:  "",
	Run:   deployRun,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	addDeployCmdFlags()
}

func addDeployCmdFlags() {
}

// deployRun ...
// Contract: https://github.com/onflow/flow-core-contracts/blob/master/contracts/epochs/FlowEpoch.cdc
// Transaction: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/deploy_epoch.cdc
func deployRun(cmd *cobra.Command, args []string) {

	// path to the root protocol snapshot json file
	snapshotPath := filepath.Join(flagBootDir, bootstrap.PathRootProtocolStateSnapshot)

	// check if root-protocol-snapshot.json file exists under the dir provided
	exists, err := pathExists(snapshotPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", snapshotPath).Msgf("could not check if root protocol-snapshot.json exists")
	}
	if !exists {
		log.Error().Str("path", snapshotPath).Msgf("root-protocol-snapshot.json file does not exists in the --boot-dir given")
		return
	}

	// construct path to the JSON encoded cadence arguments
	path, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get working directory path")
	}
	argsPath := filepath.Join(path, deployArgsFileName)

	// read root protocol-snapshot.json
	bz, err := io.ReadFile(snapshotPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not read root snapshot file")
	}
	log.Info().Str("snapshot_path", snapshotPath).Msg("read in root-protocol-snapshot.json")

	// unmarshal bytes to inmem protocol snapshot
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert array of bytes to snapshot")
	}

	fmt.Printf("%v, %v", argsPath, snapshot)
}

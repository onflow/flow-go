package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/io"
)

var moveMachineAcctCmd = &cobra.Command{
	Use:   "mv-machine-acct",
	Short: "move machine account files to the appropriate private folder",
	Run:   moveMachineAcctRun,
}

var flagMachineAccountsSrcDir string

func init() {
	rootCmd.AddCommand(moveMachineAcctCmd)
	moveMachineAcctCmd.Flags().StringVar(&flagMachineAccountsSrcDir, "machine-accounts-dir", "", "directory of machine account files (formatted as '<nodeid>-node-machine-account-key.priv.json'")
}

func moveMachineAcctRun(cmd *cobra.Command, args []string) {

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

	// retrieve participants
	identities, err := snapshot.Identities(filter.Any)
	if err != nil {
		log.Fatal().Err(err).Msg("could not retrieve identities")
	}

	// check that machine accounts dir exists
	exists, err = pathExists(flagMachineAccountsSrcDir)
	if err != nil {
		log.Fatal().Err(err).Str("path", snapshotPath).Msgf("could not check if root protocol-snapshot.json exists")
	}
	if !exists {
		log.Error().Str("path", snapshotPath).Msgf("root-protocol-snapshot.json file does not exists in the --boot-dir given")
		return
	}

	// identities with machine accounts
	machineAcctIdentities := identities.Filter(filter.HasRole(flow.RoleCollection, flow.RoleConsensus))

	machineAcctFiles, err := ioutil.ReadDir(flagMachineAccountsSrcDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read machine account dir")
	}
	if len(machineAcctFiles) != len(machineAcctIdentities) {
		log.Warn().Msgf("number of machine acct files is not the same as machine account identities (%d != %d)", len(machineAcctFiles), len(machineAcctIdentities))
	}

	for _, identity := range machineAcctIdentities {
		machineAccountSrcPath := filepath.Join(flagMachineAccountsSrcDir, fmt.Sprintf("%s-node-machine-account-key.priv.json", identity.NodeID))
		machineAccountDstPath := filepath.Join(flagBootDir, fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, identity.NodeID))
		err = os.Rename(machineAccountSrcPath, machineAccountDstPath)
		if err != nil {
			log.Warn().Err(err).
				Str("src_path", machineAccountSrcPath).
				Str("dst_path", machineAccountDstPath).
				Str("node_id", identity.NodeID.String()).
				Msg("could not move machine account")
		}
	}
}

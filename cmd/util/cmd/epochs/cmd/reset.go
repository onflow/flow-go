package cmd

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	epochcmdutil "github.com/onflow/flow-go/cmd/util/cmd/epochs/utils"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

const resetArgsFileName = "reset-epoch-args.json"

// resetCmd represents a command to generate `reset_epoch` transaction arguments and writes it to the
// working directory this command was run.
var resetCmd = &cobra.Command{
	Use:   "reset-tx-args",
	Short: "Generates `resetEpoch` JSON transaction arguments",
	Long: "Generates `resetEpoch` transaction arguments from a root protocol state snapshot and writes it to a JSON file." +
		"If the epoch setup phase fails (either the DKG, QC voting, or smart contract bug)," +
		"manual intervention is needed to transition to the next epoch.",
	Run: resetRun,
}

func init() {
	rootCmd.AddCommand(resetCmd)
	addResetCmdFlags()
}

func addResetCmdFlags() {
	resetCmd.Flags().StringVar(&flagPayout, "payout", "", "the payout eg. 10000.0")
}

// resetRun generates `resetEpoch` transaction arguments from a root protocol state snapshot and writes it to a JSON file
func resetRun(cmd *cobra.Command, args []string) {

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

	// path to the JSON encoded cadence arguments for the `resetEpoch` transaction
	path, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get working directory path")
	}
	argsPath := filepath.Join(path, resetArgsFileName)

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

	// extract arguments from reset epoch tx from snapshot
	txArgs := extractResetEpochArgs(snapshot)
	log.Info().Msg("extracted resetEpoch transaction arguments from snapshot")

	// encode to JSON
	enc, err := epochcmdutil.EncodeArgs(txArgs)
	if err != nil {
		log.Fatal().Err(err).Msg("could not encode epoch transaction arguments")
	}

	// write JSON args to file
	err = io.WriteFile(argsPath, enc)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
	}
	log.Info().Str("path", argsPath).Msg("wrote resetEpoch transaction arguments")
}

// extractResetEpochArgs extracts the required transaction arguments for the `resetEpoch` transaction
func extractResetEpochArgs(snapshot *inmem.Snapshot) []cadence.Value {

	// get current epoch
	epoch := snapshot.Epochs().Current()

	// Note: The epochCounter value expected by the smart contract is the epoch being
	// replaced, which is one less than the epoch beginning after the spork.
	epochCounter, err := epoch.Counter()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get epoch counter")
	}
	epochCounter = epochCounter - 1

	// read random source from epoch
	randomSource, err := epoch.RandomSource()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get random source from epoch")
	}

	// read first view
	firstView, err := epoch.FirstView()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get first view from epoch")
	}

	// read final view
	finalView, err := epoch.FinalView()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get final view from epoch")
	}

	return convertResetEpochArgs(epochCounter, randomSource, flagPayout, firstView, finalView)
}

// convertResetEpochArgs converts the arguments required by `resetEpoch` to cadence representations
// Contract Method: https://github.com/onflow/flow-core-contracts/blob/master/contracts/epochs/FlowEpoch.cdc#L423-L432
// Transaction: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/reset_epoch.cdc
func convertResetEpochArgs(epochCounter uint64, randomSource []byte, payout string, firstView, finalView uint64) []cadence.Value {

	args := make([]cadence.Value, 0)

	// add epoch counter
	args = append(args, cadence.NewUInt64(epochCounter))

	// add random source
	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert random source to cadence type")
	}
	args = append(args, cdcRandomSource)

	// add payout
	var cdcPayout cadence.Value
	if payout != "" {
		index := strings.Index(payout, ".")
		if index == -1 {
			log.Fatal().Msg("invalid --payout, eg: 10000.0")
		}

		cdcPayout, err = cadence.NewUFix64(payout)
		if err != nil {
			log.Fatal().Err(err).Msg("could not convert payout to cadence type")
		}
	} else {
		cdcPayout = cadence.NewOptional(nil)
	}
	args = append(args, cdcPayout)

	// add first view
	args = append(args, cadence.NewUInt64(firstView))

	// add final view
	args = append(args, cadence.NewUInt64(finalView))

	return args
}

// TODO: unify methods from transit, bootstrap and here
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

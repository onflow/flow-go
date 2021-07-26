package cmd

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

const resetArgsFileName = "reset-epoch-args.json"

var (
	flagBootDir string
	flagPayout  string
)

// resetCmd represents a command to reset epoch data in the Epoch smart contract
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Generates `resetEpoch` JSON transaction arguments",
	Long:  "Generates `resetEpoch` transaction arguments from a root protocol state snapshot and writes it to a JSON file",
	Run:   resetRun,
}

func init() {
	rootCmd.AddCommand(resetCmd)
	addResetCmdFlags()
}

func addResetCmdFlags() {
	resetCmd.Flags().StringVar(&flagBootDir, "boot-dir", "", "path to the directory containing the bootstrap files")
	_ = resetCmd.MarkFlagRequired("boot-dir")

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

	// ancode to JSON
	enc := encodeArgs(txArgs)

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
// Ref: https://github.com/onflow/flow-core-contracts/blob/feature/epochs/contracts/epochs/FlowEpoch.cdc#L370-L410
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

// encodeArgs JSON encodes `resetEpoch` transaction arguments
func encodeArgs(args []cadence.Value) []byte {

	arguments := make([]interface{}, len(args))

	for index, cdcVal := range args {

		encoded, err := jsoncdc.Encode(cdcVal)
		if err != nil {
			log.Fatal().Err(err).Msg("could not encode cadence arguments")
		}

		var arg interface{}
		err = json.Unmarshal(encoded, &arg)
		if err != nil {
			log.Fatal().Err(err).Msg("could not unmarshal cadence arguments")
		}

		arguments[index] = arg
	}

	bz, err := json.Marshal(arguments)
	if err != nil {
		log.Fatal().Err(err).Msg("could not marshal interface")
	}

	return bz
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

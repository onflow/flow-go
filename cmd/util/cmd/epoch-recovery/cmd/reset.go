package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

var (
	flagBootDir string
	flagPayout  uint64
)

// resetCmd represents a command to reset epoch data in the Epoch smart contract
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Generates `resetEpoch` JSON transaction arguments.",
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

	resetCmd.Flags().Uint64Var(&flagPayout, "payout", 0, "the payout")
}

// resetRun resets epoch data in the Epoch smart contract with fields generated
// from the root-protocol-snapshot.json
func resetRun(cmd *cobra.Command, args []string) {

	// path to the JSON encoded cadence arguments for the `resetEpoch` transaction
	path, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get working directory path")
	}
	argsPath := filepath.Join(path, "reset-epoch-args.json")

	// path to the root protocol snapshot json file
	snapshotPath := filepath.Join(flagBootDir, bootstrap.PathRootProtocolStateSnapshot)

	// check if root-protocol-snapshot.json file exists under the dir provided
	exists, err := pathExists(snapshotPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", snapshotPath).Msgf("could not check if root protocol-snapshot.json exists")
	}
	if !exists {
		log.Fatal().Str("path", snapshotPath).Msgf("root-protocol-snapshot.json file does not exists in the `boot-dir` given")
	}

	// read root protocol-snapshot.json
	bz, err := io.ReadFile(snapshotPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not read root snapshot file")
	}

	// unmarshal bytes to inmem protocol snapshot
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert array of bytes to snapshot")
	}

	// extract arguments from reset epoch tx from snapshot
	txArgs := extractResetEpochArgs(snapshot)

	// ancode to JSON
	enc := encodeArgs(txArgs)

	// write JSON args to file
	err = io.WriteFile(argsPath, enc)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
	}
}

// extractResetEpochArgs extracts the required transaction arguments for the `resetEpoch` transaction
func extractResetEpochArgs(snapshot *inmem.Snapshot) []cadence.Value {

	// get current epoch
	epoch := snapshot.Epochs().Current()

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

	return convertResetEpochArgs(randomSource, flagPayout, firstView, finalView)
}

// convertResetEpochArgs converts the arguments required by `resetEpoch` to cadence representations
func convertResetEpochArgs(randomSource []byte, payout, firstView, finalView uint64) []cadence.Value {

	args := make([]cadence.Value, 0)

	// add random source
	args = append(args, cadence.NewString(hex.EncodeToString(randomSource)))

	// add payout
	var cdcPayout cadence.Value
	var err error

	if payout != 0 {
		cdcPayout, err = cadence.NewUFix64(fmt.Sprintf("%d.0", payout))
		if err != nil {
			log.Fatal().Err(err).Msg("could not convert payout to cadence type")
		}
		args = append(args, cdcPayout)
	} else {
		cdcPayout = cadence.NewOptional(nil)
	}

	// add first view
	args = append(args, cadence.NewUInt64(firstView))

	// add final view
	args = append(args, cadence.NewUInt64(finalView))

	return args
}

// encodeArgs JSON encodes `resetEpoch` transaction arguments
func encodeArgs(args []cadence.Value) []byte {

	arguments := make([]interface{}, 0, len(args))

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

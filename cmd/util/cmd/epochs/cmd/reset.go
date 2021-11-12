package cmd

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
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

// rootSnapshotBucketURL is a format string for the location of the root snapshot file in GCP.
const rootSnapshotBucketURL = "https://storage.googleapis.com/flow-genesis-bootstrap/%s/public-root-information/root-protocol-state-snapshot.json"

// resetCmd represents a command to generate transaction arguments for the resetEpoch
// transaction used when resetting the FlowEpoch smart contract during the sporking process.
//
// When we perform a spork, the network is instantiated with a new protocol state which
// in generate is inconsistent with the state in the FlowEpoch smart contract. The resetEpoch
// transaction is the mechanism for re-synchronizing these two states.
//
var resetCmd = &cobra.Command{
	Use:   "reset-tx-args",
	Short: "Generates `resetEpoch` JSON transaction arguments",
	Long: "Generates `resetEpoch` transaction arguments from a root protocol state snapshot and writes it to STDOUT." +
		"For use during the sporking process.",
	Run: resetRun,
}

func init() {
	rootCmd.AddCommand(resetCmd)
	addResetCmdFlags()
}

func addResetCmdFlags() {
	resetCmd.Flags().StringVar(&flagPayout, "payout", "", "the payout eg. 10000.0")
	resetCmd.Flags().StringVar(&flagBucketNetworkName, "bucket-network-name", "", "when retrieving the root snapshot from a GCP bucket, the network name portion of the URL (eg. \"mainnet-13\")")
}

// resetRun generates `resetEpoch` transaction arguments from a root protocol state snapshot and writes it to a JSON file
func resetRun(cmd *cobra.Command, args []string) {

	stdout := cmd.OutOrStdout()

	// determine the source we will use for retrieving the root state snapshot,
	// prioritizing downloading from a GCP bucket
	var (
		snapshot *inmem.Snapshot
		err      error
	)

	if flagBucketNetworkName != "" {
		url := fmt.Sprintf(rootSnapshotBucketURL, flagBucketNetworkName)
		snapshot, err = getSnapshotFromBucket(url)
		if err != nil {
			log.Error().Err(err).Str("url", url).Msg("failed to retrieve root snapshot from bucket")
			return
		}
	} else if flagBootDir != "" {
		path := filepath.Join(flagBootDir, bootstrap.PathRootProtocolStateSnapshot)
		snapshot, err = getSnapshotFromLocalBootstrapDir(path)
		if err != nil {
			log.Error().Err(err).Str("path", path).Msg("failed to retrieve root snapshot from local bootstrap directory")
			return
		}
	} else {
		log.Fatal().Msg("must provide a source for root snapshot (specify either --boot-dir or --bucket-network-name)")
	}

	// extract arguments from reset epoch tx from snapshot
	txArgs := extractResetEpochArgs(snapshot)

	// encode to JSON
	encodedTxArgs, err := epochcmdutil.EncodeArgs(txArgs)
	if err != nil {
		log.Fatal().Err(err).Msg("could not encode epoch transaction arguments")
	}

	// write JSON args to stdout
	_, err = stdout.Write(encodedTxArgs)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
	}
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

// getSnapshotFromBucket downloads the root snapshot file from the given URL,
// decodes it, and returns the Snapshot object.
func getSnapshotFromBucket(url string) (*inmem.Snapshot, error) {

	// download root snapshot from provided URL
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not download root snapshot (url=%s): %w", url, err)
	}
	defer res.Body.Close()

	bz, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read from response body (url=%s): %w", url, err)
	}

	// unmarshal bytes to inmem protocol snapshot
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		return nil, fmt.Errorf("could not decode root snapshot: %w", err)
	}

	return snapshot, nil
}

// getSnapshotFromLocalBootstrapDir reads the root snapshot file from the given local path,
// decodes it, and returns the Snapshot object.
func getSnapshotFromLocalBootstrapDir(path string) (*inmem.Snapshot, error) {

	// check if root-protocol-snapshot.json file exists under the dir provided
	exists, err := pathExists(path)
	if err != nil {
		return nil, fmt.Errorf("could not check that root snapshot exists (path=%s): %w", path, err)
	}
	if !exists {
		return nil, fmt.Errorf("root snapshot file does not exist in local bootstrap dir (path=%s)", path)
	}

	// read root protocol-snapshot.json
	bz, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read root snapshot file: %w", err)
	}

	// unmarshal bytes to inmem protocol snapshot
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		return nil, fmt.Errorf("could not decode root snapshot: %w", err)
	}

	return snapshot, nil
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

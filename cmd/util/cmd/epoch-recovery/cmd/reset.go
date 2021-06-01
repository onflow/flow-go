package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/io"
)

var (
	flagBootDir string
)

// resetCmd represents a command to reset epoch data in the Epoch smart contract
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset Epoch details in the Epoch smart contract",

	// TODO: Write a longer description
	Long:  ``,
	Run:   resetRun,
}

func init() {
	rootCmd.AddCommand(resetCmd)
	addResetCmdFlags()
}

func addResetCmdFlags() {
	resetCmd.Flags().StringVar(&flagBootDir, "boot-dir", "", "path to the directory containing the bootstrap file")
	_ = resetCmd.MarkFlagRequired("boot-dir")
}

// resetRun resets epoch data in the Epoch smart contract with fields generated 
// from the root-protocol-snapshot.json
func resetRun(cmd *cobra.Command, args []string) {

	// check if root-protocol-snapshot.json file exists under the dir provided
	snapshotPath := filepath.Join(flagBootDir, bootstrap.PathRootProtocolStateSnapshot)
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

	// get current epoch
	epoch := snapshot.Epochs().Current()

	// extract arguments from reset epoch tx from snapshot
	payout := uint64(0)
	rndSource, firstView, finalView, clustering, qcs, dkgKeys := extractResetEpochTxArgs(epoch)
	
	// create resetEpoch transaction (to be signed by service account)
	_ = createResetEpochTx(rndSource, payout, firstView, finalView, clustering, qcs, dkgKeys)

	// TODO: handle signing tx with service account
}

// extractResetEpochTxArguments
func extractResetEpochTxArgs(epoch protocol.Epoch) ([]byte, uint64, uint64, flow.ClusterList, []string, []string) {

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

	// read collector clusters
	clustering, err := epoch.Clustering()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get clustering from epoch")
	}

	// TODO: read in DKG keys, and QCs

	// dkg, err := epoch.DKG()
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("could not get dkg from epoch")
	// }
		
	return randomSource, firstView, finalView, clustering, []string{}, []string{}
}

// createResetEpochTx creates the reset epoch transaction to be signed by the service account
func createResetEpochTx(randomSource []byte, 
	payout, firstView, finalView uint64, 
	clustering flow.ClusterList, clusterQCs, dkgPubKeys []string) *sdk.Transaction {
	
	env := templates.Environment{}

	// TODO: define authoriser, payer and proposer as Service Account
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateResetEpochScript(env)).
		SetGasLimit(9999)

	
	err := tx.AddArgument(cadence.NewString(hex.EncodeToString(randomSource)))
	if err != nil {
		log.Fatal().Err(err).Msg("could not add random source to tx arguments")
	}

	cPayout, err := cadence.NewUFix64(fmt.Sprint(payout))
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert payout to cadence type")
	}
	err = tx.AddArgument(cPayout)
	if err != nil {
		log.Fatal().Err(err).Msg("could not add random source to tx arguments")
	}

	err = tx.AddArgument(cadence.NewUInt64(firstView))
	if err != nil {
		log.Fatal().Err(err).Msg("could not add first view to tx arguments")
	}

	err = tx.AddArgument(cadence.NewUInt64(finalView))
	if err != nil {
		log.Fatal().Err(err).Msg("could not add final view to tx arguments")
	}

	// TODO: add to tx aerguments for clustering, QCs and DKG keys

	return tx
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
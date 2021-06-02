package cmd

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

var (
	flagBootDir string
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
	resetCmd.Flags().StringVar(&flagBootDir, "boot-dir", "", "path to the directory containing the bootstrap file")
	_ = resetCmd.MarkFlagRequired("boot-dir")
}

// resetRun resets epoch data in the Epoch smart contract with fields generated
// from the root-protocol-snapshot.json
func resetRun(cmd *cobra.Command, args []string) {

	// path to the JSON encoded cadence arguments for the `resetEpoch` transaction
	argsPath := ""

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
	cdcArgs := extractResetEpochArgs(snapshot)

	// encode cadence arguments to JSON
	encoded, err := jsoncdc.Encode(cdcArgs)
	if err != nil {
		log.Fatal().Err(err).Msg("could not encode cadence arguments")
	}

	// write JSON args to file
	err = io.WriteFile(argsPath, encoded)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
	}
}

// extractResetEpochArgs extracts the required transaction arguments for the `resetEpoch` transaction
func extractResetEpochArgs(snapshot *inmem.Snapshot) (cadence.Array) {

	// get current epoch
	epoch := snapshot.Epochs().Current()

	// set payout
	payout := uint64(0)

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

	// read collector clusters and convert to strings
	clustering, err := epoch.Clustering()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get clustering from epoch")
	}

	// read dkg public keys for all participants
	initialIdentities, err := epoch.InitialIdentities()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get initial identities from epoch")
	}

	dkg, err := epoch.DKG()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get dkg from epoch")
	}

	dkgKeys := make([]crypto.PublicKey, 0, len(initialIdentities))
	for _, identity := range initialIdentities {
		keyShare, err := dkg.KeyShare(identity.NodeID)
		if err != nil {
			log.Fatal().Str("node_id", identity.NodeID.String()).Err(err).Msg("coiuld get key share for node id")
		}
		dkgKeys = append(dkgKeys, keyShare)
	}

	// TODO: read in QCs

	return generateResetEpochArgsCadence(randomSource, payout, firstView, finalView, clustering, []string{}, dkgKeys)
}

// generateResetEpochArgsCadence creates the arguments required by 
func generateResetEpochArgsCadence(randomSource []byte,
	payout, firstView, finalView uint64,
	clustering flow.ClusterList, clusterQCs []string, dkgPubKeys []crypto.PublicKey) cadence.Array {

	args := make([]cadence.Value, 0)
	
	// add random source 
	args = append(args, cadence.NewString(hex.EncodeToString(randomSource)))
	
	// add payout
	cdcPayout, err := cadence.NewUFix64(fmt.Sprint(payout))
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert payout to cadence type")
	}
	args = append(args, cdcPayout)
	
	// add first view
	args = append(args, cadence.NewUInt64(firstView))

	// add final view
	args = append(args, cadence.NewUInt64(finalView))

	// TODO: convert clustering, clusterQC and dkg pub keys to cadence repr

	// clusterStrings := make([][]string, len(clustering))
	// for i, cluster := range clustering {
	// 	clusterNodeIDs := cluster.NodeIDs()
	// 	nodeIDStrings := make([]string, len(clusterNodeIDs))
	// 	for j, nodeID := range clusterNodeIDs {
	// 		nodeIDStrings[j] = nodeID.String()
	// 	}
	// 	clusterStrings[i] = nodeIDStrings
	// }

	return cadence.NewArray(args)
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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
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
	deployCmd.Flags().StringVar(&flagFungibleTokenAddress, "fungible-token-addr", "", "the hex address of the FungibleToken contract")
	deployCmd.Flags().StringVar(&flagFlowTokenAddress, "flow-token-addr", "", "the hex address of the FlowToken contract")
	deployCmd.Flags().StringVar(&flagIDTableAddress, "id-table-addr", "", "the hex address of the IDTable contract")
	deployCmd.Flags().Float64Var(&flagFlowSupplyIncreasePercentage, "flow-supply-increase-percentage", 0, "the FLOW supply increase percentage")

	_ = deployCmd.MarkFlagRequired("fungible-token-addr")
	_ = deployCmd.MarkFlagRequired("flow-token-addr")
	_ = deployCmd.MarkFlagRequired("id-table-addr")
	_ = deployCmd.MarkFlagRequired("flow-supply-increase-percentage")
}

// deployRun ...
// Contract: https://github.com/onflow/flow-core-contracts/blob/master/contracts/epochs/FlowEpoch.cdc
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

// getDeployEpochTransactionArguments pulls out required arguments for the `deploy_epoch` transaction from the root
// protocol snapshot and takes into any required ajustments to align the state of the contract with the protocol state
// and returns an array of the cadence representations of the arguments.
// Transaction: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/deploy_epoch.cdc
func getDeployEpochTransactionArguments(snapshot inmem.Snapshot) []cadence.Value {

	// current epoch
	currentEpoch := snapshot.Epochs().Current()

	head, err := snapshot.Head()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get head from snapshot")
	}

	// root chain id and system contractsRegister
	chainID := head.ChainID
	systemContracts, err := systemcontracts.SystemContractsForChain(chainID)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get system contracts for chainID")
	}

	// current epoch counter
	currentEpochCounter, err := currentEpoch.Counter()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get current epoch counter from snapshot")
	}

	// epoch contract name and get code for contract
	epochContractName := "FlowEpoch"
	epochContractCode := contracts.FlowEpoch(flagFungibleTokenAddress,
		flagFlowTokenAddress, flagIDTableAddress,
		systemContracts.ClusterQC.Address.Hex(), systemContracts.DKG.Address.Hex())

	// get final view from snapshot
	finalView, err := currentEpoch.FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get finalView for current epoch from snapshot")
	}

	dkgPhase1FinalView, err := currentEpoch.DKGPhase1FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get dkgPhase1FinalView from snapshot")
	}
	dkgPhase2FinalView, err := currentEpoch.DKGPhase2FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get dkgPhase2FinalView from snapshot")
	}

	// assume the first view after a spork is 0
	numViewsInEpoch := (finalView + 1) - head.View // 1000
	numViewsInDKGPhase := dkgPhase2FinalView - dkgPhase1FinalView + 1
	numViewsInStakingAuction := dkgPhase1FinalView - numViewsInDKGPhase - head.View + 1

	// number of collectors clusters
	clustering, err := currentEpoch.Clustering()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get clustering for current epoch from snapshot")
	}
	numCollectorClusters := len(clustering)

	// random source
	randomSource, err := currentEpoch.RandomSource()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get randomSource for current epoch from snapshot")
	}

	return convertDeployEpochTransactionArguments(epochContractName,
		epochContractCode,
		currentEpochCounter,
		numViewsInEpoch,
		numViewsInStakingAuction,
		numViewsInDKGPhase,
		numCollectorClusters,
		flagFlowSupplyIncreasePercentage,
		randomSource,
		clustering,
	)
}

// convertDeployEpochTransactionArguments converts the `deploy_epoch` transaction arguments to cadence representations
func convertDeployEpochTransactionArguments(contractName string, epochContractCode []byte, currentEpochCounter uint64,
	numViewsInEpoch, numViewsInStakingAuction, numViewsInDKGPhase uint64, numCollectorClusters int,
	FLOWsupplyIncreasePercentage float64, randomSource []byte, clustering flow.ClusterList) []cadence.Value {

	// arguments array
	args := make([]cadence.Value, 0)

	// add contractName
	cdcContractName, err := cadence.NewString(contractName)

	return args
}

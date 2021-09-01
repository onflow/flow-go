package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	epochcmdutil "github.com/onflow/flow-go/cmd/util/cmd/epochs/utils"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

const deployArgsFileName = "deploy-epoch-args.json"
const deployTxFileName = "deploy-epoch-tx.cdc"

// deployCmd represents a command to generate `deploy_epoch_relative` transaction arguments and writes it to the
// working directory this command was run.
var deployCmd = &cobra.Command{
	Use:   "deploy-tx-args",
	Short: "Generates `deploy_epoch_relative` transaction arguments and writes it to the working directory this command was run",
	Long: `The Epoch smart contract is deployed after a spork. The spork contains the initial information for the first epoch.
	 When we deploy the Epoch smart contract for the first time, we need to ensure the epoch length 
	 and staking auction length are consistent with the protocol state.`,
	Run: deployRun,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	addDeployCmdFlags()
}

func addDeployCmdFlags() {
	deployCmd.Flags().StringVar(&flagFungibleTokenAddress, "fungible-token-addr", "", "the hex address of the FungibleToken contract")
	deployCmd.Flags().StringVar(&flagFlowTokenAddress, "flow-token-addr", "", "the hex address of the FlowToken contract")
	deployCmd.Flags().StringVar(&flagIDTableAddress, "id-table-addr", "", "the hex address of the IDTable contract")
	deployCmd.Flags().StringVar(&flagFlowSupplyIncreasePercentage, "flow-supply-increase-percentage", "0.0", "the FLOW supply increase percentage")

	_ = deployCmd.MarkFlagRequired("fungible-token-addr")
	_ = deployCmd.MarkFlagRequired("flow-token-addr")
	_ = deployCmd.MarkFlagRequired("id-table-addr")
	_ = deployCmd.MarkFlagRequired("flow-supply-increase-percentage")
}

// deployRun generates `deploy_epoch_relative` transaction arguments from a root protocol state snapshot and writes it to a JSON file
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

	txArgs := getDeployEpochTransactionArguments(snapshot)
	log.Info().Msg("extracted `deploy_epoch_relative` transaction arguments from snapshot")

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
	log.Info().Str("path", argsPath).Msg("wrote `deploy_epoch_relative` transaction arguments")

	// write the transaction text to a file
	tx := getDeployEpochTransactionText(snapshot)
	err = io.WriteFile(deployTxFileName, tx)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write transaction file")
	}
}

// getDeployEpochTransactionArguments pulls out required arguments for the `deploy_epoch_relative` transaction from the root
// protocol snapshot and takes into any required ajustments to align the state of the contract with the protocol state
// and returns an array of the cadence representations of the arguments.
func getDeployEpochTransactionArguments(snapshot *inmem.Snapshot) []cadence.Value {

	// current epoch
	currentEpoch := snapshot.Epochs().Current()

	head, err := snapshot.Head()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get head from snapshot")
	}

	epochContractName := systemcontracts.ContractNameEpoch

	// current epoch counter
	currentEpochCounter, err := currentEpoch.Counter()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `currentEpochCounter` from snapshot")
	}

	// get final view from snapshot
	finalView, err := currentEpoch.FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `finalView` for current epoch from snapshot")
	}

	dkgPhase1FinalView, err := currentEpoch.DKGPhase1FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `dkgPhase1FinalView` from snapshot")
	}
	dkgPhase2FinalView, err := currentEpoch.DKGPhase2FinalView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `dkgPhase2FinalView` from snapshot")
	}

	numViewsInEpoch := (finalView + 1) - head.View
	numViewsInDKGPhase := dkgPhase2FinalView - dkgPhase1FinalView + 1
	numViewsInStakingAuction := dkgPhase1FinalView - numViewsInDKGPhase - head.View + 1

	// number of collectors clusters
	clustering, err := currentEpoch.Clustering()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `clustering` for current epoch from snapshot")
	}
	numCollectorClusters := len(clustering)

	// random source
	randomSource, err := currentEpoch.RandomSource()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get `randomSource` for current epoch from snapshot")
	}

	return convertDeployEpochTransactionArguments(
		epochContractName,
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

// convertDeployEpochTransactionArguments converts the `deploy_epoch_relative` transaction arguments to cadence representations
func convertDeployEpochTransactionArguments(
	contractName string,
	currentCounter uint64,
	numViewsInEpoch uint64,
	numViewsInStakingAuction uint64,
	numViewsInDKGPhase uint64,
	numCollectorClusters int,
	FLOWsupplyIncreasePercentage string,
	randomSource []byte,
	clustering flow.ClusterList,
) []cadence.Value {

	// arguments array
	args := make([]cadence.Value, 0)

	// add contractName
	cdcContractName := cadence.NewString(contractName)
	args = append(args, cdcContractName)

	// add epoch current counter
	cdcCurrentCounter := cadence.NewUInt64(currentCounter)
	args = append(args, cdcCurrentCounter)

	// add numViewsInEpoch
	cdcNumViewsInEpoch := cadence.NewUInt64(numViewsInEpoch)
	args = append(args, cdcNumViewsInEpoch)

	// add numViewsInStakingAuction
	cdcNumViewsInStakingAuction := cadence.NewUInt64(numViewsInStakingAuction)
	args = append(args, cdcNumViewsInStakingAuction)

	// add numViewsInDKGPhase
	cdcNumViewsInDKGPhase := cadence.NewUInt64(numViewsInDKGPhase)
	args = append(args, cdcNumViewsInDKGPhase)

	// add numCollectorClusters
	cdcNumCollectorClusters := cadence.NewUInt16(uint16(numCollectorClusters))
	args = append(args, cdcNumCollectorClusters)

	// add FLOWSupplyIncreasePercentage
	cdcFlowSupplyIncreasePercentage, err := cadence.NewUFix64(FLOWsupplyIncreasePercentage)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not convert `FLOWSupplyIncreasePercentage` to cadence representation")
	}
	args = append(args, cdcFlowSupplyIncreasePercentage)

	// add randomSource
	cdcRandomSource := cadence.NewString(hex.EncodeToString(randomSource))
	args = append(args, cdcRandomSource)

	return args
}

// getDeployEpochTransactionText returns the transaction text for the transaction
// to deploy FlowEpoch for the given snapshot. Primarily this involves setting
// the appropriate import addresses and injecting the FlowEpoch contract code
// as a literal in the transaction text.
func getDeployEpochTransactionText(snapshot *inmem.Snapshot) []byte {

	head, err := snapshot.Head()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get head from snapshot")
	}

	// root chain id and system contractsRegister
	chainID := head.ChainID
	systemContracts, err := systemcontracts.SystemContractsForChain(chainID)
	if err != nil {
		log.Fatal().Err(err).Str("chain_id", chainID.String()).Msgf("could not get system contracts for chainID")
	}

	// epoch contract name and get code for contract
	epochContractCode := contracts.FlowEpoch(
		flagFungibleTokenAddress,
		flagFlowTokenAddress,
		flagIDTableAddress,
		systemContracts.ClusterQC.Address.Hex(),
		systemContracts.DKG.Address.Hex(),
	)

	// convert the epoch contract code to an [UInt8] literal string that can be
	// injected to the Cadence transaction text.
	// NOTE: this is encoded as a literal in the transaction text rather than an
	// argument because the size of the encoded argument could exceed the stack
	// limit.
	builder := new(strings.Builder)
	builder.WriteString("[")
	for i, codeByte := range epochContractCode {
		if i < len(epochContractCode)-1 {
			builder.WriteString(fmt.Sprintf("%d,", codeByte))
			continue
		}
		builder.WriteString(fmt.Sprintf("%d", codeByte))
	}
	builder.WriteString("]")

	epochContractCodeAsCadenceLiteral := builder.String()

	deployEpochTransactionText := fmt.Sprintf(deployEpochTransactionTemplate, epochContractCodeAsCadenceLiteral)

	return []byte(deployEpochTransactionText)
}

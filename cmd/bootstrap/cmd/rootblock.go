package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/state/protocol/inmem"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/state/protocol"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagRootChain                  string
	flagRootParent                 string
	flagRootHeight                 uint64
	flagRootTimestamp              string
	flagProtocolVersion            uint
	flagEpochCommitSafetyThreshold uint64
	flagCollectionClusters         uint
	flagEpochCounter               uint64
	flagNumViewsInEpoch            uint64
	flagNumViewsInStakingAuction   uint64
	flagNumViewsInDKGPhase         uint64
	// Epoch target end time config
	flagUseDefaultEpochTargetEndTime bool
	flagEpochTimingRefCounter        uint64
	flagEpochTimingRefTimestamp      uint64
	flagEpochTimingDuration          uint64
)

// rootBlockCmd represents the rootBlock command
var rootBlockCmd = &cobra.Command{
	Use:   "rootblock",
	Short: "Generate root block data",
	Long:  `Run Beacon KeyGen, generate root block and votes for root block needed for constructing QC. Serialize all info into file`,
	Run:   rootBlock,
}

func init() {
	rootCmd.AddCommand(rootBlockCmd)
	addRootBlockCmdFlags()
}

func addRootBlockCmdFlags() {
	// required parameters for network configuration and generation of root node identities
	rootBlockCmd.Flags().StringVar(&flagConfig, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Weight)")
	rootBlockCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	rootBlockCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", "path to directory "+
		"containing one JSON file starting with node-info.pub.<NODE_ID>.json for every partner node (fields "+
		" in the JSON file: Role, Address, NodeID, NetworkPubKey, StakingPubKey)")
	rootBlockCmd.Flags().StringVar(&deprecatedFlagPartnerStakes, "partner-stakes", "", "deprecated: use --partner-weights")
	rootBlockCmd.Flags().StringVar(&flagPartnerWeights, "partner-weights", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")

	cmd.MarkFlagRequired(rootBlockCmd, "config")
	cmd.MarkFlagRequired(rootBlockCmd, "internal-priv-dir")
	cmd.MarkFlagRequired(rootBlockCmd, "partner-dir")
	cmd.MarkFlagRequired(rootBlockCmd, "partner-weights")

	// required parameters for generation of epoch setup and commit events
	rootBlockCmd.Flags().Uint64Var(&flagEpochCounter, "epoch-counter", 0, "epoch counter for the epoch beginning with the root block")
	rootBlockCmd.Flags().Uint64Var(&flagNumViewsInEpoch, "epoch-length", 4000, "length of each epoch measured in views")
	rootBlockCmd.Flags().Uint64Var(&flagNumViewsInStakingAuction, "epoch-staking-phase-length", 100, "length of the epoch staking phase measured in views")
	rootBlockCmd.Flags().Uint64Var(&flagNumViewsInDKGPhase, "epoch-dkg-phase-length", 1000, "length of each DKG phase measured in views")

	// optional parameters to influence various aspects of identity generation
	rootBlockCmd.Flags().UintVar(&flagCollectionClusters, "collection-clusters", 2, "number of collection clusters")

	cmd.MarkFlagRequired(rootBlockCmd, "epoch-counter")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-length")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-staking-phase-length")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-dkg-phase-length")

	// required parameters for generation of root block, root execution result and root block seal
	rootBlockCmd.Flags().StringVar(&flagRootChain, "root-chain", "local", "chain ID for the root block (can be 'main', 'test', 'sandbox', 'bench', or 'local'")
	rootBlockCmd.Flags().StringVar(&flagRootParent, "root-parent", "0000000000000000000000000000000000000000000000000000000000000000", "ID for the parent of the root block")
	rootBlockCmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0, "height of the root block")
	rootBlockCmd.Flags().StringVar(&flagRootTimestamp, "root-timestamp", time.Now().UTC().Format(time.RFC3339), "timestamp of the root block (RFC3339)")
	rootBlockCmd.Flags().UintVar(&flagProtocolVersion, "protocol-version", flow.DefaultProtocolVersion, "major software version used for the duration of this spork")
	rootBlockCmd.Flags().Uint64Var(&flagEpochCommitSafetyThreshold, "epoch-commit-safety-threshold", 500, "defines epoch commitment deadline")

	cmd.MarkFlagRequired(rootBlockCmd, "root-chain")
	cmd.MarkFlagRequired(rootBlockCmd, "root-parent")
	cmd.MarkFlagRequired(rootBlockCmd, "root-height")
	cmd.MarkFlagRequired(rootBlockCmd, "protocol-version")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-commit-safety-threshold")

	// Epoch timing config - these values must be set identically to `EpochTimingConfig` in the FlowEpoch smart contract.
	// See https://github.com/onflow/flow-core-contracts/blob/240579784e9bb8d97d91d0e3213614e25562c078/contracts/epochs/FlowEpoch.cdc#L259-L266
	// Must specify either:
	//   1. --use-default-epoch-timing and no other `--epoch-timing*` flags
	//   2. All `--epoch-timing*` flags except --use-default-epoch-timing
	//
	// Use Option 1 for Benchnet, Localnet, etc.
	// Use Option 2 for Mainnet, Testnet, Canary.
	finalizeCmd.Flags().BoolVar(&flagUseDefaultEpochTargetEndTime, "use-default-epoch-timing", false, "whether to use the default target end time")
	finalizeCmd.Flags().Uint64Var(&flagEpochTimingRefCounter, "epoch-timing-ref-counter", 0, "the reference epoch for computing the root epoch's target end time")
	finalizeCmd.Flags().Uint64Var(&flagEpochTimingRefTimestamp, "epoch-timing-ref-timestamp", 0, "the end time of the reference epoch, specified in second-precision Unix time, to use to compute the root epoch's target end time")
	finalizeCmd.Flags().Uint64Var(&flagEpochTimingDuration, "epoch-timing-duration", 0, "the duration of each epoch in seconds, used to compute the root epoch's target end time")

	finalizeCmd.MarkFlagsOneRequired("use-default-epoch-timing", "epoch-timing-ref-counter", "epoch-timing-ref-timestamp", "epoch-timing-duration")
	finalizeCmd.MarkFlagsRequiredTogether("epoch-timing-ref-counter", "epoch-timing-ref-timestamp", "epoch-timing-duration")
	for _, flag := range []string{"epoch-timing-ref-counter", "epoch-timing-ref-timestamp", "epoch-timing-duration"} {
		finalizeCmd.MarkFlagsMutuallyExclusive("use-default-epoch-timing", flag)
	}
}

func rootBlock(cmd *cobra.Command, args []string) {

	// maintain backward compatibility with old flag name
	if deprecatedFlagPartnerStakes != "" {
		log.Warn().Msg("using deprecated flag --partner-stakes (use --partner-weights instead)")
		if flagPartnerWeights == "" {
			flagPartnerWeights = deprecatedFlagPartnerStakes
		} else {
			log.Fatal().Msg("cannot use both --partner-stakes and --partner-weights flags (use only --partner-weights)")
		}
	}

	// validate epoch configs
	err := validateEpochConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid or unsafe epoch commit threshold config")
	}
	err = validateOrPopulateEpochTimingConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid epoch timing config")
	}

	log.Info().Msg("collecting partner network and staking keys")
	partnerNodes := readPartnerNodeInfos()
	log.Info().Msg("")

	log.Info().Msg("generating internal private networking and staking keys")
	internalNodes := readInternalNodeInfos()
	log.Info().Msg("")

	log.Info().Msg("checking constraints on consensus nodes")
	checkConstraints(partnerNodes, internalNodes)
	log.Info().Msg("")

	log.Info().Msg("assembling network and staking keys")
	stakingNodes := mergeNodeInfos(internalNodes, partnerNodes)
	writeJSON(model.PathNodeInfosPub, model.ToPublicNodeInfoList(stakingNodes))
	log.Info().Msg("")

	log.Info().Msg("running DKG for consensus nodes")
	dkgData := runBeaconKG(model.FilterByRole(stakingNodes, flow.RoleConsensus))
	log.Info().Msg("")

	// create flow.IdentityList representation of the participant set
	participants := model.ToIdentityList(stakingNodes).Sort(flow.Canonical[flow.Identity])

	log.Info().Msg("computing collection node clusters")
	assignments, clusters, err := constructClusterAssignment(partnerNodes, internalNodes)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate cluster assignment")
	}
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := run.GenerateRootClusterBlocks(flagEpochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := constructRootQCsForClusters(clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	log.Info().Msg("constructing root header")
	header := constructRootHeader(flagRootChain, flagRootParent, flagRootHeight, flagRootTimestamp)
	log.Info().Msg("")

	log.Info().Msg("constructing intermediary bootstrapping data")
	epochSetup, epochCommit := constructRootEpochEvents(header.View, participants, assignments, clusterQCs, dkgData)
	committedEpoch := inmem.NewCommittedEpoch(epochSetup, epochCommit)
	encodableEpoch, err := inmem.FromEpoch(committedEpoch)
	if err != nil {
		log.Fatal().Msg("could not convert root epoch to encodable")
	}
	epochConfig := generateExecutionStateEpochConfig(epochSetup, clusterQCs, dkgData)
	intermediaryEpochData := IntermediaryEpochData{
		ProtocolStateRootEpoch: encodableEpoch.Encodable(),
		ExecutionStateConfig:   epochConfig,
	}
	intermediaryParamsData := IntermediaryParamsData{
		EpochCommitSafetyThreshold: flagEpochCommitSafetyThreshold,
		ProtocolVersion:            flagProtocolVersion,
	}
	intermediaryData := IntermediaryBootstrappingData{
		IntermediaryEpochData:  intermediaryEpochData,
		IntermediaryParamsData: intermediaryParamsData,
	}
	writeJSON(model.PathIntermediaryBootstrappingData, intermediaryData)
	log.Info().Msg("")

	log.Info().Msg("constructing root block")
	block := constructRootBlock(header, epochSetup, epochCommit)
	writeJSON(model.PathRootBlockData, block)
	log.Info().Msg("")

	log.Info().Msg("constructing and writing votes")
	constructRootVotes(
		block,
		model.FilterByRole(stakingNodes, flow.RoleConsensus),
		model.FilterByRole(internalNodes, flow.RoleConsensus),
		dkgData,
	)
	log.Info().Msg("")
}

// validateEpochConfig validates configuration of the epoch commitment deadline.
func validateEpochConfig() error {
	chainID := parseChainID(flagRootChain)
	dkgFinalView := flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*3 // 3 DKG phases
	epochCommitDeadline := flagNumViewsInEpoch - flagEpochCommitSafetyThreshold

	defaultSafetyThreshold, err := protocol.DefaultEpochCommitSafetyThreshold(chainID)
	if err != nil {
		return fmt.Errorf("could not get default epoch commit safety threshold: %w", err)
	}

	// sanity check: the safety threshold is >= the default for the chain
	if flagEpochCommitSafetyThreshold < defaultSafetyThreshold {
		return fmt.Errorf("potentially unsafe epoch config: epoch commit safety threshold smaller than expected (%d < %d)", flagEpochCommitSafetyThreshold, defaultSafetyThreshold)
	}
	// sanity check: epoch commitment deadline cannot be before the DKG end
	if epochCommitDeadline <= dkgFinalView {
		return fmt.Errorf("invalid epoch config: the epoch commitment deadline (%d) is before the DKG final view (%d)", epochCommitDeadline, dkgFinalView)
	}
	// sanity check: the difference between DKG end and safety threshold is also >= the default safety threshold
	if epochCommitDeadline-dkgFinalView < defaultSafetyThreshold {
		return fmt.Errorf("potentially unsafe epoch config: time between DKG end and epoch commitment deadline is smaller than expected (%d-%d < %d)",
			epochCommitDeadline, dkgFinalView, defaultSafetyThreshold)
	}
	return nil
}

// generateExecutionStateEpochConfig generates epoch-related configuration used
// to generate an empty root execution state. This config is generated in the
// `rootblock` alongside the root epoch and root protocol state ID for consistency.
func generateExecutionStateEpochConfig(
	epochSetup *flow.EpochSetup,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData,
) epochs.EpochConfig {

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	if _, err := rand.Read(randomSource); err != nil {
		log.Fatal().Err(err).Msg("failed to generate a random source")
	}
	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid random source")
	}

	epochConfig := epochs.EpochConfig{
		EpochTokenPayout:             cadence.UFix64(0),
		RewardCut:                    cadence.UFix64(0),
		FLOWsupplyIncreasePercentage: cadence.UFix64(0),
		CurrentEpochCounter:          cadence.UInt64(epochSetup.Counter),
		NumViewsInEpoch:              cadence.UInt64(flagNumViewsInEpoch),
		NumViewsInStakingAuction:     cadence.UInt64(flagNumViewsInStakingAuction),
		NumViewsInDKGPhase:           cadence.UInt64(flagNumViewsInDKGPhase),
		NumCollectorClusters:         cadence.UInt16(flagCollectionClusters),
		RandomSource:                 cdcRandomSource,
		CollectorClusters:            epochSetup.Assignments,
		ClusterQCs:                   clusterQCs,
		DKGPubKeys:                   encodable.WrapRandomBeaconPubKeys(dkgData.PubKeyShares),
	}
	return epochConfig
}

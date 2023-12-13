package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/module/epochs"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var (
	flagRootChain                   string
	flagRootParent                  string
	flagRootHeight                  uint64
	flagRootCommit                  string
	flagRootTimestamp               string
	flagCollectionClusters          uint
	flagEpochCounter                uint64
	flagNumViewsInEpoch             uint64
	flagNumViewsInStakingAuction    uint64
	flagNumViewsInDKGPhase          uint64
	flagServiceAccountPublicKeyJSON string
	flagGenesisTokenSupply          string
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
	rootBlockCmd.Flags().StringVar(&flagRootCommit, "root-commit", "0000000000000000000000000000000000000000000000000000000000000000", "state commitment of root execution state")

	cmd.MarkFlagRequired(rootBlockCmd, "root-chain")
	cmd.MarkFlagRequired(rootBlockCmd, "root-parent")
	cmd.MarkFlagRequired(rootBlockCmd, "root-height")
	cmd.MarkFlagRequired(rootBlockCmd, "root-commit")

	// these two flags are only used when setup a network from genesis
	rootBlockCmd.Flags().StringVar(&flagServiceAccountPublicKeyJSON, "service-account-public-key-json",
		"{\"PublicKey\":\"ABCDEFGHIJK\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}",
		"encoded json of public key for the service account")
	rootBlockCmd.Flags().StringVar(&flagGenesisTokenSupply, "genesis-token-supply", "10000000.00000000",
		"genesis flow token supply")
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
	participants := model.ToIdentityList(stakingNodes).Sort(order.Canonical[flow.Identity])

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

	log.Info().Msg("constructing epoch events")
	epochSetup, epochCommit := constructRootEpochEvents(header.View, participants, assignments, clusterQCs, dkgData)
	committedEpoch := inmem.NewCommittedEpoch(epochSetup, epochCommit)
	encodableEpoch, err := inmem.FromEpoch(committedEpoch)
	if err != nil {
		log.Fatal().Msg("could not convert root epoch to encodable")
	}
	writeJSON(model.PathRootEpoch, encodableEpoch.Encodable())
	log.Info().Msg("")

	log.Info().Msg("constructing root block")
	block := constructRootBlock(header, epochSetup, epochCommit)
	writeJSON(model.PathRootBlockData, block)
	log.Info().Msg("")

	// if no root commit is specified, bootstrap an empty execution state
	if flagRootCommit == "0000000000000000000000000000000000000000000000000000000000000000" {
		generateEmptyExecutionState(
			block.Header.ChainID,
			epochSetup,
			clusterQCs,
			dkgData,
			participants,
		)
	}

	log.Info().Msg("constructing root execution result and block seal")
	result, seal := constructRootResultAndSeal(flagRootCommit, block, epochSetup, epochCommit)
	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
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
// TODO figure out where this should live
//func validateEpochConfig() error {
//	chainID := parseChainID(flagRootChain)
//	dkgFinalView := flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*3 // 3 DKG phases
//	epochCommitDeadline := flagNumViewsInEpoch - flagEpochCommitSafetyThreshold
//
//	defaultSafetyThreshold, err := protocol.DefaultEpochCommitSafetyThreshold(chainID)
//	if err != nil {
//		return fmt.Errorf("could not get default epoch commit safety threshold: %w", err)
//	}
//
//	// sanity check: the safety threshold is >= the default for the chain
//	if flagEpochCommitSafetyThreshold < defaultSafetyThreshold {
//		return fmt.Errorf("potentially unsafe epoch config: epoch commit safety threshold smaller than expected (%d < %d)", flagEpochCommitSafetyThreshold, defaultSafetyThreshold)
//	}
//	// sanity check: epoch commitment deadline cannot be before the DKG end
//	if epochCommitDeadline <= dkgFinalView {
//		return fmt.Errorf("invalid epoch config: the epoch commitment deadline (%d) is before the DKG final view (%d)", epochCommitDeadline, dkgFinalView)
//	}
//	// sanity check: the difference between DKG end and safety threshold is also >= the default safety threshold
//	if epochCommitDeadline-dkgFinalView < defaultSafetyThreshold {
//		return fmt.Errorf("potentially unsafe epoch config: time between DKG end and epoch commitment deadline is smaller than expected (%d-%d < %d)",
//			epochCommitDeadline, dkgFinalView, defaultSafetyThreshold)
//	}
//	return nil
//}

// generateEmptyExecutionState generates a new empty execution state with the
// given configuration. Sets the flagRootCommit variable for future reads.
func generateEmptyExecutionState(
	chainID flow.ChainID,
	epochSetup *flow.EpochSetup,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData,
	identities flow.IdentityList,
) (commit flow.StateCommitment) {

	log.Info().Msg("generating empty execution state")
	var serviceAccountPublicKey flow.AccountPublicKey
	err := serviceAccountPublicKey.UnmarshalJSON([]byte(flagServiceAccountPublicKeyJSON))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse the service account public key json")
	}

	cdcInitialTokenSupply, err := cadence.NewUFix64(flagGenesisTokenSupply)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid genesis token supply")
	}

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	if _, err = rand.Read(randomSource); err != nil {
		log.Fatal().Err(err).Msg("failed to generate a random source")
	}
	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid random source")
	}

	epochConfig := epochs.EpochConfig{
		// Staking contract config
		EpochTokenPayout:             cadence.UFix64(0),
		RewardCut:                    cadence.UFix64(0),
		FLOWsupplyIncreasePercentage: cadence.UFix64(0),
		// Epoch config
		CurrentEpochCounter:      cadence.UInt64(epochSetup.Counter),
		NumViewsInEpoch:          cadence.UInt64(epochSetup.NumViews()),
		NumViewsInStakingAuction: cadence.UInt64(flagNumViewsInStakingAuction),
		NumViewsInDKGPhase:       cadence.UInt64(flagNumViewsInDKGPhase),
		NumCollectorClusters:     cadence.UInt16(flagCollectionClusters),
		RandomSource:             cdcRandomSource,
		CollectorClusters:        epochSetup.Assignments,
		ClusterQCs:               clusterQCs,
		DKGPubKeys:               dkgData.PubKeyShares,
	}

	commit, err = run.GenerateExecutionState(
		filepath.Join(flagOutdir, model.DirnameExecutionState),
		serviceAccountPublicKey,
		chainID.Chain(),
		fvm.WithInitialTokenSupply(cdcInitialTokenSupply),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithEpochConfig(epochConfig),
		fvm.WithIdentities(identities),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate execution state")
	}
	flagRootCommit = hex.EncodeToString(commit[:])
	log.Info().Msg("")
	return
}

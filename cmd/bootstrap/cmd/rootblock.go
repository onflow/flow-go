package cmd

import (
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
	flagRootChain                string
	flagRootParent               string
	flagRootHeight               uint64
	flagRootTimestamp            string
	flagEpochCounter             uint64
	flagNumViewsInEpoch          uint64
	flagNumViewsInStakingAuction uint64
	flagNumViewsInDKGPhase       uint64
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

	cmd.MarkFlagRequired(rootBlockCmd, "epoch-counter")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-length")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-staking-phase-length")
	cmd.MarkFlagRequired(rootBlockCmd, "epoch-dkg-phase-length")

	// required parameters for generation of root block, root execution result and root block seal
	rootBlockCmd.Flags().StringVar(&flagRootChain, "root-chain", "local", "chain ID for the root block (can be 'main', 'test', 'sandbox', 'bench', or 'local'")
	rootBlockCmd.Flags().StringVar(&flagRootParent, "root-parent", "0000000000000000000000000000000000000000000000000000000000000000", "ID for the parent of the root block")
	rootBlockCmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0, "height of the root block")
	rootBlockCmd.Flags().StringVar(&flagRootTimestamp, "root-timestamp", time.Now().UTC().Format(time.RFC3339), "timestamp of the root block (RFC3339)")

	cmd.MarkFlagRequired(rootBlockCmd, "root-chain")
	cmd.MarkFlagRequired(rootBlockCmd, "root-parent")
	cmd.MarkFlagRequired(rootBlockCmd, "root-height")
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

	log.Info().Msg("constructing and writing votes")
	constructRootVotes(
		block,
		model.FilterByRole(stakingNodes, flow.RoleConsensus),
		model.FilterByRole(internalNodes, flow.RoleConsensus),
		dkgData,
	)
	log.Info().Msg("")
}

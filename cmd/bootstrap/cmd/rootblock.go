package cmd

import (
	"encoding/hex"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagFastKG        bool
	flagRootChain     string
	flagRootParent    string
	flagRootHeight    uint64
	flagRootTimestamp string
)

// rootBlockCmd represents the rootBlock command
var rootBlockCmd = &cobra.Command{
	Use:   "rootblock",
	Short: "Generate root block data",
	Long:  `Run DKG, generate root block and votes for root block needed for constructing QC. Serialize all info into file`,
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

	// required parameters for generation of root block, root execution result and root block seal
	rootBlockCmd.Flags().StringVar(&flagRootChain, "root-chain", "local", "chain ID for the root block (can be 'main', 'test', 'canary', 'bench', or 'local'")
	rootBlockCmd.Flags().StringVar(&flagRootParent, "root-parent", "0000000000000000000000000000000000000000000000000000000000000000", "ID for the parent of the root block")
	rootBlockCmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0, "height of the root block")
	rootBlockCmd.Flags().StringVar(&flagRootTimestamp, "root-timestamp", time.Now().UTC().Format(time.RFC3339), "timestamp of the root block (RFC3339)")

	cmd.MarkFlagRequired(rootBlockCmd, "root-chain")
	cmd.MarkFlagRequired(rootBlockCmd, "root-parent")
	cmd.MarkFlagRequired(rootBlockCmd, "root-height")

	rootBlockCmd.Flags().BytesHexVar(&flagBootstrapRandomSeed, "random-seed", GenerateRandomSeed(flow.EpochSetupRandomSourceLength), "The seed used to for DKG, Clustering and Cluster QC generation")

	// optional parameters to influence various aspects of identity generation
	rootBlockCmd.Flags().BoolVar(&flagFastKG, "fast-kg", false, "use fast (centralized) random beacon key generation instead of DKG")
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

	if len(flagBootstrapRandomSeed) != flow.EpochSetupRandomSourceLength {
		log.Error().Int("expected", flow.EpochSetupRandomSourceLength).Int("actual", len(flagBootstrapRandomSeed)).Msg("random seed provided length is not valid")
		return
	}

	log.Info().Str("seed", hex.EncodeToString(flagBootstrapRandomSeed)).Msg("deterministic bootstrapping random seed")
	log.Info().Msg("")

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
	dkgData := runDKG(model.FilterByRole(stakingNodes, flow.RoleConsensus))
	log.Info().Msg("")

	log.Info().Msg("constructing root block")
	block := constructRootBlock(flagRootChain, flagRootParent, flagRootHeight, flagRootTimestamp)
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

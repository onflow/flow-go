package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	cluster2 "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol/prg"
)

var (
	flagClusteringRandomSeed []byte
)

// clusterAssignmentCmd represents the clusterAssignment command
var clusterAssignmentCmd = &cobra.Command{
	Use:   "cluster-assignment",
	Short: "Generate cluster assignment",
	Long:  `Generate cluster assignment for collection nodes based on partner and internal node info and weights. Serialize into file with Epoch Counter`,
	Run:   clusterAssignment,
}

func init() {
	rootCmd.AddCommand(clusterAssignmentCmd)
	addClusterAssignmentCmdFlags()
}

func addClusterAssignmentCmdFlags() {
	// required parameters for network configuration and generation of root node identities
	clusterAssignmentCmd.Flags().StringVar(&flagConfig, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Weight)")
	clusterAssignmentCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	clusterAssignmentCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", "path to directory "+
		"containing one JSON file starting with node-info.pub.<NODE_ID>.json for every partner node (fields "+
		" in the JSON file: Role, Address, NodeID, NetworkPubKey, StakingPubKey)")
	clusterAssignmentCmd.Flags().StringVar(&flagPartnerWeights, "partner-weights", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")

	cmd.MarkFlagRequired(clusterAssignmentCmd, "config")
	cmd.MarkFlagRequired(clusterAssignmentCmd, "internal-priv-dir")
	cmd.MarkFlagRequired(clusterAssignmentCmd, "partner-dir")
	cmd.MarkFlagRequired(clusterAssignmentCmd, "partner-weights")

	// optional parameters for cluster assignment
	clusterAssignmentCmd.Flags().UintVar(&flagCollectionClusters, "collection-clusters", 2, "number of collection clusters")

	// required parameters for generation of cluster root blocks
	clusterAssignmentCmd.Flags().Uint64Var(&flagEpochCounter, "epoch-counter", 0, "epoch counter for the epoch beginning with the root block")
	cmd.MarkFlagRequired(clusterAssignmentCmd, "epoch-counter")

	clusterAssignmentCmd.Flags().BytesHexVar(&flagClusteringRandomSeed, "clustering-random-seed", nil, "random seed to generate the clustering assignment")
	cmd.MarkFlagRequired(clusterAssignmentCmd, "clustering-random-seed")

}

func clusterAssignment(cmd *cobra.Command, args []string) {
	// Read partner node's information and internal node's information.
	// With "internal nodes" we reference nodes, whose private keys we have. In comparison,
	// for "partner nodes" we generally do not have their keys. However, we allow some overlap,
	// in that we tolerate a configuration where information about an "internal node" is also
	// duplicated in the list of "partner nodes".
	log.Info().Msg("collecting partner network and staking keys")
	rawPartnerNodes, err := common.ReadFullPartnerNodeInfos(log, flagPartnerWeights, flagPartnerNodeInfoDir)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read full partner node infos")
	}
	log.Info().Msg("")

	log.Info().Msg("generating internal private networking and staking keys")
	internalNodes, err := common.ReadFullInternalNodeInfos(log, flagInternalNodePrivInfoDir, flagConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read full internal node infos")
	}
	log.Info().Msg("")

	// we now convert to the strict meaning of: "internal nodes" vs "partner nodes"
	//  • "internal nodes" we have they private keys for
	//  • "partner nodes" we don't have the keys for
	//  • both sets are disjoint (no common nodes)
	log.Info().Msg("remove internal partner nodes")
	partnerNodes := common.FilterInternalPartners(rawPartnerNodes, internalNodes)
	log.Info().Msgf("removed %d internal partner nodes", len(rawPartnerNodes)-len(partnerNodes))

	log.Info().Msg("checking constraints on consensus nodes")
	checkConstraints(partnerNodes, internalNodes)
	log.Info().Msg("")

	log.Info().Msg("assembling network and staking keys")
	stakingNodes, err := mergeNodeInfos(internalNodes, partnerNodes)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to merge node infos")
	}
	publicInfo, err := model.ToPublicNodeInfoList(stakingNodes)
	if err != nil {
		log.Fatal().Msg("failed to read public node info")
	}
	err = common.WriteJSON(model.PathNodeInfosPub, flagOutdir, publicInfo)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}
	log.Info().Msgf("wrote file %s/%s", flagOutdir, model.PathNodeInfosPub)
	log.Info().Msg("")

	// Convert to IdentityList
	partnerList := model.ToIdentityList(partnerNodes)
	internalList := model.ToIdentityList(internalNodes)

	clusteringPrg, err := prg.New(flagClusteringRandomSeed, prg.BootstrapClusterAssignment, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize pseudorandom generator")
	}

	log.Info().Msg("computing collection node clusters")
	assignments, clusters, canConstructQCs, err := common.ConstructClusterAssignment(log, partnerList, internalList, int(flagCollectionClusters), clusteringPrg)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate cluster assignment")
	}
	log.Info().Msg("")

	// Output assignment with epoch counter
	output := IntermediaryClusteringData{
		EpochCounter: flagEpochCounter,
		Assignments:  assignments,
		Clusters:     clusters,
	}
	err = common.WriteJSON(model.PathClusteringData, flagOutdir, output)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write json")
	}
	log.Info().Msgf("wrote file %s/%s", flagOutdir, model.PathClusteringData)
	log.Info().Msg("")

	log.Info().Msg("constructing and writing cluster block votes for internal nodes")
	constructClusterRootVotes(
		output,
		model.FilterByRole(internalNodes, flow.RoleCollection),
	)
	log.Info().Msg("")

	if canConstructQCs {
		log.Info().Msg("enough votes for collection clusters are present - bootstrapping can continue with root block creation")
	} else {
		log.Info().Msg("not enough internal votes to generate cluster QCs, need partner votes before root block creation")
	}
}

// constructClusterRootVotes generates and writes vote files for internal collector nodes with private keys available.
func constructClusterRootVotes(data IntermediaryClusteringData, internalCollectors []model.NodeInfo) {
	for i := range data.Clusters {
		clusterRootBlock, err := cluster2.CanonicalRootBlock(data.EpochCounter, data.Assignments[i])
		if err != nil {
			log.Fatal().Err(err).Msg("could not construct cluster root block")
		}
		block := hotstuff.GenesisBlockFromFlow(clusterRootBlock.ToHeader())
		// collate private NodeInfos for internal nodes in this cluster
		signers := make([]model.NodeInfo, 0)
		for _, nodeID := range data.Assignments[i] {
			for _, node := range internalCollectors {
				if node.NodeID == nodeID {
					signers = append(signers, node)
				}
			}
		}
		votes, err := run.CreateClusterRootBlockVotes(signers, block)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create cluster root block votes")
		}
		for _, vote := range votes {
			path := filepath.Join(model.DirnameRootBlockVotes, fmt.Sprintf(model.FilenameRootClusterBlockVote, vote.SignerID))
			err = common.WriteJSON(path, flagOutdir, vote)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to write json")
			}
			log.Info().Msgf("wrote file %s/%s", flagOutdir, path)
		}
	}
}

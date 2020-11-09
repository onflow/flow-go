package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	model "github.com/onflow/flow-go/model/bootstrap"
)

var (
	flagStakingNodesDir string
)

// finallistCmd represents the final list command
var finalListCmd = &cobra.Command{
	Use:   "finallist",
	Short: "",
	Long:  ``,
	Run:   finalList,
}

func init() {
	rootCmd.AddCommand(finalListCmd)
	addFinalListFlags()
}

func addFinalListFlags() {
	// partner node info flag
	finalListCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-infos", "", "path to a directory containing all parnter nodes details")
	_ = finalListCmd.MarkFlagRequired("partner-infos")

	// internal/flow node info flag
	finalListCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "flow-infos", "", "path to a directory containing all internal/flow nodes details")
	_ = finalListCmd.MarkFlagRequired("flow-infos")

	// staking nodes dir containing staking nodes json
	finalListCmd.Flags().StringVar(&flagStakingNodesDir, "staking-nodes", "", "path to a directory containing a JSON file of all staking nodes")
	_ = finalListCmd.MarkFlagRequired("staking-nodes")
}

func finalList(cmd *cobra.Command, args []string) {
	// read public partner node infos
	log.Info().Msgf("reading parnter public node information: %s", flagPartnerNodeInfoDir)
	partnerNodes := readPartnerNodes()

	// read internal private node infos
	log.Info().Msgf("reading internal/flow private node information: %s", flagInternalNodePrivInfoDir)
	flowNodes := readInternalNodes()

	log.Info().Msgf("reading staking contract node information: %s", flagStakingNodesDir)
	stakingNodes := readStakingContractDetails()

	// merge internal and partner node infos
	allNodes := mergeNodeInfos(flowNodes, partnerNodes)

	// reconcile nodes from staking contract nodes
	reconcileNodes(allNodes, stakingNodes)
}

func readStakingContractDetails() []model.NodeInfoPub {
	var nodes []model.NodeInfoPub
	path := filepath.Join(flagStakingNodesDir, "node-infos.pub.json")
	readJSON(path, &nodes)
	return nodes
}

func reconcileNodes(nodes []model.NodeInfo, stakingNodes model.NodeInfo) {
	// check node count
	// check node id mismatch
	// check node type mismathc
}

package cmd

import (
	"fmt"
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
	Long:  "",
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
	partnerNodes := assemblePartnerNodesWithoutStake()

	// read internal private node infos
	log.Info().Msgf("reading internal/flow private node information: %s", flagInternalNodePrivInfoDir)
	flowNodes := assembleInternalNodesWithoutStake()

	log.Info().Msg("checking constraints on consensus/cluster nodes")
	checkConstraints(partnerNodes, flowNodes)

	log.Info().Msgf("reading staking contract node information: %s", flagStakingNodesDir)
	stakingNodes := readStakingContractDetails()

	// merge internal and partner node infos
	allNodes := mergeNodeInfos(flowNodes, partnerNodes)

	// reconcile nodes from staking contract nodes
	reconcileNodes(allNodes, stakingNodes)

	// TODO: output a new nodes-config.json ... what is this config?
	writeJSON(fmt.Sprintf(flagOutdir, "node-config.json"), allNodes)
}

func readStakingContractDetails() []model.NodeInfo {
	var stakingNodes []model.NodeInfoPub
	path := filepath.Join(flagStakingNodesDir, "node-infos.pub.json")
	readJSON(path, &stakingNodes)

	var nodes []model.NodeInfo
	for _, staking := range stakingNodes {
		validateAddressFormat(staking.Address)

		// validate every single partner node
		nodeID := validateNodeID(staking.NodeID)
		networkPubKey := validateNetworkPubKey(staking.NetworkPubKey)
		stakingPubKey := validateStakingPubKey(staking.StakingPubKey)

		node := model.NewPublicNodeInfo(
			nodeID,
			staking.Role,
			staking.Address,
			0,
			networkPubKey,
			stakingPubKey,
		)
		nodes = append(nodes, node)
	}

	return nodes
}

func reconcileNodes(nodes []model.NodeInfo, stakingNodes []model.NodeInfo) {
	// check node count
	if len(nodes) != len(stakingNodes) {
		log.Error().Int("nodes", len(nodes)).Int("staked nodes", len(stakingNodes)).
			Msg("staked node count does not match internal and parnter node count")
	}

	var nodesByAddress map[string]model.NodeInfo
	for _, node := range nodes {
		nodesByAddress[node.Address] = node
	}

	// check node id mismatch
	for _, stakedNode := range stakingNodes {
		matchingNode := nodesByAddress[stakedNode.Address]

		if matchingNode.NodeID != stakedNode.NodeID {
			log.Error().Str("staked node", stakedNode.NodeID.String()).
				Str("node", matchingNode.NodeID.String()).
				Msg("node id does not match staked contract nodeID")
		}
	}

	// check node type mismatch
	for _, stakedNode := range stakingNodes {
		matchingNode := nodesByAddress[stakedNode.Address]

		if matchingNode.NodeID != stakedNode.NodeID {
			log.Error().Str("staked node", stakedNode.NodeID.String()).
				Str("staked node type", stakedNode.Role.String()).
				Str("node", matchingNode.NodeID.String()).
				Str("node type", matchingNode.Role.String()).
				Msg("node type does not match")
		}
	}
}

func assembleInternalNodesWithoutStake() []model.NodeInfo {
	privInternals := readInternalNodes()
	log.Info().Msgf("read %v internal private node-info files", len(privInternals))

	var nodes []model.NodeInfo
	for _, internal := range privInternals {
		// check if address is valid format
		validateAddressFormat(internal.Address)

		// validate every single internal node
		nodeID := validateNodeID(internal.NodeID)
		node := model.NewPrivateNodeInfo(
			nodeID,
			internal.Role,
			internal.Address,
			0,
			internal.NetworkPrivKey,
			internal.StakingPrivKey,
		)

		nodes = append(nodes, node)
	}

	return nodes
}

func assemblePartnerNodesWithoutStake() []model.NodeInfo {
	partners := readPartnerNodes()
	log.Info().Msgf("read %v partner node configuration files", len(partners))

	var nodes []model.NodeInfo
	for _, partner := range partners {
		validateAddressFormat(partner.Address)

		// validate every single partner node
		nodeID := validateNodeID(partner.NodeID)
		networkPubKey := validateNetworkPubKey(partner.NetworkPubKey)
		stakingPubKey := validateStakingPubKey(partner.StakingPubKey)

		node := model.NewPublicNodeInfo(
			nodeID,
			partner.Role,
			partner.Address,
			0,
			networkPubKey,
			stakingPubKey,
		)
		nodes = append(nodes, node)
	}

	return nodes
}

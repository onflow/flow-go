package cmd

import (
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagStakingNodesPath string
)

// finallistCmd represents the final list command
var finalListCmd = &cobra.Command{
	Use:   "finallist",
	Short: "generates a final list of nodes to be used for next network",
	Long:  "generates a final list of nodes to be used for next network after validating node data and matching against staking contract nodes ",
	Run:   finalList,
}

func init() {
	rootCmd.AddCommand(finalListCmd)
	addFinalListFlags()
}

func addFinalListFlags() {
	// partner node info flag
	finalListCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-infos", "", "path to a directory containing all parnter nodes details")
	cmd.MarkFlagRequired(finalListCmd, "partner-infos")

	// internal/flow node info flag
	finalListCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "flow-infos", "", "path to a directory containing all internal/flow nodes details")
	cmd.MarkFlagRequired(finalListCmd, "flow-infos")

	// staking nodes dir containing staking nodes json
	finalListCmd.Flags().StringVar(&flagStakingNodesPath, "staking-nodes", "", "path to a JSON file of all staking nodes")
	cmd.MarkFlagRequired(finalListCmd, "staking-nodes")

	finalListCmd.Flags().UintVar(&flagCollectionClusters, "collection-clusters", 2,
		"number of collection clusters")
}

func finalList(cmd *cobra.Command, args []string) {
	// read public partner node infos
	log.Info().Msgf("reading parnter public node information: %s", flagPartnerNodeInfoDir)
	partnerNodes := assemblePartnerNodesWithoutStake()

	// read internal private node infos
	log.Info().Msgf("reading internal/flow private node information: %s", flagInternalNodePrivInfoDir)
	flowNodes := assembleInternalNodesWithoutStake()

	log.Info().Msg("checking constraints on consensus/cluster nodes")
	checkConsensusConstraints(partnerNodes, flowNodes)
	checkCollectionConstraints(partnerNodes, flowNodes)

	log.Info().Msgf("reading staking contract node information: %s", flagStakingNodesPath)
	stakingNodes := readStakingContractDetails()

	// merge internal and partner node infos
	mixedNodeInfos := mergeNodeInfos(flowNodes, partnerNodes)

	// reconcile nodes from staking contract nodes
	validateNodes(mixedNodeInfos, stakingNodes)

	// write node-config.json with the new list of nodes to be used for the `finalize` command
	writeJSON(model.PathFinallist, model.ToPublicNodeInfoList(mixedNodeInfos))
}

func validateNodes(nodes []model.NodeInfo, stakingNodes []model.NodeInfo) {
	// check node count
	if len(nodes) != len(stakingNodes) {
		log.Error().Int("nodes", len(nodes)).Int("staked nodes", len(stakingNodes)).
			Msg("staked node count does not match flow and parnter node count")
	}

	// check staked and collected nodes and make sure node ID are not missing
	validateNodeIDs(nodes, stakingNodes)

	// print mis matching nodes
	checkMisMatchingNodes(nodes, stakingNodes)

	// create map
	nodesByID := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range nodes {
		nodesByID[node.NodeID] = node
	}

	// check node type mismatch
	for _, stakedNode := range stakingNodes {

		// win have matching node as we have a check before
		matchingNode := nodesByID[stakedNode.NodeID]

		// check node type and error if mismatch
		if matchingNode.Role != stakedNode.Role {
			log.Error().Str("staked node", stakedNode.NodeID.String()).
				Str("staked node-type", stakedNode.Role.String()).
				Str("node", matchingNode.NodeID.String()).
				Str("node-type", matchingNode.Role.String()).
				Msg("node type does not match")
		}

		// No need to error adderss, and public keys as they may be different
		// we only keep node-id constant through out sporks

		// check address match
		if matchingNode.Address != stakedNode.Address {
			log.Warn().Str("staked node", stakedNode.NodeID.String()).
				Str("node", matchingNode.NodeID.String()).
				Msg("address do not match")
		}

		// flow nodes contain private key info
		if matchingNode.NetworkPubKey().String() != "" {
			// check networking pubkey match
			matchNodeKey := matchingNode.NetworkPubKey().String()
			stakedNodeKey := stakedNode.NetworkPubKey().String()

			if matchNodeKey != stakedNodeKey {
				log.Warn().Str("staked network key", stakedNodeKey).
					Str("network key", matchNodeKey).
					Msg("networking keys do not match")
			}
		}

		// flow nodes contain priv atekey info
		if matchingNode.StakingPubKey().String() != "" {
			matchNodeKey := matchingNode.StakingPubKey().String()
			stakedNodeKey := stakedNode.StakingPubKey().String()

			if matchNodeKey != stakedNodeKey {
				log.Warn().Str("staked staking key", stakedNodeKey).
					Str("staking key", matchNodeKey).
					Msg("staking keys do not match")
			}
		}
	}
}

// validateNodeIDs will go through both sets of nodes and ensure that no node-id
// are missing. It will log all missing node ID's and throw an error.
func validateNodeIDs(collectedNodes []model.NodeInfo, stakedNodes []model.NodeInfo) {

	// go through staking nodes
	invalidStakingNodes := make([]model.NodeInfo, 0)
	for _, node := range stakedNodes {
		if node.NodeID.String() == "" {

			// we warn here but exit later
			invalidStakingNodes = append(invalidStakingNodes, node)
			log.Warn().
				Str("node-address", node.Address).
				Msg("missing node-id from staked nodes")
		}
	}

	// go through staking nodes
	invalidNodes := make([]model.NodeInfo, 0)
	for _, node := range collectedNodes {
		if node.NodeID.String() == "" {

			// we warn here but exit later
			invalidNodes = append(invalidNodes, node)
			log.Warn().
				Str("node-address", node.Address).
				Msg("missing node-id from collected nodes")
		}
	}

	if len(invalidNodes) != 0 || len(invalidStakingNodes) != 0 {
		log.Fatal().Msg("found missing nodes ids. fix and re-run")
	}
}

func checkMisMatchingNodes(collectedNodes []model.NodeInfo, stakedNodes []model.NodeInfo) {

	collectedNodesByID := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range collectedNodes {
		collectedNodesByID[node.NodeID] = node
	}

	stakedNodesByID := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range stakedNodes {
		stakedNodesByID[node.NodeID] = node
	}

	// try match collected nodes to staked nodes
	invalidCollectedNodes := make([]model.NodeInfo, 0)
	for _, node := range collectedNodes {
		if _, ok := stakedNodesByID[node.NodeID]; !ok {
			log.Warn().Str("collected-node-id", node.NodeID.String()).Str("role", node.Role.String()).Str("address", node.Address).
				Msg("matching staked node not found for collected node")
			invalidCollectedNodes = append(invalidCollectedNodes, node)
		}
	}

	invalidStakedNodes := make([]model.NodeInfo, 0)
	for _, node := range stakedNodes {
		if _, ok := collectedNodesByID[node.NodeID]; !ok {
			log.Warn().Str("staked-node-id", node.NodeID.String()).Str("role", node.Role.String()).Str("address", node.Address).
				Msg("matching collected node not found for staked node")
			invalidStakedNodes = append(invalidStakedNodes, node)
		}
	}

	if len(invalidCollectedNodes) != 0 || len(invalidStakedNodes) != 0 {
		log.Fatal().Msg("found missing mismatching nodes")
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
			1000,
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
	return createPublicNodeInfo(partners)
}

func readStakingContractDetails() []model.NodeInfo {
	var stakingNodes []model.NodeInfoPub
	readJSON(flagStakingNodesPath, &stakingNodes)
	return createPublicNodeInfo(stakingNodes)
}

func createPublicNodeInfo(nodes []model.NodeInfoPub) []model.NodeInfo {
	var publicInfoNodes []model.NodeInfo
	for _, n := range nodes {
		validateAddressFormat(n.Address)

		// validate every single partner node
		nodeID := validateNodeID(n.NodeID)
		networkPubKey := validateNetworkPubKey(n.NetworkPubKey)
		stakingPubKey := validateStakingPubKey(n.StakingPubKey)

		// stake set to 1000 to give equal weight to each node
		node := model.NewPublicNodeInfo(
			nodeID,
			n.Role,
			n.Address,
			1000,
			networkPubKey,
			stakingPubKey,
		)

		publicInfoNodes = append(publicInfoNodes, node)
	}

	return publicInfoNodes
}

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

// finalListCmd represents the final list command
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
	log.Info().Msgf("reading partner public node information: %s", flagPartnerNodeInfoDir)
	partnerNodes := assemblePartnerNodesWithoutWeight()

	// read internal private node infos
	log.Info().Msgf("reading internal/flow private node information: %s", flagInternalNodePrivInfoDir)
	internalNodes := assembleInternalNodesWithoutWeight()

	log.Info().Msg("checking constraints on consensus/cluster nodes")
	checkConstraints(partnerNodes, internalNodes)

	// nodes which are registered on-chain
	log.Info().Msgf("reading staking contract node information: %s", flagStakingNodesPath)
	registeredNodes := readStakingContractDetails()

	// merge internal and partner node infos (from local files)
	localNodes := mergeNodeInfos(internalNodes, partnerNodes)

	// reconcile nodes from staking contract nodes
	validateNodes(localNodes, registeredNodes)

	// write node-config.json with the new list of nodes to be used for the `finalize` command
	writeJSON(model.PathFinallist, model.ToPublicNodeInfoList(localNodes))
}

func validateNodes(localNodes []model.NodeInfo, registeredNodes []model.NodeInfo) {
	// check node count
	if len(localNodes) != len(registeredNodes) {
		log.Error().
			Int("local", len(localNodes)).
			Int("onchain", len(registeredNodes)).
			Msg("onchain node count does not match local internal+partner node count")
	}

	// check registered and local nodes to make sure node ID are not missing
	validateNodeIDs(localNodes, registeredNodes)

	// print mismatching nodes
	checkMismatchingNodes(localNodes, registeredNodes)

	// create map
	localNodeMap := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range localNodes {
		localNodeMap[node.NodeID] = node
	}

	// check node type mismatch
	for _, registeredNode := range registeredNodes {

		// win have matching node as we have a check before
		matchingNode := localNodeMap[registeredNode.NodeID]

		// check node type and error if mismatch
		if matchingNode.Role != registeredNode.Role {
			log.Error().
				Str("registered node id", registeredNode.NodeID.String()).
				Str("registered node role", registeredNode.Role.String()).
				Str("local node", matchingNode.NodeID.String()).
				Str("local node role", matchingNode.Role.String()).
				Msg("node role does not match")
		}

		if matchingNode.Address != registeredNode.Address {
			log.Error().
				Str("registered node id", registeredNode.NodeID.String()).
				Str("registered node address", registeredNode.Address).
				Str("local node", matchingNode.NodeID.String()).
				Str("local node address", matchingNode.Address).
				Msg("node address does not match")
		}

		// check address match
		if matchingNode.Address != registeredNode.Address {
			log.Warn().
				Str("registered node", registeredNode.NodeID.String()).
				Str("node id", matchingNode.NodeID.String()).
				Msg("address do not match")
		}

		// flow localNodes contain private key info
		if matchingNode.NetworkPubKey().String() != "" {
			// check networking pubkey match
			matchNodeKey := matchingNode.NetworkPubKey().String()
			registeredNodeKey := registeredNode.NetworkPubKey().String()

			if matchNodeKey != registeredNodeKey {
				log.Error().
					Str("registered network key", registeredNodeKey).
					Str("network key", matchNodeKey).
					Msg("networking keys do not match")
			}
		}

		// flow localNodes contain privatekey info
		if matchingNode.StakingPubKey().String() != "" {
			matchNodeKey := matchingNode.StakingPubKey().String()
			registeredNodeKey := registeredNode.StakingPubKey().String()

			if matchNodeKey != registeredNodeKey {
				log.Error().
					Str("registered staking key", registeredNodeKey).
					Str("staking key", matchNodeKey).
					Msg("staking keys do not match")
			}
		}
	}
}

// validateNodeIDs will go through both sets of nodes and ensure that no node-id
// are missing. It will log all missing node ID's and throw an error.
func validateNodeIDs(localNodes []model.NodeInfo, registeredNodes []model.NodeInfo) {

	// go through registered nodes
	invalidStakingNodes := make([]model.NodeInfo, 0)
	for _, node := range registeredNodes {
		if node.NodeID.String() == "" {

			// we warn here but exit later
			invalidStakingNodes = append(invalidStakingNodes, node)
			log.Warn().
				Str("node-address", node.Address).
				Msg("missing node-id from registered nodes")
		}
	}

	// go through local nodes
	invalidNodes := make([]model.NodeInfo, 0)
	for _, node := range localNodes {
		if node.NodeID.String() == "" {

			// we warn here but exit later
			invalidNodes = append(invalidNodes, node)
			log.Warn().
				Str("node-address", node.Address).
				Msg("missing node-id from local nodes")
		}
	}

	if len(invalidNodes) != 0 || len(invalidStakingNodes) != 0 {
		log.Fatal().Msg("found missing nodes ids. fix and re-run")
	}
}

func checkMismatchingNodes(localNodes []model.NodeInfo, registeredNodes []model.NodeInfo) {

	localNodesByID := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range localNodes {
		localNodesByID[node.NodeID] = node
	}

	registeredNodesByID := make(map[flow.Identifier]model.NodeInfo)
	for _, node := range registeredNodes {
		registeredNodesByID[node.NodeID] = node
	}

	// try match local nodes to registered nodes
	invalidLocalNodes := make([]model.NodeInfo, 0)
	for _, node := range localNodes {
		if _, ok := registeredNodesByID[node.NodeID]; !ok {
			log.Warn().
				Str("local-node-id", node.NodeID.String()).
				Str("role", node.Role.String()).
				Str("address", node.Address).
				Msg("matching registered node not found for local node")
			invalidLocalNodes = append(invalidLocalNodes, node)
		}
	}

	invalidRegisteredNodes := make([]model.NodeInfo, 0)
	for _, node := range registeredNodes {
		if _, ok := localNodesByID[node.NodeID]; !ok {
			log.Warn().
				Str("registered-node-id", node.NodeID.String()).
				Str("role", node.Role.String()).
				Str("address", node.Address).
				Msg("matching local node not found for local node")
			invalidRegisteredNodes = append(invalidRegisteredNodes, node)
		}
	}

	if len(invalidLocalNodes) != 0 || len(invalidRegisteredNodes) != 0 {
		log.Fatal().Msg("found missing mismatching nodes")
	}
}

func assembleInternalNodesWithoutWeight() []model.NodeInfo {
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
			flow.DefaultInitialWeight,
			internal.NetworkPrivKey,
			internal.StakingPrivKey,
		)

		nodes = append(nodes, node)
	}

	return nodes
}

func assemblePartnerNodesWithoutWeight() []model.NodeInfo {
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

		// all nodes should have equal weight
		node := model.NewPublicNodeInfo(
			nodeID,
			n.Role,
			n.Address,
			flow.DefaultInitialWeight,
			networkPubKey,
			stakingPubKey,
		)

		publicInfoNodes = append(publicInfoNodes, node)
	}

	return publicInfoNodes
}

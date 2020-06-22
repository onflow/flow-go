package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
)

var (
	flagConfig                                       string
	flagCollectionClusters                           uint16
	flagGeneratedCollectorAddressTemplate            string
	flagGeneratedCollectorStake                      uint64
	flagPartnerNodeInfoDir                           string
	flagPartnerStakes                                string
	flagCollectorGenerationMaxHashGrindingIterations uint
	flagFastKG                                       bool
	flagStateCommitment                              string
	flagChainID                                      string
)

type PartnerStakes map[flow.Identifier]uint64

// finalizeCmd represents the finalize command
var finalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Finalize the bootstrapping process",
	Long: `Finalize the bootstrapping process, which includes generating of internal networking and staking keys,
running the DKG for the generation of the random beacon keys and generating the root block, QC, execution result
and block seal.`,
	Run: func(cmd *cobra.Command, args []string) {

		chainID := parseChainID(flagChainID)

		log.Info().Msg("✨ collecting partner network and staking keys")
		partnerNodes := assemblePartnerNodes()
		log.Info().Msg("")

		log.Info().Msg("✨ generating internal private networking and staking keys")
		internalNodes := genNetworkAndStakingKeys(partnerNodes)
		log.Info().Msg("")

		log.Info().Msg("✨ assembling network and staking keys")
		stakingNodes := mergeNodeInfos(internalNodes, partnerNodes)
		writeJSON(model.PathNodeInfosPub, model.ToPublicNodeInfoList(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("✨ running DKG for consensus nodes")
		dkgData := runDKG(model.FilterByRole(stakingNodes, flow.RoleConsensus))
		log.Info().Msg("")

		log.Info().Msg("✨ constructing root block")
		block := constructRootBlock(stakingNodes, chainID)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing root QC")
		constructRootQC(
			block,
			model.FilterByRole(stakingNodes, flow.RoleConsensus),
			model.FilterByRole(internalNodes, flow.RoleConsensus),
			dkgData,
		)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing root execution result and block seal")
		constructRootResultAndSeal(flagStateCommitment, block)
		log.Info().Msg("")

		log.Info().Msg("✨ computing collection node clusters")
		clusters := protocol.Clusters(uint(flagCollectionClusters), model.ToIdentityList(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("✨ constructing root blocks for collection node clusters")
		clusterBlocks := constructRootBlocksForClusters(clusters)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing root QCs for collection node clusters")
		constructRootQCsForClusters(clusters, internalNodes, block, clusterBlocks)
		log.Info().Msg("")

		log.Info().Msg("🌊 🏄 🤙 Done – ready to flow!")
	},
}

func init() {
	rootCmd.AddCommand(finalizeCmd)

	finalizeCmd.Flags().StringVar(&flagConfig, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Stake)")
	_ = finalizeCmd.MarkFlagRequired("config")
	finalizeCmd.Flags().Uint16Var(&flagCollectionClusters, "collection-clusters", 2,
		"number of collection clusters")
	finalizeCmd.Flags().StringVar(&flagGeneratedCollectorAddressTemplate, "generated-collector-address-template",
		"collector-%v.example.com", "address template for collector nodes that will be generated (%v "+
			"will be replaced by an index)")
	finalizeCmd.Flags().Uint64Var(&flagGeneratedCollectorStake, "generated-collector-stake", 100,
		"stake for collector nodes that will be generated")
	finalizeCmd.Flags().UintVar(&flagCollectorGenerationMaxHashGrindingIterations, "collector-gen-max-iter", 1000,
		"max hash grinding iterations for collector generation")
	finalizeCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", fmt.Sprintf("path to directory "+
		"containing one JSON file starting with %v for every partner node (fields Role, Address, NodeID, "+
		"NetworkPubKey, StakingPubKey)", model.PathPartnerNodeInfoPrefix))
	_ = finalizeCmd.MarkFlagRequired("partner-dir")
	finalizeCmd.Flags().StringVar(&flagPartnerStakes, "partner-stakes", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")
	_ = finalizeCmd.MarkFlagRequired("partner-stakes")
	finalizeCmd.Flags().BoolVar(&flagFastKG, "fast-kg", false, "use fast (centralized) random beacon key generation "+
		"instead of DKG")
	finalizeCmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "", "initial state commitment for "+
		"bootstrap execution result & block seal")
	_ = finalizeCmd.MarkFlagRequired("state-commitment")
	finalizeCmd.Flags().StringVar(&flagChainID, "chain-id", "", "chain ID for the root block (can be \"main\", \"test\" or \"emulator\"")
}

func assemblePartnerNodes() []model.NodeInfo {

	partners := readPartnerNodes()
	log.Info().Msgf("read %v partner node configuration files", len(partners))

	var stakes PartnerStakes
	readJSON(flagPartnerStakes, &stakes)
	log.Info().Msgf("read %v stakes for partner nodes", len(stakes))

	var nodes []model.NodeInfo
	for _, partner := range partners {
		// validate every single partner node
		nodeID := validateNodeID(partner.NodeID)
		networkPubKey := validateNetworkPubKey(partner.NetworkPubKey)
		stakingPubKey := validateStakingPubKey(partner.StakingPubKey)
		stake := validateStake(stakes[partner.NodeID])

		node := model.NewPublicNodeInfo(
			nodeID,
			partner.Role,
			partner.Address,
			stake,
			networkPubKey,
			stakingPubKey,
		)

		nodes = append(nodes, node)
	}

	return nodes
}

func validateNodeID(nodeID flow.Identifier) flow.Identifier {
	if nodeID == flow.ZeroID {
		log.Fatal().Msg("NodeID must not be zero")
	}
	return nodeID
}

func validateNetworkPubKey(key model.EncodableNetworkPubKey) model.EncodableNetworkPubKey {
	if key.PublicKey == nil {
		log.Fatal().Msg("NetworkPubKey must not be nil")
	}
	return key
}

func validateStakingPubKey(key model.EncodableStakingPubKey) model.EncodableStakingPubKey {
	if key.PublicKey == nil {
		log.Fatal().Msg("StakingPubKey must not be nil")
	}
	return key
}

func validateStake(stake uint64) uint64 {
	if stake == 0 {
		log.Fatal().Msg("Stake must be bigger than 0")
	}
	return stake
}

func readPartnerNodes() []model.PartnerNodeInfoPub {
	var partners []model.PartnerNodeInfoPub
	files, err := filesInDir(flagPartnerNodeInfoDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read partner node infos")
	}
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, model.PathPartnerNodeInfoPrefix) {
			continue
		}

		// read file and append to partners
		var p model.PartnerNodeInfoPub
		readJSON(f, &p)
		partners = append(partners, p)
	}
	return partners
}

func mergeNodeInfos(internalNodes, partnerNodes []model.NodeInfo) []model.NodeInfo {
	nodes := append(internalNodes, partnerNodes...)

	// test for duplicate Addresses
	addressLookup := make(map[string]struct{})
	for _, node := range nodes {
		if _, ok := addressLookup[node.Address]; ok {
			log.Fatal().Str("address", node.Address).Msg("duplicate node address")
		}
	}

	// test for duplicate node IDs
	idLookup := make(map[flow.Identifier]struct{})
	for _, node := range nodes {
		if _, ok := idLookup[node.NodeID]; ok {
			log.Fatal().Str("NodeID", node.NodeID.String()).Msg("duplicate node ID")
		}
	}

	return nodes
}

func parseChainID(chainID string) flow.ChainID {
	switch chainID {
	case "main":
		return flow.Mainnet
	case "test":
		return flow.Testnet
	case "emulator":
		return flow.Emulator
	default:
		log.Fatal().Str("chain_id", chainID).Msg("invalid chain ID")
		return ""
	}
}

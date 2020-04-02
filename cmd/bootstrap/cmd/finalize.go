package cmd

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	flagConfig                            string
	flagCollectionClusters                uint16
	flagGeneratedCollectorAddressTemplate string
	flagGeneratedCollectorStake           uint64
	flagPartnerNodeInfoDir                string
	flagPartnerStakes                     string
)

type PartnerStakes map[flow.Identifier]uint64

// finalizeCmd represents the finalize command
var finalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Finalize the bootstrapping process",
	Long: `Finalize the bootstrapping process, which includes generating of internal networking and staking keys,
running the DKG for generating the random beacon keys, generating genesis execution state, seal, block and QC.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msg("✨ collecting partner network and staking keys")
		partnerNodes := assemblePartnerNodes()
		log.Info().Msg("")

		log.Info().Msg("✨ generating internal private networking and staking keys")
		internalNodes := genNetworkAndStakingKeys(partnerNodes)
		log.Info().Msg("")

		log.Info().Msg("✨ assembling network and staking keys")
		stakingNodes := mergeNodeInfos(internalNodes, partnerNodes)
		writeJSON(model.FilenameNodeInfosPub, publicNodeInfos(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("✨ running DKG for consensus nodes")
		dkgData := runDKG(filterConsensusNodes(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("✨ generating private key for account 0 and generating genesis execution state")
		stateCommitment := genGenesisExecutionState()
		log.Info().Msg("")

		log.Info().Msg("✨ constructing genesis seal and genesis block")
		block := constructGenesisBlock(stateCommitment, stakingNodes, dkgData)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing genesis QC")
		constructGenesisQC(&block, filterConsensusNodes(stakingNodes), filterConsensusNodes(internalNodes), dkgData)
		log.Info().Msg("")

		log.Info().Msg("✨ computing collector clusters")
		clusters := computeCollectorClusters(stakingNodes)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing genesis blocks for collector clusters")
		clusterBlocks := constructGenesisBlocksForCollectorClusters(clusters)
		log.Info().Msg("")

		log.Info().Msg("✨ constructing genesis QCs for collector clusters")
		constructGenesisQCsForCollectorClusters(clusters, internalNodes, block, clusterBlocks)
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
	finalizeCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", fmt.Sprintf("path to directory "+
		"containing one JSON file ending with %v for every partner node (fields Role, Address, NodeID, "+
		"NetworkPubKey, StakingPubKey)", model.FilenamePartnerNodeInfoSuffix))
	_ = finalizeCmd.MarkFlagRequired("partner-dir")
	finalizeCmd.Flags().StringVar(&flagPartnerStakes, "partner-stakes", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")
	_ = finalizeCmd.MarkFlagRequired("partner-stakes")
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

		node := model.NewNodeInfoWithPublicKeys(
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

func readPartnerNodes() []PartnerNodeInfoPub {
	var partners []PartnerNodeInfoPub
	files, err := filesInDir(flagPartnerNodeInfoDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read partner node infos")
	}
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.HasSuffix(f, model.FilenamePartnerNodeInfoSuffix) {
			continue
		}

		// read file and append to partners
		var p PartnerNodeInfoPub
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

func publicNodeInfos(nodeInfos []model.NodeInfo) []model.NodeInfoPub {
	pub := make([]model.NodeInfoPub, 0, len(nodeInfos))
	for _, node := range nodeInfos {
		pub = append(pub, node.Public())
	}
	return pub
}

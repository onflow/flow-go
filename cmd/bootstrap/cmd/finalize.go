package cmd

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"

	"github.com/onflow/cadence"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
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
	flagRootChain                                    string
	flagRootParent                                   string
	flagRootHeight                                   uint64
	flagRootTimestamp                                string
	flagRootCommit                                   string
	flagServiceAccountPublicKeyJSON                  string
	flagGenesisTokenSupply                           string
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

		log.Info().Msg("collecting partner network and staking keys")
		partnerNodes := assemblePartnerNodes()
		log.Info().Msg("")

		log.Info().Msg("generating internal private networking and staking keys")
		internalNodes := genNetworkAndStakingKeys(partnerNodes)
		log.Info().Msg("")

		log.Info().Msg("assembling network and staking keys")
		stakingNodes := mergeNodeInfos(internalNodes, partnerNodes)
		writeJSON(model.PathNodeInfosPub, model.ToPublicNodeInfoList(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("running DKG for consensus nodes")
		dkgData := runDKG(model.FilterByRole(stakingNodes, flow.RoleConsensus))
		log.Info().Msg("")

		var commit []byte
		if flagRootCommit == "0000000000000000000000000000000000000000000000000000000000000000" {
			log.Info().Msg("generating empty execution state")

			var err error
			serviceAccountPublicKey := flow.AccountPublicKey{}
			err = serviceAccountPublicKey.UnmarshalJSON([]byte(flagServiceAccountPublicKeyJSON))
			if err != nil {
				log.Fatal().Err(err).Msg("unable to parse the service account public key json")
			}
			value, err := cadence.NewUFix64(flagGenesisTokenSupply)
			if err != nil {
				log.Fatal().Err(err).Msg("invalid genesis token supply")
			}
			commit, err = run.GenerateExecutionState(filepath.Join(flagOutdir, model.DirnameExecutionState), serviceAccountPublicKey, value, parseChainID(flagRootChain).Chain())
			if err != nil {
				log.Fatal().Err(err).Msg("unable to generate execution state")
			}
			flagRootCommit = hex.EncodeToString(commit)
			log.Info().Msg("")
		}

		log.Info().Msg("constructing root block")
		block := constructRootBlock(flagRootChain, flagRootParent, flagRootHeight, flagRootTimestamp, stakingNodes)
		log.Info().Msg("")

		log.Info().Msg("constructing root QC")
		constructRootQC(
			block,
			model.FilterByRole(stakingNodes, flow.RoleConsensus),
			model.FilterByRole(internalNodes, flow.RoleConsensus),
			dkgData,
		)
		log.Info().Msg("")

		log.Info().Msg("constructing root execution result and block seal")
		constructRootResultAndSeal(flagRootCommit, block)
		log.Info().Msg("")

		log.Info().Msg("computing collection node clusters")
		clusters := protocol.Clusters(uint(flagCollectionClusters), model.ToIdentityList(stakingNodes))
		log.Info().Msg("")

		log.Info().Msg("constructing root blocks for collection node clusters")
		clusterBlocks := constructRootBlocksForClusters(clusters)
		log.Info().Msg("")

		log.Info().Msg("constructing root QCs for collection node clusters")
		constructRootQCsForClusters(clusters, internalNodes, block, clusterBlocks)
		log.Info().Msg("")

		log.Info().Msg("saving the number of clusters")
		savingNClusters(flagCollectionClusters)
		log.Info().Msg("")

		log.Info().Msg("🌊 🏄 🤙 Done – ready to flow!")
	},
}

func init() {
	rootCmd.AddCommand(finalizeCmd)

	// required parameters for network configuration and generation of root node identities
	finalizeCmd.Flags().StringVar(&flagConfig, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Stake)")
	finalizeCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", "path to directory "+
		"containing one JSON file starting with node-info.pub.<NODE_ID>.json for every partner node (fields "+
		" in the JSON file: Role, Address, NodeID, NetworkPubKey, StakingPubKey)")
	finalizeCmd.Flags().StringVar(&flagPartnerStakes, "partner-stakes", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")

	_ = finalizeCmd.MarkFlagRequired("config")
	_ = finalizeCmd.MarkFlagRequired("partner-dir")
	_ = finalizeCmd.MarkFlagRequired("partner-stakes")

	// required parameters for generation of root block, root execution result and root block seal
	finalizeCmd.Flags().StringVar(&flagRootChain, "root-chain", "emulator", "chain ID for the root block (can be \"main\", \"test\" or \"emulator\"")
	finalizeCmd.Flags().StringVar(&flagRootParent, "root-parent", "0000000000000000000000000000000000000000000000000000000000000000", "ID for the parent of the root block")
	finalizeCmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0, "height of the root block")
	finalizeCmd.Flags().StringVar(&flagRootTimestamp, "root-timestamp", time.Now().UTC().Format(time.RFC3339), "timestamp of the root block (RFC3339)")
	finalizeCmd.Flags().StringVar(&flagRootCommit, "root-commit", "0000000000000000000000000000000000000000000000000000000000000000", "state commitment of root execution state")

	_ = finalizeCmd.MarkFlagRequired("root-chain")
	_ = finalizeCmd.MarkFlagRequired("root-parent")
	_ = finalizeCmd.MarkFlagRequired("root-height")
	_ = finalizeCmd.MarkFlagRequired("root-commit")

	// optional parameters to influence various aspects of identity generation
	finalizeCmd.Flags().Uint16Var(&flagCollectionClusters, "collection-clusters", 2,
		"number of collection clusters")
	finalizeCmd.Flags().StringVar(&flagGeneratedCollectorAddressTemplate, "generated-collector-address-template",
		"collector-%v.example.com", "address template for collector nodes that will be generated (%v "+
			"will be replaced by an index)")
	finalizeCmd.Flags().Uint64Var(&flagGeneratedCollectorStake, "generated-collector-stake", 100,
		"stake for collector nodes that will be generated")
	finalizeCmd.Flags().UintVar(&flagCollectorGenerationMaxHashGrindingIterations, "collector-gen-max-iter", 1000,
		"max hash grinding iterations for collector generation")
	finalizeCmd.Flags().BoolVar(&flagFastKG, "fast-kg", false, "use fast (centralized) random beacon key generation "+
		"instead of DKG")

	// these two flags are only used when setup a network from genesis
	finalizeCmd.Flags().StringVar(&flagServiceAccountPublicKeyJSON, "service-account-public-key-json",
		"{\"PublicKey\":\"ABCDEFGHIJK\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}",
		"encoded json of public key for the service account")
	finalizeCmd.Flags().StringVar(&flagGenesisTokenSupply, "genesis-token-supply", "10000000.00000000",
		"genesis flow token supply")
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

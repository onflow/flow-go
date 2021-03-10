package cmd

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagConfig                      string
	flagInternalNodePrivInfoDir     string
	flagCollectionClusters          uint
	flagPartnerNodeInfoDir          string
	flagPartnerStakes               string
	flagFastKG                      bool
	flagRootChain                   string
	flagRootParent                  string
	flagRootHeight                  uint64
	flagRootTimestamp               string
	flagRootCommit                  string
	flagEpochCounter                uint64
	flagServiceAccountPublicKeyJSON string
	flagGenesisTokenSupply          string
)

// PartnerStakes ...
type PartnerStakes map[flow.Identifier]uint64

// finalizeCmd represents the finalize command
var finalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Finalize the bootstrapping process",
	Long: `Finalize the bootstrapping process, which includes running the DKG for the generation of the random beacon
	keys and generating the root block, QC, execution result and block seal.`,
	Run: finalize,
}

func init() {
	rootCmd.AddCommand(finalizeCmd)
	addFinalizeCmdFlags()
}

func addFinalizeCmdFlags() {
	// required parameters for network configuration and generation of root node identities
	finalizeCmd.Flags().StringVar(&flagConfig, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Stake)")
	finalizeCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	finalizeCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", "path to directory "+
		"containing one JSON file starting with node-info.pub.<NODE_ID>.json for every partner node (fields "+
		" in the JSON file: Role, Address, NodeID, NetworkPubKey, StakingPubKey)")
	finalizeCmd.Flags().StringVar(&flagPartnerStakes, "partner-stakes", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their stake")

	_ = finalizeCmd.MarkFlagRequired("config")
	_ = finalizeCmd.MarkFlagRequired("internal-priv-dir")
	_ = finalizeCmd.MarkFlagRequired("partner-dir")
	_ = finalizeCmd.MarkFlagRequired("partner-stakes")

	// required parameters for generation of root block, root execution result and root block seal
	finalizeCmd.Flags().StringVar(&flagRootChain, "root-chain", "emulator", "chain ID for the root block (can be \"main\", \"test\" or \"emulator\"")
	finalizeCmd.Flags().StringVar(&flagRootParent, "root-parent", "0000000000000000000000000000000000000000000000000000000000000000", "ID for the parent of the root block")
	finalizeCmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0, "height of the root block")
	finalizeCmd.Flags().StringVar(&flagRootTimestamp, "root-timestamp", time.Now().UTC().Format(time.RFC3339), "timestamp of the root block (RFC3339)")
	finalizeCmd.Flags().StringVar(&flagRootCommit, "root-commit", "0000000000000000000000000000000000000000000000000000000000000000", "state commitment of root execution state")
	finalizeCmd.Flags().Uint64Var(&flagEpochCounter, "epoch-counter", 0, "epoch counter for the epoch beginning with the root block")

	_ = finalizeCmd.MarkFlagRequired("root-chain")
	_ = finalizeCmd.MarkFlagRequired("root-parent")
	_ = finalizeCmd.MarkFlagRequired("root-height")
	_ = finalizeCmd.MarkFlagRequired("root-commit")
	_ = finalizeCmd.MarkFlagRequired("epoch-counter")

	// optional parameters to influence various aspects of identity generation
	finalizeCmd.Flags().UintVar(&flagCollectionClusters, "collection-clusters", 2,
		"number of collection clusters")
	finalizeCmd.Flags().BoolVar(&flagFastKG, "fast-kg", false, "use fast (centralized) random beacon key generation "+
		"instead of DKG")

	// these two flags are only used when setup a network from genesis
	finalizeCmd.Flags().StringVar(&flagServiceAccountPublicKeyJSON, "service-account-public-key-json",
		"{\"PublicKey\":\"ABCDEFGHIJK\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}",
		"encoded json of public key for the service account")
	finalizeCmd.Flags().StringVar(&flagGenesisTokenSupply, "genesis-token-supply", "10000000.00000000",
		"genesis flow token supply")
}

func finalize(cmd *cobra.Command, args []string) {

	log.Info().Msg("collecting partner network and staking keys")
	partnerNodes := assemblePartnerNodes()
	log.Info().Msg("")

	log.Info().Msg("generating internal private networking and staking keys")
	internalNodes := assembleInternalNodes()
	log.Info().Msg("")

	log.Info().Msg("checking constraints on consensus/cluster nodes")
	checkConstraints(partnerNodes, internalNodes)
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
	block := constructRootBlock(flagRootChain, flagRootParent, flagRootHeight, flagRootTimestamp)
	log.Info().Msg("")

	log.Info().Msg("constructing root QC")
	constructRootQC(
		block,
		model.FilterByRole(stakingNodes, flow.RoleConsensus),
		model.FilterByRole(internalNodes, flow.RoleConsensus),
		dkgData,
	)
	log.Info().Msg("")

	log.Info().Msg("computing collection node clusters")
	assignments, clusters := constructClusterAssignment(partnerNodes, internalNodes)
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := run.GenerateRootClusterBlocks(flagEpochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := constructRootQCsForClusters(clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	log.Info().Msg("constructing root execution result and block seal")
	constructRootResultAndSeal(flagRootCommit, block, stakingNodes, assignments, clusterQCs, dkgData)
	log.Info().Msg("")

	// copy files only if the directories differ
	log.Info().Str("private_dir", flagInternalNodePrivInfoDir).Str("output_dir", flagOutdir).Msg("attempting to copy private key files")
	if flagInternalNodePrivInfoDir != flagOutdir {
		log.Info().Msg("copying internal private keys to output folder")
		err := copyDir(flagInternalNodePrivInfoDir, filepath.Join(flagOutdir, model.DirPrivateRoot))
		if err != nil {
			log.Error().Err(err).Msg("could not copy private key files")
		}
	} else {
		log.Info().Msg("skipping copy of private keys to output dir")
	}
	log.Info().Msg("")

	// print count of all nodes
	roleCounts := nodeCountByRole(stakingNodes)
	for role, count := range roleCounts {
		log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", count, role.String()))
	}

	log.Info().Msg("🌊 🏄 🤙 Done – ready to flow!")
}

// assemblePartnerNodes returns a list of partner nodes after gathering stake
// and public key information from configuration files
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
		stake, valid := validateStake(stakes[partner.NodeID])
		if !valid {
			log.Error().Msgf("stakes: %v", stakes)
			log.Fatal().Msgf("partner node id %v has no stake", nodeID)
		}

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

// readParnterNodes reads the partner node information
func readPartnerNodes() []model.NodeInfoPub {
	var partners []model.NodeInfoPub
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
		var p model.NodeInfoPub
		readJSON(f, &p)
		partners = append(partners, p)
	}
	return partners
}

// assembleInternalNodes returns a list of internal nodes after collecting stakes
// from configuration files
func assembleInternalNodes() []model.NodeInfo {
	privInternals := readInternalNodes()
	log.Info().Msgf("read %v internal private node-info files", len(privInternals))

	stakes := internalStakesByAddress()
	log.Info().Msgf("read %v stakes for internal nodes", len(stakes))

	var nodes []model.NodeInfo
	for _, internal := range privInternals {
		// check if address is valid format
		validateAddressFormat(internal.Address)

		// validate every single internal node
		nodeID := validateNodeID(internal.NodeID)
		stake, valid := validateStake(stakes[internal.Address])
		if !valid {
			log.Error().Msgf("stakes: %v", stakes)
			log.Fatal().Msgf("internal node %v has no stake. Did you forget to update the node address?", internal)
		}

		node := model.NewPrivateNodeInfo(
			nodeID,
			internal.Role,
			internal.Address,
			stake,
			internal.NetworkPrivKey,
			internal.StakingPrivKey,
		)

		nodes = append(nodes, node)
	}

	return nodes
}

// readInternalNodes reads our internal node private infos generated by
// `keygen` command and returns it
func readInternalNodes() []model.NodeInfoPriv {
	var internalPrivInfos []model.NodeInfoPriv

	// get files in internal priv node infos directory
	files, err := filesInDir(flagInternalNodePrivInfoDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read partner node infos")
	}

	// for each of the files
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, model.PathPrivNodeInfoPrefix) {
			continue
		}

		// read file and append to partners
		var p model.NodeInfoPriv
		readJSON(f, &p)
		internalPrivInfos = append(internalPrivInfos, p)
	}

	return internalPrivInfos
}

// internalStakesByAddress returns a mapping of node address by stake for internal nodes
func internalStakesByAddress() map[string]uint64 {
	// read json
	var configs []model.NodeConfig
	readJSON(flagConfig, &configs)
	log.Info().Msgf("read internal %v node configurations", configs)

	stakes := make(map[string]uint64)
	for _, config := range configs {
		if _, ok := stakes[config.Address]; !ok {
			stakes[config.Address] = config.Stake
		} else {
			log.Error().Msgf("duplicate internal node address %s", config.Address)
		}
	}

	return stakes
}

// mergeNodeInfos merges the inernal and partner nodes and checks if there is no
// duplicate addresses or node Ids
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

// Validation utility methods ------------------------------------------------

func validateNodeID(nodeID flow.Identifier) flow.Identifier {
	if nodeID == flow.ZeroID {
		log.Fatal().Msg("NodeID must not be zero")
	}
	return nodeID
}

func validateNetworkPubKey(key encodable.NetworkPubKey) encodable.NetworkPubKey {
	if key.PublicKey == nil {
		log.Fatal().Msg("NetworkPubKey must not be nil")
	}
	return key
}

func validateStakingPubKey(key encodable.StakingPubKey) encodable.StakingPubKey {
	if key.PublicKey == nil {
		log.Fatal().Msg("StakingPubKey must not be nil")
	}
	return key
}

func validateStake(stake uint64) (uint64, bool) {
	return stake, stake > 0
}

package cmd

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/onflow/cadence"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/fvm"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

var (
	flagConfig                  string
	flagInternalNodePrivInfoDir string
	flagCollectionClusters      uint
	flagPartnerNodeInfoDir      string
	// Deprecated: use flagPartnerWeights instead
	deprecatedFlagPartnerStakes     string
	flagPartnerWeights              string
	flagDKGDataPath                 string
	flagRootBlock                   string
	flagRootBlockVotesDir           string
	flagRootCommit                  string
	flagProtocolVersion             uint
	flagServiceAccountPublicKeyJSON string
	flagGenesisTokenSupply          string
	flagEpochCounter                uint64
	flagNumViewsInEpoch             uint64
	flagNumViewsInStakingAuction    uint64
	flagNumViewsInDKGPhase          uint64

	// this flag is used to seed the DKG, clustering and cluster QC generation
	flagBootstrapRandomSeed []byte
)

// PartnerWeights is the format of the JSON file specifying partner node weights.
type PartnerWeights map[flow.Identifier]uint64

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
		"path to a JSON file containing multiple node configurations (fields Role, Address, Weight)")
	finalizeCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	finalizeCmd.Flags().StringVar(&flagPartnerNodeInfoDir, "partner-dir", "", "path to directory "+
		"containing one JSON file starting with node-info.pub.<NODE_ID>.json for every partner node (fields "+
		" in the JSON file: Role, Address, NodeID, NetworkPubKey, StakingPubKey)")
	// Deprecated: remove this flag
	finalizeCmd.Flags().StringVar(&deprecatedFlagPartnerStakes, "partner-stakes", "", "deprecated: use partner-weights instead")
	finalizeCmd.Flags().StringVar(&flagPartnerWeights, "partner-weights", "", "path to a JSON file containing "+
		"a map from partner node's NodeID to their weight")
	finalizeCmd.Flags().StringVar(&flagDKGDataPath, "dkg-data", "", "path to a JSON file containing data as output from DKG process")

	cmd.MarkFlagRequired(finalizeCmd, "config")
	cmd.MarkFlagRequired(finalizeCmd, "internal-priv-dir")
	cmd.MarkFlagRequired(finalizeCmd, "partner-dir")
	cmd.MarkFlagRequired(finalizeCmd, "partner-weights")
	cmd.MarkFlagRequired(finalizeCmd, "dkg-data")

	// required parameters for generation of root block, root execution result and root block seal
	finalizeCmd.Flags().StringVar(&flagRootBlock, "root-block", "",
		"path to a JSON file containing root block")
	finalizeCmd.Flags().StringVar(&flagRootBlockVotesDir, "root-block-votes-dir", "", "path to directory with votes for root block")
	finalizeCmd.Flags().StringVar(&flagRootCommit, "root-commit", "0000000000000000000000000000000000000000000000000000000000000000", "state commitment of root execution state")
	finalizeCmd.Flags().Uint64Var(&flagEpochCounter, "epoch-counter", 0, "epoch counter for the epoch beginning with the root block")
	finalizeCmd.Flags().Uint64Var(&flagNumViewsInEpoch, "epoch-length", 4000, "length of each epoch measured in views")
	finalizeCmd.Flags().Uint64Var(&flagNumViewsInStakingAuction, "epoch-staking-phase-length", 100, "length of the epoch staking phase measured in views")
	finalizeCmd.Flags().Uint64Var(&flagNumViewsInDKGPhase, "epoch-dkg-phase-length", 1000, "length of each DKG phase measured in views")
	finalizeCmd.Flags().BytesHexVar(&flagBootstrapRandomSeed, "random-seed", GenerateRandomSeed(flow.EpochSetupRandomSourceLength), "The seed used to for DKG, Clustering and Cluster QC generation")
	finalizeCmd.Flags().UintVar(&flagProtocolVersion, "protocol-version", flow.DefaultProtocolVersion, "major software version used for the duration of this spork")

	cmd.MarkFlagRequired(finalizeCmd, "root-block")
	cmd.MarkFlagRequired(finalizeCmd, "root-block-votes-dir")
	cmd.MarkFlagRequired(finalizeCmd, "root-commit")
	cmd.MarkFlagRequired(finalizeCmd, "epoch-counter")
	cmd.MarkFlagRequired(finalizeCmd, "epoch-length")
	cmd.MarkFlagRequired(finalizeCmd, "epoch-staking-phase-length")
	cmd.MarkFlagRequired(finalizeCmd, "epoch-dkg-phase-length")
	cmd.MarkFlagRequired(finalizeCmd, "protocol-version")

	// optional parameters to influence various aspects of identity generation
	finalizeCmd.Flags().UintVar(&flagCollectionClusters, "collection-clusters", 2, "number of collection clusters")

	// these two flags are only used when setup a network from genesis
	finalizeCmd.Flags().StringVar(&flagServiceAccountPublicKeyJSON, "service-account-public-key-json",
		"{\"PublicKey\":\"ABCDEFGHIJK\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}",
		"encoded json of public key for the service account")
	finalizeCmd.Flags().StringVar(&flagGenesisTokenSupply, "genesis-token-supply", "10000000.00000000",
		"genesis flow token supply")
}

func finalize(cmd *cobra.Command, args []string) {

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

	log.Info().Msg("checking constraints on consensus/cluster nodes")
	checkConstraints(partnerNodes, internalNodes)
	log.Info().Msg("")

	log.Info().Msg("assembling network and staking keys")
	stakingNodes := mergeNodeInfos(internalNodes, partnerNodes)
	log.Info().Msg("")

	// create flow.IdentityList representation of participant set
	participants := model.ToIdentityList(stakingNodes).Sort(order.Canonical)

	log.Info().Msg("reading root block data")
	block := readRootBlock()
	log.Info().Msg("")

	log.Info().Msg("reading root block votes")
	votes := readRootBlockVotes()
	log.Info().Msg("")

	log.Info().Msg("reading dkg data")
	dkgData := readDKGData()
	log.Info().Msg("")

	log.Info().Msg("constructing root QC")
	rootQC := constructRootQC(
		block,
		votes,
		model.FilterByRole(stakingNodes, flow.RoleConsensus),
		model.FilterByRole(internalNodes, flow.RoleConsensus),
		dkgData,
	)
	log.Info().Msg("")

	log.Info().Msg("computing collection node clusters")
	clusterAssignmentSeed := binary.BigEndian.Uint64(flagBootstrapRandomSeed)
	assignments, clusters := constructClusterAssignment(partnerNodes, internalNodes, int64(clusterAssignmentSeed))
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := run.GenerateRootClusterBlocks(flagEpochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := constructRootQCsForClusters(clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	// if no root commit is specified, bootstrap an empty execution state
	if flagRootCommit == "0000000000000000000000000000000000000000000000000000000000000000" {
		generateEmptyExecutionState(
			block.Header.ChainID,
			flagBootstrapRandomSeed,
			assignments,
			clusterQCs,
			dkgData,
			participants,
		)
	}

	log.Info().Msg("constructing root execution result and block seal")
	result, seal := constructRootResultAndSeal(flagRootCommit, block, participants, assignments, clusterQCs, dkgData)
	log.Info().Msg("")

	// construct serializable root protocol snapshot
	log.Info().Msg("constructing root protocol snapshot")
	snapshot, err := inmem.SnapshotFromBootstrapStateWithProtocolVersion(block, result, seal, rootQC, flagProtocolVersion)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate root protocol snapshot")
	}

	// validate the generated root snapshot is valid
	verifyResultID := true
	err = badger.IsValidRootSnapshot(snapshot, verifyResultID)
	if err != nil {
		log.Fatal().Err(err).Msg("the generated root snapshot is invalid")
	}

	// validate the generated root snapshot QCs
	err = badger.IsValidRootSnapshotQCs(snapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("root snapshot contains invalid QCs")
	}

	// write snapshot to disk
	writeJSON(model.PathRootProtocolStateSnapshot, snapshot.Encodable())
	log.Info().Msg("")

	// read snapshot and verify consistency
	rootSnapshot, err := loadRootProtocolSnapshot(model.PathRootProtocolStateSnapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to load seralized root protocol")
	}

	savedResult, savedSeal, err := rootSnapshot.SealedResult()
	if err != nil {
		log.Fatal().Err(err).Msg("could not load sealed result")
	}

	if savedSeal.ID() != seal.ID() {
		log.Fatal().Msgf("inconsistent seralization of the root seal: %v != %v", savedSeal.ID(), seal.ID())
	}

	if savedResult.ID() != result.ID() {
		log.Fatal().Msgf("inconsistent seralization of the root result: %v != %v", savedResult.ID(), result.ID())
	}

	if savedSeal.ResultID != savedResult.ID() {
		log.Fatal().Msgf("mismatch saved seal's resultID  %v and result %v", savedSeal.ResultID, savedResult.ID())
	}

	log.Info().Msg("saved result and seal are matching")

	err = badger.IsValidRootSnapshot(rootSnapshot, verifyResultID)
	if err != nil {
		log.Fatal().Err(err).Msg("saved snapshot is invalid")
	}

	// validate the generated root snapshot QCs
	err = badger.IsValidRootSnapshotQCs(snapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("root snapshot contains invalid QCs")
	}

	log.Info().Msgf("saved root snapshot is valid")

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
	log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", roleCounts[flow.RoleConsensus], flow.RoleConsensus.String()))
	log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", roleCounts[flow.RoleCollection], flow.RoleCollection.String()))
	log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", roleCounts[flow.RoleVerification], flow.RoleVerification.String()))
	log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", roleCounts[flow.RoleExecution], flow.RoleExecution.String()))
	log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", roleCounts[flow.RoleAccess], flow.RoleAccess.String()))

	log.Info().Msg("🌊 🏄 🤙 Done – ready to flow!")
}

// readRootBlockVotes reads votes for root block
func readRootBlockVotes() []*hotstuff.Vote {
	var votes []*hotstuff.Vote
	files, err := filesInDir(flagRootBlockVotesDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read root block votes")
	}
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, model.FilenameRootBlockVotePrefix) {
			continue
		}

		// read file and append to partners
		var vote hotstuff.Vote
		readJSON(f, &vote)
		votes = append(votes, &vote)
		log.Info().Msgf("read vote %v for block %v from signerID %v", vote.ID(), vote.BlockID, vote.SignerID)
	}
	return votes
}

// readPartnerNodeInfos returns a list of partner nodes after gathering weights
// and public key information from configuration files
func readPartnerNodeInfos() []model.NodeInfo {
	partners := readPartnerNodes()
	log.Info().Msgf("read %d partner node configuration files", len(partners))

	var weights PartnerWeights
	readJSON(flagPartnerWeights, &weights)
	log.Info().Msgf("read %d weights for partner nodes", len(weights))

	var nodes []model.NodeInfo
	for _, partner := range partners {
		// validate every single partner node
		nodeID := validateNodeID(partner.NodeID)
		networkPubKey := validateNetworkPubKey(partner.NetworkPubKey)
		stakingPubKey := validateStakingPubKey(partner.StakingPubKey)
		weight, valid := validateWeight(weights[partner.NodeID])
		if !valid {
			log.Error().Msgf("weights: %v", weights)
			log.Fatal().Msgf("partner node id %x has no weight", nodeID)
		}
		if weight != flow.DefaultInitialWeight {
			log.Warn().Msgf("partner node (id=%x) has non-default weight (%d != %d)", partner.NodeID, weight, flow.DefaultInitialWeight)
		}

		node := model.NewPublicNodeInfo(
			nodeID,
			partner.Role,
			partner.Address,
			weight,
			networkPubKey.PublicKey,
			stakingPubKey.PublicKey,
		)
		nodes = append(nodes, node)
	}

	return nodes
}

// readPartnerNodes reads the partner node information
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

// readInternalNodeInfos returns a list of internal nodes after collecting weights
// from configuration files
func readInternalNodeInfos() []model.NodeInfo {
	privInternals := readInternalNodes()
	log.Info().Msgf("read %v internal private node-info files", len(privInternals))

	weights := internalWeightsByAddress()
	log.Info().Msgf("read %d weights for internal nodes", len(weights))

	var nodes []model.NodeInfo
	for _, internal := range privInternals {
		// check if address is valid format
		validateAddressFormat(internal.Address)

		// validate every single internal node
		nodeID := validateNodeID(internal.NodeID)
		weight, valid := validateWeight(weights[internal.Address])
		if !valid {
			log.Error().Msgf("weights: %v", weights)
			log.Fatal().Msgf("internal node %v has no weight. Did you forget to update the node address?", internal)
		}
		if weight != flow.DefaultInitialWeight {
			log.Warn().Msgf("internal node (id=%x) has non-default weight (%d != %d)", internal.NodeID, weight, flow.DefaultInitialWeight)
		}

		node := model.NewPrivateNodeInfo(
			nodeID,
			internal.Role,
			internal.Address,
			weight,
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

// internalWeightsByAddress returns a mapping of node address by weight for internal nodes
func internalWeightsByAddress() map[string]uint64 {
	// read json
	var configs []model.NodeConfig
	readJSON(flagConfig, &configs)
	log.Info().Interface("config", configs).Msgf("read internal node configurations")

	weights := make(map[string]uint64)
	for _, config := range configs {
		if _, ok := weights[config.Address]; !ok {
			weights[config.Address] = config.Weight
		} else {
			log.Error().Msgf("duplicate internal node address %s", config.Address)
		}
	}

	return weights
}

// mergeNodeInfos merges the internal and partner nodes and checks if there are no
// duplicate addresses or node Ids.
//
// IMPORTANT: node infos are returned in the canonical ordering, meaning this
// is safe to use as the input to the DKG and protocol state.
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

	// sort nodes using the canonical ordering
	nodes = model.Sort(nodes, order.Canonical)

	return nodes
}

// readRootBlock reads root block data from disc, this file needs to be prepared with
// rootblock command
func readRootBlock() *flow.Block {
	rootBlock, err := utils.ReadRootBlock(flagRootBlock)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read root block data")
	}
	return rootBlock
}

func readDKGData() dkg.DKGData {
	encodableDKG, err := utils.ReadDKGData(flagDKGDataPath)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read DKG data")
	}

	dkgData := dkg.DKGData{
		PrivKeyShares: nil,
		PubGroupKey:   encodableDKG.GroupKey.PublicKey,
		PubKeyShares:  nil,
	}

	for _, pubKey := range encodableDKG.PubKeyShares {
		dkgData.PubKeyShares = append(dkgData.PubKeyShares, pubKey.PublicKey)
	}

	for _, privKey := range encodableDKG.PrivKeyShares {
		dkgData.PrivKeyShares = append(dkgData.PrivKeyShares, privKey.PrivateKey)
	}

	return dkgData
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

func validateWeight(weight uint64) (uint64, bool) {
	return weight, weight > 0
}

// loadRootProtocolSnapshot loads the root protocol snapshot from disk
func loadRootProtocolSnapshot(path string) (*inmem.Snapshot, error) {
	data, err := io.ReadFile(filepath.Join(flagOutdir, path))
	if err != nil {
		return nil, err
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return inmem.SnapshotFromEncodable(snapshot), nil
}

// generateEmptyExecutionState generates a new empty execution state with the
// given configuration. Sets the flagRootCommit variable for future reads.
func generateEmptyExecutionState(
	chainID flow.ChainID,
	randomSource []byte,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData,
	identities flow.IdentityList,
) (commit flow.StateCommitment) {

	log.Info().Msg("generating empty execution state")
	var serviceAccountPublicKey flow.AccountPublicKey
	err := serviceAccountPublicKey.UnmarshalJSON([]byte(flagServiceAccountPublicKeyJSON))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse the service account public key json")
	}

	cdcInitialTokenSupply, err := cadence.NewUFix64(flagGenesisTokenSupply)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid genesis token supply")
	}

	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid random source")
	}

	epochConfig := epochs.EpochConfig{
		EpochTokenPayout:             cadence.UFix64(0),
		RewardCut:                    cadence.UFix64(0),
		CurrentEpochCounter:          cadence.UInt64(flagEpochCounter),
		NumViewsInEpoch:              cadence.UInt64(flagNumViewsInEpoch),
		NumViewsInStakingAuction:     cadence.UInt64(flagNumViewsInStakingAuction),
		NumViewsInDKGPhase:           cadence.UInt64(flagNumViewsInDKGPhase),
		NumCollectorClusters:         cadence.UInt16(flagCollectionClusters),
		FLOWsupplyIncreasePercentage: cadence.UFix64(0),
		RandomSource:                 cdcRandomSource,
		CollectorClusters:            assignments,
		ClusterQCs:                   clusterQCs,
		DKGPubKeys:                   dkgData.PubKeyShares,
	}

	commit, err = run.GenerateExecutionState(
		filepath.Join(flagOutdir, model.DirnameExecutionState),
		serviceAccountPublicKey,
		chainID.Chain(),
		fvm.WithInitialTokenSupply(cdcInitialTokenSupply),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithEpochConfig(epochConfig),
		fvm.WithIdentities(identities),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate execution state")
	}
	flagRootCommit = hex.EncodeToString(commit[:])
	log.Info().Msg("")
	return
}

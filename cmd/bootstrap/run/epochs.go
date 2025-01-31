package run

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/bootstrap"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// GenerateRecoverEpochTxArgs generates the required transaction arguments for the `recoverEpoch` transaction.
// No errors are expected during normal operation.
func GenerateRecoverEpochTxArgs(log zerolog.Logger,
	internalNodePrivInfoDir string,
	nodeConfigJson string,
	collectionClusters int,
	recoveryEpochCounter uint64,
	rootChainID flow.ChainID,
	numViewsInStakingAuction uint64,
	numViewsInEpoch uint64,
	recoveryEpochTargetDuration uint64,
	unsafeAllowOverWrite bool,
	excludeNodeIDs flow.IdentifierList, // applied as set-minus operation
	includeNodeIDs flow.IdentifierList, // applied as set-union operation
	snapshot *inmem.Snapshot,
) ([]cadence.Value, error) {
	log.Info().Msg("collecting internal node network and staking keys")
	internalNodes, err := common.ReadFullInternalNodeInfos(log, internalNodePrivInfoDir, nodeConfigJson)
	if err != nil {
		return nil, fmt.Errorf("failed to read full internal node infos: %w", err)
	}

	epochProtocolState, err := snapshot.EpochProtocolState()
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch protocol state from snapshot: %w", err)
	}
	currentEpochCommit := epochProtocolState.EpochCommit()

	return GenerateRecoverTxArgsWithDKG(
		log,
		internalNodes,
		collectionClusters,
		recoveryEpochCounter,
		rootChainID,
		numViewsInStakingAuction,
		numViewsInEpoch,
		recoveryEpochTargetDuration,
		unsafeAllowOverWrite,
		currentEpochCommit.DKGIndexMap,
		currentEpochCommit.DKGParticipantKeys,
		currentEpochCommit.DKGGroupKey,
		excludeNodeIDs,
		includeNodeIDs,
		snapshot,
	)
}

// GenerateRecoverTxArgsWithDKG generates the required transaction arguments for the `recoverEpoch` transaction.
// No errors are expected during normal operation.
func GenerateRecoverTxArgsWithDKG(log zerolog.Logger,
	internalNodes []bootstrap.NodeInfo,
	collectionClusters int,
	recoveryEpochCounter uint64,
	rootChainID flow.ChainID,
	numViewsInStakingAuction uint64,
	numViewsInEpoch uint64,
	recoveryEpochTargetDuration uint64,
	unsafeAllowOverWrite bool,
	dkgIndexMap flow.DKGIndexMap,
	dkgParticipantKeys []crypto.PublicKey,
	dkgGroupKey crypto.PublicKey,
	excludeNodeIDs flow.IdentifierList, // applied as set-minus operation
	includeNodeIDs flow.IdentifierList, // applied as set-union operation
	snapshot *inmem.Snapshot,
) ([]cadence.Value, error) {
	epoch := snapshot.Epochs().Current()
	currentEpochCounter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch counter: %w", err)
	}
	if recoveryEpochCounter != currentEpochCounter+1 {
		return nil, fmt.Errorf("invalid recovery epoch counter, expect %d", currentEpochCounter+1)
	}
	currentEpochPhase, err := snapshot.EpochPhase()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch phase for snapshot: %w", err)
	}
	if currentEpochPhase == flow.EpochPhaseCommitted {
		return nil, fmt.Errorf("next epoch has been already committed, will not build recovery transaction")
	}

	// including only (conjunction):
	//  * nodes authorized to actively participate in the current epoch (i.e. excluding ejected, joining or leaving nodes)
	//  * with _positive_ weight,
	//  * nodes that were not explicitly excluded
	eligibleEpochIdentities, err := snapshot.Identities(filter.And(
		filter.IsValidCurrentEpochParticipant,
		filter.HasWeightGreaterThanZero[flow.Identity],
		filter.Not(filter.HasNodeID[flow.Identity](excludeNodeIDs...))))
	if err != nil {
		return nil, fmt.Errorf("failed to get valid protocol participants from snapshot: %w", err)
	}
	// We expect canonical ordering here, because the Identities are originating from a protocol state snapshot,
	// which by protocol convention maintains the Identities in canonical order. Removing elements from a
	// canonically-ordered list, still retains canonical ordering. Sanity check to enforce this:
	if !eligibleEpochIdentities.Sorted(flow.Canonical[flow.Identity]) {
		return nil, fmt.Errorf("identies from snapshot not in canonical order")
	}
	// It would be contradictory if both `excludeNodeIDs` and `includeNodeIDs` contained the same ID.
	// Specifically, we expect the set intersection between `excludeNodeIDs` and `includeNodeIDs` to
	// be empty. To prevent first removing a node and then adding it back, we sanity-check consistency:
	includeIDsLookup := includeNodeIDs.Lookup()
	for _, id := range excludeNodeIDs {
		if _, found := includeIDsLookup[id]; found {
			return nil, fmt.Errorf("contradictory input: node ID %s is listed in both includeNodeIDs and excludeNodeIDs", id)
		}
	}

	// STEP I: compile Cluster Assignment, Cluster Root Blocks and QCs
	//         which are needed to initiate each cluster's consensus process
	// ─────────────────────────────────────────────────────────────────────────────────────────────
	// SHORTCUT, must be removed for full Collector Node decentralization.
	// Currently, we are assuming access to a supermajority of Collector Nodes' private staking keys.
	// In the future, this part needs to be refactored. At maturity, we need a supermajority of
	// Collector nodes' votes on their respective cluster root block. The cluster root block can be
	// deterministically generated by each Collector locally - though the root block for each cluster
	// is different (for BFT reasons). Hence, the Collectors must be given the cluster-assignment
	// upfront in order to generate their cluster's root block and vote for it.
	// GenerateRecoverEpochTxArgs could aggregate the votes to a QC or alternatively the QC could
	// be passed in as an input to GenerateRecoverEpochTxArgs (with vote collection and aggregation
	// happening e.g. via a smart contract or mediated by the Flow Foundation. Either way, we
	// need a QC for each cluster's root block in order to initiate each cluster's consensus process.

	// separate collector nodes by internal and partner nodes
	collectors := eligibleEpochIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := make(flow.IdentityList, 0)
	partnerCollectors := make(flow.IdentityList, 0)

	internalNodesMap := make(map[flow.Identifier]struct{})
	for _, node := range internalNodes {
		if !eligibleEpochIdentities.Exists(node.Identity()) {
			log.Warn().Msgf("node with ID %s is not part of the network according to the bootstrapping data; we might not get any data", node.NodeID)
		}
		internalNodesMap[node.NodeID] = struct{}{}
	}

	for _, collector := range collectors {
		if _, ok := internalNodesMap[collector.NodeID]; ok {
			internalCollectors = append(internalCollectors, collector)
		} else {
			partnerCollectors = append(partnerCollectors, collector)
		}
	}

	log.Info().Msg("computing collection node clusters")
	assignments, clusters, err := common.ConstructClusterAssignment(log, partnerCollectors, internalCollectors, collectionClusters)
	if err != nil {
		return nil, fmt.Errorf("unable to generate cluster assignment: %w", err)
	}
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := GenerateRootClusterBlocks(recoveryEpochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	// STEP II: Public Key Vector with Random Beacon public keys
	//          determines which consensus nodes can contribute to the Random Beacon during the Recovery Epoch
	// ─────────────────────────────────────────────────────────────────────────────────────────────────────────────────
	// Context: recovering from Epoch Fallback Mode requires that a sufficiency large fraction of consensus participants
	// has valid random beacon keys (threshold signature scheme). The specific origin of those threshold keys is largely
	// irrelevant. Running a centralized key generation process, using keys from an off-chain DKG, or reusing the random
	// beacon keys from a prior epoch are all conceptually possible - provided the intersection between the consensus
	// committee and the random beacon committee is large enough (for liveness).
	// Implemented here:
	// In a nutshell, we are carrying the current consensus and collector nodes forward into the next epoch (the Recovery
	// Epoch). Removing or adding a small number of nodes here would be possible, but is not implemented at the moment.
	// In all cases, a core requirement for liveness is: the fraction of consensus participants in the recovery epoch with
	// valid random beacon key should be significantly larger than the threshold of the threshold-cryptography scheme.
	// The EFM Recovery State Machine will heuristically reject recovery attempts (specifically reject EpochRecover Service
	// events, when the intersection between consensus and random beacon committees is too small.

	// NOTE: The RecoveryEpoch will re-use the last successful DKG output. This means that the random beacon committee can be
	// different from the consensus committee. This could happen if the node was ejected from the consensus committee, but it still has to be
	// included in the DKG committee since the threshold signature scheme operates on pre-defined number of participants and cannot be changed.
	dkgGroupKeyCdc, cdcErr := cadence.NewString(hex.EncodeToString(dkgGroupKey.Encode()))
	if cdcErr != nil {
		return nil, fmt.Errorf("failed to convert Random Beacon group key to cadence representation: %w", cdcErr)
	}

	// copy DKG index map from the current epoch
	dkgIndexMapPairs := make([]cadence.KeyValuePair, 0)
	for nodeID, index := range dkgIndexMap {
		dkgIndexMapPairs = append(dkgIndexMapPairs, cadence.KeyValuePair{
			Key:   cadence.String(nodeID.String()),
			Value: cadence.NewInt(index),
		})
	}
	// copy DKG public keys from the current epoch
	dkgPubKeys := make([]cadence.Value, 0)
	for k, dkgPubKey := range dkgParticipantKeys {
		dkgPubKeyCdc, cdcErr := cadence.NewString(hex.EncodeToString(dkgPubKey.Encode()))
		if cdcErr != nil {
			return nil, fmt.Errorf("failed convert public beacon key of participant %d to cadence representation: %w", k, cdcErr)
		}
		dkgPubKeys = append(dkgPubKeys, dkgPubKeyCdc)
	}
	// Compile list of NodeIDs that are allowed to participate in the recovery epoch:
	//   (i) eligible node IDs from the Epoch that the input `snapshot` is from
	//  (ii) node IDs (manually) specified in `includeNodeIDs`
	// We use the set union to combine (i) and (ii). Important: the resulting list of node IDs must be canonically ordered!
	nodeIds := make([]cadence.Value, 0)
	unionIds := eligibleEpochIdentities.NodeIDs().Union(includeNodeIDs)
	// CAUTION: unionIDs may not be canonically ordered anymore, due to set union
	for _, id := range unionIds.Sort(flow.IdentifierCanonical) {
		nodeIdCdc, err := cadence.NewString(id.String())
		if err != nil {
			return nil, fmt.Errorf("failed to convert node ID %s to cadence string: %w", id, err)
		}
		nodeIds = append(nodeIds, nodeIdCdc)
	}

	clusterQCAddress := systemcontracts.SystemContractsForChain(rootChainID).ClusterQC.Address.String()
	qcVoteData, err := common.ConvertClusterQcsCdc(clusterQCs, clusters, clusterQCAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cluster qcs to cadence type")
	}
	currEpochFinalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("failed to get final view of current epoch")
	}
	currEpochTargetEndTime, err := epoch.TargetEndTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get target end time of current epoch")
	}

	// STEP III: compile list of arguments for `recoverEpoch` governance transaction.
	// ───────────────────────────────────────────────────────────────────────────────
	// order of arguments are taken from Cadence transaction defined in core-contracts repo: https://github.com/onflow/flow-core-contracts/blob/807cf69d387d9a46b50fb4b8784a43ce9c2c0471/transactions/epoch/admin/recover_epoch.cdc#L16
	args := []cadence.Value{
		// recovery epoch counter
		cadence.NewUInt64(recoveryEpochCounter),
		// epoch start view
		cadence.NewUInt64(currEpochFinalView + 1),
		// staking phase end view
		cadence.NewUInt64(currEpochFinalView + numViewsInStakingAuction),
		// epoch end view
		cadence.NewUInt64(currEpochFinalView + numViewsInEpoch),
		// recovery epoch target duration
		cadence.NewUInt64(recoveryEpochTargetDuration),
		// target end time
		cadence.NewUInt64(currEpochTargetEndTime + recoveryEpochTargetDuration),
		// clusters,
		common.ConvertClusterAssignmentsCdc(assignments),
		// cluster qcVoteData
		cadence.NewArray(qcVoteData),
		// dkg pub keys
		cadence.NewArray(dkgPubKeys),
		// dkg group key,
		dkgGroupKeyCdc,
		// dkg index map
		cadence.NewDictionary(dkgIndexMapPairs),
		// node ids
		cadence.NewArray(nodeIds),
		// recover the network by initializing a new recover epoch which will increment the smart contract epoch counter
		// or overwrite the epoch metadata for the current epoch
		cadence.NewBool(unsafeAllowOverWrite),
	}

	return args, nil
}

// ConstructRootQCsForClusters constructs a root QC for each cluster in the list.
// Args:
// - log: the logger instance.
// - clusterList: list of clusters
// - nodeInfos: list of NodeInfos (must contain all internal nodes)
// - clusterBlocks: list of root blocks (one for each cluster)
// Returns:
// - flow.AssignmentList: the generated assignment list.
// - flow.ClusterList: the generate collection cluster list.
func ConstructRootQCsForClusters(log zerolog.Logger, clusterList flow.ClusterList, nodeInfos []bootstrap.NodeInfo, clusterBlocks []*cluster.Block) []*flow.QuorumCertificate {
	if len(clusterBlocks) != len(clusterList) {
		log.Fatal().Int("len(clusterBlocks)", len(clusterBlocks)).Int("len(clusterList)", len(clusterList)).
			Msg("number of clusters needs to equal number of cluster blocks")
	}

	qcs := make([]*flow.QuorumCertificate, len(clusterBlocks))
	for i, cluster := range clusterList {
		signers := filterClusterSigners(cluster, nodeInfos)

		qc, err := GenerateClusterRootQC(signers, cluster, clusterBlocks[i])
		if err != nil {
			log.Fatal().Err(err).Int("cluster index", i).Msg("generating collector cluster root QC failed")
		}
		qcs[i] = qc
	}

	return qcs
}

// Filters a list of nodes to include only nodes that will sign the QC for the
// given cluster. The resulting list of nodes is only nodes that are in the
// given cluster AND are not partner nodes (ie. we have the private keys).
func filterClusterSigners(cluster flow.IdentitySkeletonList, nodeInfos []model.NodeInfo) []model.NodeInfo {
	var filtered []model.NodeInfo
	for _, node := range nodeInfos {
		_, isInCluster := cluster.ByNodeID(node.NodeID)
		isPrivateKeyAvailable := node.Type() == model.NodeInfoTypePrivate

		if isInCluster && isPrivateKeyAvailable {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

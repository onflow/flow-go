package run

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
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
func GenerateRecoverEpochTxArgs(log zerolog.Logger,
	internalNodePrivInfoDir string,
	nodeConfigJson string,
	collectionClusters int,
	epochCounter uint64,
	rootChainID flow.ChainID,
	numViewsInStakingAuction uint64,
	numViewsInEpoch uint64,
	targetDuration uint64,
	targetEndTime uint64,
	initNewEpoch bool,
	snapshot *inmem.Snapshot,
) ([]cadence.Value, error) {
	epoch := snapshot.Epochs().Current()

	currentEpochIdentities, err := snapshot.Identities(filter.IsValidProtocolParticipant)
	if err != nil {
		return nil, fmt.Errorf("failed to get  valid protocol participants from snapshot: %w", err)
	}
	// We need canonical ordering here; sanity check to enforce this:
	if !currentEpochIdentities.Sorted(flow.Canonical[flow.Identity]) {
		return nil, fmt.Errorf("identies from snapshot not in canonical order")
	}

	// separate collector nodes by internal and partner nodes
	collectors := currentEpochIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := make(flow.IdentityList, 0)
	partnerCollectors := make(flow.IdentityList, 0)

	log.Info().Msg("collecting internal node network and staking keys")
	internalNodes, err := common.ReadFullInternalNodeInfos(log, internalNodePrivInfoDir, nodeConfigJson)
	if err != nil {
		return nil, fmt.Errorf("failed to read full internal node infos: %w", err)
	}

	internalNodesMap := make(map[flow.Identifier]struct{})
	for _, node := range internalNodes {
		if !currentEpochIdentities.Exists(node.Identity()) {
			return nil, fmt.Errorf("node ID found in internal node infos missing from protocol snapshot identities %s: %w", node.NodeID, err)
		}
		internalNodesMap[node.NodeID] = struct{}{}
	}
	log.Info().Msg("")

	for _, collector := range collectors {
		if _, ok := internalNodesMap[collector.NodeID]; ok {
			internalCollectors = append(internalCollectors, collector)
		} else {
			partnerCollectors = append(partnerCollectors, collector)
		}
	}

	currentEpochDKG, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("failed to get DKG for current epoch: %w", err)
	}

	log.Info().Msg("computing collection node clusters")

	assignments, clusters, err := common.ConstructClusterAssignment(log, partnerCollectors, internalCollectors, collectionClusters)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate cluster assignment")
	}
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := GenerateRootClusterBlocks(epochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	dkgPubKeys := make([]cadence.Value, 0)
	nodeIds := make([]cadence.Value, 0)

	// NOTE: The RecoveryEpoch will re-use the last successful DKG output. This means that the consensus
	// committee in the RecoveryEpoch must be identical to the committee which participated in that DKG.
	dkgGroupKeyCdc, cdcErr := cadence.NewString(currentEpochDKG.GroupKey().String())
	if cdcErr != nil {
		log.Fatal().Err(cdcErr).Msg("failed to get dkg group key cadence string")
	}
	dkgPubKeys = append(dkgPubKeys, dkgGroupKeyCdc)
	for _, id := range currentEpochIdentities {
		if id.GetRole() == flow.RoleConsensus {
			dkgPubKey, keyShareErr := currentEpochDKG.KeyShare(id.GetNodeID())
			if keyShareErr != nil {
				log.Fatal().Err(keyShareErr).Msg(fmt.Sprintf("failed to get dkg pub key share for node: %s", id.GetNodeID()))
			}
			dkgPubKeyCdc, cdcErr := cadence.NewString(dkgPubKey.String())
			if cdcErr != nil {
				log.Fatal().Err(cdcErr).Msg(fmt.Sprintf("failed to get dkg pub key cadence string for node: %s", id.GetNodeID()))
			}
			dkgPubKeys = append(dkgPubKeys, dkgPubKeyCdc)
		}
		nodeIdCdc, err := cadence.NewString(id.GetNodeID().String())
		if err != nil {
			log.Fatal().Err(err).Msg(fmt.Sprintf("failed to convert node ID to cadence string: %s", id.GetNodeID()))
		}
		nodeIds = append(nodeIds, nodeIdCdc)
	}
	clusterQCAddress := systemcontracts.SystemContractsForChain(rootChainID).ClusterQC.Address.String()
	qcVoteData, err := common.ConvertClusterQcsCdc(clusterQCs, clusters, clusterQCAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert cluster qcs to cadence type")
	}

	currEpochFinalView, err := epoch.FinalView()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get final view of current epoch")
	}

	args := []cadence.Value{
		// epoch start view
		cadence.NewUInt64(currEpochFinalView + 1),
		// staking phase end view
		cadence.NewUInt64(currEpochFinalView + numViewsInStakingAuction),
		// epoch end view
		cadence.NewUInt64(currEpochFinalView + numViewsInEpoch),
		// target duration
		cadence.NewUInt64(targetDuration),
		// target end time
		cadence.NewUInt64(targetEndTime),
		// clusters,
		common.ConvertClusterAssignmentsCdc(assignments),
		// qcVoteData
		cadence.NewArray(qcVoteData),
		// dkg pub keys
		cadence.NewArray(dkgPubKeys),
		// node ids
		cadence.NewArray(nodeIds),
		// recover the network by initializing a new recover epoch which will increment the smart contract epoch counter
		// or overwrite the epoch metadata for the current epoch
		cadence.NewBool(initNewEpoch),
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

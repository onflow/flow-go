package common

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/bootstrap"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/assignment"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ConstructClusterAssignment random cluster assignment with internal and partner nodes.
// The number of nodes in each cluster is deterministic and only depends on the number of clusters
// and the number of nodes. The repartition of internal and partner nodes is also deterministic
// and only depends on the number of clusters and nodes.
// The identity of internal and partner nodes in each cluster is the non-deterministic and is randomized
// using the system entropy.
// The function guarantees a specific constraint when partitioning the nodes into clusters:
// Each cluster must contain strictly more than 2/3 of internal nodes. If the constraint can't be
// satisfied, an exception is returned.
// Note that if an exception is returned with a certain number of internal/partner nodes, there is no chance
// of succeeding the assignment by re-running the function without increasing the internal nodes ratio.
// Args:
// - log: the logger instance.
// - partnerNodes: identity list of partner nodes.
// - internalNodes: identity list of internal nodes.
// - numCollectionClusters: the number of clusters to generate
// Returns:
// - flow.AssignmentList: the generated assignment list.
// - flow.ClusterList: the generate collection cluster list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ConstructClusterAssignment(log zerolog.Logger, partnerNodes, internalNodes flow.IdentityList, numCollectionClusters int) (flow.AssignmentList, flow.ClusterList, error) {

	partners := partnerNodes.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internals := internalNodes.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	nCollectors := len(partners) + len(internals)

	// ensure we have at least as many collection nodes as clusters
	if nCollectors < int(numCollectionClusters) {
		log.Fatal().Msgf("network bootstrap is configured with %d collection nodes, but %d clusters - must have at least one collection node per cluster",
			nCollectors, numCollectionClusters)
	}

	// shuffle both collector lists based on a non-deterministic algorithm
	partners, err := partners.Shuffle()
	if err != nil {
		log.Fatal().Err(err).Msg("could not shuffle partners")
	}
	internals, err = internals.Shuffle()
	if err != nil {
		log.Fatal().Err(err).Msg("could not shuffle internals")
	}

	identifierLists := make([]flow.IdentifierList, numCollectionClusters)
	// array to track the 2/3 internal-nodes constraint (internal_nodes > 2 * partner_nodes)
	constraint := make([]int, numCollectionClusters)

	// first, round-robin internal nodes into each cluster
	for i, node := range internals {
		clusterIndex := i % numCollectionClusters
		identifierLists[clusterIndex] = append(identifierLists[clusterIndex], node.NodeID)
		constraint[clusterIndex] += 1
	}

	// next, round-robin partner nodes into each cluster
	for i, node := range partners {
		clusterIndex := i % numCollectionClusters
		identifierLists[clusterIndex] = append(identifierLists[clusterIndex], node.NodeID)
		constraint[clusterIndex] -= 2
	}

	// check the 2/3 constraint: for every cluster `i`, constraint[i] must be strictly positive
	for i := 0; i < numCollectionClusters; i++ {
		if constraint[i] <= 0 {
			return nil, nil, errors.New("there isn't enough internal nodes to have at least 2/3 internal nodes in each cluster")
		}
	}

	assignments := assignment.FromIdentifierLists(identifierLists)

	collectors := append(partners, internals...)
	clusters, err := factory.NewClusterList(assignments, collectors.ToSkeleton())
	if err != nil {
		log.Fatal().Err(err).Msg("could not create cluster list")
	}

	return assignments, clusters, nil
}

// ConstructRootQCsForClusters constructs a root QC for each cluster in the list.
// Args:
// - log: the logger instance.
// - clusterList: list of clusters
// - nodeInfos: list of NodeInfos (must contain all internal nodes)
// - clusterBlocks: list of root blocks for each cluster
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

		qc, err := run.GenerateClusterRootQC(signers, cluster, clusterBlocks[i])
		if err != nil {
			log.Fatal().Err(err).Int("cluster index", i).Msg("generating collector cluster root QC failed")
		}
		qcs[i] = qc
	}

	return qcs
}

// ConvertClusterAssignmentsCdc converts golang cluster assignments type to Cadence type `[[String]]`.
func ConvertClusterAssignmentsCdc(assignments flow.AssignmentList) cadence.Array {
	assignmentsCdc := make([]cadence.Value, len(assignments))
	for i, asmt := range assignments {
		vals := make([]cadence.Value, asmt.Len())
		for j, nodeID := range asmt {
			vals[j] = cadence.String(nodeID.String())
		}
		assignmentsCdc[i] = cadence.NewArray(vals).WithType(cadence.NewVariableSizedArrayType(cadence.StringType{}))
	}

	return cadence.NewArray(assignmentsCdc).WithType(cadence.NewVariableSizedArrayType(cadence.NewVariableSizedArrayType(cadence.StringType{})))
}

// ConvertClusterQcsCdc converts cluster QCs from `QuorumCertificate` type to `ClusterQCVoteData` type.
func ConvertClusterQcsCdc(qcs []*flow.QuorumCertificate, clusterList flow.ClusterList) ([]*flow.ClusterQCVoteData, error) {
	voteData := make([]*flow.ClusterQCVoteData, len(qcs))
	for i, qc := range qcs {
		c, ok := clusterList.ByIndex(uint(i))
		if !ok {
			return nil, fmt.Errorf("could not get cluster list for cluster index %v", i)
		}
		voterIds, err := signature.DecodeSignerIndicesToIdentifiers(c.NodeIDs(), qc.SignerIndices)
		if err != nil {
			return nil, fmt.Errorf("could not decode signer indices: %w", err)
		}
		voteData[i] = &flow.ClusterQCVoteData{
			SigData:  qc.SigData,
			VoterIDs: voterIds,
		}
	}

	return voteData, nil
}

// Filters a list of nodes to include only nodes that will sign the QC for the
// given cluster. The resulting list of nodes is only nodes that are in the
// given cluster AND are not partner nodes (ie. we have the private keys).
func filterClusterSigners(cluster flow.IdentitySkeletonList, nodeInfos []model.NodeInfo) []model.NodeInfo {

	var filtered []model.NodeInfo
	for _, node := range nodeInfos {
		_, isInCluster := cluster.ByNodeID(node.NodeID)
		isNotPartner := node.Type() == model.NodeInfoTypePrivate

		if isInCluster && isNotPartner {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

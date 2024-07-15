package common

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	cdcCommon "github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/assignment"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ConstructClusterAssignment generates a partially randomized collector cluster assignment with internal and partner nodes.
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

	// The following is a heuristic for distributing the internal collector nodes (private staking key available
	// to generate QC for cluster root block) and partner nodes (private staking unknown). We need internal nodes
	// to control strictly more than 2/3 of the cluster's total weight.
	// The following is a heuristic that distributes collectors round-robbin across the specified number of clusters.
	// This heuristic only works when all collectors have equal weight! The following sanity check enforces this:
	if len(partnerNodes) > 0 && len(partnerNodes) > 2*len(internalNodes) {
		return nil, nil, fmt.Errorf("requiring at least x>0 number of partner nodes and y > 2x number of internal nodes, but got x,y=%d,%d", len(partnerNodes), len(internalNodes))
	}
	// sanity check ^ enforces that there is at least one internal node, hence `internalNodes[0].InitialWeight` is always a valid reference weight
	refWeight := internalNodes[0].InitialWeight

	identifierLists := make([]flow.IdentifierList, numCollectionClusters)
	// array to track the 2/3 internal-nodes constraint (internal_nodes > 2 * partner_nodes)
	constraint := make([]int, numCollectionClusters)

	// first, round-robin internal nodes into each cluster
	for i, node := range internals {
		if node.InitialWeight != refWeight {
			return nil, nil, fmt.Errorf("current implementation requires all collectors (partner & interal nodes) to have equal weight")
		}
		clusterIndex := i % numCollectionClusters
		identifierLists[clusterIndex] = append(identifierLists[clusterIndex], node.NodeID)
		constraint[clusterIndex] += 1
	}

	// next, round-robin partner nodes into each cluster
	for i, node := range partners {
		if node.InitialWeight != refWeight {
			return nil, nil, fmt.Errorf("current implementation requires all collectors (partner & interal nodes) to have equal weight")
		}
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

// ConvertClusterAssignmentsCdc converts golang cluster assignments type to Cadence type `[[String]]`.
func ConvertClusterAssignmentsCdc(assignments flow.AssignmentList) cadence.Array {
	stringArrayType := cadence.NewVariableSizedArrayType(cadence.StringType)

	assignmentsCdc := make([]cadence.Value, len(assignments))
	for i, asmt := range assignments {
		vals := make([]cadence.Value, asmt.Len())
		for j, nodeID := range asmt {
			vals[j] = cadence.String(nodeID.String())
		}
		assignmentsCdc[i] = cadence.NewArray(vals).
			WithType(stringArrayType)
	}

	return cadence.NewArray(assignmentsCdc).
		WithType(cadence.NewVariableSizedArrayType(stringArrayType))
}

// ConvertClusterQcsCdc converts cluster QCs from `QuorumCertificate` type to `ClusterQCVoteData` type.
// Args:
//   - qcs: list of quorum certificates.
//   - clusterList: the list of cluster lists each used to generate one of the quorum certificates in qcs.
//   - flowClusterQCAddress: the FlowClusterQC contract address where the ClusterQCVoteData type is defined.
//
// Returns:
//   - []cadence.Value: cadence representation of the list of cluster qcs.
//   - error: error if the cluster qcs and cluster lists don't match in size or
//     signature indices decoding fails.
func ConvertClusterQcsCdc(qcs []*flow.QuorumCertificate, clusterList flow.ClusterList, flowClusterQCAddress string) ([]cadence.Value, error) {
	voteDataType := newFlowClusterQCVoteDataStructType(flowClusterQCAddress)
	qcVoteData := make([]cadence.Value, len(qcs))
	for i, qc := range qcs {
		c, ok := clusterList.ByIndex(uint(i))
		if !ok {
			return nil, fmt.Errorf("could not get cluster list for cluster index %v", i)
		}
		voterIds, err := signature.DecodeSignerIndicesToIdentifiers(c.NodeIDs(), qc.SignerIndices)
		if err != nil {
			return nil, fmt.Errorf("could not decode signer indices: %w", err)
		}
		cdcVoterIds := make([]cadence.Value, len(voterIds))
		for i, id := range voterIds {
			cdcVoterIds[i] = cadence.String(id.String())
		}

		qcVoteData[i] = cadence.NewStruct([]cadence.Value{
			// aggregatedSignature
			cadence.String(fmt.Sprintf("%#x", qc.SigData)),
			// voterIDs
			cadence.NewArray(cdcVoterIds).WithType(cadence.NewVariableSizedArrayType(cadence.StringType)),
		}).WithType(voteDataType)

	}

	return qcVoteData, nil
}

// newFlowClusterQCVoteDataStructType returns the FlowClusterQC cadence struct type.
func newFlowClusterQCVoteDataStructType(clusterQcAddress string) *cadence.StructType {

	// FlowClusterQC.ClusterQCVoteData
	address, _ := cdcCommon.HexToAddress(clusterQcAddress)
	location := cdcCommon.NewAddressLocation(nil, address, "FlowClusterQC")

	return &cadence.StructType{
		Location:            location,
		QualifiedIdentifier: "FlowClusterQC.ClusterQCVoteData",
		Fields: []cadence.Field{
			{
				Identifier: "aggregatedSignature",
				Type:       cadence.StringType,
			},
			{
				Identifier: "voterIDs",
				Type:       cadence.NewVariableSizedArrayType(cadence.StringType),
			},
		},
	}
}

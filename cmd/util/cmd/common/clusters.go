package common

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/crypto/random"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	cdcCommon "github.com/onflow/cadence/common"

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
// The identity of internal and partner nodes in each cluster is deterministically randomized
// using the provided entropy `randomSource`. Ideally this entropy should be derived from the random beacon of the
// previous epoch, or some other verifiable random source.
// The function guarantees a specific constraint when partitioning the nodes into clusters:
// Strictly more than 1/3 of nodes in each cluster must be internal nodes. If the constraint can't be
// satisfied, an exception is returned. However, note that a successful cluster assignment does not imply
// that enough cluster votes can be obtained from internal nodes to create any cluster QCs.
// Note that if an exception is returned with a certain number of internal/partner nodes, there is no chance
// of succeeding the assignment by re-running the function without increasing the internal nodes ratio.
// Args:
// - log: the logger instance.
// - partnerNodes: identity list of partner nodes.
// - internalNodes: identity list of internal nodes.
// - numCollectionClusters: the number of clusters to generate
// - randomSource: entropy used to randomize the assignment.
// Returns:
// - flow.AssignmentList: the generated assignment list.
// - flow.ClusterList: the generate collection cluster list.
// - bool: whether all cluster QCs can be created with only votes from internal nodes.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ConstructClusterAssignment(log zerolog.Logger, partnerNodes, internalNodes flow.IdentityList, numCollectionClusters int, randomSource random.Rand) (flow.AssignmentList, flow.ClusterList, bool, error) {

	partnerCollectors := partnerNodes.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := internalNodes.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	nCollectors := len(partnerCollectors) + len(internalCollectors)

	// ensure we have at least as many collection nodes as clusters
	if nCollectors < int(numCollectionClusters) {
		log.Fatal().Msgf("network bootstrap is configured with %d collection nodes, but %d clusters - must have at least one collection node per cluster",
			nCollectors, numCollectionClusters)
	}

	// shuffle partner nodes in-place using the provided randomness
	err := randomSource.Shuffle(len(partnerCollectors), func(i, j int) {
		partnerCollectors[i], partnerCollectors[j] = partnerCollectors[j], partnerCollectors[i]
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not shuffle partners")
	}
	// shuffle internal nodes in-place using the provided randomness
	err = randomSource.Shuffle(len(internalCollectors), func(i, j int) {
		internalCollectors[i], internalCollectors[j] = internalCollectors[j], internalCollectors[i]
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not shuffle internals")
	}

	// capture first reference weight to validate that all collectors have equal weight
	refWeight := internalCollectors[0].InitialWeight

	identifierLists := make([]flow.IdentifierList, numCollectionClusters)
	// array to track the 2/3 internal-nodes constraint (internal_nodes > 2 * partner_nodes)
	constraint := make([]int, numCollectionClusters)

	// first, round-robin internal nodes into each cluster
	for i, node := range internalCollectors {
		if node.InitialWeight != refWeight {
			return nil, nil, false, fmt.Errorf("current implementation requires all collectors (partner & interal nodes) to have equal weight")
		}
		clusterIndex := i % numCollectionClusters
		identifierLists[clusterIndex] = append(identifierLists[clusterIndex], node.NodeID)
		constraint[clusterIndex] += 2
	}

	// next, round-robin partner nodes into each cluster
	for i, node := range partnerCollectors {
		if node.InitialWeight != refWeight {
			return nil, nil, false, fmt.Errorf("current implementation requires all collectors (partner & interal nodes) to have equal weight")
		}
		clusterIndex := i % numCollectionClusters
		identifierLists[clusterIndex] = append(identifierLists[clusterIndex], node.NodeID)
		constraint[clusterIndex] -= 1
	}

	// check the 2/3 constraint: for every cluster `i`, constraint[i] must be strictly positive
	canConstructAllClusterQCs := true
	for i := 0; i < numCollectionClusters; i++ {
		if constraint[i] <= 0 {
			canConstructAllClusterQCs = false
		}
	}

	assignments := assignment.FromIdentifierLists(identifierLists)

	collectors := append(partnerCollectors, internalCollectors...)
	clusters, err := factory.NewClusterList(assignments, collectors.ToSkeleton())
	if err != nil {
		log.Fatal().Err(err).Msg("could not create cluster list")
	}

	return assignments, clusters, canConstructAllClusterQCs, nil
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
	voteDataType := newClusterQCVoteDataCdcType(flowClusterQCAddress)
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
			cadence.String(hex.EncodeToString(qc.SigData)),
			// Node IDs of signers
			cadence.NewArray(cdcVoterIds).WithType(cadence.NewVariableSizedArrayType(cadence.StringType)),
		}).WithType(voteDataType)

	}

	return qcVoteData, nil
}

// newClusterQCVoteDataCdcType returns the FlowClusterQC cadence struct type.
func newClusterQCVoteDataCdcType(clusterQcAddress string) *cadence.StructType {

	// FlowClusterQC.ClusterQCVoteData
	address, _ := cdcCommon.HexToAddress(clusterQcAddress)
	location := cdcCommon.NewAddressLocation(nil, address, "FlowClusterQC")

	return cadence.NewStructType(
		location,
		"FlowClusterQC.ClusterQCVoteData",
		[]cadence.Field{
			{
				Identifier: "aggregatedSignature",
				Type:       cadence.StringType,
			},
			{
				Identifier: "voterIDs",
				Type:       cadence.NewVariableSizedArrayType(cadence.StringType),
			},
		},
		nil,
	)
}

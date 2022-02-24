package cmd

import (
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// Construct cluster assignment with internal and partner nodes uniformly
// distributed across clusters. This function will produce the same cluster
// assignments for the same partner and internal lists, and the same seed.
func constructClusterAssignment(partnerNodes, internalNodes []model.NodeInfo, seed int64) (flow.AssignmentList, flow.ClusterList) {

	partners := model.ToIdentityList(partnerNodes).Filter(filter.HasRole(flow.RoleCollection))
	internals := model.ToIdentityList(internalNodes).Filter(filter.HasRole(flow.RoleCollection))

	// deterministically shuffle both collector lists based on the input seed
	// by using a different seed each spork, we will have different clusters
	// even with the same collectors
	partners = partners.DeterministicShuffle(seed)
	internals = internals.DeterministicShuffle(seed)

	nClusters := flagCollectionClusters
	assignments := make(flow.AssignmentList, nClusters)

	// first, round-robin internal nodes into each cluster
	for i, node := range internals {
		assignments[i%len(assignments)] = append(assignments[i%len(assignments)], node.NodeID)
	}

	// next, round-robin partner nodes into each cluster
	for i, node := range partners {
		assignments[i%len(assignments)] = append(assignments[i%len(assignments)], node.NodeID)
	}

	collectors := append(partners, internals...)
	clusters, err := flow.NewClusterList(assignments, collectors)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create cluster list")
	}

	return assignments, clusters
}

func constructRootQCsForClusters(
	clusterList flow.ClusterList,
	nodeInfos []model.NodeInfo,
	clusterBlocks []*cluster.Block,
) []*flow.QuorumCertificate {

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

// Filters a list of nodes to include only nodes that will sign the QC for the
// given cluster. The resulting list of nodes is only nodes that are in the
// given cluster AND are not partner nodes (ie. we have the private keys).
func filterClusterSigners(cluster flow.IdentityList, nodeInfos []model.NodeInfo) []model.NodeInfo {

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

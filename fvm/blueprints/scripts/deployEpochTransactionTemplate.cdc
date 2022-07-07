import FlowClusterQC from 0x%s

transaction(clusterWeights: [{String: UInt64}]) {
  prepare(serviceAccount: AuthAccount)	{

    // first, construct Cluster objects from cluster weights
    let clusters: [FlowClusterQC.Cluster] = []
    var clusterIndex: UInt16 = 0
    for weightMapping in clusterWeights {
      let cluster = FlowClusterQC.Cluster(index: clusterIndex, nodeWeights: weightMapping)
      clusterIndex = clusterIndex + 1
    }

	serviceAccount.contracts.add(
		name: "FlowEpoch",
		code: "%s".decodeHex(),
		currentEpochCounter: UInt64(%d),
		numViewsInEpoch: UInt64(%d),
		numViewsInStakingAuction: UInt64(%d),
		numViewsInDKGPhase: UInt64(%d),
		numCollectorClusters: UInt16(%d),
		FLOWsupplyIncreasePercentage: UFix64(%d),
		randomSource: %s,
		collectorClusters: clusters,
        // NOTE: clusterQCs and dkgPubKeys are empty because these initial values are not used
		clusterQCs: [] as [FlowClusterQC.ClusterQC],
		dkgPubKeys: [] as [String],
	)
  }
}

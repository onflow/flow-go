import FlowClusterQC from "FlowClusterQC"

transaction(
    name: String,
    code: String,
    currentEpochCounter: UInt64,
    numViewsInEpoch: UInt64,
    numViewsInStakingAuction: UInt64,
    numViewsInDKGPhase: UInt64,
    numCollectorClusters: UInt16,
    FLOWsupplyIncreasePercentage: UFix64,
    randomSource: String,
    clusterWeights: [{String: UInt64}]) {
  prepare(serviceAccount: auth(AddContract) &Account)	{

    // first, construct Cluster objects from cluster weights
    let clusters: [FlowClusterQC.Cluster] = []
    var clusterIndex: UInt16 = 0
    for weightMapping in clusterWeights {
       let cluster = FlowClusterQC.Cluster(index: clusterIndex, nodeWeights: weightMapping)
      clusterIndex = clusterIndex + 1
    }

	serviceAccount.contracts.add(
		name: name,
		code: code.decodeHex(),
        currentEpochCounter: currentEpochCounter,
        numViewsInEpoch: numViewsInEpoch,
        numViewsInStakingAuction: numViewsInStakingAuction,
        numViewsInDKGPhase: numViewsInDKGPhase,
        numCollectorClusters: numCollectorClusters,
        FLOWsupplyIncreasePercentage: FLOWsupplyIncreasePercentage,
        randomSource: randomSource,
		collectorClusters: clusters,
        // NOTE: clusterQCs and dkgPubKeys are empty because these initial values are not used
		clusterQCs: [] as [FlowClusterQC.ClusterQC],
		dkgPubKeys: [] as [String],
	)
  }
}

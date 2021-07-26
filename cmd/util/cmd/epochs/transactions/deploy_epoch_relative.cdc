import FlowClusterQC from 0xQCADDRESS

transaction(name: String, 
            code: [UInt8],
            currentEpochCounter: UInt64,
            numViewsInEpoch: UInt64,
            numViewsInStakingAuction: UInt64,
            numViewsInDKGPhase: UInt64,
            numCollectorClusters: UInt16,
            FLOWsupplyIncreasePercentage: UFix64,
            randomSource: String,
            collectorClusters: [FlowClusterQC.Cluster]) {

  prepare(signer: AuthAccount) {

    let currentBlock = getCurrentBlock()

    signer.contracts.add(name: name, 
            code: code,
            currentEpochCounter: currentEpochCounter,
            numViewsInEpoch: numViewsInEpoch - currentBlock.view,
            numViewsInStakingAuction: numViewsInStakingAuction, 
            numViewsInDKGPhase: numViewsInDKGPhase, 
            numCollectorClusters: numCollectorClusters,
            FLOWsupplyIncreasePercentage: FLOWsupplyIncreasePercentage,
            randomSource: randomSource,
            collectorClusters: collectorClusters,
            clusterQCs: [],
            dkgPubKeys: [])
  }
}
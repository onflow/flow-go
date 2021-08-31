package cmd

// deployEpochTransactionTemplate is the transaction text for deploying the FLowEpoch
// contract. The contract code (including correct imports for the given chain
// is encoded into the transaction text (via fmt.Sprintf).
var deployEpochTransactionTemplate = `
// The Epoch smart contract is deployed after a spork.
// When we deploy the Epoch smart contract for the first time, we need to ensure 
// the epoch length and staking auction length are consistent with the protocol state.
// This transaction is needed to adjust the numViewsInEpoch and numViewsInStakingAuction
// value based on the current block when epochs is deployed

transaction(name: String, 
            currentEpochCounter: UInt64,
            // this value should be the number of views in the epoch, as computed from the 
            // first and final views of the epoch info from the protocol state
            numViewsInEpoch: UInt64,
            numViewsInStakingAuction: UInt64,
            numViewsInDKGPhase: UInt64,
            numCollectorClusters: UInt16,
            FLOWsupplyIncreasePercentage: UFix64,
            randomSource: String) {

  prepare(signer: AuthAccount) {

    let currentBlock = getCurrentBlock()

	let code: [UInt8] = %s

    // The smart contract computes the final view of the epoch like currentBlock.view+numViewsInEpoch. 
    // Since the Epoch contract is deployed after the spork while the network is running, 
    // we need to adjust the numViewsInEpoch value based on the current block when epochs is deployed
    // ASSUMPTION: first view after the spork is 0
    let adjustedNumViewsInEpoch = numViewsInEpoch - currentBlock.view

    // The epoch smart contract computes the stakingEndView like currentBlock.view+numViewsInStakingAuction
    // We adjust the numViewsInStakingAuction value based of the view of the current block when the contract is deployed.
    // ASSUMPTION: first view after the spork is 0
    let adjustedNumViewsInStakingAuction = numViewsInStakingAuction - currentBlock.view

    signer.contracts.add(name: name, 
            code: code,
            currentEpochCounter: currentEpochCounter,
            numViewsInEpoch: adjustedNumViewsInEpoch,
            numViewsInStakingAuction: adjustedNumViewsInStakingAuction, 
            numViewsInDKGPhase: numViewsInDKGPhase, 
            numCollectorClusters: numCollectorClusters,
            FLOWsupplyIncreasePercentage: FLOWsupplyIncreasePercentage,
            randomSource: randomSource,

            // the below arguments are unused and are safe to be left empty
            collectorClusters: [],
            clusterQCs: [],
            dkgPubKeys: [])
  }
}`

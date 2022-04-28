import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import FlowIDTableStaking from 0x8624b52f9ddcd04a
import FlowClusterQC from 0x8624b52f9ddcd04a
import FlowDKG from 0x8624b52f9ddcd04a
import FlowFees from 0xf919ee77447b7497

// The top-level smart contract managing the lifecycle of epochs. In Flow,
// epochs are the smallest unit of time where the identity table (the set of 
// network operators) is static. Operators may only leave or join the network 
// at epoch boundaries. Operators may be ejected during an epoch for various
// misdemeanours, but they remain in the identity table until the epoch ends.
//
// Epochs are split into 3 phases:
// |==========================================================
// | EPOCH N                                   || EPOCH N+1 ...
// |----- Staking -----|- Setup -|- Committed -|| ...
// |==========================================================
//
// 1)  STAKING PHASE
// Node operators are able to submit staking requests for the NEXT epoch during
// this phase. At the end of this phase, the Epoch smart contract resolves the 
// outstanding staking requests and determines the identity table for the next 
// epoch. The Epoch smart contract emits the EpochSetup service event containing 
// the identity table for the next epoch, which initiates the transition to the 
// Epoch Setup Phase.
//
// 2) SETUP PHASE
// When this phase begins the participants in the next epoch are set. During this
// phase, these participants prepare for the next epoch. In particular, collection
// nodes submit votes for their cluster's root quorum certificate and consensus
// nodes run the distributed key generation protocol (DKG) to set up the random
// beacon. When these preparations are complete, the Epoch smart contract emits the
// EpochCommit service event containing the artifacts of the set process, which
// initiates the transition to the Epoch Commit Phase.
//
// 3) COMMITTED PHASE
// When this phase begins, the network is fully prepared to transition to the next
// epoch. A failure to enter this phase before transitioning to the next epoch
// indicates that the participants in the next epoch failed to complete the set up
// procedure, which is a critical failure and will cause the chain to halt.

pub contract FlowEpoch {

    pub enum EpochPhase: UInt8 {
        pub case STAKINGAUCTION
        pub case EPOCHSETUP
        pub case EPOCHCOMMIT
    }

    pub enum NodeRole: UInt8 {
        pub case NONE
        pub case Collector
        pub case Consensus
        pub case Execution
        pub case Verification
        pub case Access
    }

    /// The Epoch Setup service event is emitted when we transition to the Epoch Setup
    /// phase. It contains the finalized identity table for the upcoming epoch.
    pub event EpochSetup(
        
        /// The counter for the upcoming epoch. Must be one greater than the
        /// counter for the current epoch.
        counter: UInt64,

        /// Identity table for the upcoming epoch with all node information.
        /// Includes:
        /// nodeID, staking key, networking key, networking address, role,
        /// staking information, weight, and more.
        nodeInfo: [FlowIDTableStaking.NodeInfo],

        /// The first view (inclusive) of the upcoming epoch.
        firstView: UInt64,

        /// The last view (inclusive) of the upcoming epoch.
        finalView: UInt64,

        /// The cluster assignment for the upcoming epoch. Each element in the list
        /// represents one cluster and contains all the node IDs assigned to that
        /// cluster, with their weights and votes
        collectorClusters: [FlowClusterQC.Cluster],

        /// The source of randomness to seed the leader selection algorithm with 
        /// for the upcoming epoch.
        randomSource: String,

        /// The deadlines of each phase in the DKG protocol to be completed in the upcoming
        /// EpochSetup phase. Deadlines are specified in terms of a consensus view number. 
        /// When a DKG participant observes a finalized and sealed block with view greater 
        /// than the given deadline, it can safely transition to the next phase. 
        DKGPhase1FinalView: UInt64,
        DKGPhase2FinalView: UInt64,
        DKGPhase3FinalView: UInt64
    )

    /// The EpochCommit service event is emitted when we transition from the Epoch
    /// Committed phase. It is emitted only when all preparation for the upcoming epoch
    /// has been completed
    pub event EpochCommit(

        /// The counter for the upcoming epoch. Must be equal to the counter in the
        /// previous EpochSetup event.
        counter: UInt64,

        /// The result of the QC aggregation process. Each element contains 
        /// all the nodes and votes received for a particular cluster
        /// QC stands for quorum certificate that each cluster generates.
        clusterQCs: [FlowClusterQC.ClusterQC],

        /// The resulting public keys from the DKG process, encoded as by the flow-go
        /// crypto library, then hex-encoded.
        /// Group public key is the first element, followed by the individual keys
        dkgPubKeys: [String],
    )

    /// Contains specific metadata about a particular epoch
    /// All historical epoch metadata is stored permanently
    pub struct EpochMetadata {

        /// The identifier for the epoch
        pub let counter: UInt64

        /// The seed used for generating the epoch setup
        pub let seed: String

        /// The first view of this epoch
        pub let startView: UInt64

        /// The last view of this epoch
        pub let endView: UInt64

        /// The last view of the staking auction
        pub let stakingEndView: UInt64

        /// The total rewards that are paid out for the epoch
        pub var totalRewards: UFix64

        /// The reward amounts that are paid to each individual node and its delegators
        pub var rewardAmounts: [FlowIDTableStaking.RewardsBreakdown]

        /// Tracks if rewards have been paid for this epoch
        pub var rewardsPaid: Bool

        /// The organization of collector node IDs into clusters
        /// determined by a round robin sorting algorithm
        pub let collectorClusters: [FlowClusterQC.Cluster]

        /// The Quorum Certificates from the ClusterQC contract
        pub var clusterQCs: [FlowClusterQC.ClusterQC]

        /// The public keys associated with the Distributed Key Generation
        /// process that consensus nodes participate in
        /// Group key is the last element at index: length - 1
        pub var dkgKeys: [String]

        init(counter: UInt64,
             seed: String,
             startView: UInt64,
             endView: UInt64,
             stakingEndView: UInt64,
             totalRewards: UFix64,
             collectorClusters: [FlowClusterQC.Cluster],
             clusterQCs: [FlowClusterQC.ClusterQC],
             dkgKeys: [String]) {

            self.counter = counter
            self.seed = seed
            self.startView = startView
            self.endView = endView
            self.stakingEndView = stakingEndView
            self.totalRewards = totalRewards
            self.rewardAmounts = []
            self.rewardsPaid = false
            self.collectorClusters = collectorClusters
            self.clusterQCs = clusterQCs
            self.dkgKeys = dkgKeys
        }

        access(account) fun setTotalRewards(_ newRewards: UFix64) {
            self.totalRewards = newRewards
        }

        access(account) fun setRewardAmounts(_ rewardBreakdown: [FlowIDTableStaking.RewardsBreakdown]) {
            self.rewardAmounts = rewardBreakdown
        }

        access(account) fun setRewardsPaid(_ rewardsPaid: Bool) {
            self.rewardsPaid = rewardsPaid
        } 

        access(account) fun setClusterQCs(qcs: [FlowClusterQC.ClusterQC]) {
            self.clusterQCs = qcs
        }

        access(account) fun setDKGGroupKey(keys: [String]) {
            self.dkgKeys = keys
        }
    }

    /// Metadata that is managed and can be changed by the Admin///
    pub struct Config {
        /// The number of views in an entire epoch
        pub(set) var numViewsInEpoch: UInt64

        /// The number of views in the staking auction
        pub(set) var numViewsInStakingAuction: UInt64
        
        /// The number of views in each dkg phase
        pub(set) var numViewsInDKGPhase: UInt64

        /// The number of collector clusters in each epoch
        pub(set) var numCollectorClusters: UInt16

        /// Tracks the rate at which the rewards payout increases every epoch
        /// This value is multiplied by the FLOW total supply to get the next payout
        pub(set) var FLOWsupplyIncreasePercentage: UFix64

        init(numViewsInEpoch: UInt64, numViewsInStakingAuction: UInt64, numViewsInDKGPhase: UInt64, numCollectorClusters: UInt16, FLOWsupplyIncreasePercentage: UFix64) {
            self.numViewsInEpoch = numViewsInEpoch
            self.numViewsInStakingAuction = numViewsInStakingAuction
            self.numViewsInDKGPhase = numViewsInDKGPhase
            self.numCollectorClusters = numCollectorClusters
            self.FLOWsupplyIncreasePercentage = FLOWsupplyIncreasePercentage
        }
    }

    /// Holds the `FlowEpoch.Config` struct with the configurable metadata
    access(contract) let configurableMetadata: Config

    /// Metadata that is managed by the smart contract
    /// and cannot be changed by the Admin

    /// Contains a historical record of the metadata from all previous epochs
    /// indexed by epoch number

    /// Returns the metadata from the specified epoch
    /// or nil if it isn't found
    /// Epoch Metadata is stored in account storage so the growing dictionary
    /// does not have to be loaded every time the contract is loaded
    pub fun getEpochMetadata(_ epochCounter: UInt64): EpochMetadata? {
        if let metadataDictionary = self.account.borrow<&{UInt64: EpochMetadata}>(from: self.metadataStoragePath) {
            return metadataDictionary[epochCounter]
        }
        return nil
    }

    /// Saves a modified EpochMetadata struct to the metadata in account storage
    /// 
    access(contract) fun saveEpochMetadata(_ newMetadata: EpochMetadata) {
        pre {
            self.currentEpochCounter == (0 as UInt64) ||
            (newMetadata.counter >= self.currentEpochCounter - (1 as UInt64) &&
            newMetadata.counter <= self.proposedEpochCounter()):
                "Cannot modify epoch metadata from epochs after the proposed epoch or before the previous epoch"
        }
        if let metadataDictionary = self.account.borrow<&{UInt64: EpochMetadata}>(from: self.metadataStoragePath) {
            if let metadata = metadataDictionary[newMetadata.counter] {
                assert (
                    metadata.counter == newMetadata.counter,
                    message: "Cannot save metadata with mismatching epoch counters"
                )
            }
            metadataDictionary[newMetadata.counter] = newMetadata
        }
    }

    /// The counter, or ID, of the current epoch
    pub var currentEpochCounter: UInt64

    /// The current phase that the epoch is in
    pub var currentEpochPhase: EpochPhase

    /// Path where the `FlowEpoch.Admin` resource is stored
    pub let adminStoragePath: StoragePath

    /// Path where the `FlowEpoch.Heartbeat` resource is stored
    pub let heartbeatStoragePath: StoragePath

    /// Path where the `{UInt64: EpochMetadata}` dictionary is stored
    pub let metadataStoragePath: StoragePath

    /// Resource that can update some of the contract fields
    pub resource Admin {
        pub fun updateEpochViews(_ newEpochViews: UInt64) {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only update fields during the staking auction"
                FlowEpoch.isValidPhaseConfiguration(FlowEpoch.configurableMetadata.numViewsInStakingAuction,
                    FlowEpoch.configurableMetadata.numViewsInDKGPhase,
                    newEpochViews): "New Epoch Views must be greater than the sum of staking and DKG Phase views"
            }

            FlowEpoch.configurableMetadata.numViewsInEpoch = newEpochViews
        }

        pub fun updateAuctionViews(_ newAuctionViews: UInt64) {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only update fields during the staking auction"
                FlowEpoch.isValidPhaseConfiguration(newAuctionViews,
                    FlowEpoch.configurableMetadata.numViewsInDKGPhase,
                    FlowEpoch.configurableMetadata.numViewsInEpoch): "Epoch Views must be greater than the sum of new staking and DKG Phase views"
            }

            FlowEpoch.configurableMetadata.numViewsInStakingAuction = newAuctionViews
        }

        pub fun updateDKGPhaseViews(_ newPhaseViews: UInt64) {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only update fields during the staking auction"
                FlowEpoch.isValidPhaseConfiguration(FlowEpoch.configurableMetadata.numViewsInStakingAuction,
                    newPhaseViews,
                    FlowEpoch.configurableMetadata.numViewsInEpoch): "Epoch Views must be greater than the sum of staking and new DKG Phase views"
            }

            FlowEpoch.configurableMetadata.numViewsInDKGPhase = newPhaseViews
        }

        pub fun updateNumCollectorClusters(_ newNumClusters: UInt16) {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only update fields during the staking auction"
            }

            FlowEpoch.configurableMetadata.numCollectorClusters = newNumClusters
        }

        pub fun updateFLOWSupplyIncreasePercentage(_ newPercentage: UFix64) {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only update fields during the staking auction"
                newPercentage <= 1.0 as UFix64: "New value must be between zero and one"
            }

            FlowEpoch.configurableMetadata.FLOWsupplyIncreasePercentage = newPercentage
        }

        // Enable or disable automatic rewards calculations and payments
        pub fun updateAutomaticRewardsEnabled(_ enabled: Bool) {
            FlowEpoch.account.load<Bool>(from: /storage/flowAutomaticRewardsEnabled)
            FlowEpoch.account.save(enabled, to: /storage/flowAutomaticRewardsEnabled)
        }
    }

    /// Resource that is controlled by the protocol and is used
    /// to change the current phase of the epoch or reset the epoch if needed
    /// 
    pub resource Heartbeat {

        /// Function that is called every block to advance the epoch
        /// and change phase if the required conditions have been met
        pub fun advanceBlock() {

            switch FlowEpoch.currentEpochPhase {
                case EpochPhase.STAKINGAUCTION:
                    let currentBlock = getCurrentBlock()
                    let currentEpochMetadata = FlowEpoch.getEpochMetadata(FlowEpoch.currentEpochCounter)!
                    if currentBlock.view >= currentEpochMetadata.stakingEndView {
                        self.endStakingAuction()
                    }
                case EpochPhase.EPOCHSETUP:
                    if FlowClusterQC.votingCompleted() && (FlowDKG.dkgCompleted() != nil) {
                        self.startEpochCommit()
                    }
                case EpochPhase.EPOCHCOMMIT:
                    let currentBlock = getCurrentBlock()
                    let currentEpochMetadata = FlowEpoch.getEpochMetadata(FlowEpoch.currentEpochCounter)!
                    if currentBlock.view >= currentEpochMetadata.endView {
                        self.calculateAndSetRewards()
                        self.endEpoch()
                        if FlowEpoch.automaticRewardsEnabled() {
                            self.payRewards()
                        }
                    }
                default:
                    return
            }
        }

        /// Calls `FlowEpoch` functions to end the staking auction phase
        /// and start the Epoch Setup phase
        pub fun endStakingAuction() {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION: "Can only end staking auction during the staking auction"
            }

            FlowEpoch.endStakingAuction()

            FlowEpoch.startEpochSetup(randomSource: unsafeRandom().toString())
        }

        /// Calls `FlowEpoch` functions to end the Epoch Setup phase
        /// and start the Epoch Setup Phase
        pub fun startEpochCommit() {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.EPOCHSETUP: "Can only end Epoch Setup during Epoch Setup"
            }

            FlowEpoch.startEpochCommit()
        }

        /// Calls `FlowEpoch` functions to end the Epoch Commit phase
        /// and start the Staking Auction phase of a new epoch
        pub fun endEpoch() {
            pre {
                FlowEpoch.currentEpochPhase == EpochPhase.EPOCHCOMMIT: "Can only end epoch during Epoch Commit"
            }

            FlowEpoch.startNewEpoch()
        }

        /// Needs to be called before the epoch is over
        /// Calculates rewards for the current epoch and stores them in epoch metadata
        pub fun calculateAndSetRewards() {
            FlowEpoch.calculateAndSetRewards()
        }

        pub fun payRewards() {
            FlowEpoch.payRewards()
        }

        /// Protocol can use this to reboot the epoch with a new genesis
        /// in case the epoch setup phase did not complete properly
        /// before the end of an epoch
        pub fun resetEpoch(
            currentEpochCounter: UInt64,
            randomSource: String,
            newPayout: UFix64?,
            startView: UInt64,
            stakingEndView: UInt64,
            endView: UInt64,
            collectorClusters: [FlowClusterQC.Cluster],
            clusterQCs: [FlowClusterQC.ClusterQC],
            dkgPubKeys: [String])
        {
            pre {
                currentEpochCounter == FlowEpoch.currentEpochCounter:
                    "Cannot submit a current Epoch counter that does not match the current counter stored in the smart contract"
                FlowEpoch.isValidPhaseConfiguration(stakingEndView-startView+1, FlowEpoch.configurableMetadata.numViewsInDKGPhase, endView-startView+1):
                    "Invalid startView, stakingEndView, and endView configuration"
            }

            if FlowEpoch.currentEpochPhase == EpochPhase.STAKINGAUCTION {
                // Since we are resetting the epoch, we do not need to
                // start epoch setup also. We only need to end the staking auction
                FlowEpoch.borrowStakingAdmin().endStakingAuction()
            } else {
                // force reset the QC and DKG
                FlowEpoch.borrowClusterQCAdmin().forceStopVoting()
                FlowEpoch.borrowDKGAdmin().forceEndDKG()
            }

            // Start a new Epoch, which increments the current epoch counter
            FlowEpoch.startNewEpoch()

            let currentBlock = getCurrentBlock()

            let newEpochMetadata = EpochMetadata(
                    counter: FlowEpoch.currentEpochCounter,
                    seed: randomSource,
                    startView: startView,
                    endView: endView,
                    stakingEndView: stakingEndView,
                    totalRewards: FlowIDTableStaking.getEpochTokenPayout(),
                    collectorClusters: collectorClusters,
                    clusterQCs: clusterQCs,
                    dkgKeys: dkgPubKeys)

            FlowEpoch.saveEpochMetadata(newEpochMetadata)
        }
    }

    /// Calculates a new token payout for the current epoch
    /// and sets the new payout for the next epoch
    access(account) fun calculateAndSetRewards() {

        let stakingAdmin = self.borrowStakingAdmin()

        // Calculate rewards for the current epoch that is about to end
        // and save that reward breakdown in the epoch metadata for the current epoch
        let rewardsBreakdown = stakingAdmin.calculateRewards()
        let currentMetadata = self.getEpochMetadata(self.currentEpochCounter)!
        currentMetadata.setRewardAmounts(rewardsBreakdown)
        self.saveEpochMetadata(currentMetadata)

        if FlowEpoch.automaticRewardsEnabled() {
            // Calculate the total supply of FLOW after the current epoch's payout
            // the calculation includes the tokens that haven't been minted for the current epoch yet
            let currentPayout = FlowIDTableStaking.getEpochTokenPayout()
            let feeAmount = FlowFees.getFeeBalance()
            var flowTotalSupplyAfterPayout = 0.0
            if feeAmount >= currentPayout {
                flowTotalSupplyAfterPayout = FlowToken.totalSupply
            } else {
                flowTotalSupplyAfterPayout = FlowToken.totalSupply + (currentPayout - feeAmount)
            }

            // Calculate the payout for the next epoch
            let proposedPayout = flowTotalSupplyAfterPayout * FlowEpoch.configurableMetadata.FLOWsupplyIncreasePercentage

            // Set the new payout in the staking contract and proposed Epoch Metadata
            self.borrowStakingAdmin().setEpochTokenPayout(proposedPayout)
            let proposedMetadata = self.getEpochMetadata(self.proposedEpochCounter())
                ?? panic("Cannot set rewards for the next epoch becuase it hasn't been proposed yet")
            proposedMetadata.setTotalRewards(proposedPayout)
            self.saveEpochMetadata(proposedMetadata)
        }
    }

    /// Pays rewards to the nodes and delegators of the previous epoch
    access(account) fun payRewards() {
        if let previousEpochMetadata = self.getEpochMetadata(self.currentEpochCounter - (1 as UInt64)) {
            if !previousEpochMetadata.rewardsPaid {
                let rewardsBreakdownArray = previousEpochMetadata.rewardAmounts
                self.borrowStakingAdmin().payRewards(rewardsBreakdownArray)
                previousEpochMetadata.setRewardsPaid(true)
                self.saveEpochMetadata(previousEpochMetadata)
            }
        }
    }

    /// Moves staking tokens between buckets,
    /// and starts the new epoch staking auction
    access(account) fun startNewEpoch() {

        // End QC and DKG if they are still enabled
        if FlowClusterQC.inProgress {
            self.borrowClusterQCAdmin().stopVoting()
        }
        if FlowDKG.dkgEnabled {
            self.borrowDKGAdmin().endDKG()
        }

        self.borrowStakingAdmin().moveTokens()

        self.currentEpochPhase = EpochPhase.STAKINGAUCTION

        // Update the epoch counters
        self.currentEpochCounter = self.proposedEpochCounter()
    }

    /// Ends the staking Auction with all the proposed nodes approved
    access(account) fun endStakingAuction() {
        self.borrowStakingAdmin().endStakingAuction()
    }

    /// Starts the EpochSetup phase and emits the epoch setup event
    /// This has to be called directly after `endStakingAuction`
    access(account) fun startEpochSetup(randomSource: String) {

        // Get all the nodes that are proposed for the next epoch
        let ids = FlowIDTableStaking.getProposedNodeIDs()

        // Holds the node Information of all the approved nodes
        var nodeInfoArray: [FlowIDTableStaking.NodeInfo] = []

        // Holds node IDs of only collector nodes for QC
        var collectorNodeIDs: [String] = []

        // Holds node IDs of only consensus nodes for DKG
        var consensusNodeIDs: [String] = []

        // Get NodeInfo for all the nodes
        // Get all the collector and consensus nodes
        // to initialize the QC and DKG
        for id in ids {
            let nodeInfo = FlowIDTableStaking.NodeInfo(nodeID: id)

            nodeInfoArray.append(nodeInfo)

            if nodeInfo.role == NodeRole.Collector.rawValue {
                collectorNodeIDs.append(nodeInfo.id)
            }

            if nodeInfo.role == NodeRole.Consensus.rawValue {
                consensusNodeIDs.append(nodeInfo.id)
            }
        }
        
        // Organize the collector nodes into clusters
        let collectorClusters = self.createCollectorClusters(nodeIDs: collectorNodeIDs)

        // Start QC Voting with the supplied clusters
        self.borrowClusterQCAdmin().startVoting(clusters: collectorClusters)

        // Start DKG with the consensus nodes
        self.borrowDKGAdmin().startDKG(nodeIDs: consensusNodeIDs)

        let currentEpochMetadata = self.getEpochMetadata(self.currentEpochCounter)!

        // Initialze the metadata for the next epoch
        // QC and DKG metadata will be filled in later
        let proposedEpochMetadata = EpochMetadata(counter: self.proposedEpochCounter(),
                                                seed: randomSource,
                                                startView: currentEpochMetadata.endView + UInt64(1),
                                                endView: currentEpochMetadata.endView + self.configurableMetadata.numViewsInEpoch,
                                                stakingEndView: currentEpochMetadata.endView + self.configurableMetadata.numViewsInStakingAuction,
                                                totalRewards: 0.0 as UFix64,
                                                collectorClusters: collectorClusters,
                                                clusterQCs: [],
                                                dkgKeys: [])

        self.saveEpochMetadata(proposedEpochMetadata)

        self.currentEpochPhase = EpochPhase.EPOCHSETUP

        emit EpochSetup(counter: proposedEpochMetadata.counter,
                        nodeInfo: nodeInfoArray, 
                        firstView: proposedEpochMetadata.startView,
                        finalView: proposedEpochMetadata.endView,
                        collectorClusters: collectorClusters,
                        randomSource: randomSource,
                        DKGPhase1FinalView: proposedEpochMetadata.startView + self.configurableMetadata.numViewsInStakingAuction + self.configurableMetadata.numViewsInDKGPhase - 1 as UInt64,
                        DKGPhase2FinalView: proposedEpochMetadata.startView + self.configurableMetadata.numViewsInStakingAuction + (2 as UInt64 * self.configurableMetadata.numViewsInDKGPhase) - 1 as UInt64,
                        DKGPhase3FinalView: proposedEpochMetadata.startView + self.configurableMetadata.numViewsInStakingAuction + (3 as UInt64 * self.configurableMetadata.numViewsInDKGPhase) - 1 as UInt64)
    }

    /// Ends the EpochSetup phase when the QC and DKG are completed
    /// and emits the EpochCommit event with the results
    access(account) fun startEpochCommit() {
        if !FlowClusterQC.votingCompleted() || FlowDKG.dkgCompleted() == nil {
            return
        }

        let clusters = FlowClusterQC.getClusters()

        // Holds the quorum certificates for each cluster
        var clusterQCs: [FlowClusterQC.ClusterQC] = []

        // iterate through all the clusters and create their certificate arrays
        for cluster in clusters {
            var certificate = cluster.generateQuorumCertificate()
                ?? panic("Could not generate the quorum certificate for this cluster")

            clusterQCs.append(certificate)
        }

        // Set cluster QCs in the proposed epoch metadata
        // and stop QC voting
        let proposedEpochMetadata = self.getEpochMetadata(self.proposedEpochCounter())!
        proposedEpochMetadata.setClusterQCs(qcs: clusterQCs)

        // Set DKG result keys in the proposed epoch metadata
        // and stop DKG
        let dkgKeys = FlowDKG.dkgCompleted()!
        let unwrappedKeys: [String] = []
        for key in dkgKeys {
            unwrappedKeys.append(key!)
        }
        proposedEpochMetadata.setDKGGroupKey(keys: unwrappedKeys)

        self.saveEpochMetadata(proposedEpochMetadata)

        self.currentEpochPhase = EpochPhase.EPOCHCOMMIT

        emit EpochCommit(counter: self.proposedEpochCounter(),
                            clusterQCs: clusterQCs,
                            dkgPubKeys: unwrappedKeys)
    }

    /// Borrow a reference to the FlowIDTableStaking Admin resource
    access(contract) fun borrowStakingAdmin(): &FlowIDTableStaking.Admin {
        let adminRef = self.account.borrow<&FlowIDTableStaking.Admin>(from: FlowIDTableStaking.StakingAdminStoragePath)
            ?? panic("Could not borrow staking admin")

        return adminRef
    }

    /// Borrow a reference to the ClusterQCs Admin resource
    access(contract) fun borrowClusterQCAdmin(): &FlowClusterQC.Admin {
        let adminRef = self.account.borrow<&FlowClusterQC.Admin>(from: FlowClusterQC.AdminStoragePath)
            ?? panic("Could not borrow qc admin")

        return adminRef
    }

    /// Borrow a reference to the DKG Admin resource
    access(contract) fun borrowDKGAdmin(): &FlowDKG.Admin {
        let adminRef = self.account.borrow<&FlowDKG.Admin>(from: FlowDKG.AdminStoragePath)
            ?? panic("Could not borrow dkg admin")

        return adminRef
    }

    /// Makes sure the set of phase lengths (in views) are valid.
    /// Sub-phases cannot be greater than the full epoch length.
    pub fun isValidPhaseConfiguration(_ auctionLen: UInt64, _ dkgPhaseLen: UInt64, _ epochLen: UInt64): Bool {
        return (auctionLen + ((3 as UInt64)*dkgPhaseLen)) < epochLen
    }

    /// Randomizes the list of collector node ID and uses a round robin algorithm
    /// to assign all collector nodes to equal sized clusters
    pub fun createCollectorClusters(nodeIDs: [String]): [FlowClusterQC.Cluster] {
        pre {
            UInt16(nodeIDs.length) >= self.configurableMetadata.numCollectorClusters: "Cannot have less collector nodes than clusters"
        }
        var shuffledIDs = self.randomize(nodeIDs)

        // Holds cluster assignments for collector nodes
        let clusters: [FlowClusterQC.Cluster] = []
        var clusterIndex: UInt16 = 0
        let nodeWeightsDictionary: [{String: UInt64}] = []
        while clusterIndex < self.configurableMetadata.numCollectorClusters {
            nodeWeightsDictionary.append({})
            clusterIndex = clusterIndex + 1 as UInt16
        }
        clusterIndex = 0

        for id in shuffledIDs {

            let nodeInfo = FlowIDTableStaking.NodeInfo(nodeID: id)

            nodeWeightsDictionary[clusterIndex][id] = nodeInfo.initialWeight
            
            // Advance to the next cluster, or back to the first if we have gotten to the last one
            clusterIndex = clusterIndex + 1 as UInt16
            if clusterIndex == self.configurableMetadata.numCollectorClusters {
                clusterIndex = 0
            }
        }

        // Create the clusters Array that is sent to the QC contract
        // and emitted in the EpochSetup event
        clusterIndex = 0
        while clusterIndex < self.configurableMetadata.numCollectorClusters {
            clusters.append(FlowClusterQC.Cluster(index: clusterIndex, nodeWeights: nodeWeightsDictionary[clusterIndex]!))
            clusterIndex = clusterIndex + 1 as UInt16
        }

        return clusters
    }
  
    /// A function to generate a random permutation of arr[] 
    /// using the fisher yates shuffling algorithm
    pub fun randomize(_ array: [String]): [String] {  

        var i = array.length - 1

        // Start from the last element and swap one by one. We don't 
        // need to run for the first element that's why i > 0 
        while i > 0
        { 
            // Pick a random index from 0 to i 
            var randomNum = unsafeRandom()
            var randomIndex = randomNum % UInt64(i + 1)
    
            // Swap arr[i] with the element at random index 
            var temp = array[i]
            array[i] = array[randomIndex]
            array[randomIndex] = temp

            i = i - 1
        }

        return array
    }

    /// Collector nodes call this function to get their QC Voter resource
    /// in order to participate the the QC generation for their cluster
    pub fun getClusterQCVoter(nodeStaker: &FlowIDTableStaking.NodeStaker): @FlowClusterQC.Voter {
        let nodeInfo = FlowIDTableStaking.NodeInfo(nodeID: nodeStaker.id)

        assert (
            nodeInfo.role == NodeRole.Collector.rawValue,
            message: "Node operator must be a collector node to get a QC Voter object"
        )

        let clusterQCAdmin = self.borrowClusterQCAdmin()
        return <-clusterQCAdmin.createVoter(nodeID: nodeStaker.id, stakingKey: nodeInfo.stakingKey)
    }

    /// Consensus nodes call this function to get their DKG Participant resource
    /// in order to participate in the DKG for the next epoch
    pub fun getDKGParticipant(nodeStaker: &FlowIDTableStaking.NodeStaker): @FlowDKG.Participant {
        let nodeInfo = FlowIDTableStaking.NodeInfo(nodeID: nodeStaker.id)

        assert (
            nodeInfo.role == NodeRole.Consensus.rawValue,
            message: "Node operator must be a consensus node to get a DKG Participant object"
        )

        let dkgAdmin = self.borrowDKGAdmin()
        return <-dkgAdmin.createParticipant(nodeID: nodeStaker.id)
    }

    /// Returns the metadata that is able to be configured by the admin
    pub fun getConfigMetadata(): Config {
        return self.configurableMetadata
    }

    /// The proposed Epoch counter is always the current counter plus 1
    pub fun proposedEpochCounter(): UInt64 {
        return self.currentEpochCounter + 1 as UInt64
    }

    pub fun automaticRewardsEnabled(): Bool {
        return self.account.copy<Bool>(from: /storage/flowAutomaticRewardsEnabled) ?? false
    }

    init (currentEpochCounter: UInt64,
          numViewsInEpoch: UInt64, 
          numViewsInStakingAuction: UInt64, 
          numViewsInDKGPhase: UInt64, 
          numCollectorClusters: UInt16,
          FLOWsupplyIncreasePercentage: UFix64,
          randomSource: String,
          collectorClusters: [FlowClusterQC.Cluster],
          clusterQCs: [FlowClusterQC.ClusterQC],
          dkgPubKeys: [String]) {
        pre {
            FlowEpoch.isValidPhaseConfiguration(numViewsInStakingAuction, numViewsInDKGPhase, numViewsInEpoch):
                "Invalid startView and endView configuration"
        }

        self.configurableMetadata = Config(numViewsInEpoch: numViewsInEpoch,
                                           numViewsInStakingAuction: numViewsInStakingAuction,
                                           numViewsInDKGPhase: numViewsInDKGPhase,
                                           numCollectorClusters: numCollectorClusters,
                                           FLOWsupplyIncreasePercentage: FLOWsupplyIncreasePercentage)
        
        self.currentEpochCounter = currentEpochCounter
        self.currentEpochPhase = EpochPhase.STAKINGAUCTION
        self.adminStoragePath = /storage/flowEpochAdmin
        self.heartbeatStoragePath = /storage/flowEpochHeartbeat
        self.metadataStoragePath = /storage/flowEpochMetadata

        let epochMetadata: {UInt64: EpochMetadata} = {}
        self.account.save(epochMetadata, to: self.metadataStoragePath)

        self.account.save(<-create Admin(), to: self.adminStoragePath)
        self.account.save(<-create Heartbeat(), to: self.heartbeatStoragePath)

        self.borrowStakingAdmin().startStakingAuction()

        let currentBlock = getCurrentBlock()

        let firstEpochMetadata = EpochMetadata(counter: self.currentEpochCounter,
                    seed: randomSource,
                    startView: currentBlock.view,
                    endView: currentBlock.view + self.configurableMetadata.numViewsInEpoch - (1 as UInt64),
                    stakingEndView: currentBlock.view + self.configurableMetadata.numViewsInStakingAuction - (1 as UInt64),
                    totalRewards: FlowIDTableStaking.getEpochTokenPayout(),
                    collectorClusters: collectorClusters,
                    clusterQCs: clusterQCs,
                    dkgKeys: dkgPubKeys)
        self.saveEpochMetadata(firstEpochMetadata)
    }
}

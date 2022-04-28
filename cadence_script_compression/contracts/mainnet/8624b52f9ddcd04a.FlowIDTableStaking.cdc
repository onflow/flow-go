/*

    FlowIDTableStaking

    The Flow ID Table and Staking contract manages
    node operators' and delegators' information
    and Flow tokens that are staked as part of the Flow Protocol.

    Nodes submit their stake to the public addNodeInfo function
    during the staking auction phase.

    This records their info and committed tokens. They also will get a Node
    Object that they can use to stake, unstake, and withdraw rewards.

    Each node has multiple token buckets that hold their tokens
    based on their status: committed, staked, unstaking, unstaked, and rewarded.

    Delegators can also register to delegate FLOW to a node operator
    during the staking auction phase by using the registerNewDelegator() function.
    They have the same token buckets that node operators do.

    The Admin has the authority to remove node records,
    refund insufficiently staked nodes, pay rewards,
    and move tokens between buckets. These will happen once every epoch.

    See additional staking documentation here: https://docs.onflow.org/staking/

 */

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import FlowFees from 0xf919ee77447b7497
import Crypto

pub contract FlowIDTableStaking {

    /****** ID Table and Staking Events ******/

    pub event NewEpoch(totalStaked: UFix64, totalRewardPayout: UFix64)
    pub event EpochTotalRewardsPaid(total: UFix64, fromFees: UFix64, minted: UFix64, feesBurned: UFix64)

    /// Node Events
    pub event NewNodeCreated(nodeID: String, role: UInt8, amountCommitted: UFix64)
    pub event TokensCommitted(nodeID: String, amount: UFix64)
    pub event TokensStaked(nodeID: String, amount: UFix64)
    pub event TokensUnstaking(nodeID: String, amount: UFix64)
    pub event TokensUnstaked(nodeID: String, amount: UFix64)
    pub event NodeRemovedAndRefunded(nodeID: String, amount: UFix64)
    pub event RewardsPaid(nodeID: String, amount: UFix64)
    pub event UnstakedTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event RewardTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event NetworkingAddressUpdated(nodeID: String, newAddress: String)

    /// Delegator Events
    pub event NewDelegatorCreated(nodeID: String, delegatorID: UInt32)
    pub event DelegatorTokensCommitted(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorTokensStaked(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorTokensUnstaking(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorTokensUnstaked(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorRewardsPaid(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorUnstakedTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorRewardTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)

    /// Contract Field Change Events
    pub event NewDelegatorCutPercentage(newCutPercentage: UFix64)
    pub event NewWeeklyPayout(newPayout: UFix64)
    pub event NewStakingMinimums(newMinimums: {UInt8: UFix64})

    /// Holds the identity table for all the nodes in the network.
    /// Includes nodes that aren't actively participating
    /// key = node ID
    /// value = the record of that node's info, tokens, and delegators
    access(contract) var nodes: @{String: NodeRecord}

    /// The minimum amount of tokens that each node type has to stake
    /// in order to be considered valid
    access(account) var minimumStakeRequired: {UInt8: UFix64}

    /// The total amount of tokens that are staked for all the nodes
    /// of each node type during the current epoch
    access(account) var totalTokensStakedByNodeType: {UInt8: UFix64}

    /// The total amount of tokens that are paid as rewards every epoch
    /// could be manually changed by the admin resource
    access(account) var epochTokenPayout: UFix64

    /// The ratio of the weekly awards that each node type gets
    /// key = node role
    /// value = decimal number between 0 and 1 indicating a percentage
    /// NOTE: Currently is not used
    access(contract) var rewardRatios: {UInt8: UFix64}

    /// The percentage of rewards that every node operator takes from
    /// the users that are delegating to it
    access(account) var nodeDelegatingRewardCut: UFix64

    /// Paths for storing staking resources
    pub let NodeStakerStoragePath: StoragePath
    pub let NodeStakerPublicPath: PublicPath
    pub let StakingAdminStoragePath: StoragePath
    pub let DelegatorStoragePath: StoragePath

    /*********** ID Table and Staking Composite Type Definitions *************/

    /// Contains information that is specific to a node in Flow
    pub resource NodeRecord {

        /// The unique ID of the node
        /// Set when the node is created
        pub let id: String

        /// The type of node:
        /// 1 = collection
        /// 2 = consensus
        /// 3 = execution
        /// 4 = verification
        /// 5 = access
        pub var role: UInt8

        pub(set) var networkingAddress: String
        pub(set) var networkingKey: String
        pub(set) var stakingKey: String

        /// TODO: Proof of Possession (PoP) of the staking private key

        /// The total tokens that only this node currently has staked, not including delegators
        /// This value must always be above the minimum requirement to stay staked or accept delegators
        pub var tokensStaked: @FlowToken.Vault

        /// The tokens that this node has committed to stake for the next epoch.
        /// Moves to the tokensStaked bucket at the end of an epoch
        pub var tokensCommitted: @FlowToken.Vault

        /// The tokens that this node has unstaked from the previous epoch
        /// Moves to the tokensUnstaked bucket at the end of an epoch.
        pub var tokensUnstaking: @FlowToken.Vault

        /// Tokens that this node has unstaked and are able to withdraw whenever they want
        pub var tokensUnstaked: @FlowToken.Vault

        /// Staking rewards are paid to this bucket
        pub var tokensRewarded: @FlowToken.Vault

        /// list of delegators for this node operator
        pub let delegators: @{UInt32: DelegatorRecord}

        /// The incrementing ID used to register new delegators
        pub(set) var delegatorIDCounter: UInt32

        /// The amount of tokens that this node has requested to unstake for the next epoch
        pub(set) var tokensRequestedToUnstake: UFix64

        /// weight as determined by the amount staked after the staking auction
        pub(set) var initialWeight: UInt64

        init(
            id: String,
            role: UInt8,
            networkingAddress: String,
            networkingKey: String,
            stakingKey: String,
            tokensCommitted: @FungibleToken.Vault
        ) {
            pre {
                id.length == 64: "Node ID length must be 32 bytes (64 hex characters)"
                FlowIDTableStaking.isValidNodeID(id): "The node ID must have only numbers and lowercase hex characters"
                FlowIDTableStaking.nodes[id] == nil: "The ID cannot already exist in the record"
                role >= UInt8(1) && role <= UInt8(5): "The role must be 1, 2, 3, 4, or 5"
                networkingAddress.length > 0 && networkingAddress.length <= 510: "The networkingAddress must be less than 510 characters"
                networkingKey.length == 128: "The networkingKey length must be exactly 64 bytes (128 hex characters)"
                stakingKey.length == 192: "The stakingKey length must be exactly 96 bytes (192 hex characters)"
                !FlowIDTableStaking.getNetworkingAddressClaimed(address: networkingAddress): "The networkingAddress cannot have already been claimed"
                !FlowIDTableStaking.getNetworkingKeyClaimed(key: networkingKey): "The networkingKey cannot have already been claimed"
                !FlowIDTableStaking.getStakingKeyClaimed(key: stakingKey): "The stakingKey cannot have already been claimed"
            }

            let stakeKey = PublicKey(
                publicKey: stakingKey.decodeHex(),
                signatureAlgorithm: SignatureAlgorithm.BLS_BLS12_381
            )

            let netKey = PublicKey(
                publicKey: networkingKey.decodeHex(),
                signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
            )

            // TODO: Verify the provided Proof of Possession of the staking private key

            self.id = id
            self.role = role
            self.networkingAddress = networkingAddress
            self.networkingKey = networkingKey
            self.stakingKey = stakingKey
            self.initialWeight = 0
            self.delegators <- {}
            self.delegatorIDCounter = 0

            FlowIDTableStaking.updateClaimed(path: /storage/networkingAddressesClaimed, networkingAddress, claimed: true)
            FlowIDTableStaking.updateClaimed(path: /storage/networkingKeysClaimed, networkingKey, claimed: true)
            FlowIDTableStaking.updateClaimed(path: /storage/stakingKeysClaimed, stakingKey, claimed: true)

            self.tokensCommitted <- tokensCommitted as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaking <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0

            emit NewNodeCreated(nodeID: self.id, role: self.role, amountCommitted: self.tokensCommitted.balance)
        }

        destroy() {
            let flowTokenRef = FlowIDTableStaking.account.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
            FlowIDTableStaking.totalTokensStakedByNodeType[self.role] = FlowIDTableStaking.totalTokensStakedByNodeType[self.role]! - self.tokensStaked.balance
            flowTokenRef.deposit(from: <-self.tokensStaked)
            flowTokenRef.deposit(from: <-self.tokensCommitted)
            flowTokenRef.deposit(from: <-self.tokensUnstaking)
            flowTokenRef.deposit(from: <-self.tokensUnstaked)
            flowTokenRef.deposit(from: <-self.tokensRewarded)

            // Return all of the delegators' funds
            for delegator in self.delegators.keys {
                let delRecord = self.borrowDelegatorRecord(delegator)
                flowTokenRef.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                flowTokenRef.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensStaked.balance))
                flowTokenRef.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: delRecord.tokensUnstaked.balance))
                flowTokenRef.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: delRecord.tokensRewarded.balance))
                flowTokenRef.deposit(from: <-delRecord.tokensUnstaking.withdraw(amount: delRecord.tokensUnstaking.balance))
            }

            destroy self.delegators
        }

        /// Utility Function that checks a node's overall committed balance from its borrowed record
        access(account) fun nodeFullCommittedBalance(): UFix64 {
            if (self.tokensCommitted.balance + self.tokensStaked.balance) < self.tokensRequestedToUnstake {
                return 0.0
            } else {
                return self.tokensCommitted.balance + self.tokensStaked.balance - self.tokensRequestedToUnstake
            }
        }

        /// borrow a reference to to one of the delegators for a node in the record
        access(account) fun borrowDelegatorRecord(_ delegatorID: UInt32): &DelegatorRecord {
            pre {
                self.delegators[delegatorID] != nil:
                    "Specified delegator ID does not exist in the record"
            }
            return &self.delegators[delegatorID] as! &DelegatorRecord
        }
    }

    /// Struct to create to get read-only info about a node
    pub struct NodeInfo {
        pub let id: String
        pub let role: UInt8
        pub let networkingAddress: String
        pub let networkingKey: String
        pub let stakingKey: String
        pub let tokensStaked: UFix64
        pub let tokensCommitted: UFix64
        pub let tokensUnstaking: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRewarded: UFix64

        /// list of delegator IDs for this node operator
        pub let delegators: [UInt32]
        pub let delegatorIDCounter: UInt32
        pub let tokensRequestedToUnstake: UFix64
        pub let initialWeight: UInt64

        init(nodeID: String) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            self.id = nodeRecord.id
            self.role = nodeRecord.role
            self.networkingAddress = nodeRecord.networkingAddress
            self.networkingKey = nodeRecord.networkingKey
            self.stakingKey = nodeRecord.stakingKey
            self.tokensStaked = nodeRecord.tokensStaked.balance
            self.tokensCommitted = nodeRecord.tokensCommitted.balance
            self.tokensUnstaking = nodeRecord.tokensUnstaking.balance
            self.tokensUnstaked = nodeRecord.tokensUnstaked.balance
            self.tokensRewarded = nodeRecord.tokensRewarded.balance
            self.delegators = nodeRecord.delegators.keys
            self.delegatorIDCounter = nodeRecord.delegatorIDCounter
            self.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake
            self.initialWeight = nodeRecord.initialWeight
        }

        /// Derived Fields
        pub fun totalCommittedWithDelegators(): UFix64 {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)
            var committedSum = self.totalCommittedWithoutDelegators()
            for delegator in self.delegators {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)
                committedSum = committedSum + delRecord.delegatorFullCommittedBalance()
            }
            return committedSum
        }

        pub fun totalCommittedWithoutDelegators(): UFix64 {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)
            return nodeRecord.nodeFullCommittedBalance()
        }

        pub fun totalStakedWithDelegators(): UFix64 {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)
            var stakedSum = self.tokensStaked
            for delegator in self.delegators {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)
                stakedSum = stakedSum + delRecord.tokensStaked.balance
            }
            return stakedSum
        }

        pub fun totalTokensInRecord(): UFix64 {
            return self.tokensStaked + self.tokensCommitted + self.tokensUnstaking + self.tokensUnstaked + self.tokensRewarded
        }
    }

    /// Records the staking info associated with a delegator
    /// This resource is stored in the NodeRecord object that is being delegated to
    pub resource DelegatorRecord {
        /// Tokens this delegator has committed for the next epoch
        pub var tokensCommitted: @FlowToken.Vault

        /// Tokens this delegator has staked for the current epoch
        pub var tokensStaked: @FlowToken.Vault

        /// Tokens this delegator has requested to unstake and is locked for the current epoch
        pub var tokensUnstaking: @FlowToken.Vault

        /// Tokens this delegator has been rewarded and can withdraw
        pub let tokensRewarded: @FlowToken.Vault

        /// Tokens that this delegator unstaked and can withdraw
        pub let tokensUnstaked: @FlowToken.Vault

        /// Amount of tokens that the delegator has requested to unstake
        pub(set) var tokensRequestedToUnstake: UFix64

        init() {
            self.tokensCommitted <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaking <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0
        }

        destroy () {
            destroy self.tokensCommitted
            destroy self.tokensStaked
            destroy self.tokensUnstaking
            destroy self.tokensRewarded
            destroy self.tokensUnstaked
        }

        /// Utility Function that checks a delegator's overall committed balance from its borrowed record
        access(contract) fun delegatorFullCommittedBalance(): UFix64 {
            if (self.tokensCommitted.balance + self.tokensStaked.balance) < self.tokensRequestedToUnstake {
                return 0.0
            } else {
                return self.tokensCommitted.balance + self.tokensStaked.balance - self.tokensRequestedToUnstake
            }
        }
    }

    /// Struct that can be returned to show all the info about a delegator
    pub struct DelegatorInfo {
        pub let id: UInt32
        pub let nodeID: String
        pub let tokensCommitted: UFix64
        pub let tokensStaked: UFix64
        pub let tokensUnstaking: UFix64
        pub let tokensRewarded: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRequestedToUnstake: UFix64

        init(nodeID: String, delegatorID: UInt32) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)
            let delegatorRecord = nodeRecord.borrowDelegatorRecord(delegatorID)
            self.id = delegatorID
            self.nodeID = nodeID
            self.tokensCommitted = delegatorRecord.tokensCommitted.balance
            self.tokensStaked = delegatorRecord.tokensStaked.balance
            self.tokensUnstaking = delegatorRecord.tokensUnstaking.balance
            self.tokensUnstaked = delegatorRecord.tokensUnstaked.balance
            self.tokensRewarded = delegatorRecord.tokensRewarded.balance
            self.tokensRequestedToUnstake = delegatorRecord.tokensRequestedToUnstake
        }

        pub fun totalTokensInRecord(): UFix64 {
            return self.tokensStaked + self.tokensCommitted + self.tokensUnstaking + self.tokensUnstaked + self.tokensRewarded
        }
    }

    pub resource interface NodeStakerPublic {
        pub let id: String
    }

    /// Resource that the node operator controls for staking
    pub resource NodeStaker: NodeStakerPublic {

        /// Unique ID for the node operator
        pub let id: String

        init(id: String) {
            self.id = id
        }

        /// Change the node's networking address to a new one
        pub fun updateNetworkingAddress(_ newAddress: String) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot update networking address if the staking auction isn't in progress"
                newAddress.length > 0 && newAddress.length <= 510: "The networkingAddress must be less than 510 characters"
                !FlowIDTableStaking.getNetworkingAddressClaimed(address: newAddress): "The networkingAddress cannot have already been claimed"
            }

            // Borrow the node's record from the staking contract
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            FlowIDTableStaking.updateClaimed(path: /storage/networkingAddressesClaimed, nodeRecord.networkingAddress, claimed: false)

            nodeRecord.networkingAddress = newAddress

            FlowIDTableStaking.updateClaimed(path: /storage/networkingAddressesClaimed, newAddress, claimed: true)

            emit NetworkingAddressUpdated(nodeID: self.id, newAddress: newAddress)
        }

        /// Add new tokens to the system to stake during the next epoch
        pub fun stakeNewTokens(_ tokens: @FungibleToken.Vault) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot stake if the staking auction isn't in progress"
            }

            // Borrow the node's record from the staking contract
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit TokensCommitted(nodeID: nodeRecord.id, amount: tokens.balance)

            // Add the new tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-tokens)
        }

        /// Stake tokens that are in the tokensUnstaked bucket
        pub fun stakeUnstakedTokens(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot stake if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            var remainingAmount = amount

            // If there are any tokens that have been requested to unstake for the current epoch,
            // cancel those first before staking new unstaked tokens
            if remainingAmount <= nodeRecord.tokensRequestedToUnstake {
                nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake - remainingAmount
                remainingAmount = 0.0
            } else if remainingAmount > nodeRecord.tokensRequestedToUnstake {
                remainingAmount = remainingAmount - nodeRecord.tokensRequestedToUnstake
                nodeRecord.tokensRequestedToUnstake = 0.0
            }

            // Commit the remaining amount from the tokens unstaked bucket
            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensUnstaked.withdraw(amount: remainingAmount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: remainingAmount)
        }

        /// Stake tokens that are in the tokensRewarded bucket
        pub fun stakeRewardedTokens(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot stake if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensRewarded.withdraw(amount: amount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: amount)
        }

        /// Request amount tokens to be removed from staking at the end of the next epoch
        pub fun requestUnstaking(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot unstake if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            // If the request is greater than the total number of tokens
            // that can be unstaked, revert
            assert (
                nodeRecord.tokensStaked.balance +
                nodeRecord.tokensCommitted.balance
                >= amount + nodeRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            // Node operators who have delegators have to have enough of their own tokens staked
            // to meet the minimum, without any contributions from delegators
            assert (
                nodeRecord.delegators.length == 0 ||
                FlowIDTableStaking.isGreaterThanMinimumForRole(numTokens: FlowIDTableStaking.NodeInfo(nodeID: nodeRecord.id).totalCommittedWithoutDelegators() - amount, role: nodeRecord.role),
                message: "Cannot unstake below the minimum if there are delegators"
            )

            // Get the balance of the tokens that are currently committed
            let amountCommitted = nodeRecord.tokensCommitted.balance

            // If the request can come from committed, withdraw from committed to unstaked
            if amountCommitted >= amount {

                // withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                let amountCommitted = nodeRecord.tokensCommitted.balance

                // withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amountCommitted))

                // update request to show that leftover amount is requested to be unstaked
                nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Requests to unstake all of the node operators staked and committed tokens
        /// as well as all the staked and committed tokens of all of their delegators
        pub fun unstakeAll() {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot unstake if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            /// if the request can come from committed, withdraw from committed to unstaked
            /// withdraw the requested tokens from committed since they have not been staked yet
            nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))

            /// update request to show that leftover amount is requested to be unstaked
            nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance
        }

        /// Withdraw tokens from the unstaked bucket
        pub fun withdrawUnstakedTokens(amount: UFix64): @FungibleToken.Vault {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit UnstakedTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensUnstaked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit RewardTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensRewarded.withdraw(amount: amount)
        }
    }

    /// Public interface to query information about a delegator
    /// from the account it is stored in 
    pub resource interface NodeDelegatorPublic {
        pub let id: UInt32
        pub let nodeID: String
    }

    /// Resource object that the delegator stores in their account to perform staking actions
    pub resource NodeDelegator: NodeDelegatorPublic {

        pub let id: UInt32
        pub let nodeID: String

        init(id: UInt32, nodeID: String) {
            self.id = id
            self.nodeID = nodeID
        }

        /// Delegate new tokens to the node operator
        pub fun delegateNewTokens(from: @FungibleToken.Vault) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot delegate if the staking auction isn't in progress"
            }

            // borrow the node record of the node in order to get the delegator record
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorTokensCommitted(nodeID: self.nodeID, delegatorID: self.id, amount: from.balance)

            // Commit the new tokens to the delegator record
            delRecord.tokensCommitted.deposit(from: <-from)
        }

        /// Delegate tokens from the unstaked bucket to the node operator
        pub fun delegateUnstakedTokens(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot delegate if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            var remainingAmount = amount

            // If there are any tokens that have been requested to unstake for the current epoch,
            // cancel those first before staking new unstaked tokens
            if remainingAmount <= delRecord.tokensRequestedToUnstake {
                delRecord.tokensRequestedToUnstake = delRecord.tokensRequestedToUnstake - remainingAmount
                remainingAmount = 0.0
            } else if remainingAmount > delRecord.tokensRequestedToUnstake {
                remainingAmount = remainingAmount - delRecord.tokensRequestedToUnstake
                delRecord.tokensRequestedToUnstake = 0.0
            }

            // Commit the remaining unstaked tokens
            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: remainingAmount))

            emit DelegatorTokensCommitted(nodeID: self.nodeID, delegatorID: self.id, amount: amount)
        }

        /// Delegate tokens from the rewards bucket to the node operator
        pub fun delegateRewardedTokens(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot delegate if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: amount))

            emit DelegatorTokensCommitted(nodeID: self.nodeID, delegatorID: self.id, amount: amount)
        }

        /// Request to unstake delegated tokens during the next epoch
        pub fun requestUnstaking(amount: UFix64) {
            pre {
                FlowIDTableStaking.stakingEnabled(): "Cannot request unstaking if the staking auction isn't in progress"
            }

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            // The delegator must have enough tokens to unstake
            assert (
                delRecord.tokensStaked.balance +
                delRecord.tokensCommitted.balance
                >= amount + delRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            // if the request can come from committed, withdraw from committed to unstaked
            if delRecord.tokensCommitted.balance >= amount {

                // withdraw the requested tokens from committed since they have not been staked yet
                delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                /// Get the balance of the tokens that are currently committed
                let amountCommitted = delRecord.tokensCommitted.balance

                if amountCommitted > 0.0 {
                    delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amountCommitted))
                }

                /// update request to show that leftover amount is requested to be unstaked
                delRecord.tokensRequestedToUnstake = delRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Withdraw tokens from the unstaked bucket
        pub fun withdrawUnstakedTokens(amount: UFix64): @FungibleToken.Vault {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorUnstakedTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            return <- delRecord.tokensUnstaked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)
            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorRewardTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            return <- delRecord.tokensRewarded.withdraw(amount: amount)
        }
    }

    pub struct RewardsBreakdown {
        pub let nodeID: String
        pub(set) var nodeRewards: UFix64
        pub let delegatorRewards: {UInt32: UFix64}

        init(nodeID: String) {
            self.nodeID = nodeID
            self.nodeRewards = 0.0
            self.delegatorRewards = {}
        }

        /// Scale the rewards of a single delegator by a scaling factor
        pub fun scaleDelegatorRewards(delegatorID: UInt32, scalingFactor: UFix64) {
            if let reward = self.delegatorRewards[delegatorID] {
                    self.delegatorRewards[delegatorID] = reward * scalingFactor
            }
        }
        
        pub fun scaleOperatorRewards(scalingFactor: UFix64) {
            self.nodeRewards = self.nodeRewards * scalingFactor
        }

        /// Scale the rewards of all the stakers in the record
        pub fun scaleAllRewards(scalingFactor: UFix64) {
            self.scaleOperatorRewards(scalingFactor: scalingFactor)
            for id in self.delegatorRewards.keys {
                self.scaleDelegatorRewards(delegatorID: id, scalingFactor: scalingFactor)
            }
        }
    }
    
    /// Admin resource that has the ability to create new staker objects, remove insufficiently staked nodes
    /// at the end of the staking auction, and pay rewards to nodes at the end of an epoch
    pub resource Admin {
        pub fun removeNode(_ nodeID: String): @NodeRecord {
            let node <- FlowIDTableStaking.nodes.remove(key: nodeID)
                ?? panic("Could not find a node with the specified ID")

            FlowIDTableStaking.updateClaimed(path: /storage/networkingAddressesClaimed, node.networkingAddress, claimed: false)
            FlowIDTableStaking.updateClaimed(path: /storage/networkingKeysClaimed, node.networkingKey, claimed: false)
            FlowIDTableStaking.updateClaimed(path: /storage/stakingKeysClaimed, node.stakingKey, claimed: false)

            return <-node
        }

        /// Sets a list of approved node IDs for the next epoch
        /// Nodes not on this list will be unstaked at the end of the staking auction
        /// and not considered to be a proposed/staked node
        pub fun setApprovedList(_ nodeIDs: [String]) {
            let list = FlowIDTableStaking.account.load<[String]>(from: /storage/idTableApproveList)

            FlowIDTableStaking.account.save<[String]>(nodeIDs, to: /storage/idTableApproveList)
        }

        /// Sets a list of node IDs who will not receive rewards for the current epoch
        /// This is used during epochs to punish nodes who have poor uptime 
        /// or who do not update to latest node software quickly enough
        /// The parameter is a dictionary mapping node IDs
        /// to a percentage, which is the percentage of their expected rewards that
        /// they will receive instead of the full amount
        pub fun setNonOperationalNodesList(_ nodeIDs: {String: UFix64}) {
            for percentage in nodeIDs.values {
                assert(
                    percentage >= 0.0 && percentage < 1.0,
                    message: "Percentage value to decrease rewards payout should be between 0 and 1"
                )
            }

            let list = FlowIDTableStaking.account.load<{String: UFix64}>(from: /storage/idTableNonOperationalNodesList)

            FlowIDTableStaking.account.save<{String: UFix64}>(nodeIDs, to: /storage/idTableNonOperationalNodesList)
        }

        /// Starts the staking auction, the period when nodes and delegators
        /// are allowed to perform staking related operations
        pub fun startStakingAuction() {
            FlowIDTableStaking.account.load<Bool>(from: /storage/stakingEnabled)
            FlowIDTableStaking.account.save(true, to: /storage/stakingEnabled)
        }

        /// Ends the staking Auction by removing any unapproved nodes
        /// and setting stakingEnabled to false
        pub fun endStakingAuction() {
            let approvedList = FlowIDTableStaking.getApprovedList()
            let approvedNodeIDs: {String: Bool} = {}
            for id in approvedList {
                approvedNodeIDs[id] = true
            }

            self.removeUnapprovedNodes(approvedNodeIDs: approvedNodeIDs)

            FlowIDTableStaking.account.load<Bool>(from: /storage/stakingEnabled)
            FlowIDTableStaking.account.save(false, to: /storage/stakingEnabled)
        }

        /// Iterates through all the registered nodes and if it finds
        /// a node that has insufficient tokens committed for the next epoch or isn't in the approved list
        /// it moves their committed tokens to their unstaked bucket
        ///
        /// Parameter: approvedNodeIDs: A list of nodeIDs that have been approved
        /// by the protocol to be a staker for the next epoch. The node software
        /// checks if the node that corresponds to each proposed ID is running properly
        /// and that its node info is correct
        pub fun removeUnapprovedNodes(approvedNodeIDs: {String: Bool}) {
            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            for nodeID in allNodeIDs {
                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                let totalTokensCommitted = nodeRecord.nodeFullCommittedBalance()

                // Remove nodes if they do not meet the minimum staking requirements
                if !FlowIDTableStaking.isGreaterThanMinimumForRole(numTokens: totalTokensCommitted, role: nodeRecord.role) ||
                   (approvedNodeIDs[nodeID] == nil) {

                    emit NodeRemovedAndRefunded(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance)

                    // move their committed tokens back to their unstaked tokens
                    nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))

                    // Set their request to unstake equal to all their staked tokens
                    // since they are forced to unstake
                    nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance

                    // Iterate through all delegators and unstake their tokens
                    // since their node has unstaked
                    for delegator in nodeRecord.delegators.keys {
                        let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                        if delRecord.tokensCommitted.balance > 0.0 {
                            emit DelegatorTokensUnstaked(nodeID: nodeRecord.id, delegatorID: delegator, amount: delRecord.tokensCommitted.balance)

                            // move their committed tokens back to their unstaked tokens
                            delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                        }

                        // Request to unstake all tokens
                        delRecord.tokensRequestedToUnstake = delRecord.tokensStaked.balance
                    }

                    // Clear initial weight because the node is not staked any more
                    nodeRecord.initialWeight = 0
                } else {
                    // Set weight to 100
                    // Calculations for node weight will come with a future version of epochs
                    nodeRecord.initialWeight = 100
                }
            }
        }

        /// Called at the end of the epoch to pay rewards to node operators
        /// based on the tokens that they have staked
        pub fun payRewards(_ rewardsBreakdownArray: [RewardsBreakdown]) {

            let totalRewards = FlowIDTableStaking.epochTokenPayout
            let feeBalance = FlowFees.getFeeBalance()
            var mintedRewards: UFix64 = 0.0
            if feeBalance < totalRewards {
                mintedRewards = totalRewards - feeBalance
            }

            // Borrow the fee admin and withdraw all the fees that have been collected since the last rewards payment
            let feeAdmin = FlowIDTableStaking.borrowFeesAdmin()
            let rewardsVault <- feeAdmin.withdrawTokensFromFeeVault(amount: feeBalance)

            // Mint the remaining FLOW for rewards
            if mintedRewards > 0.0 {
                let flowTokenMinter = FlowIDTableStaking.account.borrow<&FlowToken.Minter>(from: /storage/flowTokenMinter)
                    ?? panic("Could not borrow minter reference")
                rewardsVault.deposit(from: <-flowTokenMinter.mintTokens(amount: mintedRewards))
            }

            for rewardBreakdown in rewardsBreakdownArray {
                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(rewardBreakdown.nodeID)
                let nodeReward = rewardBreakdown.nodeRewards
                
                nodeRecord.tokensRewarded.deposit(from: <-rewardsVault.withdraw(amount: nodeReward))

                emit RewardsPaid(nodeID: rewardBreakdown.nodeID, amount: nodeReward)

                for delegator in rewardBreakdown.delegatorRewards.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)
                    let delegatorReward = rewardBreakdown.delegatorRewards[delegator]!
                        
                    delRecord.tokensRewarded.deposit(from: <-rewardsVault.withdraw(amount: delegatorReward))

                    emit DelegatorRewardsPaid(nodeID: rewardBreakdown.nodeID, delegatorID: delegator, amount: delegatorReward)
                }
            }

            var fromFees = feeBalance
            if feeBalance >= totalRewards {
                fromFees = totalRewards
            }
            emit EpochTotalRewardsPaid(total: totalRewards, fromFees: fromFees, minted: mintedRewards, feesBurned: rewardsVault.balance)

            // Clear the non-operational node list so it doesn't persist to the next rewards payment
            let emptyNodeList: {String: UFix64} = {}
            self.setNonOperationalNodesList(emptyNodeList)

            // Destroy the remaining fees, even if there are some left
            destroy rewardsVault
        }

        /// Calculates rewards for all the staked node operators and delegators
        pub fun calculateRewards(): [RewardsBreakdown] {
            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            // Get the sum of all tokens staked
            var totalStaked = FlowIDTableStaking.getTotalStaked()
            if totalStaked == 0.0 {
                return []
            }
            // Calculate the scale to be multiplied by number of tokens staked per node
            var totalRewardScale = FlowIDTableStaking.epochTokenPayout / totalStaked

            var rewardsBreakdownArray: [FlowIDTableStaking.RewardsBreakdown] = []

            // The total rewards that are withheld from the non-operational nodes
            var sumRewardsWithheld = 0.0

            // The total amount of stake from non-operational nodes and delegators
            var sumStakeFromNonOperationalStakers = 0.0

            // Iterate through all the non-operational nodes and calculate
            // their rewards that will be withheld
            let nonOperationalNodes = FlowIDTableStaking.getNonOperationalNodesList()
            for nodeID in nonOperationalNodes.keys {
                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                // Each node's rewards can be decreased to a different percentage
                // Its delegator's rewards are also decreased to the same percentage
                let rewardDecreaseToPercentage = nonOperationalNodes[nodeID]!

                sumStakeFromNonOperationalStakers = sumStakeFromNonOperationalStakers + nodeRecord.tokensStaked.balance

                // Calculate the normal reward amount, then the rewards left after the decrease
                var nodeRewardAmount = nodeRecord.tokensStaked.balance * totalRewardScale
                var nodeRewardsAfterWithholding = nodeRewardAmount * rewardDecreaseToPercentage

                // Add the remaining to the total number of rewards withheld
                sumRewardsWithheld = sumRewardsWithheld + (nodeRewardAmount - nodeRewardsAfterWithholding)

                let rewardsBreakdown = FlowIDTableStaking.RewardsBreakdown(nodeID: nodeID)

                // Iterate through all the withheld node's delegators
                // and calculate their decreased rewards as well
                for delegator in nodeRecord.delegators.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    sumStakeFromNonOperationalStakers = sumStakeFromNonOperationalStakers + delRecord.tokensStaked.balance

                    // Calculate the amount of tokens that this delegator receives
                    // decreased to the percentage from the non-operational node
                    var delegatorRewardAmount = delRecord.tokensStaked.balance * totalRewardScale
                    var delegatorRewardsAfterWithholding = delegatorRewardAmount * rewardDecreaseToPercentage

                    // Add the withheld rewards to the total sum
                    sumRewardsWithheld = sumRewardsWithheld + (delegatorRewardAmount - delegatorRewardsAfterWithholding)

                    if delegatorRewardsAfterWithholding == 0.0 { continue }

                    // take the node operator's cut
                    if (delegatorRewardsAfterWithholding * FlowIDTableStaking.nodeDelegatingRewardCut) > 0.0 {

                        let nodeCutAmount = delegatorRewardsAfterWithholding * FlowIDTableStaking.nodeDelegatingRewardCut

                        nodeRewardsAfterWithholding = nodeRewardsAfterWithholding + nodeCutAmount

                        delegatorRewardsAfterWithholding = delegatorRewardsAfterWithholding - nodeCutAmount
                    }
                    rewardsBreakdown.delegatorRewards[delegator] = delegatorRewardsAfterWithholding
                }

                rewardsBreakdown.nodeRewards = nodeRewardsAfterWithholding
                rewardsBreakdownArray.append(rewardsBreakdown)
            }

            var withheldRewardsScale = sumRewardsWithheld / (totalStaked - sumStakeFromNonOperationalStakers)
            let totalRewardsPlusWithheld = totalRewardScale + withheldRewardsScale

            /// iterate through all the nodes to pay
            for nodeID in allNodeIDs {
                if nonOperationalNodes[nodeID] != nil { continue }

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                var nodeRewardAmount = nodeRecord.tokensStaked.balance * totalRewardsPlusWithheld

                if nodeRewardAmount == 0.0 || nodeRecord.role == UInt8(5)  { continue }

                let rewardsBreakdown = FlowIDTableStaking.RewardsBreakdown(nodeID: nodeID)

                // Iterate through all delegators and reward them their share
                // of the rewards for the tokens they have staked for this node
                for delegator in nodeRecord.delegators.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    /// Calculate the amount of tokens that this delegator receives
                    var delegatorRewardAmount = delRecord.tokensStaked.balance * totalRewardsPlusWithheld

                    if delegatorRewardAmount == 0.0 { continue }

                    // take the node operator's cut
                    if (delegatorRewardAmount * FlowIDTableStaking.nodeDelegatingRewardCut) > 0.0 {

                        let nodeCutAmount = delegatorRewardAmount * FlowIDTableStaking.nodeDelegatingRewardCut

                        nodeRewardAmount = nodeRewardAmount + nodeCutAmount

                        delegatorRewardAmount = delegatorRewardAmount - nodeCutAmount
                    }
                    rewardsBreakdown.delegatorRewards[delegator] = delegatorRewardAmount
                }
                
                rewardsBreakdown.nodeRewards = nodeRewardAmount
                rewardsBreakdownArray.append(rewardsBreakdown)
            }
            return rewardsBreakdownArray
        }

        /// Called at the end of the epoch to move tokens between buckets
        /// for stakers
        /// Tokens that have been committed are moved to the staked bucket
        /// Tokens that were unstaking during the last epoch are fully unstaked
        /// Unstaking requests are filled by moving those tokens from staked to unstaking
        pub fun moveTokens() {
            pre {
                !FlowIDTableStaking.stakingEnabled(): "Cannot move tokens if the staking auction is still in progress"
            }
            
            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            for nodeID in allNodeIDs {
                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + nodeRecord.tokensCommitted.balance

                // mark the committed tokens as staked
                if nodeRecord.tokensCommitted.balance > 0.0 {
                    emit TokensStaked(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance)
                    nodeRecord.tokensStaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))
                }

                // marked the unstaking tokens as unstaked
                if nodeRecord.tokensUnstaking.balance > 0.0 {
                    emit TokensUnstaked(nodeID: nodeRecord.id, amount: nodeRecord.tokensUnstaking.balance)
                    nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensUnstaking.withdraw(amount: nodeRecord.tokensUnstaking.balance))
                }

                // unstake the requested tokens and move them to tokensUnstaking
                if nodeRecord.tokensRequestedToUnstake > 0.0 {
                    emit TokensUnstaking(nodeID: nodeRecord.id, amount: nodeRecord.tokensRequestedToUnstake)
                    nodeRecord.tokensUnstaking.deposit(from: <-nodeRecord.tokensStaked.withdraw(amount: nodeRecord.tokensRequestedToUnstake))
                }

                // move all the delegators' tokens between buckets
                for delegator in nodeRecord.delegators.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + delRecord.tokensCommitted.balance

                    // mark their committed tokens as staked
                    if delRecord.tokensCommitted.balance > 0.0 {
                        emit DelegatorTokensStaked(nodeID: nodeRecord.id, delegatorID: delegator, amount: delRecord.tokensCommitted.balance)
                        delRecord.tokensStaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                    }

                    // marked the unstaking tokens as unstaked
                    if delRecord.tokensUnstaking.balance > 0.0 {
                        emit DelegatorTokensUnstaked(nodeID: nodeRecord.id, delegatorID: delegator, amount: delRecord.tokensUnstaking.balance)
                        delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensUnstaking.withdraw(amount: delRecord.tokensUnstaking.balance))
                    }

                    // unstake the requested tokens and move them to tokensUnstaking
                    if delRecord.tokensRequestedToUnstake > 0.0 {
                        emit DelegatorTokensUnstaking(nodeID: nodeRecord.id, delegatorID: delegator, amount: delRecord.tokensRequestedToUnstake)
                        delRecord.tokensUnstaking.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensRequestedToUnstake))
                    }

                    // subtract their requested tokens from the total staked for their node type
                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - delRecord.tokensRequestedToUnstake

                    delRecord.tokensRequestedToUnstake = 0.0
                }

                // subtract their requested tokens from the total staked for their node type
                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - nodeRecord.tokensRequestedToUnstake

                // Reset the tokens requested field so it can be used for the next epoch
                nodeRecord.tokensRequestedToUnstake = 0.0
            }

            // Start the new epoch's staking auction
            self.startStakingAuction()

            // Set the current Epoch node list
            FlowIDTableStaking.setCurrentNodeList(FlowIDTableStaking.getApprovedList())

            // Indicates that the tokens have moved and the epoch has ended
            // Tells what the new reward payout will be. The new payout is calculated and changed
            // before this method is executed and will not be changed for the rest of the epoch
            emit NewEpoch(totalStaked: FlowIDTableStaking.getTotalStaked(), totalRewardPayout: FlowIDTableStaking.epochTokenPayout)
        }

        /// Sets a new set of minimum staking requirements for all the nodes
        pub fun setMinimumStakeRequirements(_ newRequirements: {UInt8: UFix64}) {
            pre {
                newRequirements.keys.length == 5: "Incorrect number of nodes"
            }
            FlowIDTableStaking.minimumStakeRequired = newRequirements
            emit NewStakingMinimums(newMinimums: newRequirements)
        }

        /// Changes the total weekly payout to a new value
        pub fun setEpochTokenPayout(_ newPayout: UFix64) {
            if newPayout != FlowIDTableStaking.epochTokenPayout {
                emit NewWeeklyPayout(newPayout: newPayout)
            }
            FlowIDTableStaking.epochTokenPayout = newPayout
        }

        /// Sets a new delegator cut percentage that nodes take from delegator rewards
        pub fun setCutPercentage(_ newCutPercentage: UFix64) {
            pre {
                newCutPercentage > 0.0 && newCutPercentage < 1.0:
                    "Cut percentage must be between 0 and 1!"
            }
            if newCutPercentage != FlowIDTableStaking.nodeDelegatingRewardCut {
                emit NewDelegatorCutPercentage(newCutPercentage: newCutPercentage)
            }
            FlowIDTableStaking.nodeDelegatingRewardCut = newCutPercentage
        }

        /// Called only once when the contract is upgraded to use the claimed storage fields
        /// to initialize all their values
        pub fun setClaimed() {

            let claimedNetAddressDictionary: {String: Bool} = {}

            for nodeID in FlowIDTableStaking.nodes.keys {
                claimedNetAddressDictionary[FlowIDTableStaking.nodes[nodeID]?.networkingAddress!] = true
            }

            let oldDictionary = FlowIDTableStaking.account.load<{String: Bool}>(from: /storage/networkingAddressesClaimed)
            FlowIDTableStaking.account.save(claimedNetAddressDictionary, to: /storage/networkingAddressesClaimed)
        }
    }

    /// Any user can call this function to register a new Node
    /// It returns the resource for nodes that they can store in their account storage
    pub fun addNodeRecord(id: String, role: UInt8, networkingAddress: String, networkingKey: String, stakingKey: String, tokensCommitted: @FungibleToken.Vault): @NodeStaker {
        pre {
            FlowIDTableStaking.stakingEnabled(): "Cannot register a node operator if the staking auction isn't in progress"
        }
        let newNode <- create NodeRecord(id: id, role: role, networkingAddress: networkingAddress, networkingKey: networkingKey, stakingKey: stakingKey, tokensCommitted: <-tokensCommitted)
        FlowIDTableStaking.nodes[id] <-! newNode

        // return a new NodeStaker object that the node operator stores in their account
        return <-create NodeStaker(id: id)
    }

    /// Registers a new delegator with a unique ID for the specified node operator
    /// and returns a delegator object to the caller
    pub fun registerNewDelegator(nodeID: String): @NodeDelegator {
        pre {
            FlowIDTableStaking.stakingEnabled(): "Cannot register a node operator if the staking auction isn't in progress"
        }

        let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

        assert (
            nodeRecord.role != UInt8(5),
            message: "Cannot register a delegator for an access node"
        )

        assert (
            FlowIDTableStaking.isGreaterThanMinimumForRole(numTokens: nodeRecord.nodeFullCommittedBalance(), role: nodeRecord.role),
            message: "Cannot register a delegator if the node operator is below the minimum stake"
        )

        // increment the delegator ID counter for this node
        nodeRecord.delegatorIDCounter = nodeRecord.delegatorIDCounter + UInt32(1)

        // Create a new delegator record and store it in the contract
        nodeRecord.delegators[nodeRecord.delegatorIDCounter] <-! create DelegatorRecord()

        emit NewDelegatorCreated(nodeID: nodeRecord.id, delegatorID: nodeRecord.delegatorIDCounter)

        // Return a new NodeDelegator object that the owner stores in their account
        return <-create NodeDelegator(id: nodeRecord.delegatorIDCounter, nodeID: nodeRecord.id)
    }

    /// borrow a reference to to one of the nodes in the record
    access(account) fun borrowNodeRecord(_ nodeID: String): &NodeRecord {
        pre {
            FlowIDTableStaking.nodes[nodeID] != nil:
                "Specified node ID does not exist in the record"
        }
        return &FlowIDTableStaking.nodes[nodeID] as! &NodeRecord
    }

    /// borrow a reference to the `FlowFees` admin resource for paying rewards
    access(account) fun borrowFeesAdmin(): &FlowFees.Administrator {
        let feesAdmin = self.account.borrow<&FlowFees.Administrator>(from: /storage/flowFeesAdmin)
            ?? panic("Could not borrow a reference to the FlowFees Admin object")

        return feesAdmin
    }

    /// Updates a claimed boolean for a specific path to indicate that
    /// a piece of node metadata has been claimed by a node
    access(account) fun updateClaimed(path: StoragePath, _ key: String, claimed: Bool) {
        let claimedDictionary = self.account.load<{String: Bool}>(from: path)
            ?? panic("Invalid path for dictionary")

        if claimed {
            claimedDictionary[key] = true
        } else {
            claimedDictionary[key] = nil
        }

        self.account.save(claimedDictionary, to: path)
    }

    /// Sets a list of approved node IDs for the current epoch
    access(contract) fun setCurrentNodeList(_ nodeIDs: [String]) {
        let list = self.account.load<[String]>(from: /storage/idTableCurrentList)

        self.account.save<[String]>(nodeIDs, to: /storage/idTableCurrentList)
    }

    /// Checks if the given string has all numbers or lowercase hex characters
    /// Used to ensure that there are no duplicate node IDs
    pub fun isValidNodeID(_ input: String): Bool {
        let byteVersion = input.utf8

        for character in byteVersion {
            if ((character < 48) || (character > 57 && character < 97) || (character > 102)) {
                return false
            }
        }

        return true
    }

    /// Indicates if the staking auction is currently enabled
    pub fun stakingEnabled(): Bool {
        return self.account.copy<Bool>(from: /storage/stakingEnabled) ?? false
    }

    /// Gets an array of the node IDs that are proposed and approved for the next epoch
    pub fun getProposedNodeIDs(): [String] {
        var proposedNodes: [String] = []

        let approvedList = FlowIDTableStaking.getApprovedList()
        let approvedNodeIDs: {String: Bool} = {}
        for id in approvedList {
            approvedNodeIDs[id] = true
        }

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)
            let approved = approvedNodeIDs[nodeID] ?? false

            // To be considered proposed, a node has to have tokens staked + committed equal or above the minimum
            // Access nodes have a minimum of 0, so they need to be strictly greater than zero to be considered proposed
            if self.isGreaterThanMinimumForRole(numTokens: self.NodeInfo(nodeID: nodeRecord.id).totalCommittedWithoutDelegators(), role: nodeRecord.role)
               && approved
            {
                proposedNodes.append(nodeID)
            }
        }
        return proposedNodes
    }

    /// Gets an array of all the nodeIDs that are staked.
    /// Only nodes that are participating in the current epoch
    /// can be staked, so this is an array of all the active
    /// node operators
    pub fun getStakedNodeIDs(): [String] {
        var stakedNodes: [String] = []

        let currentList = self.account.copy<[String]>(from: /storage/idTableCurrentList)
            ?? panic("Could not get current list")
        let currentNodeIDs: {String: Bool} = {}
        for id in currentList {
            currentNodeIDs[id] = true
        }

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)
            let current = currentNodeIDs[nodeID] ?? false

            // To be considered staked, a node has to have tokens staked equal or above the minimum
            // Access nodes have a minimum of 0, so they need to be strictly greater than zero to be considered staked
            if self.isGreaterThanMinimumForRole(numTokens: nodeRecord.tokensStaked.balance, role: nodeRecord.role)
               && current
            {
                stakedNodes.append(nodeID)
            }
        }
        return stakedNodes
    }

    /// Gets an array of all the node IDs that have ever registered
    pub fun getNodeIDs(): [String] {
        return FlowIDTableStaking.nodes.keys
    }

    /// Checks if the amount of tokens is greater
    /// than the minimum staking requirement for the specified role
    pub fun isGreaterThanMinimumForRole(numTokens: UFix64, role: UInt8): Bool {
        return numTokens >= self.minimumStakeRequired[role]!
    }

    /// Indicates if the specified networking address is claimed by a node
    pub fun getNetworkingAddressClaimed(address: String): Bool {
        return self.getClaimed(path: /storage/networkingAddressesClaimed, key: address)
    }

    /// Indicates if the specified networking key is claimed by a node
    pub fun getNetworkingKeyClaimed(key: String): Bool {
        return self.getClaimed(path: /storage/networkingKeysClaimed, key: key)
    }

    /// Indicates if the specified staking key is claimed by a node
    pub fun getStakingKeyClaimed(key: String): Bool {
        return self.getClaimed(path: /storage/stakingKeysClaimed, key: key)
    }

    /// Gets the claimed status of a particular piece of node metadata
    access(account) fun getClaimed(path: StoragePath, key: String): Bool {
		let claimedDictionary = self.account.borrow<&{String: Bool}>(from: path)
            ?? panic("Invalid path for dictionary")
        return claimedDictionary[key] ?? false
    }

    /// Returns the list of approved node IDs that the admin has set
    pub fun getApprovedList(): [String] {
        return self.account.copy<[String]>(from: /storage/idTableApproveList)
            ?? panic("could not get approved list")
    }

    /// Returns the list of node IDs whose rewards will be reduced in the next payment
    pub fun getNonOperationalNodesList(): {String: UFix64} {
        return self.account.copy<{String: UFix64}>(from: /storage/idTableNonOperationalNodesList)
            ?? panic("could not get non-operational node list")
    }

    /// Gets the minimum stake requirements for all the node types
    pub fun getMinimumStakeRequirements(): {UInt8: UFix64} {
        return self.minimumStakeRequired
    }

    /// Gets a dictionary that indicates the current number of tokens staked
    /// by all the nodes of each type
    pub fun getTotalTokensStakedByNodeType(): {UInt8: UFix64} {
        return self.totalTokensStakedByNodeType
    }

    /// Gets the total number of FLOW that is currently staked
    /// by all of the staked nodes in the current epoch
    pub fun getTotalStaked(): UFix64 {
        var totalStaked: UFix64 = 0.0
        for nodeType in FlowIDTableStaking.totalTokensStakedByNodeType.keys {
            // Do not count access nodes
            if nodeType != UInt8(5) {
                totalStaked = totalStaked + FlowIDTableStaking.totalTokensStakedByNodeType[nodeType]!
            }
        }
        return totalStaked
    }

    /// Gets the token payout value for the current epoch
    pub fun getEpochTokenPayout(): UFix64 {
        return self.epochTokenPayout
    }

    /// Gets the cut percentage for delegator rewards paid to node operators
    pub fun getRewardCutPercentage(): UFix64 {
        return self.nodeDelegatingRewardCut
    }

    /// Gets the ratios of rewards that different node roles recieve
    /// NOTE: Currently is not used
    pub fun getRewardRatios(): {UInt8: UFix64} {
        return self.rewardRatios
    }

    init(_ epochTokenPayout: UFix64, _ rewardCut: UFix64) {
        self.account.save(true, to: /storage/stakingEnabled)

        self.nodes <- {}

        let claimedDictionary: {String: Bool} = {}
        self.account.save(claimedDictionary, to: /storage/stakingKeysClaimed)
        self.account.save(claimedDictionary, to: /storage/networkingKeysClaimed)
        self.account.save(claimedDictionary, to: /storage/networkingAddressesClaimed)

        self.NodeStakerStoragePath = /storage/flowStaker
        self.NodeStakerPublicPath = /public/flowStaker
        self.StakingAdminStoragePath = /storage/flowStakingAdmin
        self.DelegatorStoragePath = /storage/flowStakingDelegator

        self.minimumStakeRequired = {UInt8(1): 250000.0, UInt8(2): 500000.0, UInt8(3): 1250000.0, UInt8(4): 135000.0, UInt8(5): 0.0}
        self.totalTokensStakedByNodeType = {UInt8(1): 0.0, UInt8(2): 0.0, UInt8(3): 0.0, UInt8(4): 0.0, UInt8(5): 0.0}
        self.epochTokenPayout = epochTokenPayout
        self.nodeDelegatingRewardCut = rewardCut
        self.rewardRatios = {UInt8(1): 0.168, UInt8(2): 0.518, UInt8(3): 0.078, UInt8(4): 0.236, UInt8(5): 0.0}

        let list: [String] = []
        self.setCurrentNodeList(list)
        self.account.save<[String]>(list, to: /storage/idTableApproveList)

        let nonOperationalList: {String: UFix64} = {}
        self.account.save<{String: UFix64}>(nonOperationalList, to: /storage/idTableNonOperationalNodesList)

        self.account.save(<-create Admin(), to: self.StakingAdminStoragePath)
    }
}
 
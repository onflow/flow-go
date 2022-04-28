/*

    BloctoTokenStaking

    The BloctoToken Staking contract manages stakers' information.
    Forked from FlowIDTableStaking contract.

 */

import FungibleToken from 0xf233dcee88fe0abe
import BloctoToken from 0x0f9df91c9121c460

pub contract BloctoTokenStaking {

    /****** Staking Events ******/

    pub event NewEpoch(epoch: UInt64, totalStaked: UFix64, totalRewardPayout: UFix64)

    /// Staker Events
    pub event NewStakerCreated(stakerID: UInt64, amountCommitted: UFix64)
    pub event TokensCommitted(stakerID: UInt64, amount: UFix64)
    pub event TokensStaked(stakerID: UInt64, amount: UFix64)
    pub event TokensUnstaking(stakerID: UInt64, amount: UFix64)
    pub event TokensUnstaked(stakerID: UInt64, amount: UFix64)
    pub event NodeRemovedAndRefunded(stakerID: UInt64, amount: UFix64)
    pub event RewardsPaid(stakerID: UInt64, amount: UFix64)
    pub event MoveToken(stakerID: UInt64)
    pub event UnstakedTokensWithdrawn(stakerID: UInt64, amount: UFix64)
    pub event RewardTokensWithdrawn(stakerID: UInt64, amount: UFix64)

    /// Contract Field Change Events
    pub event NewWeeklyPayout(newPayout: UFix64)

    /// Holds the identity table for all the stakers in the network.
    /// Includes stakers that aren't actively participating
    /// key = staker ID (also corresponds to BloctoPass ID)
    /// value = the record of that staker's info, tokens, and delegators
    access(contract) var stakers: @{UInt64: StakerRecord}

    /// The total amount of tokens that are staked for all the stakers
    access(contract) var totalTokensStaked: UFix64

    /// The total amount of tokens that are paid as rewards every epoch
    /// could be manually changed by the admin resource
    access(contract) var epochTokenPayout: UFix64

    /// Indicates if the staking auction is currently enabled
    access(contract) var stakingEnabled: Bool

    /// Paths for storing staking resources
    pub let StakingAdminStoragePath: StoragePath

    /*********** Staking Composite Type Definitions *************/

    /// Contains information that is specific to a staker
    pub resource StakerRecord {

        /// The unique ID of the staker
        /// Corresponds to the BloctoPass NFT ID
        pub let id: UInt64

        /// The total tokens that only this staker currently has staked
        pub var tokensStaked: @BloctoToken.Vault

        /// The tokens that this staker has committed to stake for the next epoch.
        pub var tokensCommitted: @BloctoToken.Vault

        /// Tokens that this staker is able to withdraw whenever they want
        pub var tokensUnstaked: @BloctoToken.Vault

        /// Staking rewards are paid to this bucket
        /// Can be withdrawn whenever
        pub var tokensRewarded: @BloctoToken.Vault

        /// The amount of tokens that this staker has requested to unstake for the next epoch
        pub(set) var tokensRequestedToUnstake: UFix64

        init(id: UInt64) {
            pre {
                BloctoTokenStaking.stakers[id] == nil: "The ID cannot already exist in the record"
            }

            self.id = id

            self.tokensCommitted <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault
            self.tokensStaked <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault
            self.tokensUnstaked <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault
            self.tokensRewarded <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault
            self.tokensRequestedToUnstake = 0.0

            emit NewStakerCreated(stakerID: self.id, amountCommitted: self.tokensCommitted.balance)
        }

        destroy() {
            let bloctoTokenRef = BloctoTokenStaking.account.borrow<&BloctoToken.Vault>(from: BloctoToken.TokenStoragePath)!
            BloctoTokenStaking.totalTokensStaked = BloctoTokenStaking.totalTokensStaked - self.tokensStaked.balance
            bloctoTokenRef.deposit(from: <-self.tokensStaked)
            bloctoTokenRef.deposit(from: <-self.tokensCommitted)
            bloctoTokenRef.deposit(from: <-self.tokensUnstaked)
            bloctoTokenRef.deposit(from: <-self.tokensRewarded)
        }

        /// Utility Function that checks a staker's overall committed balance from its borrowed record
        access(contract) fun stakerFullCommittedBalance(): UFix64 {
            if (self.tokensCommitted.balance + self.tokensStaked.balance) < self.tokensRequestedToUnstake {
                return 0.0
            } else {
                return self.tokensCommitted.balance + self.tokensStaked.balance - self.tokensRequestedToUnstake
            }
        }
    }

    /// Struct to create to get read-only info about a staker
    pub struct StakerInfo {
        pub let id: UInt64
        pub let tokensStaked: UFix64
        pub let tokensCommitted: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRewarded: UFix64
        pub let tokensRequestedToUnstake: UFix64

        init(stakerID: UInt64) {
            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(stakerID)

            self.id = stakerRecord.id
            self.tokensStaked = stakerRecord.tokensStaked.balance
            self.tokensCommitted = stakerRecord.tokensCommitted.balance
            self.tokensUnstaked = stakerRecord.tokensUnstaked.balance
            self.tokensRewarded = stakerRecord.tokensRewarded.balance
            self.tokensRequestedToUnstake = stakerRecord.tokensRequestedToUnstake
        }

        pub fun totalTokensInRecord(): UFix64 {
            return self.tokensStaked + self.tokensCommitted + self.tokensUnstaked + self.tokensRewarded
        }
    }

    pub resource interface StakerPublic {
        pub let id: UInt64
    }

    /// Resource that the staker operator controls for staking
    pub resource Staker: StakerPublic {

        /// Unique ID for the staker operator
        pub let id: UInt64

        init(id: UInt64) {
            self.id = id
        }

        /// Add new tokens to the system to stake during the next epoch
        pub fun stakeNewTokens(_ tokens: @FungibleToken.Vault) {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot stake if the staking auction isn't in progress"
            }

            // Borrow the staker's record from the staking contract
            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            emit TokensCommitted(stakerID: stakerRecord.id, amount: tokens.balance)

            // Add the new tokens to tokens committed
            stakerRecord.tokensCommitted.deposit(from: <-tokens)
        }

        /// Stake tokens that are in the tokensUnstaked bucket
        pub fun stakeUnstakedTokens(amount: UFix64) {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot stake if the staking auction isn't in progress"
            }

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            var remainingAmount = amount

            // If there are any tokens that have been requested to unstake for the current epoch,
            // cancel those first before staking new unstaked tokens
            if remainingAmount <= stakerRecord.tokensRequestedToUnstake {
                stakerRecord.tokensRequestedToUnstake = stakerRecord.tokensRequestedToUnstake - remainingAmount
                remainingAmount = 0.0
            } else if remainingAmount > stakerRecord.tokensRequestedToUnstake {
                remainingAmount = remainingAmount - stakerRecord.tokensRequestedToUnstake
                stakerRecord.tokensRequestedToUnstake = 0.0
            }

            // Commit the remaining amount from the tokens unstaked bucket
            stakerRecord.tokensCommitted.deposit(from: <-stakerRecord.tokensUnstaked.withdraw(amount: remainingAmount))

            emit TokensCommitted(stakerID: stakerRecord.id, amount: remainingAmount)
        }

        /// Stake tokens that are in the tokensRewarded bucket
        pub fun stakeRewardedTokens(amount: UFix64) {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot stake if the staking auction isn't in progress"
            }

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            stakerRecord.tokensCommitted.deposit(from: <-stakerRecord.tokensRewarded.withdraw(amount: amount))

            emit TokensCommitted(stakerID: stakerRecord.id, amount: amount)
        }

        /// Request amount tokens to be removed from staking at the end of the next epoch
        pub fun requestUnstaking(amount: UFix64) {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot unstake if the staking auction isn't in progress"
            }

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            // If the request is greater than the total number of tokens
            // that can be unstaked, revert
            assert (
                stakerRecord.tokensStaked.balance +
                stakerRecord.tokensCommitted.balance
                >= amount + stakerRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            // Get the balance of the tokens that are currently committed
            let amountCommitted = stakerRecord.tokensCommitted.balance

            // If the request can come from committed, withdraw from committed to unstaked
            if amountCommitted >= amount {

                // withdraw the requested tokens from committed since they have not been staked yet
                stakerRecord.tokensUnstaked.deposit(from: <-stakerRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                let amountCommitted = stakerRecord.tokensCommitted.balance

                // withdraw the requested tokens from committed since they have not been staked yet
                stakerRecord.tokensUnstaked.deposit(from: <-stakerRecord.tokensCommitted.withdraw(amount: amountCommitted))

                // update request to show that leftover amount is requested to be unstaked
                stakerRecord.tokensRequestedToUnstake = stakerRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Requests to unstake all of the staker operators staked and committed tokens
        /// as well as all the staked and committed tokens of all of their delegators
        pub fun unstakeAll() {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot unstake if the staking auction isn't in progress"
            }

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            /// if the request can come from committed, withdraw from committed to unstaked
            /// withdraw the requested tokens from committed since they have not been staked yet
            stakerRecord.tokensUnstaked.deposit(from: <-stakerRecord.tokensCommitted.withdraw(amount: stakerRecord.tokensCommitted.balance))

            /// update request to show that leftover amount is requested to be unstaked
            stakerRecord.tokensRequestedToUnstake = stakerRecord.tokensStaked.balance
        }

        /// Withdraw tokens from the unstaked bucket
        pub fun withdrawUnstakedTokens(amount: UFix64): @FungibleToken.Vault {

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            emit UnstakedTokensWithdrawn(stakerID: stakerRecord.id, amount: amount)

            return <- stakerRecord.tokensUnstaked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {

            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(self.id)

            emit RewardTokensWithdrawn(stakerID: stakerRecord.id, amount: amount)

            return <- stakerRecord.tokensRewarded.withdraw(amount: amount)
        }
    }

    /// Admin resource that has the ability to create new staker objects and pay rewards
    /// to stakers at the end of an epoch
    pub resource Admin {

        /// A staker record is created when a BloctoPass NFT is created
        /// It returns the resource for stakers that they can store in their account storage
        pub fun addStakerRecord(id: UInt64): @Staker {
            pre {
                BloctoTokenStaking.stakingEnabled: "Cannot register a staker operator if the staking auction isn't in progress"
            }

            let newStakerRecord <- create StakerRecord(id: id)

            // Insert the staker to the table
            BloctoTokenStaking.stakers[id] <-! newStakerRecord

            // return a new Staker object that the staker operator stores in their account
            return <-create Staker(id: id)
        }

        /// Starts the staking auction, the period when stakers and delegators
        /// are allowed to perform staking related operations
        pub fun startNewEpoch() {
            BloctoTokenStaking.stakingEnabled = true
            BloctoTokenStaking.setEpoch(BloctoTokenStaking.getEpoch() + 1)
            emit NewEpoch(epoch: BloctoTokenStaking.getEpoch(), totalStaked: BloctoTokenStaking.getTotalStaked(), totalRewardPayout: BloctoTokenStaking.epochTokenPayout)
        }

        /// Starts the staking auction, the period when stakers and delegators
        /// are allowed to perform staking related operations
        pub fun startStakingAuction() {
            BloctoTokenStaking.stakingEnabled = true
        }

        /// Ends the staking Auction by removing any unapproved stakers
        /// and setting stakingEnabled to false
        pub fun endStakingAuction() {
            BloctoTokenStaking.stakingEnabled = false
        }

        /// Called at the end of the epoch to pay rewards to staker operators
        /// based on the tokens that they have staked
        pub fun payRewards(_ stakerIDs: [UInt64]) {
            pre {
                !BloctoTokenStaking.stakingEnabled: "Cannot pay rewards if the staking auction is still in progress"
            }

            let BloctoTokenMinter = BloctoTokenStaking.account.borrow<&BloctoToken.Minter>(from: /storage/bloctoTokenStakingMinter)
                ?? panic("Could not borrow minter reference")

            // calculate the total number of tokens staked
            var totalStaked = BloctoTokenStaking.getTotalStaked()

            if totalStaked == 0.0 {
                return
            }
            var totalRewardScale = BloctoTokenStaking.epochTokenPayout / totalStaked

            var stakingRewardRecordsRefOpt = BloctoTokenStaking.account.borrow<&{String: Bool}>(from: /storage/bloctoTokenStakingStakingRewardRecords)
            if stakingRewardRecordsRefOpt == nil {
                BloctoTokenStaking.account.save<{String: Bool}>({} as {String: Bool}, to: /storage/bloctoTokenStakingStakingRewardRecords)
                stakingRewardRecordsRefOpt = BloctoTokenStaking.account.borrow<&{String: Bool}>(from: /storage/bloctoTokenStakingStakingRewardRecords)
            }
            var stakingRewardRecordsRef = stakingRewardRecordsRefOpt!
            let epoch = BloctoTokenStaking.getEpoch()

            /// iterate through stakers to pay
            for stakerID in stakerIDs {
                // add reward record
                let key = BloctoTokenStaking.getStakingRewardKey(epoch: epoch, stakerID: stakerID)
                if stakingRewardRecordsRef[key] != nil && stakingRewardRecordsRef[key]! {
                    continue
                }
                stakingRewardRecordsRef[key] = true

                let stakerRecord = BloctoTokenStaking.borrowStakerRecord(stakerID)

                if stakerRecord.tokensStaked.balance == 0.0 { continue }

                let rewardAmount = stakerRecord.tokensStaked.balance * totalRewardScale

                if rewardAmount == 0.0 { continue }

                /// Mint the tokens to reward the operator
                let tokenReward <- BloctoTokenMinter.mintTokens(amount: rewardAmount)

                if tokenReward.balance > 0.0 {
                    emit RewardsPaid(stakerID: stakerRecord.id, amount: tokenReward.balance)

                    /// Deposit the staker Rewards into their tokensRewarded bucket
                    stakerRecord.tokensRewarded.deposit(from: <-tokenReward)
                } else {
                    destroy tokenReward
                }
            }
        }

        /// Called at the end of the epoch to move tokens between buckets
        /// for stakers
        /// Tokens that have been committed are moved to the staked bucket
        /// Tokens that were unstaking during the last epoch are fully unstaked
        /// Unstaking requests are filled by moving those tokens from staked to unstaking
        pub fun moveTokens(_ stakerIDs: [UInt64]) {
            pre {
                !BloctoTokenStaking.stakingEnabled: "Cannot move tokens if the staking auction is still in progress"
            }

            for stakerID in stakerIDs {
                // get staker record
                let stakerRecord = BloctoTokenStaking.borrowStakerRecord(stakerID)

                // Update total number of tokens staked by all the stakers of each type
                BloctoTokenStaking.totalTokensStaked = BloctoTokenStaking.totalTokensStaked + stakerRecord.tokensCommitted.balance

                // mark the committed tokens as staked
                if stakerRecord.tokensCommitted.balance > 0.0 {
                    emit TokensStaked(stakerID: stakerRecord.id, amount: stakerRecord.tokensCommitted.balance)
                    stakerRecord.tokensStaked.deposit(from: <-stakerRecord.tokensCommitted.withdraw(amount: stakerRecord.tokensCommitted.balance))
                }

                // unstake the requested tokens and move them to tokensUnstaking
                if stakerRecord.tokensRequestedToUnstake > 0.0 {
                    emit TokensUnstaked(stakerID: stakerRecord.id, amount: stakerRecord.tokensRequestedToUnstake)
                    stakerRecord.tokensUnstaked.deposit(from: <-stakerRecord.tokensStaked.withdraw(amount: stakerRecord.tokensRequestedToUnstake))

                    // subtract their requested tokens from the total staked for their staker type
                    BloctoTokenStaking.totalTokensStaked = BloctoTokenStaking.totalTokensStaked - stakerRecord.tokensRequestedToUnstake

                    // Reset the tokens requested field so it can be used for the next epoch
                    stakerRecord.tokensRequestedToUnstake = 0.0
                }

                emit MoveToken(stakerID: stakerID)
            }
        }

        /// Changes the total weekly payout to a new value
        pub fun setEpochTokenPayout(_ newPayout: UFix64) {
            BloctoTokenStaking.epochTokenPayout = newPayout

            emit NewWeeklyPayout(newPayout: newPayout)
        }
    }

    /// borrow a reference to to one of the stakers in the record
    access(account) fun borrowStakerRecord(_ stakerID: UInt64): &StakerRecord {
        pre {
            BloctoTokenStaking.stakers[stakerID] != nil:
                "Specified staker ID does not exist in the record"
        }
        return &BloctoTokenStaking.stakers[stakerID] as! &StakerRecord
    }

    /// Gets an array of all the stakerIDs that are staked.
    /// Only stakers that are participating in the current epoch
    /// can be staked, so this is an array of all the active stakers
    pub fun getStakedStakerIDs(): [UInt64] {
        var stakers: [UInt64] = []

        for stakerID in BloctoTokenStaking.getStakerIDs() {
            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(stakerID)

            if stakerRecord.tokensStaked.balance > 0.0
            {
                stakers.append(stakerID)
            }
        }

        return stakers
    }

    /// Gets an slice of stakerIDs who's staking balance > 0
    pub fun getStakedStakerIDsSlice(start: UInt64, end: UInt64): [UInt64] {
        // all staker ids
        var allStakerIDs: [UInt64] = BloctoTokenStaking.getStakerIDs()

        // output
        var stakers: [UInt64] = []

        // filter staker ids by staking balance
        var current = start
        while current < end {
            let stakerID = allStakerIDs[current]
            let stakerRecord = BloctoTokenStaking.borrowStakerRecord(stakerID)
            if stakerRecord.tokensStaked.balance > 0.0 {
                stakers.append(stakerID)
            }
            current = current + 1
        }
        return stakers
    }

    /// Gets an array of all the staker IDs that have ever registered
    pub fun getStakerIDs(): [UInt64] {
        return BloctoTokenStaking.stakers.keys
    }

    /// Gets an slice of stakerIDs who's staking balance > 0
    pub fun getStakerIDsSlice(start: UInt64, end: UInt64): [UInt64] {
        // all staker ids
        var allStakerIDs: [UInt64] = BloctoTokenStaking.getStakerIDs()

        // output
        var stakers: [UInt64] = []

        // filter staker ids by staking balance
        var current = start
        while current < end {
            stakers.append(allStakerIDs[current])
            current = current + 1
        }
        return stakers
    }

    /// Gets staker id count
    pub fun getStakerIDCount(): Int {
        return BloctoTokenStaking.stakers.keys.length
    }

    /// Gets the token payout value for the current epoch
    pub fun getEpochTokenPayout(): UFix64 {
        return self.epochTokenPayout
    }

    pub fun getStakingEnabled(): Bool {
        return self.stakingEnabled
    }

    /// Gets the total number of BLT that is currently staked
    /// by all of the staked stakers in the current epoch
    pub fun getTotalStaked(): UFix64 {
        return BloctoTokenStaking.totalTokensStaked
    }

    /// Epoch
    pub fun getEpoch(): UInt64 {
        return self.account.copy<UInt64>(from: /storage/bloctoTokenStakingEpoch) ?? (0 as UInt64);
    }

    access(contract) fun setEpoch(_ epoch: UInt64) {
        self.account.load<UInt64>(from: /storage/bloctoTokenStakingEpoch)
        self.account.save<UInt64>(epoch, to: /storage/bloctoTokenStakingEpoch)
    }


    access(contract) fun getStakingRewardKey(epoch: UInt64, stakerID: UInt64): String {
        // key: {EPOCH}_{STAKER_ID}
        return epoch.toString().concat("_").concat(stakerID.toString())
    }

    /// staking reward records
    pub fun hasSentStakingReward(epoch: UInt64, stakerID: UInt64): Bool {
        let stakingRewardRecordsRef = self.account.borrow<&{String: Bool}>(from: /storage/bloctoTokenStakingStakingRewardRecords)
        if stakingRewardRecordsRef == nil {
            return false
        }

        let key = BloctoTokenStaking.getStakingRewardKey(epoch: epoch, stakerID: stakerID)
        if stakingRewardRecordsRef![key] == nil {
            return false
        }

        return stakingRewardRecordsRef![key]!
    }

    init() {
        self.stakingEnabled = true

        self.stakers <- {}

        self.StakingAdminStoragePath = /storage/bloctoTokenStakingAdmin

        self.totalTokensStaked = 0.0
        self.epochTokenPayout = 1.0

        self.account.save(<-create Admin(), to: self.StakingAdminStoragePath)
    }
}

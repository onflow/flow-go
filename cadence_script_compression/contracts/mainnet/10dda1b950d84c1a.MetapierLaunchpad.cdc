import FungibleToken from 0xf233dcee88fe0abe
import MetapierLaunchpadPass from 0x10dda1b950d84c1a
import MetapierLaunchpadOwnerPass from 0x10dda1b950d84c1a

/**

The main contract that defines and manages Metapier launch pools.

 */
pub contract MetapierLaunchpad {

    pub event ContractInitialized()

    // Event that is emitted when a new pool is added to this launchpad. 
    pub event NewPoolAdded(poolId: String)

    // Event that is emitted when admin has updated the timeline of a pool.
    pub event TimelineUpdated(poolId: String)

    // Event that is emitted when admin has updated the price of a pool.
    pub event PriceUpdated(poolId: String)

    // Event that is emitted when admin has updated the personal cap of a pool.
    pub event PersonalCapUpdated(poolId: String)

    // Event that is emitted when admin has updated the total cap of a pool.
    pub event TotalCapUpdated(poolId: String)

    // Event that is emitted when admin has updated the whitelist of a pool.
    pub event WhitelistUpdated(poolId: String)

    // Event that is emitted when admin has deposited funds token into a pool.
    pub event AdminDepositedFunds(poolId: String, amount: UFix64)

    // Event that is emitted when admin has withdrawn funds token from a pool.
    pub event AdminWithdrewFunds(poolId: String, amount: UFix64)

    // Event that is emitted when admin has withdrawn launch token from a pool.
    pub event AdminWithdrewLaunchToken(poolId: String, amount: UFix64)

    // Event that is emitted when user has deposited funds token into a pool.
    pub event UserDepositedFunds(poolId: String, address: Address, amount: UFix64)

    // Event that is emitted when user has withdrawn funds token from a pool.
    pub event UserWithdrewFunds(poolId: String, address: Address, amount: UFix64)

    // Event that is emitted when user has claimed launch token from a pool.
    pub event UserClaimedLaunchToken(poolId: String, address: Address, amount: UFix64)

    // Event that is emitted when launch token has been deposited into a pool.
    pub event LaunchTokenDeposited(poolId: String, amount: UFix64)

    // Event that is emitted when a project owner has withdrawn funds from a pool.
    pub event ProjectOwnerWithdrewFunds(poolId: String, amount: UFix64)

    // A mapping from pool id to the corresponding launch pool
    access(contract) let pools: @{String: MetapierLaunchpad.LaunchPool}

    // A record of user participation in a launch pool
    pub struct ParticipantInfo {

        // Participant's address
        pub let address: Address

        // Id of the launchpad pass used by the participant
        pub let passId: UInt64

        // Amount of funds deposited by the participant
        pub var amount: UFix64

        // Has the participant claimed the launch token
        pub var hasClaimed: Bool

        init(
            address: Address,
            passId: UInt64
        ) {
            self.address = address
            self.passId = passId
            self.amount = 0.0
            self.hasClaimed = false
        }

        access(contract) fun setAmount(amount: UFix64) {
            self.amount = amount
        }

        access(contract) fun setClaimed() {
            self.hasClaimed = true
        }
    }

    // Launch pool functions that can be used by anyone
    pub resource interface PublicLaunchPool {
        
        pub let poolId: String
        
        // type of the funds token vault
        pub fun getFundsType(): Type

        // current balance of the funds token vault
        pub fun getFundsBalance(): UFix64

        // type of the launch token vault
        pub fun getLaunchTokenType(): Type

        // current balance of the launch token vault
        pub fun getLaunchTokenBalance(): UFix64

        // price is number of launch token per funds token
        pub fun getPrice(): UFix64
        
        // the max amount of funds one participant can deposit
        pub fun getPersonalCap(): UFix64

        // the total amount of funds that the launch project wants to target
        pub fun getTotalCap(): UFix64

        // a list of all participants of the pool
        pub fun getParticipants(): [Address]

        // a list of all participant info
        pub fun getAllParticipantInfo(): [ParticipantInfo]

        // searches for the participant info of a specific address
        pub fun getParticipantInfo(address: Address): ParticipantInfo?

        // Checks whether the given account address is whitelisted for the pool,
        // and it always returns true if whitelisting is not required.
        pub fun isWhitelisted(address: Address): Bool

        // The following three timestamps define the timeline of the pool.
        // It is guaranteed that:
        //   funding start time < funding end time < claiming start time
        // 
        // Participants are allowed to:
        //  1. Deposit funds between the funding start time and the funding end time.
        //  2. Withdraw funds between the funding start time and (the funding end time - fundsDepositOnlyPeriod).
        //  3. Withdraw launch token after the claiming start time.
        pub fun getFundingStartTime(): UFix64
        pub fun getFundingEndTime(): UFix64
        pub fun getClaimingStartTime(): UFix64
        pub fun getFundsDepositOnlyPeriod(): UFix64

        // A participant can call this function to deposit funds.
        // It will withdraw the amount of funds token from the private pass
        // and deposit them into the pool.
        pub fun depositFunds(privatePass: &MetapierLaunchpadPass.NFT, amount: UFix64) {
            pre {
                amount > 0.0: "Expecting a positive amount"
                privatePass.fundsType == self.getFundsType(): "Invalid funds type"
                privatePass.launchTokenType == self.getLaunchTokenType(): "Invalid launch token type"
                self.getFundsBalance() + amount <= self.getTotalCap(): "Cannot exceed total cap"
                self.getFundingStartTime() <= getCurrentBlock().timestamp: "Funds are frozen"
                getCurrentBlock().timestamp <= self.getFundingEndTime(): "Funds are frozen"
                self.isWhitelisted(address: privatePass.originalOwner): "Address not whitelisted"
            }
        }

        // A participant can call this function to withdraw deposited funds.
        // It require a private pass to prevent someone from withdrawing others'
        // funds. It will try withdrawing the requested amount of funds token
        // and deposit them into the pass.
        pub fun withdrawFunds(privatePass: &MetapierLaunchpadPass.NFT, amount: UFix64) {
            pre {
                amount > 0.0: "Expecting a positive amount"
                self.getFundingStartTime() <= getCurrentBlock().timestamp: "Funds are frozen"
                getCurrentBlock().timestamp <= self.getFundingEndTime() - self.getFundsDepositOnlyPeriod(): "Funds are frozen"
                self.getParticipantInfo(address: privatePass.originalOwner) != nil: "No participation record"
            }
        }

        // Anyone can call this function to claim the launch token for a 
        // participant. The launch token will be deposited into the 
        // participant's pass directly.
        pub fun claimLaunchToken(address: Address) {
            pre {
                self.getClaimingStartTime() <= getCurrentBlock().timestamp: "Claiming is not available"
                self.getParticipantInfo(address: address) != nil: "No participation record"
                !self.getParticipantInfo(address: address)!.hasClaimed: "This address has already claimed"
            }
            post {
                self.getParticipantInfo(address: address)!.hasClaimed: "Unexpected ParticipantInfo state"
            }
        }

        // The launch token/project owner should use this function to
        // deposit tokens to be claimed by participants.
        pub fun depositLaunchToken(vault: @FungibleToken.Vault) {
            pre {
                vault.balance > 0.0: "Deposit zero launch tokens"
            }
        }

        // The launch token/project owner can use this function to
        // withdraw funds raised after funding period is finished.
        pub fun ownerWithdrawFunds(
            ownerPass: &MetapierLaunchpadOwnerPass.NFT, 
            amount: UFix64
        ): @FungibleToken.Vault {
            pre {
                self.poolId == ownerPass.launchPoolId: "Invalid owner pass"
                self.getFundingEndTime() < getCurrentBlock().timestamp: "Can't withdraw in funding period"
            }
        }
    }

    // Implementation of the launch pool
    pub resource LaunchPool: PublicLaunchPool {

        pub let poolId: String

        pub(set) var price: UFix64
        pub(set) var personalCap: UFix64
        pub(set) var totalCap: UFix64

        pub(set) var fundingStartTime: UFix64
        pub(set) var fundingEndTime: UFix64
        pub(set) var claimingStartTime: UFix64
        pub(set) var fundsDepositOnlyPeriod: UFix64

        // A mapping acting as a set of whitelisted accounts.
        // If it's nil, whitelisting is not required for the pool.
        pub(set) var whitelist: {Address: Bool}?
        
        // a mapping from a participant's address to the corresponding
        // ParticipantInfo
        access(self) var participations: {Address: ParticipantInfo}

        access(contract) let fundsVault: @FungibleToken.Vault
        access(contract) let launchTokenVault: @FungibleToken.Vault

        pub fun getFundsType(): Type {
            return self.fundsVault.getType()
        }

        pub fun getFundsBalance(): UFix64 {
            return self.fundsVault.balance
        }

        pub fun getLaunchTokenType(): Type {
            return self.launchTokenVault.getType()
        }

        pub fun getLaunchTokenBalance(): UFix64 {
            return self.launchTokenVault.balance
        }

        pub fun getPrice(): UFix64 {
            return self.price
        }

        pub fun getPersonalCap(): UFix64 {
            return self.personalCap
        }

        pub fun getTotalCap(): UFix64 {
            return self.totalCap
        }

        pub fun getParticipants(): [Address] {
            return self.participations.keys
        }

        pub fun getAllParticipantInfo(): [ParticipantInfo] {
            return self.participations.values
        }

        pub fun getParticipantInfo(address: Address): ParticipantInfo? {
            return self.participations[address]
        }

        pub fun isWhitelisted(address: Address): Bool {
            if self.whitelist == nil {
                return true
            }

            return self.whitelist!.containsKey(address)
        }

        pub fun getFundingStartTime(): UFix64 {
            return self.fundingStartTime
        }

        pub fun getFundingEndTime(): UFix64 {
            return self.fundingEndTime
        }

        pub fun getClaimingStartTime(): UFix64 {
            return self.claimingStartTime
        }

        pub fun getFundsDepositOnlyPeriod(): UFix64 {
            return self.fundsDepositOnlyPeriod
        }

        // gets the public launchpad pass collection stored in the given address
        priv fun getPublicPassCollection(address: Address): 
            &MetapierLaunchpadPass.Collection{MetapierLaunchpadPass.CollectionPublic}
        {
            return getAccount(address)
                .getCapability<&MetapierLaunchpadPass.Collection{MetapierLaunchpadPass.CollectionPublic}>(
                    MetapierLaunchpadPass.CollectionPublicPath
                )
                .borrow()!
        }

        pub fun depositFunds(privatePass: &MetapierLaunchpadPass.NFT, amount: UFix64) {
            post {
                self.fundsVault.balance == before(self.fundsVault.balance) + amount:
                    "New funds balance must be the sum of the previous balance and the deposited amount"
            }

            let address = privatePass.originalOwner

            if self.whitelist != nil {
                // if whitelist is enabled, ensures the original owner still holds the pass
                let publicPass = self.getPublicPassCollection(address: address).borrowPublicPass(id: privatePass.id)
                assert(publicPass.uuid == privatePass.uuid, message: "Pass ownership has changed")
            }
            
            if !self.participations.containsKey(address) {
                // creates a new ParticipantInfo if one doesn't exist in records
                self.participations[address] = ParticipantInfo(address: address, passId: privatePass.id)
            }
            
            let participantInfo = &self.participations[address]! as &ParticipantInfo

            assert(!participantInfo.hasClaimed, message: "Unexpected ParticipantInfo state")
            if participantInfo.passId != privatePass.id {
                // should always use the same pass to participate a pool
                panic("Please use pass #".concat(participantInfo.passId.toString()))
            }

            participantInfo.setAmount(amount: participantInfo.amount + amount)
            assert(participantInfo.amount <= self.personalCap, message: "Cannot exceed personal cap")

            self.fundsVault.deposit(from: <- privatePass.withdrawFunds(amount: amount))

            emit UserDepositedFunds(poolId: self.poolId, address: address, amount: amount)
        }

        pub fun withdrawFunds(privatePass: &MetapierLaunchpadPass.NFT, amount: UFix64) {
            let address = privatePass.originalOwner
            let participantInfo = &self.participations[address]! as &ParticipantInfo
            assert(participantInfo.passId == privatePass.id, message: "Pass doesn't match with participation record")
            assert(!participantInfo.hasClaimed, message: "Unexpected ParticipantInfo state")
            assert(participantInfo.amount >= amount, message: "Cannot withdraw an amount more than deposited")

            participantInfo.setAmount(amount: participantInfo.amount - amount)

            privatePass.depositFunds(vault: <- self.fundsVault.withdraw(amount: amount))

            emit UserWithdrewFunds(poolId: self.poolId, address: address, amount: amount)
        }

        pub fun claimLaunchToken(address: Address) {
            let participantInfo = &self.participations[address]! as &ParticipantInfo

            participantInfo.setClaimed()
            
            let publicPass = self.getPublicPassCollection(address: address).borrowPublicPass(id: participantInfo.passId)
            let tokenAmount = participantInfo.amount * self.price
            publicPass.depositLaunchToken(vault: <- self.launchTokenVault.withdraw(amount: tokenAmount))

            emit UserClaimedLaunchToken(poolId: self.poolId, address: address, amount: tokenAmount)
        }

        pub fun depositLaunchToken(vault: @FungibleToken.Vault) {
            let amount = vault.balance
            self.launchTokenVault.deposit(from: <- vault)
            
            emit LaunchTokenDeposited(poolId: self.poolId, amount: amount)
        }

        pub fun ownerWithdrawFunds(
            ownerPass: &MetapierLaunchpadOwnerPass.NFT, 
            amount: UFix64
        ): @FungibleToken.Vault {
            let tempVault <- self.fundsVault.withdraw(amount: amount)

            emit ProjectOwnerWithdrewFunds(poolId: self.poolId, amount: amount)

            return <- tempVault
        }

        // ensures timeline setting follows the right order
        access(contract) fun validateTimeline() {
            pre {
                self.fundingStartTime < self.fundingEndTime: "Invalid funding period"
                self.fundingEndTime < self.claimingStartTime: "Token claiming can only happen after funding ends"
            }
        }

        init(
            poolId: String,
            fundsVault: @FungibleToken.Vault,
            launchTokenVault: @FungibleToken.Vault,
            price: UFix64,
            personalCap: UFix64,
            totalCap: UFix64,
            fundingStartTime: UFix64,
            fundingEndTime: UFix64,
            claimingStartTime: UFix64,
            fundsDepositOnlyPeriod: UFix64,
            whitelist: {Address: Bool}?
        ) {
            self.poolId = poolId
            self.fundsVault <- fundsVault
            self.launchTokenVault <- launchTokenVault
            self.price = price
            self.personalCap = personalCap
            self.totalCap = totalCap
            self.fundingStartTime = fundingStartTime
            self.fundingEndTime = fundingEndTime
            self.claimingStartTime = claimingStartTime
            self.fundsDepositOnlyPeriod = fundsDepositOnlyPeriod
            self.whitelist = whitelist
            self.participations = {}

            self.validateTimeline()
        }

        destroy() {
            destroy self.fundsVault
            destroy self.launchTokenVault
        }
    }

    // returns all the available launchpad pools stored in this contract
    pub fun getPoolIds(): [String] {
        return self.pools.keys
    }

    // gets a reference to the public portion of a launch pool by its id
    pub fun getPublicLaunchPoolById(poolId: String): &{MetapierLaunchpad.PublicLaunchPool}? {
        if self.pools.containsKey(poolId) {
            let poolRef = &self.pools[poolId] as auth &MetapierLaunchpad.LaunchPool
            return poolRef as &{MetapierLaunchpad.PublicLaunchPool}
        }
        return nil
    }

    pub resource Admin {

        // creates a new launch pool resource
        pub fun createPool(
            poolId: String,
            fundsVault: @FungibleToken.Vault,
            launchTokenVault: @FungibleToken.Vault,
            price: UFix64,
            personalCap: UFix64,
            totalCap: UFix64,
            fundingStartTime: UFix64,
            fundingEndTime: UFix64,
            claimingStartTime: UFix64,
            fundsDepositOnlyPeriod: UFix64,
            whitelist: {Address: Bool}?
        ): @MetapierLaunchpad.LaunchPool {
            return <- create LaunchPool(
                poolId: poolId,
                fundsVault: <- fundsVault,
                launchTokenVault: <- launchTokenVault,
                price: price,
                personalCap: personalCap,
                totalCap: totalCap,
                fundingStartTime: fundingStartTime,
                fundingEndTime: fundingEndTime,
                claimingStartTime: claimingStartTime,
                fundsDepositOnlyPeriod: fundsDepositOnlyPeriod,
                whitelist: whitelist
            )
        }

        // stores the given pool into this contract
        pub fun addPool(pool: @MetapierLaunchpad.LaunchPool) {
            pre {
                !MetapierLaunchpad.pools.containsKey(pool.poolId): "Pool already exists"
            }

            let poolId = pool.poolId
            // add the new pool to the dictionary with a force assignment
            // if there is already a value at that key, it will fail and revert
            MetapierLaunchpad.pools[poolId] <-! pool

            emit NewPoolAdded(poolId: poolId)
        }

        // updates the timeline of the corresponding pool
        // 
        // If an argument is nil, it means the corresponding timestamp shouldn't
        // change.
        pub fun updateTimeline(
            poolId: String,
            fundingStartTime: UFix64?,
            fundingEndTime: UFix64?,
            claimingStartTime: UFix64?,
            fundsDepositOnlyPeriod: UFix64?
        ) {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.fundingStartTime = fundingStartTime ?? poolRef.fundingStartTime
            poolRef.fundingEndTime = fundingEndTime ?? poolRef.fundingEndTime
            poolRef.claimingStartTime = claimingStartTime ?? poolRef.claimingStartTime
            poolRef.fundsDepositOnlyPeriod = fundsDepositOnlyPeriod ?? poolRef.fundsDepositOnlyPeriod
            poolRef.validateTimeline()

            emit TimelineUpdated(poolId: poolId)
        }

        pub fun updatePrice(poolId: String, price: UFix64) {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.price = price

            emit PriceUpdated(poolId: poolId)
        }

        pub fun updatePersonalCap(poolId: String, personalCap: UFix64) {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.personalCap = personalCap

            emit PersonalCapUpdated(poolId: poolId)
        }

        pub fun updateTotalCap(poolId: String, totalCap: UFix64) {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.totalCap = totalCap

            emit TotalCapUpdated(poolId: poolId)
        }

        pub fun updateWhitelist(poolId: String, whitelist: {Address: Bool}?) {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.whitelist = whitelist

            emit WhitelistUpdated(poolId: poolId)
        }

        pub fun addWhitelist(poolId: String, addresses: [Address]) {
            pre {
                addresses.length > 0: "Expecting a non-empty list"
            }

            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            if poolRef.whitelist == nil {
                poolRef.whitelist = {}
            }

            for addr in addresses {
                poolRef.whitelist!.insert(key: addr, true)
            }

            emit WhitelistUpdated(poolId: poolId)
        }

        pub fun removeWhitelist(poolId: String, addresses: [Address]) {
            pre {
                addresses.length > 0: "Expecting a non-empty list"
            }

            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            assert(poolRef.whitelist != nil, message: "No existing whitelist")

            for addr in addresses {
                poolRef.whitelist!.remove(key: addr)
            }

            emit WhitelistUpdated(poolId: poolId)
        }

        pub fun withdrawFunds(poolId: String, amount: UFix64): @FungibleToken.Vault {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            let tempVault <- poolRef.fundsVault.withdraw(amount: amount)

            emit AdminWithdrewFunds(poolId: poolId, amount: amount)

            return <- tempVault
        }

        pub fun depositFunds(poolId: String, vault: @FungibleToken.Vault) {
            let amount = vault.balance
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            poolRef.fundsVault.deposit(from: <-vault)

            emit AdminDepositedFunds(poolId: poolId, amount: amount)
        }

        pub fun withdrawLaunchToken(poolId: String, amount: UFix64): @FungibleToken.Vault {
            let poolRef = &MetapierLaunchpad.pools[poolId] as &MetapierLaunchpad.LaunchPool
            let tempVault <- poolRef.launchTokenVault.withdraw(amount: amount)

            emit AdminWithdrewLaunchToken(poolId: poolId, amount: amount)

            return <- tempVault
        }
    }

    init() {
        self.pools <- {}
        
        let admin <- create Admin()
        self.account.save(<- admin, to: /storage/MetapierLaunchpadAdmin)

        emit ContractInitialized()
    }
}
 
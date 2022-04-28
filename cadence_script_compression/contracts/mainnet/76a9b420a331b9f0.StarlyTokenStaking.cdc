// Starly staking.
//
// Main features:
//   * compound interests, periodic compounding with 1 second period
//   * APY 15%, configurable
//   * users can stake/unstake anytime
//   * stakes can have min staking time in seconds
//   * stake is basically a NFT that is stored in user's wallet
//
// Admin:
//   * create custom stakes
//   * ability to refund
//
// Configurable precautions:
//   * master switches to enable/disable staking and unstaking
//   * unstaking fees (flat and percent)
//   * unstaking penalty (if fees > interest)
//   * no unstaking fees after certain staking period
//   * timestamp until unstaking is disabled

import CompoundInterest from 0x76a9b420a331b9f0
import FungibleToken from 0xf233dcee88fe0abe
import MetadataViews from 0x1d7e57aa55817448
import NonFungibleToken from 0x1d7e57aa55817448
import StarlyToken from 0x142fa6570b62fd97

pub contract StarlyTokenStaking: NonFungibleToken {

    pub event TokensStaked(
        id: UInt64,
        address: Address?,
        principal: UFix64,
        stakeTimestamp: UFix64,
        minStakingSeconds: UFix64,
        k: UFix64)
    pub event TokensUnstaked(
        id: UInt64,
        address: Address?,
        amount: UFix64,
        principal: UFix64,
        interest: UFix64,
        unstakingFees: UFix64,
        stakeTimestamp: UFix64,
        unstakeTimestamp: UFix64,
        k: UFix64)
    pub event TokensBurned(id: UInt64, principal: UFix64)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event ContractInitialized()

    pub var totalSupply: UInt64
    pub var totalPrincipalStaked: UFix64
    pub var totalInterestPaid: UFix64

    pub var stakingEnabled: Bool
    pub var unstakingEnabled: Bool

    // the unstaking fees to unstake X tokens = unstakingFlatFee + unstakingFee * X
    pub var unstakingFee: UFix64
    pub var unstakingFlatFee: UFix64

    // unstake without fees if staked for this amount of seconds
    pub var unstakingFeesNotAppliedAfterSeconds: UFix64

    // cannot unstake if not staked for this amount of seconds
    pub var minStakingSeconds: UFix64

    // minimal principal for stake
    pub var minStakePrincipal: UFix64

    // cannot unstake until this timestamp
    pub var unstakingDisabledUntilTimestamp: UFix64

    // k = log10(1+r), where r is per-second interest ratio, taken from CompoundInterest contract
    pub var k: UFix64

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let MinterStoragePath: StoragePath
    pub let BurnerStoragePath: StoragePath

    pub resource interface StakePublic {
        pub fun getPrincipal(): UFix64
        pub fun getStakeTimestamp(): UFix64
        pub fun getMinStakingSeconds(): UFix64
        pub fun getK(): UFix64
        pub fun getAccumulatedAmount(): UFix64
        pub fun getUnstakingFees(): UFix64
        pub fun canUnstake(): Bool
    }

    pub struct StakeMetadataView {
        pub let id: UInt64
        pub let principal: UFix64
        pub let stakeTimestamp: UFix64
        pub let minStakingSeconds: UFix64
        pub let k: UFix64
        pub let accumulatedAmount: UFix64
        pub let canUnstake: Bool
        pub let unstakingFees: UFix64

        init(
            id: UInt64,
            principal: UFix64,
            stakeTimestamp: UFix64,
            minStakingSeconds: UFix64,
            k: UFix64,
            accumulatedAmount: UFix64,
            canUnstake: Bool,
            unstakingFees: UFix64) {
            self.id = id
            self.principal = principal
            self.stakeTimestamp = stakeTimestamp
            self.minStakingSeconds = minStakingSeconds
            self.k = k
            self.accumulatedAmount = accumulatedAmount
            self.canUnstake = canUnstake
            self.unstakingFees = unstakingFees
        }
    }

    // Stake (named as NFT to comply with NonFungibleToken interface) contains the vault with staked tokens
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver, StakePublic {
        pub let id: UInt64
        access(contract) let principalVault: @StarlyToken.Vault
        pub let stakeTimestamp: UFix64
        pub let minStakingSeconds: UFix64
        pub let k: UFix64

        init(
            id: UInt64,
            principalVault: @StarlyToken.Vault,
            stakeTimestamp: UFix64,
            minStakingSeconds: UFix64,
            k: UFix64) {
            self.id = id
            self.principalVault <-principalVault
            self.stakeTimestamp = stakeTimestamp
            self.minStakingSeconds = minStakingSeconds
            self.k = k
        }

        // if destroyed we destroy the tokens and decrease totalPrincipalStaked
        destroy() {
            let principalAmount = self.principalVault.balance
            destroy self.principalVault
            if (principalAmount > 0.0) {
                StarlyTokenStaking.totalPrincipalStaked = StarlyTokenStaking.totalPrincipalStaked - principalAmount
                emit TokensBurned(id: self.id, principal: principalAmount)
            }
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<StakeMetadataView>()
            ];
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: "StarlyToken stake #".concat(self.id.toString()),
                        description: "id: ".concat(self.id.toString())
                            .concat(", principal: ").concat(self.principalVault.balance.toString())
                            .concat(", k: ").concat(self.k.toString())
                            .concat(", stakeTimestamp: ").concat(UInt64(self.stakeTimestamp).toString())
                            .concat(", minStakingSeconds: ").concat(UInt64(self.minStakingSeconds).toString()),
                        thumbnail: MetadataViews.HTTPFile(url: ""))
                case Type<StakeMetadataView>():
                    return StakeMetadataView(
                        id: self.id,
                        principal: self.principalVault.balance,
                        stakeTimestamp: self.stakeTimestamp,
                        minStakingSeconds: self.minStakingSeconds,
                        k: self.k,
                        accumulatedAmount: self.getAccumulatedAmount(),
                        canUnstake: self.canUnstake(),
                        unstakingFees: self.getUnstakingFees())
            }
            return nil;
        }

        pub fun getPrincipal(): UFix64 {
            return self.principalVault.balance
        }

        pub fun getStakeTimestamp(): UFix64 {
            return self.stakeTimestamp
        }

        pub fun getMinStakingSeconds(): UFix64 {
            return self.minStakingSeconds
        }

        pub fun getK(): UFix64 {
            return self.k
        }

        pub fun getAccumulatedAmount(): UFix64 {
            let timestamp = getCurrentBlock().timestamp
            let seconds = timestamp - self.stakeTimestamp
            return self.principalVault.balance * CompoundInterest.generatedCompoundInterest(seconds: seconds, k: self.k)
        }

        // calculate unstaking fees using current StarlyTokenStaking parameters
        pub fun getUnstakingFees(): UFix64 {
            return self.getUnstakingFeesInternal(
                unstakingFee: StarlyTokenStaking.unstakingFee,
                unstakingFlatFee: StarlyTokenStaking.unstakingFlatFee,
                unstakingFeesNotAppliedAfterSeconds: StarlyTokenStaking.unstakingFeesNotAppliedAfterSeconds,
            )
        }

        // ability to calculate unstaking fees using provided parameters
        access(contract) fun getUnstakingFeesInternal(
            unstakingFee: UFix64,
            unstakingFlatFee: UFix64,
            unstakingFeesNotAppliedAfterSeconds: UFix64,
        ): UFix64 {
            let timestamp = getCurrentBlock().timestamp
            let seconds = timestamp - self.stakeTimestamp
            if (seconds >= unstakingFeesNotAppliedAfterSeconds) {
                return 0.0
            } else {
                let accumulatedAmount = self.getAccumulatedAmount()
                return unstakingFlatFee + unstakingFee * accumulatedAmount
            }
        }

        pub fun canUnstake(): Bool {
            let timestamp = getCurrentBlock().timestamp
            let seconds = timestamp - self.stakeTimestamp
            if (timestamp < StarlyTokenStaking.unstakingDisabledUntilTimestamp
                || seconds < self.minStakingSeconds
                || seconds < StarlyTokenStaking.minStakingSeconds
                || self.stakeTimestamp >= timestamp) {
                return false
            } else {
                return true
            }
        }
    }

    // We put stake creation logic into minter, its job is to have checks, emit events, update counters
    pub resource NFTMinter {

        pub fun mintStake(
            address: Address?,
            principalVault: @StarlyToken.Vault,
            stakeTimestamp: UFix64,
            minStakingSeconds: UFix64,
            k: UFix64): @StarlyTokenStaking.NFT {

            pre {
                StarlyTokenStaking.stakingEnabled: "Staking is disabled"
                principalVault.balance > 0.0: "Principal cannot be zero"
                principalVault.balance >= StarlyTokenStaking.minStakePrincipal: "Principal is too small"
                k <= CompoundInterest.k2000: "K cannot be larger than 2000% APY"
            }

            let stake <- create NFT(
                id: StarlyTokenStaking.totalSupply,
                principalVault: <-principalVault,
                stakeTimestamp: stakeTimestamp,
                minStakingSeconds: minStakingSeconds,
                k: k)
            let principalAmount = stake.principalVault.balance
            StarlyTokenStaking.totalSupply = StarlyTokenStaking.totalSupply + (1 as UInt64)
            StarlyTokenStaking.totalPrincipalStaked = StarlyTokenStaking.totalPrincipalStaked + principalAmount;
            emit TokensStaked(
                id: stake.id,
                address: address,
                principal: principalAmount,
                stakeTimestamp: stakeTimestamp,
                minStakingSeconds: minStakingSeconds,
                k: stake.k)
            return <-stake
        }
    }

    // We put stake unstaking logic into burner, its job is to have checks, emit events, update counters
    pub resource NFTBurner {

        pub fun burnStake(
            stake: @StarlyTokenStaking.NFT,
            k: UFix64,
            address: Address?,
            minStakingSeconds: UFix64,
            unstakingFee: UFix64,
            unstakingFlatFee: UFix64,
            unstakingFeesNotAppliedAfterSeconds: UFix64,
            unstakingDisabledUntilTimestamp: UFix64): @StarlyToken.Vault {

            pre {
                StarlyTokenStaking.unstakingEnabled: "Unstaking is disabled"
                k <= CompoundInterest.k2000: "K cannot be larger than 2000% APY"
                stake.stakeTimestamp < getCurrentBlock().timestamp: "Cannot unstake stake with stakeTimestamp more or equal to current timestamp"
            }

            let timestamp = getCurrentBlock().timestamp
            if (timestamp < unstakingDisabledUntilTimestamp) {
                panic("Unstaking is disabled at the moment")
            }
            let seconds = timestamp - stake.stakeTimestamp
            if (seconds < minStakingSeconds || seconds < stake.minStakingSeconds) {
                panic("Staking period is too short")
            }

            let unstakingFees = stake.getUnstakingFeesInternal(
                unstakingFee: unstakingFee,
                unstakingFlatFee: unstakingFlatFee,
                unstakingFeesNotAppliedAfterSeconds: unstakingFeesNotAppliedAfterSeconds,
            )
            let principalAmount = stake.principalVault.balance
            let vault <- stake.principalVault.withdraw(amount: principalAmount) as! @StarlyToken.Vault
            let compoundInterest = CompoundInterest.generatedCompoundInterest(seconds: seconds, k: k)
            let interestAmount = principalAmount * compoundInterest - principalAmount

            let interestVaultRef = StarlyTokenStaking.account.borrow<&StarlyToken.Vault>(from: StarlyToken.TokenStoragePath)!
            if (interestAmount > unstakingFees) {
                let interestAmountMinusFees = interestAmount - unstakingFees
                vault.deposit(from: <-interestVaultRef.withdraw(amount: interestAmountMinusFees))
                StarlyTokenStaking.totalInterestPaid = StarlyTokenStaking.totalInterestPaid + interestAmountMinusFees
            } else {
                // if accumulated interest do not cover unstaking fees, user will pay penalty from principal vault
                let penalty = unstakingFees - interestAmount
                interestVaultRef.deposit(from: <-vault.withdraw(amount: penalty))
            }
            StarlyTokenStaking.totalPrincipalStaked = StarlyTokenStaking.totalPrincipalStaked - principalAmount;
            emit TokensUnstaked(
                id: stake.id,
                address: address,
                amount: vault.balance,
                principal: principalAmount,
                interest: interestAmount,
                unstakingFees: unstakingFees,
                stakeTimestamp: stake.stakeTimestamp,
                unstakeTimestamp: timestamp,
                k: k)
            destroy stake
            return <-vault
        }
    }

    pub resource interface CollectionPublic {
        pub fun borrowStakePublic(id: UInt64): &StarlyTokenStaking.NFT{StarlyTokenStaking.StakePublic, NonFungibleToken.INFT}

        // admin has to have the ability to refund stake
        access(contract) fun refund(id: UInt64, k: UFix64)
    }

    pub resource interface CollectionPrivate {
        pub fun borrowStakePrivate(id: UInt64): &StarlyTokenStaking.NFT
        pub fun stake(principalVault: @StarlyToken.Vault)
        pub fun unstake(id: UInt64): @StarlyToken.Vault
        pub fun unstakeAll(): @StarlyToken.Vault
    }

    pub resource Collection:
        NonFungibleToken.Provider,
        NonFungibleToken.Receiver,
        NonFungibleToken.CollectionPublic,
        CollectionPublic,
        CollectionPrivate,
        MetadataViews.ResolverCollection {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        destroy() {
            destroy self.ownedNFTs
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let stake <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: stake.id, from: self.owner?.address)
            return <-stake
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @StarlyTokenStaking.NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let stake = nft as! &StarlyTokenStaking.NFT
            return stake as &AnyResource{MetadataViews.Resolver}
        }

        access(contract) fun refund(id: UInt64, k: UFix64) {
            if let address = self.owner?.address {
                let receiverRef = getAccount(address).getCapability(StarlyToken.TokenPublicReceiverPath).borrow<&{FungibleToken.Receiver}>()
                    ?? panic("Could not borrow StarlyToken receiver reference to the recipient's vault!")
                let stake <- self.withdraw(withdrawID: id) as! @StarlyTokenStaking.NFT
                let burner = StarlyTokenStaking.account.borrow<&NFTBurner>(from: StarlyTokenStaking.BurnerStoragePath)!
                let unstakeVault <-burner.burnStake(
                    stake: <-stake,
                    k: k,
                    address: address,
                    minStakingSeconds: StarlyTokenStaking.minStakingSeconds,
                    unstakingFee: StarlyTokenStaking.unstakingFee,
                    unstakingFlatFee: StarlyTokenStaking.unstakingFlatFee,
                    unstakingFeesNotAppliedAfterSeconds: StarlyTokenStaking.unstakingFeesNotAppliedAfterSeconds,
                    unstakingDisabledUntilTimestamp: StarlyTokenStaking.unstakingDisabledUntilTimestamp)
                receiverRef.deposit(from: <-unstakeVault)
            }
        }

        pub fun borrowStakePublic(id: UInt64): &StarlyTokenStaking.NFT{StarlyTokenStaking.StakePublic, NonFungibleToken.INFT} {
            let stakeRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let intermediateRef = stakeRef as! auth &StarlyTokenStaking.NFT
            return intermediateRef as &StarlyTokenStaking.NFT{StarlyTokenStaking.StakePublic, NonFungibleToken.INFT}
        }

        pub fun stake(principalVault: @StarlyToken.Vault) {
            let minter = StarlyTokenStaking.account.borrow<&NFTMinter>(from: StarlyTokenStaking.MinterStoragePath)!
            let stake <- minter.mintStake(
                address: self.owner?.address,
                principalVault: <-principalVault,
                stakeTimestamp: getCurrentBlock().timestamp,
                minStakingSeconds: StarlyTokenStaking.minStakingSeconds,
                k: StarlyTokenStaking.k)
            self.deposit(token: <-stake)
        }

        pub fun unstake(id: UInt64): @StarlyToken.Vault {
            let burner = StarlyTokenStaking.account.borrow<&NFTBurner>(from: StarlyTokenStaking.BurnerStoragePath)!
            let stake <- self.withdraw(withdrawID: id) as! @StarlyTokenStaking.NFT
            let k = stake.k
            return <-burner.burnStake(
                stake: <-stake,
                k: k,
                address: self.owner?.address,
                minStakingSeconds: StarlyTokenStaking.minStakingSeconds,
                unstakingFee: StarlyTokenStaking.unstakingFee,
                unstakingFlatFee: StarlyTokenStaking.unstakingFlatFee,
                unstakingFeesNotAppliedAfterSeconds: StarlyTokenStaking.unstakingFeesNotAppliedAfterSeconds,
                unstakingDisabledUntilTimestamp: StarlyTokenStaking.unstakingDisabledUntilTimestamp)
        }

        pub fun unstakeAll(): @StarlyToken.Vault {
            let burner = StarlyTokenStaking.account.borrow<&NFTBurner>(from: StarlyTokenStaking.BurnerStoragePath)
            let unstakeVault <- StarlyToken.createEmptyVault() as! @StarlyToken.Vault
            let stakeIDs = self.getIDs()
            for stakeID in stakeIDs {
                unstakeVault.deposit(from: <-self.unstake(id: stakeID))
            }
            return <-unstakeVault
        }

        pub fun borrowStakePrivate(id: UInt64): &StarlyTokenStaking.NFT {
            let stakePassRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return stakePassRef as! &StarlyTokenStaking.NFT
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Admin resource for controlling the configuration parameters and refunding
    pub resource Admin {

        pub fun setStakingEnabled(_ enabled: Bool) {
            StarlyTokenStaking.stakingEnabled = enabled
        }

        pub fun setUnstakingEnabled(_ enabled: Bool) {
            StarlyTokenStaking.unstakingEnabled = enabled
        }

        pub fun setUnstakingFee(_ amount: UFix64) {
            StarlyTokenStaking.unstakingFee = amount
        }

        pub fun setUnstakingFlatFee(_ amount: UFix64) {
            StarlyTokenStaking.unstakingFlatFee = amount
        }

        pub fun setUnstakingFeesNotAppliedAfterSeconds(_ seconds: UFix64) {
            StarlyTokenStaking.unstakingFeesNotAppliedAfterSeconds = seconds
        }

        pub fun setMinStakingSeconds(_ seconds: UFix64) {
            StarlyTokenStaking.minStakingSeconds = seconds
        }

        pub fun setMinStakePrincipal(_ amount: UFix64) {
            StarlyTokenStaking.minStakePrincipal = amount
        }

        pub fun setUnstakingDisabledUntilTimestamp(_ timestamp: UFix64) {
            StarlyTokenStaking.unstakingDisabledUntilTimestamp = timestamp
        }

        pub fun setK(_ k: UFix64) {
            pre {
                k <= CompoundInterest.k200: "Global K cannot be large larger than 200% APY"
            }
            StarlyTokenStaking.k = k
        }

        pub fun refund(collection: &{StarlyTokenStaking.CollectionPublic}, id: UInt64, k: UFix64) {
            collection.refund(id: id, k: k)
        }

        pub fun createNFTMinter(): @NFTMinter {
            return <-create NFTMinter()
        }

        pub fun createNFTBurner(): @NFTBurner {
            return <-create NFTBurner()
        }
    }

    init() {
        self.totalSupply = 0
        self.totalPrincipalStaked = 0.0
        self.totalInterestPaid = 0.0

        self.stakingEnabled = true
        self.unstakingEnabled = true
        self.unstakingFee = 0.0
        self.unstakingFlatFee = 0.0
        self.unstakingFeesNotAppliedAfterSeconds = 0.0
        self.minStakingSeconds = 0.0
        self.minStakePrincipal = 0.0
        self.unstakingDisabledUntilTimestamp = 0.0
        self.k = CompoundInterest.k15 // 15% APY for Starly

        self.CollectionStoragePath = /storage/starlyTokenStakingCollection
        self.CollectionPublicPath = /public/starlyTokenStakingCollection
        self.AdminStoragePath = /storage/starlyTokenStakingAdmin
        self.MinterStoragePath = /storage/starlyTokenStakingMinter
        self.BurnerStoragePath = /storage/starlyTokenStakingBurner

        let admin <- create Admin()
        let minter <- admin.createNFTMinter()
        let burner <- admin.createNFTBurner()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.save(<-minter, to: self.MinterStoragePath)
        self.account.save(<-burner, to: self.BurnerStoragePath)

        // for interests we will use account's default Starly token vault
        if (self.account.borrow<&StarlyToken.Vault>(from: StarlyToken.TokenStoragePath) == nil) {
            self.account.save(<-StarlyToken.createEmptyVault(), to: StarlyToken.TokenStoragePath)
            self.account.link<&StarlyToken.Vault{FungibleToken.Receiver}>(
                StarlyToken.TokenPublicReceiverPath,
                target: StarlyToken.TokenStoragePath)
            self.account.link<&StarlyToken.Vault{FungibleToken.Balance}>(
                StarlyToken.TokenPublicBalancePath,
                target: StarlyToken.TokenStoragePath)
        }

        emit ContractInitialized()
    }
}

import FungibleToken from 0xf233dcee88fe0abe
import MetadataViews from 0x1d7e57aa55817448
import NonFungibleToken from 0x1d7e57aa55817448
import StarlyToken from 0x142fa6570b62fd97

pub contract StarlyTokenVesting: NonFungibleToken {

    pub event TokensVested(
        id: UInt64,
        beneficiary: Address,
        amount: UFix64)
    pub event TokensReleased(
        id: UInt64,
        beneficiary: Address,
        amount: UFix64,
        remainingAmount: UFix64)
    pub event VestingBurned(
        id: UInt64,
        beneficiary: Address,
        amount: UFix64)
    pub event TokensBurned(
        id: UInt64,
        amount: UFix64)
    pub event Withdraw(
        id: UInt64,
        from: Address?)
    pub event Deposit(
        id: UInt64,
        to: Address?)
    pub event VestingInitialized(beneficiary: Address)
    pub event ContractInitialized()

    pub var totalSupply: UInt64
    pub var totalVested: UFix64
    pub var totalReleased: UFix64

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let MinterStoragePath: StoragePath
    pub let BurnerStoragePath: StoragePath

    pub resource interface VestingPublic {
        pub fun getBeneficiary(): Address
        pub fun getInitialVestedAmount(): UFix64
        pub fun getVestedAmount(): UFix64
        pub fun getVestingSchedule(): &AnyResource{StarlyTokenVesting.IVestingSchedule}
        pub fun getReleasableAmount(): UFix64
    }

    pub enum VestingType: UInt8 {
        pub case linear
        pub case period
    }

    pub resource interface IVestingSchedule {
        pub fun getReleasePercent(): UFix64
        pub fun getStartTimestamp(): UFix64
        pub fun getEndTimestamp(): UFix64
        pub fun getNextUnlock(): {UFix64: UFix64}
        pub fun getVestingType(): VestingType
        pub fun toString(): String
    }

    pub resource LinearVestingSchedule: IVestingSchedule {
        pub let startTimestamp: UFix64
        pub let endTimestamp: UFix64

        init(startTimestamp: UFix64, endTimestamp: UFix64) {
            pre {
                endTimestamp > startTimestamp: "endTimestamp cannot be less than startTimestamp"
            }
            self.startTimestamp = startTimestamp
            self.endTimestamp = endTimestamp
        }

        pub fun getReleasePercent(): UFix64 {
            let timestamp = getCurrentBlock().timestamp
            if timestamp >= self.endTimestamp {
                return 1.0
            } else if timestamp < self.startTimestamp {
                return 0.0
            } else {
                let duration = self.endTimestamp - self.startTimestamp
                let progress = timestamp - self.startTimestamp
                return progress / duration
            }
        }

        pub fun getStartTimestamp(): UFix64 {
            return self.startTimestamp
        }

        pub fun getEndTimestamp(): UFix64 {
            return self.endTimestamp
        }

        pub fun getVestingType(): VestingType {
            return VestingType.linear
        }

        pub fun getNextUnlock(): {UFix64: UFix64} {
            return {getCurrentBlock().timestamp: self.getReleasePercent()}
        }

        pub fun toString(): String {
            return "LinearVestingSchedule(startTimestamp: ".concat(self.startTimestamp.toString())
                .concat(", endTimestamp: ").concat(self.endTimestamp.toString())
                .concat(", releasePercent: ").concat(self.getReleasePercent().toString())
                .concat(")")
        }
    }

    pub resource PeriodVestingSchedule: IVestingSchedule {
        pub let startTimestamp: UFix64
        pub let endTimestamp: UFix64
        access(self) let schedule: {UFix64: UFix64}

        init(schedule: {UFix64: UFix64}) {
            self.schedule = schedule
            let keys = self.schedule.keys
            var startTimestamp = 0.0
            var endTimestamp = 0.0

            for key in keys {
                if self.schedule[key]! == 0.0 {
                    startTimestamp = key
                }
                if self.schedule[key]! == 1.0 {
                    endTimestamp = key
                }
            }
            self.startTimestamp = startTimestamp
            self.endTimestamp = endTimestamp
        }

        pub fun getReleasePercent(): UFix64 {
            let timestamp = getCurrentBlock().timestamp
            let keys = self.schedule.keys
            var closestTimestamp = 0.0
            var releasePercent = 0.0

            for key in keys {
                if timestamp >= key && key >= closestTimestamp {
                    releasePercent = self.schedule[key]!
                    closestTimestamp = key
                }
            }

            return releasePercent
        }

        pub fun getStartTimestamp(): UFix64 {
            return self.startTimestamp
        }

        pub fun getEndTimestamp(): UFix64 {
            return self.endTimestamp
        }

        pub fun getVestingType(): VestingType {
            return VestingType.period
        }

        pub fun getNextUnlock(): {UFix64: UFix64} {
            let timestamp = getCurrentBlock().timestamp
            let keys = self.schedule.keys
            var closestTimestamp = self.endTimestamp
            var releasePercent = 1.0

            for key in keys {
                if timestamp <= key && key <= closestTimestamp {
                    releasePercent = self.schedule[key]!
                    closestTimestamp = key
                }
            }
            return {closestTimestamp: releasePercent}
        }

        pub fun toString(): String {
            return "PeriodVestingSchedule(releasePercent: ".concat(self.getReleasePercent().toString())
                .concat(")")
        }
    }

    pub struct VestingMetadataView {
        pub let id: UInt64
        pub let beneficiary: Address
        pub let initialVestedAmount: UFix64
        pub let remainingVestedAmount: UFix64
        pub let vestingType: VestingType
        pub let startTimestamp: UFix64
        pub let endTimestamp: UFix64
        pub let nextUnlock: {UFix64: UFix64}
        pub let releasePercent: UFix64
        pub let releasableAmount: UFix64

        init(
            id: UInt64,
            beneficiary: Address,
            initialVestedAmount: UFix64,
            remainingVestedAmount: UFix64,
            vestingType: VestingType,
            startTimestamp: UFix64,
            endTimestamp: UFix64,
            nextUnlock: {UFix64: UFix64},
            releasePercent: UFix64,
            releasableAmount: UFix64) {
            self.id = id
            self.beneficiary = beneficiary
            self.initialVestedAmount = initialVestedAmount
            self.remainingVestedAmount = remainingVestedAmount
            self.vestingType = vestingType
            self.startTimestamp = startTimestamp
            self.endTimestamp = endTimestamp
            self.nextUnlock = nextUnlock
            self.releasePercent = releasePercent
            self.releasableAmount = releasableAmount
        }
    }

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver, VestingPublic {
        pub let id: UInt64
        pub let beneficiary: Address
        pub let initialVestedAmount: UFix64
        access(contract) let vestedVault: @StarlyToken.Vault
        access(contract) let vestingSchedule: @AnyResource{StarlyTokenVesting.IVestingSchedule}

        init(
            id: UInt64,
            beneficiary: Address,
            vestedVault: @StarlyToken.Vault,
            vestingSchedule: @AnyResource{StarlyTokenVesting.IVestingSchedule}) {
            self.id = id
            self.beneficiary = beneficiary
            self.initialVestedAmount = vestedVault.balance
            self.vestedVault <-vestedVault
            self.vestingSchedule <-vestingSchedule
        }

        destroy() {
            let vestedAmount = self.vestedVault.balance
            destroy self.vestedVault
            destroy self.vestingSchedule
            if (vestedAmount > 0.0) {
                StarlyTokenVesting.totalVested = StarlyTokenVesting.totalVested - vestedAmount
                emit TokensBurned(id: self.id, amount: vestedAmount)
            }
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<VestingMetadataView>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: "StarlyTokenVesting #".concat(self.id.toString()),
                        description: "id: ".concat(self.id.toString())
                            .concat(", beneficiary: ").concat(self.beneficiary.toString())
                            .concat(", vestedAmount: ").concat(self.vestedVault.balance.toString())
                            .concat(", vestingSchedule: ").concat(self.vestingSchedule.toString()),
                        thumbnail: MetadataViews.HTTPFile(url: ""))
                case Type<VestingMetadataView>():
                    return VestingMetadataView(
                        id: self.id,
                        beneficiary: self.getBeneficiary(),
                        initialVestedAmount: self.getInitialVestedAmount(),
                        remainingVestedAmount: self.vestedVault.balance,
                        vestingType: self.vestingSchedule.getVestingType(),
                        startTimestamp: self.vestingSchedule.getStartTimestamp(),
                        endTimestamp: self.vestingSchedule.getEndTimestamp(),
                        nextUnlock: self.vestingSchedule.getNextUnlock(),
                        releasePercent: self.vestingSchedule.getReleasePercent(),
                        releasableAmount: self.getReleasableAmount())
            }
            return nil
        }

        pub fun getBeneficiary(): Address {
            return self.beneficiary
        }

        pub fun getInitialVestedAmount(): UFix64 {
            return self.initialVestedAmount
        }

        pub fun getVestedAmount(): UFix64 {
            return self.vestedVault.balance
        }

        pub fun getVestingSchedule(): &AnyResource{StarlyTokenVesting.IVestingSchedule} {
            return &self.vestingSchedule as &AnyResource{StarlyTokenVesting.IVestingSchedule}
        }

        pub fun getReleasableAmount(): UFix64 {
            let initialAmount = self.initialVestedAmount
            let currentAmount = self.vestedVault.balance
            let alreadyReleasedAmount = initialAmount - currentAmount
            let releasePercent = self.vestingSchedule.getReleasePercent()
            let releasableAmount = (initialAmount * releasePercent) - alreadyReleasedAmount
            return releasableAmount
        }
    }

    // We put vesting creation logic into minter, its job is to have checks, emit events, update counters
    pub resource NFTMinter {

        pub fun mintLinear(
            beneficiary: Address,
            vestedVault: @StarlyToken.Vault,
            startTimestamp: UFix64,
            endTimestamp: UFix64): @StarlyTokenVesting.NFT {

            let vestingSchedule <- create LinearVestingSchedule(
                startTimestamp: startTimestamp,
                endTimestamp: endTimestamp)

            return <-self.mintInternal(
                beneficiary: beneficiary,
                vestedVault: <-vestedVault,
                vestingSchedule: <-vestingSchedule)
        }

        pub fun mintPeriod(
            beneficiary: Address,
            vestedVault: @StarlyToken.Vault,
            schedule: {UFix64: UFix64}): @StarlyTokenVesting.NFT {

            let vestingSchedule <- create PeriodVestingSchedule(schedule: schedule)

            return <-self.mintInternal(
                beneficiary: beneficiary,
                vestedVault: <-vestedVault,
                vestingSchedule: <-vestingSchedule)
        }

        access(self) fun mintInternal(
            beneficiary: Address,
            vestedVault: @StarlyToken.Vault,
            vestingSchedule: @AnyResource{StarlyTokenVesting.IVestingSchedule}): @StarlyTokenVesting.NFT {

            pre {
                vestedVault.balance > 0.0: "vestedVault balance cannot be zero"
            }

            let vesting <- create NFT(
                id: StarlyTokenVesting.totalSupply,
                beneficiary: beneficiary,
                vestedVault: <-vestedVault,
                vestingSchedule: <-vestingSchedule)
            let vestedAmount = vesting.vestedVault.balance
            StarlyTokenVesting.totalSupply = StarlyTokenVesting.totalSupply + (1 as UInt64)
            StarlyTokenVesting.totalVested = StarlyTokenVesting.totalVested + vestedAmount
            emit TokensVested(
                id: vesting.id,
                beneficiary: beneficiary,
                amount: vestedAmount)
            return <-vesting
        }
    }

    // We put releasing logic into burner, its job is to have checks, emit events, update counters
    pub resource NFTBurner {

        // if admin owns the vesting NFT we can burn it to release tokens
        pub fun burn(vesting: @StarlyTokenVesting.NFT) {
            let vestedAmount = vesting.vestedVault.balance
            let returnVaultRef = StarlyTokenVesting.account.borrow<&StarlyToken.Vault>(from: StarlyToken.TokenStoragePath)!
            returnVaultRef.deposit(from: <-vesting.vestedVault.withdraw(amount: vestedAmount))
            StarlyTokenVesting.totalVested = StarlyTokenVesting.totalVested - vestedAmount
            emit VestingBurned(
                id: vesting.id,
                beneficiary: vesting.beneficiary,
                amount: vestedAmount)
            destroy vesting
        }

        // user can only release tokens
        pub fun release(vestingRef: &StarlyTokenVesting.NFT) {
            let receiverRef = getAccount(vestingRef.beneficiary).getCapability(StarlyToken.TokenPublicReceiverPath).borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow StarlyToken receiver reference to the beneficiary's vault!")
            let releaseAmount = vestingRef.getReleasableAmount()
            receiverRef.deposit(from: <-vestingRef.vestedVault.withdraw(amount: releaseAmount))
            StarlyTokenVesting.totalVested = StarlyTokenVesting.totalVested - releaseAmount
            StarlyTokenVesting.totalReleased = StarlyTokenVesting.totalReleased + releaseAmount
            emit TokensReleased(
                id: vestingRef.id,
                beneficiary: vestingRef.beneficiary,
                amount: releaseAmount,
                remainingAmount: vestingRef.vestedVault.balance)
        }
    }

    pub resource interface CollectionPublic {
        pub fun borrowVestingPublic(id: UInt64): &StarlyTokenVesting.NFT{StarlyTokenVesting.VestingPublic, NonFungibleToken.INFT}
        pub fun getIDs(): [UInt64]
    }

    pub resource interface CollectionPrivate {
        pub fun borrowVestingPrivate(id: UInt64): &StarlyTokenVesting.NFT
        pub fun release(id: UInt64)
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
            let vesting <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: vesting.id, from: self.owner?.address)
            return <-vesting
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @StarlyTokenVesting.NFT
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
            let vesting = nft as! &StarlyTokenVesting.NFT
            return vesting as &AnyResource{MetadataViews.Resolver}
        }

        pub fun borrowVestingPublic(id: UInt64): &StarlyTokenVesting.NFT{StarlyTokenVesting.VestingPublic, NonFungibleToken.INFT} {
            let vestingRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let intermediateRef = vestingRef as! auth &StarlyTokenVesting.NFT
            return intermediateRef as &StarlyTokenVesting.NFT{StarlyTokenVesting.VestingPublic, NonFungibleToken.INFT}
        }

        pub fun release(id: UInt64) {
            let burner = StarlyTokenVesting.account.borrow<&NFTBurner>(from: StarlyTokenVesting.BurnerStoragePath)!
            let vestingRef = self.borrowVestingPrivate(id: id)
            return burner.release(vestingRef: vestingRef)
        }

        pub fun borrowVestingPrivate(id: UInt64): &StarlyTokenVesting.NFT {
            let vestingRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return vestingRef as! &StarlyTokenVesting.NFT
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub fun createEmptyCollectionAndNotify(beneficiary: Address): @NonFungibleToken.Collection {
        emit VestingInitialized(beneficiary: beneficiary)
        return <- self.createEmptyCollection()
    }

    pub resource Admin {

        pub fun createNFTMinter(): @NFTMinter {
            return <-create NFTMinter()
        }

        pub fun createNFTBurner(): @NFTBurner {
            return <-create NFTBurner()
        }
    }

    init() {
        self.totalSupply = 0
        self.totalVested = 0.0
        self.totalReleased = 0.0

        self.CollectionStoragePath = /storage/starlyTokenVestingCollection
        self.CollectionPublicPath = /public/starlyTokenVestingCollection
        self.AdminStoragePath = /storage/starlyTokenVestingAdmin
        self.MinterStoragePath = /storage/starlyTokenVestingMinter
        self.BurnerStoragePath = /storage/starlyTokenVestingBurner

        let admin <- create Admin()
        let minter <- admin.createNFTMinter()
        let burner <- admin.createNFTBurner()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.save(<-minter, to: self.MinterStoragePath)
        self.account.save(<-burner, to: self.BurnerStoragePath)

        // we will use account's default Starly token vault
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

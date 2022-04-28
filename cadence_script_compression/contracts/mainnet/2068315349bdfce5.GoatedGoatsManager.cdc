/*
    A contract that manages the creation and sale of Goated Goats, Traits, and Packs.

    A manager resource exists to allow modifications to the parameters of the public
    sale and have ability to mint editions themself.
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import GoatedGoatsTrait from 0x2068315349bdfce5
import GoatedGoatsVouchers from 0xdfc74d9d561374c0
import TraitPacksVouchers from 0xdfc74d9d561374c0
import GoatedGoatsTraitPack from 0x2068315349bdfce5
import GoatedGoats from 0x2068315349bdfce5

pub contract GoatedGoatsManager {
    // -----------------------------------------------------------------------
    //  Events
    // -----------------------------------------------------------------------
    // Emitted when the contract is initialized
    pub event ContractInitialized()

    // -----------------------------------------------------------------------
    //  Trait
    // -----------------------------------------------------------------------
    // Emitted when an admin has initiated a mint of a GoatedGoat NFT
    pub event AdminMintTrait(id: UInt64)
    // Emitted when a GoatedGoatsTrait collection has had metadata updated
    pub event UpdateTraitCollectionMetadata()
    // Emitted when an edition within a GoatedGoatsTrait has had it's metadata updated
    pub event UpdateTraitEditionMetadata(id: UInt64)

    // -----------------------------------------------------------------------
    //  TraitPack
    // -----------------------------------------------------------------------
    // Emitted when an admin has initiated a mint of a GoatedGoat NFT
    pub event AdminMintTraitPack(id: UInt64)
    // Emitted when someone has redeemed a voucher for a trait pack
    pub event RedeemTraitPackVoucher(id: UInt64)
    // Emitted when someone has redeemed a trait pack for traits
    pub event RedeemTraitPack(id: UInt64, packID: UInt64, packEditionID: UInt64, address: Address)
    // Emitted when a GoatedGoatsTrait collection has had metadata updated
    pub event UpdateTraitPackCollectionMetadata()
    // Emitted when an edition within a GoatedGoatsTrait has had it's metadata updated
    pub event UpdateTraitPackEditionMetadata(id: UInt64)
    // Emitted when any info about redeem logistics has been modified
    pub event UpdateTraitPackRedeemInfo(redeemStartTime: UFix64)
    // -----------------------------------------------------------------------
    //  Goat
    // -----------------------------------------------------------------------
    // Emitted when someone has redeemed a voucher for a goat
    pub event RedeemGoatVoucher(id: UInt64, goatID: UInt64, address: Address)
    // Emitted whenever a goat trait action is done, e.g. equip/unequip
    pub event UpdateGoatTraits(id: UInt64, goatID: UInt64, address: Address)
    // Emitted when a GoatedGoats collection has had metadata updated
    pub event UpdateGoatCollectionMetadata()
    // Emitted when an edition within a GoatedGoats has had it's metadata updated
    pub event UpdateGoatEditionMetadata(id: UInt64)
    // Emitted when any info about redeem logistics has been modified
    pub event UpdateGoatRedeemInfo(redeemStartTime: UFix64)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------
    pub let ManagerStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // GoatedGoatsManager fields
    // -----------------------------------------------------------------------
    // -----------------------------------------------------------------------
    //  Trait
    // -----------------------------------------------------------------------
    access(self) let traitsMintedEditions: {UInt64: Bool}
    access(self) var traitsSequentialMintMin: UInt64
    pub var traitTotalSupply: UInt64
    // -----------------------------------------------------------------------
    //  TraitPack
    // -----------------------------------------------------------------------
    access(self) let traitPacksMintedEditions: {UInt64: Bool}
    access(self) let traitPacksByPackIdMintedEditions: {UInt64: {UInt64: Bool}}
    access(self) var traitPacksSequentialMintMin: UInt64
    access(self) var traitPacksByPackIdSequentialMintMin: {UInt64: UInt64}
    pub var traitPackTotalSupply: UInt64
    pub var traitPackRedeemStartTime: UFix64
    // -----------------------------------------------------------------------
    //  Goat
    // -----------------------------------------------------------------------
    access(self) let goatsMintedEditions: {UInt64: Bool}
    access(self) let goatsByGoatIdMintedEditions: {UInt64: Bool}
    access(self) var goatsSequentialMintMin: UInt64
    pub var goatMaxSupply: UInt64
    pub var goatTotalSupply: UInt64
    pub var goatRedeemStartTime: UFix64

    // -----------------------------------------------------------------------
    // Manager resource for all NFTs
    // -----------------------------------------------------------------------
    pub resource Manager {
        // -----------------------------------------------------------------------
        //  Trait
        // -----------------------------------------------------------------------
        pub fun updateTraitCollectionMetadata(metadata: {String: String}) {
            GoatedGoatsTrait.setCollectionMetadata(metadata: metadata)
            emit UpdateTraitCollectionMetadata()
        }
        
        pub fun updateTraitEditionMetadata(editionNumber: UInt64, metadata: {String: String}) {
            GoatedGoatsTrait.setEditionMetadata(editionNumber: editionNumber, metadata: metadata)
            emit UpdateTraitEditionMetadata(id: editionNumber)
        }

        pub fun mintTraitAtEdition(edition: UInt64, packID: UInt64): @NonFungibleToken.NFT {
            emit AdminMintTrait(id: edition)
            return <-GoatedGoatsManager.mintTrait(edition: edition, packID: packID)
        }

        pub fun mintSequentialTrait(packID: UInt64): @NonFungibleToken.NFT {
            let trait <- GoatedGoatsManager.mintSequentialTrait(packID: packID)
            emit AdminMintTrait(id: trait.id)
            return <- trait
        }

        // -----------------------------------------------------------------------
        //  TraitPack
        // -----------------------------------------------------------------------
        pub fun updateTraitPackCollectionMetadata(metadata: {String: String}) {
            GoatedGoatsTraitPack.setCollectionMetadata(metadata: metadata)
            emit UpdateTraitPackCollectionMetadata()
        }
        
        pub fun updateTraitPackEditionMetadata(editionNumber: UInt64, metadata: {String: String}) {
            GoatedGoatsTraitPack.setEditionMetadata(editionNumber: editionNumber, metadata: metadata)
            emit UpdateTraitPackEditionMetadata(id: editionNumber)
        }

        pub fun mintTraitPackAtEdition(edition: UInt64, packID: UInt64, packEditionID: UInt64): @NonFungibleToken.NFT {
            emit AdminMintTraitPack(id: edition)
            return <-GoatedGoatsManager.mintTraitPack(edition: edition, packID: packID, packEditionID: packEditionID)
        }

        pub fun mintSequentialTraitPack(packID: UInt64): @NonFungibleToken.NFT {
            let trait <- GoatedGoatsManager.mintSequentialTraitPack(packID: packID)
            emit AdminMintTraitPack(id: trait.id)
            return <- trait
        }

        pub fun updateTraitPackRedeemStartTime(_ redeemStartTime: UFix64) {
            GoatedGoatsManager.traitPackRedeemStartTime = redeemStartTime
            emit UpdateTraitPackRedeemInfo(
                redeemStartTime: GoatedGoatsManager.traitPackRedeemStartTime,
            )
        }

        // -----------------------------------------------------------------------
        //  Goat
        // -----------------------------------------------------------------------
        pub fun updateGoatCollectionMetadata(metadata: {String: String}) {
            GoatedGoats.setCollectionMetadata(metadata: metadata)
            emit UpdateGoatCollectionMetadata()
        }
        
        pub fun updateGoatEditionMetadata(goatID: UInt64, metadata: {String: String}, traitSlots: UInt8) {
            GoatedGoats.setEditionMetadata(goatID: goatID, metadata: metadata, traitSlots: traitSlots)
            emit UpdateGoatEditionMetadata(id: goatID)
        }

        pub fun updateGoatRedeemStartTime(_ redeemStartTime: UFix64) {
            GoatedGoatsManager.goatRedeemStartTime = redeemStartTime
            emit UpdateGoatRedeemInfo(
                redeemStartTime: GoatedGoatsManager.goatRedeemStartTime,
            )
        }
    }

    // -----------------------------------------------------------------------
    //  Trait
    // -----------------------------------------------------------------------
    // Mint a GoatedGoatTrait
    access(contract) fun mintTrait(edition: UInt64, packID: UInt64): @NonFungibleToken.NFT {
        pre {
            edition >= 1: "Requested edition is outside of allowed bounds."
            self.traitsMintedEditions[edition] == nil : "Requested edition has already been minted"
        }
        self.traitsMintedEditions[edition] = true
        self.traitTotalSupply = self.traitTotalSupply + 1
        let trait <- GoatedGoatsTrait.mint(nftID: edition, packID: packID)
        return <-trait
    }

    // Look for the next available trait, and mint there
    access(self) fun mintSequentialTrait(packID: UInt64): @NonFungibleToken.NFT {
        var curEditionNumber = self.traitsSequentialMintMin
        while (self.traitsMintedEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        self.traitsSequentialMintMin = curEditionNumber
        let newTrait <- self.mintTrait(edition: UInt64(curEditionNumber), packID: packID)
        return <-newTrait
    }

    // -----------------------------------------------------------------------
    //  TraitPack
    // -----------------------------------------------------------------------
    // Mint a GoatedGoatTraitPack
    access(contract) fun mintTraitPack(edition: UInt64, packID: UInt64, packEditionID: UInt64): @NonFungibleToken.NFT {
        pre {
            edition >= 1: "Requested edition is outside of allowed bounds."
            self.traitPacksMintedEditions[edition] == nil : "Requested edition has already been minted"
            self.traitPacksByPackIdMintedEditions[packID] == nil || 
                self.traitPacksByPackIdMintedEditions[packID]![packEditionID] == nil: "Requested pack edition has already been minted"
        }
        self.traitPacksMintedEditions[edition] = true
        // Setup packID if doesn't exist.
        if self.traitPacksByPackIdMintedEditions[packID] == nil {
            self.traitPacksByPackIdMintedEditions[packID] = {}
        }
        // Set packEditionID status
        let ref = self.traitPacksByPackIdMintedEditions[packID]!
        ref[packEditionID] = true
        self.traitPacksByPackIdMintedEditions[packID] = ref

        self.traitPackTotalSupply = self.traitPackTotalSupply + 1
        let trait <- GoatedGoatsTraitPack.mint(nftID: edition, packID: packID, packEditionID: packEditionID)
        return <-trait
    }

    // Look for the next available trait pack, and mint there
    access(self) fun mintSequentialTraitPack(packID: UInt64): @NonFungibleToken.NFT {
        // Grab the resource ID aka editionID
        var curEditionNumber = self.traitPacksSequentialMintMin
        while (self.traitPacksMintedEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        self.traitPacksSequentialMintMin = curEditionNumber
        
        // Setup sequential ID for new packs
        if self.traitPacksByPackIdSequentialMintMin[packID] == nil {
            self.traitPacksByPackIdSequentialMintMin[packID] = 1
        }
        // Grab the packEditionID
        var curPackEditionNumber = self.traitPacksByPackIdSequentialMintMin[packID]!
        while (self.traitPacksByPackIdMintedEditions[packID]!.containsKey(UInt64(curPackEditionNumber))) {
            curPackEditionNumber = curPackEditionNumber + 1
        }
        self.traitPacksByPackIdSequentialMintMin[packID] = curPackEditionNumber
        let newTrait <- self.mintTraitPack(edition: UInt64(curEditionNumber), packID: packID, packEditionID: UInt64(curPackEditionNumber))
        return <-newTrait
    }

    // -----------------------------------------------------------------------
    //  Goat
    // -----------------------------------------------------------------------
    // Mint a GoatedGoat
    access(contract) fun mintGoat(edition: UInt64, goatID: UInt64, traitActions: UInt64, goatCreationDate: UFix64, lastTraitActionDate: UFix64): @NonFungibleToken.NFT {
        pre {
            edition >= 1: "Requested edition is outside of allowed bounds."
            goatID >= 1 && goatID <= self.goatMaxSupply: "Requested goat ID is outside of allowed bounds."
            self.goatsMintedEditions[edition] == nil : "Requested edition has already been minted"
            self.goatsByGoatIdMintedEditions[goatID] == nil : "Requested goat ID has already been minted"
        }
        self.goatsMintedEditions[edition] = true
        self.goatsByGoatIdMintedEditions[goatID] = true
        self.goatTotalSupply = self.goatTotalSupply + 1
        let goat <- GoatedGoats.mint(nftID: edition, goatID: goatID, traitActions: traitActions, goatCreationDate: goatCreationDate, lastTraitActionDate: lastTraitActionDate)
        return <-goat
    }

    // Look for the next available goat, and mint there
    access(self) fun mintSequentialGoat(goatID: UInt64, traitActions: UInt64, goatCreationDate: UFix64, lastTraitActionDate: UFix64): @NonFungibleToken.NFT {
        // Grab the resource ID aka editionID
        var curEditionNumber = self.goatsSequentialMintMin
        while (self.goatsMintedEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        self.goatsSequentialMintMin = curEditionNumber
        
        let goat <- self.mintGoat(edition: UInt64(curEditionNumber), goatID: goatID, traitActions: traitActions, goatCreationDate: goatCreationDate, lastTraitActionDate: lastTraitActionDate)
        return <-goat
    }

    // -----------------------------------------------------------------------
    // Public Functions
    // -----------------------------------------------------------------------
    // -----------------------------------------------------------------------
    //  TraitPack
    // -----------------------------------------------------------------------
    pub fun publicRedeemTraitPackWithVoucher(traitPackVoucher: @NonFungibleToken.NFT): @NonFungibleToken.Collection {
        pre {
            getCurrentBlock().timestamp >= self.traitPackRedeemStartTime: "Redemption has not yet started"
            traitPackVoucher.isInstance(Type<@TraitPacksVouchers.NFT>()): "Invalid type provided, expected TraitPacksVoucher.NFT"
        }

        // -- Burn voucher --
        let id = traitPackVoucher.id
        destroy traitPackVoucher

        // -- Mint the trait pack --
        let traitPackCollection <- GoatedGoatsTraitPack.createEmptyCollection()
        // Default To 1 for all Voucher Based Packs.
        let traitPack <- self.mintSequentialTraitPack(packID: 1)

        traitPackCollection.deposit(token: <-traitPack)

        emit RedeemTraitPackVoucher(id: id);

        return <-traitPackCollection
    }

    pub fun publicRedeemTraitPack(traitPack: @NonFungibleToken.NFT, address: Address) {
        pre {
            getCurrentBlock().timestamp >= self.traitPackRedeemStartTime: "Redemption has not yet started"
            traitPack.isInstance(Type<@GoatedGoatsTraitPack.NFT>())
        }
        let traitPackInstance <- traitPack as! @GoatedGoatsTraitPack.NFT

        // Emit an event that our backend will read and mint traits to the associating address.
        emit RedeemTraitPack(id: traitPackInstance.id, packID: traitPackInstance.packID, packEditionID: traitPackInstance.packEditionID, address: address)
        // Burn trait pack
        destroy traitPackInstance
    }

    // -----------------------------------------------------------------------
    //  Goat
    // -----------------------------------------------------------------------
    pub fun publicRedeemGoatWithVoucher(goatVoucher: @NonFungibleToken.NFT, address: Address): @NonFungibleToken.Collection {
        pre {
            getCurrentBlock().timestamp >= self.goatRedeemStartTime: "Redemption has not yet started"
            goatVoucher.isInstance(Type<@GoatedGoatsVouchers.NFT>()): "Invalid type provided, expected GoatedGoatsVouchers.NFT"
        }

        // -- Burn voucher --
        let id = goatVoucher.id
        destroy goatVoucher

        // -- Mint the goat with same Voucher ID --
        let goatCollection <- GoatedGoats.createEmptyCollection()
        // Mint a clean goat with no equipped traits or counters
        let goat <- self.mintSequentialGoat(goatID: id, traitActions: 0, goatCreationDate: getCurrentBlock().timestamp, lastTraitActionDate: 0.0)
        let editionId = goat.id

        goatCollection.deposit(token: <-goat)

        emit RedeemGoatVoucher(id: editionId, goatID: id, address: address);

        return <-goatCollection
    }

    pub resource GoatAndTraits {
        // This is a list with only the goat.
        // Cannot move nested resource out of it otherwise.
        pub var goat: @[NonFungibleToken.NFT]
        pub var unequippedTraits: @[NonFungibleToken.NFT]

        pub fun extractGoat(): @NonFungibleToken.NFT {
            return <-self.goat.removeFirst()
        }

        pub fun extractAllTraits(): @[NonFungibleToken.NFT] {
            var assets: @[NonFungibleToken.NFT] <- []
            self.unequippedTraits <-> assets
            assert(self.unequippedTraits.length == 0, message: "Couldn't extract all goats.")
            return <-assets
        }

        init(goat: @NonFungibleToken.NFT, unequippedTraits: @[NonFungibleToken.NFT]) {
            self.goat <- [<-goat]
            self.unequippedTraits <- unequippedTraits
        }

        destroy() {
            assert(self.goat.length == 0, message: "Cannot destroy with goat.")
            assert(self.unequippedTraits.length == 0, message: "Can notdestroy with traits.")
            destroy self.goat
            destroy self.unequippedTraits
        }
    }

    pub fun updateGoatTraits(goat: @NonFungibleToken.NFT, traitsToEquip: @[NonFungibleToken.NFT], traitSlotsToUnequip: [String], address: Address): @GoatAndTraits {
        pre {
            getCurrentBlock().timestamp >= self.goatRedeemStartTime: "Updating traits on a goat in not enabled."
            goat != nil: "Goat not provided."
            goat.isInstance(Type<@GoatedGoats.NFT>()): "Invalid type provided, expected GoatedGoats.NFT"
            traitsToEquip.length != 0 || traitSlotsToUnequip.length != 0: "Must provide some action to take place."
        }
        // Get Goat typed instance
        let goatInstance <- goat as! @GoatedGoats.NFT
        // Unset the store of this Goat ID
        self.goatsByGoatIdMintedEditions[goatInstance.goatID] = nil
        // Mint a new goat with same Goat ID, traits to store, and counters (updated)
        let newGoat <- self.mintSequentialGoat(goatID: goatInstance.goatID, traitActions: goatInstance.traitActions + 1, goatCreationDate: goatInstance.goatCreationDate, lastTraitActionDate: getCurrentBlock().timestamp)
        let newGoatInstance <- newGoat as! @GoatedGoats.NFT
        let unequippedTraits: @[NonFungibleToken.NFT] <- []

        // Move traits over to the new Goat
        for traitSlot in goatInstance.traits.keys {
            let old <- newGoatInstance.traits[traitSlot] <- goatInstance.traits.remove(key: traitSlot)
            assert(old == nil, message: "Existing trait exists with this trait slot.")
            destroy old
        }
        // Destroy the old goat
        destroy goatInstance

        // First unequip anything provided and store in the return
        if (traitSlotsToUnequip.length > 0) {
            // For each trait ID, validate they exist in store, and return them.
            for traitSlot in traitSlotsToUnequip {
                assert(newGoatInstance.isTraitEquipped(traitSlot: traitSlot), message: "This goat has the provided trait slot empty.")
                let trait <- newGoatInstance.traits.remove(key: traitSlot)!
                assert(trait.getMetadata().containsKey("traitSlot"), message: "Provided trait is missing the trait slot.")
                unequippedTraits.append(<-trait)
            }
            assert(unequippedTraits.length == traitSlotsToUnequip.length, message: "Was not able to unequip all traits.")
        }

        // Equip the traits provided onto the new goat
        // If there are still traits on the goat swap them out and return them.
        if (traitsToEquip.length > 0) {
            while traitsToEquip.length > 0 {
                let trait <- traitsToEquip.removeFirst() as! @GoatedGoatsTrait.NFT
                assert(trait.getMetadata().containsKey("traitSlot"), message: "Provided trait is missing the trait slot.")
                let traitSlot = trait.getMetadata()["traitSlot"]!
                // If goat already has this trait equipped, remove it
                if newGoatInstance.isTraitEquipped(traitSlot: traitSlot) {
                    let existingTrait <- newGoatInstance.traits.remove(key: traitSlot)!
                    unequippedTraits.append(<-existingTrait)
                }
                let old <- newGoatInstance.traits[traitSlot] <- trait
                assert(old == nil, message: "Existing trait exists with this trait slot.")
                destroy old
            }
            assert(traitsToEquip.length == 0, message: "Was not able to equip all traits.")
        }
        destroy traitsToEquip

        // Validate didn't equip too many traits.
        assert(newGoatInstance.traits.length <= Int(newGoatInstance.getTraitSlots()!), message: "Equipped more traits than this goat supports.")

        // Send event to the BE that will update the Goats image.
        emit UpdateGoatTraits(id: newGoatInstance.id, goatID: newGoatInstance.goatID, address: address);

        return <-create GoatAndTraits(goat: <-newGoatInstance, unequippedTraits: <-unequippedTraits)
    }

    init() {
        // Non-human modifiable variables
        // -----------------------------------------------------------------------
        //  Trait
        // -----------------------------------------------------------------------
        self.traitTotalSupply = 0
        self.traitsSequentialMintMin = 1
        // Start with no existing editions minted
        self.traitsMintedEditions = {}
        // -----------------------------------------------------------------------
        //  TraitPack
        // -----------------------------------------------------------------------
        self.traitPackTotalSupply = 0
        self.traitPackRedeemStartTime = 4891048813.0
        self.traitPacksSequentialMintMin = 1
        // Start with no existing editions minted
        self.traitPacksMintedEditions = {}
        // Setup with initial packID.
        self.traitPacksByPackIdMintedEditions = {1: {}}
        self.traitPacksByPackIdSequentialMintMin = {1: 1}
        // -----------------------------------------------------------------------
        //  Goat
        // -----------------------------------------------------------------------
        self.goatTotalSupply = 0
        self.goatMaxSupply = 10000
        self.goatRedeemStartTime = 4891048813.0
        self.goatsSequentialMintMin = 1
        // Start with no existing editions minted
        self.goatsMintedEditions = {}
        self.goatsByGoatIdMintedEditions = {}

        // Manager resource is only saved to the deploying account's storage
        self.ManagerStoragePath = /storage/GoatedGoatsManager
        self.account.save(<- create Manager(), to: self.ManagerStoragePath)

        emit ContractInitialized()
    }
}
 
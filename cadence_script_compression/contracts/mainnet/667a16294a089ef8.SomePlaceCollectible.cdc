/*
    Description: Central Smart Contract for Some Place NFT Collectibles
    
    Some Place Collectibles are available as part of "sets", each with
    a fixed edition count.
*/

import NonFungibleToken from 0x1d7e57aa55817448
import SomePlaceCounter from 0x667a16294a089ef8

pub contract SomePlaceCollectible : NonFungibleToken {

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // SomePlace Events
    // -----------------------------------------------------------------------
    pub event Mint(id: UInt64)
    pub event Burn(id: UInt64)
    pub event SetCreated(setID: UInt64)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // SomePlace Fields
    // -----------------------------------------------------------------------

    // Maintains state of what sets and editions have been minted to ensure
    // there are never 2 of the same set + edition combination
    // Provides a Set ID -> Edition ID -> Data mapping
    access(contract) let collectibleData: {UInt64: {UInt64: CollectibleMetadata}}

    // Allows easy access to information for a set
    // Provides access from a set's setID to the information for that set
    access(contract) let setData: {UInt64: SomePlaceCollectible.SetMetadata}

    // Allows easy access to pointers from an NFT to its metadata keys
    // Provides CollectibleID -> (SetID + EditionID) mapping
    access(contract) let allCollectibleIDs: {UInt64: CollectibleEditionData}

    // -----------------------------------------------------------------------
    // SomePlace Structs
    // -----------------------------------------------------------------------
    pub struct CollectibleMetadata {
        // The NFT Id is optional so a collectible may have associated metadata prior to being minted
        // This is useful for porting existing unique collections over to Flow (I.E. ETH NFTs)
        access(self) var nftID: UInt64?
        access(self) var metadata: {String: String}
        access(self) var traits: {String : String}

        init() {
            self.metadata = {}
            self.traits = {}
            self.nftID = nil
        }

        pub fun getNftID(): UInt64? {
            return self.nftID
        }
        // Returns all metadata for this collectible
        pub fun getMetadata(): { String: String } {
            return self.metadata
        }
        
        pub fun getTraits(): { String: String } {
            return self.traits
        }

        access(account) fun updateNftID(_ nftID: UInt64) {
            pre {
                self.nftID == nil : "An NFT already exists for this collectible."
            }
            self.nftID = nftID
        }

        access(account) fun updateMetadata(_ metadata: { String: String }) {
            self.metadata = metadata
        }
        
        access(account) fun updateTraits(_ traits: { String: String }) {
            self.traits = traits
        }
    }

    pub struct CollectibleEditionData {
        access(self) let editionNumber: UInt64
        access(self) let setID: UInt64

        init(editionNumber: UInt64, setID: UInt64) {
            self.editionNumber = editionNumber
            self.setID = setID
        }
        pub fun getEditionNumber(): UInt64 {
            return self.editionNumber
        }
        pub fun getSetID(): UInt64 {
            return self.setID
        }
    }

    pub struct SetMetadata {
        access(self) let setID: UInt64
        access(self) var metadata: {String : String}
        access(self) var maxNumberOfEditions: UInt64
        access(self) var editionCount: UInt64
        access(self) var sequentialMintMin: UInt64 
        access(self) var publicFUSDSalePrice: UFix64?
        access(self) var publicFLOWSalePrice: UFix64?
        access(self) var publicSaleStartTime: UFix64?
        access(self) var publicSaleEndTime: UFix64?

        init(
            setID: UInt64,
            maxNumberOfEditions: UInt64,
            metadata: {String: String}
        ){
            self.setID = setID
            self.metadata = metadata
            self.maxNumberOfEditions = maxNumberOfEditions
            self.editionCount = 0
            self.sequentialMintMin = 1

            self.publicFUSDSalePrice = nil
            self.publicFLOWSalePrice = nil
            self.publicSaleStartTime = nil
            self.publicSaleEndTime = nil
        }

        /*
            Readonly functions
        */
        pub fun getSetID(): UInt64 {
            return self.setID
        }

        pub fun getMetadata(): {String : String} {
            return self.metadata
        }

        pub fun getMaxNumberOfEditions(): UInt64 {
            return self.maxNumberOfEditions
        }

        pub fun getEditionCount(): UInt64 {
            return self.editionCount
        }
        
        pub fun getSequentialMintMin(): UInt64 {
            return self.sequentialMintMin
        }

        pub fun getFUSDPublicSalePrice(): UFix64? {
            return self.publicFUSDSalePrice
        }

        pub fun getFLOWPublicSalePrice(): UFix64? {
            return self.publicFLOWSalePrice
        }

        pub fun getPublicSaleStartTime(): UFix64? {
            return self.publicSaleStartTime
        }

        pub fun getPublicSaleEndTime(): UFix64? {
            return self.publicSaleEndTime
        }

        pub fun getEditionMetadata(editionNumber: UInt64): { String: String } {
            pre {
                editionNumber >= 1  && editionNumber <= self.maxNumberOfEditions : "Invalid edition number provided"
                SomePlaceCollectible.collectibleData[self.setID]![editionNumber] != nil : "Requested edition has not yet been minted"
            }
            return SomePlaceCollectible.collectibleData[self.setID]![editionNumber]!.getMetadata()
        }
        
        // A public sale allowing for direct minting from the contract is considered active if we have a valid public
        // sale price listing, current time is after start time, and current time is before end time 
        pub fun isPublicSaleActive(): Bool {
            let curBlockTime = getCurrentBlock().timestamp
            return (self.publicFUSDSalePrice != nil || self.publicFLOWSalePrice != nil) &&
                        (self.publicSaleStartTime != nil && curBlockTime >= self.publicSaleStartTime!) &&
                        (self.publicSaleEndTime == nil || curBlockTime < self.publicSaleEndTime!)
        }

        /*
            Mutating functions
        */
        access(contract) fun incrementEditionCount(): UInt64 {
            post {
                self.editionCount <= self.maxNumberOfEditions : "Number of editions is larger than max allowed editions"
            }
            self.editionCount = self.editionCount + 1
            return self.editionCount
        }
        
        access(contract) fun setSequentialMintMin(newMintMin: UInt64) {
            self.sequentialMintMin = newMintMin
        }

        access(contract) fun updateSetMetadata(_ newMetadata: {String: String}) {
            self.metadata = newMetadata
        }
        
        access(contract) fun updateFLOWPublicSalePrice(_ publicFLOWSalePrice: UFix64?) {
            self.publicFLOWSalePrice = publicFLOWSalePrice
        }

        access(contract) fun updateFUSDPublicSalePrice(_ publicFUSDSalePrice: UFix64?) {
            self.publicFUSDSalePrice = publicFUSDSalePrice
        }

        access(contract) fun updatePublicSaleStartTime(_ startTime: UFix64?) {
            self.publicSaleStartTime = startTime
        }

        access(contract) fun updatePublicSaleEndTime(_ endTime: UFix64?) {
            self.publicSaleEndTime = endTime
        }
    }

    // -----------------------------------------------------------------------
    // SomePlace Interfaces
    // -----------------------------------------------------------------------
    pub resource interface CollectibleCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(collectibleCollection: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCollectible(id: UInt64): &SomePlaceCollectible.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow SomePlace reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Resources
    // -----------------------------------------------------------------------
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        pub let setID: UInt64

        pub let editionNumber: UInt64

        init(setID: UInt64, editionNumber: UInt64) {
            pre {
                SomePlaceCollectible.setData.containsKey(setID) : "Invalid Set ID"
                SomePlaceCollectible.collectibleData[setID]![editionNumber] == nil || SomePlaceCollectible.collectibleData[setID]![editionNumber]!.getNftID() == nil : "This edition already exists"
                editionNumber > 0 && editionNumber <= SomePlaceCollectible.setData[setID]!.getMaxNumberOfEditions() : "Edition number is too high"
            }
            // Update unique set
            self.id = self.uuid
            self.setID = setID
            self.editionNumber = editionNumber

            // If this edition number does not have a metadata object, create one
            if (SomePlaceCollectible.collectibleData[setID]![editionNumber] == nil) {
                let ref = SomePlaceCollectible.collectibleData[setID]!
                ref[editionNumber] = CollectibleMetadata()
                SomePlaceCollectible.collectibleData[setID] = ref
            }
            // Update the metadata object to have a reference to this newly created NFT
            SomePlaceCollectible.collectibleData[setID]![editionNumber]!.updateNftID(self.id)

            // Create mapping of new nft id to its newly created set and edition data
            SomePlaceCollectible.allCollectibleIDs[self.uuid] = CollectibleEditionData(editionNumber: editionNumber, setID: setID)

            // Increase total supply of entire someplace collection
            SomePlaceCollectible.totalSupply = SomePlaceCollectible.totalSupply + (1 as UInt64)

            emit Mint(id: self.uuid)
        }

        destroy() {
            emit Burn(id: self.uuid)
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectibleCollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Cannot withdraw: NFT does not exist in the collection")

            emit Withdraw(id: token.uuid, from: self.owner?.address)

            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }

            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SomePlaceCollectible.NFT

            let id: UInt64 = token.id

            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        pub fun batchDeposit(collectibleCollection: @NonFungibleToken.Collection) {
            let keys = collectibleCollection.getIDs()

            for key in keys {
                self.deposit(token: <-collectibleCollection.withdraw(withdrawID: key))
            }

            destroy collectibleCollection
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowCollectible(id: UInt64) : &SomePlaceCollectible.NFT? {
            if self.ownedNFTs[id] == nil {
                return nil
            } else {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SomePlaceCollectible.NFT
            }
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // SomePlace Admin Functionality
    // -----------------------------------------------------------------------
    /*
        Creation of a new NFT set under the SomePlace umbrella of collectibles
    */
    access(account) fun addNFTSet(
        maxNumberOfEditions: UInt64,
        metadata: {String:String},
    ): UInt64 {
        let id = SomePlaceCounter.nextSetID

        let newSet = SomePlaceCollectible.SetMetadata(setID: id, maxNumberOfEditions: maxNumberOfEditions, metadata: metadata)

        self.collectibleData[id] = {}
        self.setData[id] = newSet

        SomePlaceCounter.incrementSetCounter()

        emit SetCreated(setID: id)
        
        return id
    }

    /*
        Update existing set and edition data
    */
    access(account) fun updateEditionMetadata(setID: UInt64, editionNumber: UInt64, metadata: {String: String}) {
        if (SomePlaceCollectible.collectibleData[setID]![editionNumber] == nil) {
            let ref = SomePlaceCollectible.collectibleData[setID]!
            ref[editionNumber] = CollectibleMetadata()
            SomePlaceCollectible.collectibleData[setID] = ref
        }
        SomePlaceCollectible.collectibleData[setID]![editionNumber]!.updateMetadata(metadata)
    }
    
    access(account) fun updateEditionTraits(setID: UInt64, editionNumber: UInt64, traits: {String: String}) {
        if (SomePlaceCollectible.collectibleData[setID]![editionNumber] == nil) {
            let ref = SomePlaceCollectible.collectibleData[setID]!
            ref[editionNumber] = CollectibleMetadata()
            SomePlaceCollectible.collectibleData[setID] = ref
        }
        SomePlaceCollectible.collectibleData[setID]![editionNumber]!.updateTraits(traits)
    }

    access(account) fun updateSetMetadata(setID: UInt64, metadata: {String: String}) {
        SomePlaceCollectible.setData[setID]!.updateSetMetadata(metadata)
    }
    
    access(account) fun updateFLOWPublicSalePrice(setID: UInt64, price: UFix64?) {
        SomePlaceCollectible.setData[setID]!.updateFLOWPublicSalePrice(price)
    }

    access(account) fun updateFUSDPublicSalePrice(setID: UInt64, price: UFix64?) {
        SomePlaceCollectible.setData[setID]!.updateFUSDPublicSalePrice(price)
    }

    access(account) fun updatePublicSaleStartTime(setID: UInt64, startTime: UFix64?) {
        SomePlaceCollectible.setData[setID]!.updatePublicSaleStartTime(startTime)
    }

    access(account) fun updatePublicSaleEndTime(setID: UInt64, endTime: UFix64?) {
        SomePlaceCollectible.setData[setID]!.updatePublicSaleEndTime(endTime)
    }

    /*
        Minting functions to create editions within a set
    */
    // This mint is intended for sequential mints (for a normal in-order drop style)
    access(account) fun mintSequentialEditionNFT(setID: UInt64): @SomePlaceCollectible.NFT {
        // Find first valid edition number
        var curEditionNumber = self.setData[setID]!.getSequentialMintMin()
        while (SomePlaceCollectible.collectibleData[setID]![curEditionNumber] != nil &&
               SomePlaceCollectible.collectibleData[setID]![curEditionNumber]!.getNftID() != nil) {
            curEditionNumber = curEditionNumber + 1
        }
        self.setData[setID]!.setSequentialMintMin(newMintMin: curEditionNumber)
        let editionCount = self.setData[setID]!.incrementEditionCount()
        
        let newCollectible <-create SomePlaceCollectible.NFT(setID: setID, editionNumber: curEditionNumber)
        return <- newCollectible
    }

    // This mint is intended for settling auctions or manually minting editions,
    // where we mint specific editions to specific recipients when settling
    // SetID + editionID to mint is normally decided off-chain
    access(account) fun mintNFT(setID: UInt64, editionNumber: UInt64): @SomePlaceCollectible.NFT {
        self.setData[setID]!.incrementEditionCount()
        return <-create SomePlaceCollectible.NFT(setID: setID, editionNumber: editionNumber)
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }
    
    // -----------------------------------------------------------------------
    // SomePlace Functions
    // -----------------------------------------------------------------------
    
    // Retrieves all sets (This can be expensive)
    pub fun getMetadatas(): {UInt64: SomePlaceCollectible.SetMetadata} {
        return self.setData
    }
    
    // Retrieves all collectibles (This can be expensive)
    pub fun getCollectibleMetadatas(): {UInt64: {UInt64: CollectibleMetadata}} {
        return self.collectibleData
    }

    // Retrieves how many NFT sets exist
    pub fun getMetadatasCount(): UInt64 {
        return UInt64(self.setData.length)
    }

    pub fun getMetadataForSetID(setID: UInt64): SomePlaceCollectible.SetMetadata? {
        return self.setData[setID]
    }

    pub fun getSetMetadataForNFT(nft: &SomePlaceCollectible.NFT): SomePlaceCollectible.SetMetadata? {
        return self.setData[nft.setID]
    }

    pub fun getSetMetadataForNFTByUUID(uuid: UInt64): SomePlaceCollectible.SetMetadata? {
        let collectibleEditionData = self.allCollectibleIDs[uuid]!
        return self.setData[collectibleEditionData.getSetID()]!
    }

    pub fun getMetadataForNFTByUUID(uuid: UInt64): SomePlaceCollectible.CollectibleMetadata? {
        let collectibleEditionData = self.allCollectibleIDs[uuid]!
        return self.collectibleData[collectibleEditionData.getSetID()]![collectibleEditionData.getEditionNumber()]
    }

    pub fun getMetadataByEditionID(setID: UInt64, editionNumber: UInt64): SomePlaceCollectible.CollectibleMetadata? {
        return self.collectibleData[setID]![editionNumber]
    }

    pub fun getCollectibleDataForNftByUUID(uuid: UInt64): SomePlaceCollectible.CollectibleEditionData? {
        return self.allCollectibleIDs[uuid]!
    }

    init() {
        self.CollectionStoragePath = /storage/somePlaceCollectibleCollection
        self.CollectionPublicPath = /public/somePlaceCollectibleCollection
        self.CollectionPrivatePath = /private/somePlaceCollectibleCollection
        
        self.totalSupply = 0
        self.collectibleData = {}
        self.setData = {}
        self.allCollectibleIDs = {}

        emit ContractInitialized()
    }

}
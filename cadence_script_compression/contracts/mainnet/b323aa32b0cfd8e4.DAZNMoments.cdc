import NonFungibleToken from 0x1d7e57aa55817448

pub contract DAZNMoments: NonFungibleToken{
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event CollectibleCreated(collectibleId: UInt64, metadata: {String: String}, dbid: String)
    pub event CollectibleAddedToSet(setId: UInt64, collectibleId: UInt64)
    pub event CollectibleRetiredFromSet(setId: UInt64, collectibleId: UInt64, numNFTs: UInt64)
    pub event SetLocked(setId: UInt64)
    pub event SetCreated(setId: UInt64)
    pub event NFTMinted(nftId: UInt64, collectibleId: UInt64, serialNumber: UInt64, dbId: String)
    pub event NFTDestroyed(nftId: UInt64)

    access(self) var collectibles: {UInt64: Collectible}
    access(self) var collectibleSets: @{UInt64: Set}
    access(self) var setDatas: {UInt64: SetData}

    pub let CollectionStoragePath:  StoragePath
    pub let CollectionPublicPath:   PublicPath
    pub let AdminStoragePath:       StoragePath
    pub let MinterPrivatePath:      PrivatePath

    pub var nextCollectibleId: UInt64
    pub var nextNFTId: UInt64
    pub var nextSetId: UInt64
    pub var totalSupply: UInt64

    pub struct Collectible {
        pub let collectibleId: UInt64
        pub let dbId: String

        pub let metadata: {String: String}
        init(dbId: String, metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Collectible metadata cannot be empty"
            }
            self.collectibleId = DAZNMoments.nextCollectibleId
            self.metadata = metadata
            self.dbId = dbId
        }
    }

    pub struct NFTData {
        pub let setId: UInt64
        pub let collectibleId: UInt64
        pub let serialNumber: UInt64

        init(setId: UInt64, collectibleId: UInt64, serialNumber: UInt64) {
            self.setId = setId
            self.collectibleId = collectibleId
            self.serialNumber = serialNumber
        }
    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let data: NFTData

        init(serialNumber: UInt64, collectibleId: UInt64, setId: UInt64) {
            let c = DAZNMoments.collectibles[collectibleId]
            let myCollectible = c ?? panic("Missing the collectible")

            DAZNMoments.nextNFTId = DAZNMoments.nextNFTId + (1 as UInt64)
            DAZNMoments.totalSupply = DAZNMoments.totalSupply + (1 as UInt64)
            self.id = DAZNMoments.nextNFTId
            self.data = NFTData(setId: setId, collectibleId: collectibleId, serialNumber: serialNumber)
            emit NFTMinted(nftId: self.id, collectibleId: self.data.collectibleId, serialNumber: self.data.serialNumber, dbId: myCollectible.dbId)
        }
        destroy() {
            emit NFTDestroyed(nftId: self.id)
        }
    }

    // Admin is a special authorization resource that
    // allows the owner to perform important functions to modify the
    // various aspects of the Collectibles, Sets, and NFTs
    //
    pub resource Admin {

        // createCollectible creates a new Collectible struct
        // and stores it in the Collectibles dictionary in the DAZNMoments smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Collectibleer Name": "NAME", "Hometown": "Anywhereville", "Highlight URL" : "http://location.to.image.or.video"}
        //
        // Returns: the Id of the new Collectible object
        //
        pub fun createCollectible(metadata: {String: String}, dbid: String): UInt64 {
            // Create the new Collectible
            var newCollectible = Collectible(dbId: dbid, metadata: metadata)
            let newId = newCollectible.collectibleId

            // Increment the Id so that it isnt used again
            DAZNMoments.nextCollectibleId = DAZNMoments.nextCollectibleId + (1 as UInt64)

            emit CollectibleCreated(collectibleId: newCollectible.collectibleId, metadata: metadata, dbid: dbid)

            // Store it in the contract storage
            DAZNMoments.collectibles[newId] = newCollectible

            return newId
        }

        pub fun createSet(name: String): UInt64 {

            //check if set exists
            let check = DAZNMoments.getSetIdByName(setName: name)
            if check != 0 {
                return check
            }

            // Create the new Set
            var newSet <- create Set(name: name)

            // Increment the setId so that it isnt used again
            DAZNMoments.nextSetId = DAZNMoments.nextSetId + (1 as UInt64)

            let newId = newSet.setId

            emit SetCreated(setId: newSet.setId)

            // Store it in the sets mapping field
            DAZNMoments.collectibleSets[newId] <-! newSet

            return newId
        }

        pub fun borrowSet(setId: UInt64): &Set {
            pre {
                DAZNMoments.collectibleSets[setId] != nil: "Cannot borrow Set: The Set doesnt exist"
            }

            return &DAZNMoments.collectibleSets[setId] as &Set
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Collectibles Collection as
    // to allow others to deposit Collectibles into their Collection. It also allows for reading
    // the Ids of Collectibles in the Collection.
    pub resource interface CollectibleCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCollectible(id: UInt64): &DAZNMoments.NFT? {
            // If the result isnt nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Collectible reference: The Id of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs
    // will store in their account to manage their NFTS
    //
    pub resource Collection: CollectibleCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // Dictionary of Collectible conforming tokens
        // NFT is a resource type with a UInt64 Id field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Collectible from the Collection and moves it to the caller
        //
        // Parameters: withdrawId: The Id of the NFT
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: Collectible does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }

        // deposit takes a Collectible and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {

            // Cast the deposited token as a DAZNMoments NFT to make sure
            // it is the correct type
            let token <- token as! @DAZNMoments.NFT

            // Get the tokens Id
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection
            // is in an accounts storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token that was "removed"
            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            // Get an array of the IDs to be deposited
            let keys = tokens.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // getIds returns an array of the Ids that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to a Collectible in the Collection
        // so that the caller can read its Id
        //
        // Parameters: id: The Id of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the Id of the NFT,
        // not any DAZNMoments specific data. Please use borrowCollectible to
        // read Collectible data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowCollectible returns a borrowed reference to a Collectible
        // so that the caller can read data and call methods from it.
        // They can use this to read its collectibleId, serialNumber,
        // or any of the metadata associated with it by
        // getting collectibleId and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The Id of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowCollectible(id: UInt64): &DAZNMoments.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &DAZNMoments.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        // Much like when Damian Lillard destroys the hopes and
        // dreams of the entire city of Houston.
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

     pub struct SetData {
         pub let setId: UInt64
         pub let name: String

         init(name: String) {
             pre {
                 name.length > 0: "New Set name cannot be empty"
             }
             self.setId = DAZNMoments.nextSetId
             self.name = name
         }
     }

    pub resource Set {
        pub let setId: UInt64

        access(contract) var collectibles: [UInt64]
        access(contract) var retired: {UInt64: Bool}

        pub var locked: Bool

        access(contract) var numberMintedPerCollectible: {UInt64: UInt64}

        init(name: String) {
            self.setId = DAZNMoments.nextSetId
            self.collectibles = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerCollectible = {}

            // Create a new SetData for this Set and store it in contract storage
            DAZNMoments.setDatas[self.setId] = SetData(name: name)
        }

        pub fun addCollectible(collectibleId: UInt64) {
            pre {
                DAZNMoments.collectibles[collectibleId] != nil: "Cannot add the Collectible to Set: Collectible doesnt exist."
                !self.locked: "Cannot add the collectible to the Set after the set has been locked."
                self.numberMintedPerCollectible[collectibleId] == nil: "The collectible has already beed added to the set."
            }

            self.collectibles.append(collectibleId)

            self.retired[collectibleId] = false

            self.numberMintedPerCollectible[collectibleId] = 0

            emit CollectibleAddedToSet(setId: self.setId, collectibleId: collectibleId)
        }

        pub fun addCollectibles(collectibleIds: [UInt64]) {
            for collectible in collectibleIds {
                self.addCollectible(collectibleId: collectible)
            }
        }

        pub fun retireCollectible(collectibleId: UInt64) {
            pre {
                self.retired[collectibleId] != nil: "Cannot retire the Collectible: Collectible doesnt exist in this set!"
            }

            if !self.retired[collectibleId]! {
                self.retired[collectibleId] = true

                emit CollectibleRetiredFromSet(setId: self.setId, collectibleId: collectibleId, numNFTs: self.numberMintedPerCollectible[collectibleId]!)
            }
        }

        pub fun retireAll() {
            for collectible in self.collectibles {
                self.retireCollectible(collectibleId: collectible)
            }
        }

        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setId: self.setId)
            }
        }

        pub fun mintNFT(collectibleId: UInt64): @NFT {
            pre {
                self.retired[collectibleId] != nil: "Cannot mint the NFT: This collectible doesnt exist."
                !self.retired[collectibleId]!: "Cannot mint the NFT from this collectible: This collectible has been retired."
            }

            // Gets the number of NFTs that have been minted for this Collectible
            // to use as this NFTs serial number
            let numInCollectible = self.numberMintedPerCollectible[collectibleId]!

            // Mint the new collectible
            let newNFT: @NFT <- create NFT(serialNumber: numInCollectible + (1 as UInt64),
                                              collectibleId: collectibleId,
                                              setId: self.setId)

            // Increment the count of NFTs minted for this Collectible
            self.numberMintedPerCollectible[collectibleId] = numInCollectible + (1 as UInt64)

            return <-newNFT
        }

        pub fun batchMintNFT(collectibleId: UInt64, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(collectibleId: collectibleId))
                i = i + (1 as UInt64)
            }

            return <-newCollection
        }

        pub fun getCollectibles(): [UInt64] {
            return self.collectibles
        }

        pub fun getRetired(): {UInt64: Bool} {
            return self.retired
        }

        pub fun getNumMintedPerCollectible(): {UInt64: UInt64} {
            return self.numberMintedPerCollectible
        }
    }

    // -----------------------------------------------------------------------
    // DAZNMoments contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // NFTs in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create DAZNMoments.Collection()
    }

    // getAllCollectibles returns all the collectibles in DAZNMoments
    //
    // Returns: An array of all the collectibles that have been created
    pub fun getAllCollectibles(): [DAZNMoments.Collectible] {
        return DAZNMoments.collectibles.values
    }

    // getAllCollectibles returns all the collectibles in topshot
    //
    // Returns: An array of all the sets that have been created
    pub fun getAllSets(): [DAZNMoments.SetData] {
        return DAZNMoments.setDatas.values
    }

    // getCollectibleMetaData returns all the metadata associated with a specific Collectible
    //
    // Parameters: collectibleId: The id of the Collectible that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getCollectibleMetaData(collectibleId: UInt64): {String: String}? {
        return self.collectibles[collectibleId]?.metadata
    }

    // getCollectibleMetaDataByField returns the metadata associated with a
    //                        specific field of the metadata
    //
    // Parameters: CollectibleId: The id of the Collectible that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getCollectibleMetaDataByField(collectibleId: UInt64, field: String): String? {
        // Dont force a revert if the CollectibleId or field is invalid
        if let Collectible = DAZNMoments.collectibles[collectibleId] {
            return Collectible.metadata[field]
        } else {
            return nil
        }
    }

    pub fun getSetName(setId: UInt64): String? {
        // Dont force a revert if the setID is invalid
        return DAZNMoments.setDatas[setId]?.name
    }

    // getSetIDByName returns the ID that the specified Set name
    //                 is associated with.
    //
    // Parameters: setName: The name of the Set that is being searched
    //
    // Returns: The ID of the Set if it exists, or nil if doesnt
    pub fun getSetIdByName(setName: String): UInt64 {
        var setId : UInt64 = 0

        // Iterate through all the setDatas and search for the name
        for setData in DAZNMoments.setDatas.values {
            if setName == setData.name {
                // If the name is found, return the ID
                setId = setData.setId
            }
        }

        return setId
    }

    pub fun getCollectiblesInSet(setId: UInt64): [UInt64]? {
        // Dont force a revert if the setID is invalid
        return DAZNMoments.collectibleSets[setId]?.collectibles
    }

    // -----------------------------------------------------------------------
    // DAZNMoments initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.CollectionStoragePath = /storage/DAZNMomentsNFTCollection
        self.CollectionPublicPath = /public/DAZNMomentsNFTCollection
        self.AdminStoragePath = /storage/DAZNMomentsAdmin
        self.MinterPrivatePath = /private/DAZNMomentsMinter

        self.collectibles = {}
        self.setDatas = {}
        self.collectibleSets <- {}
        self.nextCollectibleId = 1
        self.nextSetId = 1
        self.nextNFTId = 0
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{CollectibleCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}

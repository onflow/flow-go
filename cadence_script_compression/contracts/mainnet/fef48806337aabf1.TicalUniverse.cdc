import NonFungibleToken from 0x1d7e57aa55817448

pub contract TicalUniverse: NonFungibleToken {

    // -----------------------------------------------------------------------
    // TicalUniverse contract Paths
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // TicalUniverse contract Events
    // -----------------------------------------------------------------------

    // Emitted when the TicalUniverse contract is created
    pub event ContractInitialized()

    // Emitted when a new Item struct is created
    pub event ItemCreated(id: UInt32, metadata: {String:String})
    // Emitted when a new series has been started
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Set-Related actions
    //
    // Emitted when a new Set is created
    pub event SetCreated(setId: UInt32, series: UInt32)
    // Emitted when a new Item is added to a Set
    pub event ItemAddedToSet(setId: UInt32, itemId: UInt32)
    // Emitted when an Item is retired from a Set and cannot be used to mint
    pub event ItemRetiredFromSet(setId: UInt32, itemId: UInt32, minted: UInt32)
    // Emitted when a Set is locked, meaning collectibles cannot be added
    pub event SetLocked(setId: UInt32)
    // Emitted when a collectible is minted from a Set
    pub event CollectibleMinted(id: UInt64, itemId: UInt32, setId: UInt32, serialNumber: UInt32)
    // Emitted when a collectible is destroyed
    pub event CollectibleDestroyed(id: UInt64)

    // Events for Collection-related actions
    //
    // Emitted when a collectible is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a collectible is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // TicalUniverse contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Series that this Set belongs to.
    // Many Sets can exist at a time, but only one series.
    pub var currentSeries: UInt32

    // Variable size dictionary of Item structs
    access(self) var itemDatas: {UInt32: Item}

    // Variable size dictionary of SetData structs
    access(self) var setDatas: {UInt32: SetData}

    // Variable size dictionary of Set resources
    access(self) var sets: @{UInt32: Set}

    // The Id that is used to create Items.
    // Every time an Item is created, nextItemId is assigned
    // to the new Item's Id and then is incremented by one.
    pub var nextItemId: UInt32

    // The Id that is used to create Sets.
    // Every time a Set is created, nextSetId is assigned
    // to the new Set's Id and then is incremented by one.
    pub var nextSetId: UInt32

    // The total number of Collectible NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // TicalUniverse contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Item is a Struct that holds metadata associated with a specific collectible item.
    pub struct Item {

        // The unique Id for the Item
        pub let itemId: UInt32

        // Stores all the metadata about the item as a string mapping.
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Item metadata cannot be empty"
            }
            self.itemId = TicalUniverse.nextItemId
            self.metadata = metadata

            // Increment the Id so that it isn't used again
            TicalUniverse.nextItemId = TicalUniverse.nextItemId + UInt32(1)

            emit ItemCreated(id: self.itemId, metadata: metadata)
        }
    }

    // A Set is a grouping of Items that make up a related group of collectibles,
    // like sets of baseball cards.
    // An Item can exist in multiple different sets.
    //
    // SetData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a set by calling various getters located
    // at the end of the contract. Only the admin has the ability
    // to modify any data in the private Set resource.
    pub struct SetData {

        // Unique Id for the Set
        pub let setId: UInt32

        // Name of the Set
        pub let name: String

        // Description of the Set
        pub let description: String?

        // Series that this Set belongs to
        pub let series: UInt32

        init(name: String, description: String?) {
            pre {
                name.length > 0: "New Set name cannot be empty"
            }
            self.setId = TicalUniverse.nextSetId
            self.name = name
            self.description = description
            self.series = TicalUniverse.currentSeries

            // Increment the setId so that it isn't used again
            TicalUniverse.nextSetId = TicalUniverse.nextSetId + UInt32(1)

            emit SetCreated(setId: self.setId, series: self.series)
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // Items from a set and mint Collectibles.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Items to a Set so that the set can mint Collectibles.
    // The Collectible that is minted by a Set will be listed as belonging to
    // the Set that minted it, as well as the Item it reference.
    //
    // Admin can also retire Items from the Set, meaning that the retired
    // Item can no longer have Collectibles minted from it.
    //
    // If the admin locks the Set, no more Items can be added to it, but
    // Collectibles can still be minted.
    //
    // If retireAll() and lock() are called back-to-back,
    // the Set is closed off forever and nothing more can be done with it.
    pub resource Set {

        // Unique Id for the set
        pub let setId: UInt32

        // Array of items that are a part of this set.
        // When an item is added to the set, its Id gets appended here.
        // The Id does not get removed from this array when an Item is retired.
        pub var items: [UInt32]

        // Map of Item Ids that indicates if an Item in this Set can be minted.
        // When an Item is added to a Set, it is mapped to false (not retired).
        // When an Item is retired, this is set to true and cannot be changed.
        pub var retired: {UInt32: Bool}

        // Indicates if the Set is currently locked.
        // When a Set is created, it is unlocked and Items are allowed to be added to it.
        // When a set is locked, Items cannot be added to it.
        // A Set can't transition from locked to unlocked. Locking is final.
        // If a Set is locked, Items cannot be added, but Collectibles can still be minted
        // from Items that exist in the Set.
        pub var locked: Bool

        // Mapping of Item Ids that indicates the number of Collectibles
        // that have been minted for specific Items in this Set.
        // When a Collectible is minted, this value is stored in the Collectible to
        // show its place in the Set, eg. 42 of 100.
        pub var numberMintedPerItem: {UInt32: UInt32}

        init(name: String, description: String?) {
            self.setId = TicalUniverse.nextSetId
            self.items = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerItem = {}

            // Create a new SetData for this Set and store it in contract storage
            TicalUniverse.setDatas[self.setId] = SetData(name: name, description: description)
        }

        // Add an Item to the Set
        //
        // Pre-Conditions:
        // The Item exists.
        // The Set is unlocked.
        // The Item is not present in the Set.
        pub fun addItem(itemId: UInt32) {
            pre {
                TicalUniverse.itemDatas[itemId] != nil: "Cannot add the Item to Set: Item doesn't exist."
                !self.locked: "Cannot add the Item to the Set after the set has been locked."
                self.numberMintedPerItem[itemId] == nil: "The Item has already beed added to the set."
            }

            // Add the Item to the array of Items
            self.items.append(itemId)

            // Allow minting for Item
            self.retired[itemId] = false

            // Initialize the Collectible count to zero
            self.numberMintedPerItem[itemId] = 0

            emit ItemAddedToSet(setId: self.setId, itemId: itemId)
        }

        // Adds multiple Items to the Set
        pub fun addItems(itemIds: [UInt32]) {
            for id in itemIds {
                self.addItem(itemId: id)
            }
        }

        // Retire an Item from the Set. The Set can't mint new Collectibles for the Item.
        // Pre-Conditions:
        // The Item is part of the Set and not retired.
        pub fun retireItem(itemId: UInt32) {
            pre {
                self.retired[itemId] != nil: "Cannot retire the Item: Item doesn't exist in this set!"
            }

            if !self.retired[itemId]! {
                self.retired[itemId] = true

                emit ItemRetiredFromSet(setId: self.setId, itemId: itemId, minted: self.numberMintedPerItem[itemId]!)
            }
        }

        // Retire all the Items in the Set
        pub fun retireAll() {
            for id in self.items {
                self.retireItem(itemId: id)
            }
        }

        // Lock the Set so that no more Items can be added to it.
        //
        // Pre-Conditions:
        // The Set is unlocked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setId: self.setId)
            }
        }

        // Mint a new Collectible and returns the newly minted Collectible.
        // Pre-Conditions:
        // The Item must exist in the Set and be allowed to mint new Collectibles
        pub fun mintCollectible(itemId: UInt32): @NFT {
            pre {
                self.retired[itemId] != nil: "Cannot mint the collectible: This item doesn't exist."
                !self.retired[itemId]!: "Cannot mint the collectible from this item: This item has been retired."
            }

            // Gets the number of Collectibles that have been minted for this Item
            // to use as this Collectibles's serial number
            let minted = self.numberMintedPerItem[itemId]!

            // Mint the new collectible
            let newCollectible: @NFT <- create NFT(serialNumber: minted + UInt32(1),
                                              itemId: itemId,
                                              setId: self.setId)

            // Increment the count of Collectibles minted for this Item
            self.numberMintedPerItem[itemId] = minted + UInt32(1)

            return <-newCollectible
        }

        // Mint an arbitrary quantity of Collectibles and return them as a Collection
        pub fun batchMintCollectible(itemId: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintCollectible(itemId: itemId))
                i = i + UInt64(1)
            }

            return <-newCollection
        }
    }

    // Struct of Collectible metadata
    pub struct CollectibleData {

        // The Id of the Set that the Collectible comes from
        pub let setId: UInt32

        // The Id of the Item that the Collectible references
        pub let itemId: UInt32

        // The place in the edition that this Collectible was minted
        pub let serialNumber: UInt32

        init(setId: UInt32, itemId: UInt32, serialNumber: UInt32) {
            self.setId = setId
            self.itemId = itemId
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the Collectible NFT
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Collectible Id
        pub let id: UInt64

        // Struct of Collectible metadata
        pub let data: CollectibleData

        init(serialNumber: UInt32, itemId: UInt32, setId: UInt32) {

            // Increment the global Collectible Id
            TicalUniverse.totalSupply = TicalUniverse.totalSupply + UInt64(1)

            self.id = TicalUniverse.totalSupply

            self.data = CollectibleData(setId: setId, itemId: itemId, serialNumber: serialNumber)

            emit CollectibleMinted(id: self.id, itemId: itemId, setId: setId, serialNumber: self.data.serialNumber)
        }

        // If the Collectible is destroyed, emit an event
        destroy() {
            emit CollectibleDestroyed(id: self.id)
        }
    }

    // Admin is an authorization resource that allows the owner to modify
    // various aspects of the Items, Sets, and Collectibles
    pub resource Admin {
        // Create a new Item struct and store it in the Items dictionary in the contract
        pub fun createItem(metadata: {String: String}): UInt32 {
            var newItem = Item(metadata: metadata)
            let newId = newItem.itemId

            TicalUniverse.itemDatas[newId] = newItem

            return newId
        }

        // Create a new Set resource and store it in the sets mapping in the contract
        pub fun createSet(name: String, description: String?) {
            var newSet <- create Set(name: name, description: description)
            TicalUniverse.sets[newSet.setId] <-! newSet
        }

        // Return a reference to a set in the contract
        pub fun borrowSet(setId: UInt32): &Set {
            pre {
                TicalUniverse.sets[setId] != nil: "Cannot borrow set: The set doesn't exist."
            }

            return &TicalUniverse.sets[setId] as &Set
        }

        // End the current series and start a new one
        pub fun startNewSeries(): UInt32 {
            TicalUniverse.currentSeries = TicalUniverse.currentSeries + UInt32(1)

            emit NewSeriesStarted(newCurrentSeries: TicalUniverse.currentSeries)

            return TicalUniverse.currentSeries
        }

        // Create a new Admin resource
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // Interface that users can cast their TicalUniverse Collection as
    // to allow others to deposit TicalUniverse Collectibles into their Collection.
    pub resource interface TicalUniverseCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCollectible(id: UInt64): &TicalUniverse.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow collectible reference: The id of the returned reference is incorrect."
            }
        }
    }

    // Collection is a resource that every user who owns NFTs
    // will store in their account to manage their NFTS
    pub resource Collection: TicalUniverseCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // Dictionary of Collectible conforming tokens
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // Remove a Collectible from the Collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: Collectible does not exist in the collection.")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // Withdraw multiple tokens and returns them as a Collection
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }

            return <-batchCollection
        }

        // Add a Collectible to the Collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {

            // Cast the deposited token as a Collectible NFT to make sure
            // it is the correct type
            let token <- token as! @TicalUniverse.NFT

            let id = token.id

            let oldToken <- self.ownedNFTs[id] <- token

            // Emit a deposit event if the Collection is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty removed token
            destroy oldToken
        }

        // Deposit multiple NFTs into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // Get the Ids that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // Return a borrowed reference to a Collectible in the Collection
        // This only allows the caller to read the ID of the NFT,
        // not any Collectible specific data.
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Return a borrowed reference to a Collectible
        // This  allows the caller to read the setId, itemId, serialNumber,
        // and use them to read the setData or Item data from the contract
        pub fun borrowCollectible(id: UInt64): &TicalUniverse.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TicalUniverse.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // TicalUniverse contract-level function definitions
    // -----------------------------------------------------------------------

    // Create a new, empty Collection object so that a user can store it in their account storage
    // and be able to receive Collectibles
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TicalUniverse.Collection()
    }

    // Return all the Collectible Items
    pub fun getAllItems(): [TicalUniverse.Item] {
        return TicalUniverse.itemDatas.values
    }

    // Get all metadata of an Item
    pub fun getItemMetadata(itemId: UInt32): {String: String}? {
        return self.itemDatas[itemId]?.metadata
    }

    // Get a metadata field of an Item
    pub fun getItemMetadataByField(itemId: UInt32, field: String): String? {
        if let item = TicalUniverse.itemDatas[itemId] {
            return item.metadata[field]
        } else {
            return nil
        }
    }

    // Get the name of the Set
    pub fun getSetName(setId: UInt32): String? {
        return TicalUniverse.setDatas[setId]?.name
    }

    // Get the description of the Set
    pub fun getSetDescription(setId: UInt32): String? {
        return TicalUniverse.setDatas[setId]?.description
    }

    // Get the series that the specified Set is associated with
    pub fun getSetSeries(setId: UInt32): UInt32? {
        return TicalUniverse.setDatas[setId]?.series
    }

    // Get the Ids that the specified Set name is associated with
    pub fun getSetIdsByName(setName: String): [UInt32]? {
        var setIds: [UInt32] = []

        for setData in TicalUniverse.setDatas.values {
            if setName == setData.name {
                setIds.append(setData.setId)
            }
        }

        if setIds.length == 0 {
            return nil
        } else {
            return setIds
        }
    }

    // Get the list of Item Ids that are in the Set
    pub fun getItemsInSet(setId: UInt32): [UInt32]? {
        return TicalUniverse.sets[setId]?.items
    }

    // Indicates if a Set/Item combo (otherwise known as an edition) is retired
    pub fun isEditionRetired(setId: UInt32, itemId: UInt32): Bool? {
        if let setToRead <- TicalUniverse.sets.remove(key: setId) {
            let retired = setToRead.retired[itemId]
            TicalUniverse.sets[setId] <-! setToRead
            return retired
        } else {
            return nil
        }
    }

    // Indicates if the Set is locked or not
    pub fun isSetLocked(setId: UInt32): Bool? {
        return TicalUniverse.sets[setId]?.locked
    }

    // Total number of Collectibles that have been minted from an edition
    pub fun getNumberCollectiblesInEdition(setId: UInt32, itemId: UInt32): UInt32? {
        if let setToRead <- TicalUniverse.sets.remove(key: setId) {
            let amount = setToRead.numberMintedPerItem[itemId]

            // Put the Set back into the Sets dictionary
            TicalUniverse.sets[setId] <-! setToRead

            return amount
        } else {
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // TicalUniverse initialization function
    // -----------------------------------------------------------------------

    init() {
        // Paths
        self.CollectionPublicPath= /public/TicalUniverseCollection
        self.CollectionPrivatePath= /private/TicalUniverseCollection
        self.CollectionStoragePath= /storage/TicalUniverseCollection
        self.AdminPublicPath= /public/TicalUniverseAdmin
        self.AdminStoragePath=/storage/TicalUniverseAdmin

        self.currentSeries = 0
        self.itemDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextItemId = 1
        self.nextSetId = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: TicalUniverse.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{TicalUniverseCollectionPublic}>(TicalUniverse.CollectionPublicPath, target: TicalUniverse.CollectionStoragePath)

        // Put the Admin in storage
        self.account.save<@Admin>(<- create Admin(), to: TicalUniverse.AdminStoragePath)

        emit ContractInitialized()
    }
}


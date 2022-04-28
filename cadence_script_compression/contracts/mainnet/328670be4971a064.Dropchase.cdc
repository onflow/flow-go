//SPDX-License-Identifier: UNLICENSED

/*
    Description: Central Smart Contract for Dropchase
    Copied from the smart contract of NBA Top Shot except some modifications
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract Dropchase: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Dropchase contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Dropchase contract is created
    pub event ContractInitialized()

    // Emitted when a new Stat struct is created
    pub event StatCreated(id: UInt32, metadata: {String:String})
    // Emitted when a new series has been triggered by an admin
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Set-Related actions
    //
    // Emitted when a new Set is created
    pub event SetCreated(setID: UInt32, series: UInt32)
    // Emitted when a new Stat is added to a Set
    pub event StatAddedToSet(setID: UInt32, statID: UInt32)
    // Emitted when a Stat is retired from a Set and cannot be used to mint
    pub event StatRetiredFromSet(setID: UInt32, statID: UInt32, numItems: UInt32)
    // Emitted when a Set is locked, meaning Stats cannot be added
    pub event SetLocked(setID: UInt32)
    // Emitted when a Item is minted from a Set
    pub event ItemMinted(ItemID: UInt64, statID: UInt32, setID: UInt32, serialNumber: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a Item is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a Item is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Item is destroyed
    pub event ItemDestroyed(id: UInt64)

    // Emitted when an Item is destroyed
    pub event ItemBurned(itemID: UInt64, from: Address?)

    // -----------------------------------------------------------------------
    // Dropchase contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Series that this Set belongs to.
    // Series is a concept that indicates a group of Sets through time.
    // Many Sets can exist at a time, but only one series.
    pub var currentSeries: UInt32

    // Variable size dictionary of Stat structs
    access(self) var statDatas: {UInt32: Stat}


    // Variable size dictionary of Set resources
    access(self) var sets: @{UInt32: Set}

    // The ID that is used to create Stats. 
    // Every time a Stat is created, statID is assigned 
    // to the new Stat's ID and then is incremented by 1.
    pub var nextStatID: UInt32

    // The ID that is used to create Sets. Every time a Set is created
    // setID is assigned to the new set's ID and then is incremented by 1.
    pub var nextSetID: UInt32

    // The total number of  Drop chase Item NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global Item IDs for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // Dropchase contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Stat is a Struct that holds metadata associated 
    // Item NFTs will all reference a single stat as the owner of
    // its metadata. The stats are publicly accessible, so anyone can
    // read the metadata associated with a specific stat ID
    //
    pub struct Stat {

        // The unique ID for the Stat
        pub let statID: UInt32

        // Stores all the metadata about the stat as a string mapping
        // This is not the long term way NFT metadata will be stored. It's a temporary
        // construct while we figure out a better way to do metadata.
        //
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Stat metadata cannot be empty"
            }
            self.statID = Dropchase.nextStatID
            self.metadata = metadata
        }
    }

    // A Set is a grouping of Stats that have occured in the real world
    // that make up a related group of collectibles, like sets of baseball
    // or Magic cards. A Stat can exist in multiple different sets.
    // 
    // SetData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a set by calling various getters located 
    // at the end of the contract. Only the admin has the ability 
    // to modify any data in the private Set resource.
    //
    pub struct SetData {

        pub let setID: UInt32
        pub let name: String
        pub let series: UInt32
        pub var stats: [UInt32]
        pub var retired: {UInt32: Bool}
        pub var locked: Bool
        pub var numberMintedPerStat: {UInt32: UInt32}

        init(setID: UInt32) {
        let set = &Dropchase.sets[setID] as &Set

        self.setID = set.setID
        self.name = set.name
        self.series = set.series
        self.stats = set.stats
        self.retired = set.retired
        self.locked = set.locked
        self.numberMintedPerStat = set.numberMintedPerStat
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // Stats from a set and mint Items.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Stats to a Set so that the set can mint Items
    // that reference that statdata.
    // The Items that are minted by a Set will be listed as belonging to
    // the Set that minted it, as well as the Stat it references.
    // 
    // Admin can also retire Stats from the Set, meaning that the retired
    // Stat can no longer have Items minted from it.
    //
    // If the admin locks the Set, no more Stats can be added to it, but 
    // Items can still be minted.
    //
    // If retireAll() and lock() are called back-to-back, 
    // the Set is closed off forever and nothing more can be done with it.
    pub resource Set {

        // Unique ID for the set
        pub let setID: UInt32

        // Name of the Set
        pub let name: String

        // Series that this Set belongs to.
        // Series is a concept that indicates a group of Sets through time.
        // Many Sets can exist at a time, but only one series.
        pub let series: UInt32

        // Array of stats that are a part of this set.
        // When a stat is added to the set, its ID gets appended here.
        // The ID does not get removed from this array when a Stat is retired.
        access(contract) var stats: [UInt32]

        // Map of Stat IDs that Indicates if a Stat in this Set can be minted.
        // When a Stat is added to a Set, it is mapped to false (not retired).
        // When a Stat is retired, this is set to true and cannot be changed.
        access(contract) var retired: {UInt32: Bool}

        // Indicates if the Set is currently locked.
        // When a Set is created, it is unlocked 
        // and Stats are allowed to be added to it.
        // When a set is locked, Stats cannot be added.
        // A Set can never be changed from locked to unlocked,
        // the decision to lock a Set it is final.
        // If a Set is locked, Stats cannot be added, but
        // Items can still be minted from Stats
        // that exist in the Set.
        pub var locked: Bool

        // Mapping of Stat IDs that indicates the number of Items 
        // that have been minted for specific Stats in this Set.
        // When a Item is minted, this value is stored in the Item to
        // show its place in the Set, eg. 13 of 60.
        access(contract) var numberMintedPerStat: {UInt32: UInt32}

        init(name: String) {

            self.setID = Dropchase.nextSetID
            self.name = name
            self.series = Dropchase.currentSeries
            self.stats = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerStat = {}

        }

        // addStat adds a stat to the set
        //
        // Parameters: statID: The ID of the Stat that is being added
        //
        // Pre-Conditions:
        // The Stat needs to be an existing stat
        // The Set needs to be not locked
        // The Stat can't have already been added to the Set
        //
        pub fun addStat(statID: UInt32) {
            pre {
                Dropchase.statDatas[statID] != nil: "Cannot add the Stat to Set: Stat doesn't exist."
                !self.locked: "Cannot add the stat to the Set after the set has been locked."
                self.numberMintedPerStat[statID] == nil: "The stat has already beed added to the set."
            }

            // Add the Stat to the array of Stats
            self.stats.append(statID)

            // Open the Stat up for minting
            self.retired[statID] = false

            // Initialize the Item count to zero
            self.numberMintedPerStat[statID] = 0

            emit StatAddedToSet(setID: self.setID, statID: statID)
        }

        // addStats adds multiple Stats to the Set
        //
        // Parameters: statIDs: The IDs of the Stats that are being added
        //                      as an array
        //
        pub fun addStats(statIDs: [UInt32]) {
            for stat in statIDs {
                self.addStat(statID: stat)
            }
        }

        // retireStat retires a Stat from the Set so that it can't mint new Items
        //
        // Parameters: statID: The ID of the Stat that is being retired
        //
        // Pre-Conditions:
        // The Stat is part of the Set and not retired (available for minting).
        // 
        pub fun retireStat(statID: UInt32) {
            pre {
                self.retired[statID] != nil: "Cannot retire the Stat: Stat doesn't exist in this set!"
            }

            if !self.retired[statID]! {
                self.retired[statID] = true

                emit StatRetiredFromSet(setID: self.setID, statID: statID, numItems: self.numberMintedPerStat[statID]!)
            }
        }

        // retireAll retires all the stats in the Set
        // Afterwards, none of the retired Stats will be able to mint new Items
        //
        pub fun retireAll() {
            for stat in self.stats {
                self.retireStat(statID: stat)
            }
        }

        // lock() locks the Set so that no more Stats can be added to it
        //
        // Pre-Conditions:
        // The Set should not be locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        // mintItem mints a new Item and returns the newly minted Item
        // 
        // Parameters: statID: The ID of the Stat that the Item references
        //
        // Pre-Conditions:
        // The Stat must exist in the Set and be allowed to mint new Items
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintItem(statID: UInt32): @NFT {
            pre {
                self.retired[statID] != nil: "Cannot mint the Item: This stat doesn't exist."
                !self.retired[statID]!: "Cannot mint the Item from this stat: This stat has been retired."
            }

            // Gets the number of Items that have been minted for this Stat
            // to use as this Item's serial number
            let numInStat = self.numberMintedPerStat[statID]!

            // Mint the new Item
            let newItem: @NFT <- create NFT(serialNumber: numInStat + 1 as UInt32,
                                              statID: statID,
                                              setID: self.setID)

            // Increment the count of Items minted for this Stat
            self.numberMintedPerStat[statID] = numInStat + 1 as UInt32

            return <-newItem
        }

        // batchMintItem mints an arbitrary quantity of Items 
        // and returns them as a Collection
        //
        // Parameters: statID: the ID of the Stat that the Items are minted for
        //             quantity: The quantity of Items to be minted
        //
        // Returns: Collection object that contains all the Items that were minted
        //
        pub fun batchMintItem(statID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintItem(statID: statID))
                i = i + 1 as UInt64
            }

            return <-newCollection
        }

        pub fun getStats(): [UInt32] {
            return self.stats
        }

        pub fun getRetired(): {UInt32: Bool} {
            return self.retired
        }

        pub fun getNumMintedPerStat(): {UInt32: UInt32} {
            return self.numberMintedPerStat
        }
    }

    // Struct that contains all of the important data about a set
    // Can be easily queried by instantiating the `QuerySetData` object
    // with the desired set ID
    // let setData = Dropchase.QuerySetData(setID: 12)
    //
    pub struct QuerySetData {
        pub let setID: UInt32
        pub let name: String
        pub let series: UInt32
        access(self) var stats: [UInt32]
        access(self) var retired: {UInt32: Bool}
        pub var locked: Bool
        access(self) var numberMintedPerStat: {UInt32: UInt32}

        init(setID: UInt32) {
            pre {
                Dropchase.sets[setID] != nil: "The set with the provided ID does not exist"
            }

            let set = &Dropchase.sets[setID] as &Set
            self.setID = setID
            self.name = set.name
            self.series = set.series
            self.stats = set.stats
            self.retired = set.retired
            self.locked = set.locked
            self.numberMintedPerStat = set.numberMintedPerStat
        }

        pub fun getStats(): [UInt32] {
            return self.stats
        }

        pub fun getRetired(): {UInt32: Bool} {
            return self.retired
        }

        pub fun getNumberMintedPerStat(): {UInt32: UInt32} {
            return self.numberMintedPerStat
        }
    }

    pub struct ItemData {

        // The ID of the Set that the Item comes from
        pub let setID: UInt32

        // The ID of the Stat that the Item references
        pub let statID: UInt32

        // The place in the edition that this Item was minted
        // Otherwise know as the serial number
        pub let serialNumber: UInt32

        init(setID: UInt32, statID: UInt32, serialNumber: UInt32) {
            self.setID = setID
            self.statID = statID
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the Item NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Item ID
        pub let id: UInt64
        
        // Struct of Item metadata
        pub let data: ItemData

        init(serialNumber: UInt32, statID: UInt32, setID: UInt32) {
            // Increment the global Item IDs
            Dropchase.totalSupply = Dropchase.totalSupply + 1 as UInt64

            self.id = Dropchase.totalSupply

            // Set the metadata struct
            self.data = ItemData(setID: setID, statID: statID, serialNumber: serialNumber)

            emit ItemMinted(ItemID: self.id, statID: statID, setID: self.data.setID, serialNumber: self.data.serialNumber)
        }

        // If the Item is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            emit ItemDestroyed(id: self.id)
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Stats, Sets, and Items
    //
    pub resource Admin {

        // createStat creates a new Stat struct 
        // and stores it in the Stats dictionary in the Dropchase smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //
        // Returns: the ID of the new Stat object
        //
        pub fun createStat(metadata: {String: String}): UInt32 {
            // Create the new Stat
            var newStat = Stat(metadata: metadata)
            let newID = newStat.statID

            // Increment the ID so that it isn't used again
            Dropchase.nextStatID = Dropchase.nextStatID + 1 as UInt32

            emit StatCreated(id: newStat.statID, metadata: metadata)

            // Store it in the contract storage
            Dropchase.statDatas[newID] = newStat

            return newID
        }

        // createSet creates a new Set resource and stores it
        // in the sets mapping in the Dropchase contract
        //
        // Parameters: name: The name of the Set
        //
        // Returns: The ID of the created set
        pub fun createSet(name: String): UInt32 {

            // Create the new Set
            var newSet <- create Set(name: name)

            // Increment the setID so that it isn't used again
            Dropchase.nextSetID = Dropchase.nextSetID + 1 as UInt32

            let newID = newSet.setID

            emit SetCreated(setID: newSet.setID, series: Dropchase.currentSeries)

            // Store it in the sets mapping field
            Dropchase.sets[newID] <-! newSet

            return newID
        }

        // borrowSet returns a reference to a set in the Dropchase
        // contract so that the admin can call methods on it
        //
        // Parameters: setID: The ID of the Set that you want to
        // get a reference to
        //
        // Returns: A reference to the Set with all of the fields
        // and methods exposed
        //
        pub fun borrowSet(setID: UInt32): &Set {
            pre {
                Dropchase.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
            }
            
            // Get a reference to the Set and return it
            // use `&` to indicate the reference to the object and type
            return &Dropchase.sets[setID] as &Set
        }

        // startNewSeries ends the current series by incrementing
        // the series number, meaning that Items minted after this
        // will use the new series number
        //
        // Returns: The new series number
        //
        pub fun startNewSeries(): UInt32 {
            // End the current series and start a new one
            // by incrementing the Dropchase series number
            Dropchase.currentSeries = Dropchase.currentSeries + 1 as UInt32

            emit NewSeriesStarted(newCurrentSeries: Dropchase.currentSeries)

            return Dropchase.currentSeries
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Item Collection as
    // to allow others to deposit Items into their Collection. It also allows for reading
    // the IDs of Items in the Collection.
    pub resource interface ItemCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItem(id: UInt64): &Dropchase.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Item reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: ItemCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Item conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Item from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Item does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Items
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a Item and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a Dropchase NFT to make sure
            // it is the correct type
            let token <- token as! @Dropchase.NFT

            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
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

        // getIDs returns an array of the IDs that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to a Item in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any Dropchase specific data. Please use borrowItem to 
        // read Item data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowItem returns a borrowed reference to a Item
        // so that the caller can read data and call methods from it.
        // They can use this to read its setID, statID, serialNumber,
        // or any of the setData or Stat data associated with it by
        // getting the setID or statID and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowItem(id: UInt64): &Dropchase.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Dropchase.NFT
            } else {
                return nil
            }
        }

        pub fun burnItem(withdrawID: UInt64) {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Item does not exist in the collection")

            emit ItemBurned(itemID: token.id ,from: self.owner?.address)
            
            //destroy old token
            destroy token

        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Dropchase contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Items in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create Dropchase.Collection()
    }

    // getAllStats returns all the stats in Dropchase
    //
    // Returns: An array of all the stats that have been created
    pub fun getAllStats(): [Dropchase.Stat] {
        return Dropchase.statDatas.values
    }

    // getStatMetaData returns all the metadata associated with a specific Stat
    // 
    // Parameters: statID: The id of the Stat that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getStatMetaData(statID: UInt32): {String: String}? {
        return self.statDatas[statID]?.metadata
    }

    // getStatMetaDataByField returns the metadata associated with a 
    //                        specific field of the metadata
    // 
    // Parameters: statID: The id of the Stat that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getStatMetaDataByField(statID: UInt32, field: String): String? {
        // Don't force a revert if the statID or field is invalid
        if let stat = Dropchase.statDatas[statID] {
            return stat.metadata[field]
        } else {
            return nil
        }
    }

    // getSetData returns the data that the specified Set
    //            is associated with.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: The QuerySetData struct that has all the important information about the set
    pub fun getSetData(setID: UInt32): QuerySetData? {
        if Dropchase.sets[setID] == nil {
            return nil
        } else {
            return QuerySetData(setID: setID)
        }
    }

    // getSetName returns the name that the specified Set
    //            is associated with.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: The name of the Set
    pub fun getSetName(setID: UInt32): String? {
        // Don't force a revert if the setID is invalid
        return Dropchase.sets[setID]?.name
    }

    // getSetSeries returns the series that the specified Set
    //              is associated with.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: The series that the Set belongs to
    pub fun getSetSeries(setID: UInt32): UInt32? {
        // Don't force a revert if the setID is invalid
        return Dropchase.sets[setID]?.series
    }

    // getSetIDsByName returns the IDs that the specified Set name
    //                 is associated with.
    // 
    // Parameters: setName: The name of the Set that is being searched
    //
    // Returns: An array of the IDs of the Set if it exists, or nil if doesn't
    /* ### First let it commented out
    pub fun getSetIDsByName(setName: String): [UInt32]? {
        var setIDs: [UInt32] = []
        // Iterate through all the setDatas and search for the name
        for setData in Dropchase.se {
            if setName == setData.name {
                // If the name is found, return the ID
                setIDs.append(setData.setID)
            }
        }

        // If the name isn't found, return nil
        // Don't force a revert if the setName is invalid
        if setIDs.length == 0 {
            return nil
        } else {
            return setIDs
        }
    }
    */

    // getStatsInSet returns the list of Stat IDs that are in the Set
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: An array of Stat IDs
    pub fun getStatsInSet(setID: UInt32): [UInt32]? {
        // Don't force a revert if the setID is invalid
        return Dropchase.sets[setID]?.stats
    }

    // isEditionRetired returns a boolean that indicates if a Set/Stat combo
    //                  (otherwise known as an edition) is retired.
    //                  If an edition is retired, it still remains in the Set,
    //                  but Items can no longer be minted from it.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //             statID: The id of the Stat that is being searched
    //
    // Returns: Boolean indicating if the edition is retired or not
    pub fun isEditionRetired(setID: UInt32, statID: UInt32): Bool? {

        if let setdata = self.getSetData(setID: setID) {

            // See if the Stat is retired from this Set
            let retired = setdata.getRetired()[statID]

            // Return the retired status
            return retired
        } else {

            // If the Set wasn't found, return nil
            return nil
        }
    }

    // isSetLocked returns a boolean that indicates if a Set
    //             is locked. If it's locked, 
    //             new Stats can no longer be added to it,
    //             but Items can still be minted from Stats the set contains.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: Boolean indicating if the Set is locked or not
    pub fun isSetLocked(setID: UInt32): Bool? {
        // Don't force a revert if the setID is invalid
        return Dropchase.sets[setID]?.locked
    }

    // getNumItemsInEdition return the number of Items that have been 
    //                        minted from a certain edition.
    //
    // Parameters: setID: The id of the Set that is being searched
    //             statID: The id of the Stat that is being searched
    //
    // Returns: The total number of Items 
    //          that have been minted from an edition
    pub fun getNumItemsInEdition(setID: UInt32, statID: UInt32): UInt32? {
        if let setdata = self.getSetData(setID: setID) {

            // Read the numMintedPerStat
            let amount = setdata.getNumberMintedPerStat()[statID]

            return amount
        } else {
            // If the set wasn't found return nil
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // Dropchase initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.currentSeries = 1
        self.statDatas = {}
        self.sets <- {}
        self.nextStatID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: /storage/DropchaseItemCollection)

        // Create a public capability for the Collection
        self.account.link<&{ItemCollectionPublic}>(/public/DropchaseItemCollection, target: /storage/DropchaseItemCollection)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/DropchaseAdmin)

        emit ContractInitialized()
    }
}
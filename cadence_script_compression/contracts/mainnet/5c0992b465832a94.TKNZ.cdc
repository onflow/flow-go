// Deployed for TKNZ Ltd. - https://tknz.gg

/*
    Description: Smart Contract for TKNZ

    This smart contract contains the core functionality for TKNZ

    The contract manages the data associated with all the plays and drops
    that are used as templates for the TKNZ NFTs

    When a new Play wants to be added to the records, an Admin creates
    a new Play struct that is stored in the smart contract.

    Then an Admin can create new Drops. Drops consist of a public struct that 
    contains public information about a drop, and a private resource used
    to mint new NFTs based off of plays that have been linked to the Drop.

    The admin resource has the power to do all of the important actions
    in the smart contract. When admins want to call functions in a drop,
    they call their borrowDrop function to get a reference 
    to a drop in the contract. Then, they can call functions on the drop using that reference.
    
    When NFTs are minted, they are initialized with a NFTData struct and
    are returned by the minter.

    The contract also defines a Collection resource. This is an object that 
    every TKNZ NFT owner will store in their account
    to manage their NFT collection.

    The main TKNZ account will also have its own NFT collections
    it can use to hold its own NFTs that have not yet been sent to a user.

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.

*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract TKNZ: NonFungibleToken {

    // -----------------------------------------------------------------------
    // TKNZ contract Events
    // -----------------------------------------------------------------------

    // Emitted when the TKNZ contract is created
    pub event ContractInitialized()

    // Emitted when a new Play struct is created
    pub event PlayCreated(id: UInt32, metadata: {String:String})
    // Emitted when a new series has been triggered by an admin
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Drop-Related actions
    //
    // Emitted when a new Drop is created
    pub event DropCreated(dropID: UInt32, series: UInt32)
    // Emitted when a new Play is added to a Drop
    pub event PlayAddedToDrop(dropID: UInt32, playID: UInt32)
    // Emitted when a Play is retired from a Drop and cannot be used to mint
    pub event PlayRetiredFromDrop(dropID: UInt32, playID: UInt32, numNFTs: UInt32)
    // Emitted when a Drop is locked, meaning Plays cannot be added
    pub event DropLocked(dropID: UInt32)
    // Emitted when a NFT is minted from a Drop
    pub event NFTMinted(NFTID: UInt64, playID: UInt32, dropID: UInt32, serialNumber: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a NFT is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a NFT is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a NFT is destroyed
    pub event NFTDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // TKNZ contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Series that this Drop belongs to.
    // Series is a concept that indicates a group of Drops through time.
    // Many Drops can exist at a time, but only one series.
    pub var currentSeries: UInt32

    // Variable size dictionary of Play structs
    access(self) var playDatas: {UInt32: Play}

    // Variable size dictionary of DropData structs
    access(self) var dropDatas: {UInt32: DropData}

    // Variable size dictionary of Drop resources
    access(self) var drops: @{UInt32: Drop}

    // The ID that is used to create Plays. 
    // Every time a Play is created, playID is assigned 
    // to the new Play's ID and then is incremented by 1.
    pub var nextPlayID: UInt32

    // The ID that is used to create Drops. Every time a Drop is created
    // dropID is assigned to the new drop's ID and then is incremented by 1.
    pub var nextDropID: UInt32

    // The total number of TKNZ NFT NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global NFT IDs for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // TKNZ contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Play is a Struct that holds metadata associated 
    // with a specific NBA play, like the legendary NFT when 
    // Ray Allen hit the 3 to tie the Heat and Spurs in the 2013 finals game 6
    // or when Lance Stephenson blew in the ear of Lebron James.
    //
    // NFT NFTs will all reference a single play as the owner of
    // its metadata. The plays are publicly accessible, so anyone can
    // read the metadata associated with a specific play ID
    //
    pub struct Play {

        // The unique ID for the Play
        pub let playID: UInt32

        // Stores all the metadata about the play as a string mapping
        // This is not the long term way NFT metadata will be stored. It's a temporary
        // construct while we figure out a better way to do metadata.
        //
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Play metadata cannot be empty"
            }
            self.playID = TKNZ.nextPlayID
            self.metadata = metadata
        }
    }

    // A Drop is a grouping of Plays that have occured in the real world
    // that make up a related group of collectibles, like drops of baseball
    // or Magic cards. A Play can exist in multiple different drops.
    // 
    // DropData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a drop by calling various getters located 
    // at the end of the contract. Only the admin has the ability 
    // to modify any data in the private Drop resource.
    //
    pub struct DropData {

        // Unique ID for the Drop
        pub let dropID: UInt32

        // Name of the Drop
        // ex. "Times when the Toronto Raptors choked in the playoffs"
        pub let name: String

        // Series that this Drop belongs to.
        // Series is a concept that indicates a group of Drops through time.
        // Many Drops can exist at a time, but only one series.
        pub let series: UInt32

        init(name: String) {
            pre {
                name.length > 0: "New Drop name cannot be empty"
            }
            self.dropID = TKNZ.nextDropID
            self.name = name
            self.series = TKNZ.currentSeries
        }
    }

    // Drop is a resource type that contains the functions to add and remove
    // Plays from a drop and mint NFTs.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Plays to a Drop so that the drop can mint NFTs
    // that reference that playdata.
    // The NFTs that are minted by a Drop will be listed as belonging to
    // the Drop that minted it, as well as the Play it references.
    // 
    // Admin can also retire Plays from the Drop, meaning that the retired
    // Play can no longer have NFTs minted from it.
    //
    // If the admin locks the Drop, no more Plays can be added to it, but 
    // NFTs can still be minted.
    //
    // If retireAll() and lock() are called back-to-back, 
    // the Drop is closed off forever and nothing more can be done with it.
    pub resource Drop {

        // Unique ID for the drop
        pub let dropID: UInt32

        // Array of plays that are a part of this drop.
        // When a play is added to the drop, its ID gets appended here.
        // The ID does not get removed from this array when a Play is retired.
        access(contract) var plays: [UInt32]

        // Map of Play IDs that Indicates if a Play in this Drop can be minted.
        // When a Play is added to a Drop, it is mapped to false (not retired).
        // When a Play is retired, this is drop to true and cannot be changed.
        access(contract) var retired: {UInt32: Bool}

        // Indicates if the Drop is currently locked.
        // When a Drop is created, it is unlocked 
        // and Plays are allowed to be added to it.
        // When a drop is locked, Plays cannot be added.
        // A Drop can never be changed from locked to unlocked,
        // the decision to lock a Drop it is final.
        // If a Drop is locked, Plays cannot be added, but
        // NFTs can still be minted from Plays
        // that exist in the Drop.
        pub var locked: Bool

        // Mapping of Play IDs that indicates the number of NFTs 
        // that have been minted for specific Plays in this Drop.
        // When a NFT is minted, this value is stored in the NFT to
        // show its place in the Drop, eg. 13 of 60.
        access(contract) var numberMintedPerPlay: {UInt32: UInt32}

        init(name: String) {
            self.dropID = TKNZ.nextDropID
            self.plays = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerPlay = {}

            // Create a new DropData for this Drop and store it in contract storage
            TKNZ.dropDatas[self.dropID] = DropData(name: name)
        }

        // addPlay adds a play to the drop
        //
        // Parameters: playID: The ID of the Play that is being added
        //
        // Pre-Conditions:
        // The Play needs to be an existing play
        // The Drop needs to be not locked
        // The Play can't have already been added to the Drop
        //
        pub fun addPlay(playID: UInt32) {
            pre {
                TKNZ.playDatas[playID] != nil: "Cannot add the Play to Drop: Play doesn't exist."
                !self.locked: "Cannot add the play to the Drop after the drop has been locked."
                self.numberMintedPerPlay[playID] == nil: "The play has already beed added to the drop."
            }

            // Add the Play to the array of Plays
            self.plays.append(playID)

            // Open the Play up for minting
            self.retired[playID] = false

            // Initialize the NFT count to zero
            self.numberMintedPerPlay[playID] = 0

            emit PlayAddedToDrop(dropID: self.dropID, playID: playID)
        }

        // addPlays adds multiple Plays to the Drop
        //
        // Parameters: playIDs: The IDs of the Plays that are being added
        //                      as an array
        //
        pub fun addPlays(playIDs: [UInt32]) {
            for play in playIDs {
                self.addPlay(playID: play)
            }
        }

        // retirePlay retires a Play from the Drop so that it can't mint new NFTs
        //
        // Parameters: playID: The ID of the Play that is being retired
        //
        // Pre-Conditions:
        // The Play is part of the Drop and not retired (available for minting).
        // 
        pub fun retirePlay(playID: UInt32) {
            pre {
                self.retired[playID] != nil: "Cannot retire the Play: Play doesn't exist in this drop!"
            }

            if !self.retired[playID]! {
                self.retired[playID] = true

                emit PlayRetiredFromDrop(dropID: self.dropID, playID: playID, numNFTs: self.numberMintedPerPlay[playID]!)
            }
        }

        // retireAll retires all the plays in the Drop
        // Afterwards, none of the retired Plays will be able to mint new NFTs
        //
        pub fun retireAll() {
            for play in self.plays {
                self.retirePlay(playID: play)
            }
        }

        // lock() locks the Drop so that no more Plays can be added to it
        //
        // Pre-Conditions:
        // The Drop should not be locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit DropLocked(dropID: self.dropID)
            }
        }

        // mintNFT mints a new NFT and returns the newly minted NFT
        // 
        // Parameters: playID: The ID of the Play that the NFT references
        //
        // Pre-Conditions:
        // The Play must exist in the Drop and be allowed to mint new NFTs
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintNFT(playID: UInt32): @NFT {
            pre {
                self.retired[playID] != nil: "Cannot mint the NFT: This play doesn't exist."
                !self.retired[playID]!: "Cannot mint the NFT from this play: This play has been retired."
            }

            // Gets the number of NFTs that have been minted for this Play
            // to use as this NFT's serial number
            let numInPlay = self.numberMintedPerPlay[playID]!

            // Mint the new NFT
            let newNFT: @NFT <- create NFT(serialNumber: numInPlay + UInt32(1),
                                              playID: playID,
                                              dropID: self.dropID)

            // Increment the count of NFTs minted for this Play
            self.numberMintedPerPlay[playID] = numInPlay + UInt32(1)

            return <-newNFT
        }

        // batchMintNFT mints an arbitrary quantity of NFTs 
        // and returns them as a Collection
        //
        // Parameters: playID: the ID of the Play that the NFTs are minted for
        //             quantity: The quantity of NFTs to be minted
        //
        // Returns: Collection object that contains all the NFTs that were minted
        //
        pub fun batchMintNFT(playID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(playID: playID))
                i = i + UInt64(1)
            }

            return <-newCollection
        }

        pub fun getPlays(): [UInt32] {
            return self.plays
        }

        pub fun getRetired(): {UInt32: Bool} {
            return self.retired
        }

        pub fun getNumMintedPerPlay(): {UInt32: UInt32} {
            return self.numberMintedPerPlay
        }
    }

    // Struct that contains all of the important data about a drop
        // Can be easily queried by instantiating the `QueryDropData` object
        // with the desired drop ID
        // let dropData = TKNZ.QueryDropData(dropID: 12)
        //
        pub struct QueryDropData {
            pub let dropID: UInt32
            pub let name: String
            pub let series: UInt32
            access(self) var plays: [UInt32]
            access(self) var retired: {UInt32: Bool}
            pub var locked: Bool
            access(self) var numberMintedPerPlay: {UInt32: UInt32}
            init(dropID: UInt32) {
                pre {
                    TKNZ.drops[dropID] != nil: "The drop with the provided ID does not exist"
                }
                let drop = &TKNZ.drops[dropID] as &Drop
                let dropData = TKNZ.dropDatas[dropID]!
                self.dropID = dropID
                self.name = dropData.name
                self.series = dropData.series
                self.plays = drop.plays
                self.retired = drop.retired
                self.locked = drop.locked
                self.numberMintedPerPlay = drop.numberMintedPerPlay
            }
            pub fun getPlays(): [UInt32] {
                return self.plays
            }
            pub fun getRetired(): {UInt32: Bool} {
                return self.retired
            }
            pub fun getNumberMintedPerPlay(): {UInt32: UInt32} {
                return self.numberMintedPerPlay
            }
        }

    pub struct NFTData {

        // The ID of the Drop that the NFT comes from
        pub let dropID: UInt32

        // The ID of the Play that the NFT references
        pub let playID: UInt32

        // The place in the edition that this NFT was minted
        // Otherwise know as the serial number
        pub let serialNumber: UInt32

        init(dropID: UInt32, playID: UInt32, serialNumber: UInt32) {
            self.dropID = dropID
            self.playID = playID
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the NFT NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique NFT ID
        pub let id: UInt64
        
        // Struct of NFT metadata
        pub let data: NFTData

        init(serialNumber: UInt32, playID: UInt32, dropID: UInt32) {
            // Increment the global NFT IDs
            TKNZ.totalSupply = TKNZ.totalSupply + UInt64(1)

            self.id = TKNZ.totalSupply

            // Drop the metadata struct
            self.data = NFTData(dropID: dropID, playID: playID, serialNumber: serialNumber)

            emit NFTMinted(NFTID: self.id, playID: playID, dropID: self.data.dropID, serialNumber: self.data.serialNumber)
        }

        // If the NFT is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            emit NFTDestroyed(id: self.id)
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Plays, Drops, and NFTs
    //
    pub resource Admin {

        // createPlay creates a new Play struct 
        // and stores it in the Plays dictionary in the TKNZ smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Player Name": "Kevin Durant", "Height": "7 feet"}
        //                               (because we all know Kevin Durant is not 6'9")
        //
        // Returns: the ID of the new Play object
        //
        pub fun createPlay(metadata: {String: String}): UInt32 {
            // Create the new Play
            var newPlay = Play(metadata: metadata)
            let newID = newPlay.playID

            // Increment the ID so that it isn't used again
            TKNZ.nextPlayID = TKNZ.nextPlayID + UInt32(1)
            emit PlayCreated(id: newPlay.playID, metadata: metadata)

            // Store it in the contract storage
            TKNZ.playDatas[newID] = newPlay

            return newID
        }

        // createDrop creates a new Drop resource and stores it
        // in the drops mapping in the TKNZ contract
        //
        // Parameters: name: The name of the Drop
        //
        // Returns: The ID of the created drop
        pub fun createDrop(name: String): UInt32 {
            // Create the new Drop
            var newDrop <- create Drop(name: name)

            // Increment the dropID so that it isn't used again
            TKNZ.nextDropID = TKNZ.nextDropID + UInt32(1)
            let newID = newDrop.dropID
            emit DropCreated(dropID: newDrop.dropID, series: TKNZ.currentSeries)

            // Store it in the drops mapping field
            TKNZ.drops[newDrop.dropID] <-! newDrop

            return newID
        }

        // borrowDrop returns a reference to a drop in the TKNZ
        // contract so that the admin can call methods on it
        //
        // Parameters: dropID: The ID of the Drop that you want to
        // get a reference to
        //
        // Returns: A reference to the Drop with all of the fields
        // and methods exposed
        //
        pub fun borrowDrop(dropID: UInt32): &Drop {
            pre {
                TKNZ.drops[dropID] != nil: "Cannot borrow Drop: The Drop doesn't exist"
            }
            
            // Get a reference to the Drop and return it
            // use `&` to indicate the reference to the object and type
            return &TKNZ.drops[dropID] as &Drop
        }

        // startNewSeries ends the current series by incrementing
        // the series number, meaning that NFTs minted after this
        // will use the new series number
        //
        // Returns: The new series number
        //
        pub fun startNewSeries(): UInt32 {
            // End the current series and start a new one
            // by incrementing the TKNZ series number
            TKNZ.currentSeries = TKNZ.currentSeries + UInt32(1)

            emit NewSeriesStarted(newCurrentSeries: TKNZ.currentSeries)

            return TKNZ.currentSeries
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their NFT Collection as
    // to allow others to deposit NFTs into their Collection. It also allows for reading
    // the IDs of NFTs in the Collection.
    pub resource interface NFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNFTReference(id: UInt64): &TKNZ.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow NFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: NFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of NFT conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: NFT does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn NFTs
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

        // deposit takes a NFT and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a TKNZ NFT to make sure
            // it is the correct type
            let token <- token as! @TKNZ.NFT

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

        // borrowNFTReference Returns a borrowed reference to a NFT in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any TKNZ specific data. Please use borrowNFTReference to 
        // read NFT data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowNFTReference returns a borrowed reference to a NFT
        // so that the caller can read data and call methods from it.
        // They can use this to read its dropID, playID, serialNumber,
        // or any of the dropData or Play data associated with it by
        // getting the dropID or playID and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowNFTReference(id: UInt64): &TKNZ.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TKNZ.NFT
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

    // -----------------------------------------------------------------------
    // TKNZ contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // NFTs in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TKNZ.Collection()
    }

    // getAllPlays returns all the plays in TKNZ
    //
    // Returns: An array of all the plays that have been created
    pub fun getAllPlays(): [TKNZ.Play] {
        return TKNZ.playDatas.values
    }

    // getPlayMetaData returns all the metadata associated with a specific Play
    // 
    // Parameters: playID: The id of the Play that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getPlayMetaData(playID: UInt32): {String: String}? {
        return self.playDatas[playID]?.metadata
    }

    // getPlayMetaDataByField returns the metadata associated with a 
    //                        specific field of the metadata
    //                        Ex: field: "Team" will return something
    //                        like "Memphis Grizzlies"
    // 
    // Parameters: playID: The id of the Play that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getPlayMetaDataByField(playID: UInt32, field: String): String? {
        // Don't force a revert if the playID or field is invalid
        if let play = TKNZ.playDatas[playID] {
            return play.metadata[field]
        } else {
            return nil
        }
    }

    // getDropData returns the data that the specified Drop
    //            is associated with.
    //
    // Parameters: dropID: The id of the Drop that is being searched
    //
    // Returns: The QueryDropData struct that has all the important information about the drop
    pub fun getDropData(dropID: UInt32): QueryDropData? {
        if TKNZ.drops[dropID] == nil {
            return nil
        } else {
            return QueryDropData(dropID: dropID)
        }
    }

    // getDropName returns the name that the specified Drop
    //            is associated with.
    // 
    // Parameters: dropID: The id of the Drop that is being searched
    //
    // Returns: The name of the Drop
    pub fun getDropName(dropID: UInt32): String? {
        // Don't force a revert if the dropID is invalid
        return TKNZ.dropDatas[dropID]?.name
    }

    // getDropSeries returns the series that the specified Drop
    //              is associated with.
    // 
    // Parameters: dropID: The id of the Drop that is being searched
    //
    // Returns: The series that the Drop belongs to
    pub fun getDropSeries(dropID: UInt32): UInt32? {
        // Don't force a revert if the dropID is invalid
        return TKNZ.dropDatas[dropID]?.series
    }

    // getDropIDsByName returns the IDs that the specified Drop name
    //                 is associated with.
    // 
    // Parameters: dropName: The name of the Drop that is being searched
    //
    // Returns: An array of the IDs of the Drop if it exists, or nil if doesn't
    pub fun getDropIDsByName(dropName: String): [UInt32]? {
        var dropIDs: [UInt32] = []

        // Iterate through all the dropDatas and search for the name
        for dropData in TKNZ.dropDatas.values {
            if dropName == dropData.name {
                // If the name is found, return the ID
                dropIDs.append(dropData.dropID)
            }
        }

        // If the name isn't found, return nil
        // Don't force a revert if the dropName is invalid
        if dropIDs.length == 0 {
            return nil
        } else {
            return dropIDs
        }
    }

    // getPlaysInDrop returns the list of Play IDs that are in the Drop
    // 
    // Parameters: dropID: The id of the Drop that is being searched
    //
    // Returns: An array of Play IDs
    pub fun getPlaysInDrop(dropID: UInt32): [UInt32]? {
        // Don't force a revert if the dropID is invalid
        return TKNZ.drops[dropID]?.plays
    }

    // isEditionRetired returns a boolean that indicates if a Drop/Play combo
    //                  (otherwise known as an edition) is retired.
    //                  If an edition is retired, it still remains in the Drop,
    //                  but NFTs can no longer be minted from it.
    // 
    // Parameters: dropID: The id of the Drop that is being searched
    //             playID: The id of the Play that is being searched
    //
    // Returns: Boolean indicating if the edition is retired or not
    pub fun isEditionRetired(dropID: UInt32, playID: UInt32): Bool? {
        if let dropdata = self.getDropData(dropID: dropID) {
            // See if the Play is retired from this Drop
            let retired = dropdata.getRetired()[playID]
            // Return the retired status
            return retired
        } else {

            // If the Drop wasn't found, return nil
            return nil
        }
    }

    // isDropLocked returns a boolean that indicates if a Drop
    //             is locked. If it's locked, 
    //             new Plays can no longer be added to it,
    //             but NFTs can still be minted from Plays the drop contains.
    // 
    // Parameters: dropID: The id of the Drop that is being searched
    //
    // Returns: Boolean indicating if the Drop is locked or not
    pub fun isDropLocked(dropID: UInt32): Bool? {
        // Don't force a revert if the dropID is invalid
        return TKNZ.drops[dropID]?.locked
    }

    // getNumNFTsInEdition return the number of NFTs that have been 
    //                        minted from a certain edition.
    //
    // Parameters: dropID: The id of the Drop that is being searched
    //             playID: The id of the Play that is being searched
    //
    // Returns: The total number of NFTs 
    //          that have been minted from an edition
    pub fun getNumNFTsInEdition(dropID: UInt32, playID: UInt32): UInt32? {
        if let dropdata = self.getDropData(dropID: dropID) {
            // Read the numMintedPerPlay
            let amount = dropdata.getNumberMintedPerPlay()[playID]

            return amount
        } else {
            // If the drop wasn't found return nil
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // TKNZ initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.currentSeries = 0
        self.playDatas = {}
        self.dropDatas = {}
        self.drops <- {}
        self.nextPlayID = 1
        self.nextDropID = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: /storage/NFTCollection)

        // Create a public capability for the Collection
        self.account.link<&{NFTCollectionPublic}>(/public/NFTCollection, target: /storage/NFTCollection)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/TKNZAdmin)

        emit ContractInitialized()
    }
}
 

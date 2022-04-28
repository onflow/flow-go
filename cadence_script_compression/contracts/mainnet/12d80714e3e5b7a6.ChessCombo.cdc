/*
    Description: Central Smart Contract for ChessCombo
    
    Based on NBA Top Shot Smart Contract: https://github.com/dapperlabs/nba-smart-contracts
    ChessCombo Adoption: Milos Ponjavusic

    The contract manages the data associated with all the Combinations and Compilations
    that are used as templates for the Combo NFTs

    When a new Combination is to be added, an Admin creates
    a new Combination struct that is stored in the smart contract.

    Then an Admin can create new Compilations. Compilations consist of a public struct that 
    contains public information about a Compilation, and a private resource used
    to mint new Combos based off of Combinations that have been linked to the Compilation.

    The contract also defines a Collection resource. This is an object that 
    every Combo NFT owner will store in their account
    to manage their NFT collection.

    The main ChessCombo account will also have its own Combo collections
    it can use to hold its own Combos that have not yet been sent to a user. (For example when sending packs)

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract ChessCombo: NonFungibleToken {

    // -----------------------------------------------------------------------
    // ChessCombo contract Events
    // -----------------------------------------------------------------------

    // Emitted when the ChessCombo contract is created
    pub event ContractInitialized()

    // Emitted when a new Combination struct is created
    pub event CombinationCreated(id: UInt32, metadata: {String:String})
    // Emitted when a new series has been triggered by an admin
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Compilation-Related actions
    //
    // Emitted when a new Compilation is created
    pub event CompilationCreated(compilationId: UInt32, series: UInt32, name: String)
    // Emitted when a new Combination is added to a Compilation
    pub event CombinationAddedToCompilation(compilationId: UInt32, combinationId: UInt32)
    // Emitted when a Combination is retired from a Compilation and cannot be used to mint
    pub event CombinationRetiredFromCompilation(compilationId: UInt32, combinationId: UInt32, numCombos: UInt32)
    // Emitted when a Compilation is locked, meaning Combination cannot be added
    pub event CompilationLocked(compilationId: UInt32)
    // Emitted when a Combo is minted from a Compilation
    pub event ComboMinted(comboId: UInt64, combinationId: UInt32, compilationId: UInt32, serialNumber: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a combo is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a combo is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Combo is destroyed
    pub event ComboDestroyed(id: UInt64)

    //PATHS
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let ChessComboAdminStoragePath: StoragePath
    // -----------------------------------------------------------------------
    // ChessCombo contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Series that newly created Compilation belongs to.
    // Series is a concept that indicates a group of Compilations through time.
    // Many Compilations can exist at a time, but only one series.
    pub var currentSeries: UInt32

    // Variable size dictionary of CombinationData structs
    access(self) var combinationDatas: {UInt32: CombinationData}

    // Variable size dictionary of CompilationData structs
    access(self) var compilationDatas: {UInt32: CompilationData}

    // Variable size dictionary of Compilation resources
    access(self) var compilations: @{UInt32: Compilation}

    // The ID that is used to create Combination. 
    pub var nextCombinationId: UInt32

    // The ID that is used to create Compilations. Every time a Compilation is created
    // compilationId is assigned to the new Compilation's Id and then is incremented by 1.
    pub var nextCompilationId: UInt32

    // The total number of ChessCombo Combo NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global combo ID for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // ChessCombo contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // CombinationData is a Struct that holds metadata associated 
    // with a specific chess combination.
    //
    // Combo NFTs will all reference a single Combination as the owner of
    // its metadata. The Combinations are publicly accessible, so anyone can
    // read the metadata associated with a specific Combination ID
    //
    pub struct CombinationData {

        pub let id: UInt32

        // Stores all the metadata about the Combination as a string mapping
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            self.id = ChessCombo.nextCombinationId
            self.metadata = metadata

            // Increment the ID so that it isn't used again
            ChessCombo.nextCombinationId = ChessCombo.nextCombinationId + (1 as UInt32)

            emit CombinationCreated(id: self.id, metadata: metadata)
        }
    }

    // A Compilation is a grouping of Combinations that have occured in the real world.
    // A Combination can exist in multiple different Compilations.
    // 
    // CompilationData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a compilation by calling various getters located 
    // at the end of the contract. Only the admin has the ability 
    // to modify any data in the private Compilation resource.
    //
    pub struct CompilationData {

        pub let id: UInt32

        // Name of the Compilation
        // ex. "Queen Sacrifices"
        pub let name: String

        // Series that this Compilation belongs to.
        // Series is a concept that indicates a group of Compilations through time.
        // Many Compilations can exist at a time, but only one series.
        pub let series: UInt32

        init(name: String) {
            pre {
                name.length > 0: "New Compilation name cannot be empty"
                 ChessCombo.currentSeries > 0: "Series must be created"
            }
            self.id = ChessCombo.nextCompilationId
            self.name = name
            self.series = ChessCombo.currentSeries

            // Increment the CompilationId so that it isn't used again
            ChessCombo.nextCompilationId = ChessCombo.nextCompilationId + (1 as UInt32)

            emit CompilationCreated(compilationId: self.id, series: self.series, name: self.name)
        }
    }

    // Compilation is a resource type that contains the functions to add and remove
    // Combinations from a Compilation and mint Combos.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Combinations to a Compilation so that the Compilation can mint Combos
    // that reference that combination data.
    // The Combos that are minted by a Combination will be listed as belonging to
    // the Compilation that minted it, as well as the CombinationData it references.
    // 
    // Admin can also retire Combinations from the Compilation, meaning that the retired
    // Combination can no longer have Combos minted from it for this Compilation.
    //
    // If the admin locks the Compilation, no more Combinations can be added to it, but 
    // Combos can still be minted.
    //
    // If retireAll() and lock() are called back-to-back, 
    // the Compilation is closed off forever and nothing more can be done with it.
    pub resource Compilation {

        pub let id: UInt32

        // Array of Combination Ids that are a part of this Compilation.
        // When a Combination is added to the Compilation, its Id gets appended here.
        // The Id does not get removed from this array when a Combination is retired.
        pub var combinationIds: [UInt32]

        // Map of Combination Ids that Indicates if a Combination in this Compilation is retired.
        // When a Combination is retired, this is set to true and cannot be changed.
        pub var retired: {UInt32: Bool}

        // Indicates if the Compilation is currently locked.
        // When a Compilation is locked, Combinations cannot be added.
        // A Compilation can never be changed from locked to unlocked.
        pub var locked: Bool

        // Mapping of Combination Ids that indicates the number of Combos 
        // that have been minted for specific Combination in this Compilation.        
        pub var numberMintedPerCombination: {UInt32: UInt32}

        init(name: String) {
            self.id = ChessCombo.nextCompilationId
            self.combinationIds = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerCombination = {}

            // Create a new CompilationData for this Compilation and store it in contract storage
            ChessCombo.compilationDatas[self.id] = CompilationData(name: name)
        }

        // addCombination adds a Combination to the Compilation
        //
        // Pre-Conditions:
        // The Combination needs to be existing
        // The Compilation must not be locked
        // The Combination must not already exist in the Compilation
        //
        pub fun addCombination(combinationId: UInt32) {
            pre {
                ChessCombo.combinationDatas[combinationId] != nil: "Cannot add the Combination to Compilation: Combination doesn't exist."
                !self.locked: "Cannot add the Combination to the Compilation after the Compilation has been locked."
                self.numberMintedPerCombination[combinationId] == nil: "The Combination has already beed added to the Compilation."
            }

            // Add the Combination to the array of CombinationIds
            self.combinationIds.append(combinationId)

            // Open the Combination up for minting
            self.retired[combinationId] = false

            // Initialize the Combo count to zero
            self.numberMintedPerCombination[combinationId] = 0

            emit CombinationAddedToCompilation(compilationId: self.id, combinationId: combinationId)
        }

        // addCombinations adds multiple Combinations to the Compilation
        //
        pub fun addCombinations(combinationIds: [UInt32]) {
            for combination in combinationIds {
                self.addCombination(combinationId: combination)
            }
        }

        // retireCombination retires a Combination from the Compilation so that it can't mint new Combos
        //
        // Pre-Conditions:
        // The Combination is part of the Compilation and not retired (available for minting).
        // 
        pub fun retireCombination(combinationId: UInt32) {
            pre {
                self.retired[combinationId] != nil: "Cannot retire the Combination: Combination doesn't exist in this Compilation!"
            }

            if !self.retired[combinationId]! {
                self.retired[combinationId] = true

                emit CombinationRetiredFromCompilation(
                compilationId: self.id, 
                combinationId: combinationId, 
                numCombos: self.numberMintedPerCombination[combinationId]!)
            }
        }

        // retireAll retires all the Combinations in the Compilation
        // Afterwards, none of the retired Combinations will be able to mint new Combos
        //
        pub fun retireAll() {
            for id in self.combinationIds {
                self.retireCombination(combinationId: id)
            }
        }

        // lock() locks the Compilation so that no more Combinations can be added to it
        //
        // Pre-Conditions:
        // The Compilation should not be locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit CompilationLocked(compilationId: self.id)
            }
        }

        // mintCombo mints a new Combo and returns the newly minted Combo                
        //
        // Pre-Conditions:
        // The Combination must exist in the Compilation and be allowed to mint new Combos
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintCombo(combinationId: UInt32): @NFT {
            pre {
                self.retired[combinationId] != nil: "Cannot mint the combo: This Combination doesn't exist."
                !self.retired[combinationId]!: "Cannot mint the Combo from this Combination: This Combination has been retired."
            }

            // Gets the number of Combos that have been minted for this Combination
            // to use as this Combo's serial number
            let combinationMintsCount = self.numberMintedPerCombination[combinationId]!

            // Mint the new Combo
            let newCombo: @NFT <- create NFT(serialNumber: combinationMintsCount + (1 as UInt32),
                                              combinationId: combinationId,
                                              compilationId: self.id)

            // Increment the count of Combos minted for this Combination
            self.numberMintedPerCombination[combinationId] = combinationMintsCount + (1 as UInt32)

            return <- newCombo
        }

        // batchMintCombos mints an arbitrary quantity of Combos 
        // and returns them as a Collection
        //
        // Parameters: combinationId: the Id of the Combination that the Combos are minted for
        //             quantity: The quantity of Combos to be minted
        //
        // Returns: Collection object that contains all the Combos that were minted
        //
        pub fun batchMintCombos(combinationId: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintCombo(combinationId: combinationId))
                i = i + (1 as UInt64)
            }

            return <-newCollection
        }
    }

    pub struct ComboData {

        // The ID of the Compilation that the Combo comes from
        pub let compilationId: UInt32

        // The ID of the Combination that the Combo references
        pub let combinationId: UInt32
        
        // Serial number of the Combo
        pub let serialNumber: UInt32

        init(compilationId: UInt32, combinationId: UInt32, serialNumber: UInt32) {
            self.compilationId = compilationId
            self.combinationId = combinationId
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the Combo NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Combo ID
        pub let id: UInt64
        
        // Struct of Combo metadata
        pub let data: ComboData

        init(serialNumber: UInt32, combinationId: UInt32, compilationId: UInt32) {

            ChessCombo.totalSupply = ChessCombo.totalSupply + (1 as UInt64)

            self.id = ChessCombo.totalSupply

            // Set the metadata struct
            self.data = ComboData(compilationId: compilationId, combinationId: combinationId, serialNumber: serialNumber)

            emit ComboMinted(comboId: self.id, combinationId: combinationId, compilationId: self.data.compilationId, serialNumber: self.data.serialNumber)
        }

        // If the Combo is destroyed, emit an event to indicate 
        // to outside observers that it has been destroyed
        destroy() {
            emit ComboDestroyed(id: self.id)
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Combinations, Compilations, and Combos
    //
    pub resource Admin {

        // createCombination creates a new Combination struct 
        // and stores it in the Combinations dictionary in the ChessCombo smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"WhitePlayerName": "Magnus Carlsen", "BlackPlayerName": "Hikaru Nakamura"}                                       
        //
        // Returns: the ID of the new Combination object
        //
        pub fun createCombination(metadata: {String: String}): UInt32 {
            // Create the new Combination
            var newCombination = CombinationData(metadata: metadata)
            let newId = newCombination.id

            // Store it in the contract storage
            ChessCombo.combinationDatas[newId] = newCombination

            return newId
        }

        // createCompilation creates a new Compilation resource and stores it
        // in the Compilations mapping in the ChessCombo contract
        //
        // Parameters: name: The name of the Compilation
        //
        pub fun createCompilation(name: String): UInt32 {
            // Create the new Compilation
            var newCompilation <- create Compilation(name: name)
            let newId = newCompilation.id
            // Store it in the Compilation mapping field
            ChessCombo.compilations[newId] <-! newCompilation

            return newId
        }

        // borrowCompilation returns a reference to a Compilation in the ChessCombo
        // contract so that the admin can call methods on it
        //
        // Parameters: compilationId: The Id of the Compilation that you want to
        // get a reference to
        //
        // Returns: A reference to the Compilation with all of the fields
        // and methods exposed
        //
        pub fun borrowCompilation(compilationId: UInt32): &Compilation {
            pre {
                ChessCombo.compilations[compilationId] != nil: "Cannot borrow Compilation: The Compilation doesn't exist"
            }
            
            return &ChessCombo.compilations[compilationId] as &Compilation
        }

        // startNewSeries ends the current series by incrementing
        // the series number, meaning that Combos minted after this
        // will use the new series number
        //
        // Returns: The new series number
        //
        pub fun startNewSeries(): UInt32 {
            // End the current series and start a new one
            // by incrementing the ChessCombo series number
            ChessCombo.currentSeries = ChessCombo.currentSeries + (1 as UInt32)

            emit NewSeriesStarted(newCurrentSeries: ChessCombo.currentSeries)

            return ChessCombo.currentSeries
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Combo Collection as
    // to allow others to deposit Combos into their Collection. It also allows for reading
    // the IDs of Combos in the Collection.
    pub resource interface ComboCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCombo(id: UInt64): &ChessCombo.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Combo reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTs
    //
    pub resource Collection: ComboCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Combo conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes a Combo from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Combo does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn combos
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

        // deposit takes a Combo and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a ChessCombo NFT to make sure
            // it is the correct type
            let token <- token as! @ChessCombo.NFT

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

        // borrowNFT Returns a borrowed reference to a Combo in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any ChessCombo specific data. Please use borrowCombo to 
        // read Combo data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowCombo returns a borrowed reference to a Combo
        // so that the caller can read data and call methods from it.
        // They can use this to read its compilationId, combinationId, serialNumber,
        // or any of the CompilationData or Combination data associated with it by
        // getting the compilationId or combinationId and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowCombo(id: UInt64): &ChessCombo.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ChessCombo.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // ChessCombo contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Combos in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create ChessCombo.Collection()
    }

    // getAllCombinations returns all the Combinations in ChessCombo
    //
    // Returns: An array of all the CombinationDatas that have been created
    pub fun getAllCombinations(): [ChessCombo.CombinationData] {
        return ChessCombo.combinationDatas.values
    }

    // getCombinationMetaData returns all the metadata associated with a specific Combination
    // 
    // Parameters: combinationId: The id of the Combination that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getCombinationMetaData(combinationId: UInt32): {String: String}? {
        return self.combinationDatas[combinationId]?.metadata
    }

    // getCombinationMetaDataByField returns the metadata associated with a 
    //                        specific field of the metadata
    //                        Ex: field: "WhitePlayer" will return something
    //                        like "Magnus Carlsen"
    // 
    // Parameters: combinationId: The id of the Combination that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getCombinationMetaDataByField(combinationId: UInt32, field: String): String? {
        if let combinationData = ChessCombo.combinationDatas[combinationId] {
            return combinationData.metadata[field]
        } else {
            return nil
        }
    }

    // getCompilationName returns the name that the specified Compilation
    //            is associated with.
    // 
    // Parameters: compilationId: The id of the Compilation that is being searched
    //
    // Returns: The name of the Compilation
    pub fun getCompilationName(compilationId: UInt32): String? {
        return ChessCombo.compilationDatas[compilationId]?.name
    }

    // getCompilationSeries returns the series that the specified Compilation
    //              is associated with.
    // 
    // Parameters: compilationId: The id of the Compilation that is being searched
    //
    // Returns: The series that the Compilation belongs to
    pub fun getCompilationSeries(compilationId: UInt32): UInt32? {
        return ChessCombo.compilationDatas[compilationId]?.series
    }

    // getCompilationIdsByName returns the Ids that the specified Compilation name
    //                 is associated with.
    // 
    // Parameters: compilationName: The name of the Compilation that is being searched
    //
    // Returns: An array of the Ids of the Compilation if it exists, or nil if doesn't
    pub fun getCompilationIdsByName(compilationName: String): [UInt32]? {
        var compilationIds: [UInt32] = []

        // Iterate through all the compilationDatas and search for the name
        for compilationData in ChessCombo.compilationDatas.values {
            if compilationName == compilationData.name {
                // If the name is found add the Id to list that we will return
                compilationIds.append(compilationData.id)
            }
        }

        // If the name isn't found, return nil
        if compilationIds.length == 0 {
            return nil
        } else {
            return compilationIds
        }
    }

    // getCombinationsInCompilation returns the list of Combination Ids that are in the Compilation
    // 
    // Parameters: compilationId: The id of the Compilation that is being searched
    //
    // Returns: An array of Combination Ids
    pub fun getCombinationsInCompilation(compilationId: UInt32): [UInt32]? {
        return ChessCombo.compilations[compilationId]?.combinationIds
    }

    // isEditionRetired returns a boolean that indicates if a Compilation/Combination
    //                  edition is retired.
    //                  If an edition is retired, it still remains in the Compilation,
    //                  but Combos can no longer be minted from it.
    // 
    // Parameters: compilationId: The id of the Compilation that is being searched
    //             combinationId: The id of the Combination that is being searched
    //
    // Returns: Boolean indicating if the edition is retired or not
    pub fun isEditionRetired(compilationId: UInt32, combinationId: UInt32): Bool? {
        // Remove the compilation from the dictionary to get its field
        if let compilationToRead <- ChessCombo.compilations.remove(key: compilationId) {

            // See if the Combination is retired from this Compilation
            let retired = compilationToRead.retired[combinationId]

            // Put the Compilation back in the contract storage
            ChessCombo.compilations[compilationId] <-! compilationToRead

            // Return the retired status
            return retired
        } else {
            return nil
        }
    }

    // isCompilationLocked returns a boolean that indicates if a Compilation
    //             is locked. If it's locked, 
    //             new Combinations can no longer be added to it,
    //             but Combos can still be minted from Combinations the Compilation contains.
    // 
    // Parameters: compilationId: The id of the Compilation that is being searched
    //
    // Returns: Boolean indicating if the Compilation is locked or not
    pub fun isCompilationLocked(compilationId: UInt32): Bool? {
        return ChessCombo.compilations[compilationId]?.locked
    }


    // Parameters: compilationId: The id of the Compilation that is being searched
    //             combinationId: The id of the Combination that is being searched
    //
    // Returns: The total number of Combos 
    //          that have been minted from an edition
    pub fun getNumCombosInEdition(compilationId: UInt32, combinationId: UInt32): UInt32? {

        // Remove the Compilation from the dictionary to get its field
        if let compilationToRead <- ChessCombo.compilations.remove(key: compilationId) {

            // Read the numMintedPerCombination
            let amount = compilationToRead.numberMintedPerCombination[combinationId]

            // Put the Compilation back into the Compilations dictionary
            ChessCombo.compilations[compilationId] <-! compilationToRead

            return amount
        } else {
            // If the Compilation wasn't found return nil
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // ChessCombo initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.currentSeries = 0
        self.combinationDatas = {}
        self.compilationDatas = {}
        self.compilations <- {}
        self.nextCombinationId = 1
        self.nextCompilationId = 1
        self.totalSupply = 0

        //Initialize storage paths
        self.CollectionStoragePath = /storage/ChessComboCollection
        self.CollectionPublicPath = /public/ChessComboCollection
        self.ChessComboAdminStoragePath = /storage/ChessComboAdmin

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{ComboCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.ChessComboAdminStoragePath)

        emit ContractInitialized()
    }
}
 

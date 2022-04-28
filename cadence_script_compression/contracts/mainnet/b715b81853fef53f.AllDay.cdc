/*
    Adapted from: Genies.cdc
    Author: Rhea Myers rhea.myers@dapperlabs.com
    Author: Sadie Freeman sadie.freeman@dapperlabs.com
*/


import NonFungibleToken from 0x1d7e57aa55817448

/*
    AllDay is structured similarly to Genies and TopShot.
    Unlike TopShot, we use resources for all entities and manage access to their data
    by copying it to structs (this simplifies access control, in particular write access).
    We also encapsulate resource creation for the admin in member functions on the parent type.
    
    There are 5 levels of entity:
    1. Series
    2. Sets
    3. Plays
    4. Editions
    4. Moment NFT (an NFT)
    
    An Edition is created with a combination of a Series, Set, and Play
    Moment NFTs are minted out of Editions.

    Note that we cache some information (Series names/ids, counts of entities) rather
    than calculate it each time.
    This is enabled by encapsulation and saves gas for entity lifecycle operations.
 */

// The AllDay NFTs and metadata contract
//
pub contract AllDay: NonFungibleToken {
    //------------------------------------------------------------
    // Events
    //------------------------------------------------------------

    // Contract Events
    //
    pub event ContractInitialized()

    // NFT Collection Events
    //
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // Series Events
    //
    // Emitted when a new series has been created by an admin
    pub event SeriesCreated(id: UInt64, name: String)
    // Emitted when a series is closed by an admin
    pub event SeriesClosed(id: UInt64)

    // Set Events
    //
    // Emitted when a new set has been created by an admin
    pub event SetCreated(id: UInt64, name: String)

    // Play Events
    //
    // Emitted when a new play has been created by an admin
    pub event PlayCreated(id: UInt64, classification: String, metadata: {String: String})

    // Edition Events
    //
    // Emitted when a new edition has been created by an admin
    pub event EditionCreated(
        id: UInt64, 
        seriesID: UInt64, 
        setID: UInt64, 
        playID: UInt64, 
        maxMintSize: UInt64?,
        tier: String,
    )
    // Emitted when an edition is either closed by an admin, or the max amount of moments have been minted
    pub event EditionClosed(id: UInt64)

    // NFT Events
    //
    pub event MomentNFTMinted(id: UInt64, editionID: UInt64, serialNumber: UInt64)
    pub event MomentNFTBurned(id: UInt64)

    //------------------------------------------------------------
    // Named values
    //------------------------------------------------------------

    // Named Paths
    //
    pub let CollectionStoragePath:  StoragePath
    pub let CollectionPublicPath:   PublicPath
    pub let AdminStoragePath:       StoragePath
    pub let MinterPrivatePath:      PrivatePath

    //------------------------------------------------------------
    // Publicly readable contract state
    //------------------------------------------------------------

    // Entity Counts
    //
    pub var totalSupply:        UInt64
    pub var nextSeriesID:       UInt64
    pub var nextSetID:          UInt64
    pub var nextPlayID:         UInt64
    pub var nextEditionID:      UInt64

    //------------------------------------------------------------
    // Internal contract state
    //------------------------------------------------------------

    // Metadata Dictionaries
    //
    // This is so we can find Series by their names (via seriesByID)
    access(self) let seriesIDByName:    {String: UInt64}
    access(self) let seriesByID:        @{UInt64: Series}
    access(self) let setIDByName:       {String: UInt64}
    access(self) let setByID:           @{UInt64: Set}
    access(self) let playByID:          @{UInt64: Play}
    access(self) let editionByID:       @{UInt64: Edition}

    //------------------------------------------------------------
    // Series
    //------------------------------------------------------------

    // A public struct to access Series data
    //
    pub struct SeriesData {
        pub let id: UInt64
        pub let name: String
        pub let active: Bool

        // initializer
        //
        init (id: UInt64) {
            let series = &AllDay.seriesByID[id] as! &AllDay.Series
            self.id = series.id
            self.name = series.name
            self.active = series.active
        }
    }

    // A top-level Series with a unique ID and name
    //
    pub resource Series {
        pub let id: UInt64
        pub let name: String
        pub var active: Bool

        // Close this series
        //
        pub fun close() {
            pre {
                self.active == true: "not active"
            }

            self.active = false

            emit SeriesClosed(id: self.id)
        }

        // initializer
        //
        init (name: String) {
            pre {
                !AllDay.seriesIDByName.containsKey(name): "A Series with that name already exists"
            }
            self.id = AllDay.nextSeriesID
            self.name = name
            self.active = true   

            // Cache the new series's name => ID
            AllDay.seriesIDByName[name] = self.id
            // Increment for the nextSeriesID
            AllDay.nextSeriesID = self.id + 1 as UInt64

            emit SeriesCreated(id: self.id, name: self.name)
        }
    }

    // Get the publicly available data for a Series by id
    //
    pub fun getSeriesData(id: UInt64): AllDay.SeriesData {
        pre {
            AllDay.seriesByID[id] != nil: "Cannot borrow series, no such id"
        }

        return AllDay.SeriesData(id: id)
    }

    // Get the publicly available data for a Series by name
    //
    pub fun getSeriesDataByName(name: String): AllDay.SeriesData {
        pre {
            AllDay.seriesIDByName[name] != nil: "Cannot borrow series, no such name"
        }

        let id = AllDay.seriesIDByName[name]!

        return AllDay.SeriesData(id: id)
    }

    // Get all series names (this will be *long*)
    //
    pub fun getAllSeriesNames(): [String] {
        return AllDay.seriesIDByName.keys
    }

    // Get series id for name
    //
    pub fun getSeriesIDByName(name: String): UInt64? {
        return AllDay.seriesIDByName[name]
    }

    //------------------------------------------------------------
    // Set
    //------------------------------------------------------------

    // A public struct to access Set data
    //
    pub struct SetData {
        pub let id: UInt64
        pub let name: String
        pub var setPlaysInEditions: {UInt64: Bool}

        // member function to check the setPlaysInEditions to see if this Set/Play combination already exists
        pub fun setPlayExistsInEdition(playID: UInt64): Bool {
           return self.setPlaysInEditions.containsKey(playID)
        }

        // initializer
        //
        init (id: UInt64) {
            let set = &AllDay.setByID[id] as! &AllDay.Set
            self.id = id
            self.name = set.name
            self.setPlaysInEditions = set.setPlaysInEditions
        }
    }

    // A top level Set with a unique ID and a name
    //
    pub resource Set {
        pub let id: UInt64
        pub let name: String
        // Store a dictionary of all the Plays which are paired with the Set inside Editions
        // This enforces only one Set/Play unique pair can be used for an Edition
        pub var setPlaysInEditions: {UInt64: Bool}

        // member function to insert a new Play to the setPlaysInEditions dictionary
        pub fun insertNewPlay(playID: UInt64) {
            self.setPlaysInEditions[playID] = true
        }

        // initializer
        //
        init (name: String) {
            pre {
                !AllDay.setIDByName.containsKey(name): "A Set with that name already exists"
            }
            self.id = AllDay.nextSetID
            self.name = name
            self.setPlaysInEditions = {}

            // Cache the new set's name => ID
            AllDay.setIDByName[name] = self.id
            // Increment for the nextSeriesID
            AllDay.nextSetID = self.id + 1 as UInt64

            emit SetCreated(id: self.id, name: self.name)
        }
    }

    // Get the publicly available data for a Set
    //
    pub fun getSetData(id: UInt64): AllDay.SetData {
        pre {
            AllDay.setByID[id] != nil: "Cannot borrow set, no such id"
        }

        return AllDay.SetData(id: id)
    }

    // Get the publicly available data for a Set by name
    //
    pub fun getSetDataByName(name: String): AllDay.SetData {
        pre {
            AllDay.setIDByName[name] != nil: "Cannot borrow set, no such name"
        }

        let id = AllDay.setIDByName[name]!

        return AllDay.SetData(id: id)
    }

    // Get all set names (this will be *long*)
    //
    pub fun getAllSetNames(): [String] {
        return AllDay.setIDByName.keys
    }


    //------------------------------------------------------------
    // Play
    //------------------------------------------------------------

    // A public struct to access Play data
    //
    pub struct PlayData {
        pub let id: UInt64
        pub let classification: String
        pub let metadata: {String: String}

        // initializer
        //
        init (id: UInt64) {
            let play = &AllDay.playByID[id] as! &AllDay.Play
            self.id = id
            self.classification = play.classification
            self.metadata = play.metadata
        }
    }

    // A top level Play with a unique ID and a classification
    //
    pub resource Play {
        pub let id: UInt64
        pub let classification: String
        // Contents writable if borrowed!
        // This is deliberate, as it allows admins to update the data.
        pub let metadata: {String: String}

        // initializer
        //
        init (classification: String, metadata: {String: String}) {
            self.id = AllDay.nextPlayID
            self.classification = classification
            self.metadata = metadata

            AllDay.nextPlayID = self.id + 1 as UInt64

            emit PlayCreated(id: self.id, classification: self.classification, metadata: self.metadata)
        }
    }

    // Get the publicly available data for a Play
    //
    pub fun getPlayData(id: UInt64): AllDay.PlayData {
        pre {
            AllDay.playByID[id] != nil: "Cannot borrow play, no such id"
        }

        return AllDay.PlayData(id: id)
    }

    //------------------------------------------------------------
    // Edition
    //------------------------------------------------------------

    // A public struct to access Edition data
    //
    pub struct EditionData {
        pub let id: UInt64
        pub let seriesID: UInt64
        pub let setID: UInt64
        pub let playID: UInt64
        pub var maxMintSize: UInt64?
        pub let tier: String
        pub var numMinted: UInt64

       // member function to check if max edition size has been reached
       pub fun maxEditionMintSizeReached(): Bool {
            return self.numMinted == self.maxMintSize 
        }

        // initializer
        //
        init (id: UInt64) {
            let edition = &AllDay.editionByID[id] as! &AllDay.Edition
            self.id = id
            self.seriesID = edition.seriesID
            self.playID = edition.playID
            self.setID = edition.setID
            self.maxMintSize = edition.maxMintSize
            self.tier = edition.tier
            self.numMinted = edition.numMinted
        }
    }

    // A top level Edition that contains a Series, Set, and Play
    //
    pub resource Edition {
        pub let id: UInt64
        pub let seriesID: UInt64
        pub let setID: UInt64
        pub let playID: UInt64
        pub let tier: String
        // Null value indicates that there is unlimited minting potential for the Edition
        pub var maxMintSize: UInt64?
        // Updates each time we mint a new moment for the Edition to keep a running total
        pub var numMinted: UInt64

        // Close this edition so that no more Moment NFTs can be minted in it
        //
        access(contract) fun close() {
            pre {
                self.numMinted != self.maxMintSize: "max number of minted moments has already been reached"
            }

            self.maxMintSize = self.numMinted

            emit EditionClosed(id: self.id)
        }

        // Mint a Moment NFT in this edition, with the given minting mintingDate.
        // Note that this will panic if the max mint size has already been reached.
        //
        pub fun mint(): @AllDay.NFT {
            pre {
                self.numMinted != self.maxMintSize: "max number of minted moments has been reached"
            }

            // Create the Moment NFT, filled out with our information
            let momentNFT <- create NFT(
                id: AllDay.totalSupply + 1,
                editionID: self.id,
                serialNumber: self.numMinted + 1
            )
            AllDay.totalSupply = AllDay.totalSupply + 1
            // Keep a running total (you'll notice we used this as the serial number)
            self.numMinted = self.numMinted + 1 as UInt64

            return <- momentNFT
        }

        // initializer
        //
        init (
            seriesID: UInt64,
            setID: UInt64,
            playID: UInt64,
            maxMintSize: UInt64?,
            tier: String,
        ) {
            pre {
                maxMintSize != 0: "max mint size is zero, must either be null or greater than 0"
                AllDay.seriesByID.containsKey(seriesID): "seriesID does not exist"
                AllDay.setByID.containsKey(setID): "setID does not exist"
                AllDay.playByID.containsKey(playID): "playID does not exist"
                SeriesData(id: seriesID).active == true: "cannot create an Edition with a closed Series"
                SetData(id: setID).setPlayExistsInEdition(playID: playID) != true: "set play combination already exists in an edition"
            }

            self.id = AllDay.nextEditionID
            self.seriesID = seriesID
            self.setID = setID
            self.playID = playID

            // If an edition size is not set, it has unlimited minting potential
            if maxMintSize == 0 {
                self.maxMintSize = nil
            } else {
                self.maxMintSize = maxMintSize
            }

            self.tier = tier
            self.numMinted = 0 as UInt64

            AllDay.nextEditionID = AllDay.nextEditionID + 1 as UInt64
            AllDay.setByID[setID]?.insertNewPlay(playID: playID)

            emit EditionCreated(
                id: self.id,
                seriesID: self.seriesID,
                setID: self.setID,
                playID: self.playID,
                maxMintSize: self.maxMintSize,
                tier: self.tier,
            )
        }
    }

    // Get the publicly available data for an Edition
    //
    pub fun getEditionData(id: UInt64): EditionData {
        pre {
            AllDay.editionByID[id] != nil: "Cannot borrow edition, no such id"
        }

        return AllDay.EditionData(id: id)
    }

    //------------------------------------------------------------
    // NFT
    //------------------------------------------------------------

    // A Moment NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let editionID: UInt64
        pub let serialNumber: UInt64
        pub let mintingDate: UFix64

        // Destructor
        //
        destroy() {
            emit MomentNFTBurned(id: self.id)
        }

        // NFT initializer
        //
        init(
            id: UInt64,
            editionID: UInt64,
            serialNumber: UInt64
        ) {
            pre {
                AllDay.editionByID[editionID] != nil: "no such editionID"
                EditionData(id: editionID).maxEditionMintSizeReached() != true: "max edition size already reached"
            }

            self.id = id
            self.editionID = editionID
            self.serialNumber = serialNumber
            self.mintingDate = getCurrentBlock().timestamp

            emit MomentNFTMinted(id: self.id, editionID: self.editionID, serialNumber: self.serialNumber)
        }
    }

    //------------------------------------------------------------
    // Collection
    //------------------------------------------------------------

    // A public collection interface that allows Moment NFTs to be borrowed
    //
    pub resource interface MomentNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMomentNFT(id: UInt64): &AllDay.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Moment NFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // An NFT Collection
    //
    pub resource Collection:
        NonFungibleToken.Provider,
        NonFungibleToken.Receiver,
        NonFungibleToken.CollectionPublic,
        MomentNFTCollectionPublic
    {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an UInt64 ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @AllDay.NFT
            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        //
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

        // getIDs returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowMomentNFT gets a reference to an NFT in the collection
        //
        pub fun borrowMomentNFT(id: UInt64): &AllDay.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &AllDay.NFT
            } else {
                return nil
            }
        }

        // Collection destructor
        //
        destroy() {
            destroy self.ownedNFTs
        }

        // Collection initializer
        //
        init() {
            self.ownedNFTs <- {}
        }
    }

    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    //------------------------------------------------------------
    // Admin
    //------------------------------------------------------------

    // An interface containing the Admin function that allows minting NFTs
    //
    pub resource interface NFTMinter {
        // Mint a single NFT
        // The Edition for the given ID must already exist
        //
        pub fun mintNFT(editionID: UInt64): @AllDay.NFT
    }

    // A resource that allows managing metadata and minting NFTs
    //
    pub resource Admin: NFTMinter {
        // Borrow a Series
        //
        pub fun borrowSeries(id: UInt64): &AllDay.Series {
            pre {
                AllDay.seriesByID[id] != nil: "Cannot borrow series, no such id"
            }

            return &AllDay.seriesByID[id] as &AllDay.Series
        }

        // Borrow a Set
        //
        pub fun borrowSet(id: UInt64): &AllDay.Set {
            pre {
                AllDay.setByID[id] != nil: "Cannot borrow Set, no such id"
            }

            return &AllDay.setByID[id] as &AllDay.Set
        }

        // Borrow a Play
        //
        pub fun borrowPlay(id: UInt64): &AllDay.Play {
            pre {
                AllDay.playByID[id] != nil: "Cannot borrow Play, no such id"
            }

            return &AllDay.playByID[id] as &AllDay.Play
        }

        // Borrow an Edition
        //
        pub fun borrowEdition(id: UInt64): &AllDay.Edition {
            pre {
                AllDay.editionByID[id] != nil: "Cannot borrow edition, no such id"
            }

            return &AllDay.editionByID[id] as &AllDay.Edition
        }

        // Create a Series
        //
        pub fun createSeries(name: String): UInt64 {
            // Create and store the new series
            let series <- create AllDay.Series(
                name: name,
            )
            let seriesID = series.id
            AllDay.seriesByID[series.id] <-! series

            // Return the new ID for convenience
            return seriesID
        }
        
        // Close a Series
        //
        pub fun closeSeries(id: UInt64): UInt64 {
            let series = &AllDay.seriesByID[id] as &AllDay.Series
            series.close()
            return series.id
        }

        // Create a Set
        //
        pub fun createSet(name: String): UInt64 {
            // Create and store the new set
            let set <- create AllDay.Set(
                name: name,
            )
            let setID = set.id
            AllDay.setByID[set.id] <-! set

            // Return the new ID for convenience
            return setID
        }

        // Create a Play
        //
        pub fun createPlay(classification: String, metadata: {String: String}): UInt64 {
            // Create and store the new play
            let play <- create AllDay.Play(
                classification: classification,
                metadata: metadata,
            )
            let playID = play.id
            AllDay.playByID[play.id] <-! play

            // Return the new ID for convenience
            return playID
        }

        // Create an Edition
        //
        pub fun createEdition(            
            seriesID: UInt64,
            setID: UInt64,
            playID: UInt64,
            maxMintSize: UInt64?,
            tier: String): UInt64 {
            let edition <- create Edition(
                seriesID: seriesID,
                setID: setID,
                playID: playID,
                maxMintSize: maxMintSize,
                tier: tier,
            )
            let editionID = edition.id
            AllDay.editionByID[edition.id] <-! edition

            return editionID
        }

        // Close an Edition
        //
        pub fun closeEdition(id: UInt64): UInt64 {
            let edition = &AllDay.editionByID[id] as &AllDay.Edition
            edition.close()
            return edition.id
        }

        // Mint a single NFT
        // The Edition for the given ID must already exist
        //
        pub fun mintNFT(editionID: UInt64): @AllDay.NFT {
            pre {
                // Make sure the edition we are creating this NFT in exists
                AllDay.editionByID.containsKey(editionID): "No such EditionID"
            }

            return <- self.borrowEdition(id: editionID).mint()
        }
    }

    //------------------------------------------------------------
    // Contract lifecycle
    //------------------------------------------------------------

    // AllDay contract initializer
    //
    init() {
        // Set the named paths
        self.CollectionStoragePath = /storage/AllDayNFTCollection
        self.CollectionPublicPath = /public/AllDayNFTCollection
        self.AdminStoragePath = /storage/AllDayAdmin
        self.MinterPrivatePath = /private/AllDayMinter

        // Initialize the entity counts
        self.totalSupply = 0
        self.nextSeriesID = 1
        self.nextSetID = 1
        self.nextPlayID = 1
        self.nextEditionID = 1

        // Initialize the metadata lookup dictionaries
        self.seriesByID <- {}
        self.seriesIDByName = {}
        self.setIDByName = {}
        self.setByID <- {}
        self.playByID <- {}
        self.editionByID <- {}

        // Create an Admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)
        // Link capabilites to the admin constrained to the Minter
        // and Metadata interfaces
        self.account.link<&AllDay.Admin{AllDay.NFTMinter}>(
            self.MinterPrivatePath,
            target: self.AdminStoragePath
        )

        // Let the world know we are here
        emit ContractInitialized()
    }
}
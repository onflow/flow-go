/*
    Description: Central Smart Contract for NBA TopShot

    This smart contract contains the core functionality for 
    NBA Top Shot, created by Dapper Labs

    The contract manages the data associated with all the plays and sets
    that are used as templates for the Moment NFTs

    When a new Play wants to be added to the records, an Admin creates
    a new Play struct that is stored in the smart contract.

    Then an Admin can create new Sets. Sets consist of a public struct that 
    contains public information about a set, and a private resource used
    to mint new moments based off of plays that have been linked to the Set.

    The admin resource has the power to do all of the important actions
    in the smart contract. When admins want to call functions in a set,
    they call their borrowSet function to get a reference 
    to a set in the contract. Then, they can call functions on the set using that reference.

    In this way, the smart contract and its defined resources interact 
    with great teamwork, just like the Indiana Pacers, the greatest NBA team
    of all time.
    
    When moments are minted, they are initialized with a MomentData struct and
    are returned by the minter.

    The contract also defines a Collection resource. This is an object that 
    every TopShot NFT owner will store in their account
    to manage their NFT collection.

    The main Top Shot account will also have its own Moment collections
    it can use to hold its own moments that have not yet been sent to a user.

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.

    It is also important to remember that 
    The Golden State Warriors blew a 3-1 lead in the 2016 NBA finals.

*/

import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract TopShot: NonFungibleToken {

    // -----------------------------------------------------------------------
    // TopShot contract Events
    // -----------------------------------------------------------------------

    // Emitted when the TopShot contract is created
    pub event ContractInitialized()

    // Emitted when a new Play struct is created
    pub event PlayCreated(id: UInt32, metadata: {String:String})
    // Emitted when a new series has been triggered by an admin
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Set-Related actions
    //
    // Emitted when a new Set is created
    pub event SetCreated(setID: UInt32, series: UInt32)
    // Emitted when a new Play is added to a Set
    pub event PlayAddedToSet(setID: UInt32, playID: UInt32)
    // Emitted when a Play is retired from a Set and cannot be used to mint
    pub event PlayRetiredFromSet(setID: UInt32, playID: UInt32, numMoments: UInt32)
    // Emitted when a Set is locked, meaning Plays cannot be added
    pub event SetLocked(setID: UInt32)
    // Emitted when a Moment is minted from a Set
    pub event MomentMinted(momentID: UInt64, playID: UInt32, setID: UInt32, serialNumber: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a moment is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a moment is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Moment is destroyed
    pub event MomentDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // TopShot contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Series that this Set belongs to.
    // Series is a concept that indicates a group of Sets through time.
    // Many Sets can exist at a time, but only one series.
    pub var currentSeries: UInt32

    // Variable size dictionary of Play structs
    access(self) var playDatas: {UInt32: Play}

    // Variable size dictionary of SetData structs
    access(self) var setDatas: {UInt32: SetData}

    // Variable size dictionary of Set resources
    access(self) var sets: @{UInt32: Set}

    // The ID that is used to create Plays. 
    // Every time a Play is created, playID is assigned 
    // to the new Play's ID and then is incremented by 1.
    pub var nextPlayID: UInt32

    // The ID that is used to create Sets. Every time a Set is created
    // setID is assigned to the new set's ID and then is incremented by 1.
    pub var nextSetID: UInt32

    // The total number of Top shot Moment NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global moment IDs for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // TopShot contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Play is a Struct that holds metadata associated 
    // with a specific NBA play, like the legendary moment when 
    // Ray Allen hit the 3 to tie the Heat and Spurs in the 2013 finals game 6
    // or when Lance Stephenson blew in the ear of Lebron James.
    //
    // Moment NFTs will all reference a single play as the owner of
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
            self.playID = TopShot.nextPlayID
            self.metadata = metadata
        }
    }

    // A Set is a grouping of Plays that have occured in the real world
    // that make up a related group of collectibles, like sets of baseball
    // or Magic cards. A Play can exist in multiple different sets.
    // 
    // SetData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a set by calling various getters located 
    // at the end of the contract. Only the admin has the ability 
    // to modify any data in the private Set resource.
    //
    pub struct SetData {

        // Unique ID for the Set
        pub let setID: UInt32

        // Name of the Set
        // ex. "Times when the Toronto Raptors choked in the playoffs"
        pub let name: String

        // Series that this Set belongs to.
        // Series is a concept that indicates a group of Sets through time.
        // Many Sets can exist at a time, but only one series.
        pub let series: UInt32

        init(name: String) {
            pre {
                name.length > 0: "New Set name cannot be empty"
            }
            self.setID = TopShot.nextSetID
            self.name = name
            self.series = TopShot.currentSeries
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // Plays from a set and mint Moments.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Plays to a Set so that the set can mint Moments
    // that reference that playdata.
    // The Moments that are minted by a Set will be listed as belonging to
    // the Set that minted it, as well as the Play it references.
    // 
    // Admin can also retire Plays from the Set, meaning that the retired
    // Play can no longer have Moments minted from it.
    //
    // If the admin locks the Set, no more Plays can be added to it, but 
    // Moments can still be minted.
    //
    // If retireAll() and lock() are called back-to-back, 
    // the Set is closed off forever and nothing more can be done with it.
    pub resource Set {

        // Unique ID for the set
        pub let setID: UInt32

        // Array of plays that are a part of this set.
        // When a play is added to the set, its ID gets appended here.
        // The ID does not get removed from this array when a Play is retired.
        access(contract) var plays: [UInt32]

        // Map of Play IDs that Indicates if a Play in this Set can be minted.
        // When a Play is added to a Set, it is mapped to false (not retired).
        // When a Play is retired, this is set to true and cannot be changed.
        access(contract) var retired: {UInt32: Bool}

        // Indicates if the Set is currently locked.
        // When a Set is created, it is unlocked 
        // and Plays are allowed to be added to it.
        // When a set is locked, Plays cannot be added.
        // A Set can never be changed from locked to unlocked,
        // the decision to lock a Set it is final.
        // If a Set is locked, Plays cannot be added, but
        // Moments can still be minted from Plays
        // that exist in the Set.
        pub var locked: Bool

        // Mapping of Play IDs that indicates the number of Moments 
        // that have been minted for specific Plays in this Set.
        // When a Moment is minted, this value is stored in the Moment to
        // show its place in the Set, eg. 13 of 60.
        access(contract) var numberMintedPerPlay: {UInt32: UInt32}

        init(name: String) {
            self.setID = TopShot.nextSetID
            self.plays = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerPlay = {}

            // Create a new SetData for this Set and store it in contract storage
            TopShot.setDatas[self.setID] = SetData(name: name)
        }

        // addPlay adds a play to the set
        //
        // Parameters: playID: The ID of the Play that is being added
        //
        // Pre-Conditions:
        // The Play needs to be an existing play
        // The Set needs to be not locked
        // The Play can't have already been added to the Set
        //
        pub fun addPlay(playID: UInt32) {
            pre {
                TopShot.playDatas[playID] != nil: "Cannot add the Play to Set: Play doesn't exist."
                !self.locked: "Cannot add the play to the Set after the set has been locked."
                self.numberMintedPerPlay[playID] == nil: "The play has already beed added to the set."
            }

            // Add the Play to the array of Plays
            self.plays.append(playID)

            // Open the Play up for minting
            self.retired[playID] = false

            // Initialize the Moment count to zero
            self.numberMintedPerPlay[playID] = 0

            emit PlayAddedToSet(setID: self.setID, playID: playID)
        }

        // addPlays adds multiple Plays to the Set
        //
        // Parameters: playIDs: The IDs of the Plays that are being added
        //                      as an array
        //
        pub fun addPlays(playIDs: [UInt32]) {
            for play in playIDs {
                self.addPlay(playID: play)
            }
        }

        // retirePlay retires a Play from the Set so that it can't mint new Moments
        //
        // Parameters: playID: The ID of the Play that is being retired
        //
        // Pre-Conditions:
        // The Play is part of the Set and not retired (available for minting).
        // 
        pub fun retirePlay(playID: UInt32) {
            pre {
                self.retired[playID] != nil: "Cannot retire the Play: Play doesn't exist in this set!"
            }

            if !self.retired[playID]! {
                self.retired[playID] = true

                emit PlayRetiredFromSet(setID: self.setID, playID: playID, numMoments: self.numberMintedPerPlay[playID]!)
            }
        }

        // retireAll retires all the plays in the Set
        // Afterwards, none of the retired Plays will be able to mint new Moments
        //
        pub fun retireAll() {
            for play in self.plays {
                self.retirePlay(playID: play)
            }
        }

        // lock() locks the Set so that no more Plays can be added to it
        //
        // Pre-Conditions:
        // The Set should not be locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        // mintMoment mints a new Moment and returns the newly minted Moment
        // 
        // Parameters: playID: The ID of the Play that the Moment references
        //
        // Pre-Conditions:
        // The Play must exist in the Set and be allowed to mint new Moments
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintMoment(playID: UInt32): @NFT {
            pre {
                self.retired[playID] != nil: "Cannot mint the moment: This play doesn't exist."
                !self.retired[playID]!: "Cannot mint the moment from this play: This play has been retired."
            }

            // Gets the number of Moments that have been minted for this Play
            // to use as this Moment's serial number
            let numInPlay = self.numberMintedPerPlay[playID]!

            // Mint the new moment
            let newMoment: @NFT <- create NFT(serialNumber: numInPlay + UInt32(1),
                                              playID: playID,
                                              setID: self.setID)

            // Increment the count of Moments minted for this Play
            self.numberMintedPerPlay[playID] = numInPlay + UInt32(1)

            return <-newMoment
        }

        // batchMintMoment mints an arbitrary quantity of Moments 
        // and returns them as a Collection
        //
        // Parameters: playID: the ID of the Play that the Moments are minted for
        //             quantity: The quantity of Moments to be minted
        //
        // Returns: Collection object that contains all the Moments that were minted
        //
        pub fun batchMintMoment(playID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintMoment(playID: playID))
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

    // Struct that contains all of the important data about a set
    // Can be easily queried by instantiating the `QuerySetData` object
    // with the desired set ID
    // let setData = TopShot.QuerySetData(setID: 12)
    //
    pub struct QuerySetData {
        pub let setID: UInt32
        pub let name: String
        pub let series: UInt32
        access(self) var plays: [UInt32]
        access(self) var retired: {UInt32: Bool}
        pub var locked: Bool
        access(self) var numberMintedPerPlay: {UInt32: UInt32}

        init(setID: UInt32) {
            pre {
                TopShot.sets[setID] != nil: "The set with the provided ID does not exist"
            }

            let set = &TopShot.sets[setID] as &Set
            let setData = TopShot.setDatas[setID]!

            self.setID = setID
            self.name = setData.name
            self.series = setData.series
            self.plays = set.plays
            self.retired = set.retired
            self.locked = set.locked
            self.numberMintedPerPlay = set.numberMintedPerPlay
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

    pub struct MomentData {

        // The ID of the Set that the Moment comes from
        pub let setID: UInt32

        // The ID of the Play that the Moment references
        pub let playID: UInt32

        // The place in the edition that this Moment was minted
        // Otherwise know as the serial number
        pub let serialNumber: UInt32

        init(setID: UInt32, playID: UInt32, serialNumber: UInt32) {
            self.setID = setID
            self.playID = playID
            self.serialNumber = serialNumber
        }

    }

    // This is an implementation of a custom metadata view for Top Shot.
    // This view contains the play metadata.
    //
    pub struct TopShotMomentMetadataView {

        pub let fullName: String?
        pub let firstName: String?
        pub let lastName: String?
        pub let birthdate: String?
        pub let birthplace: String?
        pub let jerseyNumber: String?
        pub let draftTeam: String?
        pub let draftYear: String?
        pub let draftSelection: String?
        pub let draftRound: String?
        pub let teamAtMomentNBAID: String?
        pub let teamAtMoment: String?
        pub let primaryPosition: String?
        pub let height: String?
        pub let weight: String?
        pub let totalYearsExperience: String?
        pub let nbaSeason: String?
        pub let dateOfMoment: String?
        pub let playCategory: String?
        pub let playType: String?
        pub let homeTeamName: String?
        pub let awayTeamName: String?
        pub let homeTeamScore: String?
        pub let awayTeamScore: String?
        pub let seriesNumber: UInt32?
        pub let setName: String?
        pub let serialNumber: UInt32
        pub let playID: UInt32
        pub let setID: UInt32
        pub let numMomentsInEdition: UInt32?

        init(
            fullName: String?,
            firstName: String?,
            lastName: String?,
            birthdate: String?,
            birthplace: String?,
            jerseyNumber: String?,
            draftTeam: String?,
            draftYear: String?,
            draftSelection: String?,
            draftRound: String?,
            teamAtMomentNBAID: String?,
            teamAtMoment: String?,
            primaryPosition: String?,
            height: String?,
            weight: String?,
            totalYearsExperience: String?,
            nbaSeason: String?,
            dateOfMoment: String?,
            playCategory: String?,
            playType: String?,
            homeTeamName: String?,
            awayTeamName: String?,
            homeTeamScore: String?,
            awayTeamScore: String?,
            seriesNumber: UInt32?,
            setName: String?,
            serialNumber: UInt32,
            playID: UInt32,
            setID: UInt32,
            numMomentsInEdition: UInt32?
        ) {
            self.fullName = fullName
            self.firstName = firstName
            self.lastName = lastName
            self.birthdate = birthdate
            self.birthplace = birthplace
            self.jerseyNumber = jerseyNumber
            self.draftTeam = draftTeam
            self.draftYear = draftYear
            self.draftSelection = draftSelection
            self.draftRound = draftRound
            self.teamAtMomentNBAID = teamAtMomentNBAID
            self.teamAtMoment = teamAtMoment
            self.primaryPosition = primaryPosition
            self.height = height
            self.weight = weight
            self.totalYearsExperience = totalYearsExperience
            self.nbaSeason = nbaSeason
            self.dateOfMoment= dateOfMoment
            self.playCategory = playCategory
            self.playType = playType
            self.homeTeamName = homeTeamName
            self.awayTeamName = awayTeamName
            self.homeTeamScore = homeTeamScore
            self.awayTeamScore = awayTeamScore
            self.seriesNumber = seriesNumber
            self.setName = setName
            self.serialNumber = serialNumber
            self.playID = playID
            self.setID = setID
            self.numMomentsInEdition = numMomentsInEdition
        }
    }

    // The resource that represents the Moment NFTs
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {

        // Global unique moment ID
        pub let id: UInt64
        
        // Struct of Moment metadata
        pub let data: MomentData

        init(serialNumber: UInt32, playID: UInt32, setID: UInt32) {
            // Increment the global Moment IDs
            TopShot.totalSupply = TopShot.totalSupply + UInt64(1)

            self.id = TopShot.totalSupply

            // Set the metadata struct
            self.data = MomentData(setID: setID, playID: playID, serialNumber: serialNumber)

            emit MomentMinted(momentID: self.id, playID: playID, setID: self.data.setID, serialNumber: self.data.serialNumber)
        }

        // If the Moment is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            emit MomentDestroyed(id: self.id)
        }

        pub fun name(): String {
            let fullName: String = TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "FullName") ?? ""
            let playType: String = TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "PlayType") ?? ""
            return fullName
                .concat(" ")
                .concat(playType)
        }

        pub fun description(): String {
            let setName: String = TopShot.getSetName(setID: self.data.setID) ?? ""
            let serialNumber: String = self.data.serialNumber.toString()
            let seriesNumber: String = TopShot.getSetSeries(setID: self.data.setID)?.toString() ?? ""
            return "A series "
                .concat(seriesNumber)
                .concat(" ")
                .concat(setName)
                .concat(" moment with serial number ")
                .concat(serialNumber)
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<TopShotMomentMetadataView>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name(),
                        description: self.description(),
                        thumbnail: MetadataViews.HTTPFile("")
                    )
                case Type<TopShotMomentMetadataView>():
                    return TopShotMomentMetadataView(
                        fullName: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "FullName"),
                        firstName: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "FirstName"),
                        lastName: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "LastName"),
                        birthdate: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "Birthdate"),
                        birthplace: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "Birthplace"),
                        jerseyNumber: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "JerseyNumber"),
                        draftTeam: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "DraftTeam"),
                        draftYear: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "DraftYear"),
                        draftSelection: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "DraftSelection"),
                        draftRound: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "DraftRound"),
                        teamAtMomentNBAID: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "TeamAtMomentNBAID"),
                        teamAtMoment: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "TeamAtMoment"),
                        primaryPosition: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "PrimaryPosition"),
                        height: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "Height"),
                        weight: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "Weight"),
                        totalYearsExperience: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "TotalYearsExperience"),
                        nbaSeason: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "NbaSeason"),
                        dateOfMoment: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "DateOfMoment"),
                        playCategory: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "PlayCategory"),
                        playType: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "PlayType"),
                        homeTeamName: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "HomeTeamName"),
                        awayTeamName: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "AwayTeamName"),
                        homeTeamScore: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "HomeTeamScore"),
                        awayTeamScore: TopShot.getPlayMetaDataByField(playID: self.data.playID, field: "AwayTeamScore"),
                        seriesNumber: TopShot.getSetSeries(setID: self.data.setID),
                        setName: TopShot.getSetName(setID: self.data.setID),
                        serialNumber: self.data.serialNumber,
                        playID: self.data.playID,
                        setID: self.data.setID,
                        numMomentsInEdition: TopShot.getNumMomentsInEdition(setID: self.data.setID, playID: self.data.playID)
                    )
            }

            return nil
        }   
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Plays, Sets, and Moments
    //
    pub resource Admin {

        // createPlay creates a new Play struct 
        // and stores it in the Plays dictionary in the TopShot smart contract
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
            TopShot.nextPlayID = TopShot.nextPlayID + UInt32(1)

            emit PlayCreated(id: newPlay.playID, metadata: metadata)

            // Store it in the contract storage
            TopShot.playDatas[newID] = newPlay

            return newID
        }

        // createSet creates a new Set resource and stores it
        // in the sets mapping in the TopShot contract
        //
        // Parameters: name: The name of the Set
        //
        // Returns: The ID of the created set
        pub fun createSet(name: String): UInt32 {

            // Create the new Set
            var newSet <- create Set(name: name)

            // Increment the setID so that it isn't used again
            TopShot.nextSetID = TopShot.nextSetID + UInt32(1)

            let newID = newSet.setID

            emit SetCreated(setID: newSet.setID, series: TopShot.currentSeries)

            // Store it in the sets mapping field
            TopShot.sets[newID] <-! newSet

            return newID
        }

        // borrowSet returns a reference to a set in the TopShot
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
                TopShot.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
            }
            
            // Get a reference to the Set and return it
            // use `&` to indicate the reference to the object and type
            return &TopShot.sets[setID] as &Set
        }

        // startNewSeries ends the current series by incrementing
        // the series number, meaning that Moments minted after this
        // will use the new series number
        //
        // Returns: The new series number
        //
        pub fun startNewSeries(): UInt32 {
            // End the current series and start a new one
            // by incrementing the TopShot series number
            TopShot.currentSeries = TopShot.currentSeries + UInt32(1)

            emit NewSeriesStarted(newCurrentSeries: TopShot.currentSeries)

            return TopShot.currentSeries
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Moment Collection as
    // to allow others to deposit Moments into their Collection. It also allows for reading
    // the IDs of Moments in the Collection.
    pub resource interface MomentCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Moment reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: MomentCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection { 
        // Dictionary of Moment conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Moment from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Moment does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn moments
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

        // deposit takes a Moment and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a TopShot NFT to make sure
            // it is the correct type
            let token <- token as! @TopShot.NFT

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

        // borrowNFT Returns a borrowed reference to a Moment in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any topshot specific data. Please use borrowMoment to 
        // read Moment data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowMoment returns a borrowed reference to a Moment
        // so that the caller can read data and call methods from it.
        // They can use this to read its setID, playID, serialNumber,
        // or any of the setData or Play data associated with it by
        // getting the setID or playID and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TopShot.NFT
            } else {
                return nil
            }
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let topShotNFT = nft as! &TopShot.NFT
            return topShotNFT as &AnyResource{MetadataViews.Resolver}
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
    // TopShot contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Moments in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TopShot.Collection()
    }

    // getAllPlays returns all the plays in topshot
    //
    // Returns: An array of all the plays that have been created
    pub fun getAllPlays(): [TopShot.Play] {
        return TopShot.playDatas.values
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
        if let play = TopShot.playDatas[playID] {
            return play.metadata[field]
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
        if TopShot.sets[setID] == nil {
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
        return TopShot.setDatas[setID]?.name
    }

    // getSetSeries returns the series that the specified Set
    //              is associated with.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: The series that the Set belongs to
    pub fun getSetSeries(setID: UInt32): UInt32? {
        // Don't force a revert if the setID is invalid
        return TopShot.setDatas[setID]?.series
    }

    // getSetIDsByName returns the IDs that the specified Set name
    //                 is associated with.
    // 
    // Parameters: setName: The name of the Set that is being searched
    //
    // Returns: An array of the IDs of the Set if it exists, or nil if doesn't
    pub fun getSetIDsByName(setName: String): [UInt32]? {
        var setIDs: [UInt32] = []

        // Iterate through all the setDatas and search for the name
        for setData in TopShot.setDatas.values {
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

    // getPlaysInSet returns the list of Play IDs that are in the Set
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: An array of Play IDs
    pub fun getPlaysInSet(setID: UInt32): [UInt32]? {
        // Don't force a revert if the setID is invalid
        return TopShot.sets[setID]?.plays
    }

    // isEditionRetired returns a boolean that indicates if a Set/Play combo
    //                  (otherwise known as an edition) is retired.
    //                  If an edition is retired, it still remains in the Set,
    //                  but Moments can no longer be minted from it.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //             playID: The id of the Play that is being searched
    //
    // Returns: Boolean indicating if the edition is retired or not
    pub fun isEditionRetired(setID: UInt32, playID: UInt32): Bool? {

        if let setdata = self.getSetData(setID: setID) {

            // See if the Play is retired from this Set
            let retired = setdata.getRetired()[playID]

            // Return the retired status
            return retired
        } else {

            // If the Set wasn't found, return nil
            return nil
        }
    }

    // isSetLocked returns a boolean that indicates if a Set
    //             is locked. If it's locked, 
    //             new Plays can no longer be added to it,
    //             but Moments can still be minted from Plays the set contains.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: Boolean indicating if the Set is locked or not
    pub fun isSetLocked(setID: UInt32): Bool? {
        // Don't force a revert if the setID is invalid
        return TopShot.sets[setID]?.locked
    }

    // getNumMomentsInEdition return the number of Moments that have been 
    //                        minted from a certain edition.
    //
    // Parameters: setID: The id of the Set that is being searched
    //             playID: The id of the Play that is being searched
    //
    // Returns: The total number of Moments 
    //          that have been minted from an edition
    pub fun getNumMomentsInEdition(setID: UInt32, playID: UInt32): UInt32? {
        if let setdata = self.getSetData(setID: setID) {

            // Read the numMintedPerPlay
            let amount = setdata.getNumberMintedPerPlay()[playID]

            return amount
        } else {
            // If the set wasn't found return nil
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // TopShot initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.currentSeries = 0
        self.playDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextPlayID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: /storage/MomentCollection)

        // Create a public capability for the Collection
        self.account.link<&{MomentCollectionPublic}>(/public/MomentCollection, target: /storage/MomentCollection)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/TopShotAdmin)

        emit ContractInitialized()
    }
}
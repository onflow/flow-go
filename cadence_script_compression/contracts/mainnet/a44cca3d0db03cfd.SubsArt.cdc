/*
    Description: Central Smart Contract for Subs “Originals” NFT

    authors: Dmytro Kabyshev dmytro@subs.tv

    This smart contract contains the core functionality for Subs “Originals” NFT
    based on NBA Top Shot, created by Dapper Labs

    -------------------
      Subs GmbH
      Bavariaring 7
      GER 80336 Munich
    -------------------

    The contract manages the data associated with all the arts and galleries
    that are used as templates for the Art NFTs

    When a new Art wants to be added to the records, an Admin creates
    a new ArtData struct that is stored in the smart contract.

    Then an Admin can create new Gallery. Gallery consist of a public struct that 
    contains public information about a gallery, and a private resource used
    to mint new NFTs based off of arts that have been linked to the Gallery.

    The admin resource has the power to do all of the important actions
    in the smart contract. When admins want to call functions in a gallery,
    they call their borrowGallery function to get a reference 
    to a gallery in the contract. Then, they can call functions on the gallery using that reference.
    
    When Arts are minted, they are initialized with a ArtData struct and
    are returned by the minter.

    The contract also defines a Collection resource. This is an object that 
    every SubsArt NFT owner will store in their account
    to manage their NFT collection.

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract SubsArt: NonFungibleToken {

    // -----------------------------------------------------------------------
    // SubsArt contract Events
    // -----------------------------------------------------------------------

    // Emitted when the SubsArt contract is created
    pub event ContractInitialized()

    // Emitted when a new Art struct is created
    pub event ArtCreated(id: UInt32, metadata: {String:String})

    // Events for Set-Related actions
    //
    // Emitted when an Art is minted
    pub event ArtNFTMinted(nftID: UInt64, artID: UInt32, galleryID: UInt32, serialNumber: UInt32)


    // Emitted when a new Gallery is created
    pub event GalleryCreated(galleryID: UInt32)
    // Emitted when a new Creator is created
    pub event CreatorCreated(creatorID: UInt32)
    // Emitted when a new Art is added to a Gallery
    pub event ArtAddedToGallery(galleryID: UInt32, artID: UInt32)
    // Emitted when an Art is concealed within a Gallery
    pub event ConcealArtInGallery(galleryID: UInt32, artID: UInt32)
    // Emitted when a Gallery is locked, meaning Arts cannot be added
    pub event GalleryLocked(galleryID: UInt32)
    // Emitted when a Gallery is unlocked, meaning Arts can be added
    pub event GalleryUnlocked(galleryID: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a art is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a art is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when an Art is destroyed
    pub event ArtNFTDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // SubsArt contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Named paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let ModeratorStoragePath: StoragePath
    pub let SubsUserStoragePath: StoragePath
    pub let SubsUserPublicPath: PublicPath

    // The ID that is used to create Arts. 
    // Every time an Art is created, artID is assigned 
    // to the new Art's ID and then is incremented by 1.
    pub var nextArtID: UInt32

    // Variable size dictionary of Art structs
    access(self) var artDatas: {UInt32: Art}

    // Variable size dictionary of Gallery resources (stored by userID from Subs users)
    access(self) var galleries: @{UInt64: Gallery}

    // The ID that is used to create Galleries. Every time a Gallery is created
    // galleryID is assigned to the new gallery's ID and then is incremented by 1.
    pub var nextGalleryID: UInt32

    // The ID that is used to create Creators. Every time a Creator is created
    // creatorID is assigned to the new creator's ID and then is incremented by 1.
    pub var nextCreatorID: UInt32

    // The total number of SubsArt NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global art IDs for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // SubsArt contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Art is a Struct that holds metadata associated 
    // with a specific art
    //
    pub struct Art {
        // The unique ID of the author
        pub let artId: UInt32

        // Stores all the metadata about the art as a string mapping
        // This is not the long term way NFT metadata will be stored. It's a temporary
        // construct while we figure out a better way to do metadata.
        //
        access(contract) let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Art metadata cannot be empty"
            }
            self.artId = SubsArt.nextArtID
            self.metadata = metadata

            // Increment the ID so that it isn't used again
            SubsArt.nextArtID = SubsArt.nextArtID + UInt32(1)

            emit ArtCreated(id: self.artId, metadata: metadata)
        }

        pub fun getMetadata(): {String: String} {
          return self.metadata
        }
    }

    pub struct ArtData {

        // The ID of the Gallery that the ArtData comes from
        pub let galleryID: UInt32

        // The ID of the Art that the ArtData references
        pub let artID: UInt32

        // The place in the edition that this NFT was minted
        // Otherwise know as the serial number
        pub let serialNumber: UInt32

        init(galleryID: UInt32, artID: UInt32, serialNumber: UInt32) {
            self.galleryID = galleryID
            self.artID = artID
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the Art NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Art ID
        pub let id: UInt64
        
        // Struct of ArtData metadata
        pub let data: ArtData

        init(serialNumber: UInt32, artID: UInt32, galleryID: UInt32) {
            // Increment the global Art IDs
            SubsArt.totalSupply = SubsArt.totalSupply + UInt64(1)

            self.id = SubsArt.totalSupply
            // Set the metadata struct
            self.data = ArtData(galleryID: galleryID, artID: artID, serialNumber: serialNumber)

            emit ArtNFTMinted(nftID: self.id, artID: artID, galleryID: self.data.galleryID, serialNumber: self.data.serialNumber)
        }

        // If the Art is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            SubsArt.totalSupply = SubsArt.totalSupply - UInt64(1)
            emit ArtNFTDestroyed(id: self.id)
        }
    }

    pub resource interface GalleryCreator {
        pub fun createArt(metadata: {String: String}): UInt32
        pub fun mintNFT(artID: UInt32): @NFT
        pub fun batchMintNFT(artID: UInt32, quantity: UInt64): @Collection
    }

    // Gallery is a resource type that contains the functions to add and remove
    // Art from a gallery and mint NFT. Each creator, who has right to mint NFT from his posts
    // would have a Gallery associated.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Arts to a Gallery so that the gallery can mint NFTs
    // that reference that artdata.
    // The NFTs that are minted by a Gallery will be listed as belonging to
    // the Gallery that minted it, as well as the Art it references.
    //
    // If the admin locks the Gallery, no more Art can be added to it.
    //
    pub resource Gallery: GalleryCreator {

        // Unique ID for the gallery
        pub let galleryID: UInt32

        // Array of arts that are a part of this gallery.
        // When an art is added to the gallery, its ID gets appended here.
        access(contract) var arts: [UInt32]

        // Map of Art IDs that Indicates if an Art in this Gallery can be minted.
        // When a Art is added to a Set, it is mapped to false (not concealed).
        // When an Art is concealed, this is set to true and cannot be changed.
        access(self) var concealed: {UInt32: Bool}

        // Indicates if the Gallery is currently locked.
        // When a Gallery is created, it is unlocked 
        // and Arts are allowed to be added to it.
        // When a gallery is locked, Arts cannot be added or new NFTs minted.
        pub var locked: Bool

        // Mapping of Art IDs that indicates the number of NFTs 
        // that have been minted for specific Art in this Gallery.
        // When an NFT is minted, this value is stored in the NFT to
        // show its place in the Gallery, eg. 13 of 60.
        access(self) var numberMintedPerArt: {UInt32: UInt32}

        init() {
            self.galleryID = SubsArt.nextGalleryID
            self.arts = []
            self.concealed = {}
            self.locked = false
            self.numberMintedPerArt = {}

            // Increment the nextgalleryID so that it isn't used again
            SubsArt.nextGalleryID = SubsArt.nextGalleryID + UInt32(1)

            emit GalleryCreated(galleryID: self.galleryID)
        }

        // createArt creates a new Art struct 
        // and stores it in the Arts dictionary in the SubsArt smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Title": "My New Artwork", "Dimensions": "1200x1200"}
        //
        // Returns: the ID of the new Art object
        //
        pub fun createArt(metadata: {String: String}): UInt32 {
            pre {
                !self.locked: "Cannot add create art from the Gallery after the gallery has been locked."
            }
            // Create the new Art
            var newArt = Art(metadata: metadata)
            let newID = newArt.artId

            // Store it in the contract storage
            SubsArt.artDatas[newID] = newArt

            // Add the Art to the array of Arts
            self.addArt(artID: newID)

            return newID
        }

        // concealArt conceals an Art from the Gallery so that it can't mint new NFT
        //
        // Parameters: artID: The ID of the Art that will be concealed
        //
        // Pre-Conditions:
        // The Art is part of the Gallery and not concealed (available for minting).
        // 
        pub fun concealArt(artID: UInt32) {
            pre {
                self.concealed[artID] != nil: "Cannot conceal the Art: Art doesn't exist in this Gallery!"
            }

            if !self.concealed[artID]! {
                self.concealed[artID] = true

                emit ConcealArtInGallery(galleryID: self.galleryID, artID: artID)
            }
        }

        // addArt adds an art to the gallery
        //
        // Parameters: artID: The ID of the Art that is being added
        //
        // Pre-Conditions:
        // The Art needs to be an existing art
        // The Gallery needs to be not locked
        // The Art can't have already been added to the Gallery
        //
        pub fun addArt(artID: UInt32) {
            pre {
                SubsArt.artDatas[artID] != nil: "Cannot add the Art to Gallery: Art doesn't exist."
                !self.locked: "Cannot add the art to the Gallery after the gallery has been locked."
                self.numberMintedPerArt[artID] == nil: "The art has already beed added to the gallery."
            }

            // Add the Art to the array of Arts
            self.arts.append(artID)

            // Open the Art up for minting
            self.concealed[artID] = false

            // Initialize the art count to zero
            self.numberMintedPerArt[artID] = 0

            emit ArtAddedToGallery(galleryID: self.galleryID, artID: artID)
        }

        // addArts adds multiple Arts to the Gallery
        //
        // Parameters: artIDs: The IDs of the Arts that are being added
        //                      as an array
        //
        pub fun addArts(artIDs: [UInt32]) {
            for artID in artIDs {
                self.addArt(artID: artID)
            }
        }


        // mintNFT mints a new NFT and returns the newly minted object
        // 
        // Parameters: artID: The ID of the Art that the NFT references
        //
        // Pre-Conditions:
        // The Art must exist in the Gallery and be allowed to mint new NFT
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintNFT(artID: UInt32): @NFT {
            pre {
                SubsArt.artDatas[artID] != nil: "Cannot add the Art to Gallery: Art doesn't exist."
                !self.locked: "Cannot mint after the gallery has been locked."
                self.numberMintedPerArt[artID] != nil: "The art has not beed added to the gallery."
                !self.concealed[artID]!: "Cannot mint the NFT from this Art: This Art has been concealed."
            }
            // Gets the number of NFTs that have been minted for this Art
            // to use as this NFT's serial number
            let numInArts = self.numberMintedPerArt[artID]!

            // Mint the new NFT
            let newNFT: @NFT <- create NFT(serialNumber: numInArts + UInt32(1),
                                            artID: artID,
                                            galleryID: self.galleryID)

            // Increment the count of NFT minted for this Art
            self.numberMintedPerArt[artID] = numInArts + UInt32(1)

            return <-newNFT
        }

        // batchMintNFT mints an arbitrary quantity of NFTs 
        // and returns them as a Collection
        //
        // Parameters: artID: the ID of the Art that the NFT are minted for
        //             quantity: The quantity of NFT to be minted
        //
        // Returns: Collection object that contains all the NFTs that were minted
        //
        pub fun batchMintNFT(artID: UInt32, quantity: UInt64): @Collection {
            pre {
                SubsArt.artDatas[artID] != nil: "Cannot add the Art to Gallery: Art doesn't exist."
                !self.locked: "Cannot mint after the gallery has been locked."
                self.numberMintedPerArt[artID] != nil: "The art has not beed added to the gallery."
                !self.concealed[artID]!: "Cannot mint the NFT from this Art: This Art has been concealed."
            }
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(artID: artID))
                i = i + UInt64(1)
            }

            return <-newCollection
        }

        access(contract) fun lock() {
            if !self.locked {
                self.locked = true
                emit GalleryLocked(galleryID: self.galleryID)
            }
        }

        access(contract) fun unlock() {
            if self.locked {
                self.locked = false
                emit GalleryUnlocked(galleryID: self.galleryID)
            }
        }
    }

    pub resource interface SubsUserPublic {
        pub fun assignCreator(creator: @SubsArt.Creator)
    }

    pub resource SubsUser: SubsUserPublic {
        access(self) var creator: @SubsArt.Creator?

        init() {
            self.creator <- nil
        }

        pub fun borrowCreatorGallery(): &Gallery{SubsArt.GalleryCreator}? {
          return self.creator?.borrowGallery()
        }

        pub fun assignCreator(creator: @SubsArt.Creator) {
            pre {
                self.creator == nil: "Cannot assing creator, it has been already assigned!"
            }
            self.creator <-! creator
        }

        destroy() {
          destroy self.creator
        }
    }

    // Creator is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Arts and Gallery. 
    // In particular Creator is approved role that is capable of minting NFT 
    // based on the created Art, within the Gallery they've been assigned.
    // Admin creates the gallery and assings to Creator.
    //
    pub resource Creator {
        // Unique ID for the creator
        pub let creatorID: UInt32

        // Unique ID of a Subs user
        pub let userID: UInt64

        init(userID: UInt64) {
            self.creatorID = SubsArt.nextCreatorID
            self.userID = userID

            // Increment the nextgalleryID so that it isn't used again
            SubsArt.nextCreatorID = SubsArt.nextCreatorID + UInt32(1)

            emit CreatorCreated(creatorID: self.creatorID)
        }


        // borrowGallery returns a reference to a gallery in the SubsArt
        // contract so that the Creator can call methods on it
        // it can borrow only this Creator's gallery by userID
        //
        // Returns: A reference to the Gallery with all of the fields
        // and methods exposed via interface GalleryCreator
        //
        pub fun borrowGallery(): &Gallery{SubsArt.GalleryCreator} {
            pre {
                SubsArt.galleries[self.userID] != nil: "Cannot borrow Gallery: The Gallery doesn't exist"
            }
            
            // Get a reference to the Gallery and return it
            // use `&` to indicate the reference to the object and type
            return &SubsArt.galleries[self.userID] as &Gallery
        }
    }

    // Moderator is a special authorization resource that 
    // allows the owner to grant permission to new user to mint NFT
    //
    pub resource Moderator {
        // createNewCreator creates a new Creator resource
        //
        // Parameters: userID: The ID of the User that this resoulve will 
        // be assosiated to
        //
        pub fun createNewCreator(userID: UInt64): @SubsArt.Creator {
            return <-create SubsArt.Creator(userID: userID)
        }

        // createGallery creates a new Gallery resource and stores it
        // in the galleries mapping in the SubsArt contract
        //
        // Parameters: userID: The ID of user the new gallery will be linked to
        //
        pub fun createGallery(userID: UInt64) {
            pre {
              SubsArt.galleries[userID] == nil: "Cannot create Gallery: The Gallery already exists"
            }

            // Create the new Gallery
            var newGallery <- create SubsArt.Gallery()

            // Store it in the galleries mapping field
            SubsArt.galleries[userID] <-! newGallery
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Arts and Galleries
    //
    pub resource Admin {

        // createArt creates a new Art struct 
        // and stores it in the Arts dictionary in the SubsArt smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Title": "My New Artwork", "Dimensions": "1200x1200"}
        //
        // Returns: the ID of the new Art object
        //
        pub fun createArt(metadata: {String: String}): UInt32 {
            // Create the new Art
            var newArt = Art(metadata: metadata)
            let newID = newArt.artId

            // Store it in the contract storage
            SubsArt.artDatas[newID] = newArt

            return newID
        }

        // borrowGallery returns a reference to a gallery in the SubsArt
        // contract so that the admin can call methods on it
        //
        // Parameters: userID: The ID of the User that you want to
        // get a Gallery's reference to
        //
        // Returns: A reference to the Gallery with all of the fields
        // and methods exposed
        //
        pub fun borrowGallery(userID: UInt64): &SubsArt.Gallery {
            pre {
                SubsArt.galleries[userID] != nil: "Cannot borrow Gallery: The Gallery doesn't exist"
            }
            
            // Get a reference to the Gallery and return it
            // use `&` to indicate the reference to the object and type
            return &SubsArt.galleries[userID] as &SubsArt.Gallery
        }

        // lockGallery locks up the gallery assigned to the specific user
        // so that he wouldn't able to mint
        //
        // Parameters: userID: The ID of the User that you want to
        // get a Gallery's reference to
        //
        pub fun lockGallery(userID: UInt64) {
            pre {
                SubsArt.galleries[userID] != nil: "Cannot borrow Gallery: The Gallery doesn't exist"
            }
            
            // Get a reference to the Gallery and return it
            // use `&` to indicate the reference to the object and type
            let gallery = &SubsArt.galleries[userID] as &SubsArt.Gallery
            gallery.lock()
        }

        // unlockGallery unlocks the gallery assigned to the specific user
        // so that he would be able to mint again
        pub fun unlockGallery(userID: UInt64) {
            pre {
                SubsArt.galleries[userID] != nil: "Cannot borrow Gallery: The Gallery doesn't exist"
            }
            
            // Get a reference to the Gallery and return it
            // use `&` to indicate the reference to the object and type
            let gallery = &SubsArt.galleries[userID] as &SubsArt.Gallery
            gallery.unlock()
        }


        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @SubsArt.Admin {
            return <-create SubsArt.Admin()
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewModerator(): @SubsArt.Moderator {
            return <-create SubsArt.Moderator()
        }
    }

    // This is the interface that users can cast their Art Collection as
    // to allow others to deposit Arts into their Collection. It also allows for reading
    // the IDs of Arts in the Collection.
    pub resource interface ArtCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowArtNFT(id: UInt64): &SubsArt.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Art reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: ArtCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Art conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Art from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Art does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Arts
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

        // deposit takes a Art and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a SubsArt NFT to make sure
            // it is the correct type
            let token <- token as! @SubsArt.NFT

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

        // borrowNFT Returns a borrowed reference to a Art in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any SubsArt specific data. Please use borrowArt to 
        // read Art data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowArt returns a borrowed reference to a Art
        // so that the caller can read data and call methods from it.
        // They can use this to read its galleryID, artID, serialNumber,
        // or any of the GalleryData or Art data associated with it by
        // getting the galleryID or artID and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowArtNFT(id: UInt64): &SubsArt.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SubsArt.NFT
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
    // SubsArt contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Arts in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create SubsArt.Collection()
    }

    // createSubsUser creates a new SubsUser object so that
    // we have a place for all internal resources.
    //
    pub fun createSubsUser(): @SubsArt.SubsUser {
        return <-create SubsArt.SubsUser()
    }

    // getAllArts returns all the arts in SubsArt
    //
    // Returns: An array of all the arts that have been created
    pub fun getAllArts(): [SubsArt.Art] {
        return SubsArt.artDatas.values
    }

    // getArtMetaData returns all the metadata associated with a specific Art
    // 
    // Parameters: artID: The id of the Art that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getArtMetaData(artID: UInt32): {String: String}? {
        return SubsArt.artDatas[artID]?.metadata
    }

    // isGalleryLocked returns a boolean that indicates if a Gallery
    //             is locked. If it's locked, 
    //             new Arts can no longer be added to it,
    //             and no new NFTs minted as well.
    // 
    // Parameters: userID: The id a User who owns the Gallery that is being searched
    //
    // Returns: Boolean indicating if the Gallery is locked or not
    pub fun isGalleryLocked(userID: UInt64): Bool? {
        // Don't force a revert if the galleryID is invalid
        return SubsArt.galleries[userID]?.locked
    }


    // isGalleryExists returns a boolean that indicates if a Gallery
    //             exists for a given userID
    // 
    // Parameters: userID: The id a User who owns the Gallery that is being searched
    //
    // Returns: Boolean indicating if the Gallery exists
    pub fun isGalleryExists(userID: UInt64): Bool {
        // Don't force a revert if the galleryID is invalid
        return SubsArt.galleries[userID] != nil
    }

    // getArtsInGallery returns the list of Arts IDs that are in the Gallery
    // 
    // Parameters: userID: The id a User who owns the Gallery that is being searched
    //
    // Returns: An array of Art IDs
    pub fun getArtsInGallery(userID: UInt64): [UInt32]? {
        // Don't force a revert if the setID is invalid
        return SubsArt.galleries[userID]?.arts
    }


    // -----------------------------------------------------------------------
    // SubsArt initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.artDatas = {}
        self.galleries <- {}
        self.totalSupply = 0
        self.nextGalleryID = 1
        self.nextCreatorID = 1
        self.nextArtID = 1

        self.CollectionStoragePath = /storage/SubsArtOriginalsCollection
        self.CollectionPublicPath = /public/SubsArtOriginalsCollection
        self.AdminStoragePath = /storage/SubsArtOriginalsAdmin
        self.ModeratorStoragePath = /storage/SubsArtOriginalsModerator
        self.SubsUserStoragePath = /storage/SubsOriginalsUser
        self.SubsUserPublicPath = /public/SubsOriginalsUserPublic

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{ArtCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Admin in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
        // Put the Moderator in storage
        self.account.save<@Moderator>(<- create Moderator(), to: self.ModeratorStoragePath)

        emit ContractInitialized()
    }
}

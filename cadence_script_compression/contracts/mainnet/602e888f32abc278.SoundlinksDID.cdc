/**
    Description: Central Smart Contract for SoundlinksDID
    This smart contract contains the core functionality for SoundlinksDID.

    Copyright 2021 Soundlinks
    SPDX-License-Identifier: Apache-2.0
**/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract SoundlinksDID: NonFungibleToken {

    // -----------------------------------------------------------------------
    // SoundlinksDID Contract Events
    // -----------------------------------------------------------------------

    /// Emitted when the SoundlinksDID contract is created
    pub event ContractInitialized()

    /// Events for DID-Related actions
    ///
    /// Emitted when a new Soundlinks DID is created
    pub event DIDMinted(id: UInt64, hash: String)

    /// Events for Collection-Related actions
    ///
    /// Emitted when a Soundlinks DID is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    /// Emitted when a Soundlinks DID is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // SoundlinksDID Named Paths
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // SoundlinksDID Contract-Level Fields
    // -----------------------------------------------------------------------

    /// The total number of Soundlinks DIDs that have been created
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // SoundlinksDID Contract-Level Composite Type Definitions
    // -----------------------------------------------------------------------

    /// The resource that represents the Soundlinks DID
    /// A Soundlinks DID as an NFT
    ///
    pub resource NFT: NonFungibleToken.INFT {

        /// The unique ID for the Soundlinks DID
        pub let id: UInt64

        /// The hash for the Soundlinks DID
        pub let hash: String

        init(initID: UInt64, initHash: String) {
            pre {
                initHash.length > 0: "New Soundlinks DID hash cannot be empty."
            }
            self.id = initID
            self.hash = initHash
        }
    }

    /// This is the interface that users can cast their Soundlinks DID Collection as
    /// to allow others to deposit Soundlinks DIDs into their Collection. It also allows
    /// for reading the IDs of Soundlinks DIDs in the Collection.
    ///
    pub resource interface SoundlinksDIDCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowSoundlinksDID(id: UInt64): &SoundlinksDID.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow SoundlinksDID reference: The ID of the returned reference is incorrect."
            }
        }
    }

    /// Collection is a resource that every user who owns Soundlinks DIDs
    /// will store in their account to manage their DIDs
    ///
    pub resource Collection: SoundlinksDIDCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        /// Dictionary of Soundlinks DID conforming tokens
        /// Soundlinks DID is a resource type with a `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        /// withdraw removes a Soundlinks DID from the Collection and moves it to the caller
        ///
        /// Parameters: withdrawID: The ID of the Soundlinks DID
        /// that is to be removed from the Collection
        ///
        /// Returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the Soundlinks DID from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: Soundlinks DID does not exist in the collection.")

            emit Withdraw(id: token.id, from: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }

        /// batchWithdraw withdraws multiple Soundlinks DIDs and returns them as a Collection
        ///
        /// Parameters: ids: An array of IDs to withdraw
        ///
        /// Returns: @NonFungibleToken.Collection: A collection that contains the withdrawn DIDs
        ///
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

        /// deposit takes a Soundlinks DID and adds it to the Collection dictionary
        ///
        /// Paramters: token: the DID to be deposited in the Collection
        ///
        pub fun deposit(token: @NonFungibleToken.NFT) {

            // Cast the deposited token as a Soundlinks DID to make sure
            // it is the correct type
            let token <- token as! @SoundlinksDID.NFT

            // Get the token's ID
            let id: UInt64 = token.id

            // Add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            // Destroy the empty old token that was "removed"
            destroy oldToken
        }

        /// batchDeposit takes a Collection object as an argument
        /// and deposits each contained DID into this Collection
        ///
        /// Paramters: tokens: the DIDs Collection
        ///
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

        /// getIDs returns an array of the IDs that are in the collection
        ///
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        /// getIDByOne returns an ID that are in the collection
        ///
        pub fun getIDByOne(): UInt64 {
            pre {
                self.ownedNFTs.length > 0: "There's not enough DID in the collection."
            }

            var currentIDs = self.getIDs()

            return currentIDs.removeFirst()
        }

        /// getIDsByAmount returns an array of the specified number of IDs that are in the collection
        ///
        pub fun getIDsByAmount(amount: UInt32): [UInt64] {
            pre {
                amount <= UInt32(self.ownedNFTs.length): "There's not enough DIDs in the collection."
            }

            var currentIDs = self.getIDs()
            var ids: [UInt64] = []

            var i: UInt32 = 0
            while i < amount {
                ids.append(currentIDs.removeFirst())
                i = i + (1 as UInt32)
            }

            return ids
        }

        /// borrowNFT returns a borrowed reference to a NFT in the Collection
        /// so that the caller can read its ID
        ///
        /// Parameters: id: The ID of the NFT to get the reference for
        ///
        /// Returns: A reference to the NFT
        ///
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        /// borrowDID returns a borrowed reference to a Soundlinks DID
        /// so that the caller can read data and call methods from it.
        ///
        /// Parameters: id: The ID of the Soundlinks DID to get the reference for
        ///
        /// Returns: A reference to the Soundlinks DID
        ///
        pub fun borrowSoundlinksDID(id: UInt64): &SoundlinksDID.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SoundlinksDID.NFT
            } else {
                return nil
            }
        }

        /// If a transaction destroys the Collection object,
        /// All the Soundlinks DIDs contained within are also destroyed!
        ///
        destroy() {
            destroy self.ownedNFTs
       }
    }

    /// Admin is a special authorization resource that
    /// allows the owner to perform important functions about DIDs
    ///
    pub resource Admin {

        /// mintDIDs mints an arbitrary quantity of DIDs
        ///
        /// Parameters: recipient: The recipient's account using their reference
        ///             hashs: An array of hashs to mint Soundlinks DIDs
        ///
        pub fun mintDIDs(recipient: &{NonFungibleToken.CollectionPublic}, hashs: [String]) {

            for hash in hashs {

                emit DIDMinted(id: SoundlinksDID.totalSupply, hash: hash)

                // Deposit it in the recipient's account using their reference
                recipient.deposit(token: <-create SoundlinksDID.NFT(initID: SoundlinksDID.totalSupply, initHash: hash))

                // Increment the global Soundlinks DID IDs
                SoundlinksDID.totalSupply = SoundlinksDID.totalSupply + (1 as UInt64)
            }
        }

        /// createNewAdmin creates a new Admin resource
        ///
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // -----------------------------------------------------------------------
    // SoundlinksDID Contract-Level Function Definitions
    // -----------------------------------------------------------------------

    /// createEmptyCollection creates a new, empty Collection object so that
    /// a user can store it in their account storage.
    /// Once they have a Collection in their storage, they are able to receive
    /// Soundlinks DID in transactions.
    ///
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create SoundlinksDID.Collection()
    }

    // -----------------------------------------------------------------------
    // SoundlinksDID Initialization Function
    // -----------------------------------------------------------------------

    init() {
        // Set named paths
        self.CollectionStoragePath = /storage/SoundlinksDIDCollection
        self.CollectionPublicPath = /public/SoundlinksDIDCollection
        self.AdminStoragePath = /storage/SoundlinksDIDAdmin

        // Initialize the total supply
        self.totalSupply = 0

        // Create an Admin resource and save it to storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
/**  
# Contract defining records and songs. 

## Record:
A record is the unique issuance of a song. 
It holds the information including song title, artist, description, cover image address (an IPFS CID) and audio address (IPFS CID). 

Songs are by default hidden, meaning the audio file is encrypted. The holder can decide to reveal the song (publish the decryption key). 
Publishing the decryption key is done by an admin when the resource is made accessible in "unlocked" mode. 
Once the decryption key has been set, it cannot be changed again.

A record unlocked but without an decryption key should be prevented from being set for auction or for sale. 
It represents the case when the song has been unlock but the admin is still uploading the encryption key. 
We do not want buyers to bid on a song they think is still unreleased, and then discover is was being released. 
If the owner wants to cancel the publication of the key before it has been done, they can call the `lock` function.

A record points to an artist, defined by an ID. An artist registery is held so that the artist share can be sent correctly after sales and auctions.
**/

import NonFungibleToken from 0x1d7e57aa55817448
import ArtistRegistery from 0x3558f4548a40dc77

pub contract Record: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Variables 
    // -----------------------------------------------------------------------

    // Access paths
    //
    // Public path allowing deposits, listing of IDs and access to records metadata
    pub let CollectionPublicPath: PublicPath

    // Private path allowing withdrawals on top of the public access
    // Useful for SaleCollection requiring withdrawals, but protecting lock/unlock functions
    pub let CollectionPrivatePath: PrivatePath
    // Storage path of the collection of records owned
    pub let CollectionStoragePath: StoragePath
    // Storage path of the Admin, responsible for uploading decryption keys
    pub let AdminStoragePath: StoragePath
    // Storage path of the miner responsible for creating new records
    pub let MinterStoragePath: StoragePath
    // The total number of Record NFTs that have been created
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------------

    // Emitted when the contract is created
    pub event ContractInitialized()

    // Emitted when a new record is minted
    pub event RecordMinted(id: UInt64, metadata: Metadata)
    // Emited when a record is deleted 
    pub event RecordDestroyed(id: UInt64)
    // Emited when a record is locked 
    pub event RecordLocked(id: UInt64)
    // Emited when a record is unlocked 
    pub event RecordUnlocked(id: UInt64)
    // Emitted when a record audio key is published
    pub event RecordKeyUploaded(id: UInt64, audiokey: String)
    // Emitted when a record is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a record is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // Resources
    // -----------------------------------------------------------------------
    
    // Structure representing the metadata of a record
    pub struct Metadata {
        pub let title: String // title of the record
        pub let artistID: UInt64 // this ID can be used in the ArtistRegistery contract functions
        pub let description: String // description an artist can write about the record
        pub let audioaddr: String // record audio address (IPFS CID)
        pub let coveraddr: String // cover image address (IPFS CID)

        init(  
            title: String, 
            artistID: UInt64,
            description: String, 
            audioaddr: String, 
            coveraddr: String
        ) {
            self.title = title
            self.artistID = artistID
            self.description = description
            self.audioaddr = audioaddr
            self.coveraddr = coveraddr
        }
    }

    // Interface for records when accessed publicly
    pub resource interface Public {
        pub let id: UInt64
        pub let metadata: Metadata
        pub var audiokey: String?

        // Whether the record decryption key is locked
        pub fun isLocked(): Bool 
        // Whether the record decryption key is unlocked but unset yet
        pub fun tradable(): Bool

        // this function can only be called by the admin, and will fail if the record is still locked
        access(contract) fun setAudioKey(audiokey: String) 
    }

    // Resource representing a unique song. Can only be created by a minter
    pub resource NFT: NonFungibleToken.INFT, Public {

        // Unique ID for the record
        pub let id: UInt64

        pub let metadata: Metadata
        pub var audiokey: String? // key used to decrypt the audio content
        pub var locked: Bool

        init(metadata: Metadata) {
            pre {
                metadata.artistID <= ArtistRegistery.numberOfArtists: "This artistID does not exist"
            }
            
            Record.totalSupply = Record.totalSupply + (1 as UInt64)
            self.id = Record.totalSupply

            self.metadata = metadata
            self.audiokey = nil
            self.locked = true

            emit RecordMinted(id: self.id, metadata: self.metadata)
        }

        // attach the url to a newly released song, when is unlocked
        access(contract) fun setAudioKey(audiokey: String) {
          pre {
            self.audiokey == nil: "The key has already been set."
            !self.locked: "This record is locked"
          }

          self.audiokey = audiokey 
          emit RecordKeyUploaded(id: self.id, audiokey: audiokey)
        }

        access(contract) fun lock() {
            self.locked = true
            emit RecordLocked(id: self.id)
        }

        access(contract) fun unlock() {
            self.locked = false
            emit RecordUnlocked(id: self.id)
        }

        pub fun isLocked(): Bool {
            return self.locked
        }

        pub fun tradable(): Bool {
            return self.locked || self.audiokey != nil
        }

        destroy() {
            emit RecordDestroyed(id: self.id)
        }
    }

    // Admin resource
    //
    // Can create new admins and set decryption keys of unlocked songs
    pub resource Admin {

        // Publish the decryption key of the record `id` from the given collection
        pub fun setRecordAudioKey(collection: &Collection{CollectionPublic}, id: UInt64, audiokey: String) {
            collection.borrowRecord(recordID: id)!.setAudioKey(audiokey: audiokey)
        }

        // New admins can be created by an admin.
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // New minters can be created by an admin.
        pub fun createNewMinter(): @Minter {
            return <-create Minter()
        }
    }

    // Minter resource
    //
    // A minter is responsible for minting new records
    pub resource Minter {

        // Mint a new record with the given information
        pub fun mintRecord(
            title: String, 
            artistID: UInt64,
            description: String, 
            audioaddr: String, 
            coveraddr: String
            ): @Record.NFT {
            
            let record <- create NFT(
                metadata: Metadata(
                    title: title, 
                    artistID: artistID,
                    description: description, 
                    audioaddr: audioaddr, 
                    coveraddr: coveraddr
                )
            )

            return <-record
        }
    }

    // Public interface for a record collection. 
    // It allows someone to check what is inside someone's collection or to deposit a record in it.
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowRecord(recordID: UInt64): &{Record.Public}? {
            post {
                (result == nil) || (result?.id == recordID): 
                    "Cannot borrow Record reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // A collection of one's records
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // Withdraw a given record from the collection
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let record <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Record does not exist in the collection")

            emit Withdraw(id: record.id, from: self.owner?.address)
            
            return <-record
        }

        // Deposit a record
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited record as a Record NFT to make sure
            // it is the correct type
            let token <- token as! @Record.NFT

            let id = token.id
            let oldRecord <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldRecord
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // Get and NFT as a NonFungibleToken.NFT reference
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Get and NFT as a Record.Public reference
        pub fun borrowRecord(recordID: UInt64): &{Record.Public}? {
            if self.ownedNFTs[recordID] != nil {
                let ref = &self.ownedNFTs[recordID] as auth &NonFungibleToken.NFT
                return ref as! &Record.NFT
            } else {
                return nil
            }
        }

        // Lock the requested record 
        pub fun lockRecord(recordID: UInt64) {
            pre {
                self.ownedNFTs[recordID] != nil: "The requested record cannot be found in the collection"
            }

            if self.ownedNFTs[recordID] != nil {
                let refNFT = &self.ownedNFTs[recordID] as auth &NonFungibleToken.NFT
                let refRecord = refNFT as! &Record.NFT
                refRecord.lock()
            }
        }

        // Unlock the requested record 
        pub fun unlockRecord(recordID: UInt64) {
            pre {
                self.ownedNFTs[recordID] != nil: "The requested record cannot be found in the collection"
            }

            if self.ownedNFTs[recordID] != nil {
                let refNFT = &self.ownedNFTs[recordID] as auth &NonFungibleToken.NFT
                let refRecord = refNFT as! &Record.NFT
                refRecord.unlock()
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Contract public functions
    // -----------------------------------------------------------------------

    // Create a collection to hold records
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create Record.Collection()
    }

    // -----------------------------------------------------------------------
    // Initialization function
    // -----------------------------------------------------------------------
    
    init() {
        self.CollectionPublicPath = /public/boulangeriev1RecordCollection
        self.CollectionPrivatePath = /private/boulangeriev1RecordCollection
        self.CollectionStoragePath = /storage/boulangeriev1RecordCollection
        self.AdminStoragePath = /storage/boulangeriev1RecordAdminStorage
        self.MinterStoragePath = /storage/boulangeriev1RecordMinterStorage
        
        self.totalSupply = 0

        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        self.account.link<&Collection{CollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        self.account.link<&Collection{Record.CollectionPublic, NonFungibleToken.Provider}>(self.CollectionPrivatePath, target: self.CollectionStoragePath)

        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
        self.account.save<@Minter>(<- create Minter(), to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

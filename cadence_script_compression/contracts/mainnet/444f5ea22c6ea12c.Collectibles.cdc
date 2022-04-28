import NonFungibleToken from 0x1d7e57aa55817448

pub contract Collectibles: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // totalSupply
    // The total number of Collectibles that have been minted
    //
    pub var totalSupply: UInt64

    // Type Definitions
    // 
    pub struct Metadata {
        pub let name: String
        pub let description: String
        // MIME type: image/png, image/jpeg, video/mp4, audio/mpeg
        pub let mediaType: String 
        // IPFS storage hash
        pub let mediaHash: String
        // URI to NFT media - incase IPFS not in use/avail
        pub let mediaURI: String

        init(name: String, description: String, mediaType: String, mediaHash: String, mediaURI: String) {
            self.name = name
            self.description = description
            self.mediaType = mediaType
            self.mediaHash = mediaHash
            self.mediaURI = mediaURI
        }
    }

    // NFT
    // A Collectible
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64

        // The token's metadata
        access(self) let metadata: Metadata

        // Implement the NFTMetadata.INFTPublic interface
        pub fun getMetadata(): Metadata {
            return self.metadata
        }

        // initializer
        //
        init(initID: UInt64, metadata: Metadata) {
            self.id = initID
            self.metadata = metadata
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCollectible(id: UInt64): &Collectibles.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Collectibles reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Collectible NFTs owned by an account
    //
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Collectibles.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowCollectible
        // Gets a reference to an NFT in the collection as a Collectible,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the Collectibles.
        //
        pub fun borrowCollectible(id: UInt64): &Collectibles.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Collectibles.NFT
            } else {
                return nil
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter {

        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: Metadata) {
            emit Minted(id: Collectibles.totalSupply)

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create Collectibles.NFT(initID: Collectibles.totalSupply, metadata: metadata))
            Collectibles.totalSupply = Collectibles.totalSupply + (1 as UInt64)
        }

        // batchMintNFT
        // Mints a batch of new NFTs
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun batchMintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: Metadata, count: Int) {
            var index = 0
        
            while index < count {
                self.mintNFT(
                    recipient: recipient,
                    metadata: metadata
                )

                index = index + 1
            }
        }
    }

    // fetch
    // Get a reference to a Collectible from an account's Collection, if available.
    // If an account does not have a Collectibles.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &Collectibles.NFT? {
        let collection = getAccount(from)
            .getCapability(Collectibles.CollectionPublicPath)!
            .borrow<&Collectibles.Collection>()
            ?? panic("Couldn't get collection")
        // We trust Collectibles.Collection.borowCollectible to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowCollectible(id: itemID)
    }

    // initializer
    //
    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/jambbLaunchCollectiblesCollection
        self.CollectionPublicPath = /public/jambbLaunchCollectiblesCollection
        self.MinterStoragePath = /storage/jambbLaunchCollectiblesMinter

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

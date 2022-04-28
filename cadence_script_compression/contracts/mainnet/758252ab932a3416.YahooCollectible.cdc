import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract YahooCollectible: NonFungibleToken {

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
    pub let AdminStoragePath: StoragePath

    // totalSupply
    // The total number of YahooCollectible that have been minted
    //
    pub var totalSupply: UInt64

    // metadata for each item
    // 
    access(contract) var itemMetadata: {UInt64: Metadata}

    // Type Definitions
    // 
    pub struct Metadata {
        pub let name: String
        pub let description: String

        // mediaType: MIME type of the media
        // - image/png
        // - image/jpeg
        // - video/mp4
        // - audio/mpeg
        pub let mediaType: String

        // mediaHash: IPFS storage hash
        pub let mediaHash: String

        // additional metadata
        access(self) let additional: {String: String}

        // number of items
        pub(set) var itemCount: UInt64

        init(name: String, description: String, mediaType: String, mediaHash: String, additional: {String: String}) {
            self.name = name
            self.description = description
            self.mediaType = mediaType
            self.mediaHash = mediaHash
            self.additional = additional
            self.itemCount = 0
        }

        pub fun getAdditional(): {String: String} {
            return self.additional
        }
    }

    // NFT
    // A Yahoo Collectible NFT
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        // The token's ID
        pub let id: UInt64

        // The token's type
        pub let itemID: UInt64

        // The token's edition number
        pub let editionNumber: UInt64

        // initializer
        //
        init(initID: UInt64, itemID: UInt64) {
            self.id = initID
            self.itemID = itemID

            let metadata = YahooCollectible.itemMetadata[itemID] ?? panic("itemID not valid")
            self.editionNumber = metadata.itemCount + (1 as UInt64)

            // Increment the edition count by 1
            metadata.itemCount = self.editionNumber

            YahooCollectible.itemMetadata[itemID] = metadata
        }

        // Expose metadata
        pub fun getMetadata(): Metadata? {
            return YahooCollectible.itemMetadata[self.itemID]
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<MetadataViews.IPFSFile>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    let metadata = self.getMetadata() ?? panic("missing metadata")

                    return MetadataViews.Display(
                        name: metadata.name,
                        description: metadata.description,
                        thumbnail: MetadataViews.IPFSFile(
                            cid: metadata.mediaHash,
                            path: nil
                        )
                    )
            
                case Type<MetadataViews.IPFSFile>():
                    return MetadataViews.IPFSFile(
                        cid: self.getMetadata()!.mediaHash,
                        path: nil
                    )
            }

            return nil
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowYahooCollectible(id: UInt64): &YahooCollectible.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow YahooCollectible reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of YahooCollectible NFTs owned by an account
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
            let token <- token as! @YahooCollectible.NFT

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

        // borrowYahooCollectible
        // Gets a reference to an NFT in the collection as a YahooCollectible,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the YahooCollectible.
        //
        pub fun borrowYahooCollectible(id: UInt64): &YahooCollectible.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &YahooCollectible.NFT
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

    // Admin
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource Admin {

        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, itemID: UInt64, codeHash: String?) {
            pre {
                codeHash == nil || !YahooCollectible.checkCodeHashUsed(codeHash: codeHash!): "duplicated codeHash"
            }
            
            emit Minted(id: YahooCollectible.totalSupply)

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create YahooCollectible.NFT(initID: YahooCollectible.totalSupply, itemID: itemID))
            YahooCollectible.totalSupply = YahooCollectible.totalSupply + (1 as UInt64)

            // if minter passed in codeHash, register it to dictionary
            if let checkedCodeHash = codeHash {
                let redeemedCodes = YahooCollectible.account.load<{String: Bool}>(from: /storage/redeemedCodes)!
                redeemedCodes[checkedCodeHash] = true
                YahooCollectible.account.save<{String: Bool}>(redeemedCodes, to: /storage/redeemedCodes)
            }
        }

        // batchMintNFT
        // Mints a batch of new NFTs
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun batchMintNFT(recipient: &{NonFungibleToken.CollectionPublic}, itemID: UInt64, count: Int) {
            var index = 0
        
            while index < count {
                self.mintNFT(
                    recipient: recipient,
                    itemID: itemID,
                    codeHash: nil
                )

                index = index + 1
            }
        }

        // registerMetadata
        // Registers metadata for a itemID
        //
        pub fun registerMetadata(itemID: UInt64, metadata: Metadata) {
            pre {
                YahooCollectible.itemMetadata[itemID] == nil: "duplicated itemID"
            }

            YahooCollectible.itemMetadata[itemID] = metadata
        }

        // updateMetadata
        // Registers metadata for a itemID
        //
        pub fun updateMetadata(itemID: UInt64, metadata: Metadata) {
            pre {
                YahooCollectible.itemMetadata[itemID] != nil: "itemID does not exist"
            }

            metadata.itemCount = YahooCollectible.itemMetadata[itemID]!.itemCount

            // update metadata
            YahooCollectible.itemMetadata[itemID] = metadata
        }
    }

    // fetch
    // Get a reference to a YahooCollectible from an account's Collection, if available.
    // If an account does not have a YahooCollectible.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &YahooCollectible.NFT? {
        let collection = getAccount(from)
            .getCapability(YahooCollectible.CollectionPublicPath)!
            .borrow<&YahooCollectible.Collection>()
            ?? panic("Couldn't get collection")
        // We trust YahooCollectible.Collection.borowYahooCollectible to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowYahooCollectible(id: itemID)
    }

    // getMetadata
    // Get the metadata for a specific type of YahooCollectible
    //
    pub fun getMetadata(itemID: UInt64): Metadata? {
        return YahooCollectible.itemMetadata[itemID]
    }

    // checkCodeHashUsed
    // Check if a codeHash has been registered
    //
    pub fun checkCodeHashUsed(codeHash: String): Bool {
        var redeemedCodes = YahooCollectible.account.copy<{String: Bool}>(from: /storage/redeemedCodes)!
        return redeemedCodes[codeHash] ?? false
    }

    // initializer
    //
    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/yahooCollectibleCollection
        self.CollectionPublicPath = /public/yahooCollectibleCollection
        self.AdminStoragePath = /storage/yahooCollectibleAdmin

        // Initialize the total supply
        self.totalSupply = 0

        // Initialize predefined metadata
        self.itemMetadata = {}

        // Create a Admin resource and save it to storage
        let minter <- create Admin()
        self.account.save(<-minter, to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}

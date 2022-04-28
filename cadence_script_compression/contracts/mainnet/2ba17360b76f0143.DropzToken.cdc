import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract DropzToken: NonFungibleToken {
    // The total supply of the token
    pub var totalSupply: UInt64

    // The maximum supply of the token
    pub var maximumSupply: UInt64

    // The description of the token
    pub var description: String

    // The IPFS CID of the image directory
    pub var imageCID: String

    // The IPFS CID of the metadata directory
    pub var metadataCID: String

    // Tracks if the token is revealed
    pub var revealed: Bool

    // See MetadataViews.IPFSFile
    pub struct IPFSTokenMetadata: MetadataViews.File {
        pub let cid: String
        pub let path: String?

        init(itemID: String) {
            self.cid = DropzToken.metadataCID
            self.path = itemID.concat(".json")
        }

        pub fun uri(): String {
            if let path = self.path {
                return "ipfs://".concat(self.cid).concat("/").concat(path)
            }

            return "ipfs://".concat(self.cid)
        }
    }

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)
    pub event Revealed(imageCID: String, metadataCID: String)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64

        init() {
            // We're using 1-indexed token IDs
            self.id = DropzToken.totalSupply + 1
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<DropzToken.IPFSTokenMetadata>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    if (!DropzToken.revealed) {
                        return MetadataViews.Display(
                            name: "Drop #".concat(self.id.toString()),
                            description: DropzToken.description,
                            thumbnail: MetadataViews.IPFSFile(
                                cid: DropzToken.imageCID,
                                path: "placeholder.png"
                            )
                        )
                    } else {
                        return MetadataViews.Display(
                            name: "Drop #".concat(self.id.toString()),
                            description: DropzToken.description,
                            thumbnail: MetadataViews.IPFSFile(
                                cid: DropzToken.imageCID,
                                path: self.id.toString().concat(".png")
                            )
                        )
                    }
                case Type<DropzToken.IPFSTokenMetadata>():
                    if (!DropzToken.revealed) {
                        return DropzToken.IPFSTokenMetadata("placeholder")
                    } else {
                        return DropzToken.IPFSTokenMetadata(self.id.toString())
                    }
            }

            return nil
        }
    }

    pub resource interface DropzTokenCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowDropzToken(id: UInt64): &DropzToken.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow DropzToken reference: the ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: DropzTokenCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @DropzToken.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowDropzToken(id: UInt64): &DropzToken.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &DropzToken.NFT
            }

            return nil
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let DropzToken = nft as! &DropzToken.NFT
            return DropzToken as &AnyResource{MetadataViews.Resolver}
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs and reveal the tokens
    //
    pub resource Admin {

        // mintNFT mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}) {
            pre {
                DropzToken.totalSupply < DropzToken.maximumSupply:
                    "Cannot mint NFT: the maximum supply has been reached"
            }

            // create a new NFT
            var newNFT <- create NFT()

            emit Minted(
                id: newNFT.id
            )

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)

            DropzToken.totalSupply = DropzToken.totalSupply + 1
        }

        // reveal updates the metadata CID and sets the revealed status to true.
        // This can only be executed when the token has not been revealed yet
        pub fun reveal(imageCID: String, metadataCID: String) {
            pre {
                !DropzToken.revealed:
                    "Cannot reveal tokens: Tokens have already been revealed"
            }

            DropzToken.imageCID = imageCID
            DropzToken.metadataCID = metadataCID
            DropzToken.revealed = true

            emit Revealed(imageCID: imageCID, metadataCID: metadataCID)
        }
    }

    init(description: String, maximumSupply: UInt64, imageCID: String, metadataCID: String) {
        // Initialize the total supply
        self.totalSupply = 0

        // Initialize the maximum supply
        self.maximumSupply = maximumSupply

        // Initialize the image CID
        self.imageCID = imageCID

        // Initialize the metadata CID
        self.metadataCID = metadataCID

        // Initialize the description
        self.description = description

        // Initialize the revealed flag
        self.revealed = false

        // Set the named paths
        self.CollectionStoragePath = /storage/DropzTokenCollection
        self.CollectionPublicPath = /public/DropzTokenCollection
        self.AdminStoragePath = /storage/DropzTokenAdmin

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&DropzToken.Collection{NonFungibleToken.CollectionPublic, DropzToken.DropzTokenCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        // Create a Admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
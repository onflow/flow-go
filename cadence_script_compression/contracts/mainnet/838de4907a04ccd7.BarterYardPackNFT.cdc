import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract BarterYardPackNFT: NonFungibleToken {

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Mint(id: UInt64, packPartId: Int, edition: UInt16)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Burn(id: UInt64)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminStoragePath: StoragePath

    access(contract) let packParts: {Int: PackPart}

    pub struct interface SupplyManager {
        pub let maxSupply: UInt16
        pub var totalSupply: UInt16
        pub fun increment(): UInt16 {
            pre {
                self.totalSupply < self.maxSupply :
                    "[SupplyManager](increment): can't increment totalSupply as maxSupply has been reached"
            }
        }
    }

    /// PackPart represents a part of our future werewolf pack.
    /// eg: Alpha, Beta, Omega...
    pub struct PackPart: SupplyManager {
        pub let id: Int
        pub let name: String
        pub let description: String
        pub let ipfsThumbnailCid: String
        pub let ipfsThumbnailPath: String?
        pub let maxSupply: UInt16
        pub var totalSupply: UInt16

        pub fun increment(): UInt16 {
            self.totalSupply = self.totalSupply + 1
            return self.totalSupply
        }

        pub init(
            id: Int,
            name: String,
            description: String,
            ipfsThumbnailCid: String,
            ipfsThumbnailPath: String?,
            maxSupply: UInt16,
            totalSupply: UInt16
        ) {
            self.id = id
            self.name = name
            self.description = description
            self.ipfsThumbnailCid = ipfsThumbnailCid
            self.ipfsThumbnailPath = ipfsThumbnailPath
            self.maxSupply = maxSupply
            self.totalSupply = totalSupply
        }
    }

    pub struct PackMetadataDisplay {
        pub let packPartId: Int
        pub let edition: UInt16
        init(
            packPartId: Int,
            edition: UInt16
        ) {
            self.packPartId = packPartId
            self.edition = edition
        }
    }

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let packPartId: Int

        pub let name: String
        pub let description: String
        pub let ipfsThumbnailCid: String
        pub let ipfsThumbnailPath: String?
        pub let edition: UInt16

        init(
            id: UInt64,
            packPartId: Int,
            name: String,
            description: String,
            ipfsThumbnailCid: String,
            ipfsThumbnailPath: String?,
            edition: UInt16,
        ) {
            self.id = id
            self.packPartId = packPartId
            self.name = name
            self.description = description
            self.ipfsThumbnailCid = ipfsThumbnailCid
            self.ipfsThumbnailPath = ipfsThumbnailPath
            self.edition = edition
        }
    
        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<PackMetadataDisplay>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name,
                        description: self.description,
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.ipfsThumbnailCid,
                            path: self.ipfsThumbnailPath
                        ),
                    )
                case Type<PackMetadataDisplay>():
                    return PackMetadataDisplay(
                        packPartId: self.packPartId,
                        edition: self.edition
                    )
            }

            return nil
        }

        destroy() {
          emit Burn(id: self.id)
        }
    }

    pub resource interface BarterYardPackNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowBarterYardPackNFT(id: UInt64): &BarterYardPackNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow BarterYardPackNFT reference: the ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: BarterYardPackNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
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
            let token <- token as! @BarterYardPackNFT.NFT

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
 
        pub fun borrowBarterYardPackNFT(id: UInt64): &BarterYardPackNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &BarterYardPackNFT.NFT
            }

            return nil
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let BarterYardPackNFT = nft as! &BarterYardPackNFT.NFT
            return BarterYardPackNFT
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
    // able to mint new NFTs
    //
    pub resource Admin {

        // mintNFT mints a new NFT with a new Id
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(
            packPartId: Int,
        ) : @BarterYardPackNFT.NFT {

            let packPart = BarterYardPackNFT.packParts[packPartId]
                ?? panic("[Admin](mintNFT): can't mint nft because invalid packPartId was providen")

            let edition = packPart.increment()

            BarterYardPackNFT.packParts[packPartId] = packPart

            // create a new NFT
            var newNFT <- create NFT(
                id: BarterYardPackNFT.totalSupply,
                packPartId: packPartId,
                name: packPart.name,
                description: packPart.description,
                ipfsThumbnailCid: packPart.ipfsThumbnailCid,
                ipfsThumbnailPath: packPart.ipfsThumbnailPath,
                edition: edition,
            )

            emit Mint(id: newNFT.id, packPartId: packPartId, edition: edition)

            BarterYardPackNFT.totalSupply = BarterYardPackNFT.totalSupply + 1

            return <- newNFT
        }

        // Create a new pack part
        pub fun createNewPack(
            name: String,
            description: String,
            ipfsThumbnailCid: String,
            ipfsThumbnailPath: String?,
            maxSupply: UInt16,
        ) {
            let newPackId = BarterYardPackNFT.packParts.length
            let packPart = PackPart(
                id: newPackId,
                name: name,
                description: description,
                ipfsThumbnailCid: ipfsThumbnailCid,
                ipfsThumbnailPath: ipfsThumbnailPath,
                maxSupply: maxSupply,
                totalSupply: 0,
            )
            BarterYardPackNFT.packParts.insert(key: newPackId, packPart)
        }
    }

    pub fun getPackPartsIds(): [Int] {
        return self.packParts.keys
    }

    pub fun getPackPartById(packPartId: Int): BarterYardPackNFT.PackPart? {
        return self.packParts[packPartId]
    }
    
    init() {
        // Initialize the total supply
        self.totalSupply = 0
        self.packParts = {}

        // Set the named paths
        self.CollectionStoragePath = /storage/BarterYardPackNFTCollection
        self.CollectionPublicPath = /public/BarterYardPackNFTCollection
        self.CollectionPrivatePath = /private/BarterYardPackNFTCollection
        self.AdminStoragePath = /storage/BarterYardPackNFTMinter

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&BarterYardPackNFT.Collection{NonFungibleToken.CollectionPublic, BarterYardPackNFT.BarterYardPackNFTCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        self.account.link<&BarterYardPackNFT.Collection>(BarterYardPackNFT.CollectionPrivatePath, target: BarterYardPackNFT.CollectionStoragePath)

        // Create an Admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}

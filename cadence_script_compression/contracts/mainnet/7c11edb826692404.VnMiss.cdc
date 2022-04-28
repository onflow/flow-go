import MetadataViews from 0x1d7e57aa55817448
import NonFungibleToken from 0x1d7e57aa55817448

import VnMissCandidate from 0x7c11edb826692404

pub contract VnMiss: NonFungibleToken {
    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Minted(to: Address, level: UInt8, tokenId: UInt64, candidateID: UInt64, name: String, description: String, thumbnail: String)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub var BaseURL: String?

    pub enum Level: UInt8 {
        pub case Bronze
        pub case Silver
        pub case Diamond
    }

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let candidateID: UInt64
        pub let level: UInt8
        pub let name: String
        pub let thumbnail: String

        init(
            id: UInt64,
            candidateID: UInt64,
            level: Level,
            name: String,
            thumbnail: String,
        ) {
            self.id = id
            self.candidateID = candidateID
            self.level = level.rawValue
            self.name = name
            self.thumbnail = thumbnail
        }
    
        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            let c = VnMissCandidate.getCandidate(id: self.candidateID)!

            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name:  c.name,
                        description: c.description,
                        thumbnail: MetadataViews.HTTPFile(
                            url: (VnMiss.BaseURL ?? "").concat(self.thumbnail) 
                        )
                    )
            }

            return nil
        }
    }

    pub resource interface VnMissCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowVnMiss(id: UInt64): &VnMiss.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow VnMiss reference: the ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: VnMissCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            let nft <- token as! @VnMiss.NFT

            emit Withdraw(id: nft.id, from: self.owner?.address)

            return <-nft
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @VnMiss.NFT

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
 
        pub fun borrowVnMiss(id: UInt64): &VnMiss.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &VnMiss.NFT
            }

            return nil
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let exampleNFT = nft as! &VnMiss.NFT
            return exampleNFT as &AnyResource{MetadataViews.Resolver}
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
    pub resource NFTMinter {

        // mintNFT mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic},
            candidateID: UInt64,
            level: Level,
            name: String,
            thumbnail: String,
        ) {

            // create a new NFT
            var newNFT <- create NFT(
                id: VnMiss.totalSupply + 1,
                candidateID: candidateID,
                level: level,
                name: name,
                thumbnail: thumbnail,
            )

            let id = newNFT.id
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)

            VnMiss.totalSupply = VnMiss.totalSupply + UInt64(1)

            let c = VnMissCandidate.getCandidate(id: candidateID)!

            emit Minted(to: recipient.owner!.address, level: level.rawValue, tokenId: id, candidateID: candidateID, name: name, description: c.description, thumbnail: (VnMiss.BaseURL ?? "").concat(thumbnail))
        }

        pub fun setBaseUrl(url: String) {
            VnMiss.BaseURL = url
        }
    }

    init() {
        // Initialize the total supply
        self.totalSupply = 0
        self.BaseURL = "https://hhhv-statics.avatarart.io/nfts/"

        // Set the named paths
        self.CollectionStoragePath = /storage/BNVnMissNFTCollection006
        self.CollectionPublicPath = /public/BNVnMissNFTCollection006
        self.MinterStoragePath = /storage/BNVnMissNFTMinter006

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&VnMiss.Collection{NonFungibleToken.CollectionPublic, VnMiss.VnMissCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

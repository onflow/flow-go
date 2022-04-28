/*
    A NFT contract for the Goated Goats trait packs.
    
    Key Callouts: 
    * Unlimited supply of packs
    * Contains an id which represents which drop this pack was from == packID
    * Main id for a pack is auto-increment
    * Redeemable by public function that accepts in a TraitPacksVoucher
      * Takes in pack, burns it, and emits a new event.
    * Have an on/off switch for redeeming packs in case back-end is facing problems and needs to be temporarily turned off
    * Collection-level metadata
    * Edition-level metadata
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import MetadataViews from 0x1d7e57aa55817448

pub contract GoatedGoatsTraitPack: NonFungibleToken {
    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // GoatedGoatsTraitPack Events
    // -----------------------------------------------------------------------

    pub event Mint(id: UInt64)
    pub event Burn(id: UInt64)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------

    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // GoatedGoatsTraitPack Fields
    // -----------------------------------------------------------------------

    pub var name: String

    access(self) var collectionMetadata: { String: String }
    access(self) let idToTraitPackMetadata: { UInt64: TraitPackMetadata }

    pub struct TraitPackMetadata {
        pub let metadata: { String: String }

        init(metadata: { String: String }) {
            self.metadata = metadata
        }
    }

    pub resource NFT : NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let packID: UInt64
        pub let packEditionID: UInt64

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    var ipfsImage = MetadataViews.IPFSFile(cid: "No thumbnail cid set", path: "No thumbnail pat set")
                    if (self.getMetadata().containsKey("thumbnailCID")) {
                        ipfsImage = MetadataViews.IPFSFile(cid: self.getMetadata()["thumbnailCID"]!, path: self.getMetadata()["thumbnailPath"])
                    }
                    return MetadataViews.Display(
                        name: self.getMetadata()["name"] ?? "Goated Goat Trait Pack ".concat(self.id.toString()),
                        description: self.getMetadata()["description"] ?? "No description set",
                        thumbnail: ipfsImage
                    )
            }

            return nil
        }

        pub fun getMetadata(): {String: String} {
            if (GoatedGoatsTraitPack.idToTraitPackMetadata[self.id] != nil) {
                return GoatedGoatsTraitPack.idToTraitPackMetadata[self.id]!.metadata
            } else {
                return {}
            }
        }

        init(id: UInt64, packID: UInt64, packEditionID: UInt64) {
            self.id = id
            self.packID = packID
            self.packEditionID = packEditionID
            emit Mint(id: self.id)
        }

        destroy() {
            emit Burn(id: self.id)
        }
    }

    pub resource interface TraitPackCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTraitPack(id: UInt64): &GoatedGoatsTraitPack.NFT? {
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow GoatedGoatsTraitPack reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: TraitPackCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @GoatedGoatsTraitPack.NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowTraitPack(id: UInt64): &GoatedGoatsTraitPack.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &GoatedGoatsTraitPack.NFT
            } else {
                return nil
            }
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let traitPack = nft as! &GoatedGoatsTraitPack.NFT
            return traitPack as &AnyResource{MetadataViews.Resolver}
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Admin Functions
    // -----------------------------------------------------------------------
    access(account) fun setEditionMetadata(editionNumber: UInt64, metadata: {String: String}) {
        self.idToTraitPackMetadata[editionNumber] = TraitPackMetadata(metadata: metadata)
    }

    access(account) fun setCollectionMetadata(metadata: {String: String}) {
        self.collectionMetadata = metadata
    }

    access(account) fun mint(nftID: UInt64, packID: UInt64, packEditionID: UInt64) : @NonFungibleToken.NFT {
        self.totalSupply = self.totalSupply + 1
        return <-create NFT(id: nftID, packID: packID, packEditionID: packEditionID)
    }

    // -----------------------------------------------------------------------
    // Public Functions
    // -----------------------------------------------------------------------
    pub fun getTotalSupply(): UInt64 {
        return self.totalSupply
    }

    pub fun getName(): String {
        return self.name
    }

    pub fun getCollectionMetadata(): {String: String} {
        return self.collectionMetadata
    }

    pub fun getEditionMetadata(_ edition: UInt64): {String: String} {
        if (self.idToTraitPackMetadata[edition] != nil) {
            return self.idToTraitPackMetadata[edition]!.metadata
        } else {
            return {}
        }
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    init() {
        self.name = "Goated Goats Trait Pack"
        self.totalSupply = 0

        self.collectionMetadata = {}
        self.idToTraitPackMetadata = {}

        self.CollectionStoragePath = /storage/GoatTraitPackCollection
        self.CollectionPublicPath = /public/GoatTraitPackCollection

        emit ContractInitialized()
    }
}
 
/*
    A NFT contract which is redeemable for a Goated Goat NFT.

    Each NFT may contain metadata at the collection level, and at the
    edition level. Metadata is in the form of {String: String} allowing
    for metadata to be added as needed.
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract GoatedGoatsVouchers: NonFungibleToken {
    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // GoatVoucher Events
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
    pub var maxSupply: UInt64

    // -----------------------------------------------------------------------
    // GoatedGoatsVouchers Fields
    // -----------------------------------------------------------------------

    pub var name: String

    access(self) var collectionMetadata: { String: String }
    access(self) let idToVoucherMetadata: { UInt64: VoucherMetadata }

    pub struct VoucherMetadata {
        pub let metadata: { String: String }

        init(metadata: { String: String }) {
            self.metadata = metadata
        }
    }

    pub resource NFT : NonFungibleToken.INFT {
        pub let id: UInt64

        pub fun getMetadata(): {String: String} {
            if (GoatedGoatsVouchers.idToVoucherMetadata[self.id] != nil) {
                return GoatedGoatsVouchers.idToVoucherMetadata[self.id]!.metadata
            } else {
                return {}
            }
        }

        init(id: UInt64) {
            self.id = id
            emit Mint(id: self.id)
        }

        destroy() {
            emit Burn(id: self.id)
        }
    }

    pub resource interface GoatsVoucherCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowVoucher(id: UInt64): &GoatedGoatsVouchers.NFT? {
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow GoatVoucher reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, GoatsVoucherCollectionPublic {
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
            let token <- token as! @GoatedGoatsVouchers.NFT
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

        pub fun borrowVoucher(id: UInt64): &GoatedGoatsVouchers.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &GoatedGoatsVouchers.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Admin Functions
    // -----------------------------------------------------------------------
    access(account) fun setEditionMetadata(editionNumber: UInt64, metadata: {String: String} ) {
        self.idToVoucherMetadata[editionNumber] = VoucherMetadata(metadata: metadata)
    }

    access(account) fun setCollectionMetadata(metadata: {String: String}) {
        self.collectionMetadata = metadata
    }

    access(account) fun mint(nftID: UInt64) : @NonFungibleToken.NFT {
        post {
            self.totalSupply <= self.maxSupply : "Total supply going over max supply with invalid mint."
        }
        self.totalSupply = self.totalSupply + 1
        return <-create NFT(id: nftID)
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
        if (self.idToVoucherMetadata[edition] != nil) {
            return self.idToVoucherMetadata[edition]!.metadata
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
        self.name = "Goated Goats Vouchers"
        self.totalSupply = 0
        self.maxSupply = 10000

        self.collectionMetadata = {}
        self.idToVoucherMetadata = {}

        self.CollectionStoragePath = /storage/GoatedGoatsVoucherCollection
        self.CollectionPublicPath = /public/GoatedGoatsVoucherCollection

        emit ContractInitialized()
    }
}

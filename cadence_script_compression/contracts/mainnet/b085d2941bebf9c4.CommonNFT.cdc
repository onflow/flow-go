import NonFungibleToken from 0x1d7e57aa55817448
import NFTPlus from 0xb085d2941bebf9c4

/**
 * CommonNFT token contract
 */
pub contract CommonNFT : NonFungibleToken, NFTPlus {

    pub var totalSupply: UInt64

    pub var collectionPublicPath: PublicPath
    pub var collectionStoragePath: StoragePath
    pub var minterPublicPath: PublicPath
    pub var minterStoragePath: StoragePath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event Mint(id: UInt64, collection: String, creator: Address, metadata: String, royalties: [NFTPlus.Royalties])
    pub event Destroy(id: UInt64)
    pub event Transfer(id: UInt64, from: Address?, to: Address)

    pub struct Royalties {
        pub let address: Address
        pub let fee: UFix64

        init(address: Address, fee: UFix64) {
            self.address = address
            self.fee = fee
        }
    }

    pub resource NFT: NonFungibleToken.INFT, NFTPlus.WithRoyalties {
        pub let id: UInt64
        pub let creator: Address
        pub let metadata: String
        access(self) let royalties: [NFTPlus.Royalties]

        init(id: UInt64, creator: Address, metadata: String, royalties: [NFTPlus.Royalties]) {
            self.id = id
            self.creator = creator
            self.metadata = metadata
            self.royalties = royalties
        }

        pub fun getRoyalties(): [NFTPlus.Royalties] {
            return self.royalties
        }

        destroy() {
            emit Destroy(id: self.id)
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, NFTPlus.Transferable, NFTPlus.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <- token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @CommonNFT.NFT
            let id: UInt64 = token.id
            let dummy <- self.ownedNFTs[id] <- token
            destroy dummy
            emit Deposit(id: id, to: self.owner?.address)
        }

        pub fun transfer(tokenId: UInt64, to: Capability<&{NonFungibleToken.Receiver}>) {
            let token <- self.ownedNFTs.remove(key: tokenId) ?? panic("Missed NFT")
            emit Withdraw(id: tokenId, from: self.owner?.address)
            to.borrow()!.deposit(token: <- token)
            emit Transfer(id: tokenId, from: self.owner?.address, to: to.address)
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun getRoyalties(id: UInt64): [NFTPlus.Royalties] {
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return (ref as! &NFTPlus.NFT).getRoyalties()
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource Minter {
        pub fun mint(creator: Address, metadata: String, royalties: [NFTPlus.Royalties]): @NonFungibleToken.NFT {
            let token <- create NFT(
                id: CommonNFT.totalSupply,
                creator: creator,
                metadata: metadata,
                royalties: royalties
            )
            CommonNFT.totalSupply = CommonNFT.totalSupply + 1
            emit Mint(id: token.id, collection: token.getType().identifier, creator: creator, metadata: metadata, royalties: royalties)
            return <- token
        }

        pub fun mintTo(creator: Capability<&{NonFungibleToken.Receiver}>, metadata: String, royalties: [NFTPlus.Royalties]): &NonFungibleToken.NFT {
            let token <- create NFT(
                id: CommonNFT.totalSupply,
                creator: creator.address,
                metadata: metadata,
                royalties: royalties
            )
            CommonNFT.totalSupply = CommonNFT.totalSupply + 1
            let tokenRef = &token as &NonFungibleToken.NFT
            emit Mint(id: token.id, collection: token.getType().identifier, creator: creator.address, metadata: metadata, royalties: royalties)
            creator.borrow()!.deposit(token: <- token)
            return tokenRef
        }
    }

    pub fun receiver(_ address: Address): Capability<&{NonFungibleToken.Receiver}> {
        return getAccount(address).getCapability<&{NonFungibleToken.Receiver}>(self.collectionPublicPath)
    }

    pub fun collectionPublic(_ address: Address): Capability<&{NonFungibleToken.CollectionPublic}> {
        return getAccount(address).getCapability<&{NonFungibleToken.CollectionPublic}>(self.collectionPublicPath)
    }

    pub fun minter(): Capability<&Minter> {
        return self.account.getCapability<&Minter>(self.minterPublicPath)
    }

    init() {
        self.totalSupply = 0
        self.collectionPublicPath = /public/CommonNFTCollection
        self.collectionStoragePath = /storage/CommonNFTCollection
        self.minterPublicPath = /public/CommonNFTMinter
        self.minterStoragePath = /storage/CommonNFTMinter

        let minter <- create Minter()
        self.account.save(<- minter, to: self.minterStoragePath)
        self.account.link<&Minter>(self.minterPublicPath, target: self.minterStoragePath)

        let collection <- self.createEmptyCollection()
        self.account.save(<- collection, to: self.collectionStoragePath)
        self.account.link<&{NonFungibleToken.CollectionPublic,NonFungibleToken.Receiver}>(self.collectionPublicPath, target: self.collectionStoragePath)

        emit ContractInitialized()
    }
}

import NonFungibleToken from 0x1d7e57aa55817448

pub contract RCRDSHPNFT: NonFungibleToken {

    pub var totalSupply: UInt64
    pub let minterStoragePath: StoragePath
    pub let collectionStoragePath: StoragePath
    pub let collectionPublicPath: PublicPath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        pub var metadata: {String: String}

        init(initID: UInt64, metadata: {String : String}) {
            self.id = initID
            self.metadata = metadata
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

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
            let token <- token as! @RCRDSHPNFT.NFT
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

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {

        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, meta: {String : String}) {
            var newNFT <- create NFT(initID: RCRDSHPNFT.totalSupply, metadata: meta)
            recipient.deposit(token: <-newNFT)
            RCRDSHPNFT.totalSupply = RCRDSHPNFT.totalSupply + UInt64(1)
        }
    }

    init() {
        self.totalSupply = 0

        self.minterStoragePath = /storage/RCRDSHPNFTMinter
        self.collectionStoragePath = /storage/RCRDSHPNFTCollection
        self.collectionPublicPath  = /public/RCRDSHPNFTCollection

        let collection <- create Collection()
        self.account.save(<-collection, to: self.collectionStoragePath)

        self.account.link<&{NonFungibleToken.CollectionPublic}>(
            self.collectionPublicPath,
            target: self.collectionStoragePath
        )

        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.minterStoragePath)

        emit ContractInitialized()
    }
}

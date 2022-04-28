import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract RCRDSHPNFT: NonFungibleToken {
    pub var totalSupply: UInt64
    pub let minterStoragePath: StoragePath
    pub let collectionStoragePath: StoragePath
    pub let collectionPublicPath: PublicPath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Burn(id: UInt64, from: Address?)
    pub event Sale(id: UInt64, price: UInt64)

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub var metadata: {String: String}

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    let name = self.metadata["name"]!
                    let serial = self.metadata["serial_number"]!

                    return MetadataViews.Display(
                        name: name.concat(" #").concat(serial),
                        description: self.metadata["description"]!,
                        thumbnail: MetadataViews.HTTPFile(
                            url: self.metadata["uri"]!.concat("/thumbnail")
                        )
                    )
            }
            return nil
        }

        init(initID: UInt64, metadata: {String : String}) {
            self.id = initID
            self.metadata = metadata
        }
    }

    pub resource interface RCRDSHPNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowRCRDSHPNFT(id: UInt64): &RCRDSHPNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow RCRDSHPNFT reference: the ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: RCRDSHPNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("withdraw - missing NFT")

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

        pub fun sale(id: UInt64, price: UInt64): @NonFungibleToken.NFT {
            emit Sale(id: id, price: price)
            return <-self.withdraw(withdrawID: id)
        }

        pub fun burn(burnID: UInt64){
            let token <- self.ownedNFTs.remove(key: burnID) ?? panic("burn - missing NFT")

            emit Burn(id: token.id, from: self.owner?.address)
            destroy token
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowRCRDSHPNFT(id: UInt64): &RCRDSHPNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &RCRDSHPNFT.NFT
            }

            return nil
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let rcrdshpNFT = nft as! &RCRDSHPNFT.NFT
            return rcrdshpNFT as &AnyResource{MetadataViews.Resolver}
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

        self.account.link<&RCRDSHPNFT.Collection{NonFungibleToken.CollectionPublic, RCRDSHPNFT.RCRDSHPNFTCollectionPublic}>(
            self.collectionPublicPath,
            target: self.collectionStoragePath
        )

        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.minterStoragePath)

        emit ContractInitialized()
    }
}

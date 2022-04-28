import NonFungibleToken from 0x1d7e57aa55817448

pub contract MatrixWorldFlowFestNFT: NonFungibleToken {
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, name: String, description:String, animationUrl:String, hash: String, type: String)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64

    pub resource interface NFTPublic {
        pub let id: UInt64
        pub let metadata: Metadata
    }

    pub struct Metadata {
        pub let name: String
        pub let description: String
        pub let animationUrl: String
        pub let hash: String
        pub let type: String

        init(name: String, description: String, animationUrl: String, hash: String, type: String) {
            self.name = name
            self.description = description
            self.animationUrl = animationUrl
            self.hash = hash
            self.type = type
        }
    }

   pub resource NFT: NonFungibleToken.INFT, NFTPublic {
        pub let id: UInt64
        pub let metadata: Metadata
        init(initID: UInt64,metadata: Metadata) {
            self.id = initID
            self.metadata=metadata
        }
    }

    pub resource interface MatrixWorldFlowFestNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowVoucher(id: UInt64): &MatrixWorldFlowFestNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow MatrixWorldVoucher reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: MatrixWorldFlowFestNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @MatrixWorldFlowFestNFT.NFT

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

        pub fun borrowVoucher(id: UInt64): &MatrixWorldFlowFestNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MatrixWorldFlowFestNFT.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub struct NftData {
        pub let metadata: MatrixWorldFlowFestNFT.Metadata
        pub let id: UInt64
        init(metadata: MatrixWorldFlowFestNFT.Metadata, id: UInt64) {
            self.metadata= metadata
            self.id=id
        }
    }

    pub fun getNft(address:Address) : [NftData] {
        var nftData: [NftData] = []
        let account = getAccount(address)

        if let nftCollection = account.getCapability(self.CollectionPublicPath).borrow<&{MatrixWorldFlowFestNFT.MatrixWorldFlowFestNFTCollectionPublic}>()  {
            for id in nftCollection.getIDs() {
                var nft = nftCollection.borrowVoucher(id: id)
                nftData.append(NftData(metadata: nft!.metadata,id: id))
            }
        }
        return nftData
    }

	pub resource NFTMinter {
		pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic},
            name: String,
            description: String,
            animationUrl: String
            hash: String,
            type: String) {
            emit Minted(id: MatrixWorldFlowFestNFT.totalSupply, name: name, description: description, animationUrl: animationUrl, hash: hash, type: type)

			recipient.deposit(token: <-create MatrixWorldFlowFestNFT.NFT(
			    initID: MatrixWorldFlowFestNFT.totalSupply,
			    metadata: Metadata(
                    name: name,
                    description:description,
                    animationUrl: animationUrl,
                    hash: hash,
                    type: type
                )))

            MatrixWorldFlowFestNFT.totalSupply = MatrixWorldFlowFestNFT.totalSupply + (1 as UInt64)
		}
	}

    init() {
        self.CollectionStoragePath = /storage/MatrixWorldFlowFestNFTCollection
        self.CollectionPublicPath = /public/MatrixWorldFlowFestNFTCollection
        self.MinterStoragePath = /storage/MatrixWorldFlowFestNFTrMinter

        self.totalSupply = 0

        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

import NonFungibleToken from 0x1d7e57aa55817448

/*
    XXX: CheezeNFT is slightly modified ExampleNFT from flow-nft.
    Imports only working when you deploy contract.
*/

pub contract CheezeNFT: NonFungibleToken {
    pub let minterStorage: StoragePath

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        access(self) let metadata: {String: String}

        pub fun getMetadata(): {String:String} {
            return self.metadata
        }


        init(initID: UInt64, metadata: {String: String}) {
            self.id = initID
            self.metadata = metadata
        }
    }

    pub resource interface CollectionPublic {
        pub fun getNftMetadata(id: UInt64): {String: String}
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
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

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @CheezeNFT.NFT
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

        pub fun getNftMetadata(id: UInt64): {String: String} {
            let r = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return (r as! &CheezeNFT.NFT).getMetadata()
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: {String: String}) {
            var newNFT <- create NFT(initID: CheezeNFT.totalSupply, metadata: metadata)
            recipient.deposit(token: <-newNFT)
            CheezeNFT.totalSupply = CheezeNFT.totalSupply + UInt64(1)
        }
    }

    init() {
        self.totalSupply = 0
        self.minterStorage = /storage/NFTMinter

        self.createPrivateMinter()

        emit ContractInitialized()
    }

    access(contract) fun createPrivateMinter() {
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.minterStorage)
    }
}

// SPDX-License-Identifier: UNLICENSED
import NonFungibleToken from 0x1d7e57aa55817448

pub contract NonFungibleBeatoken: NonFungibleToken {

    pub var totalSupply: UInt64

    pub let storageCollection: StoragePath
    pub let publicReceiver: PublicPath
    pub let storageMinter: StoragePath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event CreatedNft(id: UInt64)

    pub struct Metadata {
        pub let name: String
        pub let ipfs_hash: String
        pub let token_uri: String
        pub let description: String

        init(name: String, 
            ipfs_hash:String, 
            token_uri:String,
            description:String) {

            self.name=name
            self.ipfs_hash=ipfs_hash
            self.token_uri=token_uri
            self.description=description
        }
    }

    pub resource NFT: NonFungibleToken.INFT {

        pub let id: UInt64
        pub let metadata: Metadata

        init(initID: UInt64, metadata: Metadata) {
            self.id = initID
            self.metadata = metadata
        }
    }

     pub resource interface BeatokenCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowBeatokenNFT(id: UInt64): &NonFungibleBeatoken.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow BeatokenNFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: BeatokenCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: withdrawID, from: self.owner?.address)
            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @NFT
            emit Deposit(id: token.id, to: self.owner?.address)
            let oldToken <- self.ownedNFTs[token.id] <- token
            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowBeatokenNFT(id: UInt64): &NonFungibleBeatoken.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &NonFungibleBeatoken.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {

        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, name: String, ipfs_hash: String, token_uri: String, description: String) {
            
            NonFungibleBeatoken.totalSupply = NonFungibleBeatoken.totalSupply + (1 as UInt64)
            
            let id = NonFungibleBeatoken.totalSupply
            let newNFT <- create NFT(
                initID: id,
                metadata: Metadata(
                  name: name,
                  ipfs_hash: ipfs_hash,
                  token_uri: token_uri.concat(id.toString()),
                  description: description
                )
            )

            recipient.deposit(token: <-newNFT)

            emit CreatedNft(id: id)
        }
    }

    init() {
        self.totalSupply = 0;

        // Define paths
        self.storageCollection = /storage/beatokenNFTCollection
        self.publicReceiver = /public/beatokenNFTReceiver
        self.storageMinter = /storage/beatokenNFTMinter

        // Create, store and explose capability for collection
        let collection <- self.createEmptyCollection()
        self.account.save(<- collection, to: self.storageCollection)
        self.account.link<&NonFungibleBeatoken.Collection{BeatokenCollectionPublic, NonFungibleToken.CollectionPublic}>
            (self.publicReceiver, target: self.storageCollection)

        self.account.save(<-create NFTMinter(), to: self.storageMinter)

        emit ContractInitialized()
    }
}
 
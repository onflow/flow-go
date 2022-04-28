/*
  Electables.cdc

  Description: Central Smart Contract for Electables

  This contract defines the Electables NFT, adds functionality for minting an Electable and defines the public Electables Collection. There are transactions in this repository's transactions directory that can be used to
  - Set up an account to receive Electables, i.e. create a public Electable Collection path in an account
  - Mint a new Electable
  - Transfer an Electable from on account's collection to another account's collection

  There are also scripts that can:
  - Get a the count of Electables in an account's collection
  - Get the total supply of Electables
  - Get an Electable by id and account address
  - Get an Electable's metadata by id and account address

  This contract was based on KittyItems.cdc and ExampleNFT.cdc: https://github.com/onflow/kitty-items and https://github.com/onflow/flow-nft/blob/master/contracts/ExampleNFT.cdc, respectively.
*/

import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract Electables: NonFungibleToken {
    pub var totalSupply: UInt64

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, timestamp: UFix64)
    
    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let electableType: String
        pub let name: String
        pub let description: String
        pub let thumbnail: String

        access(self) var attributes: {String: String}
        access(self) var data: {String: String}

        init(
            initID: UInt64,
            electableType: String
            name: String,
            description: String,
            thumbnail: String,
            attributes: {String: String},
            data: {String: String}
        ) {
            self.id = initID
            self.electableType = electableType
            self.name = name
            self.description = description
            self.thumbnail = thumbnail
            self.attributes = attributes
            self.data = data
        }

        pub fun getAttributes(): {String: String} {
            return self.attributes
        }

        pub fun getData(): {String: String} {
            return self.data
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                  return MetadataViews.Display(
                      name: self.name,
                      description: self.description,
                      thumbnail: MetadataViews.IPFSFile(
                          cid: self.thumbnail,
                          path: nil
                      )
                  )
            }
            return nil
        }

        access(contract) fun overwriteAttributes(attributes: {String: String}) {
            self.attributes = attributes
        }

        access(contract) fun overwriteData(data: {String: String}) {
            self.data = data
        }
    }

    pub resource interface ElectablesPublicCollection {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        // borrowNFT will return a NonFungibleToken.NFT which only allows for accessing the id field.
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        // borrowElectable will return a Electables.NFT which exposes the public fields defined in this file on Electables.NFT. Right now
        // Electables.NFT only has an id field, but will later include the electable attributes as well.
        pub fun borrowElectable(id: UInt64): &Electables.NFT?
    }

    pub resource Collection: ElectablesPublicCollection, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // Provider Interface
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <- token
        }

        // Receiver and CollectionPublic Interfaces
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // CollectionPublic Interface
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            pre {
                self.ownedNFTs[id] != nil: "NFT does not exist in the collection!"
            }

            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowElectable(id: UInt64): &Electables.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Electable reference: The ID of the returned reference is incorrect"
            }

            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Electables.NFT
            } else {
                return nil
            }
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let electable = nft as! &Electables.NFT
            return electable as &AnyResource{MetadataViews.Resolver}
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic},
            electableType: String,
            name: String,
            description: String,
            thumbnail: String,
            attributes: {String: String},
            data: {String: String},
        ) {
            emit Minted(id: Electables.totalSupply, timestamp: getCurrentBlock().timestamp)

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <- create Electables.NFT(
                initID: Electables.totalSupply,
                electableType: electableType,
                name: name,
                description: description,
                thumbnail: thumbnail,
                attributes: attributes,
                data: data)
            )

            Electables.totalSupply = Electables.totalSupply + (1 as UInt64)
        }

        pub fun overwriteNFTAttributes(nftReference: &NFT, attributes: {String: String}) {
            nftReference.overwriteAttributes(attributes: attributes)
        }

        pub fun overwriteNFTData(nftReference: &NFT, data: {String: String}) {
            nftReference.overwriteData(data: data)
        }
    }

    init () {
        self.CollectionStoragePath = /storage/electablesCollection
        self.CollectionPublicPath = /public/electablesCollection
        self.MinterStoragePath = /storage/electableMinter
        self.totalSupply = 0

        // Create the Minter and put it in Storage
        let minter <- create NFTMinter()
        self.account.save(<- minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}
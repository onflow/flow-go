//  SPDX-License-Identifier: UNLICENSED
//
//  Description: Smart Contract for Anique central.
//  All NFT contracts of Anique will inherit this.
//
//  authors: Atsushi Otani atsushi.ootani@anique.jp
//
import NonFungibleToken from 0x1d7e57aa55817448

pub contract interface Anique {

    // Event that emitted when the NFT contract is initialized
    //
    pub event ContractInitialized()

    // Interface that Anique NFTs have to conform to
    //
    pub resource interface INFT {
    }

    // Requirement that all conforming NFT smart contracts have
    // to define a resource called NFT that conforms to INFT
    pub resource NFT: INFT {
        pub let id: UInt64
    }

    // The interface that Anique NFT contract's admin
    pub resource Admin {
    }

    // Requirement for the the concrete resource type
    // to be declared in the implementing Anique contracts
    // mainly used by AniqueMarket.
    // Cooporative with NonFungibleToken.Collection because each
    // Anique NFT contract's Collection should implement both
    // NonFungibleToken.Collection and Anique.Collection
    pub resource Collection {

        // Dictionary to hold the NFTs in the Collection
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT)

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64]

        // Returns a borrowed reference to an NFT in the collection
        // so that the caller can read data and call methods from it
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            pre {
                self.ownedNFTs[id] != nil: "NFT does not exist in the collection!"
            }
        }

        // Returns a borrowed reference to an Anique.NFT in the collection
        // so that the caller can read data and call methods from it
        pub fun borrowAniqueNFT(id: UInt64): auth &Anique.NFT {
            pre {
                self.ownedNFTs[id] != nil: "Anique NFT does not exist in the collection!"
            }
        }
    }
}

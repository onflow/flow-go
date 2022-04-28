import NonFungibleToken from 0x1d7e57aa55817448

pub contract interface NFTPlus {

    pub event Transfer(id: UInt64, from: Address?, to: Address)

    pub fun receiver(_ address: Address): Capability<&{NonFungibleToken.Receiver}>
    pub fun collectionPublic(_ address: Address): Capability<&{NonFungibleToken.CollectionPublic}>

    pub struct Royalties {
        pub let address: Address
        pub let fee: UFix64
    }

    pub resource interface WithRoyalties {
        pub fun getRoyalties(): [Royalties]
    }

    pub resource interface Transferable {
        pub fun transfer(tokenId: UInt64, to: Capability<&{NonFungibleToken.Receiver}>)
    }

    pub resource NFT: NonFungibleToken.INFT, WithRoyalties {
        pub let id: UInt64
        pub fun getRoyalties(): [Royalties]
    }

    pub resource interface CollectionPublic {
        pub fun getRoyalties(id: UInt64): [Royalties]
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, Transferable, CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun getRoyalties(id: UInt64): [Royalties]
    }

}

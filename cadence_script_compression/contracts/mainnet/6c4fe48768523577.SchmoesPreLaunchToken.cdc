/*
    Description: This is an NFT that will be issued to anyone who visits the Schmoes website before 
    the official launch of the Shmoes NFT
*/

import NonFungibleToken from 0x1d7e57aa55817448


pub contract SchmoesPreLaunchToken: NonFungibleToken {
    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------

    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // SchmoesPreLaunchToken Fields
    // -----------------------------------------------------------------------
    
    // NFT level metadata
    pub var name: String
    pub var imageUrl: String
    pub var isSaleActive: Bool

    pub resource NFT : NonFungibleToken.INFT {
        pub let id: UInt64

        init(initID: UInt64) {
            self.id = initID
        }
    }

    /*
        This collection only allows the storage of a single NFT
     */
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
            pre {
                self.ownedNFTs.length == 0 : "Account already has an NFT."
            }

            let token <- token as! @SchmoesPreLaunchToken.NFT
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

    // -----------------------------------------------------------------------
    // Admin Functions
    // -----------------------------------------------------------------------
    access(account) fun setImageUrl(_ newImageUrl: String) {
        self.imageUrl = newImageUrl
    }

    access(account) fun setIsSaleActive(_ newIsSaleActive: Bool) {
        self.isSaleActive = newIsSaleActive
    }

    // -----------------------------------------------------------------------
    // Public Functions
    // -----------------------------------------------------------------------
    pub fun mint() : @SchmoesPreLaunchToken.NFT {
        pre {
            self.isSaleActive : "Sale is not active"
        }
        let id = SchmoesPreLaunchToken.totalSupply + (1 as UInt64)
        let newNFT: @SchmoesPreLaunchToken.NFT <- create SchmoesPreLaunchToken.NFT(id: id)
        SchmoesPreLaunchToken.totalSupply = id
        return <-newNFT
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    init() {
        self.name = "SchmoesPreLaunchToken"
        self.imageUrl = ""

        self.isSaleActive = false
        self.totalSupply = 0

        self.CollectionStoragePath = /storage/SchmoesPreLaunchTokenCollection
        self.CollectionPublicPath = /public/SchmoesPreLaunchTokenCollection

        emit ContractInitialized()
    }
}

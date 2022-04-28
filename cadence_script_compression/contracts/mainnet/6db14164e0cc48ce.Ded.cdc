import NonFungibleToken from 0x1d7e57aa55817448

pub contract Ded: NonFungibleToken {
    
    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, type: String, uri: String, minter: Address)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64

    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        pub let type: String
        pub let uri: String
        pub let minter: Address

        init(initID: UInt64, initType: String, initUri: String, initMinter: Address) {
            self.id = initID
            self.type = initType
            self.uri = initUri
            self.minter = initMinter

            emit Minted(id: self.id, type: self.type, uri: self.uri, minter: self.minter)
        }
    }

    pub struct AccountItem {
        pub let itemID: UInt64
        pub let type: String
        pub let uri: String
        pub let minter: Address
        pub let resourceID: UInt64
        pub let owner: Address

        init(itemID: UInt64, itemType: String, itemUri: String, itemMinter: Address, resourceID: UInt64, owner: Address) {
            self.itemID = itemID
            self.type = itemType
            self.uri = itemUri
            self.minter = itemMinter
            self.resourceID = resourceID
            self.owner = owner
        }
    }


    pub resource interface DedCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowDed(id: UInt64): &Ded.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Ded reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: DedCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Ded.NFT

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
        
        pub fun borrowDed(id: UInt64): &Ded.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Ded.NFT
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

    pub resource NFTMinter {

        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, minter: Address, type: String, uri: String) {
            let newNFT: @Ded.NFT <- create Ded.NFT(initID: Ded.totalSupply, initType: type, initUri: uri, initMinter: minter)

			recipient.deposit(token: <- newNFT)

            Ded.totalSupply = Ded.totalSupply + (1 as UInt64)
		}
	}

    pub fun fetch(_ from: Address, itemID: UInt64): &Ded.NFT? {
        let collection = getAccount(from)
            .getCapability(Ded.CollectionPublicPath)
            .borrow<&Ded.Collection{Ded.DedCollectionPublic}>()
            ?? panic("Couldn't get collection")
        return collection.borrowDed(id: itemID)
    }

    init() {
        self.CollectionStoragePath = /storage/DedCollection
        self.CollectionPublicPath = /public/DedCollection
        self.MinterStoragePath = /storage/DedMinter

        self.totalSupply = 0

        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)
        emit ContractInitialized()
    }

}
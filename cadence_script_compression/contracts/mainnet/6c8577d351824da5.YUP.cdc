/**
* SPDX-License-Identifier: UNLICENSED
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract YUP: NonFungibleToken {

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, influencerID: UInt32, editionID: UInt32, serialNumber: UInt32, url: String)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let influencerID: UInt32
        pub let editionID: UInt32
        pub let serialNumber: UInt32
        pub let url: String

        init(initID: UInt64, initInfluencerID: UInt32, initEditionID: UInt32, initSerialNumber: UInt32, initUrl: String) {
            self.id = initID
            self.influencerID = initInfluencerID
            self.editionID = initEditionID
            self.serialNumber = initSerialNumber
            self.url = initUrl
        }
    }

    pub resource interface YUPCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowYUP(id: UInt64): &YUP.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow YUP reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: YUPCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @YUP.NFT
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

        pub fun borrowYUP(id: UInt64): &YUP.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &YUP.NFT
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

		pub fun mintYUP(recipient: &{NonFungibleToken.CollectionPublic}, influencerID: UInt32, editionID: UInt32, serialNumber: UInt32, url: String) {

            emit Minted(id: YUP.totalSupply, influencerID: influencerID, editionID: editionID, serialNumber: serialNumber, url: url)

            recipient.deposit(token: <-create YUP.NFT(initID: YUP.totalSupply,
                                                        initInfluencerID: influencerID,
                                                        initEditionID: editionID,
                                                        initSerialNumber: serialNumber,
                                                        initUrl: url))
            YUP.totalSupply = YUP.totalSupply + (1 as UInt64)
        }

	}

    pub fun fetch(_ from: Address, itemID: UInt64): &YUP.NFT? {
        let collection = getAccount(from)
            .getCapability(YUP.CollectionPublicPath)!
            .borrow<&YUP.Collection{YUP.YUPCollectionPublic}>()
            ?? panic("Couldn't get collection")
        return collection.borrowYUP(id: itemID)
    }

	init() {
        self.CollectionStoragePath = /storage/YUPMobileAppCollection
        self.CollectionPublicPath = /public/YUPMobileAppCollection
        self.MinterStoragePath = /storage/YUPMobileAppMinter
        self.totalSupply = 0
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)
        emit ContractInitialized()
	}

}

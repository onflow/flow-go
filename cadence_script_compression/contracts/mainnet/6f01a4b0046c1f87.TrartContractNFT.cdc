// SPDX-License-Identifier: MIT

import NonFungibleToken from 0x1d7e57aa55817448

pub contract TrartContractNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
	
    pub event Mint(id: UInt64)
    pub event Burn(id: UInt64)

    // -----------------------------------------------------------------------
    // fields.
    // -----------------------------------------------------------------------
	
    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64

    access(self) let metadatas: {UInt64: Metadata} 

    // -----------------------------------------------------------------------
    // Metadata
    // -----------------------------------------------------------------------

    // Metadatas are content data of NFTs, those are public data. Anyone can query this to find the detail. 
    // Initialize the metadata value directly during init phase in the NFT resource section. There is no remove function since as a proof of existence.
    pub struct Metadata {
       pub let cardID: UInt64
       pub let data: {String: String} 
	   
       init(cardID: UInt64, data: {String:String}){
           self.cardID = cardID
           self.data = data
       }
    }

    // Get all metadatas
    pub fun getMetadatas(): {UInt64: Metadata} {
        return self.metadatas;
    }

    pub fun getMetadatasCount(): UInt64 {
        return UInt64(self.metadatas.length)
    }

    pub fun getMetadataForCardID(cardID: UInt64): Metadata? {
        return self.metadatas[cardID]
    }
	
    // -----------------------------------------------------------------------
    // NFT
    // -----------------------------------------------------------------------
	
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        init(initID: UInt64, metadata: {String: String}) {
            TrartContractNFT.totalSupply = TrartContractNFT.totalSupply + (1 as UInt64)
            TrartContractNFT.metadatas[initID] = Metadata(cardID: initID, data: metadata)

            self.id = initID
            emit Mint(id: self.id)
        }

        pub fun getCardMetadata(): Metadata? {
            return TrartContractNFT.getMetadataForCardID(cardID: self.id)
        }

        destroy(){
            TrartContractNFT.totalSupply = TrartContractNFT.totalSupply - (1 as UInt64)
            emit Burn(id: self.id)
        }
    }

    // createNFT
    access(account) fun createNFT(cardID: UInt64, metadata: {String: String}): @NFT {
        return <- create NFT(initID: cardID, metadata: metadata)
    }

	// -----------------------------------------------------------------------
    // Collection
    // -----------------------------------------------------------------------
	
    pub resource interface ICardCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
		
		pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun borrowCard(id: UInt64): &TrartContractNFT.NFT? {
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow NFT reference: The ID reference is incorrect"
            }
        }
    }

    pub resource Collection: ICardCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: NFT does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            
            var batchCollection <- create Collection()
            
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }

            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @TrartContractNFT.NFT

            let id: UInt64 = token.id

            let oldToken <- self.ownedNFTs[id] <- token

            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowCard(id: UInt64): &TrartContractNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TrartContractNFT.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // -----------------------------------------------------------------------
    // minter
    // -----------------------------------------------------------------------
    
    pub resource NFTMinter {

        pub fun newNFT(cardID: UInt64, data: {String: String}): @NFT {
			return <- TrartContractNFT.createNFT(cardID: cardID, metadata: data)
        }

        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, cardID: UInt64, data: {String: String}) {
			recipient.deposit(token: <- TrartContractNFT.createNFT(cardID: cardID, metadata: data))
        }
    }

    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/TrartContractNFTCollection
        self.CollectionPublicPath = /public/TrartContractNFTCollection
        self.MinterStoragePath = /storage/TrartContractNFTMinter

        // Initialize the member variants
        self.totalSupply = 0
        self.metadatas = {}

         //collection
        self.account.save(<- create Collection(), to: self.CollectionStoragePath)
        self.account.link<&TrartContractNFT.Collection{NonFungibleToken.CollectionPublic, TrartContractNFT.ICardCollectionPublic}>(TrartContractNFT.CollectionPublicPath, target: TrartContractNFT.CollectionStoragePath)

        // minter
        self.account.save(<- create NFTMinter(), to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

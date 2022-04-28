/*
    Description: Central Smart Contract for 710 Phantom

    author: Bilal Shahid bilal@zay.codes
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract SevenOneZeroPhantomNFT: NonFungibleToken {
    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)


    // -----------------------------------------------------------------------
    // 710 Phantom Events
    // -----------------------------------------------------------------------

    pub event Minted(id: UInt64)
    pub event Destroyed(id: UInt64)
    pub event PhantomDataUpdated(nftID: UInt64)


    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------

    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // SevenOneZeroPhantomNFT Fields
    // -----------------------------------------------------------------------
    
    // NFT level metadata
    access(self) var name: String

    access(self) var externalURL: String

    access(self) var maxAmount: UInt64

    // Variable size dictonary of PhantomData structs
    access(self) var phantomData: {UInt64: PhantomData}

    pub enum PhantomTrait: UInt8 {
        pub case cloak
        pub case background
        pub case faceCovering
        pub case eyeCovering
        pub case necklace
        pub case headCovering
        pub case mouthPiece
        pub case earring
        pub case poster
    }

    // -----------------------------------------------------------------------
    // SevenOneZeroPhantomNFT Struct Fields
    // -----------------------------------------------------------------------
    pub struct PhantomData {
        
        access(self) let metadata: {String: String}
        
        access(self) let ipfsMetadataHash: String

        access(self) let traits: {PhantomTrait: String}

        init(metadata: {String: String}, ipfsMetadataHash: String, traits: {PhantomTrait: String}) {
            self.metadata = metadata
            self.ipfsMetadataHash = ipfsMetadataHash
            self.traits = traits
        }  

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }  

        pub fun getIpfsMetadataHash(): String {
            return self.ipfsMetadataHash
        }

        pub fun getTraits(): {PhantomTrait: String} { 
            return self.traits
        }
    }

    // -----------------------------------------------------------------------
    // SevenOneZeroPhantomNFT Resource Interfaces
    // -----------------------------------------------------------------------
    pub resource interface SevenOneZeroPhantomNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowSevenOneZeroPhantomNFT(id: UInt64): &SevenOneZeroPhantomNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow SevenOneZeroPhantomNFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Resources
    // -----------------------------------------------------------------------
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        init(id: UInt64) {       
            self.id = id
            emit Minted(id: self.id)
        }

        destroy() {         
            emit Destroyed(id: self.id)
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, SevenOneZeroPhantomNFTCollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Cannot withdraw: NFT does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // Currently entire doesn't fail if one fails
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }

            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SevenOneZeroPhantomNFT.NFT

            let id = token.id

            let oldToken <- self.ownedNFTs[id] <- token
            
            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // Currently entire doesn't fail if one fails
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

        pub fun borrowSevenOneZeroPhantomNFT(id: UInt64) : &SevenOneZeroPhantomNFT.NFT? {
            if self.ownedNFTs[id] == nil {
                return nil
            } else {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SevenOneZeroPhantomNFT.NFT
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }


    }

    // -----------------------------------------------------------------------
    // SevenOneZeroPhantomNFT Resources
    // -----------------------------------------------------------------------

    pub resource Admin {
        
        pub fun mint(recipient: &{SevenOneZeroPhantomNFT.SevenOneZeroPhantomNFTCollectionPublic}) : UInt64 {
            pre {
                SevenOneZeroPhantomNFT.totalSupply <=  SevenOneZeroPhantomNFT.maxAmount : "NFT max amount reached"
            }
            
            let id = SevenOneZeroPhantomNFT.totalSupply + (1 as UInt64)
            
            let newNFT: @SevenOneZeroPhantomNFT.NFT <- create SevenOneZeroPhantomNFT.NFT(id: id)
            
            recipient.deposit(token: <-newNFT)

            SevenOneZeroPhantomNFT.totalSupply = id

            return id

        }

        pub fun batchMint(recipient: &{SevenOneZeroPhantomNFT.SevenOneZeroPhantomNFTCollectionPublic}, amount: UInt64) : [UInt64] {
            pre {
                amount > 0: "Dataset cannot be empty"
                SevenOneZeroPhantomNFT.totalSupply + amount <=  SevenOneZeroPhantomNFT.maxAmount : "Input data has too many values"
            }

            var nftIDs : [UInt64] = []
            var i : UInt64 = 0
            while i < amount {
                nftIDs.append(self.mint(recipient: recipient))
                i = i + 1
            }
            return nftIDs
        }

        access(self) fun updatePhantomData(nftID: UInt64, ipfsMetadataHash: String, metadata: {String:String}, traits: {SevenOneZeroPhantomNFT.PhantomTrait: String}) {
            let newPhantomData = PhantomData(
                metadata: metadata, 
                ipfsMetadataHash: ipfsMetadataHash, 
                traits: traits
            )
            SevenOneZeroPhantomNFT.phantomData[nftID] = newPhantomData

            emit PhantomDataUpdated(nftID: nftID)
        }

        pub fun batchUpdatePhantomData(nftIDs: [UInt64], ipfsMetadataHashes: [String], metadata: [{String:String}], traits: [{SevenOneZeroPhantomNFT.PhantomTrait: String}]) {
            var i = 0
            while i < nftIDs.length {
                self.updatePhantomData(nftID: nftIDs[i], ipfsMetadataHash: ipfsMetadataHashes[i], metadata: metadata[i], traits: traits[i])
                i = i + 1
            }
        }


    }

    // -----------------------------------------------------------------------
    // SevenOneZeroPhantomNFT Functions
    // -----------------------------------------------------------------------

    pub fun getName(): String {
        return self.name
    }

    pub fun getExternalURL(): String {
        return self.externalURL
    }

    pub fun getMaxAmount(): UInt64 {
        return self.maxAmount
    }

    pub fun getAllPhantomData(): {UInt64: PhantomData} {
        return self.phantomData
    }

    pub fun getPhantomData(id: UInt64): PhantomData {
        return SevenOneZeroPhantomNFT.phantomData[id]!
    }

    pub fun getPhantomMetadata(id: UInt64): {String: String} {
        return SevenOneZeroPhantomNFT.phantomData[id]!.getMetadata()
    }

    pub fun getPhantomIpfsHash(id: UInt64): String {
        return SevenOneZeroPhantomNFT.phantomData[id]!.getIpfsMetadataHash()
    }

    pub fun getPhantomTraits(id: UInt64): {PhantomTrait: String} {
        return SevenOneZeroPhantomNFT.phantomData[id]!.getTraits()
    }

    pub fun getPhantomHasTrait(id: UInt64, lookupTrait: PhantomTrait, lookupValue: String): Bool {
        let nftTrait = SevenOneZeroPhantomNFT.phantomData[id]!.getTraits()[lookupTrait]
        if(nftTrait != nil  && nftTrait == lookupValue) {
            return true
        }
        return false
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    init() {
        self.CollectionStoragePath = /storage/sevenOneZeroPhantomCollection
        self.CollectionPublicPath = /public/sevenOneZeroPhantomCollection
        self.AdminStoragePath = /storage/sevenOneZeroAdmin
        
        self.totalSupply = 0

        self.name = "710 Phantom"

        self.externalURL = "https://710phantom.com/" // TODO: Change to point to NFT Site URL
        
        self.maxAmount = 7100

        self.phantomData = {}
        
        self.account.save(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
     
}

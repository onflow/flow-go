/*
    Description: The official Schmoes NFT Contract
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import SchmoesPreLaunchToken from 0x6c4fe48768523577

pub contract SchmoesNFT: NonFungibleToken {
    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // SchmoesNFT Events
    // -----------------------------------------------------------------------

    pub event Mint(id: UInt64)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------

    pub var totalSupply: UInt64
    pub var maxSupply: UInt64

    // -----------------------------------------------------------------------
    // SchmoesNFT Fields
    // -----------------------------------------------------------------------
    
    pub var name: String
    pub var isSaleActive: Bool
    pub var price: UFix64
    pub var maxMintAmount: UInt64
    pub var provenance: String

    // launch parameters
    pub var earlyLaunchTime: UFix64
    pub var launchTime: UFix64
    pub var idsPerIncrement: UInt64
    pub var timePerIncrement: UInt64

    access(self) let editionToSchmoeData: { UInt64: SchmoeData }
    access(self) let editionToProvenance: { UInt64: String }
    access(self) let editionToNftId: { UInt64: UInt64 }
    access(self) let nftIdToEdition: { UInt64: UInt64 }

    // maps assetType to a dict of assetNames to base64 encoded images of assets
    access(self) let schmoeAssets: { SchmoeTrait: { String: String } }

    // CID for the IPFS folder
    pub var ipfsBaseCID: String

    pub enum SchmoeTrait: UInt8 {
        pub case hair
        pub case background
        pub case face
        pub case eyes
        pub case mouth
        pub case clothes
        pub case props
        pub case ears
    }

    pub struct SchmoeData {
        pub let traits: { SchmoeTrait: String }

        init(traits: { SchmoeTrait: String }) {
            self.traits = traits
        }
    }

    pub resource NFT : NonFungibleToken.INFT {
        pub let id: UInt64
        pub let edition: UInt64

        init(edition: UInt64) {
            self.id = self.uuid
            self.edition = edition
            SchmoesNFT.editionToNftId[self.edition] = self.id
            SchmoesNFT.nftIdToEdition[self.id] = self.edition
            emit Mint(id: self.id)
        }
    }

    pub resource interface SchmoesNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun getEditionsInCollection(): [UInt64]
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, SchmoesNFTCollectionPublic {
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
            let token <- token as! @SchmoesNFT.NFT
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

        pub fun getEditionsInCollection(): [UInt64] {
            let editions: [UInt64] = []
            let ids: [UInt64] = self.ownedNFTs.keys

            var i = 0
            while i < ids.length {
                editions.append(SchmoesNFT.nftIdToEdition[ids[i]]!)
                i = i + 1
            }

            return editions
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Admin Functions
    // -----------------------------------------------------------------------
    access(account) fun setIsSaleActive(_ newIsSaleActive: Bool) {
        self.isSaleActive = newIsSaleActive
    }

    access(account) fun setPrice(_ newPrice: UFix64) {
        self.price = newPrice
    }

    access(account) fun setMaxMintAmount(_ newMaxMintAmount: UInt64) {
        self.maxMintAmount = newMaxMintAmount
    }

    access(account) fun setIpfsBaseCID(_ ipfsBaseCID: String) {
        self.ipfsBaseCID = ipfsBaseCID
    }

    access(account) fun setProvenance(_ provenance: String) {
    	self.provenance = provenance
    }

    access(account) fun setProvenanceForEdition(_ edition: UInt64, _ provenance: String) {
        self.editionToProvenance[edition] = provenance
    }

    access(account) fun setSchmoeAsset(_ assetType: SchmoeTrait, _ assetName: String, _ content: String) {
        if (self.schmoeAssets[assetType] == nil) {
            let assetNameToContent: { String : String } = {
                assetName: content
            }
            self.schmoeAssets[assetType] = assetNameToContent
        } else {
            let ref = self.schmoeAssets[assetType]!
            ref[assetName] = content
            self.schmoeAssets[assetType] = ref
        }
    }

    access(account) fun batchUpdateSchmoeData(_ schmoeDataMap: { UInt64: SchmoeData }) {
        for edition in schmoeDataMap.keys {
            self.editionToSchmoeData[edition] = schmoeDataMap[edition]
        }
    }

    access(account) fun setEarlyLaunchTime(_ earlyLaunchTime: UFix64) {
        self.earlyLaunchTime = earlyLaunchTime
    }

    access(account) fun setLaunchTime(_ launchTime: UFix64) {
        self.launchTime = launchTime
    }

    access(account) fun setIdsPerIncrement(_ idsPerIncrement: UInt64) {
        self.idsPerIncrement = idsPerIncrement
    }

    access(account) fun setTimePerIncrement(_ timePerIncrement: UInt64) {
        self.timePerIncrement = timePerIncrement
    }

    // -----------------------------------------------------------------------
    // Public Functions
    // -----------------------------------------------------------------------
    pub fun getSchmoeDataForEdition(_ edition: UInt64): SchmoeData {
        return self.editionToSchmoeData[edition]!
    }

    pub fun getAllSchmoeData(): { UInt64: SchmoeData } {
        return self.editionToSchmoeData
    }
    
    pub fun getProvenanceForEdition(_ edition: UInt64): String {
        return self.editionToProvenance[edition]!
    }

    pub fun getAllProvenances(): { UInt64: String } {
        return self.editionToProvenance
    }
    
    pub fun getNftIdForEdition(_ edition: UInt64): UInt64 {
        return self.editionToNftId[edition]!
    }

    pub fun getEditionToNftIdMap(): { UInt64: UInt64 } {
        return self.editionToNftId
    }
    
    pub fun getEditionForNftId(_ nftId: UInt64): UInt64 {
        return self.nftIdToEdition[nftId]!
    }

    pub fun getNftIdToEditionMap(): { UInt64: UInt64 } {
        return self.nftIdToEdition
    }

    pub fun getSchmoeAsset(_ assetType: SchmoeTrait, _ assetName: String): String {
        return self.schmoeAssets[assetType]![assetName]!
    }

    pub fun mintWithPreLaunchToken(buyVault: @FungibleToken.Vault, mintAmount: UInt64, preLaunchToken: @SchmoesPreLaunchToken.NFT) : @NonFungibleToken.Collection {
        let schmoes: @NonFungibleToken.Collection <- self.maybeMint(<-buyVault, mintAmount, preLaunchToken.id)
        destroy preLaunchToken
        return <-schmoes
    }

    pub fun mint(buyVault: @FungibleToken.Vault, mintAmount: UInt64) : @NonFungibleToken.Collection {
        return <-self.maybeMint(<-buyVault, mintAmount, nil)
    }

    pub fun getAvailableMintTime(_ preLaunchTokenId: UInt64?): UFix64 {
        if (preLaunchTokenId == nil) {
            return self.launchTime
        } else {
            let increments = preLaunchTokenId! / self.idsPerIncrement
            let timeAfterLaunch = increments * self.timePerIncrement
            return UFix64(timeAfterLaunch) + self.earlyLaunchTime
        }
    }

    // -----------------------------------------------------------------------
    // Helper Functions
    // -----------------------------------------------------------------------
    access(contract) fun maybeMint(_ buyVault: @FungibleToken.Vault, _ mintAmount: UInt64, _ preLaunchTokenId: UInt64?) : @NonFungibleToken.Collection {
        pre {
            self.isSaleActive : "Sale is not active"
            mintAmount <= self.maxMintAmount : "Attempting to mint too many Schmoes"
            self.totalSupply + mintAmount <= self.maxSupply : "Not enough supply to mint that many Schmoes"
            buyVault.balance >= self.price * UFix64(mintAmount) : "Insufficient funds"
        }

        // epoch time in seconds
        let currTime = getCurrentBlock().timestamp

        if (currTime > self.launchTime) {
            return <-self.batchMint(<-buyVault, mintAmount)
        }
        if (currTime > self.earlyLaunchTime) {
            let id = preLaunchTokenId ?? panic("A Pre-Launch Token is required to mint") 
            let availableMintTime = self.getAvailableMintTime(id)

            if (currTime >= availableMintTime) {
                return <-self.batchMint(<-buyVault, mintAmount)
            } else {
                panic("This Pre-Launch Token is not eligble to mint yet.")
            }
        }

        panic("Minting has not started yet.")
    }
    
    access(contract) fun batchMint(_ buyVault: @FungibleToken.Vault, _ mintAmount: UInt64) : @NonFungibleToken.Collection {
        pre {
            buyVault.isInstance(self.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver).borrow()!.getType()) : "Minting requires a FlowToken.Vault"
        }

        let adminVaultRef = self.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver).borrow()!
        adminVaultRef.deposit(from: <-buyVault)

        var schmoesCollection <- self.createEmptyCollection()
        var i : UInt64 = 0
        while i < mintAmount {
            let edition = SchmoesNFT.totalSupply + (1 as UInt64)
            let schmoe: @SchmoesNFT.NFT <- create SchmoesNFT.NFT(edition: edition)
            SchmoesNFT.totalSupply = edition
            schmoesCollection.deposit(token: <-schmoe)
            i = i + 1
        }
        
        return <-schmoesCollection
    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    init() {
        self.name = "SchmoesNFT"
        self.isSaleActive = false
        self.totalSupply = 0
        self.maxSupply = 10000
        self.price = 1.0
        self.maxMintAmount = 1
        self.provenance = ""
        self.editionToSchmoeData = {}
        self.editionToProvenance = {}
        self.editionToNftId = {}
        self.nftIdToEdition = {}
        self.schmoeAssets = {}
        self.ipfsBaseCID = ""
        self.CollectionStoragePath = /storage/SchmoesNFTCollection
        self.CollectionPublicPath = /public/SchmoesNFTCollection

        self.earlyLaunchTime = 4791048813.0
        self.launchTime = 4791048813.0

        self.idsPerIncrement = 0
        self.timePerIncrement = 0

        emit ContractInitialized()
    }
}

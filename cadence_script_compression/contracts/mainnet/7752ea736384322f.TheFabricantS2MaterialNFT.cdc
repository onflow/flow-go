/*
    Description: TheFabricantS2MaterialNFT Contract
   
    TheFabricantS2MaterialNFT NFTs are minted by admins, and can be combined with 
    TheFabricantS2GarmentlNFT NFTs to mint TheFabricantS2ItemNFT NFTs.
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract TheFabricantS2MaterialNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // TheFabricantS2MaterialNFT contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Material contract is created
    pub event ContractInitialized()

    // Emitted when a new MaterialData struct is created
    pub event MaterialDataCreated(materialDataID: UInt32, designerAddress: Address, metadata: {String: String})

    // Emitted when a Material is minted
    pub event MaterialMinted(materialID: UInt64, materialDataID: UInt32, serialNumber: UInt32)

    pub event MaterialDataIDRetired(materialDataID: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a Material is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Emitted when a Material is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Material is destroyed
    pub event MaterialDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // contract-level fields.      
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Contains standard storage and public paths of resources
    pub let CollectionStoragePath: StoragePath

    pub let CollectionPublicPath: PublicPath

    pub let AdminStoragePath: StoragePath

    // Variable size dictionary of Material structs
    access(self) var materialDatas: {UInt32: MaterialData}

    // Dictionary with MaterialDataID as key and number of NFTs with MaterialDataID are minted
    access(self) var numberMintedPerMaterial: {UInt32: UInt32}

    // Dictionary of materialDataID to  whether they are retired
    access(self) var isMaterialDataRetired: {UInt32: Bool}

    // Dictionary of the nft with id and its current owner address
    access(self) var nftIDToOwner: {UInt64: Address}

    // Keeps track of how many unique MaterialData's are created
    pub var nextMaterialDataID: UInt32

    pub var totalSupply: UInt64

    // Royalty struct that each MaterialData will contain
	pub struct Royalty{

		pub let wallet:Capability<&{FungibleToken.Receiver}> 
        pub let initialCut: UFix64
		pub let cut: UFix64

		/// @param wallet : The wallet to send royalty too
		init(wallet:Capability<&{FungibleToken.Receiver}>, initialCut: UFix64, cut: UFix64){
			self.wallet=wallet
            self.initialCut= initialCut
			self.cut=cut
		}
	}

    pub struct MaterialData {

        // The unique ID for the Material Data
        pub let materialDataID: UInt32

        // The flow address of the designer
        pub let designerAddress: Address

        // Other metadata
        access(self) let metadata: {String: String}

        // mapping of royalty name to royalty struct    
        access(self) let royalty: {String: Royalty}

        init(
            designerAddress: Address,
            metadata: {String: String},
            royalty: {String: Royalty}
        ){
            self.materialDataID = TheFabricantS2MaterialNFT.nextMaterialDataID
            self.designerAddress = designerAddress
            self.metadata = metadata
            self.royalty = royalty

            TheFabricantS2MaterialNFT.isMaterialDataRetired[self.materialDataID] = false

            // Increment the ID so that it isn't used again
            TheFabricantS2MaterialNFT.nextMaterialDataID = TheFabricantS2MaterialNFT.nextMaterialDataID + 1 as UInt32

            emit MaterialDataCreated(materialDataID: self.materialDataID, designerAddress: self.designerAddress, metadata: self.metadata)
        }

        pub fun getMetadata(): {String: String}{
            return self.metadata
        }

        pub fun getRoyalty(): {String: Royalty}{
            return self.royalty
        }
    }
    
    pub struct Material {

        // The ID of the MaterialData that the Material references
        pub let materialDataID: UInt32

        // The N'th NFT with 'MaterialDataID' minted
        pub let serialNumber: UInt32

        init(materialDataID: UInt32) {
            self.materialDataID = materialDataID

            // Increment the ID so that it isn't used again
            TheFabricantS2MaterialNFT.numberMintedPerMaterial[materialDataID] = TheFabricantS2MaterialNFT.numberMintedPerMaterial[materialDataID]! + 1 as UInt32

            self.serialNumber = TheFabricantS2MaterialNFT.numberMintedPerMaterial[materialDataID]!
        }
    }    

    // The resource that represents the Material NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Material ID
        pub let id: UInt64

        // struct of Material
        pub let material: Material

        init(serialNumber: UInt32, materialDataID: UInt32) {
            TheFabricantS2MaterialNFT.totalSupply = TheFabricantS2MaterialNFT.totalSupply + 1 as UInt64
            
            self.id = TheFabricantS2MaterialNFT.totalSupply

            self.material = Material(materialDataID: materialDataID)     

            // Emitted when a Material is minted
            emit MaterialMinted(materialID: self.id, materialDataID: materialDataID, serialNumber: serialNumber)
        }

        destroy() {
            emit MaterialDestroyed(id: self.id)
        }

    }

    // Admin is a special authorization resource that
    // allows the owner to perform important functions to modify the 
    // various aspects of the Material and NFTs
    //
    pub resource Admin {

        pub fun createMaterialData(
            designerAddress: Address, 
            metadata: {String: String},
            royalty: {String: Royalty}
        ): UInt32 {
            // Create the new MaterialData
            var newMaterial = MaterialData(
                designerAddress: designerAddress,
                metadata: metadata,
                royalty: royalty
            )

            let newID = newMaterial.materialDataID

            // Store it in the contract storage
            TheFabricantS2MaterialNFT.materialDatas[newID] = newMaterial

            TheFabricantS2MaterialNFT.numberMintedPerMaterial[newID] = 0 as UInt32

            return newID
        }
        
        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // Mint the new Material
        pub fun mintNFT(materialDataID: UInt32): @NFT {
            let numInMaterial = TheFabricantS2MaterialNFT.numberMintedPerMaterial[materialDataID]??
                panic("no materialDataID found")

            if (TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID]! == nil) {
                panic("Cannot mint Material. materialData not found")
            }

            if (TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID]!) {
                panic("Cannot mint material. materialDataID retired")
            }

            let newMaterial: @NFT <- create NFT(serialNumber: numInMaterial + 1, materialDataID: materialDataID)

            return <-newMaterial
        }

        pub fun batchMintNFT(materialDataID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(materialDataID: materialDataID))
                i = i + 1 as UInt64
            }

            return <-newCollection
        }

        // Retire materialData so that it cannot be used to mint anymore
        pub fun retireMaterialData(materialDataID: UInt32) {           
            pre {
                TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID] != nil: "Cannot retire Material: Material doesn't exist!"
            }

            if !TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID]! {
                TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID] = true

                emit MaterialDataIDRetired(materialDataID: materialDataID)
            }
        }
    }

    // This is the interface users can cast their Material Collection as
    // to allow others to deposit into their Collection. It also allows for reading
    // the IDs of Material in the Collection.
    pub resource interface MaterialCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMaterial(id: UInt64): &TheFabricantS2MaterialNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Material reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: MaterialCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Material conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Material from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Material does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Material
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a Material and adds it to the Collections dictionary
        //
        // Parameters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            // Cast the deposited token as NFT to make sure
            // it is the correct type
            let token <- token as! @TheFabricantS2MaterialNFT.NFT

            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Set the global mapping of nft id to new owner
            TheFabricantS2MaterialNFT.nftIDToOwner[id] = self.owner?.address
            
            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token tMaterial was "removed"
            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            // Get an array of the IDs to be deposited
            let keys = tokens.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // getIDs returns an array of the IDs that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to a Material in the Collection
        // so tMaterial the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not an specific data. Please use borrowMaterial to 
        // read Material data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowMaterial(id: UInt64): &TheFabricantS2MaterialNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TheFabricantS2MaterialNFT.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Material contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Material in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TheFabricantS2MaterialNFT.Collection()
    }

    // get dictionary of numberMintedPerMaterial
    pub fun getNumberMintedPerMaterial(): {UInt32: UInt32} {
        return TheFabricantS2MaterialNFT.numberMintedPerMaterial
    }

    // get how many Materials with materialDataID are minted 
    pub fun getMaterialNumberMinted(id: UInt32): UInt32 {
        let numberMinted = TheFabricantS2MaterialNFT.numberMintedPerMaterial[id]??
            panic("materialDataID not found")
        return numberMinted
    }

    // get the materialData of a specific id
    pub fun getMaterialData(id: UInt32): MaterialData {
        let materialData = TheFabricantS2MaterialNFT.materialDatas[id]??
            panic("materialDataID not found")
        return materialData
    }

    // get all materialDatas created
    pub fun getMaterialDatas(): {UInt32: MaterialData} {
        return TheFabricantS2MaterialNFT.materialDatas
    }

    pub fun getMaterialDatasRetired(): {UInt32: Bool} { 
        return TheFabricantS2MaterialNFT.isMaterialDataRetired
    }

    pub fun getMaterialDataRetired(materialDataID: UInt32): Bool { 
        let isMaterialDataRetired = TheFabricantS2MaterialNFT.isMaterialDataRetired[materialDataID]??
            panic("materialDataID not found")
        return isMaterialDataRetired
    }

    pub fun getNftIdToOwner(): {UInt64: Address} { 
        return TheFabricantS2MaterialNFT.nftIDToOwner
    }


    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.materialDatas = {}
        self.nftIDToOwner = {}
        self.numberMintedPerMaterial = {}
        self.nextMaterialDataID = 1
        self.isMaterialDataRetired = {}
        self.totalSupply = 0
        self.CollectionPublicPath = /public/S2MaterialCollection0022
        self.CollectionStoragePath = /storage/S2MaterialCollection0022
        self.AdminStoragePath = /storage/S2MaterialAdmin0022

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{MaterialCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
 
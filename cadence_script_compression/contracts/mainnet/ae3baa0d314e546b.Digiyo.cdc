//SPDX-License-Identifier: MIT
import NonFungibleToken from 0x1d7e57aa55817448

pub contract Digiyo: NonFungibleToken {
    // events
    pub event ContractInitialized()
    pub event AssetCreated(id: UInt32, metadata: {String:String})
    pub event NewSeriesStarted(newCurrentSeries: UInt32)
    pub event SetCreated(setID: UInt32, series: UInt32)
    pub event AssetAddedToSet(setID: UInt32, assetID: UInt32)
    pub event AssetRetiredFromSet(setID: UInt32, assetID: UInt32, numInstances: UInt32)
    pub event SetLocked(setID: UInt32)
    pub event InstanceMinted(instanceID: UInt64, assetID: UInt32, setID: UInt32, serialNumber: UInt32)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event InstanceDestroyed(id: UInt64)

    pub let collectionPublicPath: PublicPath
    pub let collectionStoragePath: StoragePath
    pub let digiyoAdminPath: StoragePath

    // Series to which a given set belongs
    pub var currentSeries: UInt32

    // dictionary of Asset structs
    access(self) var assetDatas: {UInt32: Asset}

    // dictionary of SetData structs
    access(self) var setDatas: {UInt32: SetData}

    // dictionary of Set resources
    access(self) var sets: @{UInt32: Set}

    // the ID used to create Assets. Every time an Asset is created 
    // assetID is assigned to the new Asset's ID then incremented by 1.
    pub var nextAssetID: UInt32

    // the ID that is used to create Sets. Every time a Set is created
    // setID is assigned to the new set's ID then incremented by 1.
    pub var nextSetID: UInt32

    // the total number of Digiyo instances created.
    // Also used as global instance IDs for minting.
    pub var totalSupply: UInt64

    // Asset is a Struct holding the metadata for a given asset.
    // Instances reference an Asset for metadata provision.
    pub struct Asset {
        pub let assetID: UInt32
        pub let metadata: {String: String}
        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Asset Metadata cannot be empty"
            }
            self.assetID = Digiyo.nextAssetID
            self.metadata = metadata
            Digiyo.nextAssetID = Digiyo.nextAssetID + UInt32(1)
            emit AssetCreated(id: self.assetID, metadata: metadata)
        }
    }

    // A Set is a grouping of assets
    // SetData is a struct stored in a public field of the contract.
    pub struct SetData {
        pub let setID: UInt32 // unique ID for the set
        pub let name: String // Name of the Set
        pub let series: UInt32 // Series to which a set belongs
        init(name: String) {
            pre {
                name.length > 0: "Set must have a name"
            }
            self.setID = Digiyo.nextSetID
            self.name = name
            self.series = Digiyo.currentSeries
            Digiyo.nextSetID = Digiyo.nextSetID + UInt32(1) // increment setID
            emit SetCreated(setID: self.setID, series: self.series)
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // assets from a set and mint instances.
    // If the admin locks the Set, assets can no longer be added, but instances can be minted
    // Set is closed permanently if retireAll() & lock() are called back-to-back
    pub resource Set {
        pub let setID: UInt32 // unique ID for the set
        pub var assets: [UInt32] // Array of assets in set
        pub var retired: {UInt32: Bool} // Indicates mintability
        pub var locked: Bool // Indicates if the set is locked
        // Indicates the number of instances that have been minted per asset in this set
        // When an instance is minted, this value is stored therein
        pub var numberMintedPerAsset: {UInt32: UInt32}
        init(name: String) {
            self.setID = Digiyo.nextSetID
            self.assets = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerAsset = {}
            // Create SetData struct for this Set and store it
            Digiyo.setDatas[self.setID] = SetData(name: name)
        }

        // addAsset adds an asset to the set
        // Parameters: assetID: The ID of the asset that is being added
        // Pre-Conditions:
        // The asset needs to exist
        // The set cannot have been locked
        // The set must not yet contain the asset
        pub fun addAsset(assetID: UInt32) {
            pre {
                Digiyo.assetDatas[assetID] != nil: "Cannot add the Asset to Set: Asset doesn't exist"
                !self.locked: "Cannot add the asset to the Set after the set has been locked"
                self.numberMintedPerAsset[assetID] == nil: "The asset has already beed added to the set"
            }
            self.assets.append(assetID) // Add the asset to the array of assets
            self.retired[assetID] = false // Open the asset up for minting
            self.numberMintedPerAsset[assetID] = 0 // Initialize the instance count to zero
            emit AssetAddedToSet(setID: self.setID, assetID: assetID)
        }

        // addAssets adds multiple assets to the set
        // Parameters: assetIDs: The IDs of the assets
        pub fun addAssets(assetIDs: [UInt32]) {
            for asset in assetIDs {
                self.addAsset(assetID: asset)
            }
        }

        // retireAsset retires a asset from the set so that it can't mint new instances
        // Parameters: assetID: The ID of the asset that is being retired
        // Pre-Conditions:
        // The asset needs to be an existing, unretired asset
        pub fun retireAsset(assetID: UInt32) {
            pre {
                self.retired[assetID] != nil: "Cannot retire the Asset: Asset doesn't exist in this set!"
            }
            if !self.retired[assetID]! {
                self.retired[assetID] = true
                emit AssetRetiredFromSet(setID: self.setID, assetID: assetID, numInstances: self.numberMintedPerAsset[assetID]!)
            }
        }

        // retireAll retires all the assets in the set
        // Afterwards, none of the retired assets will be mintable
        pub fun retireAll() {
            for asset in self.assets {
                self.retireAsset(assetID: asset)
            }
        }

        // lock() locks the set so that no more assets can be added to it
        // Pre-Conditions:
        // The set cannot yet have been locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        // mintInstance mints a new instance and returns it
        // Parameters: assetID: The ID of the asset which the instance references
        // Pre-Conditions:
        // The asset must exist in the set and be mintable
        // Returns: The instance that was minted
        pub fun mintInstance(assetID: UInt32): @NFT {
            pre {
                self.retired[assetID] != nil: "Cannot mint the instance: This asset doesn't exist"
                !self.retired[assetID]!: "Cannot mint the instance from this asset: This asset has been retired"
            }
            let numInAsset = self.numberMintedPerAsset[assetID]! // get number of instances minted for this asset
            let newInstance: @NFT <- create NFT(serialNumber: numInAsset + UInt32(1), assetID: assetID, setID: self.setID)
            self.numberMintedPerAsset[assetID] = numInAsset + UInt32(1) // Increment the count of instances minted for this asset
            return <-newInstance
        }

        // batchMintInstance mints an arbitrary quantity of instances 
        // and returns them as a Collection
        // Parameters: assetID: the ID of the asset from which the instances are minted
        //             quantity: The quantity of instances to be minted
        // Returns: Collection object that contains all the instances that were minted
        pub fun batchMintInstance(assetID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()
            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintInstance(assetID: assetID))
                i = i + UInt64(1)
            }
            return <-newCollection
        }
    }

    pub struct InstanceData {
        pub let setID: UInt32 // the ID of the Set
        pub let assetID: UInt32 // the ID of the Asset
        pub let serialNumber: UInt32 // the order in which this instance was minted
        init(setID: UInt32, assetID: UInt32, serialNumber: UInt32) {
            self.setID = setID
            self.assetID = assetID
            self.serialNumber = serialNumber
        }
    }
    
    pub resource NFT: NonFungibleToken.INFT { // Resource representing the Instance NFTs
        pub let id: UInt64 // global unique instance ID
        pub let data: InstanceData // struct of instance metadata
        init(serialNumber: UInt32, assetID: UInt32, setID: UInt32) {
            Digiyo.totalSupply = Digiyo.totalSupply + UInt64(1) // Increment the global instance IDs
            self.id = Digiyo.totalSupply
            self.data = InstanceData(setID: setID, assetID: assetID, serialNumber: serialNumber) // set the metadata struct
            emit InstanceMinted(instanceID: self.id, assetID: assetID, setID: self.data.setID, serialNumber: self.data.serialNumber)
        }
        destroy() {
            emit InstanceDestroyed(id: self.id)
        }
    }

    // Admin is an authorization resource permitting execution of certain functions
    pub resource Admin {

        // createAsset creates a new Asset struct and stores it
        // Parameters: metadata: A dictionary mapping metadata keys to their values
        // Returns: the ID of the new Asset object
        pub fun createAsset(metadata: {String: String}): UInt32 {
            
            var newAsset = Asset(metadata: metadata) // Create the new Asset
            let newID = newAsset.assetID
            Digiyo.assetDatas[newID] = newAsset // Store it in the contract storage
            return newID
        }

        // createSet creates a new Set resource and returns it
        // so that the caller can store it in their account
        // Parameters: name: The name of the set
        //             series: The series to which the set
        pub fun createSet(name: String) {
            var newSet <- create Set(name: name) // Create the new Set
            Digiyo.sets[newSet.setID] <-! newSet
        }

        // borrowSet returns a reference to a set in the Digiyo
        // contract so that the admin can call methods on it
        // Parameters: setID: The ID of the set referenced
        // Returns: A reference to the set with fields and methods exposed
        pub fun borrowSet(setID: UInt32): &Set {
            pre {
                Digiyo.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
            }
            return &Digiyo.sets[setID] as &Set
        }

        // startNewSeries ends the current series by incrementing currentSeries
        // Returns: The new series number
        pub fun startNewSeries(): UInt32 {
            Digiyo.currentSeries = Digiyo.currentSeries + UInt32(1)
            emit NewSeriesStarted(newCurrentSeries: Digiyo.currentSeries)
            return Digiyo.currentSeries
        }

        // createNewAdmin creates a new Admin Resource
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }
    
    // This interface allows the admin or other users to deposit into a users collection
    pub resource interface DigiyoNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowInstance(id: UInt64): &Digiyo.NFT? {
            post { // If not nil should match argument
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Instance reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    pub resource Collection: DigiyoNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        // Dictionary of Instance conforming tokens
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Instance from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Instance does not exist in the collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        // deposit takes an Instance and adds it to the collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Digiyo.NFT
            let id = token.id
            let oldToken <- self.ownedNFTs[id] <- token // add the new token to the dictionary
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }
            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys { // iterate through the keys in the collection and deposit each one
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        // getIDs returns an array of the IDs in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to an instance in the collection
        // so that the caller can read its ID
        // Parameters: id: The ID of the instance being referenced
        // Returns: A reference to the instance
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowInstance Returns a borrowed reference to an instance in the collection
        // essentially a more robust version of the borrowNFT function (allows method calls)
        // Parameters: id: The ID of the instance to get the reference for
        // Returns: A reference to the instance
        pub fun borrowInstance(id: UInt64): &Digiyo.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Digiyo.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained therin are also destroyed
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // createEmptyCollection creates a new, empty Collection object
    // Once a user has a Collection in storage, they can receive instances
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create Digiyo.Collection()
    }

    // getAllAssets returns all the assets in digiyo
    // Returns: An array of all the assets that have been created
    pub fun getAllAssets(): [Digiyo.Asset] {
        return Digiyo.assetDatas.values
    }

    // getAssetMetaData returns the metadata associated with a given asset
    // Parameters: assetID: The id of the asset
    // Returns: The metadata as a String
    pub fun getAssetMetaData(assetID: UInt32): {String: String}? {
        return self.assetDatas[assetID]?.metadata
    }

    // getAssetMetaDataByField returns the value in a given field of the metadata
    // Parameters: assetID: The id of the asset
    //             field: The field (i.e. metadata key) sought
    // Returns: The metadata field as a String Optional
    pub fun getAssetMetaDataByField(assetID: UInt32, field: String): String? {
        if let asset = Digiyo.assetDatas[assetID] {
            return asset.metadata[field]
        } else {
            return nil
        }
    }

    // getSetName returns the name of the set whose ID is passed.
    // Parameters: setID: The id of the set
    // Returns: The name of the set
    pub fun getSetName(setID: UInt32): String? {
        return Digiyo.setDatas[setID]?.name
    }

    // getSetSeries returns the of the set whose ID is passed
    // Parameters: setID: The id of the set
    // Returns: The series that the set belongs to
    pub fun getSetSeries(setID: UInt32): UInt32? {
        return Digiyo.setDatas[setID]?.series
    }

    // getSetIDsByName returns the IDs of the set whose name is passed.
    // Parameters: setName: The name of the set
    // Returns: An array of the IDs of the set if it exists, or nil if doesn't
    pub fun getSetIDsByName(setName: String): [UInt32]? {
        var setIDs: [UInt32] = []
        for setData in Digiyo.setDatas.values { // search setDatas for name
            if setName == setData.name {
                setIDs.append(setData.setID) // if the name is found, return the ID
            }
        }
        if setIDs.length == 0 { // If name not found, return nil
            return nil
        } else {
            return setIDs
        }
    }

    // getAssetsInSet returns a list of asset IDs in the set whose ID is passed
    // Parameters: setID: The id of the set
    // Returns: An array of asset IDs
    pub fun getAssetsInSet(setID: UInt32): [UInt32]? {
        return Digiyo.sets[setID]?.assets
    }

    // isEditionRetired returns a boolean indicating whether a set/asset combo
    //                  (i.e. an edition) is retired. If retired, it still remains
    //                  in the set, but instances can no longer be minted from it.
    // Parameters: setID: The id of the set
    //             assetID: The id of the asset
    // Returns: Boolean indicating whether the edition is retired
    pub fun isEditionRetired(setID: UInt32, assetID: UInt32): Bool? {
        // remove the set from the dictionary to get its field
        if let setToRead <- Digiyo.sets.remove(key: setID) {
            let retired = setToRead.retired[assetID]
            Digiyo.sets[setID] <-! setToRead
            return retired
        } else {
            return nil
        }
    }

    // isSetLocked returns a boolean indicating whether a set is locked.
    //             If locked, new assets can no longer be added to the set,
    //             but instances can still be minted from assets therein.
    // Parameters: setID: The id of the set
    // Returns: Boolean indicating if the set is locked or not
    pub fun isSetLocked(setID: UInt32): Bool? {
        return Digiyo.sets[setID]?.locked
    }

    // getNumInstancesInEdition return the number of instances minted from an edition.
    // Parameters: setID: The id of the set
    //             assetID: The id of the asset
    // Returns: The total number of instances minted from an edition
    pub fun getNumInstancesInEdition(setID: UInt32, assetID: UInt32): UInt32? {
        if let setToRead <- Digiyo.sets.remove(key: setID) {
            let amount = setToRead.numberMintedPerAsset[assetID]
            Digiyo.sets[setID] <-! setToRead
            return amount
        } else {
            return nil
        }
    }
    
    init() {
        self.collectionPublicPath = /public/DigiyoNFTCollection
        self.collectionStoragePath = /storage/DigiyoNFTCollection
        self.digiyoAdminPath = /storage/DigiyoAdmin

        self.currentSeries = 0
        self.assetDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextAssetID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        self.account.save<@Collection>(<- create Collection(), to: self.collectionStoragePath)
        self.account.link<&{DigiyoNFTCollectionPublic}>(self.collectionPublicPath, target: self.collectionStoragePath)
        self.account.save<@Admin>(<- create Admin(), to: self.digiyoAdminPath)
        
        emit ContractInitialized()
    }
}
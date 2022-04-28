import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import S1GarmentNFT from 0x438ee7adcae39ab9
import S1MaterialNFT from 0x438ee7adcae39ab9

pub contract S1ItemNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // S1ItemNFT contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Item contract is created
    pub event ContractInitialized()

    // Emitted when a new ItemData struct is created
    pub event ItemDataCreated(itemDataID: UInt32, coCreator: Address, metadatas: {String: S1ItemNFT.Metadata})

    // Emitted when a mutable metadata in an itemData is changed
    pub event ItemMetadataChanged(itemDataID: UInt32, metadataKey: String, metadataValue: String)

    // Emitted when a mutable metadata in an itemData is set to immutable
    pub event ItemMetadataImmutable(itemDataID: UInt32, metadataKey: String)
    
    // Emitted when an Item is minted
    pub event ItemMinted(itemID: UInt64, itemDataID: UInt32, serialNumber: UInt32)

    // Emitted when an Item's name is changed
    pub event ItemNameChanged(id: UInt64, name: String)

    // Emitted when an ItemData is allocated 
    pub event ItemDataAllocated(garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String, key: String, itemDataID: UInt32)

    // Emitted when the items are set to be splittable
    pub event ItemNFTNowSplittable()

    // Emitted when the number of items with ItemDataID that can be minted changes
    pub event numberItemDataMintableChanged(itemDataID: UInt32, number: UInt32)

    // Emitted when an ItemData is retired
    pub event ItemDataIDRetired(itemDataID: UInt32)

    // Emitted when a color is added to availablePrimaryColors array
    pub event PrimaryColorAdded(color: String)

    // Emitted when a color is added to availableSecondaryColors array
    pub event SecondaryColorAdded(color: String)

    // Emitted when a color is removed from availablePrimaryColors array
    pub event PrimaryColorRemoved(color: String)

    // Emitted when a color is removed from availableSecondaryColors array
    pub event SecondaryColorRemoved(color: String)

    // Events for Collection-related actions
    //
    // Emitted when a Item is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Emitted when a Item is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Item is destroyed
    pub event ItemDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // contract-level fields.      
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    pub let CollectionStoragePath: StoragePath

    pub let CollectionPublicPath: PublicPath

    pub let AdminStoragePath: StoragePath

    // itemDataID: number of NFTs with that ItemDataID are minted
    access(self) var numberMintedPerItem: {UInt32: UInt32}

    // itemDataID: how many items with itemData can be minted
    access(self) var numberMintablePerItemData: {UInt32: UInt32}

    // itemDataID: itemData struct
    access(self) var itemDatas: {UInt32: ItemData}

    // concat of garmentDataID_materialDataID_primaryColor_secondaryColor: itemDataID (eg. 1_1_FFFFFF_000000)
    access(self) var itemDataAllocation: {String: UInt32}

    // itemDataID: whether item with ItemDataID cannot be minted anymore
    access(self) var isItemDataRetired: {UInt32: Bool}

    // array of available primaryColors
    access(self) var availablePrimaryColors: [String]

    // array of available secondaryColors
    access(self) var availableSecondaryColors: [String]

    // Keeps track of how many unique ItemDatas are created
    pub var nextItemDataID: UInt32

    // Keeps track of how many unique ItemDataAllocations are created
    pub var nextItemDataAllocation: UInt32

    // Are garment and material removable from item
    pub var isSplittable: Bool

    pub var totalSupply: UInt64

    // metadata of an ItemData struct that contains the metadata value
    // and whether it is mutable
    pub struct Metadata {
        pub var metadataValue: String
        pub var mutable: Bool
        
        init(
            metadataValue: String,
            mutable: Bool
        ){
            self.metadataValue = metadataValue
            self.mutable = mutable
        }
        
        pub fun setMetadataImmutable(){
            pre {
                self.mutable == true:
                "metadata is already immutable"
            }
            self.mutable = false
        }

        pub fun setMetadataValue(metadataValue: String){
            pre {
                self.mutable == true:
                "metadata is already immutable"
            }
            self.metadataValue = metadataValue
        }


    }
    pub struct ItemData {

        // The unique ID for the Item Data
        pub let itemDataID: UInt32
        // stores link to image

        // the address of the user who created the unique combination
        pub let coCreator: Address

        // other metadata
        access(self) let metadatas: {String: S1ItemNFT.Metadata}

        init(
            coCreator: Address,
            metadatas: {String: S1ItemNFT.Metadata}
        ){
            self.itemDataID = S1ItemNFT.nextItemDataID
            self.coCreator = coCreator
            self.metadatas = metadatas

            S1ItemNFT.isItemDataRetired[self.itemDataID] = false

            // set default number mintable of itemData to 1
            S1ItemNFT.numberMintablePerItemData[self.itemDataID] = 1

            // Increment the ID so that it isn't used again
            S1ItemNFT.nextItemDataID = S1ItemNFT.nextItemDataID + 1 as UInt32

            emit ItemDataCreated(itemDataID: self.itemDataID, coCreator: self.coCreator, metadatas: self.metadatas)
        }

        // change a mutable metadata in the metadata struct
        pub fun setMetadata(metadataKey: String, metadataValue: String){
            self.metadatas[metadataKey]!.setMetadataValue(metadataValue: metadataValue)

            emit ItemMetadataChanged(itemDataID: self.itemDataID, metadataKey: metadataKey, metadataValue: metadataValue)
        }

        // prevent changing of a metadata in the metadata struct
        pub fun setMetadataKeyImmutable(metadataKey: String){
            self.metadatas[metadataKey]!.setMetadataImmutable()

            emit ItemMetadataImmutable(itemDataID: self.itemDataID, metadataKey: metadataKey)
        }

        pub fun getMetadata(): {String: S1ItemNFT.Metadata}{
            return self.metadatas
        }
    }

    pub struct Item {

        // The ID of the itemData that the item references
        pub let itemDataID: UInt32

        // The N'th NFT with 'ItemDataID' minted
        pub let serialNumber: UInt32

        init(itemDataID: UInt32) {
            pre {
                Int(S1ItemNFT.numberMintedPerItem[itemDataID]!) < Int(S1ItemNFT.numberMintablePerItemData[itemDataID]!): 
                "maximum number of S1ItemNFT with itemDataID reached"
            }
            
            self.itemDataID = itemDataID

            // Increment the ID so that it isn't used again
            S1ItemNFT.numberMintedPerItem[itemDataID] = S1ItemNFT.numberMintedPerItem[itemDataID]! + 1 as UInt32

            self.serialNumber = S1ItemNFT.numberMintedPerItem[itemDataID]!

        }
    }      

    // Royalty struct that each S1ItemNFT will contain
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

    // The resource that represents the Item NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Item ID
        pub let id: UInt64
        
        // struct of Item
        pub let item: Item 

        // name of nft, can be changed
        pub var name: String
        
        // Royalty struct
        pub let royaltyVault: S1ItemNFT.Royalty

        // after you remove the garment and material from the item, the S1ItemNFT will be considered "dead". 
        // accounts will be unable to deposit, withdraw or call functions of the nft.
        pub var isDead : Bool

        // this is where the garment nft is stored, it cannot be moved out
        access(self) var garment: @S1GarmentNFT.NFT?

        // this is where the material nft is stored, it cannot be moved out
        access(self) var material: @S1MaterialNFT.NFT?

        init(serialNumber: UInt32, name: String, itemDataID: UInt32, royaltyVault: S1ItemNFT.Royalty, garment: @S1GarmentNFT.NFT, material: @S1MaterialNFT.NFT) {
            
            S1ItemNFT.totalSupply = S1ItemNFT.totalSupply + 1 as UInt64
            
            self.id = S1ItemNFT.totalSupply

            self.name = name

            self.royaltyVault = royaltyVault

            self.isDead = false

            self.garment <- garment

            self.material <- material

            self.item = Item(itemDataID: itemDataID)

            // Emitted when a Item is minted
            emit ItemMinted(itemID: self.id, itemDataID: itemDataID, serialNumber: serialNumber)

        }

        destroy() {
            emit ItemDestroyed(id: self.id)
            //destroy self.items
            destroy self.garment
            destroy self.material
        }

        //Make Item considered dead. Deposit garment and material to respective vaults
        pub fun split(garmentCap: Capability<&{S1GarmentNFT.GarmentCollectionPublic}>, materialCap: Capability<&{S1MaterialNFT.MaterialCollectionPublic}>) {
            pre {
                !self.isDead: 
                "Cannot split. Item is dead"
                S1ItemNFT.isSplittable:
                "Item is set to unsplittable"
                garmentCap.check():
                "Garment Capability is invalid"
                materialCap.check():
                "Material Capability is invalid"
            }
            let garmentOptional <- self.garment <- nil
            let materialOptional <- self.material <- nil
            let garmentRecipient = garmentCap.borrow()!
            let materialRecipient = materialCap.borrow()!
            let garment <- garmentOptional!
            let material <- materialOptional!
            let garmentNFT <- garment as! @NonFungibleToken.NFT
            let materialNFT <- material as! @NonFungibleToken.NFT
            garmentRecipient.deposit(token: <- garmentNFT)
            materialRecipient.deposit(token: <- materialNFT)
            S1ItemNFT.numberMintedPerItem[self.item.itemDataID] = S1ItemNFT.numberMintedPerItem[self.item.itemDataID]! - 1 as UInt32
            self.isDead = true
        }

        // get a reference to the garment that item stores
        pub fun borrowGarment(): &S1GarmentNFT.NFT? {
            let garmentOptional <- self.garment <- nil
            let garment <- garmentOptional!
            let garmentRef = &garment as auth &S1GarmentNFT.NFT
            self.garment <-! garment
            return garmentRef   
        }

        // get a reference to the material that item stores
        pub fun borrowMaterial(): &S1MaterialNFT.NFT?  {
            let materialOptional <- self.material <- nil
            let material <- materialOptional!
            let materialRef = &material as auth &S1MaterialNFT.NFT
            self.material <-! material
            return materialRef
        }

        // change name of item nft
        pub fun changeName(name: String) {
            pre {
                !self.isDead: 
                "Cannot change garment name. Item is dead"
            }
            self.name = name;

           emit ItemNameChanged(id: self.id, name: self.name)
        }
    }

    //destroy item if it is considered dead
    pub fun cleanDeadItems(item: @S1ItemNFT.NFT) {
        pre {
            item.isDead: 
            "Cannot destroy, item not dead" 
        }
        destroy item
    }

    // mint the NFT, based on a combination of garment, material, primaryColor and secondaryColor. 
    // The itemData that is used to mint the Item is based on the garment and material' garmentDataID and materialDataID
    pub fun mintNFT(name: String, royaltyVault: S1ItemNFT.Royalty, garment: @S1GarmentNFT.NFT, material: @S1MaterialNFT.NFT, primaryColor: String, secondaryColor: String): @NFT {
        let garmentDataID = garment.garment.garmentDataID

        let materialDataID = material.material.materialDataID

        let key = S1ItemNFT.getUniqueString(garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor)

        //check whether unique combination is allocated
        let itemDataID = S1ItemNFT.itemDataAllocation[key]??
            panic("Cannot mint Item. ItemData not found")

        //check whether itemdata is retired
        if (S1ItemNFT.isItemDataRetired[itemDataID]!) {
            panic("Cannot mint Item. ItemDataID retired")
        }

        let numInItem = S1ItemNFT.numberMintedPerItem[itemDataID]??
            panic("Maximum number of Items with itemDataID minted")

        let item <-create NFT(serialNumber: numInItem + 1, name: name, itemDataID: itemDataID, royaltyVault: royaltyVault, garment: <- garment, material: <- material)

        return <- item
    }

    // This is the interface that users can cast their Item Collection as
    // to allow others to deposit Items into their Collection. It also allows for reading
    // the IDs of Items in the Collection.
    pub resource interface ItemCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItem(id: UInt64): &S1ItemNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Item reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: ItemCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Item conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Item from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Item does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Items
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

        // deposit takes a Item and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            //todo: someFunction that transfers royalty
            // Cast the deposited token as  NFT to make sure
            // it is the correct type
            let token <- token as! @S1ItemNFT.NFT
            
            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token that was "removed"
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

        // borrowNFT Returns a borrowed reference to a Item in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not an specific data. Please use borrowItem to 
        // read Item data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowItem(id: UInt64): &S1ItemNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &S1ItemNFT.NFT
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

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Items and NFTs
    //
    pub resource Admin {

        // create itemdataid allocation based on the 
        // garmentDataID, materialDataID, primaryColor and secondaryColor
        // and create an itemdata struct based on it
        pub fun createItemDataWithAllocation(
            garmentDataID: UInt32, 
            materialDataID: UInt32, 
            primaryColor: String, 
            secondaryColor: String,
            metadatas: {String: S1ItemNFT.Metadata},
            coCreator: Address):UInt32 {

            // check whether colors are valid colors
            pre {
                S1ItemNFT.availablePrimaryColors.contains(primaryColor):
                    "PrimaryColor not available"

                S1ItemNFT.availableSecondaryColors.contains(secondaryColor):
                    "SecondaryColor not available"

                S1GarmentNFT.getGarmentDatas().containsKey(garmentDataID):
                    "GarmentData not found"

                S1MaterialNFT.getMaterialDatas().containsKey(materialDataID):
                    "MaterialData not found"
            }
            // set the primaryColor and secondaryColor metadata of Metadata struct
            metadatas["primaryColor"] = S1ItemNFT.Metadata(metadataValue: primaryColor, mutable: false)
            metadatas["secondaryColor"] = S1ItemNFT.Metadata(metadataValue: secondaryColor, mutable: false)

            // check whether unique combination is allocated already
            let key = S1ItemNFT.getUniqueString(garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor)
            if(S1ItemNFT.itemDataAllocation.containsKey(key)){
                panic("Item already allocated")
            } 

            // set unique combination's itemdataallocation, then increment nextItemDataAllocation
            S1ItemNFT.itemDataAllocation[key] = S1ItemNFT.nextItemDataAllocation
            emit ItemDataAllocated(garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor, key: key, itemDataID: S1ItemNFT.nextItemDataAllocation)
            S1ItemNFT.nextItemDataAllocation = S1ItemNFT.nextItemDataAllocation + 1 as UInt32

            // create new itemData 
            var newItem = ItemData(coCreator: coCreator, metadatas: metadatas)

            // Store it in the contract storage
            let newID = newItem.itemDataID
            S1ItemNFT.itemDatas[newID] = newItem
            S1ItemNFT.numberMintedPerItem[newID] = 0 as UInt32
            return newID
        }

        // change the metadatavalue of an itemData
        pub fun setMetadata(itemDataID: UInt32, metadataKey: String, metadataValue: String) {
            let itemDataTemp = S1ItemNFT.itemDatas[itemDataID]!
            itemDataTemp.setMetadata(metadataKey: metadataKey, metadataValue: metadataValue)
            S1ItemNFT.itemDatas[itemDataID] = itemDataTemp
        }

        // make the metadatavalue of an itemData immutable
        pub fun setMetadataImmutable(itemData: UInt32, metadataKey: String) {
            let itemDataTemp = S1ItemNFT.itemDatas[itemData]!
            itemDataTemp.setMetadataKeyImmutable(metadataKey: metadataKey)
            S1ItemNFT.itemDatas[itemData] = itemDataTemp
        }

        // add the primaryColor to array of availablePrimaryColors
        pub fun addPrimaryColor(color: String) {
            pre {
                !S1ItemNFT.availablePrimaryColors.contains(color):
                "availablePrimaryColors already has color"
            }
            S1ItemNFT.availablePrimaryColors.append(color)
            emit PrimaryColorAdded(color: color)
        }

        // add the primaryColor to array of availablePrimaryColors
        pub fun addSecondaryColor(color: String) {
            pre {
                !S1ItemNFT.availableSecondaryColors.contains(color):
                "availableSecondaryColors does not contain color"
            }
            S1ItemNFT.availableSecondaryColors.append(color)
            emit SecondaryColorAdded(color: color)
        }

        // add the primaryColor to array of availablePrimaryColors
        pub fun removePrimaryColor(color: String) {
            pre {
                S1ItemNFT.availablePrimaryColors.contains(color):
                "availablePrimaryColors does not contain color"
            }
            var index = 0
            for primaryColor in S1ItemNFT.availablePrimaryColors {
                if primaryColor == color {
                    S1ItemNFT.availablePrimaryColors.remove(at: index)
                } else {
                    index = index + 1
                }
            }
            emit PrimaryColorRemoved(color: color)
        }

        // add the primaryColor to array of availablePrimaryColors
        pub fun removeSecondaryColor(color: String) {
            pre {
                S1ItemNFT.availableSecondaryColors.contains(color):
                "availableSecondaryColors already has color"
            }
            var index = 0
            for secondaryColor in S1ItemNFT.availableSecondaryColors {
                if secondaryColor == color {
                    S1ItemNFT.availableSecondaryColors.remove(at: index)
                } else {
                    index = index + 1
                }
            }
            emit SecondaryColorRemoved(color: color)
        }
        
        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // Change the royalty percentage of the contract
        pub fun makeSplittable() {
            S1ItemNFT.isSplittable = true
            
            emit ItemNFTNowSplittable()
        }

        // Change the royalty percentage of the contract
        pub fun changeItemDataNumberMintable(itemDataID: UInt32, number: UInt32) {
            S1ItemNFT.numberMintablePerItemData[itemDataID] = number
            
            emit numberItemDataMintableChanged(itemDataID: itemDataID, number: number)
        }

        // Retire itemData so that it cannot be used to mint anymore
        pub fun retireItemData(itemDataID: UInt32) {           
            pre {
                S1ItemNFT.isItemDataRetired[itemDataID] != nil: "Cannot retire item: Item doesn't exist!"
            }

            if !S1ItemNFT.isItemDataRetired[itemDataID]! {
                S1ItemNFT.isItemDataRetired[itemDataID] = true

                emit ItemDataIDRetired(itemDataID: itemDataID)
            }
        }
    }
    // -----------------------------------------------------------------------
    // Item contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Items in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create S1ItemNFT.Collection()
    }

    // get dictionary of numberMintedPerItem
    pub fun getNumberMintedPerItem(): {UInt32: UInt32} {
        return S1ItemNFT.numberMintedPerItem
    }

    // get how many Items with itemDataID are minted 
    pub fun getItemNumberMinted(id: UInt32): UInt32 {
        let numberMinted = S1ItemNFT.numberMintedPerItem[id]??
            panic("itemDataID not found")
        return numberMinted
    }

    // get the ItemData of a specific id
    pub fun getItemData(id: UInt32): ItemData {
        let itemData = S1ItemNFT.itemDatas[id]??
            panic("itemDataID not found")
        return itemData
    }

    // get the map of item data allocations
    pub fun getItemDataAllocations(): {String: UInt32} {
        let itemDataAllocation = S1ItemNFT.itemDataAllocation
        return itemDataAllocation
    }

    // get the itemData allocation from the garment and material dataID
    pub fun getItemDataAllocation(garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String): UInt32 {
        let itemDataAllocation = S1ItemNFT.itemDataAllocation[S1ItemNFT.getUniqueString(
            garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor)]??
            panic("garment and material dataID pair not allocated")

        return itemDataAllocation
    }
    // get all ItemDatas created
    pub fun getItemDatas(): {UInt32: ItemData} {
        return S1ItemNFT.itemDatas
    }

    pub fun getAvailablePrimaryColors(): [String] { 
        return S1ItemNFT.availablePrimaryColors
    }

    pub fun getAvailableSecondaryColors(): [String] { 
        return S1ItemNFT.availableSecondaryColors
    }

    // get dictionary of itemdataids and whether they are retired
    pub fun getItemDatasRetired(): {UInt32: Bool} { 
        return S1ItemNFT.isItemDataRetired
    }

    // get bool of if itemdataid is retired
    pub fun getItemDataRetired(itemDataID: UInt32): Bool? { 
        return S1ItemNFT.isItemDataRetired[itemDataID]!
    }

    pub fun getUniqueString(garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String): String {
        return garmentDataID.toString()
            .concat("_")
            .concat(materialDataID.toString())
            .concat("_")
            .concat(primaryColor)
            .concat("_")
            .concat(secondaryColor)
    }
    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.itemDatas = {}
        self.itemDataAllocation = {}
        self.numberMintedPerItem = {}
        self.numberMintablePerItemData = {}
        self.nextItemDataID = 1
        self.nextItemDataAllocation = 1
        self.isSplittable = false
        self.isItemDataRetired = {}
        self.totalSupply = 0
        self.availablePrimaryColors = []
        self.availableSecondaryColors = []
        
        self.CollectionPublicPath = /public/S1ItemCollection0015
        self.CollectionStoragePath = /storage/S1ItemCollection0015
        self.AdminStoragePath = /storage/S1ItemAdmin0015

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{ItemCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        
        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}


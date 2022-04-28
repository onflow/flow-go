import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import GarmentNFT from 0xfc91de5e6566cc7c
import MaterialNFT from 0xfc91de5e6566cc7c
import FBRC from 0xfc91de5e6566cc7c


pub contract ItemNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // ItemNFT contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Item contract is created
    pub event ContractInitialized()

    // Emitted when a new ItemData struct is created
    pub event ItemDataCreated(itemDataID: UInt32, mainImage: String, images: [String])

    // Emitted when a Item is mintee
    pub event ItemMinted(itemID: UInt64, itemDataID: UInt32, serialNumber: UInt32)

    // Emitted when a Item' name is changed
    pub event ItemNameChanged(id: UInt64, name: String)

    // Emitted when the contract's royalty percentage is changed
    pub event RoyaltyPercentageChanged(newRoyaltyPercentage: UFix64)

    pub event ItemDataAllocated(garmentDataID: UInt32, materialDataID: UInt32, itemDataID: UInt32)

    // Emitted when the items are set to be splittable
    pub event ItemNFTNowSplittable()

    pub event numberItemDataMintableChanged(number: UInt32)

    pub event ItemDataIDRetired(itemDataID: UInt32)

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

    // Dictionary with ItemDataID as key and number of NFTs with that ItemDataID are minted
    access(self) var numberMintedPerItem: {UInt32: UInt32}

    // Variable size dictionary of Item structs
    access(self) var itemDatas: {UInt32: ItemData}

    // ItemData of item minted is based on garmentDataID of garment and materialDataID of material used {materialDataID: {garmentDataID: itemDataID}
    access(self) var itemDataAllocation: {UInt32: {UInt32: UInt32}}

    // Dictionary of itemDataID to  whether they are retired
    access(self) var isItemDataRetired: {UInt32: Bool}

    // Keeps track of how many unique ItemData's are created
    pub var nextItemDataID: UInt32

    pub var nextItemDataAllocation: UInt32

    // Are garment and material removable from item
    pub var isSplittable: Bool

    // The maximum number of items with itemDataID mintable
    pub var numberItemDataMintable: UInt32

    pub var royaltyPercentage: UFix64

    pub var totalSupply: UInt64

    pub struct ItemData {

        // The unique ID for the Item Data
        pub let itemDataID: UInt32
        //stores link to image
        pub let mainImage: String
        //stores link to supporting images
        pub let images: [String]

        init(
            mainImage: String,
            images: [String],
        ){
            self.itemDataID = ItemNFT.nextItemDataID
            self.mainImage = mainImage
            self.images = images

            ItemNFT.isItemDataRetired[self.itemDataID] = false

            // Increment the ID so that it isn't used again
            ItemNFT.nextItemDataID = ItemNFT.nextItemDataID + 1 as UInt32

            emit ItemDataCreated(itemDataID: self.itemDataID, mainImage: self.mainImage, images: self.images)
        }
    }

    pub struct Item {

        // The ID of the itemData that the item references
        pub let itemDataID: UInt32

        // The N'th NFT with 'ItemDataID' minted
        pub let serialNumber: UInt32

        init(itemDataID: UInt32) {
            pre {
                //Only one Item with 'ItemDataID' can be minted 
                Int(ItemNFT.numberMintedPerItem[itemDataID]!) < Int(ItemNFT.numberItemDataMintable): 
                "maximum number of ItemNFT with itemDataID minted"
            }
            
            self.itemDataID = itemDataID

            // Increment the ID so that it isn't used again
            ItemNFT.numberMintedPerItem[itemDataID] = ItemNFT.numberMintedPerItem[itemDataID]! + 1 as UInt32

            self.serialNumber = ItemNFT.numberMintedPerItem[itemDataID]!

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
        
        // Royalty capability which NFT will use
        pub let royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>

        // after you remove the garment and material from the item, the ItemNFT will be considered "dead". 
        // accounts will be unable to deposit, withdraw or call functions of the nft.
        pub var isDead : Bool

        // this is where the garment nft is stored, it cannot be moved out
        access(self) var garment: @GarmentNFT.NFT?

        // this is where the material nft is stored, it cannot be moved out
        access(self) var material: @MaterialNFT.NFT?


        init(serialNumber: UInt32, name: String, itemDataID: UInt32, royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>, garment: @GarmentNFT.NFT, material: @MaterialNFT.NFT) {
            
            ItemNFT.totalSupply = ItemNFT.totalSupply + 1 as UInt64
            
            self.id = ItemNFT.totalSupply

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
        access(contract) fun split(garmentCap: Capability<&{GarmentNFT.GarmentCollectionPublic}>, materialCap: Capability<&{MaterialNFT.MaterialCollectionPublic}>) {
            pre {
                !self.isDead: 
                "Cannot split. Item is dead"
                ItemNFT.isSplittable:
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
            ItemNFT.numberMintedPerItem[self.item.itemDataID] = ItemNFT.numberMintedPerItem[self.item.itemDataID]! - 1 as UInt32
            self.isDead = true
        }

        // get a reference to the garment that item stores
        pub fun borrowGarment(): &GarmentNFT.NFT? {
            let garmentOptional <- self.garment <- nil
            let garment <- garmentOptional!
            let garmentRef = &garment as auth &GarmentNFT.NFT
            self.garment <-! garment
            return garmentRef   
        }

        // get a reference to the material that item stores
        pub fun borrowMaterial(): &MaterialNFT.NFT?  {
            let materialOptional <- self.material <- nil
            let material <- materialOptional!
            let materialRef = &material as auth &MaterialNFT.NFT
            self.material <-! material
            return materialRef
        }

        // change name of item nft
        access(contract) fun changeName(name: String) {
            pre {
                !self.isDead: 
                "Cannot change garment name. Item is dead"
            }
            self.name = name;

           emit ItemNameChanged(id: self.id, name: self.name)
        }
    }

    //destroy item if it is considered dead
    pub fun cleanDeadItems(item: @ItemNFT.NFT) {
        pre {
            item.isDead: 
            "Cannot destroy, item not dead" 
        }
        destroy item
    }

    // mint the NFT, combining a garment and boot. 
    // The itemData that is used to mint the Item is based on the garment and material' garmentDataID and materialDataID
    pub fun mintNFT(name: String, royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>, garment: @GarmentNFT.NFT, material: @MaterialNFT.NFT): @NFT {
        pre {
            royaltyVault.check():
                "Royalty capability is invalid!"
        }

        let garmentDataID = garment.garment.garmentDataID

        let materialDataID = material.material.materialDataID
        
        let isValidGarmentMaterialPair = ItemNFT.itemDataAllocation[garmentDataID]??
            panic("garment and material dataID pair not allocated")

        // get the itemdataID of the item to be minted based on garment and material dataIDs
        let itemDataID = isValidGarmentMaterialPair[materialDataID]?? 
            panic("itemDataID not allocated")

        if (ItemNFT.isItemDataRetired[itemDataID]! == nil) {
            panic("Cannot mint Item. ItemData not found")
        }

        if (ItemNFT.isItemDataRetired[itemDataID]!) {
            panic("Cannot mint Item. ItemDataID retired")
        }

        let numInItem = ItemNFT.numberMintedPerItem[itemDataID]??
            panic("itemDataID not found")

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
        pub fun borrowItem(id: UInt64): &ItemNFT.NFT? {
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

        // change name of item nft
        pub fun changeName(id: UInt64, name: String) {
            let token <- self.ownedNFTs.remove(key: id)
                ?? panic("Cannot withdraw: Item does not exist in the collection")
            
            let item <- token as! @ItemNFT.NFT
            item.changeName(name: name)

            self.ownedNFTs[id] <-! item
        }

        pub fun split(id: UInt64, garmentCap: Capability<&{GarmentNFT.GarmentCollectionPublic}>, materialCap: Capability<&{MaterialNFT.MaterialCollectionPublic}>){
            let token <- self.ownedNFTs.remove(key: id)
                ?? panic("Cannot withdraw: Item does not exist in the collection")
            
            let item <- token as! @ItemNFT.NFT
            item.split(garmentCap: garmentCap, materialCap: materialCap)

            self.ownedNFTs[id] <-! item
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
            let token <- token as! @ItemNFT.NFT
            
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
        pub fun borrowItem(id: UInt64): &ItemNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ItemNFT.NFT
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

        // create itemdataid allocation from the garmentdataid and materialdataid
        pub fun createItemDataAllocation(garmentDataID: UInt32, materialDataID: UInt32){

            if(ItemNFT.itemDataAllocation[garmentDataID] != nil) {
                if(ItemNFT.itemDataAllocation[garmentDataID]![materialDataID] != nil){
                    panic("ItemData already allocated")
                } else {
                    let dict = ItemNFT.itemDataAllocation[garmentDataID]!
                    dict[materialDataID] = ItemNFT.nextItemDataAllocation
                    ItemNFT.itemDataAllocation[garmentDataID] = dict
                }
            } else {
                let dict: {UInt32: UInt32} = {}
                dict[materialDataID] = ItemNFT.nextItemDataAllocation
                ItemNFT.itemDataAllocation[garmentDataID] = dict
            } 
            emit ItemDataAllocated(garmentDataID: garmentDataID, materialDataID: materialDataID, itemDataID: ItemNFT.nextItemDataAllocation)
            ItemNFT.nextItemDataAllocation = ItemNFT.nextItemDataAllocation + 1 as UInt32

        }
        
        pub fun createItemData(mainImage: String, images: [String]): UInt32 {
            // Create the new Item
            var newItem = ItemData(mainImage: mainImage, images: images)
        
            let newID = newItem.itemDataID

            // Store it in the contract storage
            ItemNFT.itemDatas[newID] = newItem

            ItemNFT.numberMintedPerItem[newID] = 0 as UInt32
            return newID
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // Change the royalty percentage of the contract
        pub fun changeRoyaltyPercentage(newRoyaltyPercentage: UFix64) {
            ItemNFT.royaltyPercentage = newRoyaltyPercentage
            
            emit RoyaltyPercentageChanged(newRoyaltyPercentage: newRoyaltyPercentage)
        }

        // Change the royalty percentage of the contract
        pub fun makeSplittable() {
            ItemNFT.isSplittable = true
            
            emit ItemNFTNowSplittable()
        }

        // Change the royalty percentage of the contract
        pub fun changeItemDataNumberMintable(number: UInt32) {
            ItemNFT.numberItemDataMintable = number
            
            emit numberItemDataMintableChanged(number: number)
        }

        // Retire itemData so that it cannot be used to mint anymore
        pub fun retireItemData(itemDataID: UInt32) {           
            pre {
                ItemNFT.isItemDataRetired[itemDataID] != nil: "Cannot retire item: Item doesn't exist!"
            }

            if !ItemNFT.isItemDataRetired[itemDataID]! {
                ItemNFT.isItemDataRetired[itemDataID] = true

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
        return <-create ItemNFT.Collection()
    }

    // get dictionary of numberMintedPerItem
    pub fun getNumberMintedPerItem(): {UInt32: UInt32} {
        return ItemNFT.numberMintedPerItem
    }

    // get how many Items with itemDataID are minted 
    pub fun getItemNumberMinted(id: UInt32): UInt32 {
        let numberMinted = ItemNFT.numberMintedPerItem[id]??
            panic("itemDataID not found")
        return numberMinted
    }

    // get the ItemData of a specific id
    pub fun getItemData(id: UInt32): ItemData {
        let itemData = ItemNFT.itemDatas[id]??
            panic("itemDataID not found")
        return itemData
    }

    // get the map of item data allocations
    pub fun getItemDataAllocations(): {UInt32: {UInt32: UInt32}} {
        let itemDataAllocation = ItemNFT.itemDataAllocation
        return itemDataAllocation
    }

    // get the itemData allocation from the garment and material dataID
    pub fun getItemDataAllocation(garmentDataID: UInt32, materialDataID: UInt32): UInt32 {
        let isValidGarmentMaterialPair = ItemNFT.itemDataAllocation[garmentDataID]??
            panic("garment and material dataID pair not allocated")

        // get the itemdataID of the item to be minted based on garment and material dataIDs
        let itemDataAllocation = isValidGarmentMaterialPair[materialDataID]?? 
            panic("itemDataID not allocated")

        return itemDataAllocation
    }
    // get all ItemDatas created
    pub fun getItemDatas(): {UInt32: ItemData} {
        return ItemNFT.itemDatas
    }

    // get dictionary of itemdataids and whether they are retired
    pub fun getItemDatasRetired(): {UInt32: Bool} { 
        return ItemNFT.isItemDataRetired
    }

    // get bool of if itemdataid is retired
    pub fun getItemDataRetired(itemDataID: UInt32): Bool? { 
        return ItemNFT.isItemDataRetired[itemDataID]!
    }


    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.itemDatas = {}
        self.itemDataAllocation = {}
        self.numberMintedPerItem = {}
        self.nextItemDataID = 1
        self.nextItemDataAllocation = 1
        self.isSplittable = false
        self.numberItemDataMintable = 1
        self.isItemDataRetired = {}
        self.royaltyPercentage = 0.10
        self.totalSupply = 0
        
        self.CollectionPublicPath = /public/ItemCollection20
        self.CollectionStoragePath = /storage/ItemCollection20
        self.AdminStoragePath = /storage/ItemAdmin20



        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{ItemCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        
        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
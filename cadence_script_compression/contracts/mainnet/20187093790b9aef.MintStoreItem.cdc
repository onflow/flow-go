/*
    Description: 

    authors: Matthew Balazsi (matthew@mint.store), Joseph Djenandji (joseph@mint.store)
    
    This smart contract contains the core functionality for Mint Store NFTs. 
    
    MINT is a platform where teams can create a fully branded environment to sell NFTs and launch branded marketplaces. 
    This will give fans a fully immersive experience as they interact with drops, buy and sell in the marketplace, and 
    deepen their relationships with the brand.

    NFTs will be minted in groups called Editions. Each item in the edition will contain identical metadata except for 
    the editionNumber (or print number). Editions can either be open (no limit to number of items minted), or closed 
    (only a fixed number of items can be printed).

    When a new NFT needs to be minted...
    Here is the Recipe for the Mint Julep: 
        1 teaspoon Powdered sugar
        2 oz. Bourbon whiskey 
        2 teaspoons Water 
        4 Mint leaves
    
    Enjoy!
*/

// import NonFungibleToken from "./NonFungibleToken.cdc"
// import MetadataViews from "./MetadataViews.cdc"

// for tests
// import NonFungibleToken from NonFungibleToken
// import MetadataViews from MetadataViews

// for testnet
// import NonFungibleToken from 0x631e88ae7f1d7c20
// import MetadataViews from 0x631e88ae7f1d7c20

// for mainnet
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448


pub contract MintStoreItem: NonFungibleToken {

    // -----------------------------------------------------------------------
    // MintStoreItem contract Events
    // -----------------------------------------------------------------------

    // Emitted when the MintStoreItem contract is created
    pub event ContractInitialized()

    // Emitted when a new Edition struct is created
    pub event EditionCreated(id: UInt32, name: String, printingLimit: UInt32?)

    // Emitted when Edition Metadata is updated
    pub event EditionMetadaUpdated(editionID: UInt32)

    // Emitted when a new Merchant is created
    pub event MerchantCreated(merchantID: UInt32, name: String)


    // Emitted when a new Merchant is updated
    pub event MerchantUpdated(merchantID: UInt32, name: String)


    // Emitted when a new item was minted
    pub event ItemMinted(itemID:UInt64, merchantID: UInt32, editionID: UInt32, editionNumber: UInt32)

    // Item related events 
    //
    // Emitted when an Item is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when an Item is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)
    // Emitted when an Item is destroyed
    pub event ItemDestroyed(id: UInt64)


    // Named paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath


    // -----------------------------------------------------------------------
    // MintStoreItem contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Variable size dictionary of Editions resources
    access(self) var editions: @{UInt32: Edition}


    // The ID that is used to create Admins. 
    // Every Admins should have a unique identifier.
    pub var nextAdminID: UInt32

    

    // The ID that is used to create Editions. 
    // Every time an Edition is created, nextEditionID is assigned 
    // to the edition and then is incremented by 1.
    pub var nextEditionID: UInt32


    // Variable size dictionary of Merchant names
    access(self) var merchants: {UInt32: String}

    // The ID that is used to create Merchants. 
    // Every time a Merchant is created, nextMerchantID is assigned 
    // to the merchant and then is incremented by 1.
    pub var nextMerchantID: UInt32


    // The total number of MintStoreItem NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global nft IDs for minting.
    pub var totalSupply: UInt64


    // -----------------------------------------------------------------------
    // MintStoreItem contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------
   

    // EditionData is a struct definition to have all of the same fields as the Edition resource.
    // it can be used to publicly read Edition data

    pub struct EditionData {  
        pub let editionID: UInt32  
        pub let merchantID: UInt32  
        pub let name: String  
        pub var items: [UInt64]  
        pub var metadata: {String: String}
        pub var numberOfItemsMinted: UInt32
        pub var printingLimit: UInt32?
        
        init(editionID: UInt32) {

             if MintStoreItem.editions[editionID] == nil {
                panic("the editionID was not found")
            }
            let editionToRead = &MintStoreItem.editions[editionID] as &Edition

            self.editionID = editionID
            self.metadata = editionToRead.metadata
            self.merchantID = editionToRead.merchantID
            self.name = editionToRead.name
            self.printingLimit = editionToRead.printingLimit
            self.numberOfItemsMinted=editionToRead.numberOfItemsMinted
            self.items=editionToRead.items
        }
    }
    // Edition is a Ressource that holds metadata associated 
    // with a specific MintStoreItem
    //
    // MintStore NFTs will all reference an Edition as the owner of
    // its metadata. The Editions are publicly accessible, so anyone can
    // read the metadata associated with a specific EditionID
    //
    pub resource Edition {

        // The unique ID for the Edition
        pub let editionID: UInt32

        // The ID of the merchant that owns the edition
        pub let merchantID: UInt32

        // Stores all the metadata about the edition as a string mapping
        // This is not the long term way NFT metadata will be stored. It's a temporary
        // construct while we figure out a better way to do metadata.
        //
        pub let metadata: {String: String}

        // Array of items that are a part of this collection.
        // When an item is added to the collection, its ID gets appended here.
        pub var items: [UInt64]

        // The number of items minted in this collection.
        // When an item is added to the collection, the numberOfItems is incremented by 1
        // It will be used to identify the editionNumber of an item
        // if the edition is open (printingLimit=nil), we can keep minting new items
        // if the edition is limited (printingLimit!=nil), we can keep minting items until we reach printingLimit
        pub var numberOfItemsMinted: UInt32


        // the limit of items that can be minted. For open editions, this value should be set to nil.
        pub var printingLimit: UInt32?

        // the name of the edition
        pub var name: String

        init(merchantID: UInt32, metadata: {String: String}, name: String, printingLimit:UInt32?) {
            pre {
                metadata.length != 0: "Metadata cannot be empty"
                name!=nil: "Name is undefined"
            }
            self.editionID = MintStoreItem.nextEditionID
            self.merchantID = merchantID
            self.metadata = metadata
            self.name = name
            self.printingLimit = printingLimit
            self.numberOfItemsMinted=0
            self.items = []

            
            // Increment the ID so that it isn't used again
            MintStoreItem.nextEditionID = MintStoreItem.nextEditionID + (1 as UInt32)

            emit EditionCreated(id: self.editionID, name: self.name, printingLimit: self.printingLimit)
        }


        // mintItem mints a new Item and returns the newly minted Item
        // 
        // Pre-Conditions:
        // If the edition is limited the number of items minted in the edition must be strictly less than the printing limit
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintItem(): @NFT {
            pre {
                (self.numberOfItemsMinted < (self.printingLimit ?? (4294967295 as UInt32)  )): "We have reached the printing limit for this edition"
            }

            // Gets the number of Itms that have been minted for this Edition
            // to use as this Item's edition number
            let numMinted = self.numberOfItemsMinted + (1 as UInt32)

            // Mint the new item
            let newItem: @NFT <- create NFT(merchantID: self.merchantID, editionID: self.editionID, editionNumber: numMinted)


            // Add the Item to the array of items
            self.items.append(newItem.id)                            

            // Increment the count of Items
            self.numberOfItemsMinted = numMinted 

            return <-newItem
        }

        // batchMintItems mints an arbitrary quantity of Items 
        // and returns them as a Collection
        // Be sure there are enough 
        //
        // Parameters: quantity: The quantity of Items to be minted
        //
        // Returns: Collection object that contains all the Items that were minted
        //
        pub fun batchMintItems(quantity: UInt32): @Collection {
           
            pre {
                ((self.numberOfItemsMinted+quantity)<=(self.printingLimit ?? (4294967295 as UInt32))): "We have reached the printing limit for this edition"
            }

            let newCollection <- create Collection()

            var i: UInt32 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintItem())
                i = i + (1 as UInt32)
            }

            return <-newCollection
        }

    }

    // The struct representing an NFT Item data
    pub struct ItemData {


        // The ID of the merchant 
        pub let merchantID: UInt32

        // The ID of the edition that the NFT comes from
        pub let editionID: UInt32

        // The number of the NFT within the edition
        pub let editionNumber: UInt32



        init(merchantID: UInt32, editionID: UInt32, editionNumber: UInt32) {
            self.merchantID = merchantID
            self.editionID = editionID
            self.editionNumber = editionNumber
        }

    }

    // The resource that represents the Item NFTs
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {

        // Global unique item ID
        pub let id: UInt64

        // Struct of MintStoreItem metadata
        pub let data: ItemData


        init(merchantID: UInt32, editionID: UInt32, editionNumber: UInt32) {
            
            pre{
                editionID > (0 as UInt32): "editionID cannot be 0"
                editionNumber > (0 as UInt32): "editionNumber cannot be 0"
            }
            // Increment the global Item IDs
            MintStoreItem.totalSupply = MintStoreItem.totalSupply + (1 as UInt64)

            self.id = MintStoreItem.totalSupply

            // Set the metadata struct
            self.data = ItemData(merchantID: merchantID, editionID: editionID, editionNumber: editionNumber)

            
            emit ItemMinted(itemID: self.id, merchantID: merchantID, editionID: editionID, editionNumber: editionNumber)
        }


        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }


        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    let edition = EditionData(editionID: self.data.editionID);
                    return MetadataViews.Display(
                        name: edition.name,
                        description: edition.metadata["description"] ?? "",
                        thumbnail: MetadataViews.HTTPFile(
                            url: edition.metadata["thumbnail"] ?? ""
                        )
                    )
            }

            return nil
        }


        // If the Item is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            emit ItemDestroyed(id: self.id)
        }

    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Editions and Items
    //
    pub resource Admin {

        pub let id: UInt32
        // createEdition creates a new Edition struct 
        // and stores it in the Editions dictionary in the MintStore contract
        //

        init(id: UInt32) {
            self.id = id
        }


        // createEdition creates a new Edition resource and stores it
        // in the editions mapping in the MintStoreItem contract
        //
        // Parameters: 
        //  merchantID: The ID of the merchant
        //  metadata: the associated data
        //  name: The name of the Edition
        //  printingLimit: We can only mint this quantity of NFTs. If printingLimit is nil there is no limit (theoretically UInt32.max)
        //
        pub fun createEdition(merchantID: UInt32, metadata: {String: String}, name: String, printingLimit: UInt32?) {
            // Create the new Edition
            var newEdition <- create Edition(merchantID:merchantID, metadata: metadata, name: name, printingLimit:printingLimit)
            let newID = newEdition.editionID

            // Store it in the contract storage
            MintStoreItem.editions[newID] <-! newEdition

        }


        // borrowEdition returns a reference to an edition in the MintStoreItem
        // contract so that the admin can call methods on it
        //
        // Parameters: editionID: The ID of the Edition that you want to
        // get a reference to
        //
        // Returns: A reference to the Edition with all of the fields
        // and methods exposed
        //
        pub fun borrowEdition(editionID: UInt32): &Edition {
            pre {
                MintStoreItem.editions[editionID] != nil: "Cannot borrow Edition: it does not exist"
            }
            
            // Get a reference to the Edition and return it
            return &MintStoreItem.editions[editionID] as &Edition
        }

        // updateEditionMetadata returns a reference to an edition in the MintStoreItem
        // contract so that the admin can call methods on it
        //
        // Parameters: 
        // editionID: The ID of the Edition that you want to update
        //
        // updates: a dictionary of key - values that is requested to be appended
        //
        // suffix: If the metadata already contains an attribute with a given key, this value should still be kept 
        // for posteriority. Therefore, the old value to be replaced will be stored in a metadata entry with key = key+suffix. 
        // This can offer some reassurance to the NFT owner that the metadata will never disappear.
        // 
        // Returns: the EditionID
        //
        pub fun updateEditionMetadata(editionID: UInt32, updates: {String:String}, suffix: String): UInt32 {
            pre {
                MintStoreItem.editions[editionID] != nil: "Cannot borrow Edition: it does not exist"
            }

            let edition = self.borrowEdition(editionID: editionID)

            // prevalidation 
            // if metadata[key] exists and metadata[key+suffix] exists, we have a clash.
            for key in updates.keys {

                let newKey = key.concat(suffix)

                if edition.metadata[key] != nil && edition.metadata[newKey]!=nil {
                    var errorMsg = "attributes "
                    errorMsg = errorMsg.concat(key).concat(" and ").concat(newKey).concat(" are already defined")
                    panic(errorMsg)
                }
                    

            }

            // execution
            for key in updates.keys {

                let newKey = key.concat(suffix)

                if edition.metadata[key] != nil {
                    edition.metadata[newKey] = edition.metadata[key]    
                }
                edition.metadata[key] = updates[key]
                
            }


            emit EditionMetadaUpdated(editionID: editionID)
            
            // Return the EditionID and return it
            return editionID
        }


        pub fun createMerchant(merchantName: String): UInt32 {

            pre {
                merchantName != nil: "Cannot create the merchant: merchantName is nil"
            }
            let newID = MintStoreItem.nextMerchantID


             // Create the new Merchant
            MintStoreItem.merchants[newID] = merchantName

             // Increment the ID so that it isn't used again
            MintStoreItem.nextMerchantID = MintStoreItem.nextMerchantID + (1 as UInt32)

            emit MerchantCreated(merchantID: newID, name: merchantName)
            return newID
        }


         pub fun updateMerchant(merchantID: UInt32, merchantName: String): UInt32 {

            pre {
                MintStoreItem.merchants[merchantID] !=nil: "Cannot upate the merchant: merchantID is not initialized"
                merchantName != nil: "Cannot upate the merchant: merchantName is nil"
            }

             // Update the new Merchant
            MintStoreItem.merchants[merchantID] = merchantName

            emit MerchantUpdated(merchantID: merchantID, name: merchantName)
            return merchantID
        }



        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            

            let newID = MintStoreItem.nextAdminID
             // Increment the ID so that it isn't used again
            MintStoreItem.nextAdminID = MintStoreItem.nextAdminID + (1 as UInt32)

            return <-create Admin(id: newID)
        }

    }



    // This is the interface that users can cast their MintStoreItem Collection as
    // to allow others to deposit MintStoreItems into their Collection. It also allows for reading
    // the IDs of MintStoreItems in the Collection.
    pub resource interface MintStoreItemCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMintStoreItem(id: UInt64): &MintStoreItem.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow MintStoreItem reference: The ID of the returned reference is incorrect"
            }
        }
        
    }



    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: MintStoreItemCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of MintStore conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        // pub var ownedNFTs: @{UInt64: MintStoreItem.NFT}

        init() {
            self.ownedNFTs <- {}
        }




        // withdraw removes a MintStoreItem from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: MintStoreItem does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn MintStore items
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {

                let token <-self.withdraw(withdrawID: id)

                batchCollection.deposit(token: <-token)
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a MintStoreItem and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a MintStoreItem NFT to make sure
            // it is the correct type
            let token <- token as! @MintStoreItem.NFT

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

        // borrowNFT Returns a borrowed reference to a MintStoreItem in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any MintStoreItem specific data. Please use borrowMintStoreItem to 
        // read MintStoreItem data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowMintStoreItem returns a borrowed reference to a MintStoreItem
        // so that the caller can read data and call methods from it.
        // They can use this to read its editionID, editionNumber,
        // or any edition data associated with it by
        // getting the editionID and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowMintStoreItem(id: UInt64): &MintStoreItem.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MintStoreItem.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }



    // -----------------------------------------------------------------------
    // MintStoreItem contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // MintStoreItems in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create MintStoreItem.Collection()
    }

    pub fun createEmptyMintStoreItemCollection(): @MintStoreItem.Collection {
        return <-create MintStoreItem.Collection()
    }



    // getMerchantIDs returns an array of the merchant IDs
    pub fun getMerchantIDs(): [UInt32] {
        return self.merchants.keys
    }

    // getMerchantNames returns an array of the merchant Names
    pub fun getMerchantNames(): [String] {
        return self.merchants.values
    }

    // getMerchant returns the merchant name with a given merchantID
    pub fun getMerchant(merchantID: UInt32): String? {
        return self.merchants[merchantID]
    }


    // -----------------------------------------------------------------------
    // MintStoreItem initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.editions <- {}
        self.nextEditionID = 1
        self.totalSupply = 0
        self.merchants = {}
        self.nextMerchantID = 1


        self.CollectionStoragePath = /storage/MintStoreItemCollection
        self.CollectionPublicPath = /public/MintStoreItemCollection
        self.AdminStoragePath = /storage/MintStoreItemAdmin

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{MintStoreItemCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the admin ressource in storage
        self.account.save<@Admin>(<- create Admin(id: 1), to: self.AdminStoragePath)
        self.nextAdminID = 2

        emit ContractInitialized()
    }


}
    


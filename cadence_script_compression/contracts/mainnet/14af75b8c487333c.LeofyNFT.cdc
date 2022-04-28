/*
    Description: Central Smart Contract for Leofy

    This smart contract contains the core functionality for 
    Leofy, created by LEOFY DIGITAL S.L.

    The contract manages the data associated with all the items
    that are used as templates for the NFTs

    Then an Admin can create new Items. Items consist of a public struct that 
    contains public information about a item, and a private resource used
    to mint new NFT's linked to the Item.

    The admin resource has the power to do all of the important actions
    in the smart contract. When admins want to call functions in a Item,
    they call their borrowItem function to get a reference 
    to a item in the contract. Then, they can call functions on the item using that reference.
    
    When NFTs are minted, they are initialized with a ItemID and
    are returned by the minter.

    The contract also defines a Collection resource. This is an object that 
    every Leofy NFT owner will store in their account
    to manage their NFT collection.

    The main Leofy account will also have its own NFT's collections
    it can use to hold its own NFT's that have not yet been sent to a user.

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.

*/

import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import LeofyCoin from 0x14af75b8c487333c

pub contract LeofyNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Leofy contract Events
    // -----------------------------------------------------------------------

    // Emitted when the LeofyNFT contract is created
    pub event ContractInitialized()

    // Emitted when a new Item struct is created
    pub event ItemCreated(id: UInt64, metadata: {String:String})
    pub event SetCreated(id: UInt64, name: String)

    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, itemID: UInt64, serialNumber: UInt32)

    // Named Paths
    //
    pub let ItemStoragePath: StoragePath
    pub let ItemPublicPath: PublicPath

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // TopShot contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Variable size dictionary of Item structs
    //access(self) var items: @{UInt64: Item}

    pub var totalSupply: UInt64
    pub var totalItemSupply: UInt64

    // -----------------------------------------------------------------------
    // LeofyNFT contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    
    // Item is a Resource that holds metadata associated 
    // with a specific Artist Item, like the picture from Artist John Doe
    //
    // Leofy NFTs will all reference a single item as the owner of
    // its metadata. 
    //

    pub resource interface ItemCollectionPublic {
        pub fun getIDs(): [UInt64]
        pub fun getItemsLength(): Int
        pub fun getItemMetaDataByField(itemID: UInt64, field: String): String?
        pub fun borrowItem(itemID: UInt64): &Item{ItemPublic}
    }

    pub resource ItemCollection: ItemCollectionPublic {
        pub var items: @{UInt64: LeofyNFT.Item}

        init () {
            self.items <- {}
        }

        pub fun createItem(metadata: {String: String}, price: UFix64): UInt64 {

            // Create the new Item
            var newItem <- create Item(
               metadata: metadata,
               price: price
            )
            
            let newID = newItem.itemID

            // Store it in the contract storage
            self.items[newID] <-! newItem
            emit ItemCreated(id: LeofyNFT.totalItemSupply, metadata:metadata)

            // Increment the ID so that it isn't used again
            LeofyNFT.totalItemSupply = LeofyNFT.totalItemSupply + 1

            return newID            
        }

        pub fun borrowItem(itemID: UInt64): &Item{ItemPublic} {
            pre {
                self.items[itemID] != nil: "Cannot borrow Item: The Item doesn't exist"
            }

            return &self.items[itemID] as &Item{ItemPublic};
        }

        // getIDs returns an array of the IDs that are in the Item Collection
        pub fun getIDs(): [UInt64] {
            return self.items.keys
        }

        // getItemsLength 
        // Returns: Int length of items created
        pub fun getItemsLength(): Int {
            return self.items.length
        }

        // getItemMetaDataByField returns the metadata associated with a 
        //                        specific field of the metadata
        //                        Ex: field: "Artist" will return something
        //                        like "John Doe"
        // 
        // Parameters: itemID: The id of the Item that is being searched
        //             field: The field to search for
        //
        // Returns: The metadata field as a String Optional
        pub fun getItemMetaDataByField(itemID: UInt64, field: String): String? {
            // Don't force a revert if the itemID or field is invalid
            if( self.items[itemID] != nil){
            let item = &self.items[itemID] as &Item
            return item.metadata[field]
            }
            else{
                return nil;
            }
        }
        
        destroy(){
            destroy self.items
        }
    }

    pub resource interface ItemPublic{
        pub let itemID: UInt64
        pub var numberMinted: UInt32
        pub var price: UFix64

        pub fun getMetadata(): {String: String}
        pub fun borrowCollection(): &LeofyNFT.Collection{LeofyCollectionPublic}
        pub fun purchase(payment: @FungibleToken.Vault): @NonFungibleToken.NFT
    }

    pub resource Item: ItemPublic {

        // The unique ID for the Item
        pub let itemID: UInt64

        // Stores all the metadata about the item as a string mapping
        // This is not the long term way NFT metadata will be stored. It's a temporary
        // construct while we figure out a better way to do metadata.
        //
        access(contract) let metadata: {String: String}

        pub var numberMinted: UInt32

        pub var NFTsCollection: @LeofyNFT.Collection

        pub var price: UFix64

        init(metadata: {String: String}, price: UFix64) {
            pre {
                metadata.length != 0: "New Item metadata cannot be empty"
            }
            self.itemID = LeofyNFT.totalItemSupply
            self.metadata = metadata
            self.price = price
            self.numberMinted = 0
            self.NFTsCollection <- create Collection()
        }

        pub fun mintNFT() {
            // create a new NFT
            var newNFT <- create NFT(
                id: LeofyNFT.totalSupply,
                itemID: self.itemID,
                serialNumber: self.numberMinted + 1
            )

            // deposit it in the recipient's account using their reference
            self.NFTsCollection.deposit(token: <-newNFT)

            emit Minted(id: LeofyNFT.totalSupply, itemID: self.itemID, serialNumber: self.numberMinted + 1)

            self.numberMinted =  self.numberMinted + 1
            LeofyNFT.totalSupply = LeofyNFT.totalSupply + 1
        }

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }

        pub fun batchMintNFT(quantity: UInt64){
            var i: UInt64 = 0
            while i < quantity {
                self.mintNFT()
                i = i + 1;
            }
        }

        pub fun setPrice(price: UFix64) {
            self.price = price
        }

        pub fun borrowCollection(): &LeofyNFT.Collection{LeofyCollectionPublic} {
            return &self.NFTsCollection as &Collection{LeofyCollectionPublic}
        }

        pub fun purchase(payment: @FungibleToken.Vault): @NonFungibleToken.NFT {
            pre {
                self.NFTsCollection.getIDs().length > 0: "listing has already been purchased"
                payment.isInstance(Type<@LeofyCoin.Vault>()): "payment vault is not requested fungible token"
                payment.balance == self.price: "payment vault does not contain requested price"
            }

            let nft <- self.NFTsCollection.withdraw(withdrawID: self.NFTsCollection.getIDs()[0])
            let vault = LeofyNFT.getLeofyCoinVault()
            vault.deposit(from: <- payment)
            
            return <- nft
        }

        destroy() {
            destroy self.NFTsCollection
        }
    }

    // This is an implementation of a custom metadata view for Leofy.
    // This view contains the Item metadata.
    //
    pub struct LeofyNFTMetadataView {
        pub let author: String
        pub let name: String
        pub let description: String
        pub let thumbnail: String
        pub let itemID: UInt64
        pub let serialNumber: UInt32

        init(
            author: String,
            name: String,
            description: String,
            thumbnail: AnyStruct{MetadataViews.File},
            itemID: UInt64,
            serialNumber: UInt32
        ){
            self.author = author
            self.name = name
            self.description = description
            self.thumbnail = thumbnail.uri()
            self.itemID = itemID
            self.serialNumber = serialNumber
        }
    }

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver  {
        pub let id: UInt64
        pub let itemID: UInt64
        pub let serialNumber: UInt32

        init(
            id: UInt64,
            itemID: UInt64,
            serialNumber: UInt32
        ) {
            self.id = id
            self.itemID = itemID
            self.serialNumber = serialNumber
        }

        pub fun description(): String {
            let itemCollection = LeofyNFT.getItemCollectionPublic()
            return "NFT: '"
                .concat(itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "name") ?? "''")
                .concat("' from Author: '")
                .concat(itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "author") ?? "''")
                .concat("' with serial number ")
                .concat(self.serialNumber.toString())
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<LeofyNFTMetadataView>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            let itemCollection = LeofyNFT.getItemCollectionPublic()
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "name") ?? "",
                        description: self.description(),
                        thumbnail: MetadataViews.HTTPFile(itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "thumbnail") ?? "")
                    )
                case Type<LeofyNFTMetadataView>():
                    return LeofyNFTMetadataView(
                        author: itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "author") ?? "", 
                        name: itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "name") ?? "",
                        description: self.description(),
                        thumbnail: MetadataViews.HTTPFile(itemCollection.getItemMetaDataByField(itemID: self.itemID, field: "thumbnail") ?? ""),
                        itemID: self.itemID,
                        serialNumber: self.serialNumber
                    )
            }

            return nil
        }
    }

    pub resource interface LeofyCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowLeofyNFT(id: UInt64): &LeofyNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow LeofyNFT reference: the ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: LeofyCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT: ".concat(withdrawID.toString()))

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @LeofyNFT.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowLeofyNFT(id: UInt64): &LeofyNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &LeofyNFT.NFT
            }

            return nil
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return nft as! &LeofyNFT.NFT
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // LeofyNFT contract-level function definitions
    // -----------------------------------------------------------------------

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @LeofyNFT.Collection {
        return <- create Collection()
    }

    pub fun getItemCollectionPublic(): &AnyResource{LeofyNFT.ItemCollectionPublic} {
        return self.account.getCapability(LeofyNFT.ItemPublicPath)
        .borrow<&{LeofyNFT.ItemCollectionPublic}>()
        ?? panic("Could not borrow capability from public Item Collection")
    }

    pub fun getLeofyCoinVault(): &AnyResource{FungibleToken.Receiver} {
        return self.account.getCapability(LeofyCoin.ReceiverPublicPath)!.borrow<&{FungibleToken.Receiver}>()
			?? panic("Could not borrow receiver reference to the recipient's Vault")
    } 

    // -----------------------------------------------------------------------
    // LeofyNFT initialization function
    // -----------------------------------------------------------------------

    init() {
        self.ItemStoragePath = /storage/LeofyItemCollection
        self.ItemPublicPath = /public/LeofyItemCollection

        self.CollectionStoragePath = /storage/LeofyNFTCollection
        self.CollectionPublicPath = /public/LeofyNFTCollection
        self.AdminStoragePath = /storage/LeofyNFTMinter

        // Initialize the total supply
        self.totalSupply = 0
        self.totalItemSupply = 0

        destroy self.account.load<@ItemCollection>(from: self.ItemStoragePath)
        // create a public capability for the Item collection
        self.account.save(<-create ItemCollection(), to: self.ItemStoragePath)
        self.account.link<&LeofyNFT.ItemCollection{LeofyNFT.ItemCollectionPublic}>(
            self.ItemPublicPath,
            target: self.ItemStoragePath
        )

        // Create a Collection resource and save it to storage
        destroy self.account.load<@Collection>(from: self.CollectionStoragePath)

        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&LeofyNFT.Collection{NonFungibleToken.CollectionPublic, LeofyNFT.LeofyCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )
        
        emit ContractInitialized()
    }
}

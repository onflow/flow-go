import NonFungibleToken from 0x1d7e57aa55817448

// TFCItems
// NFT items for TheFootballClub!
pub contract TFCItems: NonFungibleToken {

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, metadata: {String: String})
    pub event Burned(id: UInt64, from: Address?)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let AdminPrivatePath: PrivatePath

    // totalSupply
    // The total number of TFCItems that have been minted
    pub var totalSupply: UInt64

    // NFT
    // A TFC Item as an NFT
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        // The items's type, e.g. 1 == Hat, 2 == Shirt, etc.
        pub let typeID: UInt64
        // String mapping to hold metadata
        access(self) let metadata: {String: String}

        // initializer
        init(initID: UInt64, initTypeID: UInt64, initMetadata: {String : String}) {
            self.id = initID
            self.typeID = initTypeID
            self.metadata = initMetadata
        }

        pub fun getMetadata(): {String: String}{
            return self.metadata
        }

    }

    // This is the interface that users can cast their TFCItems Collection as
    // to allow others to deposit TFCItems into their Collection. It also allows for reading
    // the details of TFCItems in the Collection.
    pub resource interface TFCItemsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun burn(burnID: UInt64)
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTFCItem(id: UInt64): &TFCItems.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow TFCItem reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of TFCItem NFTs owned by an account
    pub resource Collection: TFCItemsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun burn(burnID: UInt64){
             let token <- self.ownedNFTs.remove(key: burnID) ?? panic("missing NFT")
             destroy token
             emit Burned(id: burnID, from: self.owner?.address)
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @TFCItems.NFT
            let id: UInt64 = token.id
            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowTFCItem
        // Gets a reference to an NFT in the collection as a TFCItem,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the TFCItem.
        pub fun borrowTFCItem(id: UInt64): &TFCItems.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TFCItems.NFT
            } else {
                return nil
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Administrator
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    pub resource Administrator {

        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, metadata: {String : String}) {
            emit Minted(id: TFCItems.totalSupply, metadata: metadata)
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create TFCItems.NFT(initID: TFCItems.totalSupply, initTypeID: typeID, metadata: metadata))
            TFCItems.totalSupply = TFCItems.totalSupply + (1 as UInt64)
        }
    }

    // fetch
    // Get a reference to a TFCItem from an account's Collection, if available.
    // If an account does not have a TFCItems.Collection, panic.
    // If it has a collection but does not contain the itemId, return nil.
    // If it has a collection and that collection contains the itemId, return a reference to that.
    pub fun fetch(_ from: Address, itemID: UInt64): &TFCItems.NFT? {
        let collection = getAccount(from)
            .getCapability(TFCItems.CollectionPublicPath)!
            .borrow<&TFCItems.Collection{TFCItems.TFCItemsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust TFCItems.Collection.borowTFCItem to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowTFCItem(id: itemID)
    }


    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/TFCItemsCollection
        self.CollectionPublicPath = /public/TFCItemsCollection
        self.AdminStoragePath = /storage/TFCItemsMinter
        self.AdminPrivatePath=/private/TFCItemsAdminPrivate

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Admin resource and save it to storage
        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
        self.account.link<&Administrator>(self.AdminPrivatePath, target: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract Items: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, metadata:{String:String})

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let MinterPublicPath: PublicPath

    // totalSupply
    // The total number of Items that have been minted
    //
    pub var totalSupply: UInt64
   
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {

        pub let id: UInt64

        access(self) let metadata: {String:String}

        init(id: UInt64, metadata: {String:String}) {
            self.id = id
            self.metadata = metadata
        }

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }

        pub fun getAttribute(key:String): String {
            return self.metadata[key]!
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.metadata["title"]!,
                        description: self.metadata["description"]!,
                        thumbnail: MetadataViews.HTTPFile(
                            self.metadata["image"]!!
                        )
                    )
            }

            return nil
        }
    }

    // This is the interface that users can cast their Items Collection as
    // to allow others to deposit Items into their Collection. It also allows for reading
    // the details of Items in the Collection.
    pub resource interface ItemsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItem(id: UInt64): &Items.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Item reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Item NFTs owned by an account
    //
    pub resource Collection: ItemsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Items.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowItem
        // Gets a reference to an NFT in the collection as a Item,
        // This is safe as there are no functions that can be called on the Item.
        //
        pub fun borrowItem(id: UInt64): &Items.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Items.NFT
            } else {
                return nil
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }
    
    // createNFTMinter
    // public function that anyone can call to create a new NFTMinter
    //
    pub fun createNFTMinter(): @NFTMinter {
        return <- create NFTMinter()
    }

    // This is the interface that users can cast their NFTMinter as to enable minting NFTs
    pub resource interface NFTMinterPublic {
        pub fun getMintedNFT(): UInt64
    }

    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter: NFTMinterPublic {

        access(self) var mintedNFTs: UInt64

        pub fun getMintedNFT(): UInt64 {
            return self.mintedNFTs
        }

        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic}, 
            metadata: {String:String},
        ) {
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create Items.NFT(id: Items.totalSupply, metadata:metadata))

            emit Minted(
                id: Items.totalSupply,
                metadata:metadata,
            )

            Items.totalSupply = Items.totalSupply + 1
            self.mintedNFTs = self.mintedNFTs + 1
        }

        init() {
            self.mintedNFTs = 0
        }
    }

    // fetch
    // Get a reference to a Item from an account's Collection, if available.
    // If an account does not have a Items.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &Items.NFT? {
        let collection = getAccount(from)
            .getCapability(Items.CollectionPublicPath)!
            .borrow<&Items.Collection{Items.ItemsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Items.Collection.borowItem to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowItem(id: itemID)
    }

    // initializer
    //
    init() {
        // set rarity price mapping

        // Set our named paths
        self.CollectionStoragePath = /storage/ItemsCollectionV11
        self.CollectionPublicPath = /public/ItemsCollectionV11
        self.MinterStoragePath = /storage/ItemsMinterV11
        self.MinterPublicPath = /public/ItemsMinterV11

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)
        self.account.unlink(self.MinterPublicPath)
        self.account.link<&Items.NFTMinter{Items.NFTMinterPublic}>(self.MinterPublicPath, target: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

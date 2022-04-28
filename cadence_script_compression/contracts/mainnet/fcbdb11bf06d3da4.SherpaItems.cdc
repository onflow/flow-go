import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract SherpaItems: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, kind: UInt8, rarity: UInt8)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // totalSupply
    // The total number of SherpaItems that have been minted
    //
    pub var totalSupply: UInt64

    pub enum Rarity: UInt8 {
        pub case standard
        pub case special
    }

    pub fun rarityToString(_ rarity: Rarity): String {
        switch rarity {
            case Rarity.standard:
                return "standard"
            case Rarity.special:
                return "special"
        }
        return ""
    }

    pub enum Kind: UInt8 {
        pub case membership
        pub case collectable
    }

    pub fun kindToString(_ kind: Kind): String {
        switch kind {
            case Kind.membership:
                return "Membership"
            case Kind.collectable:
                return "Collectable"
        }
        return ""
    }

    // Mapping from item (kind, rarity) -> IPFS image CID
    //
    access(self) var images: {Kind: {Rarity: String}}

    // Mapping from rarity -> price
    //
    access(self) var itemRarityPriceMap: {Rarity: UFix64}

    // Return the initial sale price for an item of this rarity.
    //
    pub fun getItemPrice(rarity: Rarity): UFix64 {
        return self.itemRarityPriceMap[rarity]!
    }
    
    // A Sherpa Item as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {

        pub let id: UInt64

        // The token kind (e.g. Membership)
        pub let kind: Kind

        // The token rarity (e.g. Standard)
        pub let rarity: Rarity

        init(id: UInt64, kind: Kind, rarity: Rarity) {
            self.id = id
            self.kind = kind
            self.rarity = rarity
        }

        pub fun name(): String {
            return SherpaItems.rarityToString(self.rarity)
                .concat(" ")
                .concat(SherpaItems.kindToString(self.kind))
        }

        pub fun description(): String {
            return "A "
                .concat(SherpaItems.rarityToString(self.rarity).toLower())
                .concat(" ")
                .concat(SherpaItems.kindToString(self.kind).toLower())
                .concat(" with serial number ")
                .concat(self.id.toString())
        }

        pub fun imageCID(): String {
            return SherpaItems.images[self.kind]![self.rarity]!
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
                        name: "Sherpa Tea Club Membership",
                        description: self.description(),
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.imageCID(), 
                            path: "sm.png"
                        )
                    )
            }

            return nil
        }
    }

    // This is the interface that users can cast their SherpaItems Collection as
    // to allow others to deposit SherpaItems into their Collection. It also allows for reading
    // the details of SherpaItems in the Collection.
    pub resource interface SherpaItemsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowSherpaItem(id: UInt64): &SherpaItems.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow SherpaItem reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of SherpaItem NFTs owned by an account
    //
    pub resource Collection: SherpaItemsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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
            let token <- token as! @SherpaItems.NFT

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

        // borrowSherpaItem
        // Gets a reference to an NFT in the collection as a SherpaItem,
        // exposing all of its fields (including the typeID & rarityID).
        // This is safe as there are no functions that can be called on the SherpaItem.
        //
        pub fun borrowSherpaItem(id: UInt64): &SherpaItems.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SherpaItems.NFT
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

    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter {

        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        //
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic}, 
            kind: Kind, 
            rarity: Rarity,
        ) {
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create SherpaItems.NFT(id: SherpaItems.totalSupply, kind: kind, rarity: rarity))

            emit Minted(
                id: SherpaItems.totalSupply,
                kind: kind.rawValue,
                rarity: rarity.rawValue,
            )

            SherpaItems.totalSupply = SherpaItems.totalSupply + (1 as UInt64)
        }
    }

    // fetch
    // Get a reference to a SherpaItem from an account's Collection, if available.
    // If an account does not have a SherpaItems.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &SherpaItems.NFT? {
        let collection = getAccount(from)
            .getCapability(SherpaItems.CollectionPublicPath)!
            .borrow<&SherpaItems.Collection{SherpaItems.SherpaItemsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust SherpaItems.Collection.borowSherpaItem to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowSherpaItem(id: itemID)
    }

    // initializer
    //
    init() {
        // set rarity price mapping
        self.itemRarityPriceMap = {
            Rarity.standard: 115.0,
            Rarity.special: 250.0
        }

        self.images = {
            Kind.membership: {
                Rarity.standard: "QmTrVcndMLgX8ybTbEd4D1yUPE5WA2wVuMxqthUEHn3xXw",
                Rarity.special: "QmTrVcndMLgX8ybTbEd4D1yUPE5WA2wVuMxqthUEHn3xXw"
            },
            Kind.collectable: {
                Rarity.standard: "QmTrVcndMLgX8ybTbEd4D1yUPE5WA2wVuMxqthUEHn3xXw",
                Rarity.special: "QmTrVcndMLgX8ybTbEd4D1yUPE5WA2wVuMxqthUEHn3xXw"
            }
        }

        // Set our named paths
        self.CollectionStoragePath = /storage/sherpaItemsCollectionV10
        self.CollectionPublicPath = /public/sherpaItemsCollectionV10
        self.MinterStoragePath = /storage/sherpaItemsMinterV10

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

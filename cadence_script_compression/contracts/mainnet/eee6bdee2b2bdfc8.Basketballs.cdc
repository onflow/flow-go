import NonFungibleToken from 0x1d7e57aa55817448

// Basketballs
// NFT basketballs!
//
pub contract Basketballs: NonFungibleToken {

    // Events
    pub event ContractInitialized()
    pub event EditionCreated(editionID: UInt32, name: String, description: String, imageURL: String)
    pub event BasketballMinted(id: UInt64, editionID: UInt32, serialNumber: UInt64)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminStoragePath: StoragePath

    pub var totalSupply: UInt64
    pub var nextEditionID: UInt32
    access(self) var editions: {UInt32: Edition}

    pub struct EditionMetadata {
        pub let editionID: UInt32
        pub let name: String
        pub let description: String
        pub let imageURL: String
        pub let circulatingCount: UInt64

        init(editionID: UInt32, name: String, description: String, imageURL: String, circulatingCount: UInt64) {
            self.editionID = editionID
            self.name = name
            self.description = description
            self.imageURL = imageURL
            self.circulatingCount = circulatingCount
        }
    }

    pub struct Edition {
       pub let editionID: UInt32
       pub let name: String
       pub let description: String
       pub let imageURL: String
       access(account) var nextSerialInEdition: UInt64

        init(name: String, description: String, imageURL: String) {
            self.editionID = Basketballs.nextEditionID
            self.name = name
            self.description = description
            self.imageURL = imageURL
            self.nextSerialInEdition = 1

            Basketballs.nextEditionID = Basketballs.nextEditionID + (1 as UInt32)

            emit EditionCreated(editionID: self.editionID, name: self.name, description: self.description, imageURL: self.imageURL)
        }

        pub fun mintBasketball(): @NFT {
            let basketball: @NFT <- create NFT(editionID: self.editionID, serialNumber: self.nextSerialInEdition)

            self.nextSerialInEdition = self.nextSerialInEdition + (1 as UInt64)

            Basketballs.editions[self.editionID] = self

            return <-basketball
        }

        pub fun mintBasketballs(quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <- self.mintBasketball())
                i = i + (1 as UInt64)
            }

            return <- newCollection
        }
    }

    // NFT
    // A Basketball as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let editionID: UInt32
        pub let serialNumber: UInt64

        init(editionID: UInt32, serialNumber: UInt64) {
            Basketballs.totalSupply = Basketballs.totalSupply + (1 as UInt64)
            self.id = Basketballs.totalSupply
            self.editionID = editionID
            self.serialNumber = serialNumber

            emit BasketballMinted(id: self.id, editionID: self.editionID, serialNumber: self.serialNumber)
        }
    }

    // This is the interface that users can cast their Basketballs Collection as
    // to allow others to deposit Basketballs into their Collection. It also allows for reading
    // the details of Basketballs in the Collection.
    pub resource interface BasketballsCollectionPublic {
        pub fun borrowBasketball(id: UInt64): &Basketballs.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Basketball reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Basketball NFTs owned by an account
    //
    pub resource Collection: BasketballsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an UInt64 ID field
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
            let token <- token as! @Basketballs.NFT

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

        // borrowBasketball
        // Gets a reference to an NFT in the collection as a Basketball,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the Basketball.
        //
        pub fun borrowBasketball(id: UInt64): &Basketballs.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Basketballs.NFT
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

    pub resource Admin {
        
        pub fun createEdition(name: String, description: String, imageURL: String): UInt32 {
            let edition = Edition(name: name, description: description, imageURL: imageURL)

            Basketballs.editions[edition.editionID] = edition

            return edition.editionID
        }

        pub fun mintBasketball(editionID: UInt32): @NFT {
            pre {
                Basketballs.editions[editionID] != nil: "Mint failed: Edition does not exist"
            }

            let edition: Edition = Basketballs.editions[editionID]!;

            let basketball: @NFT <- edition.mintBasketball()

            return <-basketball
        }

        pub fun mintBasketballs(editionID: UInt32, quantity: UInt64): @Collection {
            pre {
                Basketballs.editions[editionID] != nil: "Mint failed: Edition does not exist"
            }

            let edition: Edition = Basketballs.editions[editionID]!;

            let collection: @Collection <- edition.mintBasketballs(quantity: quantity)

            return <-collection
        }

        pub fun createNewAdmin(): @Admin {
            return <- create Admin()
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // fetch
    // Get a reference to a Basketball from an account's Collection, if available.
    // If an account does not have a Basketballs.Collection, panic.
    // If it has a collection but does not contain the itemId, return nil.
    // If it has a collection and that collection contains the itemId, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &Basketballs.NFT? {
        let collection = getAccount(from)
            .getCapability(Basketballs.CollectionPublicPath)
            .borrow<&Basketballs.Collection{Basketballs.BasketballsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Basketballs.Collection.borowBasketball to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowBasketball(id: itemID)
    }

     pub fun getAllEditions(): [Edition] {
        return self.editions.values
    }

    pub fun getEditionMetadata(editionID: UInt32): EditionMetadata {
        let edition = self.editions[editionID]!
        let metadata = EditionMetadata(editionID: edition.editionID, name: edition.name, description: edition.description, imageURL: edition.imageURL, circulatingCount: edition.nextSerialInEdition - (1 as UInt64))
        return metadata
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/BallmartBasketballsCollection
        self.CollectionPublicPath = /public/BallmartBasketballsCollection
        self.CollectionPrivatePath = /private/BallmartBasketballsCollection
        self.AdminStoragePath = /storage/BallmartBasketballsAdmin

        // Initialize the total supply
        self.totalSupply = 0
        self.editions = {}
        self.nextEditionID = 1

        self.account.save(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
	}
}
//SPDX-License-Identifier: MIT License

import NonFungibleToken from 0x1d7e57aa55817448

pub contract BlueBeltNFTs: NonFungibleToken {
  
    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, typeID: UInt64)
    pub event TokensBurned(id: UInt64)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let BurnerStoragePath: StoragePath

    // totalSupply
    // The total number of BlueBeltNFTs that have been minted
    //
    pub var totalSupply: UInt64

    // Requirement that all conforming NFT smart contracts have
    // to define a resource called NFT that conforms to INFT


    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        // The token's type, e.g. 3 == Hat
        pub let typeID: UInt64

        // String mapping to hold metadata
        access(self) var metadata: {String: String} 
        // String mapping to hold metadata
        pub var urlData: String

        // initializer
        //
        init(initID: UInt64, initTypeID: UInt64, urlData: String, metadata: {String: String}?) {
            self.id = initID
            self.typeID = initTypeID
            self.urlData = urlData
            self.metadata = metadata ?? {}
        }

        pub fun getMetadata(): {String:String} {
	        return self.metadata
        }  

        destroy(){
	        emit TokensBurned(id: self.id)
        }


    }

    // This is the interface that users can cast their BlueBeltNFTs Collection as
    // to allow others to deposit BlueBeltNFTs into their Collection. It also allows for reading
    // the details of BlueBeltNFTs in the Collection.
    pub resource interface BlueBeltNFTsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowBlueBelt(id: UInt64): &BlueBeltNFTs.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow BlueBelt reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Requirement for the the concrete resource type
    // to be declared in the implementing contract
    //
    pub resource Collection: BlueBeltNFTsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        // Dictionary to hold the NFTs in the Collection
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
            let token <- token as! @BlueBeltNFTs.NFT

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

        // borrowBlueBelt
        // Gets a reference to an NFT in the collection as a BlueBelt,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the BlueBelt.
        //
        pub fun borrowBlueBelt(id: UInt64): &BlueBeltNFTs.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &BlueBeltNFTs.NFT
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

    // createEmptyCollection creates an empty Collection
    // and returns it to the caller so that they can own NFTs
    pub fun createEmptyCollection(): @BlueBeltNFTs.Collection {
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
        pub fun mintNFT(recipient: &{BlueBeltNFTs.BlueBeltNFTsCollectionPublic}, typeID: UInt64, urlData: String, metadata: {String: String}) {
            // deposit it in the recipient's account using their reference
            BlueBeltNFTs.totalSupply = BlueBeltNFTs.totalSupply + (1 as UInt64)
            recipient.deposit(token: <-create BlueBeltNFTs.NFT(initID: BlueBeltNFTs.totalSupply, initTypeID: typeID, urlData: urlData, metadata: metadata))
            emit Minted(id: BlueBeltNFTs.totalSupply, typeID: typeID)
        }

    }

     // fetch
        // Get a reference to a BluebeltNFT from an account's Collection, if available.
        // If an account does not have a BluebeltNFTs.Collection, panic.
        // If it has a collection but does not contain the itemID, return nil.
        // If it has a collection and that collection contains the itemID, return a reference to that.
        //
        pub fun fetch(_ from: Address, itemID: UInt64): &BlueBeltNFTs.NFT? {
            let collection = getAccount(from)
                .getCapability(BlueBeltNFTs.CollectionPublicPath)
                .borrow<&BlueBeltNFTs.Collection{BlueBeltNFTs.BlueBeltNFTsCollectionPublic}>()
                ?? panic("Couldn't get collection")
            // We trust BluebeltNFTs.Collection.borowBluebelt to get the correct itemID
            // (it checks it before returning it).
            return collection.borrowBlueBelt(id: itemID)
        }

    pub resource NFTBurner {
        pub fun burn(token: @NonFungibleToken.NFT) {
            let token <- token as! @BlueBeltNFTs.NFT
            let id: UInt64 = token.id

            destroy token

            emit TokensBurned(id: id)
        }
    }
    // initializer
    //
    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/BlueBeltNFTsCollection
        self.CollectionPublicPath = /public/BlueBeltNFTsPublicCollection
        self.MinterStoragePath = /storage/BlueBeltNFTsMinter
        self.BurnerStoragePath = /storage/BlueBeltNFTsBurner

        // Initialize the total supply
        self.totalSupply = 0
        // store an empty NFT Collection in account storage
        self.account.save(<-self.createEmptyCollection(), to: self.CollectionStoragePath)

        // publish a reference to the Collection in storage
        self.account.link<&BlueBeltNFTs.Collection{NonFungibleToken.CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, BlueBeltNFTs.BlueBeltNFTsCollectionPublic}>(BlueBeltNFTs.CollectionPublicPath, target: self.CollectionStoragePath)
        self.account.link<&{BlueBeltNFTs.BlueBeltNFTsCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        let burner <- create NFTBurner()

        self.account.save(<-minter, to: self.MinterStoragePath)
        self.account.save(<-burner, to: self.BurnerStoragePath)

        emit ContractInitialized()
    }
}

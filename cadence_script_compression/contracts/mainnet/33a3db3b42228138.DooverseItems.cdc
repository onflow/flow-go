/**
 This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
**/
import NonFungibleToken from 0x1d7e57aa55817448

// DooverseItems
// NFT items for Dooverse!
//
pub contract DooverseItems: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, initMeta: {String: String})
    pub event TrxMeta(trxMeta: {String: String})

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // totalSupply
    // The total number of DooverseItems that have been minted
    //
    pub var totalSupply: UInt64

    // NFT
    // A Dooverse Item as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
       // The token's ID
        pub let id: UInt64
        // The token's metadata in dict format
        access(self) let metadata: {String: String}
        
        // initializer
        //
        init(initID: UInt64, initMeta: {String: String}) {
            self.id = initID
            self.metadata = initMeta
        }

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }
    }

    // This is the interface that users can cast their DooverseItems Collection as
    // to allow others to deposit DooverseItems into their Collection. It also allows for reading
    // the details of DooverseItems in the Collection.
    pub resource interface DooverseItemsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowDooverseItem(id: UInt64): &DooverseItems.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow DooverseItem reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of DooverseItem NFTs owned by an account
    //
    pub resource Collection: DooverseItemsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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

        pub fun withdrawWithMetadata(withdrawNFTID: UInt64,trxMetadata:{String:String}): @NonFungibleToken.NFT{  
            emit TrxMeta(trxMeta:trxMetadata)        
            let nft <-self.withdraw(withdrawID: withdrawNFTID)
            return <-nft
        }
        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @DooverseItems.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }


        pub fun depositWithMetadata(depositToken: @NonFungibleToken.NFT,trxMetadata:{String:String}){  
            emit TrxMeta(trxMeta:trxMetadata)        
            self.deposit(token:<-depositToken)
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

        // borrowDooverseItem
        // Gets a reference to an NFT in the collection as a DooverseItem,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the DooverseItem.
        //
        pub fun borrowDooverseItem(id: UInt64): &DooverseItems.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &DooverseItems.NFT
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
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, initMetadata: {String: String}) {
            emit Minted(id: DooverseItems.totalSupply, initMeta: initMetadata)

			// deposit it in the recipient's account using their reference
			recipient.deposit(token: <-create DooverseItems.NFT(initID: DooverseItems.totalSupply, initMeta: initMetadata))

            DooverseItems.totalSupply = DooverseItems.totalSupply + (1 as UInt64)
		}
	}

    // fetch
    // Get a reference to a DooverseItem from an account's Collection, if available.
    // If an account does not have a DooverseItems.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &DooverseItems.NFT? {
        let collection = getAccount(from)
            .getCapability(DooverseItems.CollectionPublicPath)!
            .borrow<&DooverseItems.Collection{DooverseItems.DooverseItemsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust DooverseItems.Collection.borowDooverseItem to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowDooverseItem(id: itemID)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/DooverseItemsCollection
        self.CollectionPublicPath = /public/DooverseItemsCollection
        self.MinterStoragePath = /storage/DooverseItemsMinter

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
	}
}

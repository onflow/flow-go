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

// CryptoPiggo
// NFT items for Crypto Piggos!
//
pub contract CryptoPiggo: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, initMeta: {String: String})

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // totalSupply
    // The total number of CryptoPiggo that have been minted
    //
    pub var totalSupply: UInt64

    // maxSupply
    // The maximum number of CryptoPiggo that can be minted
    //
    pub let maxSupply: UInt64

    // idToAddress
    // Maps each item ID to its current owner
    //
    access(self) let idToAddress: [Address]

    // NFT
    // A Crypto Piggo as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64

        // The token's metadata in dict format
        access(self) let metadata: {String: String}

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }

        // initializer
        //
        init(initID: UInt64, initMeta: {String: String}) {
            self.id = initID
            self.metadata = initMeta
        }
    }

    // This is the interface that users can cast their CryptoPiggo Collection as
    // to allow others to deposit CryptoPiggo into their Collection. It also allows for reading
    // the details of CryptoPiggo in the Collection.
    pub resource interface CryptoPiggoCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun getLength(): Int
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItem(id: UInt64): &CryptoPiggo.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of CryptoPiggo NFTs owned by an account
    //
    pub resource Collection: CryptoPiggoCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an 'UInt64' ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner!.address)

            return <-token
        }

        // deposit
        // Takes an NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @CryptoPiggo.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            // update owner
            CryptoPiggo.idToAddress[id] = self.owner!.address

            emit Deposit(id: id, to: self.owner!.address)

            destroy oldToken
        }
        
        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // getLength
        // Returns the length of the collection
        //
        pub fun getLength(): Int {
            return self.ownedNFTs.length
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection using its ID
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowItem
        // Gets a reference to an NFT in the collection as a CryptoPiggo,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the CryptoPiggo.
        //
        pub fun borrowItem(id: UInt64): &CryptoPiggo.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &CryptoPiggo.NFT
            } else {
                return nil
            }
        }

        // destructor
        //
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
		pub fun mintNFT(recipient: Address, initMetadata: {String: String}) {
            let nftID = CryptoPiggo.totalSupply
            if nftID < CryptoPiggo.maxSupply {
                let receiver = getAccount(recipient)
                    .getCapability(CryptoPiggo.CollectionPublicPath)!
                    .borrow<&{NonFungibleToken.CollectionPublic}>()
                    ?? panic("Could not get receiver reference to the NFT Collection")
                emit Minted(id: nftID, initMeta: initMetadata)
                CryptoPiggo.idToAddress.append(recipient)
                CryptoPiggo.totalSupply = nftID + (1 as UInt64)
                receiver.deposit(token: <-create CryptoPiggo.NFT(initID: nftID, initMeta: initMetadata))
            } else {
                panic("No more piggos can be minted")
            }
		}
	}

    // getOwner
    // Gets the current owner of the given item
    //
    pub fun getOwner(itemID: UInt64): Address? {
        if itemID >= 0 && itemID < self.maxSupply {
            if (itemID < CryptoPiggo.totalSupply) {
                return CryptoPiggo.idToAddress[itemID]
            } else {
                return nil
            }
        }
        return nil
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/CryptoPiggoCollection
        self.CollectionPublicPath = /public/CryptoPiggoCollection
        self.MinterStoragePath = /storage/CryptoPiggoMinter

        // Initialize the total supply
        self.totalSupply = 0

        // Initialize the max supply
        self.maxSupply = 10000

        // Initalize mapping from ID to address
        self.idToAddress = []

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
	}
}
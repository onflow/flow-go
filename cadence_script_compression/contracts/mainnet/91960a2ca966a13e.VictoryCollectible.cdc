// NonFungibleToken - MAINNET
import NonFungibleToken from 0x1d7e57aa55817448

// VictoryCollectible
// NFT items for Victory Collection
//
pub contract VictoryCollectible: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, typeID: UInt64, brandID: UInt64, seriesID: UInt64, dropID: UInt64)
    pub event BundleCreated(owner: Address, id: UInt64)
    pub event BundleRemoved(owner: Address, id: UInt64)
    pub event AllBundlesRemoved(owner: Address)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // totalSupply
    // The total number of VictoryCollectible that have been minted
    //
    pub var totalSupply: UInt64

    // NFT
    // A Victory Collectible as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // Original owner (for revenue sharing)
        pub let originalOwner: Address
        // The token's ID
        pub let id: UInt64
        // The token's type, e.g. 0 == Art
        pub let typeID: UInt64
        // The token's brand ID, e.g. 0 == Victory
        pub let brandID: UInt64
        // The token's series ID, e.g. 0 == Series 1
        pub let seriesID: UInt64
        // The token's drop ID, e.g. 0 == None
        pub let dropID: UInt64
        // The token's issue number
        pub let issueNum: UInt32
        // How many tokens were issued
        pub let maxIssueNum: UInt32
        // The token's content hash
        pub let contentHash: UInt256

        // initializer
        //
        init(initOwner: Address, initID: UInt64, 
                initTypeID: UInt64, initBrandID: UInt64, initSeriesID: UInt64, initDropID: UInt64,
                initHash: UInt256, 
                initIssueNum: UInt32, initMaxIssueNum: UInt32) {
            self.originalOwner = initOwner
            self.id = initID
            self.typeID = initTypeID
            self.brandID = initBrandID
            self.seriesID = initSeriesID
            self.dropID = initDropID
            self.contentHash = initHash
            self.issueNum = initIssueNum
            self.maxIssueNum = initMaxIssueNum
        }

        pub set 
    }

    // This is the interface that users can cast their VictoryCollectible Collection as
    // to allow others to deposit VictoryCollectible into their Collection. It also allows for reading
    // the details of VictoryCollectible in the Collection.
    pub resource interface VictoryCollectibleCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun getBundleIDs(bundleID: UInt64): [UInt64]
        pub fun getNextBundleID(): UInt64
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun isNFTForSale(id: UInt64): Bool
        pub fun borrowVictoryItem(id: UInt64): &VictoryCollectible.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow VictoryItem reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // The interface that users can use to manage their own bundles for sale
    pub resource interface VictoryCollectibleBundle {
        pub fun createBundle(itemIDs: [UInt64]): UInt64
        pub fun removeBundle(bundleID: UInt64)
        pub fun removeAllBundles()
    }

    // Collection
    // A collection of Victory Collection NFTs owned by an account
    //
    pub resource Collection: VictoryCollectibleCollectionPublic, VictoryCollectibleBundle, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // dictionary of bundles for sale
        access(contract) var bundles: {UInt64: [UInt64]}

        // ID for next bundle
        pub var nextBundleID: UInt64

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            // also delete any bundle that this item was part of
            for key in self.bundles.keys {
                if self.bundles[key]!.contains(withdrawID) {
                    self.bundles.remove(key: key)
                    emit BundleRemoved(owner: self.owner?.address!, id: key)
                    break
                }
            }

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes an NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @VictoryCollectible.NFT

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

        // getBundleIDs
        // Returns an array of the IDs that are in the specified bundle
        //
        pub fun getBundleIDs(bundleID: UInt64): [UInt64] {
            return self.bundles[bundleID] ?? panic("Bundle does not exist")
        }

        // getNextBundleID
        // Returns the next bundle ID
        //
        pub fun getNextBundleID(): UInt64 {
            return self.nextBundleID
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // isNFTForSale
        // Gets a reference to an NFT in the collection as a VictoryItem,
        pub fun isNFTForSale(id: UInt64): Bool {
            for key in self.bundles.keys {
                if self.bundles[key]!.contains(id) {
                    return true
                }
            }
            return false
        }

        // borrowVictoryItem
        // Gets a reference to an NFT in the collection as a VictoryItem,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the VictoryItem.
        //
        pub fun borrowVictoryItem(id: UInt64): &VictoryCollectible.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &VictoryCollectible.NFT
            } else {
                return nil
            }
        }

        // createBundle
        // Creates a new array of itemIDs which can be used to list multiple items
        // for sale as one bundle
        // Returns the id of the bundle
        pub fun createBundle(itemIDs: [UInt64]): UInt64 {
            var bundle:[UInt64] = []
            for id in itemIDs {
                if (self.isNFTForSale(id: id)) {
                    panic("Item is already part of a bundle!")
                }
                bundle.append(id)
            }
            let id = self.nextBundleID
            self.bundles[id] = bundle
            self.nextBundleID = self.nextBundleID + (1 as UInt64)
            emit BundleCreated(owner: self.owner?.address!, id: id)
            return id
        }

        // removeBundle
        // Removes the specified bundle from the dictionary
        pub fun removeBundle(bundleID: UInt64) {
            for key in self.bundles.keys {
                if key == bundleID {
                    self.bundles.remove(key: bundleID)
                    emit BundleRemoved(owner: self.owner?.address!, id: bundleID)
                    return
                }
            }
            panic("Bundle does not exist")
        }

        // removeAllBundles
        // Removes all bundles from the dictionary - use with caution!
        // Note: nextBundleID is *not* set to 0 to avoid unexpected collision
        pub fun removeAllBundles() {
            self.bundles = {}
            emit AllBundlesRemoved(owner: self.owner?.address!)
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
            self.bundles = {}
            self.nextBundleID = 0
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
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, 
                        owner: Address,
                        typeID: UInt64, 
                        brandID: UInt64, 
                        seriesID: UInt64,
                        dropID: UInt64,
                        contentHash: UInt256, 
                        startIssueNum: UInt32, 
                        maxIssueNum: UInt32, 
                        totalIssueNum: UInt32) 
        {
            var i: UInt32 = 0;

            while i < maxIssueNum {
                emit Minted(id: VictoryCollectible.totalSupply, typeID: typeID, brandID: brandID, seriesID: seriesID, dropID: dropID)

                // deposit it in the recipient's account using their reference
                recipient.deposit(token: <-create VictoryCollectible.NFT(initOwner:owner,
                                                                initID: VictoryCollectible.totalSupply, 
                                                                initTypeID: typeID, 
                                                                initBrandID: brandID,
                                                                initSeriesID: seriesID,
                                                                initDropID: dropID,
                                                                initHash:contentHash,
                                                                initIssueNum:i + startIssueNum, 
                                                                initMaxIssueNum: totalIssueNum))

                VictoryCollectible.totalSupply = VictoryCollectible.totalSupply + (1 as UInt64)

                i = i + (1 as UInt32)
            }
		}
	}

    // fetch
    // Get a reference to a VictoryItem from an account's Collection, if available.
    // If an account does not have a VictoryCollectible.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &VictoryCollectible.NFT? {
        let collection = getAccount(from)
            .getCapability(VictoryCollectible.CollectionPublicPath)!
            .borrow<&VictoryCollectible.Collection{VictoryCollectible.VictoryCollectibleCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust VictoryCollectible.Collection.borowVictoryItem to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowVictoryItem(id: itemID)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/VictoryCollectibleCollection
        self.CollectionPublicPath = /public/VictoryCollectibleCollection
        self.MinterStoragePath = /storage/VictoryCollectibleMinter

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
	}
}

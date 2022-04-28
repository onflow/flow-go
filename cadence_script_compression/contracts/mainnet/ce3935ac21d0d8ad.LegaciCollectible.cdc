/*
    Description: Central Smart Contract for Legaci Collectibles

    authors: Aman Azad aman@legaci.shop

    This smart contract contains the core functionality for 
    Legaci Collecibles, created by Legaci, Inc.

    The contract manages the data associated with all the brands, collections,
    and units that are used as templates for the Collectible NFTs

    When a brand wants a new Legaci Collectible to be added to the records, 
    an Admin mints new Collectibles off of the Legaci brand ID, collection ID,
    and unit ID that are passed in.
    
    When Collectibles are minted, they are initialized with a LegaciCollectibleData struct and
    are returned by the minter.

    IMPORTANT NOTE: The contract also defines a Collection resource. NOTE Legaci organizes units
    into the following hiearchy Units -> Item -> Collection -> Brand. This 
    collection is different from the collection used to store Legaci Collectibles. 
    This collection is an object that every Legaci Collectible owner will store in their account
    to manage their Legaci collection.

    The main Legaci Collectibles account will have its own collection
    it will use to hold its own collectibles that have not yet been sent to a user.

    Note: All state changing functions will panic if an invalid argument is
    provided or one of its pre-conditions or post conditions aren't met.
    Functions that don't modify state will simply return 0 or nil 
    and those cases need to be handled by the caller.

*/

/*
    NFT Contract defined at these addresses:
    https://docs.onflow.org/core-contracts/non-fungible-token
*/
import NonFungibleToken from 0x1d7e57aa55817448

pub contract LegaciCollectible: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Legaci Collecible contract Events
    // -----------------------------------------------------------------------

    // Emitted when the TopShot contract is created
    pub event ContractInitialized()

    // Emitted when a Legaci Collecible is minted
    pub event LegaciCollectibleMinted(legaciCollectibleId: UInt64, brandId: String, collectionId: String, unitId: String)

    // Events for Collection-related actions
    //
    // Emitted when a moment is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)
    // Emitted when a moment is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Collectible is destroyed
    pub event LegaciCollectibleDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // Legaci Collecible Paths
    // -----------------------------------------------------------------------

    // Path to store the Legaci collectibles collection
    // in a user's account storage
    pub let LegaciCollectibleCollectionStoragePath: StoragePath

    // Path to store the Legaci collectibles collection
    // in a user's public account storage
    pub let LegaciCollectibleCollectionPublicPath: PublicPath

    // Path to admin storage for Legaci collectibles
    // currently serving to store the minter functionality
    pub let LegaciCollectibleAdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // Legaci Collectible contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Variable size dictionary of Set resources
    // Maps Brand UUID String to Brand struct data
    access(self) var brands: @{String: Brand}

    // The total number of Top shot Moment NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date. Also used as global moment IDs for minting.
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // Legaci Collectible contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Brand is a resource type that contains the functions to mint Legaci Collectibles.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The Brand can mint Legaci Collectibles that reference the brand ID, 
    // collection ID, and unit ID defined by Legaci.
    // The Collectibles that are minted by a Brand will be listed as belonging to
    // the Brand that minted it, as well as the Legaci metadata it references.
    pub resource Brand {

        // Unique ID for the set
        pub let brandId: String

        // Number of Collectibles that have been minted by the Brand.
        pub var numberOfMintedCollectibles: UInt64

        init(brandId: String) {
            self.brandId = brandId
            self.numberOfMintedCollectibles = 0
        }

        // mintCollectible mints a new Legaci Collectible and returns the newly minted Collectible
        // 
        // Parameters: 
        //     collectionId: The UUID string of the Legaci Collection ID that the Collectible references
        //     unitId: The UUID string of the Legaci Unit ID that the Collectible references
        //
        // Pre-Conditions:
        // The Brand must be allowed to mint new Collectibles
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintLegaciCollectible(collectionId: String, unitId: String): @NFT {
            // Mint the new moment
            let newLegaciCollectible: @NFT <- create NFT(brandId: self.brandId, collectionId: collectionId, unitId: unitId)

            // Increment the count of Moments minted for this Play
            self.numberOfMintedCollectibles = self.numberOfMintedCollectibles + UInt64(1)

            return <-newLegaciCollectible
        }
    }

    pub struct LegaciCollectibleData {

        // The UUID String ID of the Brand that the Legaci Collectible comes from
        pub let brandId: String

        // The UUID String ID of the Legaci Collection that the Legaci Collectible comes from
        pub let collectionId: String

        // The UUID String ID of the Legaci Unit that the Legaci Collectible comes from
        pub let unitId: String

        init(brandId: String, collectionId: String, unitId: String) {
            self.brandId = brandId
            self.collectionId = collectionId
            self.unitId = unitId
        }

    }

    // The resource that represents the Legaci Collectible NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Legaci Collectible ID
        pub let id: UInt64
        
        // Struct of Legaci Collectible metadata
        pub let data: LegaciCollectibleData

        init(brandId: String, collectionId: String, unitId: String) {
            // Increment the global Legaci Collectibles IDs
            LegaciCollectible.totalSupply = LegaciCollectible.totalSupply + UInt64(1)

            self.id = LegaciCollectible.totalSupply

            // Set the metadata struct
            self.data = LegaciCollectibleData(brandId: brandId, collectionId: collectionId, unitId: unitId)

            emit LegaciCollectibleMinted(legaciCollectibleId: self.id, brandId: self.data.brandId, collectionId: self.data.collectionId, unitId: self.data.unitId)
        }

        // If the Legaci Collectible is destroyed, emit an event to indicate 
        // to outside observers that it has been destroyed
        destroy() {
            emit LegaciCollectibleDestroyed(id: self.id)
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Brands, and Legaci Collectibles
    //
    pub resource Admin {

        // createBrand creates a new Brand resource and stores it
        // in the brands mapping in the LegaciCollectible contract
        //
        // Parameters: brandId: The UUID string of the Brand
        //
        pub fun createBrand(brandId: String) {
            // Create the new Brand
            var newBrand <- create Brand(brandId: brandId)

            // Store it in the sets mapping field
            LegaciCollectible.brands[newBrand.brandId] <-! newBrand
        }

        // getBrandMintedCollectiblesCount
        // Returns the number of minted collectibles that the specified Brand.
        // 
        // Parameters: brandId: The id of the Set that is being searched
        //
        // Returns: The number of minted collectibles from the Brand 
        // stored in the Legaci Collectible smart contract resource
        pub fun getBrandMintedCollectiblesCount(brandId: String): UInt64? {
            // Don't force a revert if the brandId is invalid
            return LegaciCollectible.brands[brandId]?.numberOfMintedCollectibles
        }

        // borrowBrand returns a reference to a brand in the Legaci
        // Collectible contract so that the admin can call methods on it
        //
        // Parameters: brandId: The UUID string of the Brand that you want to
        // get a reference to
        //
        // Returns: A reference to the Brand with all of the fields
        // and methods exposed
        //
        pub fun borrowBrand(brandId: String): &Brand {
            pre {
                LegaciCollectible.brands[brandId] != nil: "Cannot borrow Brand: The Brand doesn't exist"
            }
            
            // Get a reference to the Set and return it
            // use `&` to indicate the reference to the object and type
            return &LegaciCollectible.brands[brandId] as &Brand
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Legaci Collectibles Collection as
    // to allow others to deposit Collectibles into their Collection. It also allows for reading
    // the IDs of Collectibles in the Collection.
    pub resource interface LegaciCollectibleCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowLegaciCollectible(id: UInt64): &LegaciCollectible.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Legaci Collectible reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: LegaciCollectibleCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Moment conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Legaci Collectible from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Legaci Collectible does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Legaci Collectibles
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a Legaci Collectible and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a Legaci Collectible NFT to make sure
            // it is the correct type
            let token <- token as! @LegaciCollectible.NFT

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

        // borrowNFT Returns a borrowed reference to a Legaci Collectible 
        // in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not any Legaci Collectible specific data. Please use borrowLegaciCollectible
        //  to read Legaci Collectible data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowLegaciCollectible returns a borrowed reference to a
        // Legaci Collectible so that the caller can read data and 
        // call methods from it. They can use this to read its brandId, 
        // collectionId, unitId, or any of the brandData associated with it by
        // getting the brandId and reading those fields from
        // the smart contract.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowLegaciCollectible(id: UInt64): &LegaciCollectible.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &LegaciCollectible.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Legaci Collectible contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Legaci Collectibles in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create LegaciCollectible.Collection()
    }

    // getBrands returns all the brands
    //
    // Returns: An array of all the plays that have been created
    pub fun getBrands(): [String] {
        return LegaciCollectible.brands.keys
    }

    // -----------------------------------------------------------------------
    // TopShot initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.brands <- {}
        self.totalSupply = 0

        // Initialize storage paths
        self.LegaciCollectibleCollectionStoragePath = /storage/LegaciCollectibleCollection
        self.LegaciCollectibleCollectionPublicPath = /public/LegaciCollectibleCollection
        self.LegaciCollectibleAdminStoragePath = /storage/LegaciCollectibleAdmin

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.LegaciCollectibleCollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{LegaciCollectibleCollectionPublic}>(self.LegaciCollectibleCollectionPublicPath, target: self.LegaciCollectibleCollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.LegaciCollectibleAdminStoragePath)

        emit ContractInitialized()
    }
}

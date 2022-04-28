/**

    TrmMarketV1.cdc

    Description: Contract definitions for users to sell and/or rent out their assets

    Marketplace is where users can create a sale collection that they
    store in their account storage. In the sale collection, 
    they can put their NFTs up for sale/rent with a price and publish a 
    reference so that others can see the sale.

    If another user sees an NFT that they want to buy,
    they can send fungible tokens that equal the buy price
    to buy the NFT.  The NFT is transferred to them when
    they make the purchase.

    If another user sees an NFT that they want to rent,
    they can send fungible tokens that equal the rent price
    to rent the NFT. The Rent NFT will be minted and 
    transferred to them.

    Each user who wants to sell/rent out tokens will have a sale 
    collection instance in their account that contains price information 
    for each node in their collection. The sale holds a capability that 
    links to their main asset collection.

    They can give a reference to this collection to a central contract
    so that it can list the sales in a central place

    When a user creates a sale, they will supply four arguments:
    - A Asset.Collection capability that allows their sale to withdraw an asset when it is purchased.
    - A FungibleToken.Receiver capability as the place where the payment for the token goes.
    - A FungibleToken.Receiver capability specifying a beneficiary, where a cut of the purchase gets sent. 
    - A cut percentage, specifying how much the beneficiary will recieve.
    
    The cut percentage can be set to zero if the user desires and they 
    will receive the entirety of the purchase. Trm will initialize sales 
    for users with the Trm admin vault as the vault where cuts get 
    deposited to.
**/

import TrmAssetV1 from 0x8affe45ad5f69282
import TrmRentV1 from 0x8affe45ad5f69282
import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393

pub contract TrmMarketV1 {
    // -----------------------------------------------------------------------
    // TRM Market contract Event definitions
    // -----------------------------------------------------------------------

    // Event that emitted when the NFT contract is initialized
    pub event ContractInitialized()
    /// emitted when an Asset is listed for sale
    pub event AssetListedForSale(assetTokenID: UInt64, price: UFix64, seller: Address?)
    /// emitted when an Asset is listed for rent
    pub event AssetListedForRent(assetTokenID: UInt64, price: UFix64, rentalPeriodSeconds: UFix64, seller: Address?)
    /// emitted when the sale price of a listed asset has changed
    pub event AssetSaleListingChanged(assetTokenID: UInt64, price: UFix64, seller: Address?)
    /// emitted when the rent price of a listed asset has changed
    pub event AssetRentListingChanged(assetTokenID: UInt64, price: UFix64, rentalPeriodSeconds: UFix64, seller: Address?)
    /// emitted when a token is purchased from the market
    pub event AssetPurchased(assetTokenID: UInt64, price: UFix64, seller: Address?, buyer: Address?)
    /// emitted when a token is rented from the market
    pub event AssetRented(assetTokenID: UInt64, rentID: UInt64, price: UFix64, expiryTimestamp: UFix64, assetURL: String, kID: String, seller: Address?, buyer: Address?, rentalPeriodSeconds: UFix64)
    /// emitted when a token has been delisted for sale
    pub event AssetDelistedForSale(assetTokenID: UInt64, owner: Address?)
    /// emitted when a token has been delisted for rent
    pub event AssetDelistedForRent(assetTokenID: UInt64, owner: Address?)

    /// Path where the `SaleCollection` is stored
    pub let marketStoragePath: StoragePath

    /// Path where the public capability for the `SaleCollection` is
    pub let marketPublicPath: PublicPath

    // Path where the 'Admin' resource is stored
    pub let adminStoragePath: StoragePath

    /// The percentage that is taken from every purchase for the beneficiary
    /// For example, if the percentage is 15%, cutPercentage = 0.15
    pub var cutPercentage: UFix64

    /// The capability that is used for depositing the beneficiary's cut of every sale
    pub var beneficiaryCapability: Capability<&{FungibleToken.Receiver}>

    // The private capability for minting rent tokens
    pub var minterCapability: Capability<&TrmRentV1.Minter>

    pub struct RentListing {
        pub var price: UFix64
        pub var rentalPeriodSeconds: UFix64

        access(contract) fun setRentalPeriodSeconds(seconds: UFix64) {
            self.rentalPeriodSeconds = seconds
        }

        access(contract) fun setPrice(price: UFix64) {
            self.price = price
        }

        init(price: UFix64, rentalPeriodSeconds: UFix64) {
            self.price = price
            self.rentalPeriodSeconds = rentalPeriodSeconds
        }
    }

    pub struct SaleListing {
        pub var price: UFix64

        access(contract) fun setPrice(price: UFix64) {
            self.price = price
        }

        init(price: UFix64) {
            self.price = price
        }
    }

    // SalePublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault, recipient: &{TrmAssetV1.CollectionPublic})
        pub fun rent(tokenID: UInt64, buyTokens: @FUSD.Vault, recipient: &{TrmRentV1.CollectionPublic}): UInt64
        pub fun getSaleListing(tokenID: UInt64): SaleListing?
        pub fun getRentListing(tokenID: UInt64): RentListing?
        pub fun getSaleIDs(): [UInt64]
        pub fun getRentIDs(): [UInt64]
        pub fun borrowAsset(id: UInt64): &TrmAssetV1.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Asset reference: The ID of the returned reference is incorrect"
            }
        }
    }

    /// SaleCollection
    ///
    /// This is the main resource that token sellers will store in their account
    /// to manage the NFTs that they are selling/renting. The SaleCollection keeps 
    /// track of the price of each token.
    /// 
    /// When a token is purchased, a cut is taken from the tokens
    /// and sent to the beneficiary, then the rest are sent to the seller.
    ///
    /// The seller chooses who the beneficiary is and what percentage
    /// of the tokens gets taken from the purchase
    pub resource SaleCollection: SalePublic {

        /// A reference to collection of the user's assets 
        access(self) var ownerCollection: Capability<&TrmAssetV1.Collection>

        /// Dictionary of the sale prices for each NFT by ID
        access(self) var saleListings: {UInt64: SaleListing}

        /// Dictionary of the rent prices for each NFT by ID
        access(self) var rentListings: {UInt64: RentListing}

        /// The fungible token vault of the seller, to receive tokens when their NFT is purchased
        access(self) var ownerCapability: Capability<&{FungibleToken.Receiver}>

        /// The capability that is used for depositing the beneficiary's cut of every sale

        /// The percentage that is taken from every purchase for the beneficiary
        /// For example, if the percentage is 15%, cutPercentage = 0.15


        init (ownerCollection: Capability<&TrmAssetV1.Collection>,
              ownerCapability: Capability<&{FungibleToken.Receiver}>) {
            pre {
                // Check that the owner's asset collection capability is correct
                ownerCollection.check(): 
                    "Owner's Asset Collection Capability is invalid!"

                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.check(): 
                    "Owner's Receiver Capability is invalid!"
            }
            
            // create an empty collection to store the tokens that are for sale
            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            // prices are initially empty because there are no tokens for sale
            self.saleListings = {}
            self.rentListings = {}
        }

        /// listForSale lists an NFT for sale in this sale collection at the specified price, or updates an existing listing
        ///
        /// Parameters: tokenID: The id of the NFT to be put up for sale
        ///             price: The price of the NFT
        pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.idExists(id: tokenID):
                    "Asset does not exist in the owner's collection"
            }

            if let listing = self.saleListings[tokenID] {
                listing.setPrice(price: price)

                self.saleListings[tokenID] = listing

                emit AssetSaleListingChanged(assetTokenID: tokenID, price: price, seller: self.owner?.address)
            } else {
                self.saleListings[tokenID] = SaleListing(price: price)

                emit AssetListedForSale(assetTokenID: tokenID, price: price, seller: self.owner?.address)
            }
        }

        /// listForRent lists an NFT for rent in this sale collection at the specified price and rental period, or updates an existing listing
        ///
        /// Parameters: tokenID: The id of the NFT to be put up for rent
        ///             price: The rent price of the NFT
        ///             rentalPeriodSeconds: The rental period (in seconds)
        //
        pub fun listForRent(tokenID: UInt64, price: UFix64?, rentalPeriodSeconds: UFix64?) {
            pre {
                self.ownerCollection.borrow()!.idExists(id: tokenID):
                    "Asset does not exist in the owner's collection"
            }

            if let listing = self.rentListings[tokenID] {
                assert(price != nil || rentalPeriodSeconds != nil, message: "One of price, rental period must be supplied to update existing listing")

                if(price != nil) {
                    listing.setPrice(price: price!)
                }

                if(rentalPeriodSeconds != nil) {
                    listing.setRentalPeriodSeconds(seconds: rentalPeriodSeconds!)
                }

                self.rentListings[tokenID] = listing

                emit AssetRentListingChanged(assetTokenID: tokenID, price: listing.price, rentalPeriodSeconds: listing.rentalPeriodSeconds, seller: self.owner?.address)
            } else {
                assert(price != nil && rentalPeriodSeconds != nil, message: "Price and rental period must be supplied for new listing")

                self.rentListings[tokenID] = RentListing(price: price!, rentalPeriodSeconds: rentalPeriodSeconds!)

                emit AssetListedForRent(assetTokenID: tokenID, price: price!, rentalPeriodSeconds: rentalPeriodSeconds!, seller: self.owner?.address)
            }
        }

        /// batchListForSale lists an array of NFTs for sale in this sale collection at the specified price, or updates existing listings
        ///
        /// Parameters: tokenIDs: The array of NFT IDs to be put up for sale
        ///             price: The sale price 
        pub fun batchListForSale(tokenIDs: [UInt64], price: UFix64) {
            for tokenID in tokenIDs {
                self.listForSale(tokenID: tokenID, price: price)
            }
        }

        /// batchListForRent lists an array of NFTs for rent in this sale collection at the specified price and rental period, or updates existing listings
        ///
        /// Parameters: tokenIDs: The array of NFT IDs to be put up for rent
        ///             price: The rent price
        ///             rentalPeriodSeconds: The rental period (in seconds)
        //
        pub fun batchListForRent(tokenIDs: [UInt64], price: UFix64?, rentalPeriodSeconds: UFix64?) {
            for tokenID in tokenIDs {
                self.listForRent(tokenID: tokenID, price: price, rentalPeriodSeconds: rentalPeriodSeconds)
            }
        }

        /// cancelSale cancels an asset sale and clears its price
        ///
        /// Parameters: tokenID: the ID of the token to remove from the sale
        ///
        pub fun cancelSale(tokenID: UInt64) {
            pre {
                self.saleListings[tokenID] != nil:
                    "Asset not listed for sale"
            }

            // Set listing to nil for the withdrawn ID
            self.saleListings[tokenID] = nil
            
            // Remove the listing from the listing dictionary
            self.saleListings.remove(key: tokenID)
            
            // Emit the event for delisting a token from the Sale
            emit AssetDelistedForSale(assetTokenID: tokenID, owner: self.owner?.address)
        }

        /// cancelRent cancels an asset rent and clears its price
        ///
        /// Parameters: tokenID: the ID of the token to remove from the sale
        ///
        pub fun cancelRent(tokenID: UInt64) {
            pre {
                self.rentListings[tokenID] != nil:
                    "Asset not listed for rent"
            }
            
            // Set listing to nil for the withdrawn ID
            self.rentListings[tokenID] = nil

            // Remove the listing from the listings dictionary
            self.rentListings.remove(key: tokenID)
            
            // Emit the event for delisting an asset for rent
            emit AssetDelistedForRent(assetTokenID: tokenID, owner: self.owner?.address)
        }

        /// batchCancelSale cancels the sale listings for the array of NFTs
        ///
        /// Parameters: tokenIDs: The array of NFT IDs to be removed for sale
        pub fun batchCancelSale(tokenIDs: [UInt64]) {
            for tokenID in tokenIDs {
                self.cancelSale(tokenID: tokenID)
            }
        }

        /// batchCancelSale cancels the rent listings for the array of NFTs
        ///
        /// Parameters: tokenIDs: The array of NFT IDs to be removed for rent
        pub fun batchCancelRent(tokenIDs: [UInt64]) {
            for tokenID in tokenIDs {
                self.cancelRent(tokenID: tokenID)
            }
        }

        /// purchase lets a user send tokens to purchase an NFT that is for sale
        /// the purchased NFT is returned to the transaction context that called it
        ///
        /// Parameters: tokenID: the ID of the NFT to purchase
        ///             buyTokens: the fungible tokens that are used to buy the NFT
        ///
        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault, recipient: &{TrmAssetV1.CollectionPublic}) {
            pre {
                self.saleListings[tokenID] != nil:
                    "No token matching this ID for sale!"

                self.saleListings[tokenID]!.price == buyTokens.balance:
                    "Not enough tokens to buy the NFT!"
            }

            // Read the price for the token
            let price = self.saleListings[tokenID]!.price

            // Set the price for the token to nil
            self.saleListings[tokenID] = nil

            // Remove the sale listing
            self.saleListings.remove(key: tokenID)

            // Remove for rent listing
            self.rentListings.remove(key: tokenID)

            // Take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdraw(amount: price*TrmMarketV1.cutPercentage)

            // Deposit it into the beneficiary's Vault
            TrmMarketV1.beneficiaryCapability.borrow()!
                .deposit(from: <-beneficiaryCut)
            
            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow()!
                .deposit(from: <-buyTokens)

            let boughtAsset <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID)

            recipient.deposit(token: <-boughtAsset)

            emit AssetPurchased(assetTokenID: tokenID, price: price, seller: self.owner?.address, buyer: recipient.owner?.address)
        }

        /// rent lets a user send tokens to rent an NFT that is for rent
        /// the newly minted rent NFT is returned to the transaction context that called it
        ///
        /// Parameters: tokenID: the ID of the NFT to purchase
        ///             buyTokens: the fungible tokens that are used to buy the NFT
        ///             recipient: Recipient Collection to receive Rent Token
        ///
        pub fun rent(tokenID: UInt64, buyTokens: @FUSD.Vault, recipient: &{TrmRentV1.CollectionPublic}): UInt64 {
            pre {
                self.rentListings[tokenID] != nil:
                    "No token matching this ID for rent!"

                self.rentListings[tokenID]!.price == buyTokens.balance:
                    "Not enough tokens to rent the NFT!"
            }

            // Read the price for the token
            let listing = self.rentListings[tokenID]!

            // Take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdraw(amount: listing.price*TrmMarketV1.cutPercentage)

            // Deposit it into the beneficiary's Vault
            TrmMarketV1.beneficiaryCapability.borrow()!
                .deposit(from: <-beneficiaryCut)
            
            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow()!
                .deposit(from: <-buyTokens)

            let ownerCollectionRef = self.ownerCollection.borrow()
                ?? panic("Could not borrow reference to owner collection vault")
            
            assert(ownerCollectionRef.idExists(id: tokenID) == true, message: "Specified token ID not found in owner collection")

            let expiry = listing.rentalPeriodSeconds + getCurrentBlock().timestamp

            let minterRef = TrmMarketV1.minterCapability.borrow() ??
                panic("Could not borrow Rent Minter Reference")

            let assetURL = ownerCollectionRef.getAssetURL(id: tokenID)

            let kID = ownerCollectionRef.getKID(id: tokenID)

            let rentTokenID = minterRef.mintNFT(assetTokenID: tokenID, kID: kID, assetURL: assetURL, expiryTimestamp: expiry, recipient: recipient)

            emit AssetRented(assetTokenID: tokenID, rentID: rentTokenID, price: listing.price, expiryTimestamp: expiry, assetURL: assetURL, kID: kID, seller: self.owner?.address, buyer: recipient.owner?.address, rentalPeriodSeconds: listing.rentalPeriodSeconds)

            return rentTokenID
        }

        /// getSalePrice returns the sale price of a specific token in the collection
        /// 
        /// Parameters: tokenID: The ID of the NFT whose sell price to get
        ///
        /// Returns: SaleListing: The sale listing of the token including price
        pub fun getSaleListing(tokenID: UInt64): SaleListing? {
            if let listing = self.saleListings[tokenID] {
                return listing
            }
            return nil
        }

        /// getRentPrice returns the rent price of a specific token in the collection
        /// 
        /// Parameters: tokenID: The ID of the NFT whose rent price to get
        ///
        /// Returns: RentListing: The rent listing of the token including price, rental period
        pub fun getRentListing(tokenID: UInt64): RentListing? {
            if let listing = self.rentListings[tokenID] {
                return listing
            }
            return nil
        }

        /// getSaleIDs returns an array of token IDs that are for sale
        pub fun getSaleIDs(): [UInt64] {
            return self.saleListings.keys
        }

        /// getRentIDs returns an array of token IDs that are for sale
        pub fun getRentIDs(): [UInt64] {
            return self.rentListings.keys
        }

        /// borrowAsset Returns a borrowed reference to an Asset in the Collection so that the caller can read data from it
        ///
        /// Parameters: id: The ID of the token to borrow a reference to
        ///
        /// Returns: &TrmAssetV1.NFT? Optional reference to a token for sale so that the caller can read its data
        pub fun borrowAsset(id: UInt64): &TrmAssetV1.NFT? {
            // first check this collection
            if self.saleListings[id] != nil || self.rentListings[id] != nil {
                let ref = self.ownerCollection.borrow()!.borrowAsset(id: id)
                return ref
            } 
            return nil
        }

    }

    /// createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerCollection: Capability<&TrmAssetV1.Collection>,
                                 ownerCapability: Capability<&{FungibleToken.Receiver}>): @SaleCollection {

        return <- create SaleCollection(ownerCollection: ownerCollection,
                                        ownerCapability: ownerCapability)
    }

    // initializes the beneficary Fusd Vault to that of the contract owner to receive cut of sales
    pub fun setupBeneficiaryFusdVault(): Capability<&{FungibleToken.Receiver}> {
        if (self.account.borrow<&FUSD.Vault>(from: /storage/fusdVault) != nil) {
            return self.account.getCapability<&AnyResource{FungibleToken.Receiver}>(/public/fusdReceiver)
        }
        
        // Create a new FUSD Vault and put it in storage
        self.account.save(<-FUSD.createEmptyVault(), to: /storage/fusdVault)

        // Create a public capability to the Vault that only exposes
        // the deposit function through the Receiver interface
        self.account.link<&FUSD.Vault{FungibleToken.Receiver}>(
            /public/fusdReceiver,
            target: /storage/fusdVault
        )

        // Create a public capability to the Vault that only exposes
        // the balance field through the Balance interface
        self.account.link<&FUSD.Vault{FungibleToken.Balance}>(
            /public/fusdBalance,
            target: /storage/fusdVault
        )

        return self.account.getCapability<&AnyResource{FungibleToken.Receiver}>(/public/fusdReceiver)
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // beneficiary capability and cut percentage 
    //
    pub resource Admin {

        // Admin may update the beneficiary capability to receive a cut of sales
        //
        pub fun setBeneficiaryCapability(beneficiaryCapability: Capability<&{FungibleToken.Receiver}>) {
            TrmMarketV1.beneficiaryCapability = beneficiaryCapability
        }

        // Admin may update the cut percentage of sales sent to beneficiary
        //
        pub fun setCutPercentage(cutPercentage: UFix64) {
            TrmMarketV1.cutPercentage = cutPercentage
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <- create Admin()
        }
    }

    init() {
        self.marketStoragePath = /storage/trmMarketV1SaleCollection
        self.marketPublicPath = /public/trmMarketV1SaleCollection
        self.adminStoragePath = /storage/trmMarketV1Admin

        self.beneficiaryCapability = TrmMarketV1.setupBeneficiaryFusdVault()
        self.cutPercentage = 0.05

        // First, check to see if a admin resource already exists
        if self.account.borrow<&Admin>(from: self.adminStoragePath) == nil {
        
            // Put the Admin in storage
            self.account.save<@Admin>(<- create Admin(), to: self.adminStoragePath)
        }

        // obtain Admin's private rent minter capability
        self.minterCapability = self.account.getCapability<&TrmRentV1.Minter>(TrmRentV1.minterPrivatePath)

        emit ContractInitialized()
    }
}

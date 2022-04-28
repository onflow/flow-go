/*
    Description: 

    authors: Matthew Balazsi, Alex Béliveau, Joseph Djenandji and Francis Ouellet
    contact: support@mint.store

The Market factory will allow administrators to create public market resources that can be used by accounts that own MintStoreItems to post their NFTs for sale. 
Marketplaces can be branded, in other words, will only allow MintStoreItems from specified merchants. 
Each market will also set its own fee structure through royalties. A flow account can host a single market at a time.

*/
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import NonFungibleToken from 0x1d7e57aa55817448
import MintStoreItem from 0x20187093790b9aef

// emulator
// import FungibleToken from FungibleToken
// import FUSD from FUSD
// import NonFungibleToken from NonFungibleToken
// import MintStoreItem from MintStoreItem


pub contract MintStoreMarketFactory {

    // -----------------------------------------------------------------------
    // Royalty
    //
    // This structure encapsulate a royalty recipient's capability and part of the cutPercentage.
    // Whenever a purchase is done, a cut is taken and distributed between all royalty recipients.
    // -----------------------------------------------------------------------
    pub struct Royalty{
        pub let wallet: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        pub let rate: UFix64

        init(wallet: Capability<&FUSD.Vault{FungibleToken.Receiver}>, rate: UFix64){
            self.wallet = wallet
            self.rate = rate
        }
    }

    // -----------------------------------------------------------------------
    // SalePublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    // -----------------------------------------------------------------------
    pub resource interface SalePublic {
        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault): @MintStoreItem.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowMintStoreItem(id: UInt64): &MintStoreItem.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow MintStoreItem reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // -----------------------------------------------------------------------
    // SaleCollection
    //
    // This is the main resource that token sellers will store in their account
    // to manage the NFTs that they are selling. The SaleCollection
    // holds a MintStoreItem Collection resource to store the mint store items that are for sale.
    // The SaleCollection also keeps track of the price of each token.
    // 
    // When a token is purchased, a cut is taken from the tokens
    // and sent to the royalty recipients, then the rest are sent to the seller.
    // -----------------------------------------------------------------------
    pub resource SaleCollection: SalePublic {

        // A collection of the MintStoreItems that the user has for sale
        access(self) var ownerCollection: Capability<&MintStoreItem.Collection>

        // Dictionary of the prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        // The fungible token vault of the seller
        // so that when someone buys a token, the tokens are deposited
        // to this Vault
        access(self) var ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        // The market on which the MintStoreItems are to be sold
        access(self) var marketCapability: Capability<&{MintStoreMarketFactory.MarketPublic}>

        init (ownerCollection: Capability<&MintStoreItem.Collection>, ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>, marketCapability: Capability<&{MintStoreMarketFactory.MarketPublic}>) {
            pre {
                // Check that the owner's MintStoreItem collection capability is correct
                ownerCollection.borrow() != nil: 
                    "Owner's MintStoreItem Collection Capability is invalid!"

                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.borrow() != nil: 
                     "Owner's Receiver Capability is invalid!"

                marketCapability.borrow() != nil:
                    "Market Capability is invalid!"
            }
            
            // create an empty collection to store the MintStoreItems that are for sale
            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            self.marketCapability = marketCapability
            // prices are initially empty because there are no MintStoreItems for sale
            self.prices = {}
        }

        // listForSale lists an NFT for sale in this sale collection
        // at the specified price
        //
        // Parameters: tokenID: The id of the NFT to be put up for sale
        //             price: The price of the NFT
        pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.borrowMintStoreItem(id: tokenID) != nil:
                    "MintStoreItem does not exist in the owner's collection"
                self.marketCapability.borrow() != nil: "Market Capability is invalid!"
                self.marketCapability.borrow()!.isFrozen == false : "Market is frozen"
                self.marketCapability.borrow()!.isItemValidMerchant(
                    item: self.ownerCollection.borrow()!.borrowMintStoreItem(id: tokenID)!
                ): "MintStoreItem does not have a valid merchant"
            }

            // Set the token's price
            if self.prices[tokenID] != nil  {
                let previousPrice = self.prices[tokenID];
                emit MintStoreItemPriceChanged(
                    marketID: self.marketCapability.borrow()!.marketID,
                    marketAddress: self.marketCapability.borrow()!.marketAddress!,
                    id: tokenID, newPrice: price,
                    previousPrice: previousPrice,
                    seller: self.owner?.address!
                )
            } else {
                emit MintStoreItemForSale(
                    marketID: self.marketCapability.borrow()!.marketID,
                    marketAddress: self.marketCapability.borrow()!.marketAddress!,
                    id: tokenID,
                    price: price,
                    seller: self.owner?.address!
                )
            }

            self.prices[tokenID] = price
        }

        // cancelSale cancels a MintStoreItem sale and clears its price
        //
        // Parameters: tokenID: the ID of the token to withdraw from the sale
        //
        pub fun cancelSale(tokenID: UInt64) {
            pre {
                self.prices[tokenID] != nil: "Token with the specified ID is not already for sale"
                self.marketCapability.borrow() != nil: "Market Capability is invalid!"
            }

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil
            
            // Emit the event for withdrawing a mint store item from the Sale
            emit MintStoreItemWithdrawn(
                marketID: self.marketCapability.borrow()!.marketID,
                marketAddress: self.marketCapability.borrow()!.marketAddress!,
                id: tokenID,
                seller: self.owner?.address!
            )
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        // the purchased NFT is returned to the transaction context that called it
        //
        // Parameters: tokenID: the ID of the NFT to purchase
        //             buyTokens: the fungible tokens that are used to buy the NFT
        //
        // Returns: @MintStoreItem.NFT: the purchased NFT
        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault): @MintStoreItem.NFT {
            pre {
                self.ownerCollection.borrow()!.borrowMintStoreItem(id: tokenID) != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance == (self.prices[tokenID] ?? UFix64(0)):
                    "Not enough tokens to buy the NFT!"
                self.marketCapability.borrow() != nil: "Market Capability is invalid!"
                self.marketCapability.borrow()!.isFrozen == false : "Market is frozen"
            }

            // Read the price for the token
            let price = self.prices[tokenID]!

            let royalties = self.marketCapability.borrow()!.royalties

            // Set the price for the token to nil
            self.prices[tokenID] = nil

            for key in royalties.keys {
                let recipient = royalties[key]!
                // Take the cut of the tokens that the recipent gets from the sent tokens
                //Check if capability is valid for each recipient
               if !recipient.wallet.check() {
                panic("Unable to borrow royalties recipient capability");
               }
            }

             for key in royalties.keys {
                let recipient = royalties[key]!
                // Take the cut of the tokens that the recipent gets from the sent tokens
                let recipientCut <- buyTokens.withdraw(amount: price*recipient.rate)

                // Deposit it into the recipient's Vault
                recipient.wallet.borrow()!
                    .deposit(from: <-recipientCut)
            }
            
            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow()!
                .deposit(from: <-buyTokens)

            emit MintStoreItemPurchased(
                marketID: self.marketCapability.borrow()!.marketID,
                marketAddress: self.marketCapability.borrow()!.marketAddress!,
                id: tokenID,
                price: price,
                seller: self.owner?.address!
            )

            // Return the purchased token
            let boughtMintStoreItem <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @MintStoreItem.NFT

            return <-boughtMintStoreItem
        }

        // getPrice returns the price of a specific token in the sale
        // 
        // Parameters: tokenID: The ID of the NFT whose price to get
        //
        // Returns: UFix64: The price of the token
        pub fun getPrice(tokenID: UInt64): UFix64? {
            return self.prices[tokenID]
        }

        // getIDs returns an array of token IDs that are for sale
        pub fun getIDs(): [UInt64] {
            return self.prices.keys
        }


       
        
        // borrowMintStoreItem Returns a borrowed reference to a MintStoreItem for sale
        // so that the caller can read data from it
        //
        // Parameters: id: The ID of the MintStoreItem to borrow a reference to
        //
        // Returns: &MintStoreItem.NFT? Optional reference to a MintStoreItem for sale 
        //                        so that the caller can read its data
        pub fun borrowMintStoreItem(id: UInt64): &MintStoreItem.NFT? {
            if self.prices[id] != nil {
                let ref = self.ownerCollection.borrow()!.borrowMintStoreItem(id: id)
                return ref
            } else {
                return nil
            }
        }
    }

    // -----------------------------------------------------------------------
    // MarketPublic
    // -----------------------------------------------------------------------
    pub resource interface MarketPublic {
        pub let marketID: UInt64
        access(contract) var marketAddress: Address?
        access(account) let royalties: {String: Royalty}
        pub var isFrozen: Bool
        pub var SaleCollectionStoragePath: StoragePath
        pub var SaleCollectionPublicPath: PublicPath
        access(account) fun isItemValidMerchant(item: &MintStoreItem.NFT): Bool
        pub fun getMerchantIDs(): [UInt32]
        pub fun getTotalRoyaltyRate(): UFix64
        pub fun createSaleCollection(ownerCollection: Capability<&MintStoreItem.Collection>, ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>, marketCapability: Capability<&{MintStoreMarketFactory.MarketPublic}>): @SaleCollection
    }

    // -----------------------------------------------------------------------
    // Market
    // -----------------------------------------------------------------------
    pub resource Market: MarketPublic {

        // basic info about the market that is usedful for events
        pub let marketID: UInt64
        access(contract) var marketAddress: Address?

        // ability to freeze the marketplace (can no longer add new items for sale or make purchases. All current sales can be withdrawn)
        pub var isFrozen: Bool
        // The capabilities and the percentages that is taken from every purchase and given to royalties
        // Fpr example, if the royalties are { "A": { wallet: ..., rate: 0.15 }, "B": { wallet: ..., rate: 0.10 } },
        // then 15% of a purchase will go to A, 10% will go to B and 75% will go to the seller
        access(account) let royalties: {String: Royalty}
        // Variable size dictionary of Merchants allowed on the Market
        access(self) var allowedMerchant: {UInt32: Bool}

        // Named paths
        //
        pub var SaleCollectionStoragePath: StoragePath
        pub var SaleCollectionPublicPath: PublicPath
       access(self) fun istInitializationValid(): Bool {
                return self.SaleCollectionStoragePath != nil &&
                self.SaleCollectionPublicPath != nil  &&
                self.marketAddress != nil;

        }
        // Allow a merchant to operate in the market
        pub fun allowMerchant(merchantID: UInt32) {
            pre {
              self.istInitializationValid();
            }
            self.allowedMerchant[merchantID] = true
            emit MarketMerchantAdded(marketID: self.marketID, merchantID: merchantID);
        }

        pub fun removeMerchantAccess(merchantID: UInt32) {
            pre {
             self.istInitializationValid();
            }
            self.allowedMerchant.remove(key: merchantID)
            emit MarketMerchantRemoved(marketID: self.marketID, merchantID: merchantID)
        }

        pub fun freezeMarket() {
            pre {
             self.istInitializationValid();
            }
            self.isFrozen = true;
            emit MarketFrozen(marketID: self.marketID, marketAddress: self.marketAddress!)
        }

        pub fun unFreezeMarket() {
            pre {
                              self.istInitializationValid();

            }
            self.isFrozen = false;
            emit MarketUnfrozen(marketID: self.marketID, marketAddress: self.marketAddress!)
        }

        // setSaleCollectionStoragePath changes the Market's sale collection storage path
        //
        // Parameters: storagePath: The new path to use for SaleCollectionStoragePath
        pub fun setSaleCollectionStoragePath(storagePath: StoragePath) {
            self.SaleCollectionStoragePath = storagePath
        }

        // setSaleCollectionPublicPath changes the Market's sale collection public path
        //
        // Parameters: publicPath: The new path to use for SaleCollectionPublicPath
        pub fun setSaleCollectionPublicPath(publicPath: PublicPath) {
            self.SaleCollectionPublicPath = publicPath
        }

        // setAddress changes the Market's address so it knows it when emitting events
        pub fun setAddress(address: Address) {
            self.marketAddress = address
            emit MarketAddressChanged(marketID: self.marketID, marketAddress: address)
        }

        // createCollection returns a new collection resource to the caller
        pub fun createSaleCollection(ownerCollection: Capability<&MintStoreItem.Collection>, ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>, marketCapability: Capability<&{MintStoreMarketFactory.MarketPublic}>): @SaleCollection {
            pre {
                self.istInitializationValid();

            }
            return <- create SaleCollection(ownerCollection: ownerCollection, ownerCapability: ownerCapability, marketCapability: marketCapability)
        }

        // addRoyalty adds a recipient for the cut of the sale
        //
        // Parameters: name: the key to store the new royalty
        //             wallet: the new capability for the recipient of the cut of the sale
        //             rate: the percentage of the sale that goes to that recipient
        //
       pub fun addRoyalty(name: String, wallet: Capability<&FUSD.Vault{FungibleToken.Receiver}>, rate: UFix64) {
            pre {
                self.istInitializationValid();
                self.royalties[name] == nil: "The royalty with that name already exists"
                wallet.borrow() != nil: "Recipient's Receiver Capability is invalid!"
                rate > 0.0: "Cannot set rate to less than 0%"
                rate <= 1.0: "Cannot set rate to more than 100%"
            }
            self.royalties[name] = Royalty(wallet: wallet, rate: rate)
            emit RoyaltyAdded(marketID: self.marketID, marketAddress: self.marketAddress!, name: name, rate: rate)
        }

        // removeRoyalty removes a recipient from the cut of the sale
        //
        // Parameters: name: the key of the recipient to remove
        //
        pub fun removeRoyalty(name: String) {
            pre {
                self.istInitializationValid();
                self.royalties[name] != nil: "The royalty with that name does not exist"
            }
            self.royalties.remove(key: name)
            emit RoyaltyRemoved(marketID: self.marketID, marketAddress: self.marketAddress!, name: name)
        }

        // changeRoyaltyRate updates a recipient's part of the cut of the sale
        //
        // Parameters: name: the key of the recipient to update
        //             rate: the new percentage of the sale that goes to that recipient
        //
        pub fun changeRoyaltyRate(name: String, rate: UFix64) {
            pre {
                self.istInitializationValid();
                self.royalties[name] != nil: "The royalty with that name does not exist"
                rate > 0.0: "Cannot set rate to less than 0%"
                rate <= 1.0: "Cannot set rate to more than 100%"
            }
            let royalty = self.royalties[name]!
            let previousRate = royalty.rate
            self.royalties[name] = Royalty(wallet: royalty.wallet, rate: rate)
            emit RoyaltyRateChanged(marketID: self.marketID,  marketAddress: self.marketAddress!, name: name, previousRate: previousRate, rate: rate)
        }

        // isItemValidMerchant checks if a MintStoreItem.NFT's merchantID is allowed for this Market
        //
        // Parameters: item: the MintStoreItem.NFT to validate
        //
        // Returns true if the MintStoreItem's merchantID is contained in the Market's allowed merchant list
        access(account) fun isItemValidMerchant(item: &MintStoreItem.NFT): Bool {
            return self.allowedMerchant[item.data.merchantID] == true
        }

        // getMerchantIDs returns an array of the MerchantIDs that are allowed to operate for this market
        pub fun getMerchantIDs(): [UInt32] {
            return self.allowedMerchant.keys
        }

        pub fun getTotalRoyaltyRate(): UFix64 {
            var totalRoyalty = 0.0
            for key in self.royalties.keys {
                let royal = self.royalties[key] ?? panic("Royalty does not exist")
                totalRoyalty = totalRoyalty + royal.rate
            }
            return totalRoyalty
        }

        init(marketID: UInt64) {
            self.marketID = marketID
            self.allowedMerchant = {}
            self.royalties = {}
            self.isFrozen = false
            self.SaleCollectionStoragePath =  /storage/mintStoreMarketSaleCollection //+ self.marketID
            self.SaleCollectionPublicPath =  /public/mintStoreMarketSaleCollection //+ self.marketID
            self.marketAddress = nil
            emit MarketCreated(marketID: marketID)
        }
    }

    // -----------------------------------------------------------------------
    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Editions and Items
    // -----------------------------------------------------------------------
    pub resource Admin {

        pub let id: UInt64

        // createNewAdmin creates a new Admin resource
        //
        // Returns: @Admin: the created admin
        pub fun createNewAdmin(): @Admin {
            let newID = MintStoreMarketFactory.nextAdminID
             // Increment the ID so that it isn't used again
            MintStoreMarketFactory.nextAdminID = MintStoreMarketFactory.nextAdminID + (1 as UInt64)
            return <-create Admin(id: newID)
        }

        // createNewMarket creates a new Market resource
        //
        // Returns: @Market: the created market
        pub fun createNewMarket(): @Market {
            let newID = MintStoreMarketFactory.nextMarketID
             // Increment the ID so that it isn't used again
            MintStoreMarketFactory.nextMarketID = MintStoreMarketFactory.nextMarketID + (1 as UInt64)
            return <-create Market(marketID: newID)
        }

        init(id: UInt64) {
            self.id = id
        }
    }

    // -----------------------------------------------------------------------
    // MarketFactory contract definitions
    // -----------------------------------------------------------------------

    // emitted when a Market is created
    pub event MarketCreated(marketID: UInt64)
    // emitted when a Market's address is changed
    pub event MarketAddressChanged(marketID: UInt64, marketAddress: Address)
    // emitted when a Market's isFrozen is set to true
    pub event MarketFrozen(marketID: UInt64, marketAddress: Address)
    // emitted when a Market's isFrozen is set to false
    pub event MarketUnfrozen(marketID: UInt64, marketAddress: Address)
    // emitted when a MintStoreItem is listed for sale
    pub event MintStoreItemForSale(marketID: UInt64, marketAddress: Address, id: UInt64, price: UFix64, seller: Address)
    // emitted when the price of a listed item has changed
    pub event MintStoreItemPriceChanged(marketID: UInt64, marketAddress: Address, id: UInt64, newPrice: UFix64, previousPrice: UFix64?, seller: Address)
    // emitted when a token is purchased from the market
    pub event MintStoreItemPurchased(marketID: UInt64, marketAddress: Address, id: UInt64, price: UFix64, seller: Address)
    // emitted when a mint store item has been withdrawn from the sale
    pub event MintStoreItemWithdrawn(marketID: UInt64, marketAddress: Address, id: UInt64, seller: Address)
    // emitted when the royalty has been changed by the owner
    pub event RoyaltyAdded(marketID: UInt64, marketAddress: Address, name: String, rate: UFix64)
    pub event RoyaltyRemoved(marketID: UInt64, marketAddress: Address, name: String)
    pub event RoyaltyRateChanged(marketID: UInt64, marketAddress: Address, name: String, previousRate: UFix64, rate: UFix64)

   // Merchant events
    pub event MarketMerchantAdded(marketID: UInt64, merchantID: UInt32)
    pub event MarketMerchantRemoved(marketID: UInt64, merchantID: UInt32)

    // Named paths
    //
    pub let MarketFactoryStoragePath: StoragePath
    pub let MarketFactoryPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let MarketStoragePath: StoragePath
    pub let MarketPublicPath: PublicPath

    // the ID used to create an admin
    pub var nextAdminID: UInt64
    // The ID that is used to create Marketplaces. 
    pub var nextMarketID: UInt64

    init () {
        self.MarketFactoryStoragePath = /storage/mintStoreMarketFactory
        self.MarketFactoryPublicPath = /public/mintStoreMarketFactory
        self.AdminStoragePath = /storage/MintStoreMarketFactoryAdmin
        self.MarketStoragePath = /storage/mintStoreMarket
        self.MarketPublicPath = /public/mintStoreMarket

        // Put the admin ressource in storage
        self.account.save<@Admin>(<- create Admin(id: 1), to: self.AdminStoragePath)
        self.nextAdminID = 2
        self.nextMarketID = 1
    }
}
 
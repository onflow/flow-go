/** 

# BaguetteSale contract

This contract allows to create SaleCollection to put a record for direct sale. 
The collection has a capability to withdraw from the record collection of the seller, without getting access to record unlocking functions.

## Withdrawals and cancelations

Listings can be cancelled. 

## Accept an offer

Offers are settled automatically since it is a directl listing.

## Create a Sale
A Sale is created within a SaleCollection. A SaleCollection is kept in a user's storage and points to their record collection.
A SaleCollection is created through a contract level function which the default market and artist cuts set by an admin.

*/


import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FUSD from 0x3c5959b568896393
import Record from 0x3558f4548a40dc77
import ArtistRegistery from 0x3558f4548a40dc77


pub contract BaguetteSale {

    // -----------------------------------------------------------------------
    // Variables 
    // -----------------------------------------------------------------------

    // Public path to the sale collection, allowing to buy listed items
    pub let CollectionPublicPath: PublicPath
    // Storage path to the sale collection
    pub let CollectionStoragePath: StoragePath
    // Admin storage page, allowing sale collection creation and parameter changes
    pub let AdminStoragePath: StoragePath

    // Default parameters for auctions
    pub var parameters: SaleParameters
    access(self) var marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
    access(self) var lostFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?

    // -----------------------------------------------------------------------
    // Events 
    // -----------------------------------------------------------------------

    // Events direct offers
    //
    // A record has been purchased via direct sale
    pub event RecordPurchased(recordID: UInt64, price: UFix64, seller: Address)
    // A record has been listed for direct sale
    pub event ListingCreated(recordID: UInt64, price: UFix64, seller: Address)
    // A listing has been canceled
    pub event ListingCanceled(recordID: UInt64, seller: Address)

    // Market and Artist share
    pub event MarketplaceEarned(recordID: UInt64, seller: Address, amount: UFix64, owner: Address)
    pub event ArtistEarned(recordID: UInt64, seller: Address, amount: UFix64, artistID: UInt64)

    // lost and found events
    pub event FUSDLostAndFound(recordID: UInt64, seller: Address, amount: UFix64, address: Address)

    // -----------------------------------------------------------------------
    // Resources 
    // -----------------------------------------------------------------------

    // Structure representing auction parameters
    pub struct SaleParameters {
        pub let artistCut: UFix64 // share of the artist for a sale
        pub let marketCut: UFix64 // share of the marketplace for a sale

        init(
            artistCut: UFix64,
            marketCut: UFix64
        ) {
            self.artistCut = artistCut
            self.marketCut = marketCut
        }
    }
    
    // CollectionPublic
    //
    // Expose information retrieval functions and buyer interface
    //
    pub resource interface CollectionPublic {
        pub var parameters: SaleParameters

        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault): @Record.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun isForSale(tokenID: UInt64): Bool 
    }

    pub resource Collection: CollectionPublic {

        pub var parameters: SaleParameters

        access(self) var ownerCollection: Capability<&Record.Collection{Record.CollectionPublic, NonFungibleToken.Provider}>
        access(self) var ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        access(self) var prices: {UInt64: UFix64}
        

        init (
            parameters: SaleParameters, 
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            rCollection: Capability<&Record.Collection{Record.CollectionPublic, NonFungibleToken.Provider}>) {
            pre {
                fVault.check(): "The fungible vault should be valid."
                rCollection.check(): "The non fungible collection should be valid."
            }
            self.prices = {}
            self.ownerCapability = fVault
            self.ownerCollection = rCollection
            
            self.parameters = parameters
        }

        // List the item for sale. Note that there is no check here to see if the item is tradable (unlocked without decryption key)
        // However, the during a sale, the transaction of the trade will panic if the item is not tradable
        pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID) != nil:
                    "Record does not exist in the owner's collection"
                self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID)!.tradable(): "The item cannot be traded due to its current locked mode: it is probably waiting for its decryption key."
            }

            // Set the token's price
            self.prices[tokenID] = price

            emit ListingCreated(recordID: tokenID, price: price, seller: self.owner?.address!)
        }

        pub fun cancelSale(tokenID: UInt64) {
            pre {
                self.prices[tokenID] != nil: "Token with the specified ID is not already for sale"
            }

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil
            
            // Emit the event for withdrawing a moment from the Sale
            emit ListingCanceled(recordID: tokenID, seller: self.owner?.address!)
        }

        pub fun purchase(tokenID: UInt64, buyTokens: @FUSD.Vault): @Record.NFT {
            pre {
                self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID) != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"           
                self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID)!.tradable(): "The item cannot be traded due to its current locked mode: it is probably waiting for its decryption key."
                buyTokens.balance == (self.prices[tokenID] ?? 0.0):
                    "Not enough tokens to buy the NFT!"
            }

            let price = self.prices[tokenID]!
            self.prices[tokenID] = nil

            let record <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @Record.NFT

            // Deposit it into the beneficiary's Vault
            let amountMarket = price * self.parameters.marketCut
            let amountArtist = price * self.parameters.artistCut
            let marketCut <- buyTokens.withdraw(amount:amountMarket)
            let artistCut <- buyTokens.withdraw(amount:amountArtist)

            let marketVault = BaguetteSale.marketVault!.borrow()!
            marketVault.deposit(from: <- marketCut)
            emit MarketplaceEarned(recordID: record.id, seller: self.owner?.address!, amount: amountMarket, owner: marketVault.owner!.address)

            let artistID = record.metadata.artistID
            ArtistRegistery.sendArtistShare(id: artistID, deposit: <-artistCut)
            emit ArtistEarned(recordID: record.id, seller: self.owner?.address!, amount: amountArtist, artistID: artistID)

            // Deposit the remaining tokens into the owners vault
            // If the owner capability is unlinked, tries the lost and found. If it is unlinked too (should not happen in practice), tokens are destroyed
            var fVaultOpt = self.ownerCapability.borrow()
            if fVaultOpt == nil {
                fVaultOpt = BaguetteSale.lostFVault!.borrow()
                if fVaultOpt != nil {
                    emit FUSDLostAndFound(recordID: tokenID, seller: self.owner?.address!, amount: buyTokens.balance, address: fVaultOpt!.owner!.address)
                }
            }

            if let fVault = fVaultOpt {
                fVault.deposit(from: <-buyTokens.withdraw(amount: buyTokens.balance))
            }
            destroy buyTokens

            emit RecordPurchased(recordID: tokenID, price: price, seller: self.owner?.address!)
            return <-record
        }

        pub fun getPrice(tokenID: UInt64): UFix64? {
            return self.prices[tokenID]
        }

        pub fun getIDs(): [UInt64] {
            return self.prices.keys
        }

        // Check if the sale is still available
        pub fun isForSale(tokenID: UInt64): Bool {
            return self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID) != nil 
                && self.prices[tokenID] != nil
                && self.ownerCollection.borrow()!.borrowRecord(recordID: tokenID)!.tradable()
        }
    }

    // Admin resource
    //
    // Admin can change the market & artist cuts and market vault. 
    // The administrator can create SaleCollection with custom parameters.
    //
    pub resource Admin {
        
        pub fun setParameters(parameters: SaleParameters) {
            BaguetteSale.parameters = parameters
        }

        // Set the vault used to collect market shares
        pub fun setMarketVault(marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                marketVault.check(): "The market vault should be valid."
            }
            BaguetteSale.marketVault = marketVault
        }

        pub fun setLostAndFoundVaults(
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        ) {
            pre {
                fVault.check(): "The fungible token vault should be valid."
            }
            BaguetteSale.lostFVault = fVault
        }

        // createCollection returns a new collection resource to the caller
        pub fun createCustomSaleCollection(
            parameters: SaleParameters, 
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            ownerNFVault: Capability<&Record.Collection{Record.CollectionPublic, NonFungibleToken.Provider}>): @Collection {
            
            return <- create Collection(
                parameters: parameters,
                fVault: ownerFVault,
                rCollection: ownerNFVault
            )
        }
    }

    // -----------------------------------------------------------------------
    // Contract public functions
    // -----------------------------------------------------------------------

    // createCollection returns a new collection resource to the caller, with default market & artist shares
    pub fun createSaleCollection(
        ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
        ownerNFVault: Capability<&Record.Collection{Record.CollectionPublic, NonFungibleToken.Provider}>): @Collection {
        
        return <- create Collection(
            parameters: BaguetteSale.parameters,
            fVault: ownerFVault,
            rCollection: ownerNFVault
        )
    }

    // -----------------------------------------------------------------------
    // Initialization function
    // -----------------------------------------------------------------------

    init() {
        self.parameters = SaleParameters(
            artistCut: 0.10,
            marketCut: 0.03
        )
        self.marketVault = nil
        self.lostFVault = nil

        self.CollectionPublicPath = /public/boulangeriev1SaleCollection
        self.CollectionStoragePath = /storage/boulangeriev1SaleCollection
        self.AdminStoragePath = /storage/boulangeriev1SaleAdmin

        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}
 
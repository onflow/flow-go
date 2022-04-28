import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Blockletes_NFT from 0x9969d64233d69723
/*

    BlockleteMarket.cdc

    Description: Contract definitions for users to sell their blockletes

    Marketplace is where users can create a sale collection that they
    store in their account storage. In the sale collection, 
    they can put their NFTs up for sale with a price and publish a 
    reference so that others can see the sale.

    If another user sees an NFT that they want to buy,
    they can send fungible tokens that equal or exceed the buy price
    to buy the NFT.  The NFT is transferred to them when
    they make the purchase.

    Each user who wants to sell tokens will have a sale collection 
    instance in their account that holds the tokens that they are putting up for sale

    They can give a reference to this collection to a central contract
    so that it can list the sales in a central place

    When a user creates a sale, they will supply three arguments:
    - A FungibleToken.Receiver capability as the place where the payment for the token goes.
    - A FungibleToken.Receiver capability specifying a beneficiary, where a cut of the purchase gets sent. 
    - A cut percentage, specifying how much the beneficiary will recieve.
    
    The cut percentage can be set to zero if the user desires and they 
    will receive the entirety of the purchase. Blocklete will initialize sales 
    for users with the Blocklete admin wallet as the wallet where cuts get 
    deposited to.
*/

pub contract BlockleteMarket_NFT {

    // -----------------------------------------------------------------------
    // Blocklete Market contract Event definitions
    // -----------------------------------------------------------------------

    // emitted when a Blocklete is listed for sale
    pub event BlockleteListed(id: UInt64, price: UFix64, seller: Address?)
    // emitted when the price of a listed blocklete has changed
    pub event BlockletePriceChanged(id: UInt64, newPrice: UFix64, seller: Address?)
    // emitted when a token is purchased from the market
    pub event BlockletePurchased(id: UInt64, price: UFix64, seller: Address?)
    // emitted when a Blocklete has been withdrawn from the sale
    pub event BlockleteWithdrawn(id: UInt64, owner: Address?)
    // emitted when the cut percentage of the sale has been changed by the owner
    pub event CutPercentageChanged(newPercent: UFix64, seller: Address?)

    // Named paths
    //
    pub let SaleCollectionStoragePath: StoragePath
    pub let SaleCollectionPublicPath: PublicPath

    // SalePublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutPercentage: UFix64
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @Blockletes_NFT.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowBlocklete(id: UInt64): &Blockletes_NFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Blocklete reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // SaleCollection
    //
    // This is the main resource that token sellers will store in their account
    // to manage the NFTs that they are selling. The SaleCollection
    // holds a Blocklete Collection resource to store the Blockletes that are for sale.
    // The SaleCollection also keeps track of the price of each token.
    // 
    // When a token is purchased, a cut is taken from the tokens
    // and sent to the beneficiary, then the rest are sent to the seller.
    //
    // The seller chooses who the beneficiary is and what percentage
    // of the tokens gets taken from the purchase
    pub resource SaleCollection: SalePublic {

        // A collection of the Blocklete that the user has for sale
        access(self) var blockletesForSale: @Blockletes_NFT.Collection

        // Dictionary of the prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        // The fungible token vault of the seller
        // so that when someone buys a token, the tokens are deposited
        // to this Vault
        access(self) var ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        // The capability that is used for depositing 
        // the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        // The percentage that is taken from every purchase for the beneficiary
        // For example, if the percentage is 5%, cutPercentage = 0.05
        pub var cutPercentage: UFix64

        init (
            ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            beneficiaryCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            cutPercentage: UFix64
        ) {
            pre {
                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.borrow() != nil: 
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.borrow() != nil: 
                    "Beneficiary's Receiver Capability is invalid!" 
                cutPercentage >= 0.0 && cutPercentage <= 100.0:
                    "Cut percentage must be a percentage 0-100"
            }

            // create an empty collection to store the Blocklete that are for sale
            self.blockletesForSale <- Blockletes_NFT.createEmptyCollection() as! @Blockletes_NFT.Collection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            // prices are initially empty because there are no Blockletes for sale
            self.prices = {}
            self.cutPercentage = cutPercentage
        }

        // listForSale lists an NFT for sale in this sale collection
        // at the specified price
        //
        // Parameters: token: The NFT to be put up for sale
        //             price: The price of the NFT
        pub fun listForSale(token: @Blockletes_NFT.NFT, price: UFix64) {
            pre {
                // Check that price of the listing is > 0
                price > 0.0: 
                    "Price must be greater than 0"
            }
            // get the ID of the token
            let id = token.id

            // Set the token's price
            self.prices[token.id] = price

            // Deposit the token into the sale collection
            self.blockletesForSale.deposit(token: <-token)

            emit BlockleteListed(id: id, price: price, seller: self.owner?.address)
        }

        // Withdraw removes a Blocklete that was listed for sale
        // and clears its price
        //
        // Parameters: tokenID: the ID of the token to withdraw from the sale
        //
        // Returns: @Blockletes_NFT.NFT: The nft that was withdrawn from the sale
        pub fun withdraw(tokenID: UInt64): @Blockletes_NFT.NFT {

            // Remove and return the token.
            // Will revert if the token doesn't exist
            let token <- self.blockletesForSale.withdraw(withdrawID: tokenID) as! @Blockletes_NFT.NFT

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil
            
            // Emit the event for withdrawing a Blocklete from the Sale
            emit BlockleteWithdrawn(id: token.id, owner: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        // the purchased NFT is returned to the transaction context that called it
        //
        // Parameters: tokenID: the ID of the NFT to purchase
        //             buyTokens: the fungible tokens that are used to buy the NFT
        //
        // Returns: @Blockletes_NFT.NFT: the purchased NFT
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @Blockletes_NFT.NFT {
            pre {
                self.blockletesForSale.ownedNFTs[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"           
                buyTokens.balance == (self.prices[tokenID] ?? 0.0):
                    "Not enough tokens to buy the NFT!"
            }

            // Read the price for the token
            let price = self.prices[tokenID]!

            // Set the price for the token to nil
            self.prices[tokenID] = nil

            // Take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdraw(amount: price*self.cutPercentage)

            // Deposit it into the beneficiary's Vault
            self.beneficiaryCapability.borrow()!
                .deposit(from: <-beneficiaryCut)
            
            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow()!
                .deposit(from: <-buyTokens)

            emit BlockletePurchased(id: tokenID, price: price, seller: self.owner?.address)

            // Return the purchased token
            return <-self.withdraw(tokenID: tokenID)
        }

        // changePrice changes the price of a token that is currently for sale
        //
        // Parameters: tokenID: The ID of the NFT's price that is changing
        //             newPrice: The new price for the NFT
        pub fun changePrice(tokenID: UInt64, newPrice: UFix64) {
            pre {
                self.prices[tokenID] != nil: "Cannot change the price for a token that is not for sale"
            }
            // Set the new price
            self.prices[tokenID] = newPrice

            emit BlockletePriceChanged(id: tokenID, newPrice: newPrice, seller: self.owner?.address)
        }

        // changePercentage changes the cut percentage of the tokens that are for sale
        //
        // Parameters: newPercent: The new cut percentage for the sale
        pub fun changePercentage(_ newPercent: UFix64) {
            pre {
                newPercent >= 0.0 && newPercent <= 100.0:
                    "New cut percentage must be a percentage 0-100"
            }
            // Set the new cut percentage
            self.cutPercentage = newPercent

            emit CutPercentageChanged(newPercent: newPercent, seller: self.owner?.address)
        }

        // changeOwnerReceiver updates the capability for the sellers fungible token Vault
        //
        // Parameters: newOwnerCapability: The new fungible token capability for the account 
        //                                 who received tokens for purchases
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                newOwnerCapability.borrow() != nil: 
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        // changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        //
        // Parameters: newBeneficiaryCapability the new capability for the beneficiary of the cut of the sale
        //
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                newBeneficiaryCapability.borrow() != nil: 
                    "Beneficiary's Receiver Capability is invalid!" 
            }
            self.beneficiaryCapability = newBeneficiaryCapability
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
            return self.blockletesForSale.getIDs()
        }

        // borrowBlocklete Returns a borrowed reference to a Blocklete in the collection
        // so that the caller can read data from it
        //
        // Parameters: id: The ID of the Blocklete to borrow a reference to
        //
        // Returns: &Blockletes_NFT.NFT? Optional reference to a Blocklete for sale 
        //                        so that the caller can read its data
        //
        pub fun borrowBlocklete(id: UInt64): &Blockletes_NFT.NFT? {
            let ref = self.blockletesForSale.borrowBlocklete(id: id)
            return ref
        }

        // If the sale collection is destroyed, 
        // destroy the tokens that are for sale inside of it
        destroy() {
            destroy self.blockletesForSale
        }
    }

    // createSaleCollection returns a new sale collection resource to the caller
    pub fun createSaleCollection(
        ownerCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
        beneficiaryCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
        cutPercentage: UFix64
    ): @SaleCollection {
        return <- create SaleCollection(ownerCapability: ownerCapability, beneficiaryCapability: beneficiaryCapability, cutPercentage: cutPercentage)
    }

    init () {
        self.SaleCollectionStoragePath = /storage/BlockleteMarketCollection_NFT
        self.SaleCollectionPublicPath = /public/BlockleteMarketCollection_NFT
    }
}
 
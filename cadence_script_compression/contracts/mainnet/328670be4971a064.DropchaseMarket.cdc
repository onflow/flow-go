//SPDX-License-Identifier: UNLICENSED

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import Dropchase from 0x328670be4971a064
import DropchaseCoin from 0x328670be4971a064
import DropchaseCreatorRegistry from 0x328670be4971a064

pub contract DropchaseMarket {

    // -----------------------------------------------------------------------
    // Dropchase Market contract Event definitions
    // -----------------------------------------------------------------------

    /// emitted when a Dropchase item is listed for sale
    pub event ItemListed(id: UInt64, price: UFix64, seller: Address?)
    /// emitted when the price of a listed item has changed
    pub event ItemPriceChanged(id: UInt64, newPrice: UFix64, seller: Address?)
    /// emitted when a token is purchased from the market
    pub event ItemPurchased(id: UInt64, price: UFix64, seller: Address?)
    /// emitted when a item has been withdrawn from the sale
    pub event ItemWithdrawn(id: UInt64, owner: Address?)
    // emitted when an creator has received a cut
    pub event creatorCutReceived(name: String, cut: UFix64)

    /// Path where the `SaleCollection` is stored
    pub let marketStoragePath: StoragePath

    /// Path where the public capability for the `SaleCollection` is
    pub let marketPublicPath: PublicPath


    // SalePublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutPercentage: UFix64
        pub fun purchase(tokenID: UInt64, buyTokens: @DropchaseCoin.Vault): @Dropchase.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowItem(id: UInt64): &Dropchase.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Item reference: The ID of the returned reference is incorrect"
            }
        }
    }

    /// SaleCollection
    ///
    /// This is the main resource that token sellers will store in their account
    /// to manage the NFTs that they are selling. The SaleCollection
    /// holds a Dropchase Collection resource to store the items that are for sale.
    /// The SaleCollection also keeps track of the price of each token.
    /// 
    /// When a token is purchased, a cut is taken from the tokens
    /// and sent to the beneficiary, then the rest are sent to the seller.
    ///
    /// The seller chooses who the beneficiary is and what percentage
    /// of the tokens gets taken from the purchase
    pub resource SaleCollection: SalePublic {

        /// A collection of the items that the user has for sale
        access(self) var ownerCollection: Capability<&Dropchase.Collection>

        /// Dictionary of the low low prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        /// The fungible token vault of the seller
        /// so that when someone buys a token, the tokens are deposited
        /// to this Vault
        access(self) var ownerCapability: Capability<&{FungibleToken.Receiver}>

        /// The capability that is used for depositing 
        /// the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability<&{FungibleToken.Receiver}>

        /// The percentage that is taken from every purchase for the beneficiary
        /// For example, if the percentage is 15%, cutPercentage = 0.15
        pub var cutPercentage: UFix64
        

        init (ownerCollection: Capability<&Dropchase.Collection>,
              ownerCapability: Capability<&{FungibleToken.Receiver}>,
              beneficiaryCapability: Capability<&{FungibleToken.Receiver}>,
              cutPercentage: UFix64) {
            pre {
                // Check that the owner's item collection capability is correct
                ownerCollection.check(): 
                    "Owner's Item Collection Capability is invalid!"

                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.check(): 
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.check(): 
                    "Beneficiary's Receiver Capability is invalid!" 

            }
            
            // create an empty collection to store the items that are for sale
            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            // prices are initially empty because there are no items for sale
            self.prices = {}
            self.cutPercentage = cutPercentage
        }

        /// listForSale lists an NFT for sale in this sale collection
        /// at the specified price
        ///
        /// Parameters: tokenID: The id of the NFT to be put up for sale
        ///             price: The price of the NFT
        pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.borrowItem(id: tokenID) != nil:
                    "Item does not exist in the owner's collection"
            }

            // Set the token's price
            self.prices[tokenID] = price

            emit ItemListed(id: tokenID, price: price, seller: self.owner?.address)
        }

         // Withdraw removes a moment that was listed for sale
        // and clears its price
        //
        // Parameters: tokenID: the ID of the token to withdraw from the sale
        //
        // Returns: @Dropchase.NFT: The nft that was withdrawn from the sale
        pub fun withdraw(tokenID: UInt64): @Dropchase.NFT {

            // Remove and return the token.
            // Will revert if the token doesn't exist
            let token <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @Dropchase.NFT

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil
            
            // Emit the event for withdrawing a moment from the Sale
            emit ItemWithdrawn(id: token.id, owner: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }


        /// cancelSale cancels a item sale and clears its price
        ///
        /// Parameters: tokenID: the ID of the token to withdraw from the sale
        ///
        pub fun cancelSale(tokenID: UInt64) {
            
            // First check this version of the sale
            if self.prices[tokenID] != nil {
                // Remove the price from the prices dictionary
                self.prices.remove(key: tokenID)

                // Set prices to nil for the withdrawn ID
                self.prices[tokenID] = nil
                
                // Emit the event for withdrawing a item from the Sale
                emit ItemWithdrawn(id: tokenID, owner: self.owner?.address)
            }
        }

        /// purchase lets a user send tokens to purchase an NFT that is for sale
        /// the purchased NFT is returned to the transaction context that called it
        ///
        /// Parameters: tokenID: the ID of the NFT to purchase
        ///             buyTokens: the fungible tokens that are used to buy the NFT
        ///
        /// Returns: @Dropchase.NFT: the purchased NFT
        pub fun purchase(tokenID: UInt64, buyTokens: @DropchaseCoin.Vault): @Dropchase.NFT {

            // First check this sale collection for the NFT
            pre {
                 self.prices[tokenID] != nil:
                     "No price found for that token!"
             }
                assert(
                    buyTokens.balance == self.prices[tokenID]!,
                    message: "Not enough tokens to buy the NFT!"
                )

                // Read the price for the token
                let price = self.prices[tokenID]!

                // Set the price for the token to nil
                self.prices[tokenID] = nil

                // Take the cut of the tokens that the beneficiary gets from the sent tokens
                let beneficiaryCut <- buyTokens.withdraw(amount: price*self.cutPercentage)

                // Deposit it into the beneficiary's Vault
                self.beneficiaryCapability.borrow()!
                    .deposit(from: <-beneficiaryCut)

                // Return the purchased token
                let boughtItem <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @Dropchase.NFT

                // Find the creator name
                let creatorName = self.getCreatorNameFromItem(item: &boughtItem as &Dropchase.NFT)


                // Withdraw the influener cut
                 let creatorCutPercentage = DropchaseCreatorRegistry.getCutPercentage(name: creatorName)
                 let creatorCutAmount = price*creatorCutPercentage
                 let creatorCut <- buyTokens.withdraw(amount: creatorCutAmount)

                 // Deposit the creator cut
                 let creatorCap = DropchaseCreatorRegistry.getCapability(name: creatorName)
                   ?? panic("Cannot find the creator in the registry")
                 let creatorReceiverRef = creatorCap.borrow<&{FungibleToken.Receiver}>()
                   ?? panic("Cannot find a token receiver for the creator")
                 creatorReceiverRef.deposit(from: <-creatorCut)
                 emit creatorCutReceived(name: creatorName, cut: creatorCutAmount)
                
                // Deposit the remaining tokens into the owners vault
                self.ownerCapability.borrow()!.deposit(from: <-buyTokens)

                emit ItemPurchased(id: tokenID, price: price, seller: self.owner?.address)

                return <-boughtItem
  
        }

            access(self) fun getCreatorNameFromItem(item: &Dropchase.NFT): String {
            let statID = item.data.statID
            let metadata = Dropchase.getStatMetaData(statID: statID)
            return metadata!["Creator"]!
        }

        /// changeOwnerReceiver updates the capability for the sellers fungible token Vault
        ///
        /// Parameters: newOwnerCapability: The new fungible token capability for the account 
        ///                                 who received tokens for purchases
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability<&{FungibleToken.Receiver}>) {
            pre {
                newOwnerCapability.borrow() != nil: 
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        /// changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        ///
        /// Parameters: newBeneficiaryCapability the new capability for the beneficiary of the cut of the sale
        ///
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability<&{FungibleToken.Receiver}>) {
            pre {
                newBeneficiaryCapability.borrow() != nil: 
                    "Beneficiary's Receiver Capability is invalid!" 
            }
            self.beneficiaryCapability = newBeneficiaryCapability
        }

        /// getPrice returns the price of a specific token in the sale
        /// 
        /// Parameters: tokenID: The ID of the NFT whose price to get
        ///
        /// Returns: UFix64: The price of the token
        pub fun getPrice(tokenID: UInt64): UFix64? {
            if let price = self.prices[tokenID] {
                return price
            } 
            return nil
        }

        /// getIDs returns an array of token IDs that are for sale
        pub fun getIDs(): [UInt64] {
            let vKeys = self.prices.keys
                return vKeys
            }

        /// borrowItem Returns a borrowed reference to a Item for sale
        /// so that the caller can read data from it
        ///
        /// Parameters: id: The ID of the item to borrow a reference to
        ///
        /// Returns: &Dropchase.NFT? Optional reference to a item for sale 
        ///                        so that the caller can read its data
        ///
        pub fun borrowItem(id: UInt64): &Dropchase.NFT? {
            // first check this collection
            if self.prices[id] != nil {
                let ref = self.ownerCollection.borrow()!.borrowItem(id: id)
                return ref
            } else {
                return nil
            }
        }
    }


    /// createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerCollection: Capability<&Dropchase.Collection>,
                                 ownerCapability: Capability<&{FungibleToken.Receiver}>): @SaleCollection {

        let beneficiaryCapability = getAccount(self.account.address).getCapability<&{FungibleToken.Receiver}>(/public/DropchaseCoinReceiver)


        return <- create SaleCollection(ownerCollection: ownerCollection,
                                        ownerCapability: ownerCapability,
                                        beneficiaryCapability: beneficiaryCapability,
                                        cutPercentage: 0.05)
    }

    init() {
        self.marketStoragePath = /storage/DropchaseSaleCollection
        self.marketPublicPath = /public/DropchaseSaleCollection 
    }
}
 
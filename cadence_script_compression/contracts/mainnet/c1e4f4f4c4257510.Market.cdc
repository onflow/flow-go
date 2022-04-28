/*

    MarketTopShot.cdc

    Description: Contract definitions for users to sell their moments

    Authors: Joshua Hannan joshua.hannan@dapperlabs.com
             Dieter Shirley dete@axiomzen.com

    Marketplace is where users can create a sale collectio that they
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

    When a user creates a sale, they will specify a fungible token capability
    as the place where the payment for the token goes, and they also give
    another fungible token capability for where a cut of the purchase
    gets sent. The cut can be set to zero if the user desires and they
    will receive the entirety of the purchase. Topshot will initialize sales
    for users with the topshot admin vault as the vault where cuts get
    deposited to.
*/

import FungibleToken from 0xf233dcee88fe0abe
import DapperUtilityCoin from 0xead892083b3e2c6c
import NonFungibleToken from 0x1d7e57aa55817448
import TopShot from 0x0b2a3299cc857e29

pub contract Market {

    // -----------------------------------------------------------------------
    // TopShot Market contract Event definitions
    // -----------------------------------------------------------------------

    // emitted when a TopShot moment is listed for sale
    pub event MomentListed(id: UInt64, price: UFix64, seller: Address?)
    // emitted when the price of a listed moment has changed
    pub event MomentPriceChanged(id: UInt64, newPrice: UFix64, seller: Address?)
    // emitted when a token is purchased from the market
    pub event MomentPurchased(id: UInt64, price: UFix64, seller: Address?)
    // emitted when a moment has been withdrawn from the sale
    pub event MomentWithdrawn(id: UInt64, owner: Address?)
    // emitted when the cut percentage of the sale has been changed by the owner
    pub event CutPercentageChanged(newPercent: UFix64, seller: Address?)

    // SalePublic
    //
    // The interface that a user can publish their sale as
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutPercentage: UFix64
        pub fun purchase(tokenID: UInt64, buyTokens: @DapperUtilityCoin.Vault): @TopShot.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Moment reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // SaleCollection
    //
    // This is the main resource that token sellers will store in their account
    // to manage the NFTs that they are selling. The SaleCollection
    // holds a TopShot Collection resource to store the moments that are for sale
    // The SaleCollection also keeps track of the price of each token.
    //
    // When a token is purchased, a cut is taken from the tokens that are used to
    // purchase and sent to the beneficiary, then the rest are sent to the seller
    pub resource SaleCollection: SalePublic {

        // A collection of the moments that the user has for sale
        access(self) var forSale: @TopShot.Collection

        // Dictionary of the prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        // The fungible token vault of the seller
        // so that when someone buys a token, the tokens are deposited
        // to this Vault
        access(self) var ownerCapability: Capability

        // The capability that is used for depositing
        // the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability

        // the percentage that is taken from every purchase for the beneficiary
        // This is a literal percentage
        // For example, if the percentage is 15%, cutPercentage = 0.15
        pub var cutPercentage: UFix64

        init (ownerCapability: Capability, beneficiaryCapability: Capability, cutPercentage: UFix64) {
            pre {
                // Check that both capabilities are for fungible token Vault receivers
                // for dapper utility coin
                ownerCapability.borrow<&{FungibleToken.Receiver}>() != nil:
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.borrow<&{FungibleToken.Receiver}>() != nil:
                    "Beneficiary's Receiver Capability is invalid!"
            }

            self.forSale <- TopShot.createEmptyCollection() as! @TopShot.Collection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            self.prices = {}
            self.cutPercentage = cutPercentage
        }

        // listForSale lists an NFT for sale in this sale collection
        // at the specified price
        pub fun listForSale(token: @TopShot.NFT, price: UFix64) {

            // get the ID of the token
            let id = token.id

            // Set the token's price
            self.prices[token.id] = price

            // Deposit the token into the collection
            self.forSale.deposit(token: <-token)

            emit MomentListed(id: id, price: price, seller: self.owner?.address)
        }

        // Withdraw removes a moment that was listed for sale
        pub fun withdraw(tokenID: UInt64): @TopShot.NFT {

            // remove and return the token
            // will revert if the token doesn't exist
            let token <- self.forSale.withdraw(withdrawID: tokenID) as! @TopShot.NFT

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil

            // Emit the event for withdrawing a moment from the Sale
            emit MomentWithdrawn(id: token.id, owner: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        // the purchased NFT is returned to the transaction context that called it
        pub fun purchase(tokenID: UInt64, buyTokens: @DapperUtilityCoin.Vault): @TopShot.NFT {
            pre {
                self.forSale.ownedNFTs[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance == (self.prices[tokenID] ?? UFix64(0)):
                    "Not enough tokens to buy the NFT!"
            }

            // Read the price for the token
            let price = self.prices[tokenID]!

            // Set the price for the token to nil
            self.prices[tokenID] = nil

            // take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdraw(amount: price*self.cutPercentage)

            // deposit it into the beneficiary's Vault
            self.beneficiaryCapability.borrow<&{FungibleToken.Receiver}>()!
                .deposit(from: <-beneficiaryCut)

            // deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow<&{FungibleToken.Receiver}>()!
                .deposit(from: <-buyTokens)

            emit MomentPurchased(id: tokenID, price: price, seller: self.owner?.address)

            // return the purchased token
            return <-self.withdraw(tokenID: tokenID)
        }

        // changePrice changes the price of a token that is currently for sale
        pub fun changePrice(tokenID: UInt64, newPrice: UFix64) {
            pre {
                self.prices[tokenID] != nil: "Cannot change the price for a token that is not for sale"
            }
            // set the new price
            self.prices[tokenID] = newPrice

            emit MomentPriceChanged(id: tokenID, newPrice: newPrice, seller: self.owner?.address)
        }

        // changePercentage changes the cut percentage of the tokens that are for sale
        pub fun changePercentage(_ newPercent: UFix64) {
            self.cutPercentage = newPercent

            emit CutPercentageChanged(newPercent: newPercent, seller: self.owner?.address)
        }

        // changeOwnerReceiver updates the capability for the sellers fungible token Vault
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability) {
            pre {
                newOwnerCapability.borrow<&{FungibleToken.Receiver}>() != nil:
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        // changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability) {
            pre {
                newBeneficiaryCapability.borrow<&DapperUtilityCoin.Vault{FungibleToken.Receiver}>() != nil:
                    "Beneficiary's Receiver Capability is invalid!"
            }
            self.beneficiaryCapability = newBeneficiaryCapability
        }

        // getPrice returns the price of a specific token in the sale
        pub fun getPrice(tokenID: UInt64): UFix64? {
            return self.prices[tokenID]
        }

        // getIDs returns an array of token IDs that are for sale
        pub fun getIDs(): [UInt64] {
            return self.forSale.getIDs()
        }

        // borrowMoment Returns a borrowed reference to a Moment in the collection
        // so that the caller can read data from it
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            let ref = self.forSale.borrowMoment(id: id)
            return ref
        }

        destroy() {
            destroy self.forSale
        }
    }

    // createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerCapability: Capability, beneficiaryCapability: Capability, cutPercentage: UFix64): @SaleCollection {
        return <- create SaleCollection(ownerCapability: ownerCapability, beneficiaryCapability: beneficiaryCapability, cutPercentage: cutPercentage)
    }
}

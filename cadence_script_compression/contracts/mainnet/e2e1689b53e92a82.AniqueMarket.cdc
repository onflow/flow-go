//  SPDX-License-Identifier: UNLICENSED
//
//  Secondary marketplace for Anique NFTs.
//
import Anique from 0xe2e1689b53e92a82
import AniqueCredit from 0xe2e1689b53e92a82

pub contract AniqueMarket {

    // -----------------------------------------------------------------------
    // Anique Market contract Event definitions
    // -----------------------------------------------------------------------

    // emitted when an Anique Collectible is listed for sale
    pub event CollectibleListed(id: UInt64, price: UFix64, seller: Address?)
    // emitted when a Collectible is purchased from the market
    pub event CollectiblePurchased(id: UInt64, price: UFix64, seller: Address?)
    // emitted when a Collectible has been withdrawn from the sale
    pub event CollectibleSaleCancelled(id: UInt64, owner: Address?)
    // emitted when the cut rate of the sale has been changed by the owner
    pub event CutRateChanged(newRate: UFix64, seller: Address?)

    // SalePublic
    //
    // The interface that a user can publish their sale as
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutRate: UFix64
        pub fun purchase(tokenID: UInt64, buyTokens: @AniqueCredit.Vault, admin: &AniqueCredit.Admin): @Anique.NFT {
            post {
                result.id == tokenID:
                    "The ID of the withdrawn token must be the same as the requested ID"
                admin != nil:
                    "admin must be set"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowCollectible(id: UInt64): auth &Anique.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                //result.id == id:
                    "Cannot borrow Collectible reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // SaleCollection
    //
    // This is the main resource that token sellers will store in their account
    // to manage the NFTs that they are selling. The SaleCollection
    // holds a Anique Collection resource to store the Collectibles that are for sale
    // The SaleCollection also keeps track of the price of each Collectible.
    //
    // When a Collectible is purchased, a cut is taken from the Collectible that are used to
    // purchase and sent to the beneficiary, then the rest are sent to the seller
    pub resource SaleCollection: SalePublic {

        // A collection of the moments that the user has for sale
        access(self) var ownerCollection: Capability<&Anique.Collection>

        // Dictionary of the prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        // The fungible token vault of the seller
        // so that when someone buys a token, the tokens are deposited
        // to this Vault
        access(self) var ownerCapability: Capability

        // The capability that is used for depositing
        // the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability

        // the rate that is taken from every purchase for the beneficiary
        // This is a literal rate
        // For example, if the rate is 15%, cutRate = 0.15
        pub var cutRate: UFix64

        init (ownerCollection: Capability<&Anique.Collection>,
              ownerCapability: Capability<&{AniqueCredit.Receiver}>,
              beneficiaryCapability: Capability<&{AniqueCredit.Receiver}>,
              cutRate: UFix64
        ) {
            pre {
                // Check that the owner's Collectible collection capability is correct
                ownerCollection.borrow() != nil:
                    "Owner's Collectible Collection Capability is invalid!"
                // Check that both capabilities are for fungible token Vault receivers
                // for dapper utility coin
                ownerCapability.borrow() != nil:
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.borrow() != nil:
                    "Beneficiary's Receiver Capability is invalid!"
                cutRate >= 0.0 && cutRate < 1.0:
                    "Cut rate is invalid value.  0.0 <= cut rate < 1.0"
            }

            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            self.prices = {}
            self.cutRate = cutRate
        }

        // listForSale lists an NFT for sale in this sale collection
        // at the specified price
        pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.borrowNFT(id: tokenID) != nil:
                    "Collectible does not exist in the owner's collection"
                price > 0.0:
                    "Price should greater than 0"
            }

            // Set the token's price
            self.prices[tokenID] = price

            emit CollectibleListed(id: tokenID, price: price, seller: self.owner?.address)
        }

        // cancelSale cancels a Collectible sale and clears its price
        //
        // Parameters: tokenID: the ID of the token to withdraw from the sale
        //
        pub fun cancelSale(tokenID: UInt64) {
            pre {
                self.prices[tokenID] != nil: "Token with the specified ID is not already for sale"
            }

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil

            // Emit the event for withdrawing a moment from the Sale
            emit CollectibleSaleCancelled(id: tokenID, owner: self.owner?.address)
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        // the purchased NFT is returned to the transaction context that called it
        pub fun purchase(tokenID: UInt64, buyTokens: @AniqueCredit.Vault, admin: &AniqueCredit.Admin): @Anique.NFT {
            pre {
                self.ownerCollection.borrow()!.borrowNFT(id: tokenID) != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance == (self.prices[tokenID] ?? 0.0):
                    "Not enough tokens to buy the NFT!"
            }

            // Read the price for the token
            let price = self.prices[tokenID]!

            // Set the price for the token to nil
            self.prices[tokenID] = nil

            // take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdrawByAdmin(amount: price * self.cutRate, admin: admin)

            // deposit it into the beneficiary's Vault
            self.beneficiaryCapability
                .borrow<&{AniqueCredit.Receiver}>()!
                .deposit(from: <-beneficiaryCut)

            // deposit the remaining tokens into the owners vault
            self.ownerCapability
                .borrow<&{AniqueCredit.Receiver}>()!
                .deposit(from: <-buyTokens)

            emit CollectiblePurchased(id: tokenID, price: price, seller: self.owner?.address)

            // return the purchased token
            return <- (self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @Anique.NFT)
        }

        // changeRate changes the cut rate of the tokens that are for sale
        pub fun changeRate(_ newRate: UFix64) {
            pre {
                newRate >= 0.0 && newRate < 1.0:
                    "Cut rate is invalid value.  0.0 <= cut rate < 1.0"
            }

            self.cutRate = newRate

            emit CutRateChanged(newRate: newRate, seller: self.owner?.address)
        }

        // changeOwnerReceiver updates the capability for the sellers fungible token Vault
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability<&AnyResource{AniqueCredit.Receiver}>) {
            pre {
                newOwnerCapability.borrow() != nil:
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        // changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability<&AnyResource{AniqueCredit.Receiver}>) {
            pre {
                newBeneficiaryCapability.borrow() != nil:
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
            return self.prices.keys
        }

        // borrowCollectible Returns a borrowed reference to a Collectible in the collection
        // so that the caller can read data from it
        pub fun borrowCollectible(id: UInt64): auth &Anique.NFT? {
            if self.prices[id] != nil {
                return self.ownerCollection.borrow()!.borrowAniqueNFT(id: id)
            } else {
                return nil
            }
        }
    }

    // createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(
        ownerCollection: Capability<&Anique.Collection>,
        ownerCapability: Capability<&AnyResource{AniqueCredit.Receiver}>,
        beneficiaryCapability: Capability<&AnyResource{AniqueCredit.Receiver}>,
        cutRate: UFix64
    ): @SaleCollection
    {
        return <- create SaleCollection(
            ownerCollection: ownerCollection,
            ownerCapability: ownerCapability,
            beneficiaryCapability: beneficiaryCapability,
            cutRate: cutRate)
    }
}

/*
    This contract is mostly copied from the MarketTopShot contract but with
    modifications to integrate with Eternal's influencer system, such that
    influencers receive cuts from transactions that take place on the marketplace.
*/

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import Eternal from 0xc38aea683c0c4d38
import InfluencerRegistry from 0xc38aea683c0c4d38

pub contract Market {

    // -----------------------------------------------------------------------
    // Eternal Market contract Event definitions
    // -----------------------------------------------------------------------

    // emitted when a Eternal moment is listed for sale
    pub event MomentListed(id: UInt64, price: UFix64, seller: Address?)
    // emitted when the price of a listed moment has changed
    pub event MomentPriceChanged(id: UInt64, newPrice: UFix64, seller: Address?)
    // emitted when a token is purchased from the market
    pub event MomentPurchased(id: UInt64, price: UFix64, seller: Address?)
    // emitted when a moment has been withdrawn from the sale
    pub event MomentWithdrawn(id: UInt64, owner: Address?)
    // emitted when the cut percentage of the sale has been changed by the owner
    pub event CutPercentageChanged(newPercent: UFix64, seller: Address?)
    // emitted when an influencer has received a cut
    pub event InfluencerCutReceived(name: String, ftType: Type, cut: UFix64)

    // SalePublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutPercentage: UFix64
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @Eternal.NFT {
            post {
                result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
        pub fun getPrice(tokenID: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowMoment(id: UInt64): &Eternal.NFT? {
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
    // holds a Eternal Collection resource to store the moments that are for sale.
    // The SaleCollection also keeps track of the price of each token.
    // 
    // When a token is purchased, a cut is taken from the tokens
    // and sent to the beneficiary, then the rest are sent to the seller.
    //
    // The seller chooses who the beneficiary is and what percentage
    // of the tokens gets taken from the purchase
    pub resource SaleCollection: SalePublic {

        // A collection of the moments that the user has for sale
        access(self) var forSale: @Eternal.Collection

        // Dictionary of the low low prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        // The fungible token vault of the seller
        // so that when someone buys a token, the tokens are deposited
        // to this Vault
        access(self) var ownerCapability: Capability

        // The capability that is used for depositing 
        // the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability

        // The percentage that is taken from every purchase for the beneficiary
        // For example, if the percentage is 15%, cutPercentage = 0.15
        pub var cutPercentage: UFix64

        // The fungible token that should be used to transact with this
        // sale collection.
        pub var ftType: Type

        init (ftType: Type, ownerCapability: Capability, beneficiaryCapability: Capability, cutPercentage: UFix64) {
            pre {
                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.borrow<&{FungibleToken.Receiver}>()!.isInstance(ftType): 
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.borrow<&{FungibleToken.Receiver}>()!.isInstance(ftType): 
                    "Beneficiary's Receiver Capability is invalid!" 
            }

            // create an empty collection to store the moments that are for sale
            self.forSale <- Eternal.createEmptyCollection() as! @Eternal.Collection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            // prices are initially empty because there are no moments for sale
            self.prices = {}
            self.cutPercentage = cutPercentage
            self.ftType = ftType
        }

        // listForSale lists an NFT for sale in this sale collection
        // at the specified price
        //
        // Parameters: token: The NFT to be put up for sale
        //             price: The price of the NFT
        pub fun listForSale(token: @Eternal.NFT, price: UFix64) {

            // get the ID of the token
            let id = token.id

            // Set the token's price
            self.prices[token.id] = price

            // Deposit the token into the sale collection
            self.forSale.deposit(token: <-token)

            emit MomentListed(id: id, price: price, seller: self.owner?.address)
        }

        // Withdraw removes a moment that was listed for sale
        // and clears its price
        //
        // Parameters: tokenID: the ID of the token to withdraw from the sale
        //
        // Returns: @Eternal.NFT: The nft that was withdrawn from the sale
        pub fun withdraw(tokenID: UInt64): @Eternal.NFT {

            // Remove and return the token.
            // Will revert if the token doesn't exist
            let token <- self.forSale.withdraw(withdrawID: tokenID) as! @Eternal.NFT

            // Remove the price from the prices dictionary
            self.prices.remove(key: tokenID)

            // Set prices to nil for the withdrawn ID
            self.prices[tokenID] = nil
            
            // Emit the event for withdrawing a moment from the Sale
            emit MomentWithdrawn(id: token.id, owner: self.owner?.address)

            // Return the withdrawn token
            return <-token
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        // the purchased NFT is returned to the transaction context that called it
        //
        // Parameters: tokenID: the ID of the NFT to purchase
        //             butTokens: the fungible tokens that are used to buy the NFT
        //
        // Returns: @Eternal.NFT: the purchased NFT
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @Eternal.NFT {
            pre {
                self.forSale.ownedNFTs[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"           
                buyTokens.isInstance(self.ftType): "payment vault is not requested fungible token"
                buyTokens.balance == (self.prices[tokenID] ?? UFix64(0)):
                    "Not enough tokens to buy the NFT!"
            }

            // Read the price for the token
            let price = self.prices[tokenID]!

            // Set the price for the token to nil
            self.prices[tokenID] = nil

            // Return the purchased token
            let nft <- self.withdraw(tokenID: tokenID)

            // Find the influencer name
            let influencerName = self.getInfluencerNameFromMoment(moment: &nft as &Eternal.NFT)

            // Withdraw the influener cut
            let influencerCutPercentage = InfluencerRegistry.getCutPercentage(name: influencerName)
            let influencerCutAmount = price*influencerCutPercentage
            let influencerCut <- buyTokens.withdraw(amount: influencerCutAmount)

            // Deposit the influencer cut
            let influencerCap = InfluencerRegistry.getCapability(name: influencerName, ftType: self.ftType)
                ?? panic("Cannot find the influencer in the registry")
            let influencerReceiverRef = influencerCap.borrow<&{FungibleToken.Receiver}>()
                ?? panic("Cannot find a token receiver for the influencer")
            influencerReceiverRef.deposit(from: <-influencerCut)
            emit InfluencerCutReceived(name: influencerName, ftType: self.ftType, cut: influencerCutAmount)

            // Withdraw the beneficiary cut
            let beneficiaryCut <- buyTokens.withdraw(amount: price*self.cutPercentage)

            // Deposit the beneficiary Vault
            self.beneficiaryCapability.borrow<&{FungibleToken.Receiver}>()!
                .deposit(from: <-beneficiaryCut)

            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow<&{FungibleToken.Receiver}>()!
                .deposit(from: <-buyTokens)

            emit MomentPurchased(id: tokenID, price: price, seller: self.owner?.address)
            return <-nft
        }

        access(self) fun getInfluencerNameFromMoment(moment: &Eternal.NFT): String {
            let playID = moment.data.playID
            let metadata = Eternal.getPlayMetaData(playID: playID)
            return metadata!["Influencer"]!
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

            emit MomentPriceChanged(id: tokenID, newPrice: newPrice, seller: self.owner?.address)
        }

        // changePercentage changes the cut percentage of the tokens that are for sale
        //
        // Parameters: newPercent: The new cut percentage for the sale
        pub fun changePercentage(_ newPercent: UFix64) {
            self.cutPercentage = newPercent

            emit CutPercentageChanged(newPercent: newPercent, seller: self.owner?.address)
        }

        // changeOwnerReceiver updates the capability for the sellers fungible token Vault
        //
        // Parameters: newOwnerCapability: The new fungible token capability for the account 
        //                                 who received tokens for purchases
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability) {
            pre {
                newOwnerCapability.borrow<&{FungibleToken.Receiver}>() != nil: 
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        // changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        //
        // Parameters: newBeneficiaryCapability the new capability for the beneficiary of the cut of the sale
        //
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability) {
            pre {
                newBeneficiaryCapability.borrow<&{FungibleToken.Receiver}>() != nil: 
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
            return self.forSale.getIDs()
        }

        // borrowMoment Returns a borrowed reference to a Moment in the collection
        // so that the caller can read data from it
        //
        // Parameters: id: The ID of the moment to borrow a reference to
        //
        // Returns: &Eternal.NFT? Optional reference to a moment for sale 
        //                        so that the caller can read its data
        //
        pub fun borrowMoment(id: UInt64): &Eternal.NFT? {
            let ref = self.forSale.borrowMoment(id: id)
            return ref
        }

        // If the sale collection is destroyed, 
        // destroy the tokens that are for sale inside of it
        destroy() {
            destroy self.forSale
        }
    }

    // createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ftType: Type, ownerCapability: Capability, beneficiaryCapability: Capability, cutPercentage: UFix64): @SaleCollection {
        return <- create SaleCollection(ftType: ftType, ownerCapability: ownerCapability, beneficiaryCapability: beneficiaryCapability, cutPercentage: cutPercentage)
    }
}
 
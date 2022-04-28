import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

pub contract TokenTroveMarket {

   // -----------------------------------------------------------------------
   // TokenTroveMarket contract Event definitions
   // -----------------------------------------------------------------------

   // emitted when an NFT is listed for sale
   pub event NFTListed(id: UInt64, price: UFix64, seller: Address?, tokenAddress: Address?)
   // emitted when the price of a listed NFT has changed
   pub event NFTPriceChanged(id: UInt64, newPrice: UFix64, seller: Address?, tokenAddress: Address?)
   // emitted when an NFT is purchased from the market
   pub event NFTPurchased(id: UInt64, price: UFix64, seller: Address?, tokenAddress: Address?)
   // emitted when an NFT has been withdrawn from the sale
   pub event NFTWithdrawn(id: UInt64, owner: Address?, tokenAddress: Address?)
   // emitted when the cut percentage of the sale has been changed by the owner
   pub event CutPercentageChanged(newPercent: UFix64, seller: Address?, tokenAddress: Address?)

   // SalePublic 
   //
   // The interface that a user can publish a capability to their sale
   // to allow others to access their sale
   pub resource interface SalePublic {
      pub var cutPercentage: UFix64
      pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @NonFungibleToken.NFT {
            post {
               result.id == tokenID: "The ID of the withdrawn token must be the same as the requested ID"
            }
      }
      pub fun getPrice(tokenID: UInt64): UFix64?
      pub fun getIDs(): [UInt64]
   }

   // SaleCollection
   //
   // This is the main resource that token sellers will store in their account
   // to manage the NFTs that they are selling. The SaleCollection
   // holds a NonFungibleToken Collection resource to store the NFTs that are for sale.
   // The SaleCollection also keeps track of the price of each token.
   // 
   // When a token is purchased, a cut is taken from the tokens
   // and sent to the beneficiary, then the rest are sent to the seller.
   //
   // The seller chooses who the beneficiary is and what percentage
   // of the tokens gets taken from the purchase
   pub resource SaleCollection: SalePublic {

      // A collection of the nfts that the user has for sale
      access(self) var ownerCollection: Capability<&{NonFungibleToken.Provider,NonFungibleToken.CollectionPublic}>

      // Dictionary of the low prices for each NFT by ID
      access(self) var prices: {UInt64: UFix64}

      // The address of the tokens this saleCollection holds
      access(self) var tokenAddress: Address

      // The fungible token vault of the seller
      // so that when someone buys a token, the tokens are deposited
      // to this Vault
      access(self) var ownerCapability: Capability<&{FungibleToken.Receiver}>

      // The capability that is used for depositing 
      // the beneficiary's cut of every sale
      access(self) var beneficiaryCapability: Capability<&{FungibleToken.Receiver}>

      // The percentage that is taken from every purchase for the beneficiary
      // For example, if the percentage is 15%, cutPercentage = 0.15
      pub var cutPercentage: UFix64

      init (ownerCollection: Capability<&{NonFungibleToken.Provider,NonFungibleToken.CollectionPublic}>, ownerCapability: Capability<&{FungibleToken.Receiver}>, tokenAddress: Address, beneficiaryCapability: Capability<&{FungibleToken.Receiver}>, cutPercentage: UFix64) {
            pre {
               // Check that the owner's nft collection capability is correct
               ownerCollection.borrow() != nil: 
                  "Owner's NFT Collection Capability is invalid!"

               // Check that both capabilities are for fungible token Vault receivers
               ownerCapability.borrow() != nil: 
                  "Owner's Receiver Capability is invalid!"
               beneficiaryCapability.borrow() != nil: 
                  "Beneficiary's Receiver Capability is invalid!" 
            }

            // create an empty collection to store the nfts that are for sale
            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            self.tokenAddress = tokenAddress
            // prices are initially empty because there are no nfts for sale
            self.prices = {}
            self.cutPercentage = cutPercentage
      }

      // listForSale lists an NFT for sale in this sale collection
      // at the specified price
      //
      // Parameters: tokenID: The id of the NFT to be put up for sale
      //             price: The price of the NFT
      pub fun listForSale(tokenID: UInt64, price: UFix64) {
            pre {
               self.ownerCollection.borrow()!.borrowNFT(id: tokenID) != nil:
                  "NFT does not exist in the owner's collection"
            }

            // Set the token's price
            self.prices[tokenID] = price

            emit NFTListed(id: tokenID, price: price, seller: self.owner?.address, tokenAddress: self.tokenAddress)
      }

      // cancelSale cancels a nft sale and clears its price
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

            // Emit the event for withdrawing an nft from the Sale
            emit NFTWithdrawn(id: tokenID, owner: self.owner?.address, tokenAddress: self.tokenAddress)
      }

      // purchase lets a user send tokens to purchase an NFT that is for sale
      // the purchased NFT is returned to the transaction context that called it
      //
      // Parameters: tokenID: the ID of the NFT to purchase
      //             buyTokens: the fungible tokens that are used to buy the NFT
      //
      // Returns: @NonFungibleToken.NFT: the purchased NFT
      pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault): @NonFungibleToken.NFT {
            pre {
               self.ownerCollection.borrow()!.borrowNFT(id: tokenID) != nil && self.prices[tokenID] != nil:
                  "No token matching this ID for sale!"           
               buyTokens.balance == (self.prices[tokenID] ?? UFix64(0)):
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

            emit NFTPurchased(id: tokenID, price: price, seller: self.owner?.address, tokenAddress: self.tokenAddress)

            // Return the purchased token
            let boughtNFT <- self.ownerCollection.borrow()!.withdraw(withdrawID: tokenID) as! @NonFungibleToken.NFT

            return <-boughtNFT
      }

      pub fun changePrice(tokenID: UInt64, newPrice: UFix64) {
         pre {
             self.prices[tokenID] != nil: "Cannot change the price for a token that is not for sale"
         }
         // set the new price
         self.prices[tokenID] = newPrice

         emit NFTPriceChanged(id: tokenID, newPrice: newPrice, seller: self.owner?.address, tokenAddress: self.tokenAddress)
      }
           
      // changePercentage changes the cut percentage of the tokens that are for sale
      //
      // Parameters: newPercent: The new cut percentage for the sale
      pub fun changePercentage(_ newPercent: UFix64) {
            self.cutPercentage = newPercent

            emit CutPercentageChanged(newPercent: newPercent, seller: self.owner?.address, tokenAddress: self.tokenAddress)
      }

      // changeOwnerReceiver updates the capability for the sellers fungible token Vault
      //
      // Parameters: newOwnerCapability: The new fungible token capability for the account 
      //                                 who received tokens for purchases
      pub fun changeOwnerReceiver(_ newOwnerCapability: Capability<&{FungibleToken.Receiver}>) {
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
      pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability<&{FungibleToken.Receiver}>) {
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
            return self.prices.keys
      }
   }

   // createCollection returns a new collection resource to the caller
   pub fun createSaleCollection(ownerCollection: Capability<&{NonFungibleToken.Provider,NonFungibleToken.CollectionPublic}>, ownerCapability: Capability<&{FungibleToken.Receiver}>,tokenAddress: Address, beneficiaryCapability: Capability<&{FungibleToken.Receiver}>,  cutPercentage: UFix64): @SaleCollection {
      return <- create SaleCollection(ownerCollection: ownerCollection, ownerCapability: ownerCapability, tokenAddress:tokenAddress, beneficiaryCapability: beneficiaryCapability, cutPercentage: cutPercentage)
   }

   init() {
   }
}

// Popsycl NFT Marketplace
// Marketplace smart contract
// Version         : 0.0.1
// Blockchain      : Flow www.onFlow.org
// Owner           : Popsycl.com 
// Developer       : RubiconFinTech.com

// FlowToken FUNGIBLE TOKEN mainnet
import FungibleToken from 0xf233dcee88fe0abe

import Popsycl from 0x456d627c3ac86487

// This contract allows users to list their Popsycl NFT for sale. 
// Other users can purchase these NFTs with FLOW token.

pub contract PopsyclMarketplace {

  // Event that is emitted when a new Popnow NFT is put up for sale
  pub event ForSale(id: UInt64, price: UFix64)

  // Event that is emitted when the NFT price changes
  pub event PriceChanged(id: UInt64, newPrice: UFix64)

  // Event that is emitted when a NFT token is purchased
  pub event TokenPurchased(id: UInt64, price: UFix64)

  // Event that is emitted when a seller withdraws their NFT from the sale
  pub event SaleWithdrawn(id: UInt64)

  // Market operator Address 
  pub var marketAddress : Address
  // Market operator fee percentage 
  pub var marketFee : UFix64
  /// Path where the `SaleCollection` is stored
  pub let marketStoragePath: StoragePath

  /// Path where the public capability for the `SaleCollection` is
  pub let marketPublicPath: PublicPath
    

  // Interface public methods that are used for NFT sales actions

  pub resource interface SalePublic {
    pub fun purchase(id: UInt64, recipient: &{Popsycl.PopsyclCollectionPublic}, payment: @FungibleToken.Vault)
    pub fun purchaseGroup(tokens: [UInt64], recipient: &{Popsycl.PopsyclCollectionPublic}, payment: @FungibleToken.Vault)
    pub fun idPrice(id: UInt64): UFix64?
    pub fun tokenCreator(id: UInt64): Address??
    pub fun currentTokenOwner(id: UInt64): Address??
    pub fun isResale(id: UInt64): Bool?
    pub fun getIDs(): [UInt64]
  }

  // SaleCollection
  //
  // NFT Collection object that allows a user to list their NFT for sale
  // where others can send fungible tokens to purchase it
  //
  pub resource SaleCollection: SalePublic {

    // Dictionary of the NFTs that the user is putting up for sale
    access(self) let forSale: @{UInt64: Popsycl.NFT}

    // Dictionary of the prices for each NFT by ID
    access(self) let prices: {UInt64: UFix64}

    // Dictionary of the prices for each NFT by ID
    access(self) let creators: {UInt64: Address?}

    // Dictionary of the prices for each NFT by ID
    access(self) let influencer: {UInt64: Address?}

    // Dictionary of sale type (either sale/resale)
    access(self) let resale: {UInt64: Bool}

    access(self) let roylty: {UInt64: UFix64}

    // Dictionary of token owner
    access(self) let seller : {UInt64: Address?}

    // The fungible token vault of the owner of this sale.
    // When someone buys a token, this resource can deposit
    // tokens into their account.
    access(account) let ownerVault: Capability<&AnyResource{FungibleToken.Receiver}>

    init (vault: Capability<&AnyResource{FungibleToken.Receiver}>) {
        self.forSale <- {}
        self.ownerVault = vault
        self.prices = {}
        self.creators = {}
        self.influencer = {}
        self.resale = {}
        self.roylty= {}
        self.seller = {}
    }

    // Withdraw gives the owner the opportunity to remove a NFT from sale
    pub fun withdraw(id: UInt64): @Popsycl.NFT {
        // remove the price
        self.prices.remove(key: id)
        self.creators.remove(key: id)
        self.resale.remove(key: id)
        self.seller.remove(key: id)
        // remove and return the token
        let token <- self.forSale.remove(key: id) ?? panic("missing NFT")
        return <-token
    }
     
    // List NFTs in market collection
    pub fun listForSaleGroup(sellerRef: &Popsycl.Collection, tokens: [UInt64], price: UFix64) {
       
        pre {
            tokens.length > 0:
                "No Token found. Please provide tokens!"
            self.tokensExist(tokens: tokens, ids: sellerRef.getIDs()):
                "Token is not available!"
        }

        var count = 0
        while count < tokens.length {
            let token <- sellerRef.withdraw(withdrawID: tokens[count]) as! @Popsycl.NFT
            self.listForSale(token: <- token, price: price)
            count = count + 1
        }
    }

    // lists an NFT for sale in this collection
    pub fun listForSale(token: @Popsycl.NFT, price: UFix64) {
        let id = token.id

        // Store the price in the price array
       self.prices[id] = price

        self.creators[id] = token.creator

        self.influencer[id] = token.influencer

        self.roylty[id] = token.royality

        self.seller[id] = self.owner?.address

        self.resale[id] = (token.creator == self.owner?.address) ? false : true

        // Put NFT into the forSale dictionary
        let oldToken <- self.forSale[id] <- token
        destroy oldToken

        emit ForSale(id: id, price: price)
    }

    // Change price of a group of tokens
    pub fun changePrice(tokens: [UInt64],newPrice: UFix64) {        
        pre {
            tokens.length > 0:
                "Please provide tokens!"
            self.tokensExist(tokens:tokens, ids: self.forSale.keys):
                "Tokens not available"
        }
        var count = 0
        while count < tokens.length {
            self.prices[tokens[count]] = newPrice
            emit PriceChanged(id: tokens[count], newPrice: newPrice)
            count = count + 1
        }
    }

    pub fun tokensExist(tokens: [UInt64], ids: [UInt64]):Bool {
        var count = 0
        // let ids = self.forSale.keys
        while count < tokens.length {
            if(ids.contains(tokens[count])) {
                count = count + 1
            } else {
                return false
            } 
        }
        return true
    }

    pub fun tokensTotalPrice(tokens: [UInt64]):UFix64 {
        var count = 0
        var price:UFix64 = 0.0 
        while count < tokens.length {
            price = price + self.prices[tokens[count]]!
            count = count + 1
        }
        return price
    }

    // Purchase : Allows am user send FLOW to purchase a NFT that is for sale
    pub fun purchaseGroup(tokens: [UInt64], recipient: &{Popsycl.PopsyclCollectionPublic}, payment: @FungibleToken.Vault) {
       
        pre {
            tokens.length > 0:
                "Please provide tokens!"
            self.tokensExist(tokens: tokens, ids: self.forSale.keys):
                "Tokens are not available"
            payment.balance >= self.tokensTotalPrice(tokens:tokens) :
                "Not enough FLOW to purchase this NFT token"
        }

        let totalTokens = UFix64(tokens.length)

        var count = 0
        while count < tokens.length {
           let tokenCut <- payment.withdraw(amount:  self.prices[tokens[count]]!)
           self.purchase(id: tokens[count], recipient:recipient, payment: <-tokenCut)
           count = count + 1
        }
        
        let PopsyclvaultRef =  getAccount(PopsyclMarketplace.marketAddress)
                                  .getCapability(/public/flowTokenReceiver)
                                  .borrow<&{FungibleToken.Receiver}>()
                                  ?? panic("failed to borrow reference to PopsyclMarketplace vault")

        PopsyclvaultRef.deposit(from: <-payment)
    }

    // Purchase: Allows an user to send FLOW to purchase a NFT that is for sale
    pub fun purchase(id: UInt64, recipient: &{Popsycl.PopsyclCollectionPublic}, payment: @FungibleToken.Vault) {
        pre {
            self.forSale[id] != nil && self.prices[id] != nil:
                "No token matching this ID for sale!"
            payment.balance >= self.prices[id]! :
                "Not enough FLOW to purchase NFT"
        }
     
        let totalamount = payment.balance

        let owner = self.influencer[id]!!
    
        // Popvalut reference
        let PopsyclvaultRef =  getAccount(PopsyclMarketplace.marketAddress)
                                  .getCapability(/public/flowTokenReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to PopsyclMarketplace vault")
        // influencer vault reference
        let influencerVaultRef =  getAccount(owner)
                               .getCapability(/public/flowTokenReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to owner vault") 
         
        let marketShare = self.flowshare(price:self.prices[id]!, commission:PopsyclMarketplace.marketFee)
        let royalityShare = self.flowshare(price:self.prices[id]!, commission: self.roylty[id]!)
        
        let marketCut <- payment.withdraw(amount: marketShare)
       let royalityCut <- payment.withdraw(amount: royalityShare)

        PopsyclvaultRef.deposit(from: <-marketCut)
        // Royalty to the original creator / minted user if this is a resale.
         influencerVaultRef.deposit(from: <-royalityCut)
         let vaultRef = self.ownerVault.borrow() ?? panic("Could not borrow reference to owner token vault")

        if(self.resale[id]!) {
         vaultRef.deposit(from: <-payment)
         } else {
            influencerVaultRef.deposit(from:  <-payment)
        }
       
        // Deposit the NFT token into the buyer's collection storage
        recipient.deposit(token: <-self.withdraw(id: id))

        self.prices[id] = nil
        self.creators[id] = nil
        self.influencer[id] = nil
        self.roylty[id] = nil
        self.resale[id] = nil
        self.seller[id] = nil

        emit TokenPurchased(id: id, price: totalamount)
    }

    pub fun flowshare(price:UFix64, commission:UFix64):UFix64 {
            return (price / 100.0 ) * commission
    }

    // Return the FLOW price of a specific token that is listed for sale.
    pub fun idPrice(id: UInt64): UFix64? {
        return self.prices[id]
    }

    pub fun isResale(id: UInt64): Bool? {
        return self.resale[id]
    }

    // Returns the owner / original minter of a specific NFT token in the sale
    pub fun tokenCreator(id: UInt64): Address?? {
        return self.creators[id]
    }

    pub fun tokenInfluencer(id: UInt64): Address?? {
        return self.influencer[id]
    }

    pub fun currentTokenOwner(id: UInt64): Address?? {
        return self.seller[id]
    }

    // getIDs returns an array of token IDs that are for sale
    pub fun getIDs(): [UInt64] {
        return self.forSale.keys
    }

    // Withdraw NFTs from PopsyclMarketplace
    pub fun saleWithdrawn(tokens : [UInt64]) {
       pre {
            tokens.length > 0:
                "Please provide NFT tokens!"
            self.tokensExist(tokens:tokens, ids: self.forSale.keys):
                "Tokens are not available"
       }

       var count = 0
       let owner = self.seller[tokens[0]]!!
       let receiver = getAccount(owner)
              .getCapability(Popsycl.PopsyclPublicPath)
              .borrow<&{Popsycl.PopsyclCollectionPublic}>()
               ?? panic("Could not get receiver reference to the NFT Collection")

       while count < tokens.length {
          // Deposit the NFT back into the seller collection
          receiver.deposit(token: <-self.withdraw(id: tokens[count]))
          emit SaleWithdrawn(id: tokens[count])
          count = count + 1
       }
    } 

    destroy() {
        destroy self.forSale
    }
  }

  // createCollection returns a new collection resource to the caller
  pub fun createSaleCollection(ownerVault: Capability<&AnyResource{FungibleToken.Receiver}>): @SaleCollection {
    return <- create SaleCollection(vault: ownerVault)
  }

  pub resource Admin {
    pub fun changeoperator(newOperator: Address, marketCommission: UFix64) {
        PopsyclMarketplace.marketAddress = newOperator
        PopsyclMarketplace.marketFee = marketCommission
    } 
  }

  init() {
    self.marketAddress = 0x2f016ec6646b4f03
    // 5% Marketplace Fee
    self.marketFee = 5.0
   
    self.marketPublicPath = /public/PopsyclNFTSale

    self.marketStoragePath = /storage/PopsyclNFTSale

    self.account.save(<- create Admin(), to:/storage/PopsyclMarketAdmin)
  } 

}

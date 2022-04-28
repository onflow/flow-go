// DisruptArt NFT Marketplace
// Marketplace smart contract
// NFT Marketplace : www.disrupt.art
// Owner           : Disrupt Art, INC.
// Developer       : www.blaze.ws
// Version         : 0.0.5
// Blockchain      : Flow www.onFlow.org


// FUSD FUNGIBLE TOKEN mainnet
// 
import FungibleToken from 0xf233dcee88fe0abe
// Testnet wallet address
import DisruptArt from 0xcd946ef9b13804c6

// This contract allows users to list their DisruptArt NFT for sale. 
// Other users can purchase these NFTs with FUSD token.

pub contract DisruptArtMarketplace {

  // Event that is emitted when a new Disruptnow NFT is put up for sale
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
  // creator royality
  pub var royality : UFix64

  /// Path where the `SaleCollection` is stored
  pub let marketStoragePath: StoragePath

  /// Path where the public capability for the `SaleCollection` is
  pub let marketPublicPath: PublicPath
    

  // Interface public methods that are used for NFT sales actions

  pub resource interface SalePublic {
    pub fun purchase(id: UInt64, recipient: &{DisruptArt.DisruptArtCollectionPublic}, payment: @FungibleToken.Vault)
    pub fun purchaseGroup(tokens: [UInt64], recipient: &{DisruptArt.DisruptArtCollectionPublic}, payment: @FungibleToken.Vault)
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
    pub var forSale: @{UInt64: DisruptArt.NFT}

    // Dictionary of the prices for each NFT by ID
    pub var prices: {UInt64: UFix64}

    // Dictionary of the prices for each NFT by ID
    pub var creators: {UInt64: Address?}

    // Dictionary of sale type (either sale/resale)
    pub var resale: {UInt64: Bool}

    // Dictionary of token owner
    pub var seller : {UInt64: Address?}

    // The fungible token vault of the owner of this sale.
    // When someone buys a token, this resource can deposit
    // tokens into their account.
    access(account) let ownerVault: Capability<&AnyResource{FungibleToken.Receiver}>

    init (vault: Capability<&AnyResource{FungibleToken.Receiver}>) {
        self.forSale <- {}
        self.ownerVault = vault
        self.prices = {}
        self.creators = {}
        self.resale = {}
        self.seller = {}
    }

    // Withdraw gives the owner the opportunity to remove a NFT from sale
    pub fun withdraw(id: UInt64): @DisruptArt.NFT {
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
    pub fun listForSaleGroup(sellerRef: &DisruptArt.Collection, tokens: [UInt64], price: UFix64) {
       
        pre {
            tokens.length > 0:
                "No Token found. Please provide tokens!"
            self.tokensExist(tokens: tokens, ids: sellerRef.getIDs()):
                "Token is not available!"
        }

        var count = 0
        while count < tokens.length {
            let token <- sellerRef.withdraw(withdrawID: tokens[count]) as! @DisruptArt.NFT
            self.listForSale(token: <- token, price: price)
            count = count + 1
        }
    }

    // lists an NFT for sale in this collection
    pub fun listForSale(token: @DisruptArt.NFT, price: UFix64) {
        let id = token.id

        // Store the price in the price array
        self.prices[id] = price

        self.creators[id] = token.creator

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

    // Purchase : Allows am user send FUSD to purchase a NFT that is for sale
    pub fun purchaseGroup(tokens: [UInt64], recipient: &{DisruptArt.DisruptArtCollectionPublic}, payment: @FungibleToken.Vault) {
       
        pre {
            tokens.length > 0:
                "Please provide tokens!"
            self.tokensExist(tokens: tokens, ids: self.forSale.keys):
                "Tokens are not available"
            payment.balance >= self.tokensTotalPrice(tokens:tokens) :
                "Not enough FUSD to purchase this NFT token"
        }

        let totalTokens = UFix64(tokens.length)

        var count = 0
        while count < tokens.length {
           let tokenCut <- payment.withdraw(amount:  self.prices[tokens[count]]!)
           self.purchase(id: tokens[count], recipient:recipient, payment: <-tokenCut)
           count = count + 1
        }
        
        let disruptartvaultRef =  getAccount(DisruptArtMarketplace.marketAddress)
                                  .getCapability(/public/fusdReceiver)
                                  .borrow<&{FungibleToken.Receiver}>()
                                  ?? panic("failed to borrow reference to DisruptArtMarketplace vault")

        disruptartvaultRef.deposit(from: <-payment)
    }


    // Purchase: Allows an user to send FUSD to purchase a NFT that is for sale
    pub fun purchase(id: UInt64, recipient: &{DisruptArt.DisruptArtCollectionPublic}, payment: @FungibleToken.Vault) {
        pre {
            self.forSale[id] != nil && self.prices[id] != nil:
                "No token matching this ID for sale!"
            payment.balance >= self.prices[id]! :
                "Not enough FUSD to purchase NFT"
        }
     
        let totalamount = payment.balance

        let owner = self.creators[id]!!

        // Disruptvalut reference
        let disruptartvaultRef =  getAccount(DisruptArtMarketplace.marketAddress)
                                  .getCapability(/public/fusdReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to DisruptArtMarketplace vault")
        // Creator vault reference
        let creatorvaultRef =  getAccount(owner)
                                 .getCapability(/public/fusdReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to owner vault") 
       
        // Seller vault reference
        let vaultRef = self.ownerVault.borrow()
            ?? panic("Could not borrow reference to owner token vault")

        let marketShare = self.flowshare(price:self.prices[id]!, commission:DisruptArtMarketplace.marketFee)
        let royalityShare = self.flowshare(price:self.prices[id]!, commission:DisruptArtMarketplace.royality)
        
        let marketCut <- payment.withdraw(amount: marketShare)
        let royalityCut <- payment.withdraw(amount: royalityShare)

        disruptartvaultRef.deposit(from: <-marketCut)
        // Royalty to the original creator / minted user if this is a resale.
        if(self.resale[id]!) {
            creatorvaultRef.deposit(from: <-royalityCut)
        } else {
            disruptartvaultRef.deposit(from: <-royalityCut)
        }

        // After Marketplace fee & royalty fee deposit the balance FUSD to seller's wallet
        vaultRef.deposit(from: <-payment)

        // Deposit the NFT token into the buyer's collection storage
        recipient.deposit(token: <-self.withdraw(id: id))

        self.prices[id] = nil
        self.creators[id] = nil
        self.resale[id] = nil
        self.seller[id] = nil

        emit TokenPurchased(id: id, price: totalamount)
    }

    pub fun flowshare(price:UFix64, commission:UFix64):UFix64 {
            return (price / 100.0 ) * commission
    }

    // Return the FUSD price of a specific token that is listed for sale.
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

    pub fun currentTokenOwner(id: UInt64): Address?? {
        return self.seller[id]
    }

    // getIDs returns an array of token IDs that are for sale
    pub fun getIDs(): [UInt64] {
        return self.forSale.keys
    }

    // Withdraw NFTs from DisruptArtMarketplace
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
              .getCapability(DisruptArt.disruptArtPublicPath)
              .borrow<&{DisruptArt.DisruptArtCollectionPublic}>()
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
    pub fun changeoperator(newOperator: Address, marketCommission: UFix64, royality: UFix64 ) {
        DisruptArtMarketplace.marketAddress = newOperator
        DisruptArtMarketplace.marketFee = marketCommission
        DisruptArtMarketplace.royality = royality
    } 
  }

  init() {
    self.marketAddress = 0x1592be4ab7835516
    // 5% Marketplace Fee
    self.marketFee = 5.0
    // 10% Royalty reward for original creater / minter for every re-sale
    self.royality = 10.0

    self.marketPublicPath = /public/DisruptArtNFTSale

    self.marketStoragePath = /storage/DisruptArtNFTSale

    self.account.save(<- create Admin(), to:/storage/DisruptArtMarketAdmin)
  } 

}

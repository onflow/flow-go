import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import Art from 0xd796ff17107bbff6
import NonFungibleToken from 0x1d7e57aa55817448

// A standard marketplace contract only hardcoded against Versus art that pay out Royalty as stored int he Art NFT

pub contract Marketplace {

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // Event that is emitted when a new NFT is put up for sale
    pub event ForSale(id: UInt64, price: UFix64, from: Address)

    pub event SaleItem(id: UInt64, seller: Address, price: UFix64, active: Bool, title: String, artist: String, edition: UInt64, maxEdition: UInt64, cacheKey: String)

    // Event that is emitted when the price of an NFT changes
    pub event PriceChanged(id: UInt64, newPrice: UFix64)
    
    // Event that is emitted when a token is purchased
    pub event TokenPurchased(id: UInt64, artId: UInt64, price: UFix64, from:Address, to:Address)

    pub event RoyaltyPaid(id:UInt64, amount: UFix64, to:Address, name:String)

    // Event that is emitted when a seller withdraws their NFT from the sale
    pub event SaleWithdrawn(id: UInt64, from:Address)

    // Interface that users will publish for their Sale collection
    // that only exposes the methods that are supposed to be public
    //
    pub resource interface SalePublic {
        pub fun purchase(tokenID: UInt64, recipientCap: Capability<&{Art.CollectionPublic}>, buyTokens: @FungibleToken.Vault)
        pub fun getSaleItem(tokenID: UInt64): MarketplaceData
        pub fun getUUIDforSaleItem(tokenID:UInt64): UInt64
        pub fun getIDs(): [UInt64]
        pub fun listSaleItems() : [MarketplaceData]
        pub fun getContent(tokenID: UInt64) : String
     }

     pub struct MarketplaceData {
        pub let id: UInt64
        pub let art: Art.Metadata
        pub let cacheKey: String
        pub let price: UFix64
        init( id: UInt64, art: Art.Metadata, cacheKey:String, price: UFix64) {
            self.art= art
            self.id=id
            self.price=price
            self.cacheKey=cacheKey
        }
    }

    // SaleCollection
    //
    // NFT Collection object that allows a user to put their NFT up for sale
    // where others can send fungible tokens to purchase it
    //
    pub resource SaleCollection: SalePublic {

        // Dictionary of the NFTs that the user is putting up for sale
        pub var forSale: @{UInt64: Art.NFT}

        // Dictionary of the prices for each NFT by ID
        pub var prices: {UInt64: UFix64}

        // The fungible token vault of the owner of this sale.
        // When someone buys a token, this resource can deposit
        // tokens into their account.
        access(account) let ownerVault: Capability<&AnyResource{FungibleToken.Receiver}>

        init (vault: Capability<&AnyResource{FungibleToken.Receiver}>) {
            self.forSale <- {}
            self.ownerVault = vault
            self.prices = {}
        }

        pub fun getUUIDforSaleItem(tokenID:UInt64): UInt64 {
           return self.forSale[tokenID]?.uuid!
        }

        pub fun getContent(tokenID: UInt64) : String {
            return self.forSale[tokenID]?.content()!
        }

				pub fun getArtType(tokenID: UInt64) : String {
					return self.forSale[tokenID]?.metadata?.type!
				}
             
        pub fun listSaleItems() : [MarketplaceData] {
          var saleItems: [MarketplaceData] = []

          for id in self.getIDs() {
            saleItems.append(self.getSaleItem(tokenID: id))
          }
          return saleItems
        }

          pub fun borrowArt(id: UInt64): &{Art.Public}? {
            if self.forSale[id] != nil {
                return &self.forSale[id] as &Art.NFT
            } else {
                return nil
            }
        }

        pub fun withdraw(tokenID: UInt64): @Art.NFT {
 
            let price=self.prices.remove(key: tokenID)
            // remove and return the token
            let token <- self.forSale.remove(key: tokenID) ?? panic("missing NFT")
           
            emit SaleWithdrawn(id: tokenID, from: self.ownerVault.address)

            emit SaleItem(id: token.id, 
              seller: self.ownerVault.address, 
              price: price ?? 0.0, 
              active: false,
              title: token.metadata.name,
              artist: token.metadata.artist,
              edition: token.metadata.edition, 
              maxEdition: token.metadata.maxEdition,
              cacheKey: token.cacheKey())

            return <-token
        }

        // listForSale lists an NFT for sale in this collection
        pub fun listForSale(token: @Art.NFT, price: UFix64) {
            emit SaleItem(id: token.id, 
              seller: self.ownerVault.address, 
              price: price, 
              active: true,
              title: token.metadata.name,
              artist: token.metadata.artist,
              edition: token.metadata.edition, 
              maxEdition: token.metadata.maxEdition,
              cacheKey: token.cacheKey())

            let id = token.id

            // store the price in the price array
            self.prices[id] = price

            // put the NFT into the the forSale dictionary
            let oldToken <- self.forSale[id] <- token
            destroy oldToken

            emit ForSale(id: id, price: price, from: self.ownerVault.address)
        }

        // changePrice changes the price of a token that is currently for sale
        pub fun changePrice(tokenID: UInt64, newPrice: UFix64) {
            self.prices[tokenID] = newPrice
            emit PriceChanged(id: tokenID, newPrice: newPrice)

            let token= self.borrowArt(id: tokenID)!
            emit SaleItem(id: token.id, 
              seller: self.ownerVault.address, 
              price: newPrice, 
              active: true,
              title: token.metadata.name,
              artist: token.metadata.artist,
              edition: token.metadata.edition, 
              maxEdition: token.metadata.maxEdition,
              cacheKey: token.cacheKey())
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        pub fun purchase(tokenID: UInt64, recipientCap: Capability<&{Art.CollectionPublic}>, buyTokens: @FungibleToken.Vault) {
            pre {
                self.forSale[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance >= (self.prices[tokenID] ?? 0.0):
                    "Not enough tokens to by the NFT!"
            }

            let recipient=recipientCap.borrow()!

            // get the value out of the optional
            let price = self.prices[tokenID]!
            
            self.prices[tokenID] = nil

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            
            let token <-self.withdraw(tokenID: tokenID)
            let artId = token.id

            for royality in token.royalty.keys {
                let royaltyData= token.royalty[royality]!
                if let wallet= royaltyData.wallet.borrow() {
                    let amount= price * royaltyData.cut
                    let royaltyWallet <- buyTokens.withdraw(amount: amount)
                    wallet.deposit(from: <- royaltyWallet)
                    emit RoyaltyPaid(id: tokenID, amount:amount, to: royaltyData.wallet.address, name:royality)
                } 
            }
            // deposit the purchasing tokens into the owners vault
            vaultRef.deposit(from: <-buyTokens)

            // deposit the NFT into the buyers collection
            recipient.deposit(token: <- token)

            emit TokenPurchased(id: tokenID, artId: artId, price: price, from: vaultRef.owner!.address, to:  recipient.owner!.address)
        }

        // idPrice returns the price of a specific token in the sale
        pub fun getSaleItem(tokenID: UInt64): MarketplaceData {

            let metadata=self.forSale[tokenID]?.metadata
            let cacheKey=self.forSale[tokenID]?.cacheKey()
            return MarketplaceData(
                id: tokenID,
                art: metadata!,
                cacheKey: cacheKey!,
                price: self.prices[tokenID]!
            )
        }

        // getIDs returns an array of token IDs that are for sale
        pub fun getIDs(): [UInt64] {
            return self.forSale.keys
        }

        destroy() {
            destroy self.forSale
        }
    }

    // createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerVault: Capability<&{FungibleToken.Receiver}>): @SaleCollection {
        return <- create SaleCollection(vault: ownerVault)
    }

    pub init() {
        self.CollectionPublicPath= /public/versusArtMarketplace
        self.CollectionStoragePath= /storage/versusArtMarketplace
    }
}
 

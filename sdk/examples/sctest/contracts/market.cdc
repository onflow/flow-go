import FungibleToken from 0x0000000000000000000000000000000000000002
import NonFungibleToken from 0x0000000000000000000000000000000000000003

// Marketplace is where users can put their NFTs up for sale with a price
// if another user sees an NFT that they want to buy,
// they can send fungible tokens that equal or exceed the buy price
// to buy the NFT.  The NFT is transferred to them when
// they make the purchase


// each user who wants to sell tokens will have a sale collection 
// instance in their account that holds the tokens that they are putting up for sale

// They will give a reference to this collection to the central contract
// that it can use to list tokens

// this is only for one type of NFT for now
// TODO: make it a marketplace that can buy and sell different classes of NFTs
//       how do I do this? Generic NFTs? Using interfaces instead of resources as argument
//       and storage types?


pub contract Market {

    pub resource SaleCollection {

        // a dictionary of the NFTs that the user is putting up for sale
        pub var forSale: @{Int: NonFungibleToken.NFT}

        // dictionary of the prices for each NFT by ID
        pub var prices: {Int: Int}

        // the fungible token vault of the owner of this sale
        // so that when someone buys a token, this resource can deposit
        // tokens in their account
        pub let ownerVault: &FungibleToken.Receiver

        init (vault: &FungibleToken.Receiver) {
            self.forSale <- {}
            self.ownerVault = vault
            self.prices = {}
        }

        // withdraw gives the owner the opportunity to remove a sale from the collection
        pub fun withdraw(tokenID: Int): @NonFungibleToken.NFT {
            // remove the price
            self.prices.remove(key: tokenID)
            // remove and return the token
            let token <- self.forSale.remove(key: tokenID) ?? panic("missing NFT")
            return <-token
        }

        // listForSale lists an NFT for sale in this collection
        pub fun listForSale(token: @NonFungibleToken.NFT, price: Int) {
            let id: Int = token.id

            self.prices[id] = price

            let oldToken <- self.forSale[id] <- token
            destroy oldToken
        }

        // changePrice changes the price of a token that is currently for sale
        pub fun changePrice(tokenID: Int, newPrice: Int) {
            self.prices[tokenID] = newPrice
        }

        // purchase lets a user send tokens to purchase an NFT that is for sale
        pub fun purchase(tokenID: Int, recipient: &NonFungibleToken.NFTCollection, buyTokens: @FungibleToken.Receiver) {
            pre {
                self.forSale[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance >= (self.prices[tokenID] ?? 0):
                    "Not enough tokens to by the NFT!"
            }

            // deposit the purchasing tokens into the owners vault
            self.ownerVault.deposit(from: <-buyTokens)

            // deposit the NFT into the buyers collection
            recipient.deposit(token: <-self.withdraw(tokenID: tokenID))

        }

        // idPrice returns the price of a specific token in the sale
        pub fun idPrice(tokenID: Int): Int? {
            let price = self.prices[tokenID]
            return price
        }

        // getIDs returns an array of token IDs that are for sale
        pub fun getIDs(): [Int] {
            return self.forSale.keys
        }

        destroy() {
            destroy self.forSale
        }

        // createCollection returns a new collection resource to the caller
        pub fun createCollection(ownerVault: &FungibleToken.Receiver): @SaleCollection {
            return <- create SaleCollection(vault: ownerVault)
        }
    }

    // createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerVault: &FungibleToken.Receiver): @SaleCollection {
        return <- create SaleCollection(vault: ownerVault)
    }

    // Marketplace would be the central contract where people can post their sale
    // references so that anyone can access them
    // It is just an example and hasn't been tested so don't take it seriously
    pub resource Marketplace {
        // Data structure to store active sales
        pub var tokensForSale: [&SaleCollection]

        init() {
            self.tokensForSale = []
        }

        // listSaleCollection lists a users sale reference in the array
        // and returns the index of the sale so that users can know
        // how to remove it from the marketplace
        pub fun listSaleCollection(collection: &SaleCollection): Int {
            self.tokensForSale.append(collection)
            return (self.tokensForSale.length - 1)
        }

        // removeSaleCollection removes a user's sale from the array
        // of sale references
        pub fun removeSaleCollection(index: Int) {
            self.tokensForSale.remove(at: index)
        }

    }
}

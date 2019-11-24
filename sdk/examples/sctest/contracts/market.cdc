import Receiver, Provider, Vault, createVault from 0x0000000000000000000000000000000000000002

import INFT, NFT, INFTCollection, NFTCollection from 0x0000000000000000000000000000000000000003

// import Receiver, Provider, Vault, createVault from "fungible-token.cdc"

// import NFT, NFTCollection, createNFT, createCollection from "nft.cdc"
// importing from nft.cdc isn't working for some reason

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
//       how do I do this?


pub resource SaleCollection {

    pub var forSale: <-{Int: NFT}

    pub let ownerVault: &Receiver

    pub var prices: {Int: Int}

    init (vault: &Receiver) {
        self.forSale = {}
        self.ownerVault = vault
        self.prices = {}
    }

    pub fun withdraw(tokenID: Int): <-NFT {
        self.prices.remove(key: tokenID)
        let token <- self.forSale.remove(key: tokenID) ?? panic("missing NFT")
        return <-token
    }

    pub fun listForSale(token: <-NFT, price: Int) {
        let id: Int = token.id

        self.prices[id] = price

        let oldToken <- self.forSale[id] <- token
        destroy oldToken
    }

    pub fun purchase(tokenID: Int, recipient: &NFTCollection, buyTokens: <-Receiver) {
        pre {
            self.forSale[tokenID] != nil:
                "No token matching this ID for sale!"
        }
        let price = self.prices[tokenID] ?? panic("missing price!")
        if buyTokens.balance < price {
            panic("Not enough tokens to buy the NFT!")
        }

        self.ownerVault.deposit(from: <-buyTokens)

        recipient.deposit(token: <-self.withdraw(tokenID: tokenID))

    }

    pub fun idPrice(tokenID: Int): Int {
        let price = self.prices[tokenID] ?? panic("no price!")
        return price
    }

    pub fun getIDs(): [Int] {
        return self.forSale.keys
    }

    destroy() {
        destroy self.forSale
    }

    // createCollection returns a new collection resource to the caller
    pub fun createCollection(ownerVault: &Receiver): <-SaleCollection {
        return <- create SaleCollection(vault: ownerVault)
    }
}

// createCollection returns a new collection resource to the caller
pub fun createCollection(ownerVault: &Receiver): <-SaleCollection {
    return <- create SaleCollection(vault: ownerVault)
}

// pub resource Marketplace {
//     // Data structure to store active sales
//     var tokensForSale: [&SaleCollection]

//     init() {
//         self.tokensForSale = []
//     }

//     // pub fun listSaleCollection(collection: &SaleCollection) {

//     // }

// }
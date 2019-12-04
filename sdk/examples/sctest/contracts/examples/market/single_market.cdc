// import Vault, createVault from 0x0000000000000000000000000000000000000002

// import INFT, NFT, INFTCollection, NFTCollection from 0x0000000000000000000000000000000000000003

import Receiver, Provider, Vault, createVault from "fungible-token.cdc"

// import NFT, NFTCollection, createNFT, createCollection from "nft.cdc"
pub resource interface INFT {
    // The unique ID that each NFT has
    pub let id: Int
}


// Making a single sale resource for each NFT that holds the a generic NFT, the price, the 
// currency type, and a buy method, and an event
// Then the owner can pass references to these to a central marketplace contract
// or an app could just read their sales from their account when they log in and see events

// which would list the references as sales
// How would these be stored in an account?
    // in a collection or array
        // how to give a reference to an object stored within another object
        // how to delete the object from the parent object once it is sold

// How can we differentiate between different types of resources?
// between different fungible token classes or different NFT classes
// people will want to buy and sell with whatever currency they want

// need a way to let users put generic NFTs or FTs in here that conform to
// the interface but to enforce that buyers also are using the 
// same token type

// and for apps and users to be able to query this to find out what 
// token types are being used so they can send the correct ones


pub resource Sale {
    pub var token: <-INFT

    pub var price: Int

    pub var vault: &Receiver

    pub var collection: &INFTCollection

    init(token: <-INFT, price: Int, vault: &Receiver, collection: &INFTCollection) {
        // no way to enforce that token and collection are of the same type
        self.token <- token
        self.price = 0
        self.vault = vault
        self.collection = collection
    }

    // no way to enforce that receipient INFTCollection is the same nft type as self.token
    // also no way to enforce that buyTokens is the same type as self.vault
    pub fun purchase(recipient: &INFTCollection, buyTokens: <-Receiver) {
        if buyTokens.balance < self.price {
            panic("Not enough tokens to buy the NFT!")
        }

        self.vault.deposit(from: <-buyTokens)

        recipient.deposit(token: <-self.token)
    }


    destroy() {
        self.collection.deposit(token: <-self.token)
    }
}
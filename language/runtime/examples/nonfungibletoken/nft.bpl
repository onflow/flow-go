
// TODO:  Need to talk about how NFTs would be stored
// Cant store it like a Fungible token because we have a separate resource for each NFT

// so we'd have to store a new resource every transfer but another account can't write to 
// an account's storage

// could have each account implement a contract or high level resource that stores NFTs
// and have people call methods on that to deposit



pub contract interface NonFungibleToken {

    // shows the total supply of NFTs from this contract
    // and is used to give the NFTs their ID
    pub totalSupply: Int {
        set(newSupply) {
            post {
                newSupply >= 0:
                    "Supply is always non-negative"
            }
        }
    }

    resource interface NFT {

        // The unique ID that each NFT has
        pub let id: Int {
            set(newID) {
                post {
                    newID >= 0:
                        "token ID is always positive"
                }
            }
        }

        // transfer this NFT to another account
        owner fun transfer(recipient: &NFTCollection): {
            pre {
                recipient != Address(0):
                    "Cannot send to the zero address"
            }
        }
    }
    

    pub mint(): newToken: <-NFT {
        post {
            (newToken.id > before.totalSupply):
                "Cannot reuse an existing token ID!"
        }
    }

    // related functionality that might work with the token resources
    pub fun absorb(token: <-NFT) {
        pre {
            token.balance == 0:
                "Can only destroy empty TokenPools"
        }
    }
}

// possibility for each account with NFTs to have a copy of this resource that they keep their NFTs in
// they could send one NFT, multiple at a time, or potentially even send the entire collection in one go?
resource interface INFTCollection {

    // variable size array of NFT conforming tokens
    pub var ownedNFTs: {Int: NFT}

    owner fun transfer(recipient: &NFTCollection, tokenID: Int): Void {
        pre {
            ownedNFTs[tokenID] != nil
                "Token ID to transfer does not exist!"
        }
    }

    pub fun deposit(newToken: <-NFT): Void {
        // add the new token to the array
        ownedNFTs[newToken.tokenID] <- newToken
    }
}
resource NFTCollection { 
    // variable size array of NFT conforming tokens
    pub var ownedNFTs: {Int: NFT}

    owner fun transfer(recipient: &NFTCollection, tokenID: Int): Void {
        // remove the token from the array
        let sentNFT <- ownedNFTs[tokenID]

        // deposit it in the recipient's account
        recipient.deposit(<-recipient)
    }

    pub fun deposit(newToken: <-NFT): Void {
        // add the new token to the array
        ownedNFTs[newToken.tokenID] <- newToken
    }
}




contract JoshCollectibles: NonFungibleToken {

    pub var totalSupply: Int

    pub resource NFT: NFT {
        pub let id: Int

        owner fun transfer(recipient: Address): {

            // still needs to be discussed
            recipient.send

        }
    }

    constructor() {
        totalSupply = 0
    }

}
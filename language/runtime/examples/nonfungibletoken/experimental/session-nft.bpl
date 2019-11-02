
// TODO:  Need to talk about how NFTs would be stored
// Cant store it like a Fungible token because we have a separate resource for each NFT

// so we'd have to store a new resource every transfer but another account can't write to 
// an account's storage

// could have each account implement a contract or high level resource that stores NFTs
// and have people call methods on that to deposit



resource interface INFT {

    // The unique ID that each NFT has
    pub let id: Int //{
    //     set(newID) {
    //         post {
    //             newID >= 0:
    //                 "token ID is always positive"
    //         }
    //     }
    // }

    // transfer this NFT to another account
    // pub fun transfer(recipient: &NFTCollection): {
    //     pre {
    //         recipient != Address(0):
    //             "Cannot send to the zero address"
    //     }
    // }
}

pub resource NFT: INFT {
    pub let id: Int

    init(newID: Int) {
        self.id = newID
    }
}

// possibility for each account with NFTs to have a copy of this resource that they keep their NFTs in
// they could send one NFT, multiple at a time, or potentially even send the entire collection in one go?
resource interface INFTCollection {

    // variable size array of NFT conforming tokens
    pub var ownedNFTs: <-{Int: NFT}

    //pub fun transfer(recipient: &NFTCollection, tokenID: Int) {}
        // pre {
        //     self.ownedNFTs[tokenID] != nil:
        //         "Token ID to transfer does not exist!"
        // }
    // }

    // pub fun deposit(token: <-NFT): Void {}
        // add the new token to the array
    // }
}

resource NFTCollection: INFTCollection { 
    // variable size array of NFT conforming tokens
    pub var ownedNFTs: <-{Int: NFT}

    init(firstToken: <-NFT) {
        self.ownedNFTs <- {firstToken.id: <-firstToken}
    }

    pub fun transfer(recipient: &NFTCollection, tokenID: Int): Void {
        // remove the token from the array
        let sentNFT <- self.ownedNFTs[tokenID]

        // deposit it in the recipient's account
        recipient.deposit(token: <-sentNFT)

        destroy self.ownedNFTs[tokenID]
    }

    pub fun deposit(token: <-NFT?): Void {
        // add the new token to the array
        self.ownedNFTs[token.tokenID] <- token
    }
}



fun main() {

    let tokenA <- create NFT(newID: 1)
    let collectionA <- create NFTCollection(firstToken: <-tokenA)

    let tokenB <- create NFT(newID: 2)
    let collectionB <- create NFTCollection(firstToken: <-tokenB)


    destroy collectionA
    destroy collectionB

}
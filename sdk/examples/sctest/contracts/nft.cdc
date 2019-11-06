

resource interface INFT {

    // The unique ID that each NFT has
    pub let id: Int

    init(newID: Int) {
        pre {
            newID > 0:
                "NFT ID needs to be positive!"
        }
    }
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

    // pub fun transfer(recipient: &NFTCollection, tokenID: Int) {
    //     pre {
    //         self.ownedNFTs[tokenID] != nil:
    //             "Token ID to transfer does not exist!"
    //     }
    // }

    pub fun deposit(token: <-NFT?, id: Int): Void {
        pre {
            id >= 0:
                "ID cannot be negative"
        }
    }
}

resource NFTCollection: INFTCollection { 
    // variable size array of NFT conforming tokens
    // NFT is a resource type with an `Int` ID field
    pub var ownedNFTs: <-{Int: NFT}

    init(firstToken: <-NFT) {
        self.ownedNFTs <- {firstToken.id: <-firstToken}
    }

    pub fun withdraw(tokenID: Int): <-NFT? {
        return <-self.ownedNFTs.remove(key: tokenID)
    }

    pub fun deposit(token: <-NFT?, id: Int): Void {
        var newToken <- token

        // add the new token to the array
        // 
        self.ownedNFTs[id] <-> newToken

        destroy newToken
    }

    pub fun idExists(tokenID: Int): Bool {
        var token: <-NFT? <- self.ownedNFTs.remove(key: tokenID)
        var exists = false

        if token != nil {
            exists = true
        }

        self.ownedNFTs[tokenID] <-> token

        destroy token

        return exists
    }

    destroy() {
        destroy self.ownedNFTs
    }
}

fun createNFT(id: Int): <-NFT {
    return <- create NFT(newID: id)
}

fun createCollection(token: <-NFT): <-NFTCollection {
    return <- create NFTCollection(firstToken: <-token)
}


// fun main() {

//     let tokenA <- create NFT(newID: 1)
//     let collectionA <- create NFTCollection(firstToken: <-tokenA)

//     let tokenB <- create NFT(newID: 2)
//     let collectionB <- create NFTCollection(firstToken: <-tokenB)

//     collectionA.transfer(token: &collectionB as NFTCollection, tokenID: 1)


//     destroy collectionA
//     destroy collectionB

// }


    // takes a reference to another user's NFT collection,
    // takes the NFT out of this collection, and deposits it
    // in the reference's collection
    // pub fun transfer(recipient: &NFTCollection, tokenID: Int): Void {
    //     // remove the token from the array
    //     //
    //     let sentNFT: <-NFT? <- self.ownedNFTs.remove(key: tokenID)

    //     // deposit it in the recipient's account
    //     recipient.deposit(token: <-sentNFT, id: tokenID)

    // }

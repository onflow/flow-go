

resource interface INFT {
    // The unique ID that each NFT has
    pub let id: Int
}

pub resource NFT: INFT {
    pub let id: Int

    init(newID: Int) {
        pre {
            newID > 0:
                "NFT ID must be positive!"
        }
        self.id = newID
    }
}

// possibility for each account with NFTs to have a copy of this resource that they keep their NFTs in
// they could send one NFT, multiple at a time, or potentially even send the entire collection in one go
resource interface INFTCollection {

    // dictionary of NFT conforming tokens
    pub var ownedNFTs: <-{Int: NFT}

    pub fun transfer(recipient: &NFTCollection, tokenID: Int) {
        pre {
            self.ownedNFTs[tokenID] != nil:
                "Token ID to transfer does not exist!"
        }
    }

    pub fun withdraw(tokenID: Int): <-NFT

    pub fun deposit(token: <-NFT): Void {
        pre {
            token.id >= 0:
                "ID cannot be negative"
        }
    }
}

resource NFTCollection: INFTCollection { 
    // dictionary of NFT conforming tokens
    // NFT is a resource type with an `Int` ID field
    pub var ownedNFTs: <-{Int: NFT}

    // an array to hold the IDs of all the tokens in the collection
    pub var idArray: [Int]

    // initializer overloading is not supported yet
    // init () {
    //     self.idArray = []
    //     self.ownedNFTs = {}
    // }

    init(firstToken: <-NFT) {
        self.idArray = [firstToken.id]
        self.ownedNFTs <- {firstToken.id: <-firstToken}
    }

    // withdraw removes an NFT from the collection and moves it to the caller
    pub fun withdraw(tokenID: Int): <-NFT {
        let token <- self.ownedNFTs.remove(key: tokenID) ?? panic("missing NFT")

        if !self.idExists(tokenID: tokenID) {
            panic("missing ID in collection")
        }

        // find and remove the ID from the ID array
        var i = 0
        while i < self.idArray.length {
            if self.idArray[i] == tokenID {
                self.idArray.remove(at: i)
                break
            }
            i = i + 1
        }
            
        return <-token
    }

    // deposit takes a NFT and adds it to the collections dictionary
    // and adds the ID to the id array
    pub fun deposit(token: <-NFT): Void {
        let id: Int = token.id

        // add the id to the array
        self.idArray.append(id)

        // add the new token to the dictionary which removes the old one
        let oldToken <- self.ownedNFTs[id] <- token

        destroy oldToken
    }

    // transfer takes a reference to another user's NFT collection,
    // takes the NFT out of this collection, and deposits it
    // in the reference's collection
    pub fun transfer(recipient: &NFTCollection, tokenID: Int): Void {

        // remove the token from the dictionary get the token from the optional
        let token <- self.withdraw(tokenID: tokenID)

        // deposit it in the recipient's account
        recipient.deposit(token: <-token)
    }

    // idExists checks to see if a NFT with the given ID exists in the collection
    pub fun idExists(tokenID: Int): Bool {
        if self.idArray.length == 0 {
            return false
        }

        //var exists = false
        var i = 0

        while i < self.idArray.length {
            if self.idArray[i] == tokenID {
                return true
            }
            i = i + 1
        }

        return false
    }

    // getOwnedNFTs returns an array of the IDs that are in the collection
    pub fun getOwnedNFTs(): [Int] {
        return self.idArray
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


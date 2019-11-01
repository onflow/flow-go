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
    
    pub mint(): newToken: <-NFT {
        post {
            (newToken.id > before(totalSupply)):
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
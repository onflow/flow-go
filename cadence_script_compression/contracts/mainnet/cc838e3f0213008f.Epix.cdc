import NonFungibleToken from 0x1d7e57aa55817448

// Epix NFT Smart contract 
//
pub contract Epix: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Burn(id: UInt64)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, metadata: String, claimsSize: Int)
    pub event Claimed(id: UInt64)

    // The total number of tokens of this type in existence
    pub var totalSupply: UInt64

    // Named paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // Composite data structure to represents, packs and upgrades functionality
    pub struct NFTData {
        pub let metadata: String
        pub let claims: [NFTData]
        init(metadata: String, claims: [NFTData]) {
            self.metadata = metadata
            self.claims = claims
        }
    }

    // NFT
    // A Epix NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // NFT's ID
        pub let id: UInt64
        // NFT's data
        pub let data: NFTData

        // initializer
        //
        init(initID: UInt64, initData: NFTData) {
            self.id = initID
            self.data = initData
        }

        destroy() {
            emit Burn(id: self.id)
        }
    }

    pub resource interface EpixCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowEpixNFT(id: UInt64): &Epix.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result != nil) && (result?.id != id):
                    "Cannot borrow EpixCard reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Epix NFTs owned by an account
    //
    pub resource Collection: EpixCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Epix.NFT
            let id: UInt64 = token.id
            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowEpixNFT
        // Gets a reference to an NFT in the collection as a EpixCard,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the Epix.
        //
        pub fun borrowEpixNFT(id: UInt64): &Epix.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Epix.NFT
            } else {
                return nil
            }
        }

        // claim
        // resource owners when claiming, Mint new NFTs and burn the claimID resource.
        pub fun claim(claimID: UInt64) {
            pre {
                self.ownedNFTs[claimID] != nil : "missing claim NFT"
            }

            let claimTokenRef = (&self.ownedNFTs[claimID] as auth &NonFungibleToken.NFT) as! &Epix.NFT
            if claimTokenRef.data.claims.length == 0 {
                panic("Claim NFT has empty claims")
            }
            
            for claim in claimTokenRef.data.claims {
                Epix.totalSupply = Epix.totalSupply + (1 as UInt64)
                emit Minted(id: Epix.totalSupply, metadata: claim.metadata, claimsSize: claim.claims.length)
                // deposit it in the recipient's account using their reference
                self.deposit(token: <-create Epix.NFT(initID: Epix.totalSupply, initData: claim))
            }

            let claimToken <- self.ownedNFTs.remove(key: claimID) ?? panic("missing claim NFT")
            destroy claimToken
            emit Claimed(id: claimID)
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter {
        // mintNFT
        // Mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, data: NFTData) {
            Epix.totalSupply = Epix.totalSupply + (1 as UInt64)
            emit Minted(id: Epix.totalSupply, metadata: data.metadata, claimsSize: data.claims.length)
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create Epix.NFT(initID: Epix.totalSupply, initData: data))
        }
    }

    // initializer
    //
    init() {
        self.totalSupply = 0
        
        self.CollectionStoragePath = /storage/EpixCollection
        self.CollectionPublicPath = /public/EpixCollection
        self.MinterStoragePath = /storage/EpixMinter

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

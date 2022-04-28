// This is a complete version of the TicketMaster Business TMB2B contract

// It includes withdraw and deposit functionality, as well as a
// collection resource that can be used to bundle NFTs together.

// It also includes a definition for the Minter resource,
// which can be used by admins to mint new NFTs.


import NonFungibleToken from 0x1d7e57aa55817448

pub contract TMB2B : NonFungibleToken{
    
    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath


    // Declare the NFT resource type
    pub resource NFT : NonFungibleToken.INFT{
        // The unique ID that differentiates each NFT
        // The ID is used to reference the NFT in the collection, and is guaranteed to be unique with bit-shifting of unique batch-sequence-limit combination
        
        // The ID is stored in the collection as a 64 bit unsigned integer
        pub let id: UInt64
        pub var link: String
        pub var batch: UInt32
        pub var sequence: UInt16
        pub var limit: UInt16
        
        // Initialize both fields in the init function
        init(initID: UInt64, initlink: String, initbatch: UInt32, initsequence: UInt16, initlimit: UInt16) {
            self.id = initID //token id
            self.link = initlink
            self.batch = initbatch
            self.sequence=initsequence
            self.limit=initlimit
            
        }
    }

    // Declare the collection resource type
    pub resource interface TMB2BCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTMB2B(id: UInt64): &TMB2B.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow TMB2B reference: The ID of the returned reference is incorrect"
            }
        }
    }


    // The definition of the Collection resource that
    // holds the NFTs that a user owns
    pub resource Collection: TMB2BCollectionPublic,NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // Initialize the NFTs field to an empty collection
        init () {
            self.ownedNFTs <- {}
        }

        // withdraw 
        //
        // Function that removes an NFT from the collection 
        // and moves it to the calling context
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // If the NFT isn't found, the transaction panics and reverts
            let token <- self.ownedNFTs.remove(key: withdrawID)!
            
            // emit the withdraw event
            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit 
        //
        // Function that takes a NFT as an argument and 
        // adds it to the collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {
            // add the new token to the dictionary with a force assignment
            // if there is already a value at that key, it will fail and revert
            

            // Rhea comment- make sure to cast the received NonFungibleToken.NFT to your concrete NFT type
            let token <- token as! @TMB2B.NFT
 
            
            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
            
            
        }

        // idExists checks to see if a NFT 
        // with the given ID exists in the collection
        //pub fun idExists(id: UInt64): Bool {
        //    return self.ownedNFTs[id] != nil
        //}

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT  
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        destroy() {
            destroy self.ownedNFTs
        }

        pub fun borrowTMB2B(id: UInt64): &TMB2B.NFT? {
            if self.ownedNFTs[id] == nil {
                return nil
            } 
            else {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TMB2B.NFT
            }
        }

    }

    // creates a new empty Collection resource and returns it 
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // NFTMinter
    // Resource that would be owned by an admin or by a smart contract 
    // that allows them to mint new NFTs when needed
    pub resource NFTMinter {

        // the ID that is used to mint NFTs
        // it is only incremented so that NFT ids remain
        // unique. It also keeps track of the total number of NFTs
        // in existence
        pub var minterID: UInt64
        
        init() {
            self.minterID = 0    
        }

        // mintNFT mints a new NFT with the given batch, sequence and limit combination, by creating a UNIQUE ID
        // Function that mints a new NFT with a new ID
        // and returns it to the caller
        pub fun mintNFT(glink: String, gbatch: UInt32, glimit: UInt16, gsequence:UInt16): @NFT {

            // create a new NFT
            // Cadence does not allow applying binary operation << to types: `UInt16`, `UInt32`, hence, a small typecasting trick, recommended by FLOW team
            let tokenID = (UInt64(gbatch) << 32) | (UInt64(glimit) << 16) | UInt64(gsequence)
            var newNFT <- create NFT(initID: tokenID, initlink: glink, initbatch: gbatch, initsequence: gsequence, initlimit: glimit)

            // Set the id so that each ID is unique from this minter, ensuring unique ID combination for each asset with NFT.Kred standard
            self.minterID= tokenID
            
            //increase total supply
            TMB2B.totalSupply = TMB2B.totalSupply + UInt64(1)
            return <-newNFT
        }
    }

	init() {

        // Set our named paths
        
        
        self.CollectionStoragePath = /storage/TMB2BCollection
        self.CollectionPublicPath = /public/TMB2BCollection
        self.MinterStoragePath = /storage/TMB2BMinter


        // Initialize the total supply
        self.totalSupply = 0

		// store an empty NFT Collection in account storage
        self.account.save(<-self.createEmptyCollection(), to: self.CollectionStoragePath)

        // publish a reference to the Collection in storage
        self.account.link<&{NonFungibleToken.CollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // store a minter resource in account storage
        self.account.save(<-create NFTMinter(), to: self.MinterStoragePath)
	}
}
 
 
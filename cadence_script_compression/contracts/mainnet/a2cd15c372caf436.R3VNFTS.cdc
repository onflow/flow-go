// Join us by visiting https://r3volution.io
pub contract R3VNFTS {

    pub event NFTMinted(id: Int, md: String)
    pub event NFTWithdraw(id: Int, md: String)
    pub event NFTDeposit(id: Int, md: String)

    // the R3V NFT resource
    pub resource NFT {
        // our unique NFT serial number
        pub let id: Int
        // our metadata is a hex-encoded gzipped JSON string
        pub let metadata: String
        init(id: Int, metadata: String) {
            self.id = id
            self.metadata = metadata
        }
    }

    // the NFTReceiver interface declares methods for accessing a Collection resource
    // this receiver should always be located on an initialized account at /public/RevNFTReceiver
    // example on how to obtain contained IDs:
    //
    // pub fun main(): [Int] {
    //     let nftOwner = getAccount(0x$account)
    //     let capability = nftOwner.getCapability<&{R3VNFTS.NFTReceiver}>(/public/RevNFTReceiver)
    //     let receiverRef = capability.borrow()
    //             ?? panic("Could not borrow the receiver reference")
    //     return(receiverRef.getIDs())
    // }
    //
    pub resource interface NFTReceiver {
        // deposit an @NFT into this receiver
        pub fun deposit(token: @NFT)
        // get the IDs of the NFTs this receiver stores
        pub fun getIDs(): [Int]
        // check if a specific ID is stored in this receiver
        pub fun idExists(id: Int): Bool
        // get the metadata of the provided receivers as a [string]
        pub fun getMetadata(ids: [Int]): [String]
    }

    // the Collection resource exists at /storage/RevNFTCollection
    // to setup this collection on a user, we will need to create it with a transaction
    // we should also setup the /public/RevNFTReceiver at this time
    //
    // transaction {
    //     prepare(acct: AuthAccount) {
    //         let collection <- R3VNFTS.createEmptyCollection()
    //         acct.save<@R3VNFTS.Collection>(<-collection, to: /storage/RevNFTCollection)
    //         acct.link<&{R3VNFTS.NFTReceiver}>(/public/RevNFTReceiver, target: /storage/RevNFTCollection)
    //     }
    // }
    //
    pub resource Collection: NFTReceiver {
        // map<id, NFT>
        pub var ownedNFTs: @{Int: NFT}
        init () {
            self.ownedNFTs <- {}
        }

        // withdrawal NFT from the Collection.ownedNFTs map
        pub fun withdraw(withdrawID: Int): @NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID)!
            emit NFTWithdraw(id: token.id, md: token.metadata)
            return <-token
        }
        
        // deposit NFT into this Collection.ownedNFTs map
        pub fun deposit(token: @NFT) {
            emit NFTDeposit(id: token.id, md: token.metadata)
            self.ownedNFTs[token.id] <-! token
        }

        // returns a Bool on whether the provided ID exists within this Collection.ownedNFTs.keys
        pub fun idExists(id: Int): Bool {
            return self.ownedNFTs[id] != nil
        }

        // gets all keys within this Collection.ownedNFTs
        pub fun getIDs(): [Int] {
            return self.ownedNFTs.keys
        }

        // get an array of strings from the provided Collection.owndeNFTs.keys
        pub fun getMetadata(ids: [Int]): [String] {
            var ret: [String] = []
            for id in ids {
                ret.append(self.ownedNFTs[id]?.metadata!)
            }
            return ret
        }

        // nuke this collection upon destruction
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // helper function to create a new collection
    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    // our minter resource: internal only
    // our minting transaction will typically look like:
    //
    // transaction(metadata: [String]) {
    //
    //     let receiverRef: &{R3VNFTS.NFTReceiver}
    //     let minterRef: &R3VNFTS.NFTMinter
    //
    //     prepare(acct: AuthAccount) {
    //         self.receiverRef = acct.getCapability<&{R3VNFTS.NFTReceiver}>(/public/RevNFTReceiver)
    //             .borrow()
    //             ?? panic("Could not borrow receiver reference")
    //         self.minterRef = acct.borrow<&R3VNFTS.NFTMinter>(from: /storage/RevNFTMinter)
    //             ?? panic("Could not borrow minter reference")
    //     }
    //
    //     execute {
    //         var i: Int = 0;
    //         while i < metadata.length {
    //             let newNFT <- self.minterRef.mintNFT(metadata: metadata[i])
    //             self.receiverRef.deposit(token: <-newNFT)
    //             i = i + 1
    //         }
    //     }
    // }
    pub resource NFTMinter {
        // NFT serial number
        pub var idCount: Int
        init() {
            // starting at serial 1
            self.idCount = 1
        }
        // mint a new NFT from this NFTMinter
        pub fun mintNFT(metadata: String): @NFT {
            var newNFT <- create NFT(id: self.idCount, metadata: metadata)
            self.idCount = self.idCount + 1
            emit NFTMinted(id: newNFT.id, md: metadata)
            return <-newNFT
        }
    }

	init() {
        self.account.save(<-self.createEmptyCollection(), to: /storage/RevNFTCollection)
        self.account.link<&{NFTReceiver}>(/public/RevNFTReceiver, target: /storage/RevNFTCollection)
        self.account.save(<-create NFTMinter(), to: /storage/RevNFTMinter)
	}
}
 
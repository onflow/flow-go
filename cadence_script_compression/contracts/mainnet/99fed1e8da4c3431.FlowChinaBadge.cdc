import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract FlowChinaBadge: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminStoragePath: StoragePath

    // totalSupply
    // The total number of FlowChinaBadge that have been minted
    //
    pub var totalSupply: UInt64

    pub resource NFT: NonFungibleToken.INFT {

        pub let id: UInt64

        // The IPFS CID of the metadata file.
        pub let metadata: String

        init(id: UInt64, metadata: String) {
            self.id = id
            self.metadata = metadata
        }
    }

    pub resource interface FlowChinaBadgeCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowFlowChinaBadge(id: UInt64): &FlowChinaBadge.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow FlowChinaBadge reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: FlowChinaBadgeCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        
        // dictionary of NFTs
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <- token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @FlowChinaBadge.NFT

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

        // borrowFlowChinaBadge
        // Gets a reference to an NFT in the collection as a FlowChinaBadge.
        //
        pub fun borrowFlowChinaBadge(id: UInt64): &FlowChinaBadge.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &FlowChinaBadge.NFT
            } else {
                return nil
            }
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

    // Admin
    // Resource that an admin can use to mint NFTs.
    //
    pub resource Admin {

        // mintNFT
        // Mints a new NFT with a new ID
        //
        pub fun mintNFT(metadata: String): @FlowChinaBadge.NFT {
            let nft <- create FlowChinaBadge.NFT(id: FlowChinaBadge.totalSupply, metadata: metadata)

            emit Minted(id: nft.id)

            FlowChinaBadge.totalSupply = FlowChinaBadge.totalSupply + (1 as UInt64)

            return <- nft
        }
    }

    // fetch
    // Get a reference to a FlowChinaBadge from an account's Collection, if available.
    // If an account does not have a FlowChinaBadge.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &FlowChinaBadge.NFT? {
        let collection = getAccount(from)
            .getCapability(FlowChinaBadge.CollectionPublicPath)!
            .borrow<&{ FlowChinaBadge.FlowChinaBadgeCollectionPublic }>()
            ?? panic("Couldn't get collection")

        // We trust FlowChinaBadge.Collection.borowFlowChinaBadge to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowFlowChinaBadge(id: itemID)
    }

    // initializer
    //
    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/FlowChinaBadgeCollection
        self.CollectionPublicPath = /public/FlowChinaBadgeCollection
        self.CollectionPrivatePath = /private/FlowChinaBadgeCollection
        self.AdminStoragePath = /storage/FlowChinaBadgeAdmin

        // Initialize the total supply
        self.totalSupply = 0

        let collection <- FlowChinaBadge.createEmptyCollection()
        
        self.account.save(<- collection, to: FlowChinaBadge.CollectionStoragePath)

        self.account.link<&FlowChinaBadge.Collection>(FlowChinaBadge.CollectionPrivatePath, target: FlowChinaBadge.CollectionStoragePath)

        self.account.link<&FlowChinaBadge.Collection{NonFungibleToken.CollectionPublic, FlowChinaBadge.FlowChinaBadgeCollectionPublic}>(FlowChinaBadge.CollectionPublicPath, target: FlowChinaBadge.CollectionStoragePath)
        
        // Create an admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<- admin, to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}

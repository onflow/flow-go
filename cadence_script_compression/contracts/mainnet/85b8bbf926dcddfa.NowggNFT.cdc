import NonFungibleToken from 0x1d7e57aa55817448


pub contract NowggNFT: NonFungibleToken {

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, typeId: String)
    pub event TypeRegistered(typeId: String)
    pub event TypeSoldOut(typeId: String)
    pub event NftDestroyed(id: UInt64)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let NftTypeHelperStoragePath: StoragePath
    pub let NFTtypeHelperPublicPath: PublicPath

    // totalSupply
    // The total number of NowggNFTs that have been minted
    pub var totalSupply: UInt64
    
    // NFT type
    // It is used to keep a check on the current NFTs minted and the total number of NFTs that can be minted
    // for a given type
    pub struct NftType {
        pub let typeId: String

        pub var currentCount: UInt64

        pub let maxCount: UInt64

        pub fun updateCount(count: UInt64) {
            self.currentCount = count
        }

        init(typeId: String, maxCount: UInt64) {

            if (NowggNFT.activeNftTypes.keys.contains(typeId)) {
                panic("Type is already registered")
            }

            self.typeId = typeId
            self.maxCount = maxCount
            self.currentCount = 0

        }
    }

    // NFT types registered which can be minted
    access(contract) var activeNftTypes: {String : NftType}

    // NFT types registered which have reached the max limit for minting
    access(contract) var historicNftTypes: {String : NftType}


    // NFT
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        access(self) let metadata: {String: AnyStruct}

        // initializer
        //
        init(initID: UInt64, metadata: {String: AnyStruct}) {
            self.id = initID
            self.metadata = metadata
        }

        // getter for metadata
        pub fun getMetadata(): {String: AnyStruct} {
            return self.metadata
        }

        destroy() {
            emit NftDestroyed(id: self.id)
        }
    }

    // This is the interface that users can cast their NowggNFTs Collection as
    // to allow others to deposit NowggNFTs into their Collection. It also allows for reading
    // the details of NowggNFTs in the Collection.
    pub resource interface NowggNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNowggNFT(id: UInt64): &NowggNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow NowggNFT reference: The ID of the returned reference is incorrect"
            }
        }
    }


    // Interface that allows other users to access details of the NFTtypes
    // by providing the IDs for them
    pub resource interface NftTypeHelperPublic {
        pub fun borrowActiveNFTtype(id: String): NftType? {
            post {
                (result == nil) || (result?.typeId == id):
                    "Cannot borrow NftType reference: The ID of the returned reference is incorrect"
            }
        }

        pub fun borrowHistoricNFTtype(id: String): NftType? {
            post {
                (result == nil) || (result?.typeId == id):
                    "Cannot borrow NftType reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of NowggItem NFTs owned by an account
    pub resource Collection: NowggNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @NowggNFT.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowNowggNFT
        // Gets a reference to an NFT in the collection as a NowggItem,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the NowggItem.
        pub fun borrowNowggNFT(id: UInt64): &NowggNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &NowggNFT.NFT
            } else {
                return nil
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }


    // Resource that allows other users to access details of the NFTtypes
    // by providing the IDs for them
    pub resource NftTypeHelper : NftTypeHelperPublic {
        // public function to borrow details of NFTtype
        pub fun borrowActiveNFTtype(id: String): NftType? {
            if NowggNFT.activeNftTypes[id] != nil {
                let ref = NowggNFT.activeNftTypes[id]
                return ref
            } else {
                return nil
            }
        }

        pub fun borrowHistoricNFTtype(id: String): NftType? {
            if NowggNFT.historicNftTypes[id] != nil {
                let ref = NowggNFT.historicNftTypes[id]
                return ref
            } else {
                return nil
            }
        }
    }


    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
	pub resource NFTMinter {
	    // mintNFT
        // Mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeId: String, metaData: {String: AnyStruct}) {

            if (!NowggNFT.activeNftTypes.keys.contains(typeId)) {
                panic("Invalid typeId")
            }
            let nftType = NowggNFT.activeNftTypes[typeId]!
            let currentCount = nftType.currentCount

            if (currentCount >= nftType.maxCount) {
                panic("NFT mint limit exceeded")
            }

            let updateCount = currentCount + (1 as UInt64);

            // Adding copy number to metadata
            metaData["copyNumber"] = updateCount;

            metaData["maxCount"] = nftType.maxCount;
            
            // Create and deposit NFT in recipent's account
            recipient.deposit(token: <-create NowggNFT.NFT(initID: NowggNFT.totalSupply, metadata: metaData))

            // Increment count for NFT of particular type
            nftType.updateCount(count: updateCount)

            if (updateCount < nftType.maxCount) {
                NowggNFT.activeNftTypes[typeId] = nftType
            } else {
                NowggNFT.historicNftTypes[typeId] = nftType
                NowggNFT.activeNftTypes.remove(key: typeId)
                emit TypeSoldOut(typeId: typeId)
            }

            // emit event
            emit Minted(id: NowggNFT.totalSupply, typeId: typeId)

            // Increment total supply of NFTs
            NowggNFT.totalSupply = NowggNFT.totalSupply + (1 as UInt64)
        }

        pub fun registerType(typeId: String, maxCount: UInt64) {
            let nftType = NftType(typeId: typeId, maxCount: maxCount)
            NowggNFT.activeNftTypes[typeId] = nftType
            emit TypeRegistered(typeId: typeId)
        }
	}

    // fetch
    // Get a reference to a NowggNFT from an account's Collection, if available.
    // If an account does not have a NowggNFT.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    pub fun borrowNFT(from: Address, itemID: UInt64): &NowggNFT.NFT? {
        let collection = getAccount(from)
            .getCapability(NowggNFT.CollectionPublicPath)!
            .borrow<&NowggNFT.Collection{NowggNFT.NowggNFTCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust NowggNFT.Collection.borrowNowggNFT to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowNowggNFT(id: itemID)
    }

    // initializer
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/NowggNFTsCollection
        self.CollectionPublicPath = /public/NowggNFTsCollection
        self.MinterStoragePath = /storage/NowggNFTMinter
        self.NftTypeHelperStoragePath = /storage/NowggNftTypeHelperStoragePath
        self.NFTtypeHelperPublicPath = /public/NowggNftNFTtypeHelperPublicPath

        // Initialize the total supply
        self.totalSupply = 0

        // Initialize active NFT types
        self.activeNftTypes = {}

        // Initialize historic NFT types
        self.historicNftTypes = {}

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        // Create an empty collection
        let emptyCollection <- self.createEmptyCollection()
        self.account.save(<-emptyCollection, to: self.CollectionStoragePath)
        self.account.unlink(self.CollectionPublicPath)
        self.account.link<&NowggNFT.Collection{NonFungibleToken.CollectionPublic, NowggNFT.NowggNFTCollectionPublic}>(
            self.CollectionPublicPath, target: self.CollectionStoragePath
        )

        // Create helper for getting details of NftTypes
        let nftTypehelper <- create NftTypeHelper()
        self.account.save(<-nftTypehelper, to: self.NftTypeHelperStoragePath)
        self.account.unlink(self.NFTtypeHelperPublicPath)
        self.account.link<&NowggNFT.NftTypeHelper{NowggNFT.NftTypeHelperPublic}>(
            self.NFTtypeHelperPublicPath, target: self.NftTypeHelperStoragePath
        )

        // Emit event for contract initialized
        emit ContractInitialized()
	}
}

import NonFungibleToken from 0x1d7e57aa55817448

// RacingTime
// NFT items for RacingTime!
pub contract RacingTime: NonFungibleToken {
    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, typeID: UInt32, rewardID: UInt32,serialNumber: UInt32,ipfs: String)
    pub event Burn(id: UInt64)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    // The total number of tokens of this type in existence
    pub var totalSupply: UInt64


    pub resource interface NFTPublic {
        pub let id: UInt64
        pub let data: NFTData
    }

    pub struct NFTData {

        // The ID of the Reward that the NFT references
        pub let rewardID: UInt32
        // The token's type, e.g. 3 == ss
        pub let typeID: UInt32
        // The token mint number
        // Otherwise known as the serial number
        pub let serialNumber: UInt32
        // Image storage location
        pub let ipfs: String

        init(rewardID: UInt32,initTypeID: UInt32, serialNumber: UInt32,ipfs: String) {
            self.rewardID=rewardID
            self.typeID=initTypeID
            self.serialNumber=serialNumber
            self.ipfs=ipfs
        }
    }

    pub resource NFT: NonFungibleToken.INFT , NFTPublic {

        // global unique NFT ID
        pub let id: UInt64

        pub let data: NFTData

        init(initID: UInt64,nftData: NFTData) {
            self.id = initID
            self.data = nftData
        }

        destroy() {
            emit Burn(id: self.id)
        }
    }


    // This is the interface that users can cast their RacingTime Collection as
    // to allow others to deposit RacingTime into their Collection. It also allows for reading
    // the details of RacingTime in the Collection.
    pub resource interface RacingTimeCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun depositBatch(cardCollection: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowRacingTime(id: UInt64): &RacingTime.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow RacingTime reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of RacingTime NFTs owned by an account
    pub resource Collection: RacingTimeCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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
            let token <- token as! @RacingTime.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }
        // depositBatch
        // This is primarily called by an Admin to
        // deposit newly minted Cards into this Collection.
        pub fun depositBatch(cardCollection: @NonFungibleToken.Collection) {
            pre {
                cardCollection.getIDs().length <= 100:
                    "Too many cards being deposited. Must be less than or equal to 100"
            }

            // Get an array of the IDs to be deposited
            let keys = cardCollection.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-cardCollection.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy cardCollection
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

        // borrowRacingTime
        // Gets a reference to an NFT in the collection as a RacingTime,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the RacingTime.
        pub fun borrowRacingTime(id: UInt64): &RacingTime.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &RacingTime.NFT
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

    // NFTMinter
    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    pub resource NFTMinter {
        // mintNFT
        // Mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt32, rewardID: UInt32,serialNumber: UInt32,ipfs: String ) {
            RacingTime.totalSupply = RacingTime.totalSupply + (1 as UInt64)
            emit Minted(id: RacingTime.totalSupply, typeID: typeID,rewardID: rewardID, serialNumber: serialNumber,ipfs: ipfs)
			// deposit it in the recipient's account using their reference
			recipient.deposit(token: <-create RacingTime.NFT(initID: RacingTime.totalSupply,data: NFTData(
                rewardID: rewardID, initTypeID: typeID, serialNumber: serialNumber,ipfs:ipfs
            )))
        }
    }
    
    // initializer
    init() {
        // Initialize the total supply
        self.totalSupply = 0
        // Set our named paths
        self.CollectionStoragePath = /storage/RacingTimeCollection
        self.CollectionPublicPath = /public/RacingTimeCollection
        self.MinterStoragePath = /storage/RacingTimeMinter

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()

        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

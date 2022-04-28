import NonFungibleToken from 0x1d7e57aa55817448;

// AvatarArtNFT
// NFT items for AvatarArt!

pub contract AvatarArtNFT: NonFungibleToken {

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(tokenId: UInt64, metadata: String);

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let ReceiverPublicPath: PublicPath;

    // totalSupply
    // The total number of AvatarArtNFT that have been minted
    //
    pub var totalSupply: UInt64

    // NFT
    // A AvatarArtNFT as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        pub let metadata: String;

        // initializer
        //
        init(initID: UInt64, metadata: String) {
            self.id = initID
            self.metadata = metadata;
        }
    }

    // This is the interface that users can cast their AvatarArtNFT Collection as
    // to allow others to deposit AvatarArtNFT into their Collection. It also allows for reading
    // the details of AvatarArtNFT in the Collection.
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowAvatarArtNFT(id: UInt64): &AvatarArtNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow AvatarArtNFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of AvatarArtNFT NFTs owned by an account
    //
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
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
            let token <- token as! @AvatarArtNFT.NFT

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

        // borrowAvatarArtNFT
        // Gets a reference to an NFT in the collection as a AvatarArtNFT,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the AvatarArtNFT.
        //
        pub fun borrowAvatarArtNFT(id: UInt64): &AvatarArtNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &AvatarArtNFT.NFT
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
    pub fun createEmptyCollection(): @Collection {
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
        //
		pub fun mintNFT(metadata: String, recipient: &{AvatarArtNFT.CollectionPublic}) {
            // ID start from 1000
            let id = (1000 as UInt64) + AvatarArtNFT.totalSupply;

            emit Minted(tokenId: id, metadata: metadata);
			// deposit it in the recipient's account using their reference
			recipient.deposit(token: <-create AvatarArtNFT.NFT(initID: id, metadata: metadata))

            AvatarArtNFT.totalSupply = AvatarArtNFT.totalSupply + 1;
		}
	}

    // fetch
    // Get a reference to a AvatarArtNFT from an account's Collection, if available.
    // If an account does not have a AvatarArtNFT.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &AvatarArtNFT.NFT? {
        let collection = getAccount(from)
            .getCapability(AvatarArtNFT.CollectionPublicPath)
            .borrow<&AvatarArtNFT.Collection{AvatarArtNFT.CollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust AvatarArtNFT.Collection.borowAvatarArtNFT to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowAvatarArtNFT(id: itemID)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/avatarArtNFTCollection;
        self.CollectionPublicPath = /public/avatarArtNFTCollection;
        self.MinterStoragePath = /storage/avatarArtNFTMinter;
        self.ReceiverPublicPath = /public/avatarArtReceiver;

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
	}
}

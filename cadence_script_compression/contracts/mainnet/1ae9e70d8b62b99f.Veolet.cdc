import NonFungibleToken from 0x1d7e57aa55817448

pub contract Veolet: NonFungibleToken {
    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub resource NFT: NonFungibleToken.INFT {
        // Token ID
        pub let id: UInt64
        // URL to the media file represented by the NFT 
        pub let originalMediaURL: String
        // Flow address of the NFT creator
        pub let creatorAddress: Address
        // Name of the NFT creator
        pub let creatorName: String
        // Date of creation
        pub let createdDate: UInt64
        // Short description of the NFT
        pub let caption: String
        // SHA256 hash of the media file for verification
        pub let hash: String
        // How many NFTs with this hash (e.g. same media file) are minted
        pub let edition: UInt16

        // A mutable version of the media URL. The holder can change the URL in case the original URL is not available anymore. Through the hash variable it can be
        // verified whether the file is actually the correct one. 
        pub(set) var currentMediaURL: String


        // initializer of the token
        init(initID: UInt64,initMediaURL: String, initCreatorName: String, initCreatorAddress: Address, initCreatedDate: UInt64, initCaption: String, initHash: String, initEdition: UInt16) {
            self.id = initID
            self.originalMediaURL = initMediaURL
            self.creatorAddress = initCreatorAddress
            self.creatorName = initCreatorName
            self.createdDate = initCreatedDate
            self.caption = initCaption
            self.hash = initHash
            self.edition = initEdition

            // the mutable URL variable is also set to the originalMediaURL value initially
            self.currentMediaURL = initMediaURL
        }
    }

    // Interface to receive NFT references and deposit NFT (implemented by collection)
    pub resource interface VeoletGetter {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowVeoletRef(id: UInt64): &Veolet.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow NFT reference: The ID of the returned reference is incorrect"
            }
        }
    } 
    pub resource Collection: VeoletGetter, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // Initializer
        init () {
            self.ownedNFTs <- {}
        }

        pub fun setNFTMediaURL(id: UInt64, newMediaURL: String){
            // change the currentMediaURL field of token. This can only be done by the holder
            // Please note that the originalMediaURL will still be the same (it is immutable)
            let changetoken <- self.ownedNFTs.remove(key: id)! as! @Veolet.NFT
            changetoken.currentMediaURL = newMediaURL
            self.deposit(token: <-changetoken)
        }


        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Veolet.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Takes a NFT id as input and returns a reference to the token. Used to obtain token fields
       pub fun borrowVeoletRef(id: UInt64): &Veolet.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Veolet.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
	pub resource NFTMinter {

		// mintNFT mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic},initMediaURL: String, initCreatorName: String, initCreatorAddress: Address, initCreatedDate: UInt64, initCaption: String, initHash: String, initEdition:UInt16 ) {

			// create a new NFT
			var newNFT <- create NFT(initID: Veolet.totalSupply,initMediaURL: initMediaURL, initCreatorName: initCreatorName, initCreatorAddress: initCreatorAddress, initCreatedDate: initCreatedDate, initCaption: initCaption, initHash: initHash,initEdition:initEdition )

			// deposit it in the recipient's account using their reference
			recipient.deposit(token: <-newNFT)

            Veolet.totalSupply = Veolet.totalSupply + (1 as UInt64)
		}
	}

	init() {
        // Initialize the total supply
        self.totalSupply = 0

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: /storage/VeoletCollection)

        // create a public capability for the collection
        self.account.link<&Veolet.Collection{NonFungibleToken.CollectionPublic, Veolet.VeoletGetter}>(
            /public/VeoletCollection,
            target: /storage/VeoletCollection
        )

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: /storage/NFTMinter)

        emit ContractInitialized()
	}

}

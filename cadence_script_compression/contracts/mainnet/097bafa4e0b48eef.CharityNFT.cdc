
import NonFungibleToken from 0x1d7e57aa55817448

pub contract CharityNFT: NonFungibleToken {

	pub var totalSupply: UInt64

	pub let CollectionStoragePath: StoragePath
	pub let CollectionPublicPath: PublicPath

	pub event ContractInitialized()
	pub event Withdraw(id: UInt64, from: Address?)
	pub event Deposit(id: UInt64, to: Address?)
	pub event Minted(id: UInt64, metadata: {String:String}, to:Address)

	pub resource NFT: NonFungibleToken.INFT, Public {
		pub let id: UInt64

		access(self) let metadata: {String: String}

		init(initID: UInt64, metadata: {String : String}) {
			self.id = initID
			self.metadata = metadata
		}

		pub fun getMetadata() : { String : String} {
			return self.metadata
		}
	}

	//The public interface can show metadata and the content for the Art piece
	pub resource interface Public {
		pub let id: UInt64
		pub fun getMetadata() : {String : String}
	}

	//Standard NFT collectionPublic interface that can also borrowArt as the correct type
	pub resource interface CollectionPublic {

		pub fun deposit(token: @NonFungibleToken.NFT)
		pub fun getIDs(): [UInt64]
		pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
		pub fun borrowCharity(id: UInt64): &{Public}?
	}

	pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
		// dictionary of NFT conforming tokens
		// NFT is a resource type with an `UInt64` ID field
		pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

		init () {
			self.ownedNFTs <- {}
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
			let token <- token as! @CharityNFT.NFT

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

		//borrow charity
		pub fun borrowCharity(id: UInt64): &{CharityNFT.Public}? {
			if self.ownedNFTs[id] != nil {
				let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
				return ref as! &NFT
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


	// mintNFT mints a new NFT with a new ID
	// and deposit it in the recipients collection using their collection reference
	access(account) fun mintCharity(metadata: {String:String}, recipient: Capability<&{NonFungibleToken.CollectionPublic}>) {

		// create a new NFT
		var newNFT <- create NFT(initID: CharityNFT.totalSupply, metadata:metadata)

		// deposit it in the recipient's account using their reference
		recipient.borrow()!.deposit(token: <-newNFT)
		emit Minted(id: CharityNFT.totalSupply, metadata:metadata, to: recipient.address)

		CharityNFT.totalSupply = CharityNFT.totalSupply + 1 
	}

	init() {
		// Initialize the total supply
		self.totalSupply = 0

		emit ContractInitialized()
		self.CollectionPublicPath=/public/findCharityCollection
		self.CollectionStoragePath=/storage/findCharityCollection
	}
}


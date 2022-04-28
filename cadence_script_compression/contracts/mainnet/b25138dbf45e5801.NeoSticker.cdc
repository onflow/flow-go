
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import NeoViews from 0xb25138dbf45e5801

pub contract NeoSticker: NonFungibleToken {

	pub var totalSupply: UInt64

	pub event ContractInitialized()
	pub event Withdraw(id: UInt64, from: Address?)
	pub event Deposit(id: UInt64, to: Address?)

	pub event StickerTypeAdded(typeId:UInt64)
	pub event Minted(id:UInt64, typeId:UInt64, setId:UInt64, edition:UInt64, maxEdition:UInt64)

	pub let CollectionStoragePath: StoragePath
	pub let CollectionPublicPath: PublicPath
	
	access(contract) let stickerTypes: {UInt64: StickerType}

	pub struct StickerType{

		pub let name: String
		pub let description: String
		pub let thumbnailHash: String
		pub let rarity: UInt64
		pub let location:UInt64
		pub let properties: {String:String}

		init(name:String, description:String, thumbnailHash:String, rarity:UInt64, location:UInt64) {
			self.name=name
			self.description=description
			self.thumbnailHash=thumbnailHash
			self.rarity=rarity
			self.location=location
			self.properties={}
		}
	}


  access(account) fun createStickerType(typeId: UInt64, metadata: StickerType) {
		NeoSticker.stickerTypes[typeId]=metadata
		emit StickerTypeAdded(typeId:typeId)
	}

	pub fun getStickerType(_ typeId: UInt64): StickerType? {
			return NeoSticker.stickerTypes[typeId]
	}

	pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
		pub let id: UInt64
		pub let edition:UInt64
		pub let maxEdition:UInt64
		
		pub let typeId: UInt64
		pub let setId: UInt64
		
		init(
			id: UInt64,
			typeId:UInt64,
			setId:UInt64,
			edition:UInt64
			maxEdition:UInt64
		) {
			self.id = id
			self.typeId=typeId
			self.setId=setId
			self.edition=edition
			self.maxEdition=maxEdition
		}
		pub fun getViews(): [Type] {
			return [
			Type<MetadataViews.Display>(),
			Type<NeoViews.StickerView>()
			]
		}

		pub fun getStickerType() : StickerType{
			return NeoSticker.getStickerType(self.typeId)!
		}

		pub fun resolveView(_ view: Type): AnyStruct? {

			let type=self.getStickerType()
			switch view {
			case Type<NeoViews.StickerView>():
				return NeoViews.StickerView(
					id: self.id,
					name: type.name,
					description: type.description,
					thumbnailHash:type.thumbnailHash,
					rarity:type.rarity,
					location:type.location,
					edition:self.edition,
					maxEdition:self.maxEdition,
					typeId:self.typeId,
					setId:self.setId)
			case Type<MetadataViews.Display>():
				return MetadataViews.Display(
					name: type.name,
					description: type.description,
					thumbnail: MetadataViews.IPFSFile(
						cid: type.thumbnailHash,
						path: nil
					)
				)
			}
			return nil
		}
	}

	pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
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
			let token <- token as! @NeoSticker.NFT

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

		pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
			let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
			let neoSticker = nft as! &NeoSticker.NFT
			return neoSticker as &AnyResource{MetadataViews.Resolver}
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
	access(account) fun mintNeoSticker(
		typeId:UInt64,
		setId:UInt64,
		edition:UInt64,
		maxEdition:UInt64,
	) :@NeoSticker.NFT {

		NeoSticker.totalSupply = NeoSticker.totalSupply + 1
		var newNFT <- create NFT(
			id: NeoSticker.totalSupply,
			typeId:typeId,
			setId:setId,
			edition:edition,
			maxEdition:maxEdition
		)

		emit Minted(id:newNFT.id, typeId:typeId, setId:setId, edition:edition, maxEdition:maxEdition)
		return <- newNFT
	}

	init() {
		// Initialize the total supply
		self.totalSupply = 0

		// Set the named paths
		self.CollectionStoragePath = /storage/neoStickerCollection
		self.CollectionPublicPath = /public/neoStickerCollection

		self.stickerTypes={}

		emit ContractInitialized()
	}
}


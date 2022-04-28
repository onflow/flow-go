import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import Content from 0xd796ff17107bbff6

/// A NFT contract to store art
pub contract Art: NonFungibleToken {

	pub let CollectionStoragePath: StoragePath
	pub let CollectionPublicPath: PublicPath

	pub var totalSupply: UInt64

	pub event ContractInitialized()
	pub event Withdraw(id: UInt64, from: Address?)
	pub event Deposit(id: UInt64, to: Address?)
	pub event Created(id: UInt64, metadata: Metadata)
	pub event Editioned(id: UInt64, from: UInt64, edition: UInt64, maxEdition: UInt64)

	//The public interface can show metadata and the content for the Art piece
	pub resource interface Public {
		pub let id: UInt64
		pub let metadata: Metadata

		//these three are added because I think they will be in the standard. Atleast dieter thinks it will be needed
		pub let name: String
		pub let description: String
		pub let schema: String? 

		pub fun content() : String?

		access(account) let royalty: {String: Royalty}
		pub fun cacheKey() : String

	}

	pub struct Metadata {
		pub let name: String
		pub let artist: String
		pub let artistAddress:Address
		pub let description: String
		pub let type: String
		pub let edition: UInt64
		pub let maxEdition: UInt64


		init(name: String, 
		artist: String,
		artistAddress:Address, 
		description: String, 
		type: String, 
		edition: UInt64,
		maxEdition: UInt64) {
			self.name=name
			self.artist=artist
			self.artistAddress=artistAddress
			self.description=description
			self.type=type
			self.edition=edition
			self.maxEdition=maxEdition
		}

	}

	pub struct Royalty{
		pub let wallet:Capability<&{FungibleToken.Receiver}> 
		pub let cut: UFix64

		/// @param wallet : The wallet to send royalty too
		init(wallet:Capability<&{FungibleToken.Receiver}>, cut: UFix64 ){
			self.wallet=wallet
			self.cut=cut
		}
	}

	pub resource NFT: NonFungibleToken.INFT, Public {
		//TODO: tighten up the permission here.
		pub let id: UInt64
		pub let name: String
		pub let description: String

		pub let schema: String?
		//content can either be embedded in the NFT as and URL or a pointer to a Content collection to be stored onChain
		//a pointer will be used for all editions of the same Art when it is editioned 
		pub let contentCapability:Capability<&Content.Collection>?
		pub let contentId: UInt64?
		pub let url: String?
		pub let metadata: Metadata
		access(account) let royalty: {String: Royalty}

		init(initID: UInt64, 
		metadata: Metadata,
		contentCapability:Capability<&Content.Collection>?, 
		contentId: UInt64?, 
		url: String?,
		royalty:{String: Royalty}) {

			self.id = initID
			self.metadata=metadata
			self.contentCapability=contentCapability
			self.contentId=contentId
			self.url=url
			self.royalty=royalty
			self.schema=nil
			self.name = metadata.name
			self.description=metadata.description
		}

		pub fun cacheKey() : String {
			if self.url != nil {
				return self.url!
			}
			return self.contentId!.toString()
		}

		//return the content for this NFT
		pub fun content() : String {
			if self.url != nil {
				return self.url!
			}

			let contentCollection= self.contentCapability!.borrow()!
			return contentCollection.content(self.contentId!)
		}
	}


	//Standard NFT collectionPublic interface that can also borrowArt as the correct type
	pub resource interface CollectionPublic {

		pub fun deposit(token: @NonFungibleToken.NFT)
		pub fun getIDs(): [UInt64]
		pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
		pub fun borrowArt(id: UInt64): &{Art.Public}?
	}


	pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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
			let token <- token as! @Art.NFT

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

		// borrowArt returns a borrowed reference to a Art 
		// so that the caller can read data and call methods from it.
		//
		// Parameters: id: The ID of the NFT to get the reference for
		//
		// Returns: A reference to the NFT
		pub fun borrowArt(id: UInt64): &{Art.Public}? {
			if self.ownedNFTs[id] != nil {
				let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
				return ref as! &Art.NFT
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



	pub struct ArtData {
		pub let metadata: Art.Metadata
		pub let id: UInt64
		pub let cacheKey: String
		init(metadata: Art.Metadata, id: UInt64, cacheKey: String) {
			self.metadata= metadata
			self.id=id
			self.cacheKey=cacheKey
		}
	}



	pub fun getContentForArt(address:Address, artId:UInt64) : String? {

		let account=getAccount(address)
		if let artCollection= account.getCapability(self.CollectionPublicPath).borrow<&{Art.CollectionPublic}>()  {
			return artCollection.borrowArt(id: artId)!.content()
		}
		return nil
	}

	// We cannot return the content here since it will be too big to run in a script
	pub fun getArt(address:Address) : [ArtData] {

		var artData: [ArtData] = []
		let account=getAccount(address)

		if let artCollection= account.getCapability(self.CollectionPublicPath).borrow<&{Art.CollectionPublic}>()  {
			for id in artCollection.getIDs() {
				var art=artCollection.borrowArt(id: id) 
				artData.append(ArtData(
					metadata: art!.metadata,
					id: id, 
					cacheKey: art!.cacheKey()))
				}
			}
			return artData
		} 

		//This method can only be called from another contract in the same account. In Versus case it is called from the VersusAdmin that is used to administer the solution
		access(account) fun createArtWithContent(name: String, artist:String, artistAddress:Address, description: String, url: String, type: String, royalty: {String: Royalty}, edition: UInt64, maxEdition: UInt64) : @Art.NFT {
			var newNFT <- create NFT(
				initID: Art.totalSupply,
				metadata: Metadata(
					name: name, 
					artist: artist,
					artistAddress: artistAddress, 
					description:description,
					type:type,
					edition:edition,
					maxEdition: maxEdition
				),
				contentCapability:nil,
				contentId:nil,
				url:url, 
				royalty:royalty
			)
			emit Created(id: Art.totalSupply, metadata: newNFT.metadata)

			Art.totalSupply = Art.totalSupply + UInt64(1)
			return <- newNFT
		}

		//This method can only be called from another contract in the same account. In Versus case it is called from the VersusAdmin that is used to administer the solution
		access(account) fun createArtWithPointer(name: String, artist: String, artistAddress:Address, description: String, type: String, contentCapability:Capability<&Content.Collection>, contentId: UInt64, royalty: {String: Royalty}) : @Art.NFT{

			let metadata=Metadata( name: name, artist: artist, artistAddress: artistAddress, description:description, type:type, edition:1, maxEdition:1)
			var newNFT <- create NFT(initID: Art.totalSupply,metadata: metadata, contentCapability:contentCapability, contentId:contentId, url:nil, royalty:royalty)
			emit Created(id: Art.totalSupply, metadata: newNFT.metadata)

			Art.totalSupply = Art.totalSupply + UInt64(1)
			return <- newNFT
		}

		//This method can only be called from another contract in the same account. In Versus case it is called from the VersusAdmin that is used to administer the solution
		access(account) fun makeEdition(original: &NFT, edition: UInt64, maxEdition:UInt64) : @Art.NFT {
			let metadata=Metadata( name: original.metadata.name, artist:original.metadata.artist, artistAddress:original.metadata.artistAddress, description:original.metadata.description, type:original.metadata.type, edition: edition, maxEdition:maxEdition)
			var newNFT <- create NFT(initID: Art.totalSupply, metadata: metadata , contentCapability: original.contentCapability, contentId:original.contentId, url:original.url, royalty:original.royalty)
			emit Created(id: Art.totalSupply, metadata: newNFT.metadata)
			emit Editioned(id: Art.totalSupply, from: original.id, edition:edition, maxEdition:maxEdition)

			Art.totalSupply = Art.totalSupply + UInt64(1)
			return <- newNFT
		}


		init() {
			// Initialize the total supply
			self.totalSupply = 0
			self.CollectionPublicPath=/public/versusArtCollection
			self.CollectionStoragePath=/storage/versusArtCollection

			self.account.save<@NonFungibleToken.Collection>(<- Art.createEmptyCollection(), to: Art.CollectionStoragePath)
			self.account.link<&{Art.CollectionPublic}>(Art.CollectionPublicPath, target: Art.CollectionStoragePath)
			emit ContractInitialized()
		}
	}


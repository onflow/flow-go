import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import NeoViews from 0xb25138dbf45e5801
import MetadataViews from 0x1d7e57aa55817448

/// A NFT contract to store a NEO avatar
pub contract NeoAvatar: NonFungibleToken {

	pub let CollectionStoragePath: StoragePath
	pub let CollectionPublicPath: PublicPath

	pub var totalSupply: UInt64

	pub event ContractInitialized()
	pub event Withdraw(id: UInt64, from: Address?)
	pub event Deposit(id: UInt64, to: Address?)

	pub event Minted(id:UInt64, name:String, teamId:UInt64, role:String, series:String, imageHash:String, address:Address)
	pub event Purchased(id:UInt64, name:String, teamId:UInt64, role:String, series:String, address:Address)

	pub event OriginalMinterSet(id:UInt64, address:Address)


	pub struct NeoAvatarView{
		pub let teamId:UInt64
		pub let role:String
		pub let series:String

		init(teamId:UInt64, role:String, series:String){
			self.teamId=teamId
			self.role=role
			self.series=series
		}
	}

	pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {

		pub let id: UInt64
		pub let name:String
		pub let role:String
		pub let series:String
		pub let teamId:UInt64
		pub let imageHash: String
		access(contract) var originalMinterWallet: Capability<&{FungibleToken.Receiver}>


		init(id: UInt64, imageHash:String, wallet: Capability<&{FungibleToken.Receiver}>,name:String, teamId: UInt64, role:String, series:String, ) {
			self.id = id
			self.imageHash=imageHash
			self.originalMinterWallet=wallet
			self.teamId=teamId
			self.role=role
			self.series=series
			self.name=name
		}

		pub fun getViews(): [Type] {
			return [
			Type<MetadataViews.Display>(), 
			Type<MetadataViews.IPFSFile>(),
			Type<NeoViews.Royalties>(),
			Type<NeoAvatarView>(),
			Type<NeoViews.ExternalDomainViewUrl>()
			]
		}

		pub fun setWallet(_ wallet: Capability<&{FungibleToken.Receiver}>){
			self.originalMinterWallet=wallet
		}

		pub fun resolveView(_ view: Type): AnyStruct? {
			switch view {

			case Type<MetadataViews.Display>():
				return MetadataViews.Display(
					name: self.name,
					description: "The neo avatar minted originally for address=".concat(self.originalMinterWallet.address.toString()).concat(" during the Neo Champtionship ").concat(self.series),
					thumbnail: MetadataViews.IPFSFile(cid: self.imageHash, path: nil)
				)
			case Type<String>():
				return self.name

				case Type<MetadataViews.IPFSFile>(): 
				return MetadataViews.IPFSFile(cid: self.imageHash, path: nil)

			case Type<NeoViews.ExternalDomainViewUrl>():
				return NeoViews.ExternalDomainViewUrl(url: "https://neocollectibles.xyz")

			case Type<NeoViews.Royalties>(): 
				return self.getRoyalty()

			case Type<NeoAvatar.NeoAvatarView>() :
				return NeoAvatarView(teamId:self.teamId, role:self.role, series:self.series)
			}

			return nil
		}

		pub fun getRoyalty() : NeoViews.Royalties {
			let minterRoyalty= NeoViews.Royalty( wallet: NeoAvatar.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver), cut: 0.05)
			let founderRoyalty=NeoViews.Royalty(wallet: self.originalMinterWallet, cut: 0.01)
			return NeoViews.Royalties({ "minter" : minterRoyalty, "originalOwner" : founderRoyalty})
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
			let token <- token as! @NFT

			if token.originalMinterWallet.address == NeoAvatar.account.address {
				token.setWallet(self.owner!.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver))
				emit OriginalMinterSet(id:token.id, address: self.owner!.address)
			}

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
			let exampleNFT = nft as! &NeoAvatar.NFT
			return exampleNFT 
		}

		destroy() {
			destroy self.ownedNFTs
		}
	}

	//This is temp until we have some global admin
	pub resource NeoAvatarAdmin {

		pub fun mint(teamId:UInt64, series:String, role:String, imageHash:String, wallet: Capability<&{FungibleToken.Receiver}>, collection: Capability<&{NonFungibleToken.Receiver}>) {
			NeoAvatar.mint(teamId:teamId, series:series, role:role, imageHash:imageHash, wallet:wallet, collection:collection)
		}
	}

	//This method can only be called from another contract in the same account. In this case the Admin account
	access(account) fun mint(teamId:UInt64, series:String, role:String, imageHash:String, wallet: Capability<&{FungibleToken.Receiver}>, collection: Capability<&{NonFungibleToken.Receiver}>) {
		self.totalSupply = self.totalSupply + 1

		let name=role.concat(" for Neo Team #").concat(teamId.toString())
		var newNFT <- create NFT( id: self.totalSupply, imageHash: imageHash, wallet:wallet, name:name, teamId:teamId, role:role, series:series)

		emit Minted(id:self.totalSupply, name:name, teamId:teamId, role:role, series:series, imageHash: imageHash, address:collection.address)
		collection.borrow()!.deposit(token: <- newNFT)

	}

	pub fun purchase(id:UInt64, vault : @FungibleToken.Vault, nftReceiver: Capability<&{NonFungibleToken.Receiver}>) {

		let collection= NeoAvatar.account.borrow<&NeoAvatar.Collection>(from: self.CollectionStoragePath)!

		let ids= collection.getIDs()

		if !ids.contains(id) {
			panic("This neo avatar is already sold")
		}

		let vault <- vault as! @FlowToken.Vault

		if vault.balance != 1.0 {
			panic("The purchase does not contain the required amount of 1 flow")
		}


		let tokenRef = collection.borrowViewResolver(id: id)

		let neoAvatar =tokenRef.resolveView(Type<NeoAvatar.NeoAvatarView>())! as! NeoAvatar.NeoAvatarView
		let display =tokenRef.resolveView(Type<MetadataViews.Display>())! as! MetadataViews.Display


		emit Purchased(id: id, name: display.name, teamId:neoAvatar.teamId, role:neoAvatar.role, series:neoAvatar.series, address:nftReceiver.address)

		let token <- collection.withdraw(withdrawID: id)
		nftReceiver.borrow()!.deposit(token: <- token)

		let receiver = NeoAvatar.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver).borrow()!
		receiver.deposit(from: <- vault)

	}

	// public function that anyone can call to create a new empty collection
	pub fun createEmptyCollection(): @NonFungibleToken.Collection {
		return <- create Collection()
	}


	init() {

		let admin <- create NeoAvatarAdmin()
		self.account.save(<- admin, to: /storage/neoAvatarAdmin)

		// Initialize the total supply
		self.totalSupply = 0
		self.CollectionPublicPath=/public/neoAvatarCollection
		self.CollectionStoragePath=/storage/neoAvatarCollection

		emit ContractInitialized()
	}
}

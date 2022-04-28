import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import NeoViews from 0xb25138dbf45e5801

//This NFT contract is a grouping of a Founder (that can admin it) and its members. It lives in the Neo account always and could in essence even be just a resource and not an NFT
pub contract NeoMotorcycle: NonFungibleToken {

	pub let CollectionStoragePath: StoragePath
	pub let CollectionPrivatePath: PrivatePath

	pub var totalSupply: UInt64

	pub event ContractInitialized()
	pub event Withdraw(id: UInt64, from: Address?)
	pub event Deposit(id: UInt64, to: Address?)

	pub event Minted(teamId:UInt64)

	pub event AchievementAdded(teamId:UInt64, achievement: String)
	pub event NameChanged(teamId:UInt64, name: String)
	pub event PhysicalLinkAdded(teamId:UInt64, link: String)
	pub event FounderWalletChanged(teamId:UInt64, address: Address)

	pub struct Pointer{
		pub let collection: Capability<&{NeoMotorcycle.CollectionPublic}>
		pub let id: UInt64

		init(collection: Capability<&{NeoMotorcycle.CollectionPublic}>, id: UInt64) {
			self.collection=collection
			self.id=id
		}

		pub fun resolve() : &NeoMotorcycle.NFT? {
			return self.collection.borrow()!.borrowNeoMotorcycle(id: self.id)
		}
	}

	pub struct Achievement {
		pub let name: String
		pub let description: String

		pub init(name: String, description:String) {
			self.name=name
			self.description=description
		}
	}

	pub resource NFT: NonFungibleToken.INFT {
		pub let id: UInt64
		access(self) var name:String
		pub var physicalLink: String?
		pub var founderWalletCap: Capability<&{FungibleToken.Receiver}>

		access(self) let achievements : [Achievement]

		init(id: UInt64)  {
			self.id = id
			self.physicalLink=nil
			self.achievements=[]
			self.founderWalletCap=NeoMotorcycle.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
			self.name="Team #".concat(id.toString())
		}

		pub fun addPhysicalLink(_ link:String) {
			pre {
				self.physicalLink == nil : "Cannot change physicalLink"
			}

			emit PhysicalLinkAdded(teamId:self.id, link:link)
			self.physicalLink=link
		}

		pub fun setName(_ name:String) {
			emit NameChanged(teamId:self.id, name:name)
			self.name=name
		}

		pub fun getName():String{
			return self.name
		}

		pub fun setNeoFounderWallet(_ wallet:Capability<&{FungibleToken.Receiver}>) {
			emit FounderWalletChanged(teamId:self.id, address: wallet.address)
			self.founderWalletCap=wallet
		}

		pub fun addAchievement(_ achievement: Achievement) {
			emit AchievementAdded(teamId: self.id, achievement:achievement.name)
			self.achievements.append(achievement)
		}

		pub fun getAchievements() : [Achievement] {
			return self.achievements
		}

		pub fun getRoyalty() : NeoViews.Royalties {
			let minterRoyalty= NeoViews.Royalty( wallet: NeoMotorcycle.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver), cut: 0.10)
			let founderRoyalty=NeoViews.Royalty(wallet: self.founderWalletCap, cut: 0.05)
			return NeoViews.Royalties({ "minter" : minterRoyalty, "founder" : founderRoyalty})
		}
	}

	//Standard NFT collectionPublic interface that can also borrowArt as the correct type
	pub resource interface CollectionPublic {

		pub fun deposit(token: @NonFungibleToken.NFT)
		pub fun getIDs(): [UInt64]
		pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
		access(account) fun borrowNeoMotorcycle(id: UInt64): &NeoMotorcycle.NFT?
	}

	pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic  {
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
			let token <- token as! @NeoMotorcycle.NFT

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

		access(account) fun borrowNeoMotorcycle(id: UInt64): &NFT? {
			if self.ownedNFTs[id] != nil {
				let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
				return ref as! &NeoMotorcycle.NFT
			} else {
				return nil
			}
		}

		destroy() {
			destroy self.ownedNFTs
		}
	}

	access(account) fun mint(): @NFT {

		NeoMotorcycle.totalSupply = NeoMotorcycle.totalSupply + 1
		var newNFT <- create NFT( id: NeoMotorcycle.totalSupply)

		emit Minted(teamId: newNFT.id) 

		return <- newNFT
	}

	// public function that anyone can call to create a new empty collection
	pub fun createEmptyCollection(): @NonFungibleToken.Collection {
		return <- create Collection()
	}



	init() {
		// Initialize the total supply
		self.totalSupply = 0
		self.CollectionPrivatePath=/private/neoMotorcycylesCollection
		self.CollectionStoragePath=/storage/neoMotorcycylesCollection

		let account =self.account
		account.save(<- NeoMotorcycle.createEmptyCollection(), to: NeoMotorcycle.CollectionStoragePath)
		account.link<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath, target: NeoMotorcycle.CollectionStoragePath)


		emit ContractInitialized()
	}
}


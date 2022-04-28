/*
* Inspiration: https://flow-view-source.com/testnet/account/0xba1132bc08f82fe2/contract/Ghost
*/

import FungibleToken from 0xf233dcee88fe0abe

pub contract Profile {
	pub let publicPath: PublicPath
	pub let publicReceiverPath: PublicPath
	pub let storagePath: StoragePath

	//and event emitted when somebody follows another user
	pub event Follow(follower:Address, following: Address, tags: [String])

	//an event emitted when somebody unfollows somebody
	pub event Unfollow(follower:Address, unfollowing: Address)

	//and event emitted when a user verifies something
	pub event Verification(account:Address, message:String)

	/* 
	Represents a Fungible token wallet with a name and a supported type.
	*/
	pub struct Wallet {
		pub let name: String
		pub let receiver: Capability<&{FungibleToken.Receiver}>
		pub let balance: Capability<&{FungibleToken.Balance}>
		pub let accept: Type
		pub let tags: [String]

		init(
			name: String,
			receiver: Capability<&{FungibleToken.Receiver}>,
			balance: Capability<&{FungibleToken.Balance}>,
			accept: Type,
			tags: [String]
		) {
			self.name=name
			self.receiver=receiver
			self.balance=balance
			self.accept=accept
			self.tags=tags
		}
	}

	/*

	Represent a collection of a Resource that you want to expose
	Since NFT standard is not so great at just add Type and you have to use instanceOf to check for now
	*/
	pub struct ResourceCollection {
		pub let collection: Capability
		pub let tags: [String]
		pub let type: Type
		pub let name: String

		init(name: String, collection:Capability, type: Type, tags: [String]) {
			self.name=name
			self.collection=collection
			self.tags=tags
			self.type=type
		}
	}


	pub struct CollectionProfile{
		pub let tags: [String]
		pub let type: String
		pub let name: String

		init(_ collection: ResourceCollection){
			self.name=collection.name
			self.type=collection.type.identifier
			self.tags=collection.tags
		}
	}

	/*
	A link that you could add to your profile
	*/
	pub struct Link {
		pub let url: String
		pub let title: String
		pub let type: String

		init(title: String, type: String, url: String) {
			self.url=url
			self.title=title
			self.type=type
		}
	}

	/*
	Information about a connection between one profile and another.
	*/
	pub struct FriendStatus {
		pub let follower: Address
		pub let following:Address
		pub let tags: [String]

		init(follower: Address, following:Address, tags: [String]) {
			self.follower=follower
			self.following=following 
			self.tags= tags
		}
	}

	pub struct WalletProfile {
		pub let name: String
		pub let balance: UFix64
		pub let accept:  String
		pub let tags: [String] 

		init(_ wallet: Wallet) {
			self.name=wallet.name
			self.balance=wallet.balance.borrow()?.balance ?? 0.0 
			self.accept=wallet.accept.identifier
			self.tags=wallet.tags
		}
	}

	pub struct UserProfile {
		pub let findName: String
		pub let createdAt: String
		pub let address: Address
		pub let name: String
		pub let gender: String
		pub let description: String
		pub let tags: [String]
		pub let avatar: String
		pub let links: [Link]
		pub let wallets: [WalletProfile]
		pub let collections: [CollectionProfile]
		pub let following: [FriendStatus]
		pub let followers: [FriendStatus]
		pub let allowStoringFollowers: Bool

		init(
			findName:String,
			address: Address,
			name: String,
			gender: String,
			description: String, 
			tags: [String],
			avatar: String, 
			links: [Link],
			wallets: [WalletProfile],
			collections: [CollectionProfile],
			following: [FriendStatus],
			followers: [FriendStatus],
			allowStoringFollowers:Bool,
			createdAt: String
		) {
			self.findName=findName
			self.address=address
			self.name=name
			self.gender=gender
			self.description=description
			self.tags=tags
			self.avatar=avatar
			self.links=links
			self.collections=collections
			self.wallets=wallets
			self.following=following
			self.followers=followers
			self.allowStoringFollowers=allowStoringFollowers
			self.createdAt=createdAt
		}
	}

	pub resource interface Public{
		pub fun getName(): String
		pub fun getFindName(): String
		pub fun getCreatedAt(): String
		pub fun getGender(): String
		pub fun getDescription(): String
		pub fun getTags(): [String]
		pub fun getAvatar(): String
		pub fun getCollections(): [ResourceCollection] 
		pub fun follows(_ address: Address) : Bool
		pub fun getFollowers(): [FriendStatus]
		pub fun getFollowing(): [FriendStatus]
		pub fun getWallets() : [Wallet]
		pub fun getLinks() : [Link]
		pub fun deposit(from: @FungibleToken.Vault)
		pub fun supportedFungigleTokenTypes() : [Type]
		pub fun asProfile() : UserProfile
		pub fun isBanned(_ val: Address): Bool
		pub fun isPrivateModeEnabled() : Bool
		//TODO: should getBanned be here?

		access(contract) fun internal_addFollower(_ val: FriendStatus)
		access(contract) fun internal_removeFollower(_ address: Address) 

	}

	pub resource interface Owner {
		pub fun setName(_ val: String) {
			pre {
				val.length <= 64: "Name must be 64 or less characters"
			}
		}

		pub fun setGender(_ val:String){
			pre {
				val.length <= 64: "Gender must be 64 or less characters"
			}
		}

		pub fun setAvatar(_ val: String){
			pre {
				val.length <= 1024: "Avatar must be 1024 characters or less"
			}
		}

		pub fun setTags(_ val: [String])  {
			pre {
				Profile.verifyTags(tags: val, tagLength:64, tagSize:32) : "cannot have more then 32 tags of length 64"
			}
		}   

		//validate length of description to be 255 or something?
		pub fun setDescription(_ val: String) {
			pre {
				val.length <= 1024: "Description must be 1024 characters or less"
			}
		}

		pub fun follow(_ address: Address, tags:[String]) {
			pre {
				Profile.verifyTags(tags: tags, tagLength:64, tagSize:32) : "cannot have more then 32 tags of length 64"
			}
		}
		pub fun unfollow(_ address: Address)

		pub fun removeCollection(_ val: String)
		pub fun addCollection(_ val: ResourceCollection)

		pub fun addWallet(_ val : Wallet) 
		pub fun removeWallet(_ val: String)
		pub fun setWallets(_ val: [Wallet])

		pub fun addLink(_ val: Link)
		pub fun removeLink(_ val: String)

		//Verify that this user has signed something.
		pub fun verify(_ val:String) 

		//A user must be able to remove a follower since this data in your account is added there by another user
		pub fun removeFollower(_ val: Address)

		//manage bans
		pub fun addBan(_ val: Address)
		pub fun removeBan(_ val: Address)
		pub fun getBans(): [Address]

		//Set if user is allowed to store followers or now
		pub fun setAllowStoringFollowers(_ val: Bool)

		//set if this user prefers sensitive information about his account to be kept private, no guarantee here but should be honored
		pub fun setPrivateMode(_ val: Bool)
	}


	pub resource User: Public, Owner, FungibleToken.Receiver {
		access(self) var name: String
		access(self) var findName: String
		access(self) var createdAt: String
		access(self) var gender: String
		access(self) var description: String
		access(self) var avatar: String
		access(self) var tags: [String]
		access(self) var followers: {Address: FriendStatus}
		access(self) var bans: {Address: Bool}
		access(self) var following: {Address: FriendStatus}
		access(self) var collections: {String: ResourceCollection}
		access(self) var wallets: [Wallet]
		access(self) var links: {String: Link}
		access(self) var allowStoringFollowers: Bool

		//this is just a bag of properties if we need more fields here, so that we can do it with contract upgrade
		access(self) var additionalProperties: {String : String}

		init(name:String, createdAt: String) {
			let randomNumber = (1 as UInt64) + (unsafeRandom() % 25)
			self.createdAt=createdAt
			self.name = name
			self.findName=""
			self.gender="" 
			self.description=""
			self.tags=[]
			self.avatar = "https://find.xyz/assets/img/avatars/avatar".concat(randomNumber.toString()).concat(".png")
			self.followers = {}
			self.following = {}
			self.collections={}
			self.wallets=[]
			self.links={}
			self.allowStoringFollowers=true
			self.bans={}
			self.additionalProperties={}

		}

		/// We do not have a seperate field for this so we use the additionalProperties 'bag' to store this in
		pub fun setPrivateMode(_ val: Bool) {
			var private="true"
			if !val{
				private="false"
			}
			self.additionalProperties["privateMode"]  = private
		}


		pub fun isPrivateModeEnabled() : Bool {
			let boolString= self.additionalProperties["privateMode"]
			if boolString== nil || boolString=="false" {
				return false
			}
			return true
		}

		pub fun addBan(_ val: Address) { self.bans[val]= true}
		pub fun removeBan(_ val:Address) { self.bans.remove(key: val) }
		pub fun getBans() : [Address] { return self.bans.keys }
		pub fun isBanned(_ val:Address) : Bool { return self.bans.containsKey(val)}

		pub fun setAllowStoringFollowers(_ val: Bool) {
			self.allowStoringFollowers=val
		}

		pub fun verify(_ val:String) {
			emit Verification(account: self.owner!.address, message:val)
		}

		pub fun asProfile() : UserProfile {
			let wallets: [WalletProfile]=[]
			for w in self.wallets {
				wallets.append(WalletProfile(w))
			}

			let collections:[CollectionProfile]=[]
			for c in self.getCollections() {
				collections.append(CollectionProfile(c))
			}

			return UserProfile(
				findName: self.getFindName(),
				address: self.owner!.address,
				name: self.getName(),
				gender: self.getGender(),
				description: self.getDescription(),
				tags: self.getTags(),
				avatar: self.getAvatar(),
				links: self.getLinks(),
				wallets: wallets, 
				collections: collections,
				following: self.getFollowing(),
				followers: self.getFollowers(),
				allowStoringFollowers: self.allowStoringFollowers,
				createdAt:self.getCreatedAt()
			)
		}

		pub fun getLinks() : [Link] {
			return self.links.values
		}

		pub fun addLink(_ val: Link) {
			self.links[val.title]=val
		}

		pub fun removeLink(_ val: String) {
			self.links.remove(key: val)
		}

		pub fun supportedFungigleTokenTypes() : [Type] { 
			let types: [Type] =[]
			for w in self.wallets {
				if !types.contains(w.accept) {
					types.append(w.accept)
				}
			}
			return types
		}

		pub fun deposit(from: @FungibleToken.Vault) {
			for w in self.wallets {
				if from.isInstance(w.accept) {
					w.receiver.borrow()!.deposit(from: <- from)
					return
				}
			} 
			let identifier=from.getType().identifier
			//TODO: I need to destroy here for this to compile, but WHY?
			destroy from
			panic("could not find a supported wallet for:".concat(identifier))
		}


		pub fun getWallets() : [Wallet] { return self.wallets}
		pub fun addWallet(_ val: Wallet) { self.wallets.append(val) }
		pub fun removeWallet(_ val: String) {
			let numWallets=self.wallets.length
			var i=0
			while(i < numWallets) {
				if self.wallets[i].name== val {
					self.wallets.remove(at: i)
					return
				}
				i=i+1
			}
		}

		pub fun setWallets(_ val: [Wallet]) { self.wallets=val }

		pub fun removeFollower(_ val: Address) {
			self.followers.remove(key:val)
		}

		pub fun follows(_ address: Address) : Bool {
			return self.following.containsKey(address)
		}

		pub fun getName(): String { return self.name }
		pub fun getFindName(): String { return self.findName }
		pub fun getCreatedAt(): String { return self.createdAt }
		pub fun getGender() : String { return self.gender }
		pub fun getDescription(): String{ return self.description}
		pub fun getTags(): [String] { return self.tags}
		pub fun getAvatar(): String { return self.avatar }
		pub fun getFollowers(): [FriendStatus] { return self.followers.values }
		pub fun getFollowing(): [FriendStatus] { return self.following.values }

		pub fun setName(_ val: String) { self.name = val }
		pub fun setFindName(_ val: String) { self.findName = val }
		pub fun setGender(_ val: String) { self.gender = val }
		pub fun setAvatar(_ val: String) { self.avatar = val }
		pub fun setDescription(_ val: String) { self.description=val}
		pub fun setTags(_ val: [String]) { self.tags=val}

		pub fun removeCollection(_ val: String) { self.collections.remove(key: val)}
		pub fun addCollection(_ val: ResourceCollection) { self.collections[val.name]=val}
		pub fun getCollections(): [ResourceCollection] { return self.collections.values}


		pub fun follow(_ address: Address, tags:[String]) {
			let friendProfile=Profile.find(address)
			let owner=self.owner!.address
			let status=FriendStatus(follower:owner, following:address, tags:tags)

			self.following[address] = status
			friendProfile.internal_addFollower(status)
			emit Follow(follower:owner, following: address, tags:tags)
		}

		pub fun unfollow(_ address: Address) {
			self.following.remove(key: address)
			Profile.find(address).internal_removeFollower(self.owner!.address)
			emit Unfollow(follower: self.owner!.address, unfollowing:address)
		}

		access(contract) fun internal_addFollower(_ val: FriendStatus) {
			if self.allowStoringFollowers && !self.bans.containsKey(val.follower) {
				self.followers[val.follower] = val
			}
		}

		access(contract) fun internal_removeFollower(_ address: Address) {
			if self.followers.containsKey(address) {
				self.followers.remove(key: address)
			}
		}

	}

	pub fun findWalletCapability(_ address: Address) : Capability<&{FungibleToken.Receiver}> {
		return getAccount(address)
		.getCapability<&{FungibleToken.Receiver}>(Profile.publicReceiverPath)
	}

	pub fun find(_ address: Address) : &{Profile.Public} {
		return getAccount(address)
		.getCapability<&{Profile.Public}>(Profile.publicPath)
		.borrow()!
	}

	pub fun createUser(name: String, createdAt:String) : @Profile.User {
		pre {
			name.length <= 64: "Name must be 64 or less characters"
			createdAt.length <= 32: "createdAt must be 32 or less characters"
		}
		return <- create Profile.User(name: name,createdAt: createdAt)
	}

	pub fun verifyTags(tags : [String], tagLength: Int, tagSize: Int): Bool {
		if tags.length > tagSize {
			return false
		}

		for t in tags {
			if t.length > tagLength {
				return false
			}
		}
		return true
	}

	init() {
		self.publicPath = /public/findProfile
		self.publicReceiverPath = /public/findProfileReceiver
		self.storagePath = /storage/findProfile
	}
}

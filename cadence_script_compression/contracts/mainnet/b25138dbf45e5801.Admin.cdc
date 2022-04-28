import Debug from 0xb25138dbf45e5801
import Clock from 0xb25138dbf45e5801
import NeoMotorcycle from 0xb25138dbf45e5801
import NeoFounder from 0xb25138dbf45e5801
import NeoMember from 0xb25138dbf45e5801
import NeoVoucher from 0xb25138dbf45e5801
import NeoAvatar from 0xb25138dbf45e5801
import NeoSticker from 0xb25138dbf45e5801
import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract Admin {

	/// The path to the proxy
	pub let AdminProxyPublicPath: PublicPath

	/// The path to storage of the porxy
	pub let AdminProxyStoragePath: StoragePath


	//Admin client to use for capability receiver pattern
	pub fun createAdminProxyClient() : @AdminProxy {
		return <- create AdminProxy()
	}

	//interface to use for capability receiver pattern
	pub resource interface AdminProxyClient {
		pub fun addCapability(_ cap: Capability<&NeoMotorcycle.Collection>)
	}

	//admin proxy with capability receiver 
	pub resource AdminProxy: AdminProxyClient {

		access(self) var capability: Capability<&NeoMotorcycle.Collection>?

		pub fun addCapability(_ cap: Capability<&NeoMotorcycle.Collection>) {
			pre {
				cap.check() : "Invalid server capablity"
				self.capability == nil : "Server already set"
			}
			self.capability = cap
		}


		pub fun addPhysicalLink(id: UInt64, physicalLink: String) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}
			let motorcycleCap= Admin.account.getCapability<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath)
			motorcycleCap.borrow()!.borrowNeoMotorcycle(id:id)!.addPhysicalLink(physicalLink)
		}

		pub fun setMotorcycleName(id:UInt64, name:String) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}
			let motorcycleCap= Admin.account.getCapability<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath)
			motorcycleCap.borrow()!.borrowNeoMotorcycle(id:id)!.setName(name)

		}

		pub fun registerNeoVoucherMetadata(typeID: UInt64, metadata: NeoVoucher.Metadata) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			NeoVoucher.registerMetadata(typeID: typeID, metadata: metadata)

		}

		pub fun batchMintNeoVoucher(count: Int) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}
			let recipient=Admin.account.getCapability<&{NonFungibleToken.CollectionPublic}>(NeoVoucher.CollectionPublicPath).borrow()!

			//We only have one type right now
			NeoVoucher.batchMintNFT(recipient: recipient, typeID: 1, count: count)
		}


		//This will consume the voucher and send the reward to the user
		pub fun consumeNeoVoucher(voucherID: UInt64, rewardID:UInt64) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			NeoVoucher.consume(voucherID: voucherID, rewardID:rewardID)

		}


		pub fun getNeoMember() : &NonFungibleToken.Collection {
			return Admin.account.borrow<&NonFungibleToken.Collection>(from: NeoMember.CollectionStoragePath) ?? panic("Could not borrow a reference to the admin's collection")
		}

		pub fun getNeoVouchers() : &NonFungibleToken.Collection {
			return Admin.account.borrow<&NonFungibleToken.Collection>(from: NeoVoucher.CollectionStoragePath) ?? panic("Could not borrow a reference to the admin's collection")
		}

		pub fun getWallet() : Capability<&{FungibleToken.Receiver}> {
			return Admin.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
		}

		pub fun addAchievementToMember(user:Address, memberId: UInt64, name:String, description:String) {
			let userAccount=getAccount(user)
			let memberCap=userAccount.getCapability<&{NeoMember.CollectionPublic}>(NeoMember.CollectionPublicPath)

			let member=memberCap.borrow()!
			member.addAchievement(id: memberId, achievement: NeoMotorcycle.Achievement( name:name, description:description))
		}

		pub fun addAchievementToFounder(user:Address, founderId: UInt64, name:String, description:String) {
			let userAccount=getAccount(user)
			let founderCap=userAccount.getCapability<&{NeoFounder.CollectionPublic}>(NeoFounder.CollectionPublicPath)

			let founder=founderCap.borrow()!
			founder.addAchievement(id: founderId, achievement: NeoMotorcycle.Achievement( name:name, description:description))
		}

		pub fun addAchievementToTeam(teamId: UInt64, name:String, description:String) {
			let motorcycleCap= Admin.account.getCapability<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath)
			motorcycleCap.borrow()!.borrowNeoMotorcycle(id: teamId)!.addAchievement(NeoMotorcycle.Achievement( name:name, description:description))
		}


		/*
		pub fun mintAndAddStickerToFounder(user:Address, founderId:UInt64, name:String, description:String, thumbnailHash:String) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			let userAccount=getAccount(user)
			let founderCap=userAccount.getCapability<&{NeoFounder.CollectionPublic}>(NeoFounder.CollectionPublicPath)

			let founder=founderCap.borrow()!
		  let nft <- NeoSticker.mintNeoSticker(name:name, description:description, thumbnailHash:thumbnailHash)
			founder.addSticker(id: founderId, sticker: <- nft)
			
		}

		pub fun mintAndAddStickerToMember(user:Address, memberId:UInt64, name:String, description:String, thumbnailHash:String) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			let userAccount=getAccount(user)
			let memberCap=userAccount.getCapability<&{NeoMember.CollectionPublic}>(NeoMember.CollectionPublicPath)

			let member=memberCap.borrow()!
		  let nft <- NeoSticker.mintNeoSticker(name:name, description:description, thumbnailHash:thumbnailHash)
			member.addSticker(id: memberId, sticker: <- nft)
			
		}
		*/

		pub fun mintNeoAvatar(teamId:UInt64, series:String, role:String, mediaHash:String, wallet: Capability<&{FungibleToken.Receiver}>, collection: Capability<&{NonFungibleToken.Receiver}>) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			NeoAvatar.mint(teamId:teamId, series:series, role:role, imageHash:mediaHash, wallet:wallet, collection:collection)
		}

		pub fun mintNeoMotorcycle(description: String, metadata: {String: String}) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			let motorscycle <- NeoMotorcycle.mint() 
			let id = motorscycle.id
			let motorcycleCap= Admin.account.getCapability<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath)
			motorcycleCap.borrow()!.deposit(token: <- motorscycle)

			let motorcyclePointer=NeoMotorcycle.Pointer(collection: motorcycleCap, id: id)
			if motorcyclePointer.resolve() == nil {
				panic("Invalid motorcycle id")
			}

			let nft  <- NeoFounder.mint(motorcyclePointer: motorcyclePointer, description:description) 
			let unique=Admin.account.borrow<&NeoFounder.Collection>(from: NeoFounder.CollectionStoragePath)!
			unique.deposit(token: <- nft)
		}


		pub fun mintNeoMember(edition: UInt64, maxEdition:UInt64, role: String, description: String, motorcycleId: UInt64) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}

			let motorcycleCap= Admin.account.getCapability<&NeoMotorcycle.Collection>(NeoMotorcycle.CollectionPrivatePath)
			let motorcyclePointer=NeoMotorcycle.Pointer(collection: motorcycleCap, id: motorcycleId)

			if motorcyclePointer.resolve() == nil {
				panic("Invalid motorcycle id")
			}

			let nft <- NeoMember.mint(edition: edition, maxEdition: maxEdition, role: role, description: description, motorcyclePointer: motorcyclePointer)

			let editioned=Admin.account.borrow<&NeoMember.Collection>(from: NeoMember.CollectionStoragePath)!
			editioned.deposit(token: <- nft)
		}


		/// Advance the clock and enable debug mode
		pub fun advanceClock(_ time: UFix64) {
			pre {
				self.capability != nil: "Cannot create Neo Admin, capability is not set"
			}
			Debug.enable()
			Clock.enable()
			Clock.tick(time)
		}


		init() {
			self.capability = nil
		}

	}

	init() {
		self.AdminProxyPublicPath= /public/neoAdminClient
		self.AdminProxyStoragePath=/storage/neoAdminClient
	}

}

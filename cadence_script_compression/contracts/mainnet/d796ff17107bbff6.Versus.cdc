import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import Art from 0xd796ff17107bbff6
import Content from 0xd796ff17107bbff6
import Auction from 0xd796ff17107bbff6
import Profile from 0xd796ff17107bbff6

/*
The main contract in the Versus auction system.

A versions auction contains a single auction and a group of auctions and either of them will be fulfilled while the other will be cancelled
Currently this is modeled as 1 vs x, but It could easily be modeled as x vs y  so you could have 5 editions vs 10 editions if you want to

The auctions themselves are not implemented in this contract but rather in the Auction contract. The goal here is to be able to
reuse the Auction contract for other things if somebody would want that.

*/
pub contract Versus {

	//A set of capability and storage paths used in this contract
	pub let VersusAdminPublicPath: PublicPath
	pub let VersusAdminStoragePath: StoragePath
	pub let CollectionStoragePath: StoragePath
	pub let CollectionPublicPath: PublicPath
	pub let CollectionPrivatePath: PrivatePath

	//counter for drops that is incremented every time there is a new versus drop made
	pub var totalDrops: UInt64

	//emitted when a drop is extended
	pub event DropExtended(name: String, artist: String, dropId: UInt64, extendWith: Fix64, extendTo: Fix64)

	pub event Bid(name: String, artist: String, edition:String, bidder: Address, price: UFix64, dropId: UInt64, auctionId:UInt64)
	//emitted when a bid is made
	pub event ExtendedBid(name: String, artist: String, edition:String, bidderAddress: Address, bidderName: String, price: UFix64, oldBidderAddress: Address?, oldBidderName:String, oldPrice:UFix64?, dropId: UInt64, auctionId:UInt64, auctionEndAt: Fix64, extendWith: Fix64, cacheKey: String, oldLeader:String, newLeader:String)

	//emitted when a drop is created
	pub event DropCreated(name: String, artist: String, editions: UInt64, owner:Address, dropId: UInt64)

	pub event DropDestroyed(dropId:UInt64)

	//emitted when a drop is settled, that is it ends and either the uniqe or the edition side wins
	pub event Settle(name: String, artist: String, winner: String, price:UFix64, dropId: UInt64)

	//emitted when the winning side in the auction changes
	pub event LeaderChanged(name: String, artist: String, winning: String, dropId:UInt64)

	//A Drop in versus represents a single auction vs an editioned auction
	pub resource Drop {

		access(contract) let uniqueAuction: @Auction.AuctionItem
		access(contract) let editionAuctions: @Auction.AuctionCollection
		access(contract) let dropID: UInt64

		//this is used to be able to query events for a drop from a given start point
		access(contract) var firstBidBlock: UInt64?
		access(contract) var settledAt: UInt64?

		access(contract) var extensionOnLateBid: UFix64


		//Store metadata here would allow us to show this after the drop has ended. The NFTS are gone then but the  metadta remains here
		access(contract) let metadata: Art.Metadata

		//these two together are a pointer to the content in the Drop. Storing them here means we can show the art after the drop has ended
		access(contract) var contentId: UInt64
		access(contract) var contentCapability: Capability<&Content.Collection>

		init( uniqueAuction: @Auction.AuctionItem, editionAuctions: @Auction.AuctionCollection, extensionOnLateBid: UFix64, contentId: UInt64, contentCapability: Capability<&Content.Collection>) {

			Versus.totalDrops = Versus.totalDrops + (1 as UInt64)

			self.dropID=Versus.totalDrops
			self.uniqueAuction <-uniqueAuction
			self.editionAuctions <- editionAuctions
			self.firstBidBlock=nil
			self.settledAt=nil
			self.metadata=self.uniqueAuction.getAuctionStatus().metadata!
			self.extensionOnLateBid=extensionOnLateBid
			self.contentId=contentId
			self.contentCapability=contentCapability
		}

		destroy(){
			log("Destroy versus")
			destroy self.uniqueAuction
			destroy self.editionAuctions
			emit DropDestroyed(dropId: self.dropID)
		}

		pub fun getContent() : String {
			let contentCollection= self.contentCapability.borrow()!
			return contentCollection.content(self.contentId)
		}

		//Returns a DropStatus struct that could be used in a script to show information about the drop
		pub fun getDropStatus() : DropStatus {
			let uniqueRef = &self.uniqueAuction as &Auction.AuctionItem
			let editionRef= &self.editionAuctions as &Auction.AuctionCollection

			let editionStatuses= editionRef.getAuctionStatuses()
			var editionPrice:UFix64= 0.0

			let editionDropAcutionStatus: {UInt64:DropAuctionStatus} = {}
			for es in editionStatuses.keys {
				var status=editionStatuses[es]!
				editionDropAcutionStatus[es] = DropAuctionStatus(status)
				editionPrice = editionPrice + status.price
			}

			let uniqueStatus=uniqueRef.getAuctionStatus()

			var winningStatus=""
			var difference=0.0
			if editionPrice > uniqueStatus.price {
				winningStatus="EDITIONED"
				difference = editionPrice - uniqueStatus.price
			} else if (editionPrice == uniqueStatus.price) {
				winningStatus="TIE"
				difference=0.0
			} else {
				difference=uniqueStatus.price - editionPrice
				winningStatus="UNIQUE"
			}

			let block=getCurrentBlock()
			let time=Fix64(block.timestamp)

			var started = uniqueStatus.startTime < time
			var active=true
			if !started {
				active=false
			} else if uniqueStatus.completed {
				active=false
			} else if uniqueStatus.expired && winningStatus != "TIE" {
				active=false
			}
			return DropStatus(
				dropId: self.dropID,
				uniqueStatus: uniqueStatus,
				editionsStatuses: editionDropAcutionStatus,
				editionPrice: editionPrice,
				status: winningStatus,
				firstBidBlock: self.firstBidBlock,
				difference: difference,
				metadata: self.metadata,
				settledAt: self.settledAt,
				active: active,
				startPrice: uniqueRef.startPrice
			)
		}

		pub fun calculateStatus(edition:UFix64, unique: UFix64) : String{
			var winningStatus=""
			if edition > unique{
				winningStatus="EDITIONED"
			} else if (edition== unique) {
				winningStatus="TIE"
			} else {
				winningStatus="UNIQUE"
			}
			return winningStatus
		}

		pub fun settle(cutPercentage:UFix64, vault: Capability<&{FungibleToken.Receiver}> ) {
			let status=self.getDropStatus()

			if status.settledAt != nil {
				panic("Drop has already been settled")
			}

			if status.expired == false {
				panic("Auction has not completed yet")
			}

			let winning=status.winning
			var price=0.0
			if winning == "UNIQUE" {
				self.uniqueAuction.settleAuction(cutPercentage: cutPercentage, cutVault: vault)
				self.cancelAllEditionedAuctions()
				price=status.uniquePrice
			} else if winning == "EDITIONED" {
				self.uniqueAuction.returnAuctionItemToOwner()
				self.settleAllEditionedAuctions()
				price=status.editionPrice
			} else {
				panic("tie")
			}

			self.settledAt=getCurrentBlock().height
			emit Settle(name: status.metadata.name, artist: status.metadata.artist, winner: winning, price: price, dropId: self.dropID )
		}


		pub fun settleAllEditionedAuctions() {
			for id in self.editionAuctions.keys() {
				self.editionAuctions.settleAuction(id)
			}
		}

		pub fun cancelAllEditionedAuctions() {
			for id in self.editionAuctions.keys() {
				self.editionAuctions.cancelAuction(id)
			}
		}

		priv fun getAuction(auctionId:UInt64): &Auction.AuctionItem {
			let dropStatus = self.getDropStatus()
			if self.uniqueAuction.auctionID == auctionId {
				return &self.uniqueAuction as &Auction.AuctionItem
			} else {
				let editionStatus=dropStatus.editionsStatuses[auctionId]!
				return &self.editionAuctions.auctionItems[auctionId] as &Auction.AuctionItem
			}
		}

		pub fun currentBidForUser( auctionId: UInt64, address:Address) : UFix64 {

			let auction=self.getAuction(auctionId:auctionId)
			return auction.currentBidForUser(address: address)

		}

		//place a bid on a given auction
		pub fun placeBid( auctionId:UInt64, bidTokens: @FungibleToken.Vault, vaultCap: Capability<&{FungibleToken.Receiver}>, collectionCap: Capability<&{Art.CollectionPublic}>) {

			pre {
				collectionCap.check() == true : "Collection capability must be linked"
				vaultCap.check() == true : "Vault capability must be linked"
			}

			let dropStatus = self.getDropStatus()
			var editionPrice=dropStatus.editionPrice
			var uniquePrice=dropStatus.uniquePrice
			let block=getCurrentBlock()
			let time=Fix64(block.timestamp)

			if dropStatus.startTime > time {
				panic("The drop has not started")
			}

			if dropStatus.endTime < time && dropStatus.winning != "TIE" {
				panic("This drop has ended")
			}

			let bidEndTime = time + Fix64(self.extensionOnLateBid)

			//we save the time of the first bid so that it can be used to fetch events from that given block
			if self.firstBidBlock == nil {
				self.firstBidBlock=block.height
			}

			var endTime=dropStatus.endTime
			var extendWith=(0.0 as Fix64)

			//We need to extend the auction since there is too little time left. If we did not do this a late user could potentially win with a cheecky bid
			if dropStatus.endTime < bidEndTime {
				 extendWith=bidEndTime - dropStatus.endTime
				endTime=bidEndTime
				emit DropExtended(name: dropStatus.metadata.name, artist: dropStatus.metadata.artist, dropId:self.dropID, extendWith: extendWith, extendTo: bidEndTime)
				self.extendDropWith(UFix64(extendWith))
			}

			let bidder=vaultCap.address
			let currentBidForUser= self.currentBidForUser(auctionId: auctionId, address: bidder)
			let bidPrice = bidTokens.balance + currentBidForUser

			var edition:String="1 of 1"


			var oldBidder : Address?=nil
			var oldPrice: UFix64?=nil
			//the bid is on a unique auction so we place the bid there
			if self.uniqueAuction.auctionID == auctionId {
				let auctionRef = &self.uniqueAuction as &Auction.AuctionItem
				oldBidder=dropStatus.uniqueStatus.leader
				oldPrice=dropStatus.uniquePrice
				uniquePrice=bidPrice
				auctionRef.placeBid(bidTokens: <- bidTokens, vaultCap:vaultCap, collectionCap:collectionCap)
			} else {
				editionPrice=editionPrice+bidTokens.balance
				let editionStatus=dropStatus.editionsStatuses[auctionId]!
				oldBidder=editionStatus.leader
				oldPrice=editionStatus.price
				edition=editionStatus.edition.toString().concat( " of ").concat(editionStatus.maxEdition.toString())
				let editionsRef = &self.editionAuctions as &Auction.AuctionCollection
				editionsRef.placeBid(id: auctionId, bidTokens: <- bidTokens, vaultCap:vaultCap, collectionCap:collectionCap)
			}

			emit Bid(name: dropStatus.metadata.name, artist:dropStatus.metadata.artist, edition: edition, bidder:bidder, price:bidPrice, dropId:self.dropID, auctionId:auctionId)

			let newStatus=self.calculateStatus(edition:editionPrice, unique: uniquePrice)

			if dropStatus.winning != newStatus {
				emit LeaderChanged(name:dropStatus.metadata.name, artist: dropStatus.metadata.artist, winning:newStatus, dropId: self.dropID)
			}


			var bidderName=""
			let bidderProfileCap= getAccount(bidder).getCapability<&{Profile.Public}>(Profile.publicPath)
			if bidderProfileCap.check() {
				bidderName=bidderProfileCap.borrow()!.getName()
			}

			var oldBidderName=""
			if oldBidder != nil {
				if oldBidder == bidder {
					oldBidderName=bidderName
				} else{
					let oldBidderProfileCap= getAccount(oldBidder!).getCapability<&{Profile.Public}>(Profile.publicPath)
					if oldBidderProfileCap.check() {
					 oldBidderName=oldBidderProfileCap.borrow()!.getName()
					}
				}
			}


			emit ExtendedBid(name: dropStatus.metadata.name, artist:dropStatus.metadata.artist, edition: edition, bidderAddress:bidder, bidderName: bidderName, price:bidPrice, oldBidderAddress: oldBidder, oldBidderName: oldBidderName, oldPrice: oldPrice, dropId:self.dropID, auctionId:auctionId, auctionEndAt: endTime, extendWith: extendWith, cacheKey: self.contentId.toString(), oldLeader: dropStatus.winning, newLeader: newStatus)
		}

		//This would make it possible to extend the drop with more time from an admin interface
		//here we just delegate to the auctions and extend them all
		pub fun extendDropWith(_ time: UFix64) {
			log("Drop extended with duration")
			self.uniqueAuction.extendWith(time)
			self.editionAuctions.extendAllAuctionsWith(time)
		}

	}


	//this is a simpler version of the Acution status since we do not need to duplicate all the fields
	//edition and maxEidtion will not be kept here after the auction has been settled.
	//Really not sure on how to handle showing historic drops so for now I will just leave it as it is
	pub struct DropAuctionStatus {
		pub let id: UInt64
		pub let price : UFix64
		pub let bidIncrement : UFix64
		pub let bids : UInt64
		pub let edition: UInt64
		pub let maxEdition: UInt64
		pub let leader: Address?
		pub let minNextBid: UFix64
		init(_ auctionStatus: Auction.AuctionStatus) {
			self.price=auctionStatus.price
			self.bidIncrement=auctionStatus.bidIncrement
			self.bids=auctionStatus.bids
			self.edition=auctionStatus.metadata?.edition  ?? (0 as UInt64)
			self.maxEdition=auctionStatus.metadata?.maxEdition ?? (0 as UInt64)
			self.leader=auctionStatus.leader
			self.minNextBid=auctionStatus.minNextBid
			self.id=auctionStatus.id
		}
	}

	//The struct that holds status information of a drop.
	//this probably has some duplicated data that could go away. like do you need both a settled and settledAt? and active?
	pub struct DropStatus {
		pub let dropId: UInt64
		pub let uniquePrice: UFix64
		pub let editionPrice: UFix64
		pub let difference: UFix64
		pub let endTime: Fix64
		pub let startTime: Fix64
		pub let uniqueStatus: DropAuctionStatus
		pub let editionsStatuses: {UInt64: DropAuctionStatus}
		pub let winning: String
		pub let active: Bool
		pub let timeRemaining: Fix64
		pub let firstBidBlock:UInt64?
		pub let metadata: Art.Metadata
		pub let expired: Bool
		pub let settledAt: UInt64?
		pub let startPrice: UFix64

		init(
			dropId: UInt64,
			uniqueStatus: Auction.AuctionStatus,
			editionsStatuses: {UInt64: DropAuctionStatus},
			editionPrice: UFix64,
			status: String,
			firstBidBlock:UInt64?,
			difference:UFix64,
			metadata: Art.Metadata,
			settledAt: UInt64?
			active: Bool
			startPrice: UFix64
		) {
			self.dropId=dropId
			self.uniqueStatus=DropAuctionStatus(uniqueStatus)
			self.editionsStatuses=editionsStatuses
			self.uniquePrice= uniqueStatus.price
			self.editionPrice= editionPrice
			self.endTime=uniqueStatus.endTime
			self.startTime=uniqueStatus.startTime
			self.timeRemaining=uniqueStatus.timeRemaining
			self.active= active
			self.winning=status
			self.firstBidBlock=firstBidBlock
			self.difference=difference
			self.metadata=metadata
			self.expired=uniqueStatus.expired
			self.settledAt=settledAt
			self.startPrice =startPrice
		}
	}

	//An resource interface that everybody can access through a public capability.
	pub resource interface PublicDrop {

		pub fun currentBidForUser(dropId: UInt64, auctionId: UInt64, address:Address) : UFix64
		pub fun getAllStatuses(): {UInt64: DropStatus}
		pub fun getCacheKeyForDrop(_ dropId: UInt64) : UInt64
		pub fun getStatus(dropId: UInt64): DropStatus

		pub fun getArt(dropId: UInt64): String

		pub fun placeBid(dropId: UInt64, auctionId:UInt64, bidTokens: @FungibleToken.Vault, vaultCap: Capability<&{FungibleToken.Receiver}>, collectionCap: Capability<&{Art.CollectionPublic}>)
	}

	pub resource interface AdminDrop {

		pub fun createDrop(nft: @NonFungibleToken.NFT, editions: UInt64, minimumBidIncrement: UFix64, minimumBidUniqueIncrement: UFix64, startTime: UFix64, startPrice: UFix64, vaultCap: Capability<&{FungibleToken.Receiver}>, duration: UFix64, extensionOnLateBid:UFix64)

		pub fun settle(_ dropId: UInt64)

	}

	pub resource DropCollection: PublicDrop, AdminDrop {

		access(account) var drops: @{UInt64: Drop}

		//it is possible to adjust the cutPercentage if you own a Versus.DropCollection
		access(account) var cutPercentage:UFix64

		access(account) let marketplaceVault: Capability<&{FungibleToken.Receiver}>

		//NFTs that are not sold are put here when a bid is settled.
		access(account) let marketplaceNFTTrash: Capability<&{Art.CollectionPublic}>

		init( marketplaceVault: Capability<&{FungibleToken.Receiver}>, marketplaceNFTTrash: Capability<&{Art.CollectionPublic}>, cutPercentage: UFix64) {
			self.marketplaceNFTTrash=marketplaceNFTTrash
			self.cutPercentage= cutPercentage
			self.marketplaceVault = marketplaceVault
			self.drops <- {}
		}

		pub fun withdraw(_ withdrawID: UInt64): @Drop {
			let token <- self.drops.remove(key: withdrawID) ?? panic("missing drop")
			return <-token
		}

		/// Set the cut percentage for versus

		/// @param cut: The cut percentage as a Ufix64 that versus will take for each drop
		pub fun setCutPercentage(_ cut: UFix64) {
			self.cutPercentage=cut
		}

		// When creating a drop you send in an NFT and the number of editions you want to sell vs the unique one
		// There will then be minted edition number of extra copies and put into the editions auction
		pub fun createDrop( nft: @NonFungibleToken.NFT, editions: UInt64, minimumBidIncrement: UFix64, minimumBidUniqueIncrement: UFix64, startTime: UFix64, startPrice: UFix64, vaultCap: Capability<&{FungibleToken.Receiver}>, duration: UFix64,
			extensionOnLateBid: UFix64) {

			pre {
				vaultCap.check() == true : "Vault capability should exist"
			}

			let art <- nft as! @Art.NFT

			let contentCapability= art.contentCapability!
			let contentId= art.contentId!

			let metadata= art.metadata
			//Sending in a NFTEditioner capability here and using that instead of this loop would probably make sense.
			let editionedAuctions <- Auction.createAuctionCollection( marketplaceVault: self.marketplaceVault , cutPercentage: self.cutPercentage)

			var currentEdition=(1 as UInt64)
			while currentEdition <= editions {
					editionedAuctions.createAuction( token: <- Art.makeEdition(original: &art as &Art.NFT, edition: currentEdition, maxEdition: editions), minimumBidIncrement: minimumBidIncrement, auctionLength: duration, auctionStartTime:startTime, startPrice: startPrice, collectionCap: self.marketplaceNFTTrash, vaultCap: vaultCap)
					currentEdition=currentEdition+(1 as UInt64)
			}

			//copy the metadata of the previous art since that is used to mint the copies
			let item <- Auction.createStandaloneAuction( token: <- art, minimumBidIncrement: minimumBidUniqueIncrement, auctionLength: duration, auctionStartTime: startTime, startPrice: startPrice, collectionCap: self.marketplaceNFTTrash, vaultCap: vaultCap)

			let drop  <- create Drop( uniqueAuction: <- item, editionAuctions:  <- editionedAuctions, extensionOnLateBid: extensionOnLateBid, contentId: contentId, contentCapability: contentCapability)
			emit DropCreated(name: metadata.name, artist: metadata.artist,  editions: editions, owner: vaultCap.address, dropId: drop.dropID)

			let oldDrop <- self.drops[drop.dropID] <- drop
			destroy oldDrop
		}

  	//Get all the drop statuses
  	pub fun getAllStatuses(): {UInt64: DropStatus} {
  		var dropStatus: {UInt64: DropStatus }= {}
  		for id in self.drops.keys {
  			let itemRef = &self.drops[id] as? &Drop
  			dropStatus[id] = itemRef.getDropStatus()
  		}
  		return dropStatus
  	}

  	access(contract) fun getDrop(_ dropId:UInt64) : &Drop {
  		pre {
  			self.drops[dropId] != nil:
  			"drop doesn't exist"
  		}
  		return &self.drops[dropId] as &Drop
  	}

		pub fun getDropByCacheKey(_ cacheKey: UInt64) : DropStatus? {
			var dropStatus: {UInt64: DropStatus }= {}
			for id in self.drops.keys {
				let itemRef = &self.drops[id] as? &Drop
				if itemRef.contentId == cacheKey {
					return itemRef.getDropStatus()
				}
			}
			return nil
		}

		pub fun getCacheKeyForDrop(_ dropId: UInt64) : UInt64 {
			return self.getDrop(dropId).contentId
		}

		pub fun getStatus(dropId:UInt64): DropStatus {
			return self.getDrop(dropId).getDropStatus()
		}

		//get the art for this drop
		pub fun getArt(dropId:UInt64) : String {
			return self.getDrop(dropId).getContent()
		}

		pub fun getArtType(dropId:UInt64): String {
			return self.getDrop(dropId).metadata.type
		}

		//settle a drop
		pub fun settle(_ dropId: UInt64) {
			self.getDrop(dropId).settle(cutPercentage: self.cutPercentage, vault: self.marketplaceVault)
		}
		
		pub fun currentBidForUser( dropId: UInt64, auctionId: UInt64, address:Address) : UFix64 {
			return  self.getDrop(dropId).currentBidForUser(
				auctionId: auctionId,
				address:address
			)
		}

		//place a bid, will just delegate to the method in the drop collection
		pub fun placeBid(dropId: UInt64, auctionId:UInt64, bidTokens: @FungibleToken.Vault, vaultCap: Capability<&{FungibleToken.Receiver}>, collectionCap: Capability<&{Art.CollectionPublic}>) {
			self.getDrop(dropId).placeBid( auctionId: auctionId, bidTokens: <- bidTokens, vaultCap: vaultCap, collectionCap:collectionCap)
		}

		destroy() {
			destroy self.drops
		}
	}

	// Get the art stored on chain for this drop
	pub fun getArtForDrop(_ dropId: UInt64) : String? {
		let versusCap=Versus.account.getCapability<&{Versus.PublicDrop}>(self.CollectionPublicPath)
		if let versus = versusCap.borrow()  {
			return versus.getArt(dropId: dropId)
		}
		return nil
	}

	/*
	Get an active drop in the versus marketplace

	*/
	pub fun getDrops() : [Versus.DropStatus]{
		let account = Versus.account
		let versusCap=account.getCapability<&{Versus.PublicDrop}>(self.CollectionPublicPath)!
		return versusCap.borrow()!.getAllStatuses().values
	}

	pub fun getDrop(_ id: UInt64) : Versus.DropStatus? {
		let account = Versus.account
		let versusCap=account.getCapability<&{Versus.PublicDrop}>(Versus.CollectionPublicPath)
		if let versus = versusCap.borrow() {
			return versus.getStatus(dropId: id)
		}
		return nil
	}

	/*
	Get the first active drop in the versus marketplace
	*/
	pub fun getActiveDrop() : Versus.DropStatus?{
		// get the accounts' public address objects
		let account = Versus.account

		let versusCap=account.getCapability<&{Versus.PublicDrop}>(self.CollectionPublicPath)
		if let versus = versusCap.borrow() {
			let versusStatuses=versus.getAllStatuses()
			for s in versusStatuses.keys {
				let status = versusStatuses[s]!
				if status.active != false {
					return status
				}
			}
		}
		return nil
	}



	//The interface used to add a Versus Drop Collection capability to a AdminPublic
	pub resource interface AdminPublic {
		pub fun addCapability(_ cap: Capability<&Versus.DropCollection>)
	}

	//The versus admin resource that a client will create and store, then link up a public AdminPublic
	pub resource Admin: AdminPublic {

		access(self) var server: Capability<&Versus.DropCollection>?

		init() {
			self.server = nil
		}

		pub fun addCapability(_ cap: Capability<&Versus.DropCollection>) {
			pre {
				cap.check() : "Invalid server capablity"
				self.server == nil : "Server already set"
			}
			self.server = cap
		}

		// This will settle/end an auction
		pub fun settle(_ dropId: UInt64) {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			self.server!.borrow()!.settle(dropId)

			//since settling will return all items not sold to the NFTTrash, we take out the trash here.
			let artC=Versus.account.borrow<&NonFungibleToken.Collection>(from: Art.CollectionStoragePath)!
			for key in artC.ownedNFTs.keys{
				log("burning art with key=".concat(key.toString()))
				destroy <- artC.ownedNFTs.remove(key: key)
			}
		}

		pub fun setVersusCut(_ num:UFix64) {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}

			let dc:&Versus.DropCollection=self.server!.borrow()!
			dc.setCutPercentage(num)
		}

		pub fun createDrop(
			nft: @NonFungibleToken.NFT,
			editions: UInt64,
			minimumBidIncrement: UFix64,
			minimumBidUniqueIncrement: UFix64,
			startTime: UFix64,
			startPrice: UFix64,  //TODO: seperate startPrice for unique and edition
			vaultCap: Capability<&{FungibleToken.Receiver}>
			duration: UFix64,
			extensionOnLateBid: UFix64)  {

				pre {
					self.server != nil : "Your client has not been linked to the server"
				}

				self.server!.borrow()!.createDrop(nft: <- nft,
				editions:editions,
				minimumBidIncrement:minimumBidIncrement,
				minimumBidUniqueIncrement:minimumBidUniqueIncrement,
				startTime:startTime,
				startPrice:startPrice,
				vaultCap:vaultCap,
				duration: duration,
				extensionOnLateBid: extensionOnLateBid
			)
		}

		/* A stored Transaction to mintArt on versus to a given artist */
		pub fun mintArt(artist: Address, artistName: String, artName: String, content:String, description: String, type:String, artistCut: UFix64, minterCut:UFix64) : @Art.NFT{

			pre {
				self.server != nil : "Your client has not been linked to the server"
			}

			let artistAccount = getAccount(artist)
			var contentItem  <- Content.createContent(content)
			let contentId= contentItem.id
			let contentCapability=Versus.account.getCapability<&Content.Collection>(Content.CollectionPrivatePath)
			contentCapability.borrow()!.deposit(token: <- contentItem)

			let artistWallet= artistAccount.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
			let minterWallet= Versus.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)

			let royalty = {
				"artist" : Art.Royalty(wallet: artistWallet, cut: artistCut),
				"minter" : Art.Royalty(wallet: minterWallet, cut: minterCut)
			}
			let art <- Art.createArtWithPointer(name: artName, artist:artistName, artistAddress : artist, description: description, type: type, contentCapability: contentCapability, contentId: contentId, royalty: royalty)
			return <- art
		}


		pub fun editionArt(art: &Art.NFT, edition: UInt64, maxEdition: UInt64) : @Art.NFT {
			return <- Art.makeEdition(original: art, edition: edition, maxEdition: maxEdition)
		}

		pub fun editionAndDepositArt(art: &Art.NFT, to: [Address]) {

			let maxEdition:UInt64=UInt64(to.length)

			var i:UInt64=1

			for address in to {
				let editionedArt <- Art.makeEdition(original: art, edition: i, maxEdition: maxEdition)
				let account= getAccount(address)
				var collectionCap = account.getCapability<&{Art.CollectionPublic}>(Art.CollectionPublicPath)
				collectionCap.borrow()!.deposit(token: <- editionedArt)
				i=i+(1 as UInt64)
			}
		}

		pub fun getContent():&Content.Collection {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			return Versus.account.borrow<&Content.Collection>(from: Content.CollectionStoragePath)!
		}

		pub fun getFlowWallet():&FungibleToken.Vault {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			return Versus.account.borrow<&FungibleToken.Vault>(from: /storage/flowTokenVault)!
		}

		pub fun getArtCollection() : &NonFungibleToken.Collection {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			return Versus.account.borrow<&NonFungibleToken.Collection>(from: Art.CollectionStoragePath)!
		}

		pub fun getDropCollection(): &Versus.DropCollection {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			return self.server!.borrow()!
		}

		pub fun getVersusProfile(): &Profile.User {
			pre {
				self.server != nil : "Your client has not been linked to the server"
			}
			return Versus.account.borrow<&Profile.User>(from: Profile.storagePath)!
		}

	}

	//make it possible for a user that wants to be a versus admin to create the client
	pub fun createAdminClient(): @Admin {
		return <- create Admin()
	}

	//initialize all the paths and create and link up the admin proxy
	//init is only executed on initial deployment
	init() {

		self.CollectionPublicPath= /public/versusCollection
		self.CollectionPrivatePath= /private/versusCollection
		self.CollectionStoragePath= /storage/versusCollection
		self.VersusAdminPublicPath= /public/versusAdmin
		self.VersusAdminStoragePath=/storage/versusAdmin

		self.totalDrops = (0 as UInt64)

		let account=self.account

		let marketplaceReceiver=account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
		let marketplaceNFTTrash: Capability<&{Art.CollectionPublic}> =account.getCapability<&{Art.CollectionPublic}>(Art.CollectionPublicPath)

		log("Setting up versus capability")
		let collection <- create DropCollection(
			marketplaceVault: marketplaceReceiver,
			marketplaceNFTTrash: marketplaceNFTTrash,
			cutPercentage: 0.15
		)
		account.save(<-collection, to: Versus.CollectionStoragePath)
		account.link<&{Versus.PublicDrop}>(Versus.CollectionPublicPath, target: Versus.CollectionStoragePath)
		account.link<&Versus.DropCollection>(Versus.CollectionPrivatePath, target: Versus.CollectionStoragePath)
	}

}

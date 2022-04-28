/* SPDX-License-Identifier: UNLICENSED */

import DimeCollectibleV2 from 0xf5cdaace879e5a79
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import NonFungibleToken from 0x1d7e57aa55817448

/*
	This contract allows:
	- Anyone to create Sale Offers and place them in their storefront, making it
	  publicly accessible.
	- Anyone to accept the offer and buy the item.
	- The Dime admin account to accept offers without transferring tokens
 */

pub contract DimeStorefrontV2 {

	// SaleOffer events
	// A sale offer has been created.
	pub event SaleOfferCreated(itemId: UInt64, price: UFix64)
	// Someone has purchased an item that was offered for sale.
	pub event SaleOfferAccepted(itemId: UInt64)
	// A sale offer has been destroyed, with or without being accepted.
	pub event SaleOfferFinished(itemId: UInt64)

	// A sale offer has been removed from the collection of Address.
	pub event SaleOfferRemoved(itemId: UInt64, owner: Address)

	// A sale offer has been inserted into the collection of Address.
	pub event SaleOfferAdded(
		itemId: UInt64,
		creators: [Address],
		content: String,
		owner: Address,
		price: UFix64
	)

	// Named paths
	pub let StorefrontStoragePath: StoragePath
	pub let StorefrontPublicPath: PublicPath

	// An interface providing a read-only view of a SaleOffer
	pub resource interface SaleOfferPublic {
		pub let itemId: UInt64
		pub let creator: Address
		pub let creators: [Address]

		pub let content: String
		pub let hasHiddenContent: Bool
		pub fun getHistory(): [[AnyStruct]]

		pub var price: UFix64
		pub let dimeRoyalties: UFix64
		pub fun getRoyalties(): DimeCollectibleV2.Royalties
	}

	// A DimeCollectibleV2 NFT being offered to sale for a set fee
	pub resource SaleOffer: SaleOfferPublic {
		// Whether the sale has completed with someone purchasing the item.
		pub var saleCompleted: Bool

		// The collection containing the NFT.
		access(self) let sellerItemProvider: Capability<&DimeCollectibleV2.Collection{NonFungibleToken.Provider}>

		// The vault that will be paid when the item is purchased.
		// This isn't used right now since FUSD payments are not enabled,
		// but keeping for future compatibility
		access(self) let receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>

		// The NFT for sale.
		pub let itemId: UInt64
		pub let creator: Address
		pub let creators: [Address]

		pub let content: String
		pub let hasHiddenContent: Bool
		access(self) let history: [[AnyStruct]]

		pub var price: UFix64
		// The fraction of the sale that goes to Dime
		pub let dimeRoyalties: UFix64
		access(self) let creatorRoyalties: DimeCollectibleV2.Royalties

		pub fun getRoyalties(): DimeCollectibleV2.Royalties {
			return self.creatorRoyalties
		}

		pub fun getHistory(): [[AnyStruct]] {
			return self.history
		}

		destroy() {
			// Whether the sale completed or not, publicize that it is being withdrawn.
			emit SaleOfferFinished(itemId: self.itemId)
		}

		// Take the information required to create a sale offer
		init(nft: &DimeCollectibleV2.NFT, sellerItemProvider: Capability<&DimeCollectibleV2.Collection{NonFungibleToken.Provider}>,
			price: UFix64, receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>, dimeRoyalties: UFix64,
			creatorRoyalties: DimeCollectibleV2.Royalties) {
			self.saleCompleted = false
			self.sellerItemProvider = sellerItemProvider
			self.receiver = receiver

			self.itemId = nft.id
			self.creator = nft.getCreators()[0]
			self.creators = nft.getCreators()

			self.content = nft.content
			self.hasHiddenContent = nft.hasHiddenContent()
			self.history = nft.getHistory()

			self.price = price
			self.dimeRoyalties = dimeRoyalties
			self.creatorRoyalties = creatorRoyalties

			emit SaleOfferCreated(itemId: self.itemId, price: self.price)
		}

		pub fun setPrice(newPrice: UFix64) {
			self.price = newPrice
		}
	}

	// An interface for adding and removing SaleOffers to a collection, intended for
	// use by the collection's owner
	pub resource interface StorefrontManager {
		pub fun createSaleOffers(
			seller: Address,
			itemProvider: Capability<&DimeCollectibleV2.Collection{DimeCollectibleV2.DimeCollectionPublic, NonFungibleToken.Provider}>,
			itemsAndPrices: {UInt64: UFix64},
			receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>
		)
		pub fun removeSaleOffer(itemId: UInt64, beingPurchased: Bool)
		pub fun changePrice(itemId: UInt64, newPrice: UFix64)
	}

	// An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers in a collection
	pub resource interface StorefrontPublic {
		pub fun getSaleOfferIds(): [UInt64]
		pub fun borrowSaleOffer(itemId: UInt64): &SaleOffer{SaleOfferPublic}?
   	}

	// A resource that allows its owner to manage a list of SaleOffers, and purchasers to interact with them
	pub resource Storefront : StorefrontManager, StorefrontPublic {
		access(self) var saleOffers: @{UInt64: SaleOffer}

		// Returns an array of the Ids that are in the collection
		pub fun getSaleOfferIds(): [UInt64] {
			return self.saleOffers.keys
		}

		// Returns an Optional read-only view of the SaleItem for the given itemId if it is contained by this collection.
		// The optional will be nil if the provided itemId is not present in the collection.
		pub fun borrowSaleOffer(itemId: UInt64): &SaleOffer{SaleOfferPublic}? {
			if self.saleOffers[itemId] == nil {
				return nil
			}
			return &self.saleOffers[itemId] as &SaleOffer{SaleOfferPublic}
		}

		// Insert a SaleOffer into the collection, replacing one with the same itemId if present
		pub fun createSaleOffers(
			seller: Address,
			itemProvider: Capability<&DimeCollectibleV2.Collection{DimeCollectibleV2.DimeCollectionPublic, NonFungibleToken.Provider}>,
			itemsAndPrices: {UInt64: UFix64},
			receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>
		) {
			assert(itemProvider.borrow() != nil, message: "Couldn't get a capability to the creator's collection")

			for itemId in itemsAndPrices.keys {
				let nft = itemProvider.borrow()!.borrowCollectible(id: itemId) ?? panic("Couldn't borrow nft from seller")
				if (!nft.tradeable) {
					panic("Tried to put an untradeable item on sale")
				}

				// Values for an initial sale
				var dimeRoyalties = 0.1
				var creatorRoyalties = DimeCollectibleV2.Royalties(recipients: {})

				// Values for a secondary sale
				if (!nft.getCreators().contains(seller)) {
					dimeRoyalties = 0.01
					creatorRoyalties = nft.getRoyalties()
				}
			
				let newOffer <- create SaleOffer(
					nft: nft,
					sellerItemProvider: itemProvider,
					price: itemsAndPrices[itemId]!,
					receiver: receiver,
					dimeRoyalties: dimeRoyalties,
					creatorRoyalties: nft.getRoyalties()
				)

				// Add the new offer to the dictionary, overwriting an old one if it exists
				let oldOffer <- self.saleOffers[itemId] <- newOffer
				destroy oldOffer

				emit SaleOfferAdded(
					itemId: itemId,
					creators: nft.getCreators(),
					content: nft.content,
					owner: self.owner?.address!,
					price: itemsAndPrices[itemId]!
				)
			}
		}

		// Remove and return a SaleOffer from the collection
		pub fun removeSaleOffer(itemId: UInt64, beingPurchased: Bool) {
			let offer <- (self.saleOffers.remove(key: itemId) ?? panic("missing SaleOffer"))
			if beingPurchased {
				emit SaleOfferAccepted(itemId: itemId)
			} else {
				emit SaleOfferRemoved(itemId: itemId, owner: self.owner?.address!)
			}
			destroy offer
		}

		access(contract) fun push(offer: @SaleOffer) {
			let oldOffer <- self.saleOffers[offer.itemId] <- offer
			destroy oldOffer
		}

		access(contract) fun pop(itemId: UInt64): @SaleOffer? {
			let offer <- self.saleOffers.remove(key: itemId)
			return <- offer
		}

		pub fun changePrice(itemId: UInt64, newPrice: UFix64) {
			pre {
				self.saleOffers[itemId] != nil: "Tried to change price of an item that's not on sale"
			}

			let offer <- self.pop(itemId: itemId)!
			offer.setPrice(newPrice: newPrice)
			self.push(offer: <- offer)
		}

		destroy () {
			destroy self.saleOffers
		}

		init () {
			self.saleOffers <- {}
		}
	}

	// Make creating a Storefront publicly accessible.
	pub fun createStorefront(): @Storefront {
		return <-create Storefront()
	}

	init () {
		self.StorefrontStoragePath = /storage/DimeStorefrontV2
		self.StorefrontPublicPath = /public/DimeStorefrontV2
	}
}
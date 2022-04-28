import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import DevryCoin from 0x56c57a1d04b82fa8
import BSListings from 0x56c57a1d04b82fa8

pub contract BSMarket {
  // SaleOffer events.
  //
  // A sale offer has been created.
  pub event SaleOfferCreated(nftID: UInt64, price: UFix64, beneficiaries: {Address: UFix64})
  // Someone has purchased an item that was offered for sale.
  pub event SaleOfferAccepted(nftID: UInt64)
  // A sale offer has been destroyed, with or without being accepted.
  pub event SaleOfferFinished(nftID: UInt64)

  // Collection events.
  //
  // A sale offer has been inserted into the collection of Address.
  pub event CollectionInsertedSaleOffer(saleNFTID: UInt64, saleItemCollection: Address)
  // A sale offer has been removed from the collection of Address.
  pub event CollectionRemovedSaleOffer(saleNFTID: UInt64, saleItemCollection: Address)

  // Named paths
  //
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath

  // SaleOfferPublicView
  // An interface providing a read-only view of a SaleOffer
  //
  pub resource interface SaleOfferPublicView {
    pub var saleCompleted: Bool
    pub let saleNFTID: UInt64
    pub let salePrice: UFix64
  }

  // SaleOffer
  // A BSListings NFT being offered to sale for a set fee paid in DevryCoin.
  //
  pub resource SaleOffer: SaleOfferPublicView {
    // Whether the sale has completed with someone purchasing the item.
    pub var saleCompleted: Bool

    // The BSListings NFT ID for sale.
    pub let saleNFTID: UInt64
    // The collection containing that ID.
    access(self) let sellerItemProvider: Capability<&BSListings.Collection{BSListings.BSListingsCollectionPublic, NonFungibleToken.Provider}>

    // The sale payment price.
    pub let salePrice: UFix64

    // The DevryCoin vault that will receive that payment if the sale completes successfully.
    access(self) let sellerPaymentReceiver: Capability<&DevryCoin.Vault{FungibleToken.Receiver}>

    access(self) let beneficiaries: {Address: UFix64}

    // Called by a purchaser to accept the sale offer.
    // If they send the correct payment in DevryCoin, and if the item is still available,
    // the BSListings NFT will be placed in their BSListings.Collection.
    //
    pub fun accept(
      buyerCollection: &BSListings.Collection{NonFungibleToken.Receiver},
      buyerPayment: @FungibleToken.Vault
    ) {
      pre {
        buyerPayment.balance == self.salePrice: "payment does not equal offer price"
        self.saleCompleted == false: "the sale offer has already been accepted"
      }

      self.saleCompleted = true

      // Deposit into the beneficiaries' accounts
      for beneficiary in self.beneficiaries.keys {
        let percentage = self.beneficiaries[beneficiary]!
        let beneficiaryRef = getAccount(beneficiary)
              .getCapability(DevryCoin.ReceiverPublicPath)
              .borrow<&{FungibleToken.Receiver}>()

        let beneficiaryCut <- buyerPayment.withdraw(amount: self.salePrice * percentage)
        beneficiaryRef!.deposit(from: <-beneficiaryCut)
      }

      // Deposit the remainder into the seller's account
      self.sellerPaymentReceiver.borrow()!.deposit(from: <-buyerPayment)

      let nft <- self.sellerItemProvider.borrow()!.withdraw(withdrawID: self.saleNFTID)
      buyerCollection.deposit(token: <-nft)

      emit SaleOfferAccepted(nftID: self.saleNFTID)
    }

    // destructor
    //
    destroy() {
      // Whether the sale completed or not, publicize that it is being withdrawn.
      emit SaleOfferFinished(nftID: self.saleNFTID)
    }

    // initializer
    // Take the information required to create a sale offer, notably the capability
    // to transfer the BSListings NFT and the capability to receive DevryCoin in payment.
    //
    init(
      sellerItemProvider: Capability<&BSListings.Collection{BSListings.BSListingsCollectionPublic, NonFungibleToken.Provider}>,
      saleNFTID: UInt64,
      sellerPaymentReceiver: Capability<&DevryCoin.Vault{FungibleToken.Receiver}>,
      salePrice: UFix64,
      beneficiaries: {Address: UFix64}
    ) {
      pre {
        sellerItemProvider.borrow() != nil: "Cannot borrow seller"
        sellerPaymentReceiver.borrow() != nil: "Cannot borrow sellerPaymentReceiver"
      }

      let borrowedItemProvider = sellerItemProvider.borrow()!

      if (!borrowedItemProvider.getIDs().contains(saleNFTID)) {
        panic("NFT not found in the seller collection")
      }

      self.saleCompleted = false

      self.sellerItemProvider = sellerItemProvider
      self.saleNFTID = saleNFTID

      self.sellerPaymentReceiver = sellerPaymentReceiver
      self.salePrice = salePrice
      self.beneficiaries = beneficiaries

      // Check beneficiary percentages
      var total: UFix64 = 0.0
      for percentage in beneficiaries.values {
        total = total + percentage
      }

      if (total > 1.0) {
        panic("Beneficiary total cannot exceed 100% of future sales")
      }

      emit SaleOfferCreated(nftID: self.saleNFTID, price: self.salePrice, beneficiaries: self.beneficiaries)
    }
  }

  // createSaleOffer
  // Make creating a SaleOffer publicly accessible.
  //
  pub fun createSaleOffer(
    sellerItemProvider: Capability<&BSListings.Collection{BSListings.BSListingsCollectionPublic, NonFungibleToken.Provider}>,
    saleNFTID: UInt64,
    sellerPaymentReceiver: Capability<&DevryCoin.Vault{FungibleToken.Receiver}>,
    salePrice: UFix64,
    beneficiaries: {Address: UFix64}
  ): @SaleOffer {
    return <-create SaleOffer(
      sellerItemProvider: sellerItemProvider,
      saleNFTID: saleNFTID,
      sellerPaymentReceiver: sellerPaymentReceiver,
      salePrice: salePrice,
      beneficiaries: beneficiaries
    )
  }

  // CollectionManager
  // An interface for adding and removing SaleOffers to a collection, intended for
  // use by the collection's owner.
  //
  pub resource interface CollectionManager {
    pub fun insert(offer: @BSMarket.SaleOffer)
    pub fun remove(saleNFTID: UInt64): @SaleOffer
  }

  // CollectionPurchaser
  // An interface to allow purchasing items via SaleOffers in a collection.
  // This function is also provided by CollectionPublic, it is here to support
  // more fine-grained access to the collection for as yet unspecified future use cases.
  //
  pub resource interface CollectionPurchaser {
    pub fun purchase(
      saleNFTID: UInt64,
      buyerCollection: &BSListings.Collection{NonFungibleToken.Receiver},
      buyerPayment: @FungibleToken.Vault
    )
  }

  // CollectionPublic
  // An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers in a collection.
  //
  pub resource interface CollectionPublic {
    pub fun getSaleOfferIDs(): [UInt64]
    pub fun borrowSaleItem(saleNFTID: UInt64): &SaleOffer{SaleOfferPublicView}?
    pub fun purchase(
      saleNFTID: UInt64,
      buyerCollection: &BSListings.Collection{NonFungibleToken.Receiver},
      buyerPayment: @FungibleToken.Vault
    )
  }

  // Collection
  // A resource that allows its owner to manage a list of SaleOffers, and purchasers to interact with them.
  //
  pub resource Collection : CollectionManager, CollectionPurchaser, CollectionPublic {
    pub var saleOffers: @{UInt64: SaleOffer}

    // insert
    // Insert a SaleOffer into the collection, replacing one with the same saleNFTID if present.
    //
    pub fun insert(offer: @BSMarket.SaleOffer) {
      let id: UInt64 = offer.saleNFTID

      // add the new offer to the dictionary which removes the old one
      let oldOffer <- self.saleOffers[id] <- offer
      destroy oldOffer

      emit CollectionInsertedSaleOffer(saleNFTID: id, saleItemCollection: self.owner?.address!)
    }

    // remove
    // Remove and return a SaleOffer from the collection.
    pub fun remove(saleNFTID: UInt64): @SaleOffer {
      emit CollectionRemovedSaleOffer(saleNFTID: saleNFTID, saleItemCollection: self.owner?.address!)
      return <-(self.saleOffers.remove(key: saleNFTID) ?? panic("missing SaleOffer"))
    }

    // purchase
    // If the caller passes a valid saleNFTID and the item is still for sale, and passes a DevryCoin vault
    // typed as a FungibleToken.Vault (DevryCoin.deposit() handles the type safety of this)
    // containing the correct payment amount, this will transfer the BSListing to the caller's
    // BSListings collection.
    // It will then remove and destroy the offer.
    // Note that is means that events will be emitted in this order:
    //   1. Collection.CollectionRemovedSaleOffer
    //   2. BSListings.Withdraw
    //   3. BSListings.Deposit
    //   4. SaleOffer.SaleOfferFinished
    //
    pub fun purchase(
      saleNFTID: UInt64,
      buyerCollection: &BSListings.Collection{NonFungibleToken.Receiver},
      buyerPayment: @FungibleToken.Vault
    ) {
      pre {
        self.saleOffers[saleNFTID] != nil: "SaleOffer does not exist in the collection!"
      }
      let offer <- self.remove(saleNFTID: saleNFTID)
      offer.accept(buyerCollection: buyerCollection, buyerPayment: <-buyerPayment)
      destroy offer
    }

    // getSaleOfferIDs
    // Returns an array of the IDs that are in the collection
    //
    pub fun getSaleOfferIDs(): [UInt64] {
      return self.saleOffers.keys
    }

    // borrowSaleItem
    // Returns an Optional read-only view of the SaleItem for the given saleNFTID if it is contained by this collection.
    // The optional will be nil if the provided saleNFTID is not present in the collection.
    //
    pub fun borrowSaleItem(saleNFTID: UInt64): &SaleOffer{SaleOfferPublicView}? {
      if self.saleOffers[saleNFTID] == nil {
        return nil
      } else {
        return &self.saleOffers[saleNFTID] as &SaleOffer{SaleOfferPublicView}
      }
    }

    // destructor
    //
    destroy () {
      destroy self.saleOffers
    }

    // constructor
    //
    init () {
      self.saleOffers <- {}
    }
  }

  // createEmptyCollection
  // Make creating a Collection publicly accessible.
  //
  pub fun createEmptyCollection(): @Collection {
    return <-create Collection()
  }

  init () {
    self.CollectionStoragePath = /storage/bsMarketCollection
    self.CollectionPublicPath = /public/bsMarketCollection
  }
}

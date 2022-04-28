import Basketballs from 0xeee6bdee2b2bdfc8
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

/*
    This is a simple Basketballs initial sale contract for the DApp to use
    in order to list and sell Basketballs.

    Its structure is neither what it would be if it was the simplest possible
    marjet contract or if it was a complete general purpose market contract.
    Rather it's the simplest possible version of a more general purpose
    market contract that indicates how that contract might function in
    broad strokes. This has been done so that integrating with this contract
    is a useful preparatory exercise for code that will integrate with the
    later more general purpose market contract.

    It allows:
    - Anyone to create Sale Offers and place them in a collection, making it
      publicly accessible.
    - Anyone to accept the offer and buy the item.

    It notably does not handle:
    - Multiple different sale NFT contracts.
    - Multiple different payment FT contracts.
    - Splitting sale payments to multiple recipients.

 */

pub contract BasketballsMarket {
    // SaleOffer events.
    //
    // A sale offer has been created.
    pub event SaleOfferCreated(itemID: UInt64, price: UFix64)
    // Someone has purchased an item that was offered for sale.
    pub event SaleOfferAccepted(itemID: UInt64)
    // A sale offer has been destroyed, with or without being accepted.
    pub event SaleOfferFinished(itemID: UInt64)
    
    // Collection events.
    //
    // A sale offer has been inserted into the collection of Address.
    pub event CollectionInsertedSaleOffer(saleItemID: UInt64, saleItemCollection: Address)
    // A sale offer has been removed from the collection of Address.
    pub event CollectionRemovedSaleOffer(saleItemID: UInt64, saleItemCollection: Address)

    // Named paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // SaleOfferPublicView
    // An interface providing a read-only view of a SaleOffer
    //
    pub resource interface SaleOfferPublicView {
        pub var saleCompleted: Bool
        pub let saleItemID: UInt64
        pub let salePrice: UFix64
    }

    pub struct SaleOfferView {
        pub let saleItemID: UInt64
        pub let salePrice: UFix64
        pub let saleCompleted: Bool
        pub let editionID: UInt32
        pub let name: String
        pub let description: String
        pub let imageURL: String
        pub let serialNumber: UInt64
        pub let circulatingCount: UInt64

        init(
            saleItemID: UInt64,
            salePrice: UFix64,
            saleCompleted: Bool,
            editionID: UInt32,
            serialNumber: UInt64,
            name: String,
            description: String,
            imageURL: String,
            circulatingCount: UInt64
        ) {
            self.saleItemID = saleItemID
            self.salePrice = salePrice
            self.saleCompleted = saleCompleted
            self.editionID = editionID
            self.name = name
            self.description = description
            self.imageURL = imageURL
            self.serialNumber = serialNumber
            self.circulatingCount = circulatingCount
        }
    }

    // SaleOffer
    // A Basketballs NFT being offered to sale for a set fee
    //
    pub resource SaleOffer: SaleOfferPublicView {
        // Whether the sale has completed with someone purchasing the item.
        pub var saleCompleted: Bool

        // The Basketballs NFT ID for sale.
        pub let saleItemID: UInt64
        // The collection containing that ID.
        access(self) let sellerItemProvider: Capability<&Basketballs.Collection{NonFungibleToken.Provider}>

        // The sale payment price.
        pub let salePrice: UFix64
        // The vault that will receive that payment if teh sale completes successfully.
        access(self) let sellerPaymentReceiver: Capability<&{FungibleToken.Receiver}>

        // Called by a purchaser to accept the sale offer.
        // If they send the correct payment and if the item is still available,
        // the Basketballs NFT will be placed in their Basketballs.Collection .
        //
        pub fun accept(
            buyerCollection: &Basketballs.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault
        ) {
            pre {
                buyerPayment.balance == self.salePrice: "payment does not equal offer price"
                self.saleCompleted == false: "the sale offer has already been accepted"
            }

            self.saleCompleted = true

            self.sellerPaymentReceiver.borrow()!.deposit(from: <-buyerPayment)

            let nft <- self.sellerItemProvider.borrow()!.withdraw(withdrawID: self.saleItemID)
            buyerCollection.deposit(token: <-nft)

            emit SaleOfferAccepted(itemID: self.saleItemID)
        }

        // destructor
        //
        destroy() {
            // Whether the sale completed or not, publicize that it is being withdrawn.
            emit SaleOfferFinished(itemID: self.saleItemID)
        }

        // initializer
        // Take the information required to create a sale offer, notably the capability
        // to transfer the Basketballs NFT and the capability to receive a fungible token in payment.
        //
        init(
            sellerItemProvider: Capability<&Basketballs.Collection{NonFungibleToken.Provider}>,
            saleItemID: UInt64,
            sellerPaymentReceiver: Capability<&{FungibleToken.Receiver}>,
            salePrice: UFix64
        ) {
            pre {
                sellerItemProvider.borrow() != nil: "Cannot borrow seller"
                sellerPaymentReceiver.borrow() != nil: "Cannot borrow sellerPaymentReceiver"
            }

            self.saleCompleted = false

            self.sellerItemProvider = sellerItemProvider
            self.saleItemID = saleItemID

            self.sellerPaymentReceiver = sellerPaymentReceiver
            self.salePrice = salePrice

            emit SaleOfferCreated(itemID: self.saleItemID, price: self.salePrice)
        }
    }

    // createSaleOffer
    // Make creating a SaleOffer publicly accessible.
    //
    pub fun createSaleOffer (
        sellerItemProvider: Capability<&Basketballs.Collection{NonFungibleToken.Provider}>,
        saleItemID: UInt64,
        sellerPaymentReceiver: Capability<&{FungibleToken.Receiver}>,
        salePrice: UFix64
    ): @SaleOffer {
        return <-create SaleOffer(
            sellerItemProvider: sellerItemProvider,
            saleItemID: saleItemID,
            sellerPaymentReceiver: sellerPaymentReceiver,
            salePrice: salePrice
        )
    }

    // CollectionManager
    // An interface for adding and removing SaleOffers to a collection, intended for
    // use by the collection's owner.
    //
    pub resource interface CollectionManager {
        pub fun insert(offer: @BasketballsMarket.SaleOffer)
        pub fun remove(saleItemID: UInt64): @SaleOffer 
    }

        // CollectionPurchaser
    // An interface to allow purchasing items via SaleOffers in a collection.
    // This function is also provided by CollectionPublic, it is here to support
    // more fine-grained access to the collection for as yet unspecified future use cases.
    //
    pub resource interface CollectionPurchaser {
        pub fun purchase(
            saleItemID: UInt64,
            buyerCollection: &Basketballs.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault
        )
    }

    // CollectionPublic
    // An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers in a collection.
    //
    pub resource interface CollectionPublic {
        pub fun getSaleOfferIDs(): [UInt64]
        pub fun borrowSaleItem(saleItemID: UInt64): &SaleOffer{SaleOfferPublicView}?
        pub fun purchase(
            saleItemID: UInt64,
            buyerCollection: &Basketballs.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault
        )
   }

    // Collection
    // A resource that allows its owner to manage a list of SaleOffers, and purchasers to interact with them.
    //
    pub resource Collection : CollectionManager, CollectionPurchaser, CollectionPublic {
        pub var saleOffers: @{UInt64: SaleOffer}

        // insert
        // Insert a SaleOffer into the collection, replacing one with the same saleItemID if present.
        //
         pub fun insert(offer: @BasketballsMarket.SaleOffer) {
            let id: UInt64 = offer.saleItemID

            // add the new offer to the dictionary which removes the old one
            let oldOffer <- self.saleOffers[id] <- offer
            destroy oldOffer

            emit CollectionInsertedSaleOffer(saleItemID: id, saleItemCollection: self.owner?.address!)
        }

        // remove
        // Remove and return a SaleOffer from the collection.
        pub fun remove(saleItemID: UInt64): @SaleOffer {
            emit CollectionRemovedSaleOffer(saleItemID: saleItemID, saleItemCollection: self.owner?.address!)
            return <-(self.saleOffers.remove(key: saleItemID) ?? panic("missing SaleOffer"))
        }
 
        // purchase
        // If the caller passes a valid saleItemID and the item is still for sale, and passes a vault
        // typed as a FungibleToken.Vault (SomeToken.deposit() handles should cast to concrete type)
        // containing the correct payment amount, this will transfer the Basketball to the caller's
        // Basketballs collection.
        // It will then remove and destroy the offer.
        // Note that is means that events will be emitted in this order:
        //   1. Collection.CollectionRemovedSaleOffer
        //   2. Basketballs.Withdraw
        //   3. Basketballs.Deposit
        //   4. SaleOffer.SaleOfferFinished
        //
        pub fun purchase(
            saleItemID: UInt64,
            buyerCollection: &Basketballs.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault
        ) {
            pre {
                self.saleOffers[saleItemID] != nil: "SaleOffer does not exist in the collection!"
            }
            let offer <- self.remove(saleItemID: saleItemID)
            offer.accept(buyerCollection: buyerCollection, buyerPayment: <-buyerPayment)
            //FIXME: Is this correct? Or should we return it to the caller to dispose of?
            destroy offer
        }

        // getSaleOfferIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getSaleOfferIDs(): [UInt64] {
            return self.saleOffers.keys
        }

        // borrowSaleItem
        // Returns an Optional read-only view of the SaleItem for the given saleItemID if it is contained by this collection.
        // The optional will be nil if the provided saleItemID is not present in the collection.
        //
        pub fun borrowSaleItem(saleItemID: UInt64): &SaleOffer{SaleOfferPublicView}? {
            if self.saleOffers[saleItemID] == nil {
                return nil
            } else {
                return &self.saleOffers[saleItemID] as &SaleOffer{SaleOfferPublicView}
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
        self.CollectionStoragePath = /storage/BasketballsMarketCollection
        self.CollectionPublicPath = /public/BasketballsMarketCollection
    }
}
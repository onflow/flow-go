import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import NonFungibleToken from 0x1d7e57aa55817448
import DimeCollectible from 0xa233bed832a43947

/*
    This contract allows:
    - Anyone to create Sale Offers and place them in a collection, making it
      publicly accessible.
    - Anyone to accept the offer and buy the item.
 */

pub contract DimeMarket {
    // SaleOffer events
    // A sale offer has been created.
    pub event SaleOfferCreated(itemId: UInt64, price: UFix64)
    // Someone has purchased an item that was offered for sale.
    pub event SaleOfferAccepted(itemId: UInt64)
    // A sale offer has been destroyed, with or without being accepted.
    pub event SaleOfferFinished(itemId: UInt64)
    
    // Collection events
    // A sale offer has been removed from the collection of Address.
    pub event CollectionRemovedSaleOffer(itemId: UInt64, owner: Address)

    // A sale offer has been inserted into the collection of Address.
    pub event CollectionInsertedSaleOffer(
      itemId: UInt64,
      creator: Address,
      content: String,
      owner: Address, 
      price: UFix64
    )

    // Named paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // An interface providing a read-only view of a SaleOffer
    pub resource interface SaleOfferPublicView {
        pub let itemId: UInt64
        pub let creator: Address
        pub let content: String
        pub let price: UFix64
    }

    // A DimeCollectible NFT being offered to sale for a set fee
    pub resource SaleOffer: SaleOfferPublicView {
        // Whether the sale has completed with someone purchasing the item.
        pub var saleCompleted: Bool

        // The NFT for sale.
        pub let itemId: UInt64
        pub let creator: Address
        pub let content: String

        // The sale payment price.
        pub let price: UFix64

        // The collection containing that Id.
        access(self) let sellerItemProvider: Capability<&DimeCollectible.Collection{NonFungibleToken.Provider}>

        // Called by a purchaser to accept the sale offer.
        // As of now, there is no transfer of FTs for payment. Instead,
        // we handle the transfer of non-token currency prior to calling accept.
        pub fun accept(
            buyerCollection: &DimeCollectible.Collection{NonFungibleToken.Receiver},
        ) {
            pre {
                self.saleCompleted == false: "the sale offer has already been accepted"
            }

            self.saleCompleted = true

            let nft <- self.sellerItemProvider.borrow()!.withdraw(withdrawID: self.itemId)
            buyerCollection.deposit(token: <-nft)

            emit SaleOfferAccepted(itemId: self.itemId)
        }

        destroy() {
            // Whether the sale completed or not, publicize that it is being withdrawn.
            emit SaleOfferFinished(itemId: self.itemId)
        }

        // Take the information required to create a sale offer: the capability
        // to transfer the DimeCollectible NFT and the capability to receive payment
        init(
            sellerItemProvider: Capability<&DimeCollectible.Collection{NonFungibleToken.Provider}>,
            itemId: UInt64,
            creator: Address,
            content: String,
            price: UFix64
        ) {
            pre {
                sellerItemProvider.borrow() != nil: "Cannot borrow seller"
            }

            self.saleCompleted = false

            let collectionRef = sellerItemProvider.borrow()!

            self.sellerItemProvider = sellerItemProvider
            self.itemId = itemId

            self.price = price
            self.creator = creator
            self.content = content

            emit SaleOfferCreated(itemId: self.itemId, price: self.price)
        }
    }

    // Make creating a SaleOffer publicly accessible
    pub fun createSaleOffer (
        sellerItemProvider: Capability<&DimeCollectible.Collection{NonFungibleToken.Provider}>,
        itemId: UInt64,
        creator: Address,
        content: String,
        price: UFix64
    ): @SaleOffer {
        return <-create SaleOffer(
            sellerItemProvider: sellerItemProvider,
            itemId: itemId,
            creator: creator,
            content: content,
            price: price
        )
    }

    // An interface for adding and removing SaleOffers to a collection, intended for
    // use by the collection's owner
    pub resource interface CollectionManager {
        pub fun insert(offer: @DimeMarket.SaleOffer)
        pub fun remove(itemId: UInt64): @SaleOffer 
    }

    // An interface to allow purchasing items via SaleOffers in a collection.
    // This function is also provided by CollectionPublic, it is here to support
    // more fine-grained access to the collection for as yet unspecified future use cases
    pub resource interface CollectionPurchaser {
        pub fun purchase(
            itemId: UInt64,
            buyerCollection: &DimeCollectible.Collection{NonFungibleToken.Receiver},
        )
    }

    // An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers in a collection
    pub resource interface CollectionPublic {
        pub fun getSaleOfferIds(): [UInt64]
        pub fun borrowSaleItem(itemId: UInt64): &SaleOffer{SaleOfferPublicView}?
        pub fun purchase(
            itemId: UInt64,
            buyerCollection: &DimeCollectible.Collection{NonFungibleToken.Receiver},
        )
   }

    // A resource that allows its owner to manage a list of SaleOffers, and purchasers to interact with them
    pub resource Collection : CollectionManager, CollectionPurchaser, CollectionPublic {
        pub var saleOffers: @{UInt64: SaleOffer}

        // Insert a SaleOffer into the collection, replacing one with the same itemId if present
         pub fun insert(offer: @DimeMarket.SaleOffer) {
            let itemId: UInt64 = offer.itemId
            let creator: Address = offer.creator
            let content: String = offer.content
            let price: UFix64 = offer.price

            // Add the new offer to the dictionary, overwriting an old one if it exists
            let oldOffer <- self.saleOffers[itemId] <- offer
            destroy oldOffer

            emit CollectionInsertedSaleOffer(
              itemId: itemId,
              creator: creator,
              content: content,
              owner: self.owner?.address!,
              price: price
            )
        }

        // Remove and return a SaleOffer from the collection
        pub fun remove(itemId: UInt64): @SaleOffer {
            emit CollectionRemovedSaleOffer(itemId: itemId, owner: self.owner?.address!)
            return <-(self.saleOffers.remove(key: itemId) ?? panic("missing SaleOffer"))
        }
 
        // If the caller passes a valid itemId and the item is still for sale, and passes a
        // vault containing the correct payment amount, this will transfer the
        // NFT to the caller's Collection. It will then remove and destroy the offer.
        // Note that this means that events will be emitted in this order:
        //   1. Collection.CollectionRemovedSaleOffer
        //   2. DimeCollectible.Withdraw
        //   3. DimeCollectible.Deposit
        //   4. SaleOffer.SaleOfferFinished
        pub fun purchase(
            itemId: UInt64,
            buyerCollection: &DimeCollectible.Collection{NonFungibleToken.Receiver},
        ) {
            pre {
                self.saleOffers[itemId] != nil: "SaleOffer does not exist in the collection!"
            }
            let offer <- self.remove(itemId: itemId)
            offer.accept(buyerCollection: buyerCollection)
            //FIXME: Is this correct? Or should we return it to the caller to dispose of?
            destroy offer
        }

        // Returns an array of the Ids that are in the collection
        pub fun getSaleOfferIds(): [UInt64] {
            return self.saleOffers.keys
        }

        // Returns an Optional read-only view of the SaleItem for the given itemId if it is contained by this collection.
        // The optional will be nil if the provided itemId is not present in the collection.
        pub fun borrowSaleItem(itemId: UInt64): &SaleOffer{SaleOfferPublicView}? {
            if self.saleOffers[itemId] == nil {
                return nil
            } else {
                return &self.saleOffers[itemId] as &SaleOffer{SaleOfferPublicView}
            }
        }

        destroy () {
            destroy self.saleOffers
        }

        init () {
            self.saleOffers <- {}
        }
    }

    // Make creating a Collection publicly accessible.
    pub fun createEmptyCollection(): @Collection {
        return <-create Collection()
    }

    init () {
        self.CollectionStoragePath = /storage/DimeMarketCollection
        self.CollectionPublicPath = /public/DimeMarketCollection
    }
}

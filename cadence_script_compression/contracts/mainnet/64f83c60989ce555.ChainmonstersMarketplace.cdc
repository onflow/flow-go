import FUSD from 0x3c5959b568896393
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import ChainmonstersRewards from 0x93615d25d14fa337

/*
    This is a simple ChainmonstersRewards initial sale contract for the DApp to use
    in order to list and sell ChainmonstersRewards.

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

pub contract ChainmonstersMarketplace {
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

    // SaleOffer
    // A ChainmonstersRewards NFT being offered to sale for a set fee paid in FUSD.
    //
    pub resource SaleOffer: SaleOfferPublicView {
        // Whether the sale has completed with someone purchasing the item.
        pub var saleCompleted: Bool

        // The ChainmonstersRewards NFT ID for sale.
        pub let saleItemID: UInt64
        // The collection containing that ID.
        access(self) let sellerItemProvider: Capability<&ChainmonstersRewards.Collection{NonFungibleToken.Provider, ChainmonstersRewards.ChainmonstersRewardCollectionPublic}>

        // The sale payment price.
        pub let salePrice: UFix64
        // The FUSD vault that will receive that payment if teh sale completes successfully.
        access(self) let sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        access(self) let marketFeeReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        // Called by a purchaser to accept the sale offer.
        // If they send the correct payment in FUSD, and if the item is still available,
        // the ChainmonstersRewards NFT will be placed in their ChainmonstersRewards.Collection .
        //
        pub fun accept(
            buyerCollection: &ChainmonstersRewards.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault
        ) {
            pre {
                buyerPayment.balance == self.salePrice: "payment does not equal offer price"
                self.saleCompleted == false: "the sale offer has already been accepted"
            }

            self.saleCompleted = true

            // Take the 5% cut of the tokens that the devAccount gets from the sent tokens
            let marketFee <- buyerPayment.withdraw(amount: self.salePrice*0.05)

            // Deposit it into our devAccount's Vault               
            self.marketFeeReceiver.borrow()!.deposit(from: <-marketFee)


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
        // to transfer the ChainmonstersRewards NFT and the capability to receive FUSD in payment.
        //
        init(
            sellerItemProvider: Capability<&ChainmonstersRewards.Collection{NonFungibleToken.Provider, ChainmonstersRewards.ChainmonstersRewardCollectionPublic}>,
            saleItemID: UInt64,
            sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            salePrice: UFix64,
            marketFeeReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        ) {
            pre {
                sellerItemProvider.borrow() != nil: "Cannot borrow seller"
                sellerPaymentReceiver.borrow() != nil: "Cannot borrow sellerPaymentReceiver"
                marketFeeReceiver.borrow() != nil: "Cannot borrow marketFeeReceiver"
            }

            self.saleCompleted = false

            let collectionRef = sellerItemProvider.borrow()!
            assert(
                collectionRef.borrowReward(id: saleItemID) != nil,
                message: "Specified NFT is not available in the owner's collection"
            )

            self.sellerItemProvider = sellerItemProvider
            self.saleItemID = saleItemID

            self.sellerPaymentReceiver = sellerPaymentReceiver
            self.salePrice = salePrice

            self.marketFeeReceiver = marketFeeReceiver;

            emit SaleOfferCreated(itemID: self.saleItemID, price: self.salePrice)
        }
    }

    // createSaleOffer
    // Make creating a SaleOffer publicly accessible.
    //
    pub fun createSaleOffer (
        sellerItemProvider: Capability<&ChainmonstersRewards.Collection{NonFungibleToken.Provider, ChainmonstersRewards.ChainmonstersRewardCollectionPublic}>,
        saleItemID: UInt64,
        sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
        marketFeeReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
        salePrice: UFix64
    ): @SaleOffer {
        return <-create SaleOffer(
            sellerItemProvider: sellerItemProvider,
            saleItemID: saleItemID,
            sellerPaymentReceiver: sellerPaymentReceiver,
            salePrice: salePrice,
            marketFeeReceiver: marketFeeReceiver
        )
    }


    // CollectionManager
    // An interface for adding and removing SaleOffers to a collection, intended for
    // use by the collection's owner.
    //
    pub resource interface CollectionManager {
        pub fun insert(offer: @ChainmonstersMarketplace.SaleOffer)
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
            buyerCollection: &ChainmonstersRewards.Collection{NonFungibleToken.Receiver},
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
            buyerCollection: &ChainmonstersRewards.Collection{NonFungibleToken.Receiver},
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
            pub fun insert(offer: @ChainmonstersMarketplace.SaleOffer) {
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
        // If the caller passes a valid saleItemID and the item is still for sale, and passes a FUSD vault
        // typed as a FungibleToken.Vault (FUSD.deposit() handles the type safety of this)
        // containing the correct payment amount, this will transfer the KittyItem to the caller's
        // ChainmonstersRewards collection.
        // It will then remove and destroy the offer.
        // Note that is means that events will be emitted in this order:
        //   1. Collection.CollectionRemovedSaleOffer
        //   2. ChainmonstersRewards.Withdraw
        //   3. ChainmonstersRewards.Deposit
        //   4. SaleOffer.SaleOfferFinished
        //
        pub fun purchase(
            saleItemID: UInt64,
            buyerCollection: &ChainmonstersRewards.Collection{NonFungibleToken.Receiver},
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
        self.CollectionStoragePath = /storage/chainmonstersRewardsMarketCollection
        self.CollectionPublicPath = /public/chainmonstersRewardsMarketCollection
    }
}
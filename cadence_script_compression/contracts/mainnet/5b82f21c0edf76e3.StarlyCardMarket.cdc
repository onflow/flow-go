import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FUSD from 0x3c5959b568896393
import StarlyCard from 0x5b82f21c0edf76e3   

pub contract StarlyCardMarket {

    pub struct SaleCutReceiver {
        pub let receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        pub let percent: UFix64

        init(receiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>, percent: UFix64) {
            self.receiver = receiver
            self.percent = percent
        }
    }

    pub fun checkSaleCutReceiver(saleCutReceiver: StarlyCardMarket.SaleCutReceiver): Bool {
        return saleCutReceiver.receiver.borrow() != nil
    }

    pub fun checkSaleCutReceivers(saleCutReceivers: [StarlyCardMarket.SaleCutReceiver]): Bool {
        for saleCutReceiver in saleCutReceivers {
            if (saleCutReceiver.receiver.borrow() == nil) {
                return false
            }
        }
        return true
    }

    pub struct SaleCutReceiverV2 {
        pub let receiver: Capability<&{FungibleToken.Receiver}>
        pub let percent: UFix64

        init(receiver: Capability<&{FungibleToken.Receiver}>, percent: UFix64) {
            self.receiver = receiver
            self.percent = percent
        }
    }

    pub fun checkSaleCutReceiverV2(saleCutReceiver: StarlyCardMarket.SaleCutReceiverV2): Bool {
        return saleCutReceiver.receiver.borrow() != nil
    }

    pub fun checkSaleCutReceiversV2(saleCutReceivers: [StarlyCardMarket.SaleCutReceiverV2]): Bool {
        for saleCutReceiver in saleCutReceivers {
            if (saleCutReceiver.receiver.borrow() == nil) {
                return false
            }
        }
        return true
    }

    pub struct SaleCut {
        pub let address: Address
        pub let amount: UFix64
        pub let percent: UFix64

        init(address: Address, amount: UFix64, percent: UFix64) {
            self.address = address
            self.amount = amount
            self.percent = percent
        }
    }

    // SaleOffer events.
    //
    // A sale offer has been created.
    pub event SaleOfferCreated(
        itemID: UInt64,
        starlyID: String,
        price: UFix64,
        sellerSaleCut: SaleCut,
        beneficiarySaleCut: SaleCut,
        creatorSaleCut: SaleCut,
        additionalSaleCuts: [SaleCut])
    // Someone has purchased an item that was offered for sale.
    pub event SaleOfferAccepted(
        itemID: UInt64,
        starlyID: String,
        price: UFix64,
        buyerAddress: Address,
        sellerSaleCut: SaleCut,
        beneficiarySaleCut: SaleCut,
        creatorSaleCut: SaleCut,
        additionalSaleCuts: [SaleCut])
    // A sale offer has been destroyed, with or without being accepted.
    pub event SaleOfferFinished(itemID: UInt64, sellerAddress: Address)

    // Collection events.
    //
    // A sale offer has been removed from the collection of Address.
    pub event CollectionRemovedSaleOffer(itemID: UInt64, sellerAddress: Address)

    // A sale offer has been inserted into the collection of Address.
    pub event CollectionInsertedSaleOffer(
      itemID: UInt64,
      starlyID: String,
      price: UFix64,
      sellerAddress: Address)

    // Named paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // SaleOfferPublicView
    // An interface providing a read-only view of a SaleOffer
    //
    pub resource interface SaleOfferPublicView {
        pub let itemID: UInt64
        pub let starlyID: String
        pub let price: UFix64
    }

    // SaleOffer
    // A StarlyCard NFT being offered to sale for a set fee paid in FUSD.
    //
    pub resource SaleOffer: SaleOfferPublicView {
        // Whether the sale has completed with someone purchasing the item.
        pub var saleCompleted: Bool

        // The StarlyCard NFT ID for sale.
        pub let itemID: UInt64

        // The 'starlyID' of NFT.
        pub let starlyID: String

        // The sale payment price.
        pub let price: UFix64

        // The collection containing that ID.
        access(self) let sellerItemProvider: Capability<&StarlyCard.Collection{NonFungibleToken.Provider}>

        access(self) let sellerSaleCutReceiver: SaleCutReceiver
        access(self) let beneficiarySaleCutReceiver: SaleCutReceiver
        access(self) let creatorSaleCutReceiver: SaleCutReceiver
        access(self) let additionalSaleCutReceivers: [SaleCutReceiver]

        // Called by a purchaser to accept the sale offer.
        // If they send the correct payment in FUSD, and if the item is still available,
        // the StarlyCard NFT will be placed in their StarlyCard.Collection .
        //
        pub fun accept(
            buyerCollection: &StarlyCard.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        ) {
            pre {
                buyerPayment.balance == self.price: "payment does not equal offer price"
                self.saleCompleted == false: "the sale offer has already been accepted"
            }

            self.saleCompleted = true

            let beneficiaryCutAmount = self.price * self.beneficiarySaleCutReceiver.percent
            let beneficiaryCut <- buyerPayment.withdraw(amount: beneficiaryCutAmount)
            self.beneficiarySaleCutReceiver.receiver.borrow()!.deposit(from: <- beneficiaryCut)

            let creatorCutAmount = self.price * self.creatorSaleCutReceiver.percent
            let creatorCut <- buyerPayment.withdraw(amount: creatorCutAmount)
            self.creatorSaleCutReceiver.receiver.borrow()!.deposit(from: <- creatorCut)

            var additionalSaleCuts: [SaleCut] = []
            for additionalSaleCutReceiver in self.additionalSaleCutReceivers {
                let additionalCutAmount = self.price * additionalSaleCutReceiver.percent
                let additionalCut <- buyerPayment.withdraw(amount: additionalCutAmount)
                additionalSaleCutReceiver.receiver.borrow()!.deposit(from: <- additionalCut)
                additionalSaleCuts.append(StarlyCardMarket.SaleCut(
                    address: additionalSaleCutReceiver.receiver.address,
                    amount: additionalCutAmount,
                    percent: additionalSaleCutReceiver.percent));
            }

            // The rest goes to the seller
            let sellerCutAmount = buyerPayment.balance
            self.sellerSaleCutReceiver.receiver.borrow()!.deposit(from: <- buyerPayment)

            let nft <- self.sellerItemProvider.borrow()!.withdraw(withdrawID: self.itemID)
            buyerCollection.deposit(token: <- nft)

            emit SaleOfferAccepted(
                itemID: self.itemID,
                starlyID: self.starlyID,
                price: self.price,
                buyerAddress: buyerAddress,
                sellerSaleCut: SaleCut(
                    address: self.sellerSaleCutReceiver.receiver.address,
                    amount: sellerCutAmount,
                    percent: self.sellerSaleCutReceiver.percent),
                beneficiarySaleCut: SaleCut(
                    address: self.beneficiarySaleCutReceiver.receiver.address,
                    amount: beneficiaryCutAmount,
                    percent: self.beneficiarySaleCutReceiver.percent),
                creatorSaleCut: SaleCut(
                    address: self.creatorSaleCutReceiver.receiver.address,
                    amount: creatorCutAmount,
                    percent: self.creatorSaleCutReceiver.percent),
                additionalSaleCuts: additionalSaleCuts)
        }

        // destructor
        //
        destroy() {
            // Whether the sale completed or not, publicize that it is being withdrawn.
            // NB: not possible to get owner address anymore, see https://discord.com/channels/613813861610684416/621847426201944074/900329343597821952
            // emit SaleOfferFinished(itemID: self.itemID, sellerAddress: self.owner?.address!)
        }

        // initializer
        // Take the information required to create a sale offer, notably the capability
        // to transfer the StarlyCard NFT and the capability to receive FUSD in payment.
        //
        init(
            itemID: UInt64,
            starlyID: String,
            price: UFix64,
            sellerItemProvider: Capability<&StarlyCard.Collection{NonFungibleToken.Provider}>,
            sellerSaleCutReceiver: SaleCutReceiver,
            beneficiarySaleCutReceiver: SaleCutReceiver,
            creatorSaleCutReceiver: SaleCutReceiver,
            additionalSaleCutReceivers: [SaleCutReceiver],
        ) {
            pre {
                sellerItemProvider.borrow() != nil: "Cannot borrow seller"
                StarlyCardMarket.checkSaleCutReceiver(saleCutReceiver: sellerSaleCutReceiver): "Cannot borrow receiver in sellerSaleCutReceiver"
                StarlyCardMarket.checkSaleCutReceiver(saleCutReceiver: beneficiarySaleCutReceiver): "Cannot borrow receiver in beneficiarySaleCutReceiver"
                StarlyCardMarket.checkSaleCutReceiver(saleCutReceiver: creatorSaleCutReceiver): "Cannot borrow receiver in creatorSaleCutReceiver"
                StarlyCardMarket.checkSaleCutReceivers(saleCutReceivers: additionalSaleCutReceivers): "Cannot borrow receiver in additionalSaleCutReceivers"
            }

            self.saleCompleted = false
            self.itemID = itemID
            self.starlyID = starlyID
            self.price = price
            self.sellerItemProvider = sellerItemProvider
            self.sellerSaleCutReceiver = sellerSaleCutReceiver
            self.beneficiarySaleCutReceiver = beneficiarySaleCutReceiver
            self.creatorSaleCutReceiver = creatorSaleCutReceiver
            self.additionalSaleCutReceivers = additionalSaleCutReceivers

            let sellerCutAmount = price * sellerSaleCutReceiver.percent
            let beneficiaryCutAmount = price * beneficiarySaleCutReceiver.percent
            let creatorCutAmount = price * creatorSaleCutReceiver.percent

            var additionalSaleCuts: [SaleCut] = []
            for additionalSaleCutReceiver in additionalSaleCutReceivers {
                let additionalCutAmount = price * additionalSaleCutReceiver.percent
                additionalSaleCuts.append(StarlyCardMarket.SaleCut(
                    address: additionalSaleCutReceiver.receiver.address,
                    amount: additionalCutAmount,
                    percent: additionalSaleCutReceiver.percent));
            }

            emit SaleOfferCreated(
                itemID: self.itemID,
                starlyID: self.starlyID,
                price: self.price,
                sellerSaleCut: SaleCut(
                    address: self.sellerSaleCutReceiver.receiver.address,
                    amount: sellerCutAmount,
                    percent: self.sellerSaleCutReceiver.percent),
                beneficiarySaleCut: SaleCut(
                    address: self.beneficiarySaleCutReceiver.receiver.address,
                    amount: beneficiaryCutAmount,
                    percent: self.beneficiarySaleCutReceiver.percent),
                creatorSaleCut: SaleCut(
                    address: self.creatorSaleCutReceiver.receiver.address,
                    amount: creatorCutAmount,
                    percent: self.creatorSaleCutReceiver.percent),
                additionalSaleCuts: additionalSaleCuts)
        }
    }

    // createSaleOffer
    // Make creating a SaleOffer publicly accessible.
    //
    pub fun createSaleOffer (
        itemID: UInt64,
        starlyID: String,
        price: UFix64,
        sellerItemProvider: Capability<&StarlyCard.Collection{NonFungibleToken.Provider}>,
        sellerSaleCutReceiver: SaleCutReceiver,
        beneficiarySaleCutReceiver: SaleCutReceiver,
        creatorSaleCutReceiver: SaleCutReceiver,
        additionalSaleCutReceivers: [SaleCutReceiver]
    ): @SaleOffer {
        return <- create SaleOffer(
            itemID: itemID,
            starlyID: starlyID,
            price: price,
            sellerItemProvider: sellerItemProvider,
            sellerSaleCutReceiver: sellerSaleCutReceiver,
            beneficiarySaleCutReceiver: beneficiarySaleCutReceiver,
            creatorSaleCutReceiver: creatorSaleCutReceiver,
            additionalSaleCutReceivers: additionalSaleCutReceivers)
    }

    // CollectionManager
    // An interface for adding and removing SaleOffers to a collection, intended for
    // use by the collection's owner.
    //
    pub resource interface CollectionManager {
        pub fun insert(offer: @StarlyCardMarket.SaleOffer)
        pub fun remove(itemID: UInt64): @SaleOffer
    }

    // CollectionPurchaser
    // An interface to allow purchasing items via SaleOffers in a collection.
    // This function is also provided by CollectionPublic, it is here to support
    // more fine-grained access to the collection for as yet unspecified future use cases.
    //
    pub resource interface CollectionPurchaser {
        pub fun purchase(
            itemID: UInt64,
            buyerCollection: &StarlyCard.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        )
    }

    // CollectionPublic
    // An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers in a collection.
    //
    pub resource interface CollectionPublic {
        pub fun getSaleOfferIDs(): [UInt64]
        pub fun borrowSaleItem(itemID: UInt64): &SaleOffer{SaleOfferPublicView}?
        pub fun purchase(
            itemID: UInt64,
            buyerCollection: &StarlyCard.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        )
   }

    // Collection
    // A resource that allows its owner to manage a list of SaleOffers, and purchasers to interact with them.
    //
    pub resource Collection : CollectionManager, CollectionPurchaser, CollectionPublic {
        pub var saleOffers: @{UInt64: SaleOffer}

        // insert
        // Insert a SaleOffer into the collection, replacing one with the same itemID if present.
        //
         pub fun insert(offer: @StarlyCardMarket.SaleOffer) {
            let itemID: UInt64 = offer.itemID
            let starlyID: String = offer.starlyID
            let price: UFix64 = offer.price

            // add the new offer to the dictionary which removes the old one
            let oldOffer <- self.saleOffers[itemID] <- offer
            destroy oldOffer

            emit CollectionInsertedSaleOffer(
              itemID: itemID,
              starlyID: starlyID,
              price: price,
              sellerAddress: self.owner?.address!
            )
        }

        // remove
        // Remove and return a SaleOffer from the collection.
        pub fun remove(itemID: UInt64): @SaleOffer {
            emit CollectionRemovedSaleOffer(itemID: itemID, sellerAddress: self.owner?.address!)
            return <- (self.saleOffers.remove(key: itemID) ?? panic("missing SaleOffer"))
        }

        // purchase
        // If the caller passes a valid itemID and the item is still for sale, and passes a FUSD vault
        // typed as a FungibleToken.Vault (FUSD.deposit() handles the type safety of this)
        // containing the correct payment amount, this will transfer the StarlyCard to the caller's
        // StarlyCard collection.
        // It will then remove and destroy the offer.
        // Note that is means that events will be emitted in this order:
        //   1. Collection.CollectionRemovedSaleOffer
        //   2. StarlyCard.Withdraw
        //   3. StarlyCard.Deposit
        //   4. SaleOffer.SaleOfferFinished
        //
        pub fun purchase(
            itemID: UInt64,
            buyerCollection: &StarlyCard.Collection{NonFungibleToken.Receiver},
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        ) {
            pre {
                self.saleOffers[itemID] != nil: "SaleOffer does not exist in the collection!"
            }
            let offer <- self.remove(itemID: itemID)
            offer.accept(buyerCollection: buyerCollection, buyerPayment: <- buyerPayment, buyerAddress: buyerAddress)
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
        // Returns an Optional read-only view of the SaleItem for the given itemID if it is contained by this collection.
        // The optional will be nil if the provided itemID is not present in the collection.
        //
        pub fun borrowSaleItem(itemID: UInt64): &SaleOffer{SaleOfferPublicView}? {
            if self.saleOffers[itemID] == nil {
                return nil
            } else {
                return &self.saleOffers[itemID] as &SaleOffer{SaleOfferPublicView}
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
        return <- create Collection()
    }

    init () {
        self.CollectionStoragePath = /storage/starlyCardMarketCollection
        self.CollectionPublicPath = /public/starlyCardMarketCollection
    }
}

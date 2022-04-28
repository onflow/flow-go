import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

import AACommon from 0x39eeb4ee6f30fc3f
import AACollectionManager from 0x39eeb4ee6f30fc3f
import AACurrencyManager from 0x39eeb4ee6f30fc3f
import AAFeeManager from 0x39eeb4ee6f30fc3f
import AAReferralManager from 0x39eeb4ee6f30fc3f

pub contract AAOpenBid {
    pub event AAOpenBidInitialized()

    pub event OpenBidInitialized(openBidResourceID: UInt64)

    pub event OpenBidDestroyed(openBidResourceID: UInt64)

    pub event BidAvailable(
        openBidAddress: Address,
        bidResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        ftVaultType: Type,
        price: UFix64
    )

    pub event BidCompleted(
        bidResourceID: UInt64, 
        openBidResourceID: UInt64, 
        purchased: Bool,
        nftType: Type,
        nftID: UInt64,
        payments: [AACommon.Payment]?
    )

    // OpenBidStoragePath
    // The location in storage that a OpenBid resource should be located.
    pub let OpenBidStoragePath: StoragePath

    // OpenBidPublicPath
    // The public location for a OpenBid link.
    pub let OpenBidPublicPath: PublicPath


    // SaleCut
    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    //
    pub struct SaleCut {
        // The receiver for the payment.
        // Note that we do not store an address to find the Vault that this represents,
        // as the link or resource that we fetch in this way may be manipulated,
        // so to find the address that a cut goes to you must get this struct and then
        // call receiver.borrow()!.owner.address on it.
        // This can be done efficiently in a script.
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        // The amount of the payment FungibleToken that will be paid to the receiver.
        pub let amount: UFix64

        // initializer
        //
        init(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
            self.receiver = receiver
            self.amount = amount
        }
    }


    // BidDetails
    // A struct containing a Bid's data.
    //
    pub struct BidDetails {
        // The OpenBid that the Bid is stored in.
        // Note that this resource cannot be moved to a different OpenBid,
        // so this is OK. If we ever make it so that it *can* be moved,
        // this should be revisited.
        pub var openBidID: UInt64
        // Whether this listing has been purchased or not.
        pub var purchased: Bool
        // The Type of the NonFungibleToken.NFT that is being listed.
        pub let nftType: Type
        // The ID of the NFT within that type.
        pub let nftID: UInt64
        // The Type of the FungibleToken that payments must be made in.
        pub let salePaymentVaultType: Type
        // The amount that must be paid in the specified FungibleToken.
        pub let bidPrice: UFix64
        // The address of referral user
        pub let affiliate: Address?
        // setToPurchased
        // Irreversibly set this listing as purchased.
        //
        access(contract) fun setToPurchased() {
            self.purchased = true
        }

        // initializer
        //
        init (
            openBidID: UInt64,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            bidPrice: UFix64,
            affiliate: Address?
        ) {
            self.openBidID = openBidID
            self.purchased = false
            self.nftType = nftType
            self.nftID = nftID
            self.salePaymentVaultType = salePaymentVaultType
            self.affiliate = affiliate

            assert(bidPrice > 0.0, message: "Bid must have non-zero price")
            self.bidPrice = bidPrice
        }
    }


    // BidPublic
    // An interface providing a useful public interface to a Bid.
    //
    pub resource interface BidPublic {
        pub fun getDetails(): BidDetails
        pub fun purchase(nftProviderCapability: Capability<&{NonFungibleToken.Provider}>)
    }

    pub resource Bid: BidPublic {
        // The simple (non-Capability, non-complex) details of the sale
        access(self) let details: BidDetails
        access(self) let vaultProviderCapability: Capability<&{FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance}>
        access(self) let recipientCapability: Capability<&{NonFungibleToken.CollectionPublic}>

        // getDetails
        // Get the details of the current state of the Bid as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources). 
        //
        pub fun getDetails(): BidDetails {
            return self.details
        }
        
        pub fun purchase(nftProviderCapability: Capability<&{NonFungibleToken.Provider}>) {
            pre {
                self.details.purchased == false: "listing has already been purchased"
            }

            let nft <- nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)

            assert(nft.isInstance(self.details.nftType), message: "NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "NFT does not have specified ID")

            let seller = nftProviderCapability.address

            // Make sure the listing cannot be purchased again.
            self.details.setToPurchased()

            // Send item to bidder
            self.recipientCapability.borrow()!.deposit(token: <- nft)

            // Pay each beneficiary their amount of the payment.
            let path = AACurrencyManager.getPath(type: self.details.salePaymentVaultType)
            assert(path != nil, message: "Currency Path not setting")

            let cap = fun (_ addr: Address): Capability<&{FungibleToken.Receiver}> {
                return getAccount(addr).getCapability<&{FungibleToken.Receiver}>(path!.publicPath)
            }
            
            let payment <- self.vaultProviderCapability.borrow()!.withdraw(amount: self.details.bidPrice)
            let cuts = AAOpenBid.getSaleCuts(
                seller: seller, nftType: self.details.nftType,
                nftID: self.details.nftID, affiliate: self.details.affiliate
            )

            let payments: [AACommon.Payment] = []
            var rate = 1.0
            for cut in cuts {
                if let receiver = cap(cut.recipient).borrow() {
                    rate = rate - cut.rate
                    let amount = cut.rate * self.details.bidPrice
                    let paymentCut <- payment.withdraw(amount: amount)
                    receiver.deposit(from: <-paymentCut)

                    payments.append(
                      AACommon.Payment(
                        type: cut.type,
                        recipient: cut.recipient,
                        rate: cut.rate,
                        amount: amount
                      )
                    )
                }
            }

            payments.append(
              AACommon.Payment(
                type: "Seller Earn",
                recipient: seller,
                rate: rate,
                amount: payment.balance
              )
            )

            let sellerRecipient = cap(seller).borrow() ?? panic("Seller vault broken") 
            sellerRecipient.deposit(from: <- payment)


            AAFeeManager.markAsPurchased(type: self.details.nftType, nftID: self.details.nftID)

            // If the bid is purchased, we regard it as completed here.
            // Otherwise we regard it as completed in the destructor.        

            emit BidCompleted(
                bidResourceID: self.uuid,
                openBidResourceID: self.details.openBidID,
                purchased: self.details.purchased,
                nftType: self.details.nftType,
                nftID: self.details.nftID,
                payments: payments
            )
        }

        destroy () {
            // If the listing has not been purchased, we regard it as completed here.
            // Otherwise we regard it as completed in purchase().
            // This is because we destroy the listing in OpenBid.removeBid()
            // or OpenBid.cleanup() .
            // If we change this destructor, revisit those functions.
            if !self.details.purchased {
                emit BidCompleted(
                    bidResourceID: self.uuid,
                    openBidResourceID: self.details.openBidID,
                    purchased: self.details.purchased,
                    nftType: self.details.nftType,
                    nftID: self.details.nftID,
                    payments: nil
                )
            }
        }

        // initializer
        //
        init (
            vaultProviderCapability: Capability<&{FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance}>,
            nftType: Type,
            nftID: UInt64,
            bidPrice: UFix64,
            recipientCapability: Capability<&{NonFungibleToken.CollectionPublic}>,
            affiliate: Address?,
        ) {

            let vaultRef = vaultProviderCapability.borrow() ?? panic("vault ref broken")

            assert(AACurrencyManager.isCurrencyAccepted(type: vaultRef.getType()), message: "Currency not accepted")

            // Store the sale information
            self.details = BidDetails(
                openBidID: self.uuid,
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: vaultRef.getType(),
                bidPrice: bidPrice,
                affiliate: affiliate
            )

            self.vaultProviderCapability = vaultProviderCapability
            self.recipientCapability = recipientCapability

            let collection = self.recipientCapability.borrow()
            assert(collection != nil, message: "cannot borrow collection recipient")
        }
    }

    pub fun getSaleCuts(seller: Address, nftType: Type, nftID: UInt64, affiliate: Address?): [AACommon.PaymentCut] {
        let referrer = AAReferralManager.referrerOf(owner: seller)
        let cuts = AAFeeManager.getPlatformCuts(referralReceiver: referrer, affiliate: affiliate) ?? []
        let itemCuts = AAFeeManager.getPaymentCuts(type: nftType, nftID: nftID)
        cuts.appendAll(itemCuts)

        if let collectionCuts = AACollectionManager.getCollectionCuts(type: nftType, nftID: nftID) {
            cuts.appendAll(collectionCuts)
        }

        return cuts
    }

    // OpenBidManager
    // An interface for adding and removing Bids within a OpenBid,
    // intended for use by the OpenBid's own
    //
    pub resource interface OpenBidManager {
        // createBid
        // Allows the OpenBid owner to create and insert Bid.
        //
        pub fun createBid(
            vaultProviderCapability: Capability<&{FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance}>,
            nftType: Type,
            nftID: UInt64,
            bidPrice: UFix64,
            recipientCapability: Capability<&{NonFungibleToken.CollectionPublic}>,
            affiliate: Address?,
        ): UInt64

        // Allows the OpenBid owner to remove any bid, purchased or not.
        pub fun removeBid(bidResourceID: UInt64)
    }

    pub resource interface OpenBidPublic {
        pub fun getBidIDs(): [UInt64]
        pub fun borrowBid(bidResourceID: UInt64): &Bid{BidPublic}?
        pub fun cleanup(bidResourceID: UInt64)
   }

    // OpenBid
    // A resource that allows its owner to manage a list of Bids, and anyone to interact with them
    // in order to query their details and purchase the NFTs that they represent.
    //
    pub resource OpenBid : OpenBidManager, OpenBidPublic {
        // The dictionary of Bid uuids to Bid resources.
        access(self) var bids: @{UInt64: Bid}

        pub fun createBid(
            vaultProviderCapability: Capability<&{FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance}>,
            nftType: Type,
            nftID: UInt64,
            bidPrice: UFix64,
            recipientCapability: Capability<&{NonFungibleToken.CollectionPublic}>,
            affiliate: Address?,
         ): UInt64 {
            let bid <- create Bid(
                vaultProviderCapability: vaultProviderCapability,
                nftType: nftType,
                nftID: nftID,
                bidPrice: bidPrice,
                recipientCapability: recipientCapability,
                affiliate: affiliate,
            )

            let bidResourceID = bid.uuid
            let vaultType = bid.getDetails().salePaymentVaultType

            // Add the new listing to the dictionary.
            let oldBid <- self.bids[bidResourceID] <- bid
            // Note that oldBid will always be nil, but we have to handle it.

            destroy oldBid

            emit BidAvailable(
                openBidAddress: self.owner?.address!,
                bidResourceID: bidResourceID,
                nftType: nftType,
                nftID: nftID,
                ftVaultType: vaultType,
                price: bidPrice
            )

            return bidResourceID
        }
        
        pub fun removeBid(bidResourceID: UInt64) {
            let bid <- self.bids.remove(key: bidResourceID)
                ?? panic("missing Bid")
    
            // This will emit a BidCompleted event.
            destroy bid
        }
     
        pub fun getBidIDs(): [UInt64] {
            return self.bids.keys
        }

        pub fun borrowBid(bidResourceID: UInt64): &Bid{BidPublic}? {
            if self.bids[bidResourceID] != nil {
                return &self.bids[bidResourceID] as! &Bid{BidPublic}
            } else {
                return nil
            }
        }

        pub fun cleanup(bidResourceID: UInt64) {
            pre {
                self.bids[bidResourceID] != nil: "could not find listing with given id"
            }

            let bid <- self.bids.remove(key: bidResourceID)!
            assert(bid.getDetails().purchased == true, message: "listing is not purchased, only admin can remove")
            destroy bid
        }
      
        destroy () {
            destroy self.bids

            // Let event consumers know that this openBid will no longer exist
            emit OpenBidDestroyed(openBidResourceID: self.uuid)
        }

        init () {
            self.bids <- {}

            // Let event consumers know that this openBid exists
            emit OpenBidInitialized(openBidResourceID: self.uuid)
        }
    }

    // createOpenBid
    // Make creating a OpenBid publicly accessible.
    //
    pub fun createOpenBid(): @OpenBid {
        return <-create OpenBid()
    }

    init () {
        self.OpenBidStoragePath = /storage/AAOpenBid
        self.OpenBidPublicPath = /public/AAOpenBid

        emit AAOpenBidInitialized()
    }
}
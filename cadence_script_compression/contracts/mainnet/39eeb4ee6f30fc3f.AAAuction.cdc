import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

import AACommon from 0x39eeb4ee6f30fc3f
import AACollectionManager from 0x39eeb4ee6f30fc3f
import AACurrencyManager from 0x39eeb4ee6f30fc3f
import AAFeeManager from 0x39eeb4ee6f30fc3f
import AAReferralManager from 0x39eeb4ee6f30fc3f

pub contract AAAuction {
    access(contract) let unclaimedBids: @{UInt64: [Bid]}

    pub let AdminStoragePath: StoragePath
    pub let AAAuctionStoragePath: StoragePath
    pub let AAAuctionPublicPath: PublicPath

    pub event AuctionCenterInitialized(id: UInt64)
    pub event AuctionCenterDestroyed(id: UInt64)
    pub event AuctionAvailable(
        auctionID: UInt64,
        seller: Address,
        nftType: Type,
        nftID: UInt64,
        bidType: Type,
        startPrice: UFix64,
        startAt: UFix64,
        endAt: UFix64?,
    )
    pub event AuctionCompleted(storeResourceID: UInt64, auctionID: UInt64, seller: Address, bidder: Address?, lastPrice: UFix64?, purchased: Bool)
    pub event NewBid(storeResourceID: UInt64, auctionID: UInt64, seller: Address, nftType: Type, nftID: UInt64, bidder: Address, bidPrice: UFix64)
    pub event AuctionPayment(type: Type, payments: [AACommon.Payment])

    pub struct AuctionDetails {
        pub let nftType: Type
        pub let nftID: UInt64
        pub let startAt: UFix64
        pub let endAt: UFix64?
        pub let startPrice: UFix64
        pub let increment: UFix64
        pub let bidType: Type
        pub let storeResourceID: UInt64
        
        init(
          nftType: Type,
          nftID: UInt64,
          startAt: UFix64,
          endAt: UFix64?,
          startPrice: UFix64,
          increment: UFix64,
          bidType: Type,
          storeResourceID: UInt64
        ) {
            pre {
              endAt != nil && endAt! > startAt: "Endtime should be greater than start time"
              endAt != nil && endAt! > getCurrentBlock().timestamp: "End time should be greater than current time"
            }

            self.nftType = nftType
            self.nftID = nftID
            self.startAt = startAt
            self.endAt = endAt
            self.startPrice = startPrice
            self.increment = increment
            self.bidType = bidType

            self.storeResourceID = storeResourceID
        }
    }

    pub resource Bid {
        access(contract) let recipient: Capability<&{NonFungibleToken.CollectionPublic}>
        access(self) let vault: @FungibleToken.Vault
        access(contract) var price: UFix64
        access(contract) let refund: Capability<&{FungibleToken.Receiver}>
        access(contract) var affiliate: Address?

        init(
          recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
          vault: @FungibleToken.Vault,
          price: UFix64,
          refund: Capability<&{FungibleToken.Receiver}>,
          affiliate: Address?
        ) {
            pre {
              recipient.check(): "NFT recipient invalid"
              refund.check(): "Refund vault invalid"
              vault.getType() == refund.borrow()!.getType(): "Should you same type of fund to return"
            }

            self.recipient = recipient
            self.vault <- vault
            self.price = price
            self.refund = refund

            self.affiliate = affiliate
        }

        pub fun doRefund(): Bool {
            if let recipient = self.refund.borrow() {
                if self.vault.getType() == recipient.getType() {
                    recipient.deposit(from: <- self.vault.withdraw(amount: self.vault.balance))
                    return true
                } 
            }

            return false
        }

        pub fun doIncrease(
          affiliate: Address?,
          from: @FungibleToken.Vault
        ) {
            self.vault.deposit(from: <- from)
            self.price = self.vault.balance
            self.affiliate = affiliate
        }

        pub fun payout(cuts: [AACommon.PaymentCut], for seller: Address) {
            let path = AACurrencyManager.getPath(type: self.vault.getType())
            assert(path != nil, message: "Currency Path not setting")

            let cap = fun (_ addr: Address): Capability<&{FungibleToken.Receiver}> {
                return getAccount(addr).getCapability<&{FungibleToken.Receiver}>(path!.publicPath)
            }

            let payments: [AACommon.Payment] = []
            var rate = 1.0
            for cut in cuts {
                if let receiver = cap(cut.recipient).borrow() {
                    rate = rate - cut.rate
                    let amount = self.price * cut.rate
                    receiver.deposit(from: <-self.vault.withdraw(amount: amount))

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
                amount: self.vault.balance
              )
            )
            let sellerRecipient = cap(seller).borrow() ?? panic("Seller vault broken") 
            sellerRecipient.deposit(from: <- self.vault.withdraw(amount: self.vault.balance))

            emit AuctionPayment(type: self.vault.getType(), payments: payments)
        }

        pub fun bidder(): Address {
          return self.refund.address
        }

        pub fun bidAmount(): UFix64 {
          return self.vault.balance
        }

        destroy() {
            pre {
              self.vault.balance == 0.0: "Can't destroy a bid with non-empty vault"
            }

            destroy self.vault
        }
    }

    pub resource interface AuctionPublic {
        pub fun getDetails(): AuctionDetails
        pub fun isOpen(): Bool
        pub fun placeBid(
          recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
          refund: Capability<&{FungibleToken.Receiver}>,
          vault: @FungibleToken.Vault,
          affiliate: Address?
        )
        pub fun currentBidForUser(address: Address): UFix64
    }
    
    pub resource Auction: AuctionPublic {
        access(self) let details: AuctionDetails 
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
        access(contract) var bid: @Bid?

        pub fun borrowNFT(): &NonFungibleToken.NFT {
            let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: self.getDetails().nftID)
            assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
            assert(ref.id == self.getDetails().nftID, message: "token has wrong ID")
            return ref as &NonFungibleToken.NFT
        }

        pub fun getDetails(): AuctionDetails {
            return self.details
        }

        pub fun currentBidForUser(address: Address): UFix64 {
            if self.bid?.bidder() == address {
                return self.bid?.bidAmount()!
            }
            return 0.0
        }

        pub fun isOpen(): Bool {
            let now = getCurrentBlock().timestamp

            // startAt <= now <= endAt
            if self.details.startAt > now {
                return false
            }

            if self.details.endAt != nil && self.details.endAt! < now {
                return false
            }

            return true
        }

        pub fun placeBid(
          recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
          refund: Capability<&{FungibleToken.Receiver}>,
          vault: @FungibleToken.Vault,
          affiliate: Address?
        ) {
            pre {
              refund.check(): "Refund broken"
              recipient.check(): "Refund broken"
              refund.borrow()!.getType() == vault.getType(): "Vault and refund should same type"
            }

            if self.bid?.bidder() == refund.address {
                assert(self.bid?.recipient?.address == recipient.address, message: "New bid should use same recipient from previous")

                return self.increaseBid(
                  vault: <- vault,
                  affiliate: affiliate
                )
            }

            return self.createBid(recipient: recipient, refund: refund, vault: <- vault, affiliate: affiliate)
        }

        access(self) fun increaseBid(
          vault: @FungibleToken.Vault,
          affiliate: Address?
        ) {
            pre {
              self.bid != nil: "Invalid call"
              vault.balance >= self.details.increment
            }

            if let bid <- self.bid <- nil {
                bid.doIncrease(affiliate: affiliate, from: <- vault)
                let old <- self.bid <- bid
                destroy old
            } else {
                destroy vault
                panic("Never call this")
            }

            emit NewBid(
              storeResourceID: self.details.storeResourceID,
              auctionID: self.uuid,
              seller: self.nftProviderCapability.address,
              nftType: self.details.nftType,
              nftID: self.details.nftID,
              bidder: self.bid?.bidder()!,
              bidPrice: self.bid?.price!
            )
        }

        access(self) fun createBid(
          recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
          refund: Capability<&{FungibleToken.Receiver}>,
          vault: @FungibleToken.Vault,
          affiliate: Address?
        ) {
            pre {
                self.isOpen(): "Auction not open"
                vault.isInstance(self.details.bidType): "payment vault is not requested fungible token"
                refund.address == recipient.address: "you cannot make a bid and send item to sombody elses collection"
            }

            let price = vault.balance
            assert(price >= self.details.startPrice, message: "Bid price must be greater than start price")
            assert(price >= (self.bid?.price ?? 0.0) + self.details.increment, message: "bid amount must be larger or equal to the current price + minimum bid increment")

            let lastBid <- self.bid <- create Bid(
              recipient: recipient,
              vault: <- vault,
              price: price,
              refund: refund,
              affiliate: affiliate
            )

            let bidder = refund.address
            emit NewBid(
              storeResourceID: self.details.storeResourceID,
              auctionID: self.uuid,
              seller: self.nftProviderCapability.address,
              nftType: self.details.nftType,
              nftID: self.details.nftID,
              bidder: bidder,
              bidPrice: self.bid?.price!
            )

            if lastBid == nil {
              destroy lastBid
              return 
            } 

            let bid <- lastBid!

            if bid.doRefund() {
              destroy bid
              return 
            }

            self.addUnclaimedBid(bid: <- bid)
        }

        access(contract) fun complete() {
          pre {
            self.bid != nil: "No bid. Instead, use cancel"
            getCurrentBlock().timestamp > self.details.endAt ?? 0.0: "Auction has not ended"
          }

          self.sendNFT(to: self.bid?.recipient!)

          let seller = self.nftProviderCapability.address
          let cuts = AAAuction.getSaleCuts(
              seller: seller, nftType: self.details.nftType,
              nftID: self.details.nftID, affiliate: self.bid?.affiliate
          )

          self.bid?.payout(cuts: cuts, for: seller)

          AAFeeManager.markAsPurchased(type: self.details.nftType, nftID: self.details.nftID)

          emit AuctionCompleted(
              storeResourceID: self.details.storeResourceID,
              auctionID: self.uuid,
              seller: seller,
              bidder: self.bid?.bidder(),
              lastPrice: self.bid?.price,
              purchased: true
          )
        }

        access(self) fun addUnclaimedBid(bid: @Bid) {
            var bids <- AAAuction.unclaimedBids.remove(key: self.details.storeResourceID)

            if bids == nil {
              bids <-! [] as @[Bid]
            }

            let dummy <- bids!
            dummy.append(<-bid)

            let old <- AAAuction.unclaimedBids[self.details.storeResourceID] <- dummy
            destroy old
        }

        access(self) fun sendNFT(to receiver: Capability<&{NonFungibleToken.CollectionPublic}>) {
            let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)

            assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")

            receiver.borrow()!.deposit(token: <- nft)
        }

        init(
          nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
          details: AuctionDetails
        ) {
            self.details = details
            self.nftProviderCapability = nftProviderCapability
            self.bid <- nil
        }

        destroy() {
          destroy self.bid
        }
    }

    pub resource interface AuctionStorePublic {
        pub fun getAuctionIDs(): [UInt64]
        pub fun borrow(auctionID: UInt64): &Auction{AuctionPublic}?
    }

    pub resource AuctionStore: AuctionStorePublic {
        access(self) let auctions: @{UInt64: Auction}

        pub fun createAuction(
          nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
          nftType: Type,
          nftID: UInt64,
          startAt: UFix64,
          endAt: UFix64?,
          startPrice: UFix64,
          increment: UFix64,
          bidType: Type
        ): UInt64 {
          pre {
            AACurrencyManager.isCurrencyAccepted(type: bidType): "Currency not allow"
            nftProviderCapability.address == self.owner?.address!: "Store and NFT provider should same"
          }

          let details = AuctionDetails(
            nftType: nftType,
            nftID: nftID,
            startAt: startAt,
            endAt: endAt,
            startPrice: startPrice,
            increment: increment,
            bidType: bidType,
            storeResourceID: self.uuid
          )

          let auction <- create Auction(
            nftProviderCapability: nftProviderCapability,
            details: details
          )

          // Borrow and verify
          auction.borrowNFT()

          let auctionID = auction.uuid
          let oldAuction <- self.auctions[auctionID] <- auction

          // Note that oldAuction will always be nil, but we have to handle it.
          destroy oldAuction

          emit AuctionAvailable(
              auctionID: auctionID,
              seller: nftProviderCapability.address,
              nftType: nftType,
              nftID: nftID,
              bidType: bidType,
              startPrice: startPrice,
              startAt: startAt,
              endAt: endAt
          )

          return auctionID
        }

        pub fun cancelAuction(auctionID: UInt64) {
            let auction <- self.auctions.remove(key: auctionID)
                      ?? panic("missing auction")

            assert(auction.bid == nil, message: "Can't cancel auction with bids")

            emit AuctionCompleted(
              storeResourceID: self.uuid,
              auctionID: auctionID,
              seller: auction.nftProviderCapability.address,
              bidder: nil,
              lastPrice: nil,
              purchased: false
            )

            destroy auction
        }

        pub fun completeAuction(auctionID: UInt64) {
            let auction <- self.auctions.remove(key: auctionID)
                      ?? panic("missing auction")

            auction.complete()
            destroy auction
        }

        pub fun getAuctionIDs(): [UInt64] {
            return self.auctions.keys
        }

        pub fun borrow(auctionID: UInt64): &Auction{AuctionPublic}? {
            if self.auctions[auctionID] != nil {
              return &self.auctions[auctionID] as! &Auction{AuctionPublic}
            } else {
              return nil
            }
        }

        init() {
          self.auctions <- {}

          emit AuctionCenterInitialized(id: self.uuid)
        }

        destroy() {
          pre {
            self.auctions.length == 0: "Can't not destroy non-empty auction center"
          }

          destroy self.auctions

          emit AuctionCenterDestroyed(id: self.uuid)
        }
    }

    pub resource Admin {
        pub fun refundUnclaimedBidForUser(storeUUID: UInt64) {
            if let bids <- AAAuction.unclaimedBids.remove(key: storeUUID) {
                var i = 0

                while i < bids.length {
                    let ref = &bids[i] as &AAAuction.Bid
                    assert(ref.doRefund(), message: "Can't not refund")

                    i = i + 1
                }


                destroy bids
            }
        }
    }

    pub fun createAuctionStore(): @AuctionStore {
        return <-create AuctionStore()
    }

    access(contract) fun getSaleCuts(seller: Address, nftType: Type, nftID: UInt64, affiliate: Address?): [AACommon.PaymentCut] {
        let referrer = AAReferralManager.referrerOf(owner: seller)
        let cuts = AAFeeManager.getPlatformCuts(referralReceiver: referrer, affiliate: affiliate) ?? []
        let itemCuts = AAFeeManager.getPaymentCuts(type: nftType, nftID: nftID)
        cuts.appendAll(itemCuts)

        if let collectionCuts = AACollectionManager.getCollectionCuts(type: nftType, nftID: nftID) {
          cuts.appendAll(collectionCuts)
        }

        return cuts
    }

    access(self) fun getUnclaimBidIDs(): [UInt64] {
        return self.unclaimedBids.keys
    }


    init() {
      self.AAAuctionPublicPath = /public/AAAuction
      self.AAAuctionStoragePath = /storage/AAAuction
      self.unclaimedBids <- {}

      self.AdminStoragePath = /storage/AAAuctionAdmin
      self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }

}
 
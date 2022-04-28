import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61

import Tickets from 0x7c11edb826692404

pub contract TicketsAuction {
    access(contract) let unclaimedBids: @{Address: [Bid]}
    access(contract) let auctions: @{UInt64: Auction}

    pub var startAt: UFix64
    pub var endAt: UFix64?
    pub var startPrice: UFix64
    pub let bidType: Type
    pub var increment: UFix64

    pub var total: UInt64

    pub let AdminStoragePath: StoragePath
    pub let FlowReceiverPath: PublicPath

    pub event ConfigChange(type: String, value: [UFix64])
    pub event AuctionAvailable(auctionID: UInt64)
    pub event AuctionCompleted(auctionID: UInt64, bidder: Address?, lastPrice: UFix64?, purchased: Bool)
    pub event NewBid(uuid: UInt64, auctionID: UInt64, bidder: Address, bidPrice: UFix64)

    pub resource Bid {
        access(self) let vault: @FungibleToken.Vault
        access(contract) var price: UFix64
        access(contract) let refund: Capability<&{FungibleToken.Receiver}>
        access(contract) let recipient: Capability<&{NonFungibleToken.CollectionPublic}>
        access(contract) var ref: String?

        init(
            vault: @FungibleToken.Vault,
            price: UFix64,
            refund: Capability<&{FungibleToken.Receiver}>,
            recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
            ref: String?
        ) {
            pre {
              refund.check(): "Refund vault invalid"
              recipient.check(): "Recipient collection invalid"
              vault.getType() == refund.borrow()!.getType(): "Should you same type of fund to return"
            }

            self.vault <- vault
            self.price = price
            self.refund = refund
            self.recipient = recipient
            self.ref = ref
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

        pub fun doIncrease(from: @FungibleToken.Vault, ref: String?) {
            self.vault.deposit(from: <- from)
            self.price = self.vault.balance
            self.ref = ref
        }

        pub fun payout() {
            Tickets.payAndRewardDiamond(recipient: self.recipient, payment: <- self.vault.withdraw(amount: self.vault.balance), ref: self.ref)
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
        pub fun placeBid(
            refund: Capability<&{FungibleToken.Receiver}>,
            recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
            vault: @FungibleToken.Vault,
            ref: String?
        )
        pub fun currentBidForUser(address: Address): UFix64
    }

    pub resource Auction: AuctionPublic {
        pub let id: UInt64
        access(contract) var bid: @Bid?

        pub fun currentBidForUser(address: Address): UFix64 {
            if self.bid?.bidder() == address {
                return self.bid?.bidAmount()!
            }
            return 0.0
        }

        pub fun placeBid(
            refund: Capability<&{FungibleToken.Receiver}>,
            recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
            vault: @FungibleToken.Vault,
            ref: String?
        ) {
            pre {
                refund.address == recipient.address: "Should use same"
            }

            if self.bid?.bidder() == refund.address {
                return self.increaseBid(vault: <- vault, ref: ref)
            }

            return self.createBid(refund: refund, recipient: recipient, vault: <- vault, ref: ref)
        }

        access(contract) fun complete() {
          pre {
            getCurrentBlock().timestamp > TicketsAuction.endAt ?? 0.0: "Auction has not ended"
          }

           if self.bid == nil {
                 emit AuctionCompleted(
                    auctionID: self.id,
                    bidder: nil,
                    lastPrice: nil,
                    purchased: false
                )

                return 
            }



          self.bid?.payout()
          emit AuctionCompleted(
              auctionID: self.id,
              bidder: self.bid?.bidder(),
              lastPrice: self.bid?.price,
              purchased: true
          )
        }

        access(self) fun increaseBid(vault: @FungibleToken.Vault, ref: String?) {
            pre {
              TicketsAuction.isOpen(): "Auction not open"
              self.bid != nil: "Invalid call"
              vault.balance >= TicketsAuction.increment
            }

            if let bid <- self.bid <- nil {
                bid.doIncrease(from: <- vault, ref: ref)
                let old <- self.bid <- bid
                destroy old
            } else {
                destroy vault
                panic("Never call this")
            }

            emit NewBid(
              uuid: self.bid?.uuid!,
              auctionID: self.id,
              bidder: self.bid?.bidder()!,
              bidPrice: self.bid?.price!
            )
        }

        access(self) fun createBid(
            refund: Capability<&{FungibleToken.Receiver}>,
            recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
            vault: @FungibleToken.Vault,
            ref: String?
        ) {
            pre {
                TicketsAuction.isOpen(): "Auction not open"
                vault.isInstance(TicketsAuction.bidType): "payment vault is not requested fungible token"
            }

            let price = vault.balance
            assert(price >= TicketsAuction.startPrice, message: "Bid price must be greater than start price")
            assert(price >= (self.bid?.price ?? 0.0) + TicketsAuction.increment, message: "bid amount must be larger or equal to the current price + minimum bid increment")

            let lastBid <- self.bid <- create Bid(
              vault: <- vault,
              price: price,
              refund: refund,
              recipient: recipient,
              ref: ref
            )

            let bidder = refund.address
            emit NewBid(
                uuid: self.bid?.uuid!,
                auctionID: self.id,
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

        access(self) fun addUnclaimedBid(bid: @Bid) {
            let bidder = bid.bidder()
            var bids <- TicketsAuction.unclaimedBids.remove(key: bidder)

            if bids == nil {
              bids <-! [] as @[Bid]
            }

            let dummy <- bids!
            dummy.append(<-bid)

            let old <- TicketsAuction.unclaimedBids[bidder] <- dummy
            destroy old
        }

        init(id: UInt64) {
            self.id = id
            self.bid <- nil
        }

        destroy() {
          destroy self.bid
        }
    }

    pub resource Admin {
        pub fun setTime(startAt: UFix64, endAt: UFix64) {
            TicketsAuction.startAt = startAt
            TicketsAuction.endAt = endAt

            emit ConfigChange(type: "setTime", value: [startAt, endAt])
        }

        pub fun setPrice(startPrice: UFix64, increment: UFix64) {
            TicketsAuction.startPrice = startPrice
            TicketsAuction.increment = increment

            emit ConfigChange(type: "setStartPrice", value: [startPrice, increment])
        }

        pub fun createAuction(): UInt64 {
            pre {
                TicketsAuction.total < 71: "Can not create more auction"
            }
            TicketsAuction.total = TicketsAuction.total + 1
            let auction <- create Auction(id: TicketsAuction.total)

            let auctionID = auction.id
            let oldAuction <- TicketsAuction.auctions[auctionID] <- auction

            // Note that oldAuction will always be nil, but we have to handle it.
            destroy oldAuction

            emit AuctionAvailable(auctionID: auctionID)

            return auctionID
        }

        pub fun completeAuction(auctionID: UInt64) {
            let auction <- TicketsAuction.auctions.remove(key: auctionID)
                      ?? panic("missing auction")

            auction.complete()
            destroy auction
        }

        pub fun refundUnclaimedBidForUser(address: Address) {
            if let bids <- TicketsAuction.unclaimedBids.remove(key: address) {
                var i = 0
                while i < bids.length {
                    let ref = &bids[i] as &TicketsAuction.Bid
                    assert(ref.doRefund(), message: "Can't not refund")

                    i = i + 1
                }


                destroy bids
            }
        }
       
    }

    pub fun isOpen(): Bool {
        let now = getCurrentBlock().timestamp

        // startAt <= now <= endAt
        if self.startAt > now {
            return false
        }

        if self.endAt != nil && self.endAt! < now {
            return false
        }

        return true
    }

    pub fun getUnclaimBids(): [Address] {
        return self.unclaimedBids.keys
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
        // Thu Apr 14 2022 12:00:00 GMT+0000
        self.startAt = 1649937600.0
        // Sat Apr 16 2022 13:00:00 GMT+0000
        self.endAt = 1650114000.0
        self.startPrice = 333.3
        self.bidType = Type<@FlowToken.Vault>()
        self.unclaimedBids <- {}

        self.auctions <- {} 
        self.total = 0
        self.increment = 0.1

        self.FlowReceiverPath = /public/flowTokenReceiver
        self.AdminStoragePath = /storage/BNMUAdminTicketsAuctions
        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}
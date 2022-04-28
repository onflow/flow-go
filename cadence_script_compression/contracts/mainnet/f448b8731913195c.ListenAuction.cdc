/*
    ListenAuction 
    Author: Flowstarter
    Auction Resource represents an Auction and is always held internally by the contract
    Admin resource is required to create Auctions
 */

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

import ListenNFT from 0xf448b8731913195c
import ListenUSD from 0xf448b8731913195c

pub contract ListenAuction {

    access(contract) var nextID: UInt64                 // internal ticker for auctionIDs
    access(contract) var auctions: @{UInt64 : Auction}  // Dictionary of Auctions in existence

    access(contract) var EXTENSION_TIME : UFix64        // Currently globally set for all running auctions, consider refactoring to per auction basis for flexibility
    
    pub let AdminStoragePath : StoragePath
    pub let AdminCapabilityPath : CapabilityPath

    pub event ContractDeployed()
    pub event AuctionCreated(auctionID: UInt64, startTime: UFix64, endTime: UFix64, startingPrice: UFix64, bidStep: UFix64, prizeIDs: [UInt64] )
    pub event BidPlaced( auctionID: UInt64, bidder: Address, amount: UFix64 )
    pub event AuctionExtended( auctionID: UInt64, endTime: UFix64 )
    pub event AuctionSettled(id: UInt64, winnersAddress: Address, finalSalePrice: UFix64)
    pub event AuctionRemoved(auctionID: UInt64)

    pub resource Auction {
        access(contract) let startingPrice : UFix64
        access(contract) var bidStep : UFix64       // New bids must be at least bidStep greater than current highest bid
        access(contract) let startTime : UFix64
        
        access(contract) var nftCollection : @ListenNFT.Collection 
        access(contract) var endTime : UFix64       // variable as can be extended if there is a bid in last 30min
        access(contract) var bid : @Bid     
        access(contract) var history : [History]
        
        init( startTime: UFix64, endTime: UFix64, startingPrice: UFix64, bidStep: UFix64, nftCollection: @ListenNFT.Collection) {
            self.startTime = startTime
            self.endTime = endTime
            self.startingPrice = startingPrice
            self.bidStep = bidStep
            self.nftCollection <- nftCollection
            self.bid <- create Bid(funds: <- ListenUSD.createEmptyVault(), ftReceiverCap: nil, nftReceiverCap: nil)
            self.history = []
        }

        access(contract) fun extendAuction() {
            self.endTime = self.endTime + ListenAuction.EXTENSION_TIME
        }

        access(contract) fun placeBid(bid: @ListenAuction.Bid ) {
            var temp <- bid     // new bid in temp variable
            self.bid <-> temp   // swap temp with self.bid
            destroy temp
        }

        access(contract) fun updateHistory(history: History) {
            self.history.append(history)
        }

        access(contract) fun getHistory(): [History]{
            return self.history
        }

        access(contract) fun updateBidStep(_ bidStep: UFix64 ) {
            pre {
                bidStep != self.bidStep : "Bid step already set"
                !self.auctionHasStarted() : "Bid step cannot be changed once auction has started" 
            }
            self.bidStep = bidStep
        }

        pub fun hasBids() : Bool {
            return self.bid.ftReceiverCap != nil 
        }

        pub fun auctionHasStarted() : Bool {
            return ListenAuction.now() >= self.startTime
        }
        
        pub fun getAuctionState(): AuctionState {
            let currentTime = ListenAuction.now()
            if currentTime < self.startTime {
                return AuctionState.Upcoming
            }

            if currentTime >= self.endTime {
                return AuctionState.Complete
            }

            if currentTime < self.endTime - ListenAuction.EXTENSION_TIME {
                return AuctionState.Open
            }

            return AuctionState.Closing
        }
        destroy() {
            if self.nftCollection.getIDs().length > 0 {
                let depositRef = ListenAuction.account.getCapability(ListenNFT.CollectionPublicPath)
                    .borrow<&{NonFungibleToken.CollectionPublic}>()
                    ?? panic("Could not borrow a reference to accounts's collection") // possible safer alternative would be to store in contract 
                
                for id in self.nftCollection.getIDs() {
                    let nft <- self.nftCollection.withdraw(withdrawID: id)
                    depositRef.deposit(token: <- nft)
                }
            }
            destroy self.nftCollection 
            destroy self.bid
        }
    }

    pub struct History{
        pub let auctionID : UInt64
        pub let amount: UFix64
        pub let time : UFix64
        pub let bidderAddress : String

        init(auctionID: UInt64, amount: UFix64, time: UFix64, bidderAddress: String ) {
            self.auctionID = auctionID
            self.amount = amount
            self.time = time
            self.bidderAddress = bidderAddress
        }
    }

    pub resource Bid {
        pub var vault: @ListenUSD.Vault
        pub var ftReceiverCap: Capability?
        pub var nftReceiverCap: Capability?

        init( funds: @ListenUSD.Vault, ftReceiverCap: Capability?, nftReceiverCap: Capability?) {
            self.vault <- funds
            self.ftReceiverCap =  ftReceiverCap
            self.nftReceiverCap = nftReceiverCap
        }

        access(contract) fun returnBidToOwner() {
            let ftReceiverCap = self.ftReceiverCap!
            var ownersVaultRef = ftReceiverCap.borrow<&{FungibleToken.Receiver}>()! 
            let funds <- self.vault.withdraw(amount: self.vault.balance)
            ownersVaultRef.deposit( from: <- funds )
        }

        destroy() {
            if self.vault.balance > 0.0 {
                self.returnBidToOwner() 
            }
            destroy self.vault
        }
    }

    pub resource Admin {
        pub fun createAuction(startTime: UFix64, duration: UFix64, startingPrice: UFix64, bidStep: UFix64, nftCollection: @ListenNFT.Collection) {
            var auction <- create Auction(startTime: startTime, endTime: startTime + duration, startingPrice: startingPrice, bidStep: bidStep, nftCollection: <- nftCollection)
            emit AuctionCreated(auctionID: ListenAuction.nextID, startTime: auction.startTime, endTime: auction.endTime, startingPrice: auction.startingPrice, bidStep: auction.bidStep, prizeIDs: auction.nftCollection.getIDs() )
            
            let temp <- ListenAuction.auctions.insert(key: ListenAuction.nextID, <- auction)
            destroy temp
            
            ListenAuction.nextID = ListenAuction.nextID + 1
        }

        pub fun removeAuction(auctionID: UInt64) {
            let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("Auction ID does not exist")
            let bidRef = &auctionRef.bid as &Bid
            assert( bidRef.vault.balance == 0.0, message: "Auction still has a bid, can't remove")
            for id in auctionRef.nftCollection.getIDs() {
                let nft <- auctionRef.nftCollection.withdraw(withdrawID: id)
                destroy nft
            }

            let auction <- ListenAuction.auctions.remove(key: auctionID)
            destroy auction

            emit AuctionRemoved(auctionID:auctionID)
        }

        pub fun updateExtensionTime(duration: UFix64) {
            pre {
                ListenAuction.auctions.keys.length == 0 : "Must be no active auctions to update rules"
            }
            ListenAuction.EXTENSION_TIME = duration
        }

        pub fun settleAuction( auctionID: UInt64 ) {
            let auctionRef = ListenAuction.borrowAuction(id: auctionID)!
            let bidRef = &auctionRef.bid as &Bid
            assert( ListenAuction.now() >= auctionRef.endTime, message: "Auction must be finished to settle")
            assert( bidRef.vault.balance > 0.0, message: "Auction must have a bid")
            
            let winnerNFTcap = &bidRef.nftReceiverCap! as &Capability 
            let winnersReceiverRef = winnerNFTcap.borrow<&{NonFungibleToken.CollectionPublic}>()!
            for id in auctionRef.nftCollection.getIDs() {
                let nft <- auctionRef.nftCollection.withdraw(withdrawID: id)
                winnersReceiverRef.deposit(token: <- nft)
            }

            let finalSalePrice = bidRef.vault.balance
            let funds <- bidRef.vault.withdraw(amount: finalSalePrice)
            let ftReceiverCap = ListenAuction.account.getCapability(ListenUSD.ReceiverPublicPath) 
            let vaultRef = ftReceiverCap.borrow<&{FungibleToken.Receiver}>()!
            vaultRef.deposit(from: <- funds)

            let auction <- ListenAuction.auctions.remove(key: auctionID)
            destroy auction

            emit AuctionSettled(id: auctionID, winnersAddress: winnerNFTcap.address, finalSalePrice: finalSalePrice)
        }
    }

    pub struct AuctionMeta {
        pub let auctionID: UInt64
        pub let startTime : UFix64
        pub let endTime: UFix64
        pub let startingPrice : UFix64
        pub let bidStep : UFix64
        pub let nftIDs : [UInt64]
        pub let nftCollection: [{String:String}]
        pub let currentBid: UFix64
        pub let auctionState: String
        pub let history: [History]

        init( auctionID: UInt64, startTime: UFix64, endTime: UFix64, startingPrice: UFix64, 
                bidStep: UFix64, nftIDs: [UInt64], nftCollection: [{String:String}],
                currentBid: UFix64, auctionState: String, history: [History] ) {
            self.auctionID = auctionID
            self.startTime = startTime
            self.endTime = endTime
            self.startingPrice = startingPrice
            self.bidStep = bidStep
            self.nftIDs = nftIDs
            self.nftCollection = nftCollection
            self.currentBid = currentBid
            self.auctionState = auctionState
            self.history = history
        }
    }

    pub enum AuctionState: UInt8 {
        pub case Open
        pub case Closing
        pub case Complete
        pub case Upcoming
    }

    pub fun now() : UFix64 {
        return getCurrentBlock().timestamp
    }

    pub fun stateToString(_ auctionState: AuctionState): String {
        switch auctionState{
            case AuctionState.Open:
                return "Open"
            case AuctionState.Closing:
                return "Closing"
            case AuctionState.Complete:
                return "Complete"
            case AuctionState.Upcoming:
                return "Upcoming"
            default:
                return "Upcoming"
        }
    }

    // borrowAuction
    //
    // convenience function to borrow an auction by ID
    access(contract) fun borrowAuction( id: UInt64) : &Auction? {
        if ListenAuction.auctions[id] != nil {
            return &ListenAuction.auctions[id] as &Auction
        } else {
            return nil
        }
    }

    pub fun getAuctionMeta( auctionID: UInt64 ) : AuctionMeta {
        let auctionRef = ListenAuction.borrowAuction( id: auctionID ) ?? panic("No Auction with that ID exists")
        let bidRef = &auctionRef.bid as &Bid
        let vaultRef = &bidRef.vault as &FungibleToken.Vault

        let auctionState = ListenAuction.stateToString(auctionRef.getAuctionState())

        let history: [History] = auctionRef.getHistory()
        let nftCollection : [{String:String}] = []
        for id in auctionRef.nftCollection.getIDs() {
            nftCollection.append(auctionRef.nftCollection.getListenNFTMetadata(id: id))
        }
        return AuctionMeta( 
            auctionID: auctionID,
            startTime: auctionRef.startTime, 
            endTime: auctionRef.endTime, 
            startingPrice: auctionRef.startingPrice, 
            bidStep: auctionRef.bidStep,
            nftIDs: auctionRef.nftCollection.getIDs(), 
            nftCollection: nftCollection, 
            currentBid: vaultRef.balance,
            auctionState: auctionState,
            history: history)
    }

    pub fun placeBid( auctionID: UInt64, funds: @ListenUSD.Vault, ftReceiverCap: Capability<&{FungibleToken.Receiver}>, nftReceiverCap: Capability ) {
        let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("Auction ID does not exist")
        assert( funds.balance >= auctionRef.startingPrice, message: "Bid must be above starting bid" )
        assert( ListenAuction.now() > auctionRef.startTime, message: "Auction hasn't started")
        assert( ListenAuction.now() < auctionRef.endTime, message: "Auction has finished")

        let bidRef = &auctionRef.bid as &Bid
        let currentHighestBid = bidRef.vault.balance
        let newBidAmount = funds.balance

        if auctionRef.hasBids() {
            // bid step only enforced after first bid is placed
            assert( newBidAmount >= currentHighestBid + auctionRef.bidStep , message: "Bid must be greater than current bid + bid step" )
            bidRef.returnBidToOwner()
        }
        // create new bid
        let bid <- create Bid(funds: <- funds, ftReceiverCap: ftReceiverCap, nftReceiverCap: nftReceiverCap)
        auctionRef.placeBid( bid: <- bid)

        let ownersVaultRef = ftReceiverCap.borrow()! // <&{FungibleToken.Receiver}>()! 
        let history = History(
            auctionID: auctionID,
            amount: newBidAmount,
            time: ListenAuction.now(),
            bidderAddress:ownersVaultRef.owner!.address.toString(), 
        )
        auctionRef.updateHistory(history:history)
        
        // extend auction endTime if bid is in final 30mins
        if (ListenAuction.now() > auctionRef.endTime - ListenAuction.EXTENSION_TIME ) {
            auctionRef.extendAuction()
            emit AuctionExtended( auctionID: auctionID, endTime: auctionRef.endTime)
        }

        emit BidPlaced( auctionID: auctionID, bidder: ownersVaultRef.owner?.address!, amount: newBidAmount )
    }

    pub fun getAuctionIDs(): [UInt64] {
        return self.auctions.keys
    }

    init() {
        self.nextID = 0
        self.auctions <- {}

        // Auction extension time is just 30 second for quick testing
        self.EXTENSION_TIME = 1800.0 // = 30 * 60 seconds = 30 mins

        self.AdminStoragePath = /storage/ListenAuctionAdmin
        self.AdminCapabilityPath = /private/ListenAuctionAdmin

        self.account.save(<- create Admin(), to: ListenAuction.AdminStoragePath)
        self.account.link<&ListenAuction.Admin>(ListenAuction.AdminCapabilityPath, target: ListenAuction.AdminStoragePath)

        emit ContractDeployed()
    }
}
 
/*
    ListenAuction 
    Author: Flowstarter
    Auction Resource represents an Auction and is always held internally by the contract
    Admin resource is required to create Auctions
 */

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import ListenNFT from 0xfc354c9bb513e702
import ListenUSD from 0xfc354c9bb513e702


// ListenAuction
//
// Added support for auction NFTs 
// Currently hardcoded to use ListenNFT
// This would be more flexible and allow the use of other tokens in the future...
// Still they would be required to be of the same Type
// Admin account that wants to create a NFT / an auction NFTs
// Each NFT may be listing an auction with a reserve price, bid step, and time
// Bidder can watch, place a bid an auction and get the information of NFTs
// Admin account can create auction / from ids / from the collection
// Admin account can settle the auction
// User account can place bid

pub contract ListenAuction {

    access(contract) var nextID: UInt64                 // internal ticker for auctionIDs
    access(contract) var auctions: @{UInt64 : Auction}  // Dictionary of Auctions in existence

    access(contract) var EXTENSION_TIME : UFix64        // Currently globally set for all running auctions, consider refactoring to per auction basis for flexibility
    
    pub let AdminStoragePath : StoragePath
    pub let AdminCapabilityPath : CapabilityPath

    pub event ContractDeployed()
    pub event AuctionCreated(auctionID: UInt64, startTime: UFix64, endTime: UFix64, startingPrice: UFix64, bidStep: UFix64, position: UInt64, prizeIDs: [UInt64] )
    pub event BidPlaced(auctionID: UInt64, bidder: Address, amount: UFix64, discount: UFix64)
    pub event AuctionExtended( auctionID: UInt64, endTime: UFix64 )
    pub event AuctionSettled(id: UInt64, winnersAddress: Address, finalSalePrice: UFix64)
    pub event AuctionRemoved(auctionID: UInt64)
    pub event AuctionPositionChanged(auctionID: UInt64)

    // Auction
    // A resource that allows an Auction can store an NFT with reserve price, bid step, time, ...
    // 
    pub resource Auction {
        access(contract) let startingPrice : UFix64
        access(contract) var bidStep : UFix64       // New bids must be at least bidStep greater than current highest bid
        access(contract) let startTime : UFix64
        
        access(contract) var nftCollection : @ListenNFT.Collection 
        access(contract) var endTime : UFix64       // variable as can be extended if there is a bid in last 30min
        access(contract) var bid : @Bid     
        access(contract) var position: UInt64
        access(contract) var history : [History]
        
        // initializer
        //
        init( startTime: UFix64, endTime: UFix64, startingPrice: UFix64, bidStep: UFix64, position: UInt64, nftCollection: @ListenNFT.Collection) {
            self.startTime = startTime
            self.endTime = endTime
            self.startingPrice = startingPrice
            self.bidStep = bidStep
            self.nftCollection <- nftCollection
            self.bid <- create Bid(funds: <- ListenUSD.createEmptyVault(), ftReceiverCap: nil, nftReceiverCap: nil)
            self.position = position
            self.history = []
        }

        // extendAuction
        // allow to extend the auction time by 30 minutes (EXTENSION_TIME)
        //
        access(contract) fun extendAuction() {
            self.endTime = self.endTime + ListenAuction.EXTENSION_TIME
        }

        // placeBid
        // allow users to place a bid
        //
        access(contract) fun placeBid(bid: @ListenAuction.Bid ) {
            var temp <- bid     // new bid in temp variable
            self.bid <-> temp   // swap temp with self.bid
            destroy temp
        }

        // updateHistory
        // add the user's auction history to an auction
        //
        access(contract) fun updateHistory(history: History) {
            self.history.append(history)
        }

        // updatePosition
        // change position an auction
        //
        access(contract) fun updatePosition(position: UInt64) {
            self.position = position
        }

        // getHistory
        // get user's auction history of an auction
        //
        access(contract) fun getHistory(): [History]{
            return self.history
        }

        // updateBidStep
        // allow to change the bid step before an auction start
        //
        access(contract) fun updateBidStep(_ bidStep: UFix64 ) {
            pre {
                bidStep != self.bidStep : "Bid step already set"
                !self.auctionHasStarted() : "Bid step cannot be changed once auction has started" 
            }
            self.bidStep = bidStep
        }

        // hasBids
        // checking an auction has a bidder
        // for preventing admin deleting the NFT that is in a started auction
        //
        pub fun hasBids() : Bool {
            return self.bid.ftReceiverCap != nil 
        }

        // auctionHasStarted
        // checking status of an auction has started?
        //
        pub fun auctionHasStarted() : Bool {
            return ListenAuction.now() >= self.startTime
        }
        
        // getAuctionState
        // check and return the status of auction
        //
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
                    .borrow<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>()
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

    // History
    // A struct containing the history of an auction.
    //
    pub struct History{
        // id of Auction
        pub let auctionID : UInt64

        // bidding amount of Auction
        pub let amount: UFix64
        // discount amount of Auction
        pub let discount: UFix64
        pub let time : UFix64

        // Flow wallet address of the user who places a bid
        pub let bidderAddress : String

        // initializer
        //
        init(auctionID: UInt64, amount: UFix64, discount: UFix64, time: UFix64, bidderAddress: String) {
            self.auctionID = auctionID
            self.amount = amount
            self.discount = discount
            self.time = time
            self.bidderAddress = bidderAddress
        }
    }

    // History
    // A struct containing the history of a Bid.
    //
    pub resource Bid {
        pub var vault: @ListenUSD.Vault
        pub var ftReceiverCap: Capability<&{FungibleToken.Receiver}>?
        pub var nftReceiverCap:Capability<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>?

        // initializer
        //
        init(funds: @ListenUSD.Vault, ftReceiverCap: Capability<&{FungibleToken.Receiver}>?, nftReceiverCap: Capability<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>?) {
            self.vault <- funds
            self.ftReceiverCap =  ftReceiverCap
            self.nftReceiverCap = nftReceiverCap
        }

        // returnBidToOwner
        // function returns the number of ListenUSD tokens 
        // to the user who was beatten by another user in an auction
        //
        access(contract) fun returnBidToOwner() {
            let ftReceiverCap = self.ftReceiverCap!
            var ownersVaultRef = ftReceiverCap.borrow()! 
            let funds <- self.vault.withdraw(amount: self.vault.balance)
            ownersVaultRef.deposit(from: <- funds)
        }

        destroy() {
            if self.vault.balance > 0.0 {
                self.returnBidToOwner() 
            }
            destroy self.vault
        }
    }

    // Admin
    // A resource containing the admin right for management the auction contract.
    // Auction Resource represents an Auction and is always held internally by the contract
    // Admin resource is required to create Auctions
    //
    pub resource Admin {

        // createAuction
        // The function allows to create an auction with specific parameters 
        // such as: NFT, time, reserve price and bid step
        //
        pub fun createAuction(startTime: UFix64, duration: UFix64, startingPrice: UFix64, bidStep: UFix64, position: UInt64, nftCollection: @ListenNFT.Collection) {
            var auction <- create Auction(startTime: startTime, endTime: startTime + duration, startingPrice: startingPrice, bidStep: bidStep, position:position, nftCollection: <- nftCollection)
            emit AuctionCreated(auctionID: ListenAuction.nextID, startTime: auction.startTime, endTime: auction.endTime, startingPrice: auction.startingPrice, bidStep: auction.bidStep, position:position, prizeIDs: auction.nftCollection.getIDs() )
            
            let temp <- ListenAuction.auctions.insert(key: ListenAuction.nextID, <- auction)
            destroy temp
            
            ListenAuction.nextID = ListenAuction.nextID + 1
        }

        // removeAuction
        // The function will delete an auction with the input parameter is auctionId
        // 
        pub fun removeAuction(auctionID: UInt64) {
            let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("Auction ID does not exist")
            let bidRef = &auctionRef.bid as &Bid
            assert(bidRef.vault.balance == 0.0, message: "Auction still has a bid, can't remove")
            for id in auctionRef.nftCollection.getIDs() {
                let nft <- auctionRef.nftCollection.withdraw(withdrawID: id)
                let adminReceiver = ListenAuction.account.getCapability<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>(ListenNFT.CollectionPublicPath)
                let receiverRef = adminReceiver!.borrow()!
                receiverRef.deposit(token: <- nft)
            }

            let auction <- ListenAuction.auctions.remove(key: auctionID)
            destroy auction

            emit AuctionRemoved(auctionID:auctionID)
        }

        // updatePosition
        // The function will update position of the auction with the input parameter is auctionId and position
        // 
        pub fun updatePosition(auctionID: UInt64, position: UInt64) {
            let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("Auction ID does not exist")
            auctionRef.updatePosition(position: position)

            emit AuctionPositionChanged(auctionID:auctionID)
        }

        // updateExtensionTime
        // the function to update the remaining time of an auction
        // When the remaining time is less than 30 minutes
        // if there is a user participating in the auction
        // then time will update to 30 minute
        //
        pub fun updateExtensionTime(duration: UFix64) {
            pre {
                ListenAuction.auctions.keys.length == 0 : "Must be no active auctions to update rules"
            }
            ListenAuction.EXTENSION_TIME = duration
        }

        // settleAuction
        // the function processes the auction results,
        // sends the NFT to the winning bidder
        //
        pub fun settleAuction(auctionID: UInt64) {
            let auctionRef = ListenAuction.borrowAuction(id: auctionID)!
            let bidRef = &auctionRef.bid as &Bid
            assert(ListenAuction.now() >= auctionRef.endTime, message: "Auction must be finished to settle")
            assert(bidRef.vault.balance > 0.0, message: "Auction must have a bid")
            
            let winnerNFTcap = bidRef.nftReceiverCap as Capability<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>?
            log(winnerNFTcap)
            var test = winnerNFTcap!.borrow()
            log(test)
            let winnersReceiverRef = test!

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

            emit AuctionSettled(id: auctionID, winnersAddress: winnerNFTcap!.address, finalSalePrice: finalSalePrice)
        }
    }

    // AuctionMeta
    // A struct containing the Metadata of an auction.
    //
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
        pub let position: UInt64
        pub let history: [History]

        // initializer
        //
        init(auctionID: UInt64, startTime: UFix64, endTime: UFix64, startingPrice: UFix64, 
                bidStep: UFix64, nftIDs: [UInt64], nftCollection: [{String:String}],
                currentBid: UFix64, auctionState: String, position: UInt64, history: [History] ) {
            self.auctionID = auctionID
            self.startTime = startTime
            self.endTime = endTime
            self.startingPrice = startingPrice
            self.bidStep = bidStep
            self.nftIDs = nftIDs
            self.nftCollection = nftCollection
            self.currentBid = currentBid
            self.auctionState = auctionState
            self.position = position
            self.history = history
        }
    }


    // AuctionState
    // A enum containing the state of an auction.
    //
    pub enum AuctionState: UInt8 {
        pub case Open
        pub case Closing
        pub case Complete
        pub case Upcoming
    }

    // now
    // Get current time of an auction
    //
    pub fun now() : UFix64 {
        return getCurrentBlock().timestamp
    }

    // stateToString
    // Convert from state of an auction to string
    //
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
    // convenience function to borrow an auction by ID
    // 
    access(contract) fun borrowAuction(id: UInt64) : &Auction? {
        if ListenAuction.auctions[id] != nil {
            return &ListenAuction.auctions[id] as &Auction
        } else {
            return nil
        }
    }

    // getAuctionMeta
    // Returns metadata of an auction by ID
    // 
    pub fun getAuctionMeta(auctionID: UInt64) : AuctionMeta {
        let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("No Auction with that ID exists")
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
            position : auctionRef.position,
            history: history
        )
    }

    // placeBid
    // Function to perform auction of auction participants
    // User have to deposit amount of ListenUSD
    // That amount will be kept in the contract
    //

    // User have to deposit amount of ListenUSD
    // That amount will be kept in the contract
    //
    pub fun placeBid(auctionID: UInt64, funds: @ListenUSD.Vault, discount: UFix64, ftReceiverCap: Capability<&{FungibleToken.Receiver}>,
            nftReceiverCap: Capability<&{NonFungibleToken.CollectionPublic, ListenNFT.CollectionPublic}>) {
        let auctionRef = ListenAuction.borrowAuction(id: auctionID) ?? panic("Auction ID does not exist")
        assert(funds.balance >= auctionRef.startingPrice, message: "Bid must be above starting bid")
        assert(ListenAuction.now() > auctionRef.startTime, message: "Auction hasn't started")
        assert(ListenAuction.now() < auctionRef.endTime, message: "Auction has finished")

        let bidRef = &auctionRef.bid as &Bid
        let currentHighestBid = bidRef.vault.balance
        let newBidAmount = funds.balance

        if auctionRef.hasBids() {
            // bid step only enforced after first bid is placed
            assert(newBidAmount >= currentHighestBid + auctionRef.bidStep , message: "Bid must be greater than current bid + bid step")
            bidRef.returnBidToOwner()
        }
        // create new bid
        let bid <- create Bid(funds: <- funds, ftReceiverCap: ftReceiverCap, nftReceiverCap: nftReceiverCap)
        auctionRef.placeBid( bid: <- bid)

        let ownersVaultRef = ftReceiverCap.borrow()!
        let history = History(
            auctionID: auctionID,
            amount: newBidAmount,
            discount: discount,
            time: ListenAuction.now(),
            bidderAddress:ownersVaultRef.owner!.address.toString(),
        )
        auctionRef.updateHistory(history:history)
        
        // extend auction endTime if bid is in final 30mins
        if (ListenAuction.now() > auctionRef.endTime - ListenAuction.EXTENSION_TIME ) {
            auctionRef.extendAuction()
            emit AuctionExtended( auctionID: auctionID, endTime: auctionRef.endTime)
        }

        emit BidPlaced(auctionID: auctionID, bidder: ownersVaultRef.owner?.address!, amount: newBidAmount, discount: discount)
    }

    // getAuctionIDs
    // The function returns the ID array of the auctions
    // 
    pub fun getAuctionIDs(): [UInt64] {
        return self.auctions.keys
    }

    // initializer
    //
    init() {
        self.nextID = 0
        self.auctions <- {}
        self.EXTENSION_TIME = 1800.0 //30 mins

        self.AdminStoragePath = /storage/ListenAuctionAdmin
        self.AdminCapabilityPath = /private/ListenAuctionAdmin

        self.account.save(<- create Admin(), to: ListenAuction.AdminStoragePath)
        self.account.link<&ListenAuction.Admin>(ListenAuction.AdminCapabilityPath, target: ListenAuction.AdminStoragePath)

        emit ContractDeployed()
    }
}
 
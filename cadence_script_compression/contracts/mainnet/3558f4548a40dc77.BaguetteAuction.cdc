/** 

# BaguetteAuction contract

This contract defines the auction system of Baguette. 
The auction contract acts as an escrow for both NFTs and fungible tokens involved in auctions.
Auctions are centralized in AuctionCollection maintained by admins.  

## Withdrawals and cancelations

Neither can an auction be canceled nor bid withdrawn.

## Auction ends

When the auction has expired, the settle function can be called to transfer the funds and the record.
The settle function is public, since it is risk free (nothing can be changed anyway).

## Create an auction
An Auction is created within an AuctionCollection. An AuctionCollection can be created in two ways:
- by the contract Admin, who can choose the auction parameters. It is used for the primary sales of a record.
- by a Manager who has been initialized by an Admin. The different parameters are fixed at creation by the Admin to the contract parameters at that time.

The second option is used for secondary sales.

Users can only create an auction by accepting an offer made on their NFT as the first bid.

## TimeExtension

An auction is extended by `timeExtension` minutes if a new bid is placed less than `timeExtension` minutes before the end.
*/

import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Record from 0x3558f4548a40dc77
import ArtistRegistery from 0x3558f4548a40dc77


pub contract BaguetteAuction {

    // -----------------------------------------------------------------------
    // Variables 
    // -----------------------------------------------------------------------

    // Resource paths
    // Public path of an auction collection, allowing the place new bids and to access to public information
    pub let CollectionPublicPath: PublicPath
    // Public path of an auction collection, allowing to create new auctions
    pub let CollectionPrivatePath: PrivatePath
    // Storage path of an auction collection
    pub let CollectionStoragePath: StoragePath
    // Manager public path, allowing an Admin to initialize it
    pub let ManagerPublicPath: PublicPath
    // Manager storage path, for a manager to create auction collections
    pub let ManagerStoragePath: StoragePath
    // Admin private path, allowing initialized Manager to create collections while hidding other admin functions
    pub let AdminPrivatePath: PrivatePath
    // Admin storage path
    pub let AdminStoragePath: StoragePath

    // Default parameters for auctions
    pub var parameters: AuctionParameters
    access(self) var marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
    access(self) var lostFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
    access(self) var lostRCollection: Capability<&Record.Collection{Record.CollectionPublic}>?

    // total number of auctions and collections ever created
    pub var totalAuctions: UInt64
    pub var totalCollections: UInt64

    // -----------------------------------------------------------------------
    // Events 
    // -----------------------------------------------------------------------

    // A new auction has been created
    pub event Created(auctionID: UInt64, admin: Address, status: AuctionStatus)
    // A new bid has been placed
    pub event NewBid(auctionID: UInt64, status: AuctionStatus)
    // The auction has been settled
    pub event Settled(auctionID: UInt64)
    
    // Market and Artist share
    pub event MarketplaceEarned(auctionID: UInt64, amount: UFix64, owner: Address)
    pub event ArtistEarned(auctionID: UInt64, amount: UFix64, artistID: UInt64)

    // lost and found events
    pub event FUSDLostAndFound(auctionID: UInt64, amount: UFix64, address: Address)
    pub event RecordLostAndFound(auctionID: UInt64, recordID: UInt64, address: Address)

    // -----------------------------------------------------------------------
    // Resources 
    // -----------------------------------------------------------------------

    // Structure representing auction parameters
    pub struct AuctionParameters {
        pub let artistCut: UFix64 // share of the artist for a sale
        pub let marketCut: UFix64 // share of the marketplace for a sale
        pub let bidIncrement: UFix64 // minimal increment between bids
        pub let auctionLength: UFix64 // length of an auction (before any extension)
        pub let auctionDelay: UFix64 // delay before the start of an auction
        pub let timeExtension: UFix64 // extension when bid in the last `timeExtension` seconds

        init(
            artistCut: UFix64,
            marketCut: UFix64,
            bidIncrement: UFix64,
            auctionLength: UFix64,
            auctionDelay: UFix64,
            timeExtension: UFix64
        ) {
            self.artistCut = artistCut
            self.marketCut = marketCut
            self.bidIncrement = bidIncrement
            self.auctionLength = auctionLength
            self.auctionDelay = auctionDelay
            self.timeExtension = timeExtension
        }
    }

    // This structure holds the main information about an auction
    pub struct AuctionStatus {
        pub let id: UInt64
        pub let recordID: UInt64
        pub let metadata: Record.Metadata
        pub let owner: Address
        pub let leader: Address?
        pub let currentBid: UFix64?
        pub let nextMinBid: UFix64
        pub let numberOfBids: UInt64
        pub let bidIncrement: UFix64
        pub let startTime: Fix64
        pub let endTime: Fix64
        pub let timeExtension: UFix64
        pub let expired: Bool // the auction is expired and should be settled
        pub let timeRemaining: Fix64
    
        init(
            id: UInt64,
            recordID: UInt64, 
            metadata: Record.Metadata,
            owner: Address,
            leader: Address?, 
            currentBid: UFix64?,
            nextMinBid: UFix64,
            numberOfBids: UInt64,
            bidIncrement: UFix64,
            startTime: Fix64,
            endTime: Fix64,
            timeExtension: UFix64,
            expired: Bool
        ) {
            self.id = id
            self.recordID = recordID
            self.metadata = metadata
            self.owner = owner
            self.leader = leader
            self.currentBid = currentBid 
            self.nextMinBid = nextMinBid
            self.numberOfBids = numberOfBids
            self.bidIncrement = bidIncrement
            self.startTime = startTime
            self.endTime = endTime
            self.timeExtension = timeExtension
            if expired {
                self.timeRemaining = 0.0
            }
            else {
                self.timeRemaining = endTime - Fix64(getCurrentBlock().timestamp)
            }
            self.expired = expired
        }
    }

    // Resource representing a unique Auction
    // The item is an optional resource, as it can be sent to the owner/bidder once the auction is settled
    // If the item is nil, the auction becomes invalid (it should be destroyed)
    // It acts as an escrow for the NFT and the fungible tokens, and contains all the capabilities to send the FT and NFT.
    pub resource Auction {
        // The id of this individual auction
        pub let auctionID: UInt64
        // The record for auction
        priv var item: @Record.NFT?
        
        // auction parameters
        pub let parameters: AuctionParameters
        
        priv var numberOfBids: UInt64 // amount of bids which have been placed
        priv var auctionStartTime: UFix64
        priv var auctionEndTime: UFix64

        // the auction has been settled and should be destroyed
        priv var auctionCompleted: Bool

        // Auction State
        priv var startPrice: UFix64
        priv var currentBid: UFix64

        priv let escrow: @FUSD.Vault 

        //the capabilities pointing to the resource where you want the NFT and FT transfered to if you win this bid.
        priv var ownerAddr: Address
        priv var bidderAddr: Address?
        priv var bidderFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
        priv var bidderRCollection: Capability<&Record.Collection{Record.CollectionPublic}>?
        priv let ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        priv let ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>

        init(
            item: @Record.NFT,
            parameters: AuctionParameters,
            auctionStartTime: UFix64,
            startPrice: UFix64, 
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>,
        ) {
            pre {
                ownerFVault.check(): "The fungible vault should be valid."
                ownerRCollection.check(): "The non fungible collection should be valid."
                item.tradable(): "The item cannot be traded due to its current locked mode: it is probably waiting for its decryption key." 
                startPrice>0.0: "Starting price should be greater than 0"
            }

            BaguetteAuction.totalAuctions = BaguetteAuction.totalAuctions + (1 as UInt64)
            self.auctionID = BaguetteAuction.totalAuctions

            self.item <- item
                        
            self.startPrice = startPrice
            self.currentBid = 0.0
            self.parameters = parameters
            self.numberOfBids = 0
            
            self.auctionStartTime = auctionStartTime
            self.auctionEndTime = auctionStartTime + parameters.auctionLength
            self.auctionCompleted = false

            self.escrow <- FUSD.createEmptyVault()
            self.bidderAddr = nil
            self.bidderFVault = nil
            self.bidderRCollection = nil
            self.ownerAddr = ownerFVault.address
            self.ownerFVault = ownerFVault
            self.ownerRCollection = ownerRCollection
        }
        
        // sendNFT sends the NFT to the Collection belonging to the provided Capability or to the lost and found if the capability is broken
        // if both the receiver collection and lost and found are unlinked, the record is destroyed in the `destroy` function of this Auction
        access(self) fun sendNFT(_ capability: Capability<&Record.Collection{Record.CollectionPublic}>) {
            if let collectionRef = capability.borrow() {
                let item <- self.item <- nil
                
                collectionRef.deposit(token: <-item!)
                return
            } 
            else if let collectionRef = BaguetteAuction.lostRCollection!.borrow() {
                let item <- self.item <- nil
                let recordID = item?.id!
                
                collectionRef.deposit(token: <-item!)
                emit RecordLostAndFound(auctionID: self.auctionID, recordID: recordID, address: collectionRef.owner!.address)
                return
            }
        }

        // sendBidTokens sends the bid tokens to the Vault Receiver belonging to the provided Capability or to the lost and found if the capability is broken
        access(self) fun sendBidTokens(_ capability: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            if let vaultRef = capability.borrow() {
                if self.escrow.balance > 0.0 { 
                    vaultRef.deposit(from: <-self.escrow.withdraw(amount: self.escrow.balance))
                }
                return
            }
            else if let vaultRef = BaguetteAuction.lostFVault!.borrow() {
                let balance = self.escrow.balance
                if balance > 0.0 { 
                    vaultRef.deposit(from: <-self.escrow.withdraw(amount: balance))
                }

                emit FUSDLostAndFound(auctionID: self.auctionID, amount: balance, address: vaultRef.owner!.address)
                return
            }
        }

        // Send the previous bid back to the last bidder
        access(self) fun releasePreviousBid() {
            if let vaultCap = self.bidderFVault {
                self.sendBidTokens(vaultCap)
                return
            } 
        }

        // Return the NFT to the owner if no bid has been placed
        access(self) fun retToOwner() {
            // deposit the NFT into the owner's collection
            self.sendNFT(self.ownerRCollection)
        }

        // Extend the auction by the amount of seconds
        access(self) fun extendWith(_ amount: UFix64) {
            self.auctionEndTime = self.auctionEndTime + amount
        }

        // get the remaning time can be negative if it's expired
        pub fun timeRemaining() : Fix64 {
            let currentTime = getCurrentBlock().timestamp
            let remaining= Fix64(self.auctionEndTime) - Fix64(currentTime)
            return remaining
        }

        // Has the auction expired?
        pub fun isAuctionExpired(): Bool {
            let timeRemaining = self.timeRemaining()
            return timeRemaining < Fix64(0.0)
        }

        // Has the auction been settled
        pub fun isAuctionCompleted(): Bool {
            return self.auctionCompleted
        }

        // What the next bid has to match
        pub fun minNextBid() :UFix64{
            //If there are bids then the next min bid is the current price plus the increment
            if self.currentBid != 0.0 {
                return self.currentBid + self.parameters.bidIncrement
            }
            //else start price
            return self.startPrice
        }

        // Settle the auction once it's expired. 
        pub fun settleAuction()  {
            pre {
                !self.auctionCompleted: "The auction is already settled" // cannot be settled twice
                self.item != nil: "Record in auction does not exist" 
                self.isAuctionExpired(): "Auction has not completed yet" 
            }
               
            // return if there are no bids to settle
            if self.currentBid == 0.0 {
                self.retToOwner()
                self.auctionCompleted = true
                
                emit Settled(auctionID: self.auctionID)
                return
            }            


            // Send the market and artist share first
            let amountMarket = self.currentBid * self.parameters.marketCut
            let amountArtist = self.currentBid * self.parameters.artistCut
            let marketCut <- self.escrow.withdraw(amount:amountMarket)
            let artistCut <- self.escrow.withdraw(amount:amountArtist)

            let marketVault = BaguetteAuction.marketVault!.borrow() ?? panic("The market vault link is broken.")
            marketVault.deposit(from: <- marketCut)
            emit MarketplaceEarned(auctionID: self.auctionID, amount: amountMarket, owner: marketVault.owner!.address)
            
            let artistID = self.item?.metadata!.artistID
            ArtistRegistery.sendArtistShare(id: artistID, deposit: <-artistCut)
            emit ArtistEarned(auctionID: self.auctionID, amount: amountArtist, artistID: artistID)
            
            // Send the FUSD to the seller and the NFT to the highest bidder
            self.sendNFT(self.bidderRCollection!)
            self.sendBidTokens(self.ownerFVault)

            self.auctionCompleted = true
            
            emit Settled(auctionID: self.auctionID)
        }

        // Place a new bid
        pub fun placeBid(
            bidTokens: @FungibleToken.Vault, 
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            rCollection: Capability<&Record.Collection{Record.CollectionPublic}>) {
            pre {
                !self.auctionCompleted: "auction has already completed"
                bidTokens.balance >= self.minNextBid(): "bid amount be larger or equal to the current price + minimum bid increment"
                fVault.check(): "The fungible vault should be valid."
                rCollection.check(): "The non fungible collection should be valid."
            }
            
            if self.escrow.balance != 0.0 {
                // bidderFVault should not be nil as something has been placed in escrow
                self.sendBidTokens(self.bidderFVault!)
            }

            // Update the auction item
            self.escrow.deposit(from: <-bidTokens) // will fail if it is not a @FUSD.Vault

            // extend time if in last X seconds
            if self.timeRemaining() < Fix64(self.parameters.timeExtension) {
                self.extendWith(self.parameters.timeExtension)
            }

            // Add the bidder's Vault and NFT receiver references
            self.bidderAddr = rCollection.address
            self.bidderFVault = fVault
            self.bidderRCollection = rCollection    

            // Update the current price of the token
            self.currentBid = self.escrow.balance
            self.numberOfBids = self.numberOfBids + (1 as UInt64)

            let status = self.getAuctionStatus()
            emit NewBid(auctionID: self.auctionID, status: status)
        }

        // Get the auction status
        // Will fail if the auction is completed (item is nil). 
        // A completed auction should be deleted anyway as it is worthless
        pub fun getAuctionStatus(): AuctionStatus {
            return AuctionStatus(
                id: self.auctionID,
                recordID: self.item?.id!,
                metadata: self.item?.metadata!,
                owner: self.ownerAddr,
                leader: self.bidderAddr,
                currentBid: self.currentBid, 
                nextMinBid: self.minNextBid(),
                numberOfBids: self.numberOfBids,
                bidIncrement: self.parameters.bidIncrement,
                startTime: Fix64(self.auctionStartTime),
                endTime: Fix64(self.auctionEndTime),
                timeExtension: self.parameters.timeExtension,
                expired: self.isAuctionExpired()
            )
        }

        destroy() {
            // send the NFT back to auction owner
            if self.item != nil {
                self.sendNFT(self.ownerRCollection)
            }
            
            // if there's a bidder...
            if let vaultCap = self.bidderFVault {
                // ...send the bid tokens back to the bidder
                self.sendBidTokens(vaultCap)
            }

            destroy self.item
            destroy self.escrow
        }
    }

    // CollectionPublic
    //
    // Public methods of an AuctionCollection, allowing to settle an auction and retrieve information
    //
    pub resource interface CollectionPublic {
        pub let collectionID: UInt64
        pub let parameters: AuctionParameters

        pub fun getIDs(): [UInt64]
        pub fun getAuctionStatus(_ recordID:UInt64): AuctionStatus

        // the settle functions are public since they just transfer NFT/tokens when the auction is done
        pub fun settleAuction(_ recordID: UInt64)
    }

    // Bidder
    //
    // Public methods of an Collection, allowing to place bids
    //
    pub resource interface Bidder {
        pub let collectionID: UInt64
        pub let parameters: AuctionParameters

        pub fun placeBid(
            recordID: UInt64, 
            bidTokens: @FungibleToken.Vault, 
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            rCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        )
    }

    // AuctionCreator
    //
    // Interface giving access to the createAuction function
    //
    pub resource interface AuctionCreator {
        pub let collectionID: UInt64
        pub let parameters: AuctionParameters

        pub fun createAuction(
            record: @Record.NFT, 
            startPrice: UFix64,
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>) 
    }

    // Collection
    //
    // Collection representing a set of auctions and their parameters
    //
    pub resource Collection: CollectionPublic, AuctionCreator, Bidder {

        pub let collectionID: UInt64
        pub let parameters: AuctionParameters
        
        // Auction Items, where the key is the recordID
        access(self) var auctionItems: @{UInt64: Auction}

        init(parameters: AuctionParameters) {
            self.auctionItems <- {}
            self.parameters = parameters 

            BaguetteAuction.totalCollections = BaguetteAuction.totalCollections + (1 as UInt64)
            self.collectionID = BaguetteAuction.totalCollections
        }

        // Create an auction with the parameters given at the collection initialization
        pub fun createAuction(
            record: @Record.NFT, 
            startPrice: UFix64,
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            let id = record.id

            let auction <- create Auction(
                item: <-record,
                parameters: self.parameters, 
                auctionStartTime: getCurrentBlock().timestamp + self.parameters.auctionDelay,
                startPrice: startPrice,
                ownerFVault: ownerFVault,
                ownerRCollection: ownerRCollection
            )
            let status = auction.getAuctionStatus()
            self.auctionItems[id] <-! auction
            
            emit Created(auctionID: status.id, admin: self.owner?.address!, status: status)
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.auctionItems.keys
        }

        pub fun getAuctionStatus(_ recordID:UInt64): AuctionStatus {
            pre {
                self.auctionItems[recordID] != nil:
                    "NFT doesn't exist"
            }

            // Get the auction item resources
            return self.auctionItems[recordID]?.getAuctionStatus()!
        }

        // settleAuction sends the auction item to the highest bidder
        // and deposits the FungibleTokens into the auction owner's account
        // destroys the auction if it has already been settled
        pub fun settleAuction(_ recordID: UInt64) { 
            let auctionRef = &self.auctionItems[recordID] as &Auction
            if(auctionRef.isAuctionExpired()) {
                auctionRef.settleAuction()
                if(!auctionRef.isAuctionCompleted()) {
                    panic("Auction was not settled properly")
                }

                destroy self.auctionItems.remove(key: recordID)!
            }
        }


        // placeBid sends the bidder's tokens to the bid vault and updates the
        // currentPrice of the current auction item
        pub fun placeBid(
            recordID: UInt64, 
            bidTokens: @FungibleToken.Vault, 
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            rCollection: Capability<&Record.Collection{Record.CollectionPublic}>) {
            pre {
                self.auctionItems[recordID] != nil:
                    "Auction does not exist in this drop"
            }

            // Get the auction item resources
            let itemRef = &self.auctionItems[recordID] as &Auction
            itemRef.placeBid(
                bidTokens: <-bidTokens, 
                fVault: fVault, 
                rCollection: rCollection
            )
        }

        destroy() {
            destroy self.auctionItems
        }
    }

    // An auction collection creator can create auction collections with default parameters
    pub resource interface CollectionCreator {
        pub fun createAuctionCollection(): @Collection
    }
    
    // Admin can change the default Auction parameters, the market vault and create custom collections
    pub resource Admin: CollectionCreator {
        
        pub fun setParameters(parameters: AuctionParameters) {
            BaguetteAuction.parameters = parameters
        }

        pub fun setMarketVault(marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                marketVault.check(): "The market vault should be valid."
            }
            BaguetteAuction.marketVault = marketVault
        }

        pub fun setLostAndFoundVaults(
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            rCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            pre {
                fVault.check(): "The fungible token vault should be valid."
                rCollection.check(): "The NFT collection should be valid."
            }
            BaguetteAuction.lostFVault = fVault
            BaguetteAuction.lostRCollection = rCollection
        }

        // create collection with default parameters
        pub fun createAuctionCollection(): @Collection {
            return <- create Collection(parameters: BaguetteAuction.parameters)
        }

        // create collection with custom parameters
        pub fun createCustomAuctionCollection(parameters: AuctionParameters): @Collection {
            return <- create Collection(parameters: parameters)
        }
    }

    // This interface is used to add a Admin capability to a client
    pub resource interface ManagerClient {
        pub fun addCapability(_ cap: Capability<&Admin{CollectionCreator}>)
    }

    // An Manager can create an auction collection with the default parameters
    pub resource Manager: ManagerClient, CollectionCreator {

        access(self) var server: Capability<&Admin{CollectionCreator}>?

        init() {
            self.server = nil
        }

         pub fun addCapability(_ cap: Capability<&Admin{CollectionCreator}>) {
            pre {
                cap.check() : "Invalid server capablity"
                self.server == nil : "Server already set"
            }
            self.server = cap
        }

        pub fun createAuctionCollection(): @Collection {
            pre {
                self.server != nil: 
                    "Cannot create AuctionCollection if server is not set"
            }
            
            return <- self.server!.borrow()!.createAuctionCollection()
        }
    }
    
    // -----------------------------------------------------------------------
    // Contract public functions
    // -----------------------------------------------------------------------

    // make it possible to delegate auction collection creation (with default parameters)
    pub fun createManager(): @Manager {
        return <- create Manager()
    }
    
    
    // -----------------------------------------------------------------------
    // Initialization function
    // -----------------------------------------------------------------------

    init() {
        self.totalAuctions = 0
        self.totalCollections = 0

        self.parameters = AuctionParameters(
            artistCut: 0.10,
            marketCut: 0.03,
            bidIncrement: 1.0,
            auctionLength: 259200.0, // 3 days
            auctionDelay: 0.0,
            timeExtension: 600.0 // 10 minutes
        )
        
        self.marketVault = nil
        self.lostFVault = nil
        self.lostRCollection = nil

        self.CollectionPublicPath = /public/boulangeriev1AuctionCollection
        self.CollectionPrivatePath = /private/boulangeriev1AuctionCollection
        self.CollectionStoragePath = /storage/boulangeriev1AuctionCollection
        self.ManagerPublicPath = /public/boulangeriev1AuctionManager
        self.ManagerStoragePath = /storage/boulangeriev1AuctionManager
        self.AdminStoragePath = /storage/boulangeriev1AuctionAdmin
        self.AdminPrivatePath = /private/boulangeriev1AuctionAdmin

        self.account.save(<- create Admin(), to: self.AdminStoragePath)
        self.account.link<&Admin{CollectionCreator}>(self.AdminPrivatePath, target: self.AdminStoragePath)
    }

}
 
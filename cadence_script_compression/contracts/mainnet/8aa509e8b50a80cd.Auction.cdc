import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Collectible from 0x8aa509e8b50a80cd
import NonFungibleToken from 0x1d7e57aa55817448
import Edition from 0x8aa509e8b50a80cd

pub contract Auction {

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    pub struct AuctionStatus{
        pub let id: UInt64
        pub let price : UFix64
        pub let bidIncrement : UFix64
        pub let bids : UInt64
        pub let active: Bool
        pub let timeRemaining : Fix64
        pub let endTime : Fix64
        pub let startTime : Fix64
        pub let metadata: Collectible.Metadata?
        pub let collectibleId: UInt64?     
        pub let leader: Address?
        pub let minNextBid: UFix64
        pub let completed: Bool
        pub let expired: Bool
        pub let cancelled: Bool
        pub let currentLength: UFix64     
     
        init(
            id:UInt64, 
            currentPrice: UFix64, 
            bids:UInt64, 
            active: Bool, 
            timeRemaining:Fix64, 
            metadata: Collectible.Metadata?,
            collectibleId: UInt64?,
            leader:Address?, 
            bidIncrement: UFix64,           
            startTime: Fix64,
            endTime: Fix64,
            minNextBid:UFix64,
            completed: Bool,
            expired:Bool, 
            cancelled: Bool,
            currentLength: UFix64       
        ) {
            self.id = id
            self.price = currentPrice
            self.bids = bids
            self.active = active
            self.timeRemaining = timeRemaining
            self.metadata = metadata
            self.collectibleId = collectibleId
            self.leader = leader
            self.bidIncrement = bidIncrement       
            self.startTime = startTime
            self.endTime = endTime
            self.minNextBid = minNextBid
            self.completed = completed
            self.expired = expired
            self.cancelled = cancelled
            self.currentLength = currentLength  
        }
    }

    // The total amount of AuctionItems that have been created
    pub var totalAuctions: UInt64

    // Events
    pub event CollectionCreated()
    pub event Created(auctionID: UInt64, owner: Address, startPrice: UFix64, startTime: UFix64)
    pub event Bid(auctionID: UInt64, bidderAddress: Address, bidPrice: UFix64, placedAt: Fix64)
    pub event Settled(auctionID: UInt64, price: UFix64)
    pub event Canceled(auctionID: UInt64)
    pub event Earned(nftID: UInt64, amount: UFix64, owner: Address, type: String)
    pub event FailEarned(nftID: UInt64, amount: UFix64, owner: Address, type: String)
    pub event Extend(auctionID: UInt64, auctionLengthFrom: UFix64, auctionLengthTo: UFix64) 
    pub event AddNFT(auctionID: UInt64, nftID: UInt64) 
    pub event BurnNFT(auctionID: UInt64, nftID: UInt64) 
    pub event SendNFT(auctionID: UInt64, nftID: UInt64, to: Address)   
    pub event FailSendNFT(auctionID: UInt64, nftID: UInt64, to: Address) 
    pub event SendBidTokens(auctionID: UInt64, amount: UFix64, to: Address)   
    pub event FailSendBidTokens(auctionID: UInt64, amount: UFix64, to: Address) 

    // AuctionItem contains the Resources and metadata for a single auction
    pub resource AuctionItem {
        
        //Number of bids made, that is aggregated to the status struct
        priv var numberOfBids: UInt64

        //The Item that is sold at this auction
        priv var NFT: @Collectible.NFT?

        //This is the escrow vault that holds the tokens for the current largest bid
        priv let bidVault: @FUSD.Vault

        //The id of this individual auction
        pub let auctionID: UInt64

        //The minimum increment for a bid. This is an english auction style system where bids increase
        priv let minimumBidIncrement: UFix64

        //the time the auction should start at
        priv var auctionStartTime: UFix64

        //The length in seconds for this auction
        priv var auctionLength: UFix64

        //The period of time to extend auction 
        priv var extendedLength: UFix64

        //The period of time of rest to extend
        priv var remainLengthToExtend: UFix64

        //Right now the dropitem is not moved from the collection when it ends, it is just marked here that it has ended 
        priv var auctionCompleted: Bool

        //Start price
        access(account) var startPrice: UFix64

        //Current price
        priv var currentPrice: UFix64

        //the capability that points to the resource where you want the NFT transfered to if you win this bid. 
        priv var recipientCollectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>?

        //the capablity to send the escrow bidVault to if you are outbid
        priv var recipientVaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>?

        //the vault receive FUSD in case of the recipient of commissiona or the previous bidder are unreachable
        priv let platformVaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        //This action was cancelled
        priv var auctionCancelled: Bool

        // Manage royalty for copies of the same items
        priv let editionCap: Capability<&{Edition.EditionCollectionPublic}>

        init(          
            minimumBidIncrement: UFix64,
            auctionStartTime: UFix64,
            startPrice: UFix64, 
            auctionLength: UFix64,         
            extendedLength: UFix64, 
            remainLengthToExtend: UFix64, 
            platformVaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>,         
            editionCap: Capability<&{Edition.EditionCollectionPublic}>
        ) {
            Auction.totalAuctions = Auction.totalAuctions + (1 as UInt64)
            self.NFT <- nil
            self.bidVault <- FUSD.createEmptyVault()
            self.auctionID = Auction.totalAuctions
            self.minimumBidIncrement = minimumBidIncrement
            self.auctionLength = auctionLength            
            self.extendedLength = extendedLength
            self.remainLengthToExtend = remainLengthToExtend
            self.startPrice = startPrice
            self.currentPrice = 0.0
            self.auctionStartTime = auctionStartTime
            self.auctionCompleted = false
            self.recipientCollectionCap = nil
            self.recipientVaultCap = nil         
            self.platformVaultCap = platformVaultCap
            self.numberOfBids = 0
            self.auctionCancelled = false
            self.editionCap = editionCap
        }

        // sendNFT sends the NFT to the Collection belonging to the provided Capability
        priv fun sendNFT(_ capability: Capability<&Collectible.Collection{Collectible.CollectionPublic}>) {
            let nftId = self.NFT?.id!
            if let collectionRef = capability.borrow() {                
                let NFT <- self.NFT <- nil
                collectionRef.deposit(token: <-NFT!)
                emit SendNFT(auctionID: self.auctionID, nftID: nftId, to: collectionRef.owner!.address)  
                return
            }    
            emit FailSendNFT(auctionID: self.auctionID, nftID: nftId, to: self.recipientVaultCap!.borrow()!.owner!.address)
        }

        priv fun burnNFT() {   
            
            if(self.NFT == nil) { 
                return
            }           

            let nftId = self.NFT?.id!

            let NFT <- self.NFT <- nil

            destroy NFT 

            emit BurnNFT(auctionID: self.auctionID, nftID: nftId)          
        }
        
        // sendBidTokens sends the bid tokens to the previous bidder
        priv fun sendBidTokens(_ capability: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            // borrow a reference to the prevous bidder's vault
            if let vaultRef = capability.borrow() {
                let bidVaultRef = &self.bidVault as &FUSD.Vault
                let balance = bidVaultRef.balance
                if(bidVaultRef.balance > 0.0) {
                    vaultRef.deposit(from: <- bidVaultRef.withdraw(amount: balance))
                }

                emit SendBidTokens(auctionID: self.auctionID, amount: balance, to: vaultRef.owner!.address)
                return
            }

            //  platform vault get money in case the previous bidder vault is unreachable
            if let ownerRef = self.platformVaultCap.borrow() {
                let bidVaultRef = &self.bidVault as &FUSD.Vault
                let balance = bidVaultRef.balance
                if(bidVaultRef.balance > 0.0) {
                    ownerRef.deposit(from: <-bidVaultRef.withdraw(amount: balance))
                }
                emit FailSendBidTokens(auctionID: self.auctionID, amount: balance, to: ownerRef.owner!.address)
                return
            }
        }

        priv fun releasePreviousBid() {
            if let vaultCap = self.recipientVaultCap {
                self.sendBidTokens(self.recipientVaultCap!)
                return
            } 
        }

        pub fun getEditionNumber(id: UInt64): UInt64? {             
            return self.NFT?.editionNumber
        }

        priv fun sendCommissionPayment() {

            let editionNumber = self.NFT?.editionNumber!

            let editionRef = self.editionCap.borrow()!   

            let editionStatus = editionRef.getEdition(editionNumber)!  

            for key in editionStatus.royalty.keys {
                if (editionStatus.royalty[key]!.firstSalePercent > 0.0) {
                    let commission = self.currentPrice * editionStatus.royalty[key]!.firstSalePercent * 0.01

                    let account = getAccount(key) 

                    let vaultCap = account.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(/public/fusdReceiver)    

                    if (vaultCap.check()) {
                        let vault = vaultCap.borrow()!
                        vault.deposit(from: <- self.bidVault.withdraw(amount: commission))
                        emit Earned(nftID: self.NFT?.id!, amount: commission, owner: key, type: editionStatus.royalty[key]!.description)
                    } else {
                        emit FailEarned(nftID: self.NFT?.id!, amount: commission, owner: key, type: editionStatus.royalty[key]!.description)
                    }            
                }                
            }

            // If commission was not paid, this money get platform

            if (self.bidVault.balance > 0.0) {

                let amount = self.bidVault.balance

                let platformVault = self.platformVaultCap.borrow()!

                platformVault.deposit(from: <- self.bidVault.withdraw(amount: amount))

                emit Earned(nftID: self.NFT?.id!, amount: amount, owner: platformVault.owner!.address, type: "PLATFORM")
            }
        }

        pub fun settleAuction()  {

            pre {
                !self.auctionCancelled : "The auction was cancelled"
                !self.auctionCompleted : "The auction has been already settled"
                self.NFT != nil: "NFT in auction does not exist"
                self.isAuctionExpired() : "Auction has not completed yet"               
            }

            // burn token if there are no bids to settle
            if self.currentPrice == 0.0 {
                self.burnNFT()
                self.auctionCompleted = true
                emit Settled(auctionID: self.auctionID, price: self.currentPrice)
                return
            }       

            self.sendCommissionPayment()
           
            self.sendNFT(self.recipientCollectionCap!)
         
            self.auctionCompleted = true
            
            emit Settled(auctionID: self.auctionID, price: self.currentPrice)
        }

        //this can be negative if is expired
        pub fun timeRemaining() : Fix64 {
            let auctionLength = self.auctionLength

            let startTime = self.auctionStartTime

            let currentTime = getCurrentBlock().timestamp

            let remaining = Fix64(startTime + auctionLength) - Fix64(currentTime)

            return remaining
        }

        pub fun isAuctionExpired(): Bool {
            let timeRemaining = self.timeRemaining()
            return timeRemaining < Fix64(0.0)
        }

        pub fun minNextBid() : UFix64 {
            //If there are bids then the next min bid is the current price plus the increment
            if self.currentPrice != 0.0 {
                return self.currentPrice + self.currentPrice * self.minimumBidIncrement * 0.01
            }

            //else start Collectible price
            return self.startPrice
        }

        priv fun extendAuction() {
            if (
                //Auction time left is less than remainLengthToExtend
                self.timeRemaining() < Fix64(self.remainLengthToExtend)     
            ) {
                self.auctionLength = self.auctionLength + self.extendedLength
                emit Extend(auctionID: self.auctionID, auctionLengthFrom: self.auctionLength - self.extendedLength, auctionLengthTo: self.auctionLength)
            }            
        }

        pub fun bidder() : Address? {
            if let vaultCap = self.recipientVaultCap {
                // Check possible situation, where vault was unlinked after bid
                // Test this case in automated test
                if !vaultCap.check() {
                    return nil
                }

                return vaultCap.borrow()!.owner!.address
            }
            return nil
        }

        pub fun currentBidForUser(address:Address): UFix64 {            
            if(self.bidder() == address) {
                return self.bidVault.balance
            }
            return 0.0
        }

        // This method should probably use preconditions more
        pub fun placeBid(bidTokens: @FUSD.Vault, vaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>, collectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>) {

            pre {
                vaultCap.check() : "Fungible token storage is not initialized on account"
                collectionCap.check() : "NFT storage is not initialized on account"
                !self.auctionCancelled : "Auction was cancelled"
                self.NFT != nil: "NFT in auction does not exist"
                self.auctionStartTime < getCurrentBlock().timestamp : "The auction has not started yet"             
                !self.isAuctionExpired() : "Time expired"
                bidTokens.balance <= 999999.99 : "Bid should be less than 1 000 000.00"  
            }

            let bidderAddress = vaultCap.borrow()!.owner!.address
            let collectionAddress = collectionCap.borrow()!.owner!.address

            if bidderAddress != collectionAddress {
              panic("you cannot make a bid and send the Collectible to somebody else collection")
            }

            let amountYouAreBidding = bidTokens.balance + self.currentBidForUser(address: bidderAddress)

            let minNextBid = self.minNextBid()

            if amountYouAreBidding < minNextBid {
                panic("Bid is less than min acceptable")
            }

            if self.bidder() != bidderAddress {
              if self.bidVault.balance != 0.0 {
                // Return the previous bid 
                self.sendBidTokens(self.recipientVaultCap!)
              }
            }

            // Update the bidVault to store the current bid
            self.bidVault.deposit(from: <-bidTokens)

            //update the capability of the wallet for the address with the current highest bid
            self.recipientVaultCap = vaultCap

            // Update the current price of the token
            self.currentPrice = self.bidVault.balance

            // Add the bidder's Vault and NFT receiver references
            self.recipientCollectionCap = collectionCap
            self.numberOfBids = self.numberOfBids + (1 as UInt64)

            // Extend auction according to time left and extened length
            self.extendAuction() 

            emit Bid(auctionID: self.auctionID, bidderAddress: bidderAddress, bidPrice: self.currentPrice, placedAt: Fix64(getCurrentBlock().timestamp))
        }

        pub fun getAuctionStatus() : AuctionStatus {

            var leader : Address? = nil
            
            if let recipient = self.recipientVaultCap {
                leader = recipient.borrow()!.owner!.address
            }

            return AuctionStatus(
                id: self.auctionID,
                currentPrice: self.currentPrice, 
                bids: self.numberOfBids,
                active: !self.auctionCompleted && !self.isAuctionExpired(),
                timeRemaining: self.timeRemaining(),
                metadata: self.NFT?.metadata,
                collectibleId: self.NFT?.id,
                leader: leader,
                bidIncrement: self.minimumBidIncrement,         
                startTime: Fix64(self.auctionStartTime),
                endTime: Fix64(self.auctionStartTime+self.auctionLength),
                minNextBid: self.minNextBid(),
                completed: self.auctionCompleted,
                expired: self.isAuctionExpired(),
                cancelled: self.auctionCancelled,
                currentLength: self.auctionLength
            )
        }

        pub fun cancelAuction() {
            pre {
                !self.auctionCancelled : "The auction has been already cancelled"
                !self.auctionCompleted : "The auction was settled"           
            }
            self.releasePreviousBid()
            self.burnNFT()
            self.auctionCancelled = true
        }

        pub fun addNFT(NFT: @Collectible.NFT) {
            pre {
                self.NFT == nil : "NFT in auction has already existed"
            }

            let nftID = NFT.id

            self.NFT <-! NFT

            emit AddNFT(auctionID: self.auctionID, nftID: nftID) 
        }

        pub fun reclaimSendNFT(collectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>)  {

            pre {
                self.auctionCompleted : "The auction has not been settled yet"
                self.NFT != nil: "NFT in auction does not exist"      
            }

            self.sendNFT(collectionCap)         
        }

        destroy() {
            log("destroy auction")
                       
            // if there's a bidder, therefore minumum one bid
            if let vaultCap = self.recipientVaultCap {
                // ...send the bid tokens back to the bidder
                self.sendBidTokens(vaultCap)
            }

            self.burnNFT()

            destroy self.NFT
            destroy self.bidVault
        }
    }    

    // AuctionCollectionPublic is a resource interface that restricts users to
    // retreiving the auction price list and placing bids
    pub resource interface AuctionCollectionPublic {

        pub fun getAuctionStatuses(): {UInt64: AuctionStatus}
        pub fun getAuctionStatus(_ id:UInt64): AuctionStatus?
        pub fun getTimeLeft(_ id: UInt64): Fix64?
     
        pub fun placeBid(
            id: UInt64, 
            bidTokens: @FUSD.Vault, 
            vaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>, 
            collectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>
        )
    }

    // AuctionCollection contains a dictionary of AuctionItems and provides
    // methods for manipulating the AuctionItems
    pub resource AuctionCollection: AuctionCollectionPublic {

        // Auction Items
        access(account) var auctionItems: @{UInt64: AuctionItem}       

        init() { 
            self.auctionItems <- {}
        }

        pub fun keys() : [UInt64] {
            return self.auctionItems.keys
        }

        // addTokenToauctionItems adds an NFT to the auction items and sets the meta data
        // for the auction item
        pub fun createAuction(       
            minimumBidIncrement: UFix64, 
            auctionLength: UFix64,           
            extendedLength: UFix64, 
            remainLengthToExtend: UFix64,
            auctionStartTime: UFix64,
            startPrice: UFix64,           
            platformVaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>,         
            editionCap: Capability<&{Edition.EditionCollectionPublic}>
        ): UInt64 {

            pre {              
                auctionLength > 0.00 : "Auction lenght should be more than 0.00"
                auctionStartTime > getCurrentBlock().timestamp : "Auction start time can't be in the past"
                startPrice > 0.00 : "Start price should be more than 0.00"
                startPrice <= 999999.99 : "Start bid should be less than 1 000 000.00"
                minimumBidIncrement > 0.00 : "Minimum bid increment should be more than 0.00"
                platformVaultCap.check() : "Platform vault should be reachable"
            }
            
            // create a new auction items resource container
            let item <- create AuctionItem(         
                minimumBidIncrement: minimumBidIncrement,
                auctionStartTime: auctionStartTime,
                startPrice: startPrice,
                auctionLength: auctionLength,           
                extendedLength: extendedLength,    
                remainLengthToExtend:  remainLengthToExtend,                
                platformVaultCap: platformVaultCap,
                editionCap: editionCap
            )

            let id = item.auctionID

            // update the auction items dictionary with the new resources
            let oldItem <- self.auctionItems[id] <- item
            
            destroy oldItem

            let owner = platformVaultCap.borrow()!.owner!.address

            emit Created(auctionID: id, owner: owner, startPrice: startPrice, startTime: auctionStartTime)

            return id
        }

        // getAuctionPrices returns a dictionary of available NFT IDs with their current price
        pub fun getAuctionStatuses(): {UInt64: AuctionStatus} {
           
            if self.auctionItems.keys.length == 0 { 
                return {} 
            }           

            let priceList: {UInt64: AuctionStatus} = {}

            for id in self.auctionItems.keys {
                let itemRef = &self.auctionItems[id] as? &AuctionItem
                priceList[id] = itemRef.getAuctionStatus()
            }
            
            return priceList
        }

        pub fun getAuctionStatus(_ id:UInt64): AuctionStatus? {
    
            if  self.auctionItems[id] == nil {
                return  nil
            }        

            // Get the auction item resources
            let itemRef = &self.auctionItems[id] as &AuctionItem
            return itemRef.getAuctionStatus()
        }

        pub fun getTimeLeft(_ id: UInt64): Fix64? {
            if(self.auctionItems[id] == nil) {
                return nil
            }

            // Get the auction item resources
            let itemRef = &self.auctionItems[id] as &AuctionItem
            return itemRef.timeRemaining()
        }

        // settleAuction sends the auction item to the highest bidder
        // and deposits the FungibleTokens into the auction owner's account
        pub fun settleAuction(_ id: UInt64) {
            pre {
                self.auctionItems[id] != nil: "Auction does not exist"
            }

            let itemRef = &self.auctionItems[id] as &AuctionItem
            itemRef.settleAuction()
        }

        pub fun cancelAuction(_ id: UInt64) {
            pre {
                self.auctionItems[id] != nil: "Auction does not exist"
            }
            let itemRef = &self.auctionItems[id] as &AuctionItem           
            itemRef.cancelAuction()
            emit Canceled(auctionID: id)
        }

        // placeBid sends the bidder's tokens to the bid vault and updates the
        // currentPrice of the current auction item
        pub fun placeBid(id: UInt64, bidTokens: @FUSD.Vault, vaultCap: Capability<&FUSD.Vault{FungibleToken.Receiver}>, collectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>) {
            pre {
                self.auctionItems[id] != nil:
                    "Auction does not exist in this drop"
            }

            // Get the auction item resources
            let itemRef = &self.auctionItems[id] as &AuctionItem
            itemRef.placeBid(
                bidTokens: <- bidTokens, 
                vaultCap : vaultCap, 
                collectionCap:collectionCap
            )
        }

        pub fun addNFT(id: UInt64, NFT: @Collectible.NFT) {
            pre {
                self.auctionItems[id] != nil:
                    "Auction does not exist"
            }
            let itemRef = &self.auctionItems[id] as &AuctionItem

            itemRef.addNFT(NFT: <- NFT)
        }

        pub fun reclaimSendNFT(id: UInt64, collectionCap: Capability<&Collectible.Collection{Collectible.CollectionPublic}>) {
            pre {
                self.auctionItems[id] != nil: "Auction does not exist"
            }
            let itemRef = &self.auctionItems[id] as &AuctionItem           
            itemRef.reclaimSendNFT(collectionCap: collectionCap)   
        }

        destroy() {
            log("destroy auction collection")
            // destroy the empty resources
            destroy self.auctionItems
        }
    }

    // createAuctionCollection returns a new AuctionCollection resource to the caller
    priv fun createAuctionCollection(): @AuctionCollection {
        let auctionCollection <- create AuctionCollection()
    
        return <- auctionCollection
    }

    init() {
        self.totalAuctions = (0 as UInt64)
        self.CollectionPublicPath = /public/NFTxtinglesAuction
        self.CollectionStoragePath = /storage/NFTxtinglesAuction

        let sale <- Auction.createAuctionCollection()
        self.account.save(<-sale, to:Auction.CollectionStoragePath)         
        self.account.link<&{Auction.AuctionCollectionPublic}>(Auction.CollectionPublicPath, target:Auction.CollectionStoragePath)
    }   
}
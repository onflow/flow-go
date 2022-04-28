// DisruptArt NFT Marketplace
// Auction smart contract
// NFT Marketplace : www.disrupt.art
// Owner           : Disrupt Art, INC.
// Developer       : www.blaze.ws
// Version         : 0.0.1
// Blockchain      : Flow www.onFlow.org

import FungibleToken from 0xf233dcee88fe0abe
import DisruptArt from 0xcd946ef9b13804c6
import NonFungibleToken from 0x1d7e57aa55817448
import DisruptArtMarketplaceFlow from 0xcd946ef9b13804c6
import DisruptArtAuction from 0xcd946ef9b13804c6

pub contract DisruptArtAuctionFlow {

    // The total amount of AuctionItems that have been created
    pub var totalAuctions: UInt64

    // Events
    pub event TokenAddedToAuctionItems(auctionID: UInt64, startPrice: UFix64, minimumBidIncrement: UFix64, auctionStartBlock: UInt64, tokenID: UInt64, endTime:Fix64)
    pub event NewBid(auctionID: UInt64, bidPrice: UFix64, bidder: Address?)
    pub event AuctionSettled(auctionID: UInt64, price: UFix64)
    pub event Canceled(auctionID: UInt64)

    // Auction Storage Path
    pub let auctionStoragePath: StoragePath

    /// Auction Public Path
    pub let auctionPublicPath: PublicPath

    // This struct aggreates status for the auction and is exposed in order to create websites using auction information
    pub struct AuctionStatus{
        pub let id: UInt64
        pub let price : UFix64
        pub let bidIncrement : UFix64
        pub let bids : UInt64
        pub let active: Bool
        pub let endTime : Fix64
        pub let startTime : Fix64
        pub let artId: UInt64?
        pub let owner: Address
        pub let leader: Address?
        pub let completed: Bool
        pub let expired: Bool
    
        init(id:UInt64, 
            currentPrice: UFix64, 
            bids:UInt64, 
            active: Bool, 
            artId: UInt64?,
            leader:Address?, 
            bidIncrement: UFix64,
            owner: Address, 
            startTime: Fix64,
            endTime: Fix64,
            completed: Bool,
            expired:Bool
        ) {
            self.id=id
            self.price= currentPrice
            self.bids=bids
            self.active=active
            self.artId=artId
            self.leader= leader
            self.bidIncrement=bidIncrement
            self.owner=owner
            self.startTime=startTime
            self.endTime=endTime
            self.completed=completed
            self.expired=expired
        }
    }

    // AuctionItem contains the Resources for a single auction
    pub resource AuctionItem {

        //Number of bids made, that is aggregated to the status struct
        priv var numberOfBids: UInt64
        
        // Resources
        priv var NFT: @NonFungibleToken.NFT?
        priv let bidVault: @FungibleToken.Vault

        // Auction Settings
        pub let auctionID: UInt64
        priv let minimumBidIncrement: UFix64

        // Auction State
        access(account) var startPrice: UFix64
        priv var currentPrice: UFix64

        priv let auctionStartBlock: UInt64
        priv var auctionCompleted: Bool

        priv let endTime : Fix64
        priv let startTime : Fix64
    
        priv let resale: Bool
        priv let creator: Address?

        // Recipient's Receiver Capabilities
        priv var recipientCollectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>
        priv var recipientVaultCap: Capability<&{FungibleToken.Receiver}>?

        // Owner's Receiver Capabilities
        priv let ownerCollectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>
        priv let ownerVaultCap: Capability<&{FungibleToken.Receiver}>

        init(
            NFT: @NonFungibleToken.NFT,
            bidVault: @FungibleToken.Vault,
            auctionID: UInt64,
            minimumBidIncrement: UFix64,
            startPrice: UFix64,
            auctionStartBlock: UInt64,
            startTime : Fix64,
            endTime : Fix64,
            resale: Bool,
            creator: Address?,
            ownerCollectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>,
            ownerVaultCap: Capability<&{FungibleToken.Receiver}>
        ) {
            self.NFT <- NFT
            self.bidVault <- bidVault
            self.auctionID = auctionID
            self.minimumBidIncrement = minimumBidIncrement
            self.startPrice = startPrice
            self.currentPrice = startPrice
            self.auctionStartBlock = auctionStartBlock
            self.auctionCompleted = false
            self.endTime = endTime
            self.startTime = startTime
            self.resale = resale
            self.creator = creator
            self.recipientCollectionCap = ownerCollectionCap
            self.recipientVaultCap = ownerVaultCap
            self.ownerCollectionCap = ownerCollectionCap
            self.ownerVaultCap = ownerVaultCap
            self.numberOfBids = 0   
        }

        // depositBidTokens deposits the bidder's tokens into the AuctionItem's Vault
        pub fun depositBidTokens(vault: @FungibleToken.Vault) {
            self.bidVault.deposit(from: <-vault)
        }

        // withdrawNFT removes the NFT from the AuctionItem and returns it to the caller
        pub fun withdrawNFT(): @NonFungibleToken.NFT {
            let NFT <- self.NFT <- nil
            return <- NFT!
        }
        
        // sendNFT sends the NFT to the Collection belonging to the provided Capability
        access(contract) fun sendNFT(_ capability: Capability<&{DisruptArt.DisruptArtCollectionPublic}>) {
            // borrow a reference to the owner's NFT receiver
            if let collectionRef = capability.borrow() {
                let NFT <- self.withdrawNFT()
                // deposit the token into the owner's collection
                collectionRef.deposit(token: <-NFT)
            } else {
                panic("sendNFT(): unable to borrow collection ref")
            }
        }

        // sendBidTokens sends the bid tokens to the Vault Receiver belonging to the provided Capability
        access(contract) fun sendBidTokens(_ capability: Capability<&{FungibleToken.Receiver}>, sale: Bool) {
            // borrow a reference to the owner's NFT receiver
            if let vaultRef = capability.borrow() {
                let bidVaultRef = &self.bidVault as &FungibleToken.Vault
                var balance = 0.0
                if(sale){
                    let marketShare = (bidVaultRef.balance / 100.0 ) * DisruptArtMarketplaceFlow.marketFee
                    let royalityShare = (bidVaultRef.balance / 100.0 ) * DisruptArtMarketplaceFlow.royality
                    balance = bidVaultRef.balance - (marketShare + royalityShare)
                      
                    let marketCut <- bidVaultRef.withdraw(amount: marketShare)
                    let royalityCut <- bidVaultRef.withdraw(amount: royalityShare)

                    let disruptartvaultRef =  getAccount(DisruptArtMarketplaceFlow.marketAddress)
                                  .getCapability(/public/flowTokenReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to Marketplace vault")

                    // let itemRef = &self.auctionItems[id] as? &AuctionItem

                    let creatorvaultRef =  getAccount(self.creator!!)
                                 .getCapability(/public/flowTokenReceiver)
                                .borrow<&{FungibleToken.Receiver}>()
                                ?? panic("failed to borrow reference to owner vault")

                    disruptartvaultRef.deposit(from: <-marketCut)

                    if(self.resale) {
                       creatorvaultRef.deposit(from: <-royalityCut) 
                    } else {
                       disruptartvaultRef.deposit(from: <-royalityCut)
                    }

                } else {
                    balance = bidVaultRef.balance
                }              

                vaultRef.deposit(from: <-bidVaultRef.withdraw(amount:balance))
            } else {
                panic("couldn't get vault ref")
            }
        }

        // settleAuction sends the auction item to the highest bidder
        // and deposits the FungibleTokens into the auction owner's account
        pub fun settleAuction() {

            pre {
                !self.auctionCompleted : "The auction is already settled"
                self.NFT != nil: "NFT in auction does not exist"
                self.isAuctionExpired() : "Auction has not completed yet"
            }
               
            // return if there are no bids to settle
            if self.currentPrice == self.startPrice {
                self.returnAuctionItemToOwner()
            } else {            
                self.exchangeTokens()
            }

            self.auctionCompleted = true
            
            emit AuctionSettled(auctionID: self.auctionID, price: self.currentPrice)

        }

        // isAuctionExpired returns true if the auction has exceeded it's length in blocks,
        // otherwise it returns false
        pub fun isAuctionExpired(): Bool {
   
            let currentTime = getCurrentBlock().timestamp
            
            if Fix64(self.endTime) < Fix64(currentTime) {
                return true
            } else {
                return false
            }
        }

        // returnAuctionItemToOwner releases any bids and returns the NFT
        // to the owner's Collection
        pub fun returnAuctionItemToOwner() {
            pre {
                self.NFT != nil: "NFT in auction does not exist"
            }
            
            // release the bidder's tokens
            self.releasePreviousBid()
            
            // deposit the NFT into the owner's collection
            self.sendNFT(self.ownerCollectionCap)
        }


        // exchangeTokens sends the purchased NFT to the buyer and the bidTokens to the seller
        pub fun exchangeTokens() {
            pre {
                self.NFT != nil: "NFT in auction does not exist"
            }

            self.sendNFT(self.recipientCollectionCap)
            self.sendBidTokens(self.ownerVaultCap, sale:true)
        }

        // releasePreviousBid returns the outbid user's tokens to
        // their vault receiver
        pub fun releasePreviousBid() {
            // release the bidTokens from the vault back to the bidder
            if let vaultCap = self.recipientVaultCap {
                self.sendBidTokens(self.recipientVaultCap!, sale:false)
            } else {
                panic("unable to get vault capability")
            }
        }

         pub fun cancelAuction() {
            pre {
                !self.auctionCompleted : "The auction is already settled"
                self.NFT != nil: "NFT in auction does not exist"
                self.isAuctionExpired() == false : "Auciton expired, can't cancel"
            }
            
            self.returnAuctionItemToOwner()
          
            self.auctionCompleted = true
            
            emit Canceled(auctionID: self.auctionID)
          
        }

        pub fun placeBid(bidTokens: @FungibleToken.Vault, vaultCap: Capability<&{FungibleToken.Receiver}>, collectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>)  {
            pre {
                !self.auctionCompleted : "The auction is already settled"
                self.NFT != nil: "NFT in auction does not exist"
                !self.isAuctionExpired() : "Auciton expired, can't place a bid"
            }
           
            if bidTokens.balance < (self.currentPrice + self.minimumBidIncrement) {
                panic("bid amount be larger than minimum bid increment")
            }
            
            if self.bidVault.balance != UFix64(0) {
                if let vaultCapy = self.recipientVaultCap {
                    self.sendBidTokens(vaultCapy, sale:false)
                } else {
                    panic("unable to get recipient Vault capability")
                }
            }

            // Update the auction item
            self.depositBidTokens(vault: <-bidTokens)

            // Update the current price of the token
            self.currentPrice = self.bidVault.balance

            // Add the bidder's Vault and NFT receiver references
            self.recipientCollectionCap = collectionCap
            self.recipientVaultCap = vaultCap
            self.numberOfBids=self.numberOfBids+(1 as UInt64)

            emit NewBid(auctionID: self.auctionID, bidPrice: self.currentPrice, bidder: vaultCap.address)
        }

        pub fun getAuctionStatus() :AuctionStatus {

            var leader:Address?= nil
            if let recipient = self.recipientVaultCap {
                leader=recipient.address
            }

            return AuctionStatus(
                id:self.auctionID,
                currentPrice: self.currentPrice, 
                bids: self.numberOfBids,
                active: !self.auctionCompleted  && !self.isAuctionExpired(),
                artId: self.NFT?.id,
                leader: leader,
                bidIncrement: self.minimumBidIncrement,
                owner: self.ownerVaultCap.address,
                startTime: Fix64(self.startTime),
                endTime: Fix64(self.endTime),
                completed: self.auctionCompleted,
                expired: self.isAuctionExpired()
            )
        }


        destroy() {
            // send the NFT back to auction owner
            self.sendNFT(self.ownerCollectionCap)
            
            // if there's a bidder...
            if let vaultCap = self.recipientVaultCap {
                // ...send the bid tokens back to the bidder
                self.sendBidTokens(vaultCap, sale:false)
            }

            destroy self.NFT
            destroy self.bidVault
        }
    }

    // AuctionPublic is a resource interface that restricts users to
    // retreiving the auction price list and placing bids
    pub resource interface AuctionPublic {
        pub fun getAuctionKeys() : [UInt64]

        pub fun getAuctionStatuses(): {UInt64: AuctionStatus}
        pub fun getAuctionStatus(_ id:UInt64): AuctionStatus

        pub fun placeBid(
            id: UInt64, 
            bidTokens: @FungibleToken.Vault, 
            vaultCap: Capability<&{FungibleToken.Receiver}>, 
            collectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>
        )

        pub fun settleAuction(_ id: UInt64)
    }

    // AuctionCollection contains a dictionary of AuctionItems and provides
    // methods for manipulating the AuctionItems
    pub resource AuctionCollection: AuctionPublic {

        // Auction Items
        access(account) var auctionItems: @{UInt64: AuctionItem}
        
        init() {
            self.auctionItems <- {}
        }

        // addTokenToauctionItems adds an NFT to the auction items 
        pub fun addTokenToAuctionItems(token: @NonFungibleToken.NFT, minimumBidIncrement: UFix64, startPrice: UFix64, bidVault: @FungibleToken.Vault, collectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>, vaultCap: Capability<&{FungibleToken.Receiver}>, endTime : Fix64) {
            
            pre {
                Fix64(getCurrentBlock().timestamp) < endTime : "endtime should be greater than current time"
                minimumBidIncrement > 0.0 : "minimumBidIncrement should be greater than 0.0"
            }

            let bidtoken <-token as! @DisruptArt.NFT
            
            let tokenID = bidtoken.id

            let resale = (bidtoken.creator == self.owner?.address) ? false : true

            let creator = bidtoken.creator 

            let itemToken <- bidtoken as! @NonFungibleToken.NFT
     
            DisruptArtAuctionFlow.totalAuctions = DisruptArtAuctionFlow.totalAuctions + UInt64(1)

            let id = DisruptArtAuctionFlow.totalAuctions

            let startBlock = getCurrentBlock().height

            let startTime = Fix64(getCurrentBlock().timestamp)

            // create a new auction items resource container
            let item <- create AuctionItem(
                NFT: <-itemToken,
                bidVault: <-bidVault,
                auctionID: id,
                minimumBidIncrement: minimumBidIncrement,
                startPrice: startPrice,
                auctionStartBlock: startBlock,
                startTime : startTime,
                endTime : endTime,
                resale: resale,
                creator: creator,
                ownerCollectionCap: collectionCap,
                ownerVaultCap: vaultCap
            )

            

            // update the auction items dictionary with the new resources
            let oldItem <- self.auctionItems[id] <- item
            destroy oldItem

            emit TokenAddedToAuctionItems(auctionID: id, startPrice: startPrice, minimumBidIncrement: minimumBidIncrement, auctionStartBlock: startBlock, tokenID:tokenID, endTime:endTime)
        }

        pub fun getAuctionStatuses(): {UInt64: AuctionStatus} {
            pre {
                self.auctionItems.keys.length > 0: "There are no auction items"
            }

            let auctionList: {UInt64: AuctionStatus} = {}

            for id in self.auctionItems.keys {
                let itemRef = &self.auctionItems[id] as? &AuctionItem
                auctionList[id] = itemRef.getAuctionStatus()
            }

            return auctionList

        }


        pub fun getAuctionStatus(_ id:UInt64): AuctionStatus {
            pre {
                self.auctionItems[id] != nil:
                    "Auction doesn't exist"
            }

            // Get the auction item resources
            let itemRef = &self.auctionItems[id] as &AuctionItem
            let status = itemRef.getAuctionStatus()
            return status
        }

        pub fun getAuctionKeys() : [UInt64] {

            pre {
                self.auctionItems.keys.length > 0: "There are no auction items"
            }

            return self.auctionItems.keys

        }

        // settleAuction sends the auction item to the highest bidder
        // and deposits the FungibleTokens into the auction owner's account
        pub fun settleAuction(_ id: UInt64) {
            pre {
                self.auctionItems[id] != nil:
                    "Auction doesn't exist"
            }

            let itemRef = &self.auctionItems[id] as &AuctionItem
            itemRef.settleAuction()
        }

        pub fun cancelAuction(_ id: UInt64) {
            pre {
                self.auctionItems[id] != nil:
                    "Auction does not exist"
            }

            let itemRef = &self.auctionItems[id] as &AuctionItem
            itemRef.cancelAuction()
          
        }

        // placeBid sends the bidder's tokens to the bid vault and updates the
        // currentPrice of the current auction item
        pub fun placeBid(id: UInt64, bidTokens: @FungibleToken.Vault, vaultCap: Capability<&{FungibleToken.Receiver}>, collectionCap: Capability<&{DisruptArt.DisruptArtCollectionPublic}>)  {
           
            pre {
                self.auctionItems[id] != nil:
                    "Auction doesn't exist"
            }

            // Get the auction item resources
            let itemRef = &self.auctionItems[id] as &AuctionItem

            itemRef.placeBid(bidTokens: <- bidTokens, vaultCap: vaultCap, collectionCap: collectionCap)

        }

        destroy() {
            // destroy the empty resources
            destroy self.auctionItems
        }
    }

    // createAuctionCollection returns a new AuctionCollection resource to the caller
    pub fun createAuctionCollection(): @AuctionCollection {
        let auctionCollection <- create AuctionCollection()
        return <- auctionCollection
    }

    init() {
        self.totalAuctions = DisruptArtAuction.totalAuctions
        self.auctionStoragePath= /storage/DisruptArtAuctionFlow
        self.auctionPublicPath= /public/DisruptArtAuctionFlow
    }   
}
 

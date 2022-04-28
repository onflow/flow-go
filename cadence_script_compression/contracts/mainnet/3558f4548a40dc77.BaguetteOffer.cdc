/** 

# BaguetteOffer contract

This contract defines the offer system of Baguette. 
The offer contract acts as an escrow for fungible tokens which can be exchange when presented the corresponding Record.
Offers are centralized in OfferCollection maintained by admins.  

## Withdrawals and cancelations

Offers can be cancelled by the offeror.

## Create an offer
An Offer is created within an OfferCollection. An OfferCollection can be created in two ways:
- by the contract Admin, who can choose the offer parameters. 
- by an Manager who has been initialized by an Admin. The different parameters are fixed at creation by the Admin to the contract parameters at that time.

## Accepting an offer

A seller can present an NFT with a corresponding offer and has two solutions to accept it:
- direct sale
- accept the offer as a first bid of an auction
*/

import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Record from 0x3558f4548a40dc77
import ArtistRegistery from 0x3558f4548a40dc77
import BaguetteAuction from 0x3558f4548a40dc77

pub contract BaguetteOffer {

    // -----------------------------------------------------------------------
    // Variables 
    // -----------------------------------------------------------------------

    // Resource paths
    // Public path of an offer collection, allowing the place new offers and to access to public information
    pub let CollectionPublicPath: PublicPath
    // Storage path of an offer collection
    pub let CollectionStoragePath: StoragePath
    // Manager public path, allowing an Admin to initialize it
    pub let ManagerPublicPath: PublicPath
    // Manager storage path, for a manager to create offer collections
    pub let ManagerStoragePath: StoragePath
    // Offeror storage path
    pub let OfferorStoragePath: StoragePath
    // Admin storage path
    pub let AdminStoragePath: StoragePath
    // Admin private path, allowing initialized AuctionManager to create collections while hidding other admin functions
    pub let AdminPrivatePath: PrivatePath

    // Default parameters for offers
    pub var parameters: OfferParameters
    access(self) var marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
    access(self) var lostFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?
    access(self) var lostRCollection: Capability<&Record.Collection{Record.CollectionPublic}>?

    // total number of offers ever created
    pub var totalOffers: UInt64

    // -----------------------------------------------------------------------
    // Events 
    // -----------------------------------------------------------------------

    // A new offer has been made
    pub event NewOffer(offerID: UInt64, admin: Address, status: OfferStatus)
    // The offer had been canceled
    pub event OfferCanceled(offerID: UInt64)
    // The offer has been accepted directly
    pub event AcceptedDirectly(offerID: UInt64)
    // The offer has been accepted as first bid
    pub event AcceptedAsBid(offerID: UInt64)
    
    // Market and Artist share
    pub event MarketplaceEarned(offerID: UInt64, amount: UFix64, owner: Address)
    pub event ArtistEarned(offerID: UInt64, amount: UFix64, artistID: UInt64)

    // lost and found events
    pub event FUSDLostAndFound(offerID: UInt64, amount: UFix64, address: Address)
    pub event RecordLostAndFound(offerID: UInt64, recordID: UInt64, address: Address)

    // -----------------------------------------------------------------------
    // Resources 
    // -----------------------------------------------------------------------

    // Structure representing offer parameters
    pub struct OfferParameters {
        pub let artistCut: UFix64 // share of the artist for a sale
        pub let marketCut: UFix64 // share of the marketplace for a sale
        pub let offerIncrement: UFix64 // minimal increment between offers
        pub let timeBeforeCancel: UFix64 // minimal amount of time before an offer can be canceled

        init(
            artistCut: UFix64,
            marketCut: UFix64,
            offerIncrement: UFix64,
            timeBeforeCancel: UFix64
        ) {
            self.artistCut = artistCut
            self.marketCut = marketCut
            self.offerIncrement = offerIncrement
            self.timeBeforeCancel = timeBeforeCancel
        }
    }

    // This structure holds the main information about an offer
    pub struct OfferStatus {
        pub let id: UInt64
        pub let recordID: UInt64
        pub let offeror: Address
        pub let offer: UFix64
        pub let nextMinOffer: UFix64
        pub let offerIncrement: UFix64
    
        init(
            id: UInt64,
            recordID: UInt64,
            offeror: Address, 
            offer: UFix64,
            nextMinOffer: UFix64,
            offerIncrement: UFix64
        ) {
            self.id = id
            self.recordID = recordID
            self.offeror = offeror
            self.offer = offer 
            self.nextMinOffer = nextMinOffer
            self.offerIncrement = offerIncrement
        }
    }

    // Resource representing a unique Offer
    pub resource Offer {
        pub let creationTime: UFix64
        pub let offerID: UInt64
        pub let recordID: UInt64
        pub let parameters: OfferParameters

        priv let offer: UFix64
        priv let offeror: Address
        priv let escrow: @FUSD.Vault 
        priv var isValid: Bool

        //the capabilities pointing to the resource where you want the NFT
        priv var offerorFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        priv var offerorRCollection: Capability<&Record.Collection{Record.CollectionPublic}>

        init(
            parameters: OfferParameters,
            recordID: UInt64,
            offerTokens: @FungibleToken.Vault,
            offerorFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            offerorRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            pre {
                offerorFVault.check(): "The fungible vault should be valid."
                offerorRCollection.check(): "The non fungible collection should be valid."
            }

            self.creationTime = getCurrentBlock().timestamp

            BaguetteOffer.totalOffers = BaguetteOffer.totalOffers + (1 as UInt64)
            self.offerID = BaguetteOffer.totalOffers

            self.parameters = parameters
            self.recordID = recordID

            self.escrow <- offerTokens as! @FUSD.Vault
            self.offer = self.escrow.balance
            self.offeror = offerorFVault.address
            self.offerorFVault = offerorFVault
            self.offerorRCollection = offerorRCollection

            self.isValid = true
        }

        // sendNFT sends the NFT to the Collection belonging to the provided Capability or to the lost and found if the capability is broken
        // if both the receiver collection and lost and found are unlinked, the record is destroyed
        access(self) fun sendNFT(record: @Record.NFT, rCollection: Capability<&Record.Collection{Record.CollectionPublic}>) {
            if let collectionRef = rCollection.borrow() {
                collectionRef.deposit(token: <-record)
                return
            } 

            if let collectionRef = BaguetteOffer.lostRCollection!.borrow() {
                let recordID = record.id
                collectionRef.deposit(token: <-record)
                
                emit RecordLostAndFound(offerID: self.offerID, recordID: recordID, address: collectionRef.owner!.address)
                return 
            }
            
            // should never happen in practice
            destroy record
        }

        // sendOfferTokens sends the bid tokens to the Vault Receiver belonging to the provided Capability
        access(self) fun sendOfferTokens(_ capability: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            if let vaultRef = capability.borrow() {
                if self.escrow.balance > 0.0 { 
                    vaultRef.deposit(from: <-self.escrow.withdraw(amount: self.escrow.balance))
                }
                return
            }
            else if let vaultRef = BaguetteOffer.lostFVault!.borrow() {
                let balance = self.escrow.balance
                if balance > 0.0 { 
                    vaultRef.deposit(from: <-self.escrow.withdraw(amount: balance))
                    emit FUSDLostAndFound(offerID: self.offerID, amount: balance, address: vaultRef.owner!.address)
                }
                return
            }
        }

        // Send the previous bid back to the last bidder
        access(contract) fun cancelOffer() {
            pre {
                self.isValid: "Offer is not valid."
            }

            self.isValid = false
            self.sendOfferTokens(self.offerorFVault) 
        }

        // Accept the offer directly
        access(contract) fun acceptOffer(
            record: @Record.NFT,
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        ) {
            pre {
                record.tradable(): "The item cannot be traded due to its current locked mode: it is probably waiting for its decryption key."
                self.isValid: "Offer is not valid."
            }

            let amountMarket = self.offer * self.parameters.marketCut
            let amountArtist = self.offer * self.parameters.artistCut
            let marketCut <- self.escrow.withdraw(amount:amountMarket)
            let artistCut <- self.escrow.withdraw(amount:amountArtist)

            let marketVault = BaguetteOffer.marketVault!.borrow() ?? panic("The market vault link is broken.")
            marketVault.deposit(from: <- marketCut)
            emit MarketplaceEarned(offerID: self.offerID, amount: amountMarket, owner: marketVault.owner!.address)

            let artistID = record.metadata.artistID
            ArtistRegistery.sendArtistShare(id: artistID, deposit: <-artistCut)
            emit ArtistEarned(offerID: self.offerID, amount: amountArtist, artistID: artistID)

            self.sendOfferTokens(ownerFVault)
            self.sendNFT(record: <-record, rCollection: self.offerorRCollection)
            self.isValid = false 

            emit AcceptedDirectly(offerID: self.offerID)
        }

        // create an auction with the offer as first bid
        // if the offeror vault and collections are not valid anymore, it could block the function
        // instead, the tokens are sent to LostAndFound, and the NFT returned to the owner,
        // to deter bad behavior
        access(contract) fun acceptAuctionOffer(
            auctionCollection: &BaguetteAuction.Collection{BaguetteAuction.Bidder, BaguetteAuction.AuctionCreator},
            record: @Record.NFT,
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            pre {
                record.tradable(): "The item cannot be traded due to its current locked mode: it is probably waiting for its decryption key."
                self.isValid: "Offer is not valid."
            }

            if(!self.offerorFVault.check() || !self.offerorRCollection.check()) {
                self.sendOfferTokens(BaguetteOffer.lostFVault!)
                self.sendNFT(record: <-record, rCollection: ownerRCollection)
                self.isValid = false

                return
            }

            let recordID = record.id
            auctionCollection.createAuction(
                record: <-record, 
                startPrice: self.offer,
                ownerFVault: ownerFVault,
                ownerRCollection: ownerRCollection
            ) 

            auctionCollection.placeBid(
                recordID: recordID, 
                bidTokens: <-self.escrow.withdraw(amount: self.offer), 
                fVault: self.offerorFVault, 
                rCollection: self.offerorRCollection
            )
            self.isValid = false 

            emit AcceptedAsBid(offerID: self.offerID)
        }

        // What the next offer has to match
        pub fun minNextOffer(): UFix64{
            return self.offer + self.parameters.offerIncrement
        }

        // Get the auction status
        // Will fail is offer is not valid
        // It is worthless and should be destroyed
        pub fun getOfferStatus(): OfferStatus {
            pre {
                self.isValid: "Offer is not valid."
            }

            return OfferStatus(
                id: self.offerID,
                recordID: self.recordID,
                offeror: self.offeror,
                offer: self.offer,
                nextMinOffer: self.minNextOffer(),
                offerIncrement: self.parameters.offerIncrement
            )
        }

        destroy() {
            // if the offer is still valid, it should be canceled
            if self.isValid {
                self.cancelOffer()
            }

            destroy self.escrow
        }
    }

    // CollectionPublic
    //
    // Public methods of an OfferCollection, getting status of offers
    //
    pub resource interface CollectionPublic {
        pub let parameters: OfferParameters

        pub fun getIDs(): [UInt64]
        pub fun getOfferStatus(_ recordID:UInt64): OfferStatus
    }

    // Seller
    //
    // Interface exposing functions to accept an offer
    //
    pub resource interface Seller {
        pub let parameters: OfferParameters

        pub fun acceptDirectOffer(record: @Record.NFT, ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>)
        pub fun acceptAuctionOffer(
            record: @Record.NFT, 
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        )
    }

    // Offeror
    //
    // Interface exposing function to post or cancel an offer
    //
    pub resource interface Offeror {
        access(contract) fun addOffer(
            recordID: UInt64, 
            offerTokens: @FungibleToken.Vault,
            offerorFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            offerorRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        )
        access(contract) fun cancelOffer(recordID: UInt64, offerID: UInt64)
    }

    // AuctionCreatorClient
    //
    // Allows to receive an auctionCreator capability to create auctions when accepting offers
    //
    pub resource interface AuctionCreatorClient {
        pub fun addCapability(_ cap: Capability<&BaguetteAuction.Collection{BaguetteAuction.Bidder, BaguetteAuction.AuctionCreator}>)
    }

    // Collection
    //
    // Collection allowing to create new auctions
    //
    pub resource Collection: CollectionPublic, Seller, Offeror, AuctionCreatorClient {

        pub let parameters: OfferParameters
        access(self) var auctionServer: Capability<&BaguetteAuction.Collection{BaguetteAuction.Bidder, BaguetteAuction.AuctionCreator}>?
        access(self) var auctionServerAccepted: UInt64?
        
        // Auction Items, where the key is the recordID
        access(self) var offerItems: @{UInt64: Offer}

        init(parameters: OfferParameters) {
            self.offerItems <- {}
            self.parameters = parameters 

            self.auctionServer = nil
            self.auctionServerAccepted = nil
        }

        pub fun setAuctionServerAccepted(serverID: UInt64) {
            self.auctionServerAccepted = serverID
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.offerItems.keys
        }

        pub fun getOfferStatus(_ recordID:UInt64): OfferStatus {
            pre {
                self.offerItems[recordID] != nil:
                    "NFT doesn't exist"
            }

            // Get the auction item resources
            return self.offerItems[recordID]?.getOfferStatus()!
        }

        pub fun acceptDirectOffer(record: @Record.NFT, ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                self.offerItems[record.id] != nil:
                    "NFT doesn't exist"
            }
            let recordID = record.id 
            let itemRef = &self.offerItems[recordID] as &Offer
            itemRef.acceptOffer(record: <-record, ownerFVault: ownerFVault)

            destroy self.offerItems.remove(key: recordID)!
        }

        pub fun acceptAuctionOffer(
            record: @Record.NFT, 
            ownerFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            ownerRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            pre {
                self.offerItems[record.id] != nil:
                    "NFT doesn't exist"
                self.auctionServer != nil : "Auction server not set"
                self.auctionServer!.check() : "Auction server link broken"
            }
            let recordID = record.id 
            let itemRef = &self.offerItems[recordID] as &Offer
            itemRef.acceptAuctionOffer(
                auctionCollection: self.auctionServer!.borrow()!,
                record: <-record, 
                ownerFVault: ownerFVault,
                ownerRCollection: ownerRCollection
            )

            destroy self.offerItems.remove(key: recordID)!
        }

        access(contract) fun addOffer(
            recordID: UInt64, 
            offerTokens: @FungibleToken.Vault,
            offerorFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            offerorRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            // check if there is an existing offer
            if(self.offerItems[recordID] != nil) {
                let itemRef = &self.offerItems[recordID] as &Offer
                if(itemRef.getOfferStatus().nextMinOffer > offerTokens.balance) {
                    panic("The offer is not high enough")
                }
                let id = itemRef.offerID
                itemRef.cancelOffer()
                destroy self.offerItems.remove(key: recordID)!
                emit OfferCanceled(offerID: id)
            } 

            let offer <- create Offer(
                parameters: self.parameters,
                recordID: recordID,
                offerTokens: <-offerTokens,
                offerorFVault: offerorFVault,
                offerorRCollection: offerorRCollection
            )
            let offerStatus = offer.getOfferStatus()
            let old <- self.offerItems[recordID] <- offer
            destroy old

            emit NewOffer(offerID: offerStatus.id, admin: self.owner?.address!, status: offerStatus)
        }

        access(contract) fun cancelOffer(recordID: UInt64, offerID: UInt64) {
            pre {
                self.offerItems[recordID] != nil: "No offer for this item." 
            }
            let itemRef = &self.offerItems[recordID] as &Offer
            if(itemRef.offerID != offerID) {
                panic("The ID of the offer does not match the current offer on this item.")
            }
            if(itemRef.creationTime + self.parameters.timeBeforeCancel > getCurrentBlock().timestamp) {
                panic("The offer cannot be canceled yet.")
            }
            let id = itemRef.offerID
            itemRef.cancelOffer()

            destroy self.offerItems.remove(key: recordID)!

            emit OfferCanceled(offerID: id)
        }

        pub fun addCapability(_ cap: Capability<&BaguetteAuction.Collection{BaguetteAuction.Bidder, BaguetteAuction.AuctionCreator}>) {
            pre {
                cap.check() : "Invalid server capablity"
                self.auctionServer == nil : "Server already set"
                self.auctionServerAccepted != nil: "No auction server can be accepted yet"
                cap.borrow()!.collectionID == self.auctionServerAccepted!: "This is not the correct auction server"
            }
            self.auctionServer = cap
        }
        
        destroy() {
            destroy self.offerItems
        }
    }

    // CollectionCreator
    //
    // An auction collection creator can create offer collection with default parameters
    //
    pub resource interface CollectionCreator {
        pub fun createOfferCollection(): @Collection
    }
    
    // Admin
    //
    // Admin can change the default Offer parameters, the market vault and create custom collections
    //
    pub resource Admin: CollectionCreator {
        
        pub fun setParameters(parameters: OfferParameters) {
            BaguetteOffer.parameters = parameters
        }

        pub fun setMarketVault(marketVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                marketVault.check(): "The market vault should be valid."
            }
            BaguetteOffer.marketVault = marketVault
        }

        pub fun setLostAndFoundVaults(
            fVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            rCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            pre {
                fVault.check(): "The fungible token vault should be valid."
                rCollection.check(): "The NFT collection should be valid."
            }
            BaguetteOffer.lostFVault = fVault
            BaguetteOffer.lostRCollection = rCollection
        }

        // create collection with default parameters
        pub fun createOfferCollection(): @Collection {
            return <- create Collection(parameters: BaguetteOffer.parameters)
        }

        // create collection with custom parameters
        pub fun createCustomOfferCollection(parameters: OfferParameters): @Collection {
            return <- create Collection(parameters: parameters)
        }
    }

    // ManagerClient
    //
    // This interface is used to add a Admin capability to a client
    //
    pub resource interface ManagerClient {
        pub fun addCapability(_ cap: Capability<&Admin{CollectionCreator}>)
    }

    // Manager
    //
    // An Manager can create OfferCollection with the default parameters
    //
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

        pub fun createOfferCollection(): @Collection {
            pre {
                self.server != nil: 
                    "Cannot create OfferCollection if server is not set"
            }
            
            return <- self.server!.borrow()!.createOfferCollection()
        }
    }

    // OfferorCollection
    //
    // Lists all the offers of a buyer and give them the possibility to cancel offers
    //
    pub resource OfferorCollection {

        priv let offers: {UInt64: UInt64} // recordID -> offerID
        init() {
            self.offers = {}
        }

        pub fun addOffer(
            offerCollection: &Collection{Offeror},
            recordID: UInt64, 
            offerTokens: @FungibleToken.Vault,
            offerorFVault: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
            offerorRCollection: Capability<&Record.Collection{Record.CollectionPublic}>
        ) {
            offerCollection.addOffer(
                recordID: recordID,
                offerTokens: <-offerTokens,
                offerorFVault: offerorFVault,
                offerorRCollection: offerorRCollection
            )

            self.offers[recordID] = BaguetteOffer.totalOffers
        }

        pub fun cancelOffer(
            offerCollection: &Collection{Offeror},
            recordID: UInt64
        ) {
            pre {
                self.offers[recordID] != nil: "There is no offer for this item."
            }

            offerCollection.cancelOffer(recordID: recordID, offerID: self.offers[recordID]!)
            self.offers.remove(key: recordID)
        }

        pub fun hasOffer(recordID: UInt64): Bool {
            return false
        }
    }
    
    // -----------------------------------------------------------------------
    // Contract public functions
    // -----------------------------------------------------------------------

    // 
    pub fun createManager(): @Manager {
        return <- create Manager()
    }

    // 
    pub fun createOfferor(): @OfferorCollection {
        return <- create OfferorCollection()
    }
    
    
    // -----------------------------------------------------------------------
    // Initialization function
    // -----------------------------------------------------------------------

    init() {
        self.totalOffers = 0

        self.parameters = OfferParameters(
            artistCut: 0.10,
            marketCut: 0.03,
            offerIncrement: 1.0,
            timeBeforeCancel: 86400.0
        )
        
        self.marketVault = nil
        self.lostFVault = nil
        self.lostRCollection = nil

        self.CollectionPublicPath = /public/boulangeriev1OfferCollection
        self.CollectionStoragePath = /storage/boulangeriev1OfferCollection
        self.ManagerPublicPath = /public/boulangeriev1OfferManager
        self.ManagerStoragePath = /storage/boulangeriev1OfferManager
        self.OfferorStoragePath = /storage/boulangeriev1OfferOfferor
        self.AdminStoragePath = /storage/boulangeriev1OfferAdmin
        self.AdminPrivatePath = /private/boulangeriev1OfferAdmin

        self.account.save(<- create Admin(), to: self.AdminStoragePath)
        self.account.link<&Admin{CollectionCreator}>(self.AdminPrivatePath, target: self.AdminStoragePath)
    }

}
 
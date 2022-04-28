import VeraEvent from 0x97a1ae2c0cebb6fb
import NonFungibleToken from 0x1d7e57aa55817448

// VeraTicket
// Ticket NFT items Contract!
//
pub contract VeraTicket: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)
    pub event Destroy(id: UInt64)

    // Named Paths
    //
    pub let VeraTicketStorage: StoragePath
    pub let VeraTicketPubStorage: PublicPath
    pub let VeraMinterStorage: StoragePath

    // totalSupply
    // The total number of Tickets that have been minted
    //
    pub var totalSupply: UInt64

    // Declare an enum named `Color` which has the raw value type `UInt8`,
    // and declare three enum cases: `red`, `green`, and `blue`
    //
    pub enum NFTType: UInt8 {
        pub case GeneralAdmission
        pub case AssignedSeating
    }

    // NFT
    // A Ticket as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64

        pub let eventID: UInt64
        
        pub let type: VeraTicket.NFTType

        pub let tier: UInt64

        pub let subtier: UInt64

        pub let tokenURI: String
        
        // initializer
        //
        init(initID: UInt64, initEventID: UInt64, initType: VeraTicket.NFTType, initTier: UInt64, initSubTier: UInt64, initTokenURI: String) {
            self.id = initID
            self.eventID = initEventID
            self.type = initType
            self.tier = initTier
            self.subtier = initSubTier
            self.tokenURI = initTokenURI
        }
    }

    // NFT
    // A Ticket as an NFT
    //
    pub struct NFTStruct {
        
        pub let eventID: UInt64
        
        pub let type: VeraTicket.NFTType

        pub let tier: UInt64

        pub let subtier: UInt64

        pub let tokenURI: String
        
        // initializer
        //
        init(initEventID: UInt64, initType: VeraTicket.NFTType, initTier: UInt64, initSubTier: UInt64, initTokenURI: String) {
            self.eventID = initEventID
            self.type = initType
            self.tier = initTier
            self.subtier = initSubTier
            self.tokenURI = initTokenURI
        }
    }

    // This is the interface that users can cast their Tickets Collection as
    // to allow others to deposit Tickets into their Collection. It also allows for reading
    // the details of Tickets in the Collection.
    pub resource interface TicketsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTicket(id: UInt64): &VeraTicket.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Ticket reference: The ID of the returned reference is incorrect"
            }
        }
        pub fun destroyTicket(eventID: UInt64, id: UInt64, tier: UInt64, subtier: UInt64)
    }

    // Collection
    // A collection of Ticket NFTs owned by an account
    //
    pub resource Collection: TicketsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @VeraTicket.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowTicket
        // Gets a reference to an NFT in the collection as a Ticket,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the Ticket.
        //
        pub fun borrowTicket(id: UInt64): &VeraTicket.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &VeraTicket.NFT
            } else {
                return nil
            }
        }

        // destructor
        pub fun destroyTicket(eventID: UInt64, id: UInt64, tier: UInt64, subtier: UInt64) {
            if self.ownedNFTs[id] != nil {
                let token <- self.ownedNFTs.remove(key: id) ?? panic("missing NFT")
                destroy token
                let eventCollection = VeraTicket.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
                eventCollection.decrementTicketMinted(eventId: eventID, tier: tier, subtier: subtier)
                emit Destroy(id: id)
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {

		// mintNFT
        // Mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
        //
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic, VeraTicket.TicketsCollectionPublic}, eventID: UInt64, type: VeraTicket.NFTType, tier: UInt64, subtier: UInt64, tokenURI: String) {

            let veraevent: VeraEvent.EventStruct = VeraEvent.getEvent(id: eventID)
            var eventMaxTickets:UInt64 = veraevent.maxTickets;
            var eventTotalTicketsMinted:UInt64 = veraevent.totalTicketsMinted;
            var maxTickets:UInt64 = 0;
            var ticketsMinted:UInt64 = 0

            if (type == VeraTicket.NFTType.GeneralAdmission) {
                var eventtier: VeraEvent.Tier = veraevent.getTier(tier: tier)!
                maxTickets = eventtier.maxTickets
                ticketsMinted = eventtier.ticketsMinted
            } else if (type == VeraTicket.NFTType.AssignedSeating) {
                var eventtier: VeraEvent.Tier = veraevent.getTier(tier: tier)!
                var eventsubtier: VeraEvent.SubTier = veraevent.getSubTier(tier: tier, subtier: subtier)!
                maxTickets = eventsubtier.maxTickets
                ticketsMinted = eventsubtier.ticketsMinted
            }

            if (ticketsMinted < maxTickets && eventTotalTicketsMinted < eventMaxTickets) {
                // deposit it in the recipient's account using their reference
                recipient.deposit(token: <-create VeraTicket.NFT(initID: VeraTicket.totalSupply, initEventID: eventID, initType: type, initTier: tier, initSubTier: subtier, initTokenURI: tokenURI))

                emit Minted(id: VeraTicket.totalSupply)

                VeraTicket.totalSupply = VeraTicket.totalSupply + (1 as UInt64)
            } else {
                panic("Max Tickets Minted")
            }
		}


    // mintNFT
        // Mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
        //
		pub fun mintMultipleNFT(recipient: &{NonFungibleToken.CollectionPublic, VeraTicket.TicketsCollectionPublic}, eventID: UInt64, tickets:[VeraTicket.NFTStruct], gatickets: UInt64, astickets: UInt64) {
            let eventCollection = VeraTicket.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
            let veraevent: VeraEvent.EventStruct = VeraEvent.getEvent(id: eventID)
            var eventMaxTickets:UInt64 = veraevent.maxTickets;
            var eventTotalTicketsMinted:UInt64 = veraevent.totalTicketsMinted + gatickets + astickets;
            var maxTickets:UInt64 = 0;
            var ticketsMinted:UInt64 = 0

            if (tickets.length > 100) {
              panic("Cannot Mint Tickets more than 100 in one batch")
            }

            if (eventTotalTicketsMinted > eventMaxTickets) {
              panic("Cannot Mint Tickets more than Event Max Tickets")
            }

            var gaMaxTicket: UInt64 = 0;
            var asMaxTicket: UInt64 = 0;
            var gaTicketsMinted: UInt64 = 0;
            var asTicketsMinted: UInt64 = 0;

            for nft in tickets {
              var eventtier: VeraEvent.Tier = veraevent.getTier(tier: nft.tier)!
              maxTickets = eventtier.maxTickets
              if (nft.type == VeraTicket.NFTType.GeneralAdmission) {
                gaTicketsMinted = gaTicketsMinted + eventtier.ticketsMinted
                gaMaxTicket = gaMaxTicket + maxTickets
              } else if (nft.type == VeraTicket.NFTType.AssignedSeating) {
                asTicketsMinted = asTicketsMinted + eventtier.ticketsMinted
                asMaxTicket = asMaxTicket + maxTickets
              }
            }

            gaTicketsMinted = gaTicketsMinted + gatickets;
            asTicketsMinted = asTicketsMinted + astickets;

            if (gaTicketsMinted > gaMaxTicket || asTicketsMinted > asMaxTicket) {
                panic("Max Tickets Minted")
            }

            for nft in tickets {
                // deposit it in the recipient's account using their reference
			    recipient.deposit(token: <-create VeraTicket.NFT(initID: VeraTicket.totalSupply, initEventID: eventID, initType: nft.type, initTier: nft.tier, initSubTier: nft.subtier, initTokenURI: nft.tokenURI))
                emit Minted(id: VeraTicket.totalSupply)
                eventCollection.incrementTicketMinted(eventId: eventID, tier: nft.tier, subtier: nft.subtier)
                VeraTicket.totalSupply = VeraTicket.totalSupply + (1 as UInt64)
            }
	    }
    }

    // fetch
    // Get a reference to a Ticket from an account's Collection, if available.
    // If an account does not have a Tickets.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &VeraTicket.NFT? {
        let collection = getAccount(from)
            .getCapability(VeraTicket.VeraTicketPubStorage)
            .borrow<&VeraTicket.Collection{VeraTicket.TicketsCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Tickets.Collection.borowTicket to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowTicket(id: itemID)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.VeraTicketStorage = /storage/veraTicketCollection
        self.VeraTicketPubStorage = /public/veraTicketCollection
        self.VeraMinterStorage = /storage/veraTicketMinter

        // Initialize the total supply
        self.totalSupply = 0
        

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.VeraMinterStorage)

        emit ContractInitialized()
	}
}

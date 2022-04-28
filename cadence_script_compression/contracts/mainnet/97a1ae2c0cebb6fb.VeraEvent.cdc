// VeraEvent
// Events Contract!
//
pub contract VeraEvent {

    // Event
    //
    pub event ContractInitialized()
    pub event EventCreated(eventId: UInt64)
    pub event EventUpdated(eventId: UInt64)

    // Named Paths
    //
    pub let VeraEventStorage: StoragePath
    pub let VeraEventPubStorage: PublicPath
    pub let VeraAdminStorage: StoragePath

    // totalEvents
    // The total number of Events that have been created
    //
    pub var totalEvents: UInt64

    // Declare an enum named `Color` which has the raw value type `UInt8`,
    // and declare three enum cases: `red`, `green`, and `blue`
    //
    pub enum EventType: UInt8 {
        pub case Public
        pub case Private
    }

    // Declare an enum named `Color` which has the raw value type `UInt8`,
    // and declare three enum cases: `red`, `green`, and `blue`
    //
    pub enum TierType: UInt8 {
        pub case GeneralAdmission
        pub case AssignedSeating
    }

    // Declare an enum named `Color` which has the raw value type `UInt8`,
    // and declare three enum cases: `red`, `green`, and `blue`
    //
    pub enum RoyaltyType: UInt8 {
        pub case Fixed
        pub case Percent
    }

    // Royalty Struct
    // type can be Fixed or Percent
    pub struct Royalty {
        pub let id: UInt64
        pub var type: RoyaltyType
        pub var value: UInt64
        
        init(id: UInt64, type: RoyaltyType, value: UInt64) {
            self.id = id
            self.type = type
            self.value = value
        }
    }

    // AccountRoyalty Struct
    // type can be Fixed or Percent
    pub struct AccountRoyalty {
        pub let id: UInt64
        pub var account: Address
        pub var royaltyValue: UFix64
        
        init(id: UInt64, account: Address, royaltyValue: UFix64) {
            self.id = id
            self.account = account
            self.royaltyValue = royaltyValue
        }
    }

    pub struct SubTier {
        pub let id: UInt64
        pub var name: String
        pub var cost: UFix64
        pub let maxTickets: UInt64
        pub var ticketsMinted: UInt64
        init(id: UInt64, name: String, cost: UFix64, maxTickets: UInt64) {
            self.id = id
            self.name = name
            self.cost = cost
            self.maxTickets = maxTickets
            self.ticketsMinted = 0
        }

        pub fun incrementTicketMinted() {
            self.ticketsMinted = self.ticketsMinted + 1
        }

        pub fun decrementTicketMinted() {
            self.ticketsMinted = self.ticketsMinted - 1
        }
    }

    pub struct Tier {
        pub let id: UInt64
        pub let type: VeraEvent.TierType
        pub var name: String
        pub var cost: UFix64
        pub let maxTickets: UInt64
        pub var ticketsMinted: UInt64
        pub var subtier: {UInt64: VeraEvent.SubTier}
        
        init(id: UInt64, type: VeraEvent.TierType, name: String, cost: UFix64, maxTickets: UInt64, subtier:{UInt64: VeraEvent.SubTier}) {
            self.id = id
            self.type = type
            self.name = name
            self.cost = cost
            self.maxTickets = maxTickets
            self.ticketsMinted = 0
            self.subtier = subtier
        }

        pub fun incrementTicketMinted() {
            self.ticketsMinted = self.ticketsMinted + 1
        }

        pub fun decrementTicketMinted() {
            self.ticketsMinted = self.ticketsMinted - 1
        }
    }

    pub struct EventStruct {
        pub let id: UInt64
        pub var type: VeraEvent.EventType
        pub var tier: {UInt64: VeraEvent.Tier}
        pub var maxTickets: UInt64
        pub var buyLimit: UInt64
        pub var defaultRoyaltyPercent: UFix64
        pub var defaultRoyaltyAddress: Address
        pub var totalTicketsMinted: UInt64
        pub var eventURI: String;
        pub var royalty: Royalty
        pub var royalties: {UInt64: VeraEvent.AccountRoyalty}
        
        init(id: UInt64, type: VeraEvent.EventType, tier: {UInt64: VeraEvent.Tier}, maxTickets: UInt64, buyLimit: UInt64, defaultRoyaltyAddress: Address, defaultRoyaltyPercent: UFix64, royalty: Royalty, royalties: {UInt64: VeraEvent.AccountRoyalty}, eventURI: String) {
            self.id = id
            self.type = type
            self.tier = tier
            self.maxTickets = maxTickets
            self.buyLimit = buyLimit
            self.defaultRoyaltyAddress = defaultRoyaltyAddress
            self.defaultRoyaltyPercent = defaultRoyaltyPercent
            self.royalty = royalty
            self.royalties = royalties
            self.eventURI = eventURI
            self.totalTicketsMinted = 0
        }

        pub fun incrementTicketMinted(tier: UInt64, subtier: UInt64) {
            self.totalTicketsMinted = self.totalTicketsMinted + 1
        }

        pub fun decrementTicketMinted(tier: UInt64, subtier: UInt64) {
            self.totalTicketsMinted = self.totalTicketsMinted - 1
        }

        pub fun getTier(tier: UInt64): VeraEvent.Tier {
            let tier = self.tier[tier] ?? panic("missing Tier")
            return tier
        }

        pub fun getSubTier(tier: UInt64, subtier:UInt64): VeraEvent.SubTier {
            let tier = self.tier[tier] ?? panic("missing Tier")
            let subtier = tier.subtier[subtier] ?? panic("missing Sub Tier")
            return subtier
        }
    }

    pub resource EventCollection {
        pub var eventsCollection: {UInt64: VeraEvent.EventStruct}
        pub var metadataObjs: {UInt64: { String : String }}

        pub fun addEvent(veraevent: VeraEvent.EventStruct, metadata: {String : String}) {
            let eventId = veraevent.id;
            self.eventsCollection[eventId] = veraevent
            self.metadataObjs[eventId] = metadata
            emit EventCreated(eventId: eventId)
        }

        pub fun updateEvent(veraevent: VeraEvent.EventStruct, metadata: {String : String}) {
            let eventId = veraevent.id;
            self.eventsCollection[eventId] = veraevent
            self.metadataObjs[eventId] = metadata
            emit EventUpdated(eventId: eventId)
        }

        pub fun getEvent(eventId: UInt64): VeraEvent.EventStruct {
            let veraevent = self.eventsCollection[eventId] ?? panic("missing Event")
            return veraevent
        }

        pub fun getMetadata(eventId: UInt64): { String : String } {
            let metadata = self.metadataObjs[eventId] ?? panic("missing Metadata")
            return metadata
        }

        pub fun incrementTicketMinted(eventId : UInt64, tier: UInt64, subtier: UInt64) {
            let veraevent = self.eventsCollection[eventId] ?? panic("missing Event")
            self.eventsCollection[eventId]!.incrementTicketMinted(tier: tier, subtier: subtier)
            if let eventTier: VeraEvent.Tier = self.eventsCollection[eventId]!.tier[tier] {
                self.eventsCollection[eventId]!.tier[tier]!.incrementTicketMinted()
            }
            if let eventSubtier: VeraEvent.SubTier = self.eventsCollection[eventId]!.tier[tier]!.subtier[subtier] {
                self.eventsCollection[eventId]!.tier[tier]!.subtier[subtier]!.incrementTicketMinted()
            }
        }

        pub fun decrementTicketMinted(eventId : UInt64, tier: UInt64, subtier: UInt64) {
            let veraevent = self.eventsCollection[eventId] ?? panic("missing Event")
            self.eventsCollection[eventId]!.decrementTicketMinted(tier: tier, subtier: subtier)
            if let eventTier: VeraEvent.Tier = self.eventsCollection[eventId]!.tier[tier] {
                self.eventsCollection[eventId]!.tier[tier]!.decrementTicketMinted()
            }
            if let eventSubtier: VeraEvent.SubTier = self.eventsCollection[eventId]!.tier[tier]!.subtier[subtier] {
                self.eventsCollection[eventId]!.tier[tier]!.subtier[subtier]!.decrementTicketMinted()
            }
        }

        pub fun getIDs(): [UInt64] {
            return self.eventsCollection.keys
        }

        // initializer
        //
        init () {
            self.eventsCollection = {}
            self.metadataObjs = {}
        }
    }
    
    priv fun createEmptyCollection(): @VeraEvent.EventCollection {
        return <- create EventCollection()
    }

    pub resource EventAdmin {

        // createEvent
        // Create an Event
		//
		pub fun createEvent(type: VeraEvent.EventType, tier: {UInt64: VeraEvent.Tier}, maxTickets: UInt64, buyLimit: UInt64, defaultRoyaltyAddress: Address, defaultRoyaltyPercent: UFix64, royalty: Royalty, royalties: {UInt64: VeraEvent.AccountRoyalty}, eventURI: String, metadata: {String: String}) {
			// deposit it in the recipient's account using their reference
			let veraevent = VeraEvent.EventStruct(id: VeraEvent.totalEvents, type: type, tier: tier, maxTickets: maxTickets, buyLimit: buyLimit, defaultRoyaltyAddress:defaultRoyaltyAddress, defaultRoyaltyPercent:defaultRoyaltyPercent, royalty: royalty, royalties: royalties, eventURI: eventURI)
            let collection = VeraEvent.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
            collection.addEvent(veraevent: veraevent, metadata: metadata)
            VeraEvent.totalEvents = VeraEvent.totalEvents + (1 as UInt64)
		}

        // updateEvent
        // Update an Event
		//
        pub fun updateEvent(id:UInt64, type: VeraEvent.EventType, tier: {UInt64: VeraEvent.Tier}, maxTickets: UInt64, buyLimit: UInt64, defaultRoyaltyAddress: Address, defaultRoyaltyPercent: UFix64, royalty: Royalty, royalties: {UInt64: VeraEvent.AccountRoyalty}, eventURI: String, metadata: {String: String}) {
			// deposit it in the recipient's account using their reference
			let veraevent = VeraEvent.EventStruct(id: id, type: type, tier: tier, maxTickets: maxTickets, buyLimit: buyLimit, defaultRoyaltyAddress:defaultRoyaltyAddress, defaultRoyaltyPercent:defaultRoyaltyPercent, royalty: royalty, royalties: royalties, eventURI: eventURI)
            let collection = VeraEvent.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
            collection.updateEvent(veraevent: veraevent, metadata: metadata)
		}
	}

    // mintNFT
    // Mints a new NFT with a new ID
    // and deposit it in the recipients collection using their collection reference
    //
    pub fun getEvent(id:UInt64): VeraEvent.EventStruct {
        let collection = VeraEvent.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
        return collection.getEvent(eventId: id)
    }

    pub fun getMetadata(id:UInt64): { String : String} {
        let collection = VeraEvent.account.borrow<&VeraEvent.EventCollection>(from: VeraEvent.VeraEventStorage)!
        return collection.getMetadata(eventId: id)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.VeraEventStorage = /storage/veraeventCollection
        self.VeraEventPubStorage = /public/veraeventCollection
        self.VeraAdminStorage = /storage/veraEventdmin

        // Initialize the total events
        self.totalEvents = 0
        
        // Create a Minter resource and save it to storage
        let eventAdmin <- create EventAdmin()
        self.account.save(<-eventAdmin, to: self.VeraAdminStorage)

        // Create a Collection resource and save it to storage
        let eventCollection <- self.createEmptyCollection()
        self.account.save(<-eventCollection, to: self.VeraEventStorage)

        emit ContractInitialized()
	}
}

import NonFungibleToken from 0x1d7e57aa55817448

pub contract Moments: NonFungibleToken {
    // Standard Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // Admin-level Creator Events
    pub event CreatorRegistered(creator: Address)
    pub event CreatorRevoked(creator: Address)
    pub event CreatorReinstated(creator: Address)
    pub event CreatorAttributed(creator: Address, contentID: UInt64)
    pub event CreatorAttributionRemoved(creator: Address, contentID: UInt64)

    // Emitted on Creation
    pub event ContentCreated(contentID: UInt64, creator: Address)
    pub event SeriesCreated(seriesID: UInt64)
    pub event SetCreated(setID: UInt64) 
    pub event ContentAddedToSeries(contentID: UInt64, seriesID: UInt64)
    pub event ContentAddedToSet(contentID: UInt64, setID: UInt64, contentEditionID: UInt64)

    // Emitted on Updates
    pub event ContentUpdated(contentID: UInt64)

    pub event SetUpdated(setID: UInt64)
    pub event SeriesUpdated(seriesID: UInt64)

    // Emitted on Metadata destruction
    pub event ContentDestroyed(contentID: UInt64)
    
    // Emitted when a Set is retired
    pub event SetRetired(setID: UInt64)

    // Emitted when a Moment is minted
    pub event MomentMinted(momentID: UInt64, contentID: UInt64, contentEditionID: UInt64, 
                           serialNumber: UInt64, seriesID: UInt64, setID: UInt64)

    // Emitted when a Moment is destroyed
    pub event MomentDestroyed(momentID: UInt64)

    ///////
    // PATHS
    ///////

    // Moment Collection Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // CreatorProxy Receiver
    pub let CreatorProxyStoragePath: StoragePath
    pub let CreatorProxyPublicPath: PublicPath

    // Contract Owner ContentCreator Resource
    pub let ContentCreatorStoragePath: StoragePath
    pub let ContentCreatorPrivatePath: PrivatePath
    pub let ContentCreatorPublicPath: PublicPath

    // AdminProxy Receiver
    pub let AdminProxyStoragePath: StoragePath
    pub let AdminProxyPublicPath: PublicPath

    // Contract Owner Root Administrator Resource
    pub let AdministratorStoragePath: StoragePath
    pub let AdministratorPrivatePath: PrivatePath
    pub let RevokerPublicPath: PublicPath

    // totalSupply
    // The total number of Moments that have been minted
    //
    pub var totalSupply: UInt64

    // Content Metadata
    pub struct ContentMetadata {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        // descriptor of the source of the content came from
        pub let source: String
        // creator address
        pub let creator: Address
        // performer names and other credits
        pub let credits: {String: String}
        // uri to a preview image
        pub let previewImage: String
        // URI to content video
        pub let videoURI: String
        // IPFS storage hash
        pub let videoHash: String

        init(id: UInt64, name: String, description: String, source: String, creator: Address, credits: {String: String}, previewImage: String, videoURI: String, videoHash: String) {
            self.id = id
            self.name = name
            self.description = description
            self.source = source
            self.creator = creator
            self.credits = credits
            self.previewImage = previewImage
            self.videoURI = videoURI
            self.videoHash = videoHash
        }
    }

    // Edition Metadata
    // any given Edition is the Moment-run contained in a Set
    pub struct EditionMetadata {
        pub let id: UInt64
        pub let contentID: UInt64
        pub let rarity: String
        pub(set) var momentIDs: [UInt64]
        
        init(id: UInt64, contentID: UInt64, rarity: String) {
            self.id = id
            self.contentID = contentID
            self.rarity = rarity
            self.momentIDs = []
        }
    }
    
    // Series Metadata
    // 
    pub struct SeriesMetadata {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let art: String?

        // content ids in the series
        pub(set) var contentIDs: [UInt64]

        init(id: UInt64, name: String, description: String, art: String?) {
            pre {
                name.length > 0: "New Series name cannot be empty"
            }
            self.id = id
            self.name = name
            self.contentIDs = []
            self.description = description
            self.art = art
        }
    }
    // Set Metadata
    // 
    pub struct SetMetadata {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let art: String?
        pub let rarityCaps: {String: UInt64}

        // map of contentIDs to their edition details used in this set
        // - includes array of momentIDs that use each contentID
        pub(set) var contentEditions: {UInt64:EditionMetadata}

        // retired state of this set
        pub(set) var retired: Bool

        
        init(id: UInt64, name: String, description: String, rarityCaps: {String: UInt64}, art: String?) {
            pre {
                name.length > 0: "New Set name cannot be empty"
            }
            self.id = id
            self.name = name
            self.description = description
            self.art = art
            self.rarityCaps = rarityCaps
            self.contentEditions = {}
            self.retired = false
        }
    }


    // Moment Metadata
    // provides a wrapper to all the relevant data of a Moment
    pub struct MomentMetadata {
        // moment
        pub let id: UInt64
        pub let serialNumber: UInt64
        // content
        pub let contentID: UInt64
        pub let contentCreator: Address
        pub let contentCredits: {String: String}
        pub let contentName: String
        pub let contentDescription: String
        pub let previewImage: String
        pub let videoURI: String
        pub let videoHash: String
        // series
        pub let seriesID: UInt64
        pub let seriesName: String
        pub let seriesArt: String?
        pub let seriesDescription: String
        // set
        pub let setID: UInt64
        pub let setName: String
        pub let setArt: String?
        pub let setDescription: String
        pub let retired: Bool
        // contentedition
        pub let contentEditionID: UInt64
        pub let rarity: String
        pub let run: UInt64
        init(id: UInt64, serialNumber: UInt64, 
            contentID: UInt64, contentCreator: Address, 
            contentCredits: {String: String}, contentName: String, contentDescription: String, 
            previewImage: String, videoURI: String, videoHash: String,
            seriesID: UInt64, seriesName: String, seriesArt: String?, seriesDescription: String,
            setID: UInt64, setName: String, setArt: String?, setDescription: String, retired: Bool,
            contentEditionID: UInt64, rarity: String, run: UInt64) {
            // moments
            self.id = id
            self.serialNumber = serialNumber
            // content
            self.contentID = contentID
            self.contentCreator = contentCreator
            self.contentCredits = contentCredits
            self.contentName = contentName
            self.contentDescription = contentDescription
            self.previewImage = previewImage
            self.videoURI = videoURI
            self.videoHash = videoHash
            // series
            self.seriesID = seriesID
            self.seriesName = seriesName
            self.seriesArt = seriesArt
            self.seriesDescription = seriesDescription
            // set
            self.setID = setID
            self.setName = setName
            self.setArt = setArt
            self.setDescription = setDescription
            self.retired = retired
            // edition
            self.contentEditionID = contentEditionID
            self.rarity = rarity
            self.run = run
        }
    }

    // public creation for accounts to proxy from
    pub fun createCreatorProxy(): @CreatorProxy {
        return <- create CreatorProxy()
    }
    pub resource interface CreatorProxyPublic {
        pub fun empowerCreatorProxy(_ cap: Capability<&Moments.ContentCreator>)
    }
    pub resource CreatorProxy: CreatorProxyPublic {
        access(self) var powerOfCreation: Capability<&Moments.ContentCreator>?
        init () {
            self.powerOfCreation = nil
        }
        pub fun empowerCreatorProxy(_ cap: Capability<&Moments.ContentCreator>){ 
            pre {
                cap.check() : "Invalid ContentCreator capability"
                self.powerOfCreation == nil : "ContentCreator capability already set"
            }
            self.powerOfCreation = cap
            let creator = self.powerOfCreation!.borrow()!
            creator.registerCreator(creator: self.owner!.address)
        }

        // borrow a reference to the ContentCreator
        // 
        pub fun borrowContentCreator(): &Moments.ContentCreator {
            pre {
                self.powerOfCreation!.check() : "Your CreatorProxy has no capabilities."
            }
            let revoker = Moments.account.getCapability<&Moments.Administrator{Moments.Revoker}>(Moments.RevokerPublicPath).borrow()
                ?? panic("Can't find the revoker/admin!")

            if (revoker.revoked(address: self.owner!.address)) { panic("Creator privileges revoked") }
                
            let aPowerToBehold = self.powerOfCreation!.borrow()
                ?? panic("Your CreatorProxy has no capabilities.")
            
            return aPowerToBehold
        }
    }
    pub resource interface ContentCreatorPublic {
        // getters
        pub fun getMomentMetadata(momentID: UInt64): MomentMetadata
        pub fun getContentMetadata(contentID: UInt64): ContentMetadata
        pub fun getContentEditions(contentID: UInt64): [UInt64]
        pub fun getSetMetadata(setID: UInt64): SetMetadata
        pub fun getSeriesMetadata(seriesID: UInt64): SeriesMetadata
        pub fun getCreatorAttributions(address: Address): [UInt64]
        pub fun isSetRetired(setID:UInt64): Bool
    }

    pub resource ContentCreator: ContentCreatorPublic {
        // stores an address of the people that create content and the id's they created
        access(self) let creators: {Address: [UInt64]}

        // content tracks contentID's to their respective ContentMetadata
        access(self) let content: {UInt64: ContentMetadata}

        // seriesMetadata tracks seriesID's to their respective SeriesMetadata
        access(self) let series: {UInt64: SeriesMetadata}
        
        // sets tracks setID's to their respective SetMetadata
        access(self) let sets: {UInt64: SetMetadata}

        // editions tracks the setIDs in which a given piece of Content was minted
        access(self) let editions: {UInt64: [UInt64]}
        
        // moments tracks momentID's to their respective MomentMetadata
        //  -- MomentMetadata is the aggregate of {Series:Set:Content}
        access(self) let moments: {UInt64: MomentMetadata}

        // protected incrementors for new things
        access(contract) var newContentID: UInt64
        access(contract) var newSetID: UInt64
        access(contract) var newSeriesID: UInt64
        access(contract) var newEditionID: UInt64

        init() {
            self.creators = {}
            self.content = {}
            self.editions = {}
            self.series = {}
            self.sets = {}
            self.moments = {}

            self.newContentID = 1
            self.newSetID = 1
            self.newSeriesID = 1
            self.newEditionID = 1
        }

        ///////
        // CREATION FUNCTIONS -> THESE ADD TO ROOT MOMENTS DATA
        //// These are the only mechanisms to create what Content can be minted from and how.
        ///////
        // createContent
        // 
        pub fun createContent(content: ContentMetadata, creator: &CreatorProxy): UInt64 {
            pre {
                self.creators[creator.owner!.address] != nil : "That creator has not been registered"
                creator.borrowContentCreator() != nil : "That proxy is invalid"
                !self.isCreatorRevoked(creator: creator.owner!.address) : "That creator's proxy has been revoked"
            }
            // store creator attribution
            let address = creator.owner!.address
            // create the new contentMetadata at the new ID
            let newID = self.newContentID
            self.creators[address]!.append(newID)
            // enforce orderly id's by recreating contentMetadata here            
            let newContentMetadata = ContentMetadata(id:newID, 
                name: content.name, 
                description: content.description,
                source: content.source,
                creator: address,
                credits: content.credits,
                previewImage: content.previewImage,
                videoURI: content.videoURI,
                videoHash: content.videoHash)
            
            // increment and emit before giving back the new ID
            self.newContentID = self.newContentID + (1 as UInt64)

            self.editions[newID] = []
            self.content[newID] = newContentMetadata

            emit ContentCreated(contentID: newID, creator: address)
            return newID
        }
        // createSeries
        //
        pub fun createSeries(name: String, description: String, art: String?): UInt64 {
            // create the new metadata at the new ID
            let newID = self.newSeriesID
            self.series[newID] = SeriesMetadata(id: newID, name: name, description: description, art: art)

            // increment and emit before giving back the new ID
            self.newSeriesID = self.newSeriesID + (1 as UInt64)
            emit SeriesCreated(seriesID: newID)
            return newID
        }
        // createSet
        //
        pub fun createSet(name: String, description: String, art: String?, rarityCaps: {String: UInt64}): UInt64 {
            // create the new setMetadata at the new ID
            let newID = self.newSetID
            self.sets[newID] = SetMetadata(id: newID, name: name, description: description, rarityCaps: rarityCaps, art: art)

            // increment and emit before giving back the new ID
            self.newSetID = self.newSetID + (1 as UInt64)
            emit SetCreated(setID: newID)
            return newID
        }
        ////// CONTENT ADDERS //////
        // Content must be represented within a given Series and Set before
        // it can be minted into a Moment
        ////////////////////////////

        // addContentToSeries
        // - Adds a given ContentID to a given series
        //
        pub fun addContentToSeries(contentID: UInt64, seriesID: UInt64) {
            pre {
                self.content[contentID] != nil: "Cannot add the Content to Series: Content doesn't exist"
                self.series[seriesID] != nil: "Cannot mint Moment from this Series: the Series does not exist"    
                !self.series[seriesID]!.contentIDs.contains(contentID) : "That ContentID is already a part of that Series"
            }
            // add this content to the Series specified by the caller
            self.series[seriesID]!.contentIDs.append(contentID)
            
            emit ContentAddedToSeries(contentID: contentID, seriesID: seriesID)
        }

        pub fun addContentToSet(contentID: UInt64, setID: UInt64, rarity: String) {
            pre {
                self.content[contentID] != nil: "Cannot add the ContentEdition to Set: Content doesn't exist"
                self.sets[setID] != nil: "Cannot add ContentEdition to Set: Set doesn't exist"
                self.sets[setID]!.contentEditions[contentID] == nil : "That ContentID is already a part of that Set"
                self.editions[contentID] != nil : "That edition already contains that ContentID"
                !self.sets[setID]!.retired: "Cannot add ContentEdition to Set after it has been Retired"
            }
            // establish the back-map from content to set, for finding peer content easily
            self.editions[contentID]!.append(setID)
            let editionID = self.newEditionID + (1 as UInt64)

            // add this content to the set to be minted from as a Moment
            let set = self.sets[setID]!
            assert(set.rarityCaps.keys.contains(rarity), message: "That Rarity is invalid")
            set.contentEditions[contentID] = EditionMetadata(id: editionID, contentID: contentID, rarity: rarity)
            
            // update state
            self.newEditionID = editionID
            self.sets[setID] = set

            emit ContentAddedToSet(contentID: contentID, setID: setID, contentEditionID: editionID)
        }

        ////// MINT //////

        // mintMoment
        //
        pub fun mintMoment(contentID: UInt64, seriesID: UInt64, setID: UInt64): @NFT {
            pre {
                self.content[contentID] != nil: "Cannot mint Moment: This Content doesn't exist."
                self.sets[setID] != nil: "Cannot mint Moment from this Set: This Set does not exist."
                self.sets[setID]!.contentEditions[contentID] != nil : "Cannot mint from this Set: it has no Edition of that Content to Mint from"
                !self.sets[setID]!.retired: "Cannot add ContentEdition to Set after it has been Retired"
                self.series[seriesID] != nil: "Cannot mint Moment from this Series: the Series does not exist"    
                self.series[seriesID]!.contentIDs.contains(contentID) : "Cannot mint Moment from this Series: the Series does not contain this Content"
            }
            // get the set from which this is being minted
            let set = self.sets[setID]!
            // get the edition of this content from the set
            let contentEdition = set.contentEditions[contentID]!
            let serialNumber = UInt64(contentEdition.momentIDs.length + 1)

            // check that we're not blowing the minting cap for this edition
            assert(serialNumber <= set.rarityCaps[contentEdition.rarity]!, message: "The cap for that rarity has already been minted for that Moment.")

            // Mint the new moment
            let newMoment: @NFT <- create NFT(contentID: contentID,
                                              contentEditionID: contentEdition.id,
                                              serialNumber: serialNumber,
                                              seriesID: seriesID,
                                              setID: setID)
            
            // add this moment to that content edition
            contentEdition.momentIDs.append(newMoment.id)
            set.contentEditions[contentID] = contentEdition
            
            // replace this set with the updated version that knows this was just minted
            self.sets[setID] = set

            let moment = self.makeMomentMetadata(
                id: newMoment.id, 
                contentID: contentID,
                contentEditionID: newMoment.contentEditionID,
                serialNumber: newMoment.serialNumber,
                seriesID: seriesID,
                setID: setID)
            
            // store a metadata copy to lookup later
            self.moments[newMoment.id] = moment
            return <- newMoment
        }

        // batchMintMoment
        //
        pub fun batchMintMoment(contentID: UInt64, seriesID: UInt64, setID: UInt64, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintMoment(contentID: contentID, seriesID: seriesID, setID: setID))
                i = i + (1 as UInt64)
            }

            return <- newCollection
        }

        ////// UPDATE & RETIRE //////

        // updateContentMetadata
        // NOTE: we allow the content to be updated in case host changes, or credits change etc.
        // PRE: must exist, thus cannot bypass creation route
        // 
        pub fun updateContentMetadata(content: ContentMetadata, creatorProxy: &CreatorProxy) {     
            pre {
                self.content[content.id] != nil: "Cannot update that Content, it doesn't exist!"
                creatorProxy.borrowContentCreator() != nil : "That Proxy is invalid"
                content.creator == creatorProxy.owner!.address : "The creator has not authorized this update"
            }
            self.content[content.id] = content

            emit ContentUpdated(contentID: content.id)
        }

        // retireSet
        //
        pub fun retireSet(setID: UInt64) {
            pre {
                self.sets[setID] != nil: "Cannot retire that Set, it doesn't exist!"
                !self.sets[setID]!.retired : "Cannot retire that Set, it already is retired!"
            }
            let set = self.sets[setID]!
            set.retired = true
            self.sets[setID] = set
            emit SetRetired(setID: setID)
        }

        ////// ADMIN FUNCS //////

        // isCreatorRevoked
        // contract-level getter of revoke for code sugar
        //
        access(contract) fun isCreatorRevoked(creator: Address): Bool {
            pre {
               self.creators[creator] != nil : "That creator is not yet registered" 
            }
            let revoker = Moments.account.getCapability<&Moments.Administrator{Moments.Revoker}>(Moments.RevokerPublicPath).borrow()
                ?? panic("Can't find the revoker/admin!")

            return revoker.revoked(address: creator)
        }

        // registerCreator
        //
        access(contract) fun registerCreator(creator: Address) {
            pre {
                self.creators[creator] == nil : "That creator is already registered"
            }
            self.creators[creator] = []
            emit CreatorRegistered(creator: creator)
        }
        // addCreatorAttribution
        //
        access(contract) fun addCreatorAttribution(contentID: UInt64, creator: Address) {
            pre {
                self.creators[creator] != nil : "That creator is unregistered"
                self.content[contentID] != nil : "That content does not exist"
                !self.isCreatorRevoked(creator: creator) : "That creator's proxy has been revoked"
                !self.creators[creator]!.contains(contentID) : "That creator is already attributed to that content"
            }
            self.creators[creator]!.append(contentID)
            emit CreatorAttributed(creator: creator, contentID: contentID)
        }

        // removeCreatorAttribution
        //
        access(contract) fun removeCreatorAttribution(contentID: UInt64, creator: Address) {
            pre {
                self.creators[creator] != nil : "That creator is unregistered"
                self.content[contentID] != nil : "That content does not exist"
                self.creators[creator]!.contains(contentID) : "That creator is not attributed to that content"
            }
            var findIndex:UInt64 = 0
            for id in self.creators[creator]! {
                if id == contentID {
                    break;
                }
                findIndex = findIndex + 1 as UInt64
            }
            let removed = self.creators[creator]!.remove(at: findIndex)
            assert(removed == contentID, message: "The wrong content has been removed")
            emit CreatorAttributionRemoved(creator: creator, contentID: removed)
        }

        /////
        // ADMIN FORCE UPDATE FUNCS
        /////

        // admin route to forcibly update contentMetadata
        //
        access(contract) fun forceUpdateContentMetadata(content: ContentMetadata) {     
            pre {
                self.content[content.id] != nil: "Cannot update that Content, it doesn't exist!"
            }
            self.content[content.id] = content

            emit ContentUpdated(contentID: content.id)
        }
        // admin-level contract-bound updateSet call
        // - this should rarely ever happen, require admin-approval
        //
        access(contract) fun forceUpdateSetMetadata(set: SetMetadata) {     
            pre {
                self.sets[set.id] != nil: "Cannot update that Content, it doesn't exist!"
            }
            self.sets[set.id] = set

            emit SetUpdated(setID: set.id)
        }
        // admin-level contract-bound updateSet call
        // - this should rarely ever happen, require admin-approval
        //
        access(contract) fun forceUpdateSeriesMetadata(series: SeriesMetadata) {     
            pre {
                self.series[series.id] != nil: "Cannot update that Content, it doesn't exist!"
            }
            self.series[series.id] = series

            emit SeriesUpdated(seriesID: series.id)
        }

        //////// GETTERS AND SUGAR ////////
        // makeMomentMetadata
        // simple sugar around grabbing hydrated metadata
        // -- arguably defeats the purpose of STORING metadata this way at all
        // -- TODO : potential tech debt: remove need for storage of MomentMetadata, 
        // -- -- -- -- fetch hydrated info per-getMomentsMetadata/caller
        //
        access(contract) fun makeMomentMetadata(id: UInt64, contentID: UInt64, contentEditionID: UInt64, serialNumber: UInt64, seriesID: UInt64, setID: UInt64): MomentMetadata {
            pre {
                self.content[contentID] != nil : "That content does not exist"
                self.series[seriesID] != nil : "That series does not exist"
                self.sets[setID] != nil : "That set does not exist"
                self.sets[setID]!.contentEditions[contentID] != nil : "That content is not a part of that set"
            }
            let content = self.content[contentID]!
            let series = self.series[seriesID]!
            let set = self.sets[setID]!
            let moment = MomentMetadata(
                id: id,
                serialNumber: serialNumber,
                contentID: contentID,
                contentCreator: content.creator,
                contentCredits: content.credits,
                contentName: content.name,
                contentDescription: content.description,
                previewImage: content.previewImage,
                videoURI: content.videoURI,
                videoHash: content.videoHash,
                seriesID: seriesID,
                seriesName: series.name,
                seriesArt: series.art,
                seriesDescription: series.description,
                setID: setID,
                setName: set.name,
                setArt: set.art,
                setDescription: set.description,
                retired: false,
                contentEditionID: set.contentEditions[contentID]!.id,
                rarity: set.contentEditions[contentID]!.rarity,
                run: UInt64(set.contentEditions[contentID]!.momentIDs.length))
            
            return moment
        }
        // getMomentMetadata
        // - Always hydrates on pull
        //
        pub fun getMomentMetadata(momentID: UInt64): MomentMetadata {
            pre {
                self.moments[momentID] != nil : "That momentID has no metadata associated with it"
            }
            let lastFetch = self.moments[momentID]!
            let freshMoment = self.makeMomentMetadata(
                id: momentID,
                contentID: lastFetch.contentID, 
                contentEditionID: lastFetch.contentEditionID, 
                serialNumber: lastFetch.serialNumber,
                seriesID: lastFetch.seriesID,
                setID: lastFetch.setID)

            return freshMoment
        }
        pub fun getContentMetadata(contentID: UInt64): ContentMetadata {
            pre {
                self.content[contentID] != nil : "That contentID has no metadata associated with it"
            }
            return self.content[contentID]!
        }
        // responds with an array of SetID's that content can be found
        pub fun getContentEditions(contentID: UInt64): [UInt64] {
            pre {
                self.editions[contentID] != nil : "There are no editions of that content"
            }
            return self.editions[contentID]!
        }
        pub fun getSetMetadata(setID: UInt64): SetMetadata {
            pre {
                self.sets[setID] != nil : "That setID has no metadata associated with it"
            }
            return self.sets[setID]!
        }
        pub fun getSeriesMetadata(seriesID: UInt64): SeriesMetadata {
            pre {
                self.series[seriesID] != nil : "That seriesID has no metadata associated with it"
            }
            return self.series[seriesID]!
        }
        pub fun getCreatorAttributions(address: Address): [UInt64] {
            pre {
                self.creators[address] != nil : "That creator has never been registered"
            }
            return self.creators[address]!
        }
        pub fun isSetRetired(setID:UInt64): Bool {
            pre {
                self.sets[setID] != nil : "That set does not exist"
            }
            return self.sets[setID]!.retired
        }
    }

    // NFT
    // Moment
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID per NFT standard
        pub let id: UInt64
        // The place in the Edition that this Moment was minted
        pub let serialNumber: UInt64
        // The ID of the Content that the Moment references
        pub let contentID: UInt64
        // the edition of the content this Moment references
        pub let contentEditionID: UInt64
        // The ID of the Series that the Moment comes from
        pub let seriesID: UInt64 
        // The ID of the Set that the Moment comes from
        pub let setID: UInt64

        init(contentID: UInt64, contentEditionID: UInt64, serialNumber: UInt64, seriesID: UInt64, setID: UInt64, ) {
            Moments.totalSupply = Moments.totalSupply + (1 as UInt64)
            self.id = Moments.totalSupply
            self.contentID = contentID
            self.contentEditionID = contentEditionID
            self.serialNumber = serialNumber
            self.seriesID = seriesID
            self.setID = setID

            emit MomentMinted(momentID: self.id, contentID: self.contentID, contentEditionID: self.contentEditionID, serialNumber: self.serialNumber, seriesID: self.seriesID, setID: self.setID)
        }

        // sugar func
        pub fun getMetadata(): MomentMetadata {
            return Moments.getContentCreator().getMomentMetadata(momentID: self.id)
        }

        destroy() {
            emit MomentDestroyed(momentID: self.id)
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMoment(id: UInt64): &Moments.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Moments reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Moments NFTs owned by an account
    //
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
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
            let token <- token as! @Moments.NFT

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

        // borrowMoment
        // Gets a reference to an NFT in the collection as a Moments.NFT,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the Moments.
        //
        pub fun borrowMoment(id: UInt64): &Moments.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Moments.NFT
            } else {
                return nil
            }
        }

        // destructor
        //
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

    // AdminUsers will create a Proxy and be granted
    // access to the Administrator resource through their receiver, which
    // they can then borrowSudo() to utilize
    //
    pub fun createAdminProxy(): @AdminProxy { 
        return <- create AdminProxy()
    }

    // public receiver for the Administrator capability
    //
    pub resource interface AdminProxyPublic {
        pub fun addCapability(_ cap: Capability<&Moments.Administrator>)
    }

    /// AdminProxy
    /// This is a simple receiver for the Administrator resource, which
    /// can be borrowed if capability has been established.
    ///
    pub resource AdminProxy: AdminProxyPublic {
        // requisite receiver of Administrator capability
        access(self) var sudo: Capability<&Moments.Administrator>?
        
        // initializer
        //
        init () {
            self.sudo = nil
        }

        // must receive capability to take administrator actions
        //
        pub fun addCapability(_ cap: Capability<&Moments.Administrator>){ 
            pre {
                cap.check() : "Invalid Administrator capability"
                self.sudo == nil : "Administrator capability already set"
            }
            self.sudo = cap
        }

        // borrow a reference to the Administrator
        // 
        pub fun borrowSudo(): &Moments.Administrator {
            pre {
                self.sudo != nil : "Your AdminProxy has no Administrator capabilities."
            }
            let sudoReference = self.sudo!.borrow()
                ?? panic("Your AdminProxy has no Administrator capabilities.")

            return sudoReference
        }
    }

    pub resource interface Revoker {
        pub fun revoked(address: Address): Bool
    }

    /// Administrator
    /// Deployer-owned resource that Privately grants Capabilities to Proxies
    pub resource Administrator: Revoker {
        access(self) let creators: {Address: Bool}

        init () {
            self.creators = {}
        }

        // registerCreator
        //   - registers a new account's CC Proxy
        //   - does not allow re-registration
        //
        pub fun registerCreator(address: Address, cc: Capability<&Moments.ContentCreator>) { 
            pre {
                getAccount(address).getCapability<&Moments.CreatorProxy{Moments.CreatorProxyPublic}>(Moments.CreatorProxyPublicPath).check() : "Creator account does not have a valid Proxy"
                self.creators[address] == nil : "That creator has already been registered"
                cc.check(): "that contentcreator is invalid"
            }
            let pCap = getAccount(address).getCapability<&Moments.CreatorProxy{Moments.CreatorProxyPublic}>(Moments.CreatorProxyPublicPath)
            let proxy = pCap.borrow() ?? panic("failed to borrow the creator's proxy")
            self.creators[address] = true // don't break the rules
            // this will register the proxy with the ContentCreator
            // and that will emit the registration event
            proxy.empowerCreatorProxy(cc)
        }

        // revokeProxy
        //  - revoke's a proxy's ability to borrow its CC cap
        //
        pub fun revokeCreator(address: Address) {
            pre {
                self.creators[address] != nil : "That creator has never been registered"
                self.creators[address]! : "That creator has already been revoked"
            }
            self.creators[address] = false

            emit CreatorRevoked(creator: address)
        }

        // reinstateCreator
        //  - hopefully this never needs to be called :)
        //
        pub fun reinstateCreator(address: Address) {
            pre {
                self.creators[address] != nil : "That creator has never been registered"
                !self.creators[address]! : "That creator has already been reinstated"
            }
            self.creators[address] = true
            
            emit CreatorReinstated(creator: address)
        }

        // revoked
        //  - yes means no!
        //
        pub fun revoked(address: Address): Bool {
            pre {
                self.creators[address] != nil : "That creator has never been registered"
            }
            
            return !self.creators[address]!
        }

        // addCreatorAttribution
        //   - specify what address was involved in creating a given piece of content 
        //
        pub fun addCreatorAttribution(creator: Address, contentID: UInt64) {
            pre {
                self.creators[creator] != nil: "That creator has never been registered"
            }
            let cc = Moments.account.borrow<&Moments.ContentCreator>(from: Moments.ContentCreatorStoragePath)
                ?? panic("NO CONTENT CREATOR! What'd you doooo")
            cc.addCreatorAttribution(contentID: contentID, creator: creator)
        }

        // removeCreatorAttribution
        //   - specify what address was involved in creating a given piece of content 
        //
        pub fun removeCreatorAttribution(creator: Address, contentID: UInt64) {
            pre {
                self.creators[creator] != nil: "That creator has never been registered"
            }
            let cc = Moments.account.borrow<&Moments.ContentCreator>(from: Moments.ContentCreatorStoragePath)
                ?? panic("NO CONTENT CREATOR! What'd you doooo")
            cc.removeCreatorAttribution(contentID: contentID, creator: creator)
        }

        ////
        // FORCE UPDATE - Admin is the overlord of all metadata!
        ////

        // updateContentMetadata
        //
        pub fun updateContentMetadata(content: ContentMetadata) {
            let cc = Moments.account.borrow<&Moments.ContentCreator>(from: Moments.ContentCreatorStoragePath)
                ?? panic("NO CONTENT CREATOR! What'd you doooo")
            cc.forceUpdateContentMetadata(content: content)
        }

        // updateSetMetadata
        //
        pub fun updateSetMetadata(set: SetMetadata) {
            let cc = Moments.account.borrow<&Moments.ContentCreator>(from: Moments.ContentCreatorStoragePath)
                ?? panic("NO CONTENT CREATOR! What'd you doooo")
            cc.forceUpdateSetMetadata(set: set)
        }

        // updateSeriesMetadata
        //
        pub fun updateSeriesMetadata(series: SeriesMetadata) {
            let cc = Moments.account.borrow<&Moments.ContentCreator>(from: Moments.ContentCreatorStoragePath)
                ?? panic("NO CONTENT CREATOR! What'd you doooo")
            cc.forceUpdateSeriesMetadata(series: series)
        }
    }

    // fetch
    // Get a reference to a Moments from an account's Collection, if available.
    // If an account does not have a Moments.Collection, panic.
    // If it has a collection but does not contain the momentID, return nil.
    // If it has a collection and that collection contains the momentID, return a reference to that.
    //
    pub fun fetch(_ from: Address, momentID: UInt64): &Moments.NFT? {
        let collection = getAccount(from)
            .getCapability<&Moments.Collection{Moments.CollectionPublic}>(Moments.CollectionPublicPath)
            .borrow() ?? panic("Couldn't get collection")
        // We trust Moments.Collection.borrowMoment to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowMoment(id: momentID)
    }

    // getContentCreator
    //   - easy route to the CC public path
    //
    pub fun getContentCreator(): &{Moments.ContentCreatorPublic} {
        let publicContent = Moments.account.getCapability<&Moments.ContentCreator{Moments.ContentCreatorPublic}>(Moments.ContentCreatorPublicPath).borrow() 
            ?? panic("Could not get the public content from the contract")
        return publicContent
    }

    // initializer
    //
    init() {
        self.CollectionStoragePath = /storage/jambbMomentsCollection
        self.CollectionPublicPath = /public/jambbMomentsCollection
		
        // only one content creator should ever exist, in the deployer storage
        self.ContentCreatorStoragePath = /storage/jambbMomentsContentCreator
        self.ContentCreatorPrivatePath = /private/jambbMomentsContentCreator
        self.ContentCreatorPublicPath = /public/jambbMomentsContentCreator

        // users can be proxied in and allowed to create content
        self.CreatorProxyStoragePath = /storage/jambbMomentsCreatorProxy
        self.CreatorProxyPublicPath = /public/jambbMomentsCreatorProxy
        
        // only one Administrator should ever exist, in deployer storage
        self.AdministratorStoragePath = /storage/jambbMomentsAdministrator
        self.AdministratorPrivatePath = /private/jambbMomentsAdministrator
        self.RevokerPublicPath = /public/jambbMomentsAdministrator

        self.AdminProxyStoragePath = /storage/jambbMomentsMomentsAdminProxy
        self.AdminProxyPublicPath = /public/jambbMomentsAdminProxy

        // Initialize the total supply
        self.totalSupply = 0

        // Create a NFTAdministrator resource and save it to storage
        let admin <- create Administrator()
        self.account.save(<- admin, to: self.AdministratorStoragePath)
        // Link it to provide shareable access route to capabilities
        self.account.link<&Moments.Administrator>(self.AdministratorPrivatePath, target: self.AdministratorStoragePath)
        self.account.link<&Moments.Administrator{Moments.Revoker}>(self.RevokerPublicPath, target: self.AdministratorStoragePath)
        

        // Create a ContentCreator resource and save it to storage
        let cc <- create ContentCreator()
        self.account.save(<- cc, to: self.ContentCreatorStoragePath)
        // Link it to provide shareable access route to capabilities
        self.account.link<&Moments.ContentCreator>(self.ContentCreatorPrivatePath, target: self.ContentCreatorStoragePath)
        self.account.link<&Moments.ContentCreator{Moments.ContentCreatorPublic}>(self.ContentCreatorPublicPath, target: self.ContentCreatorStoragePath)

        // create a personal collection just in case contract ever holds Moments to distribute later etc
        let collection <- create Collection()
        self.account.save(<- collection, to: self.CollectionStoragePath)
        self.account.link<&Moments.Collection{Moments.CollectionPublic, NonFungibleToken.Receiver}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        
        emit ContractInitialized()
    }
}
 

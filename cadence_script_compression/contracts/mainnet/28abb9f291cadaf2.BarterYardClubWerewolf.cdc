import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract BarterYardClubWerewolf: NonFungibleToken {

    /// Counter for all the minted Werewolves
    pub var totalSupply: UInt64

    /// Maximum Werewolf NFT supply that will ever exist
    pub let maxSupply: UInt64

    pub event ContractInitialized()
    pub event Mint(id: UInt64)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Burn(id: UInt64)
    /// WerewolfTransformed: Event sent when werewolves in the pack need to transform for better intoperability
    pub event WerewolfTransformed(ids: [UInt64], imageType: UInt8)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminStoragePath: StoragePath
    pub let AdminPrivatePath: PrivatePath

    /// traits contains all single NFT Traits as struct to keep track of supply
    access(self) let traits: {UInt8: Trait}
    /// traitTypes all NFT Trait Types
    access(self) let traitTypes: {Int8: String}

    /// timezonedWerewolves stores werewolves by timezone with a boolean value to tell if auto thumbnail is active or not
    access(self) let timezonedWerewolves: {Int8: {UInt64: Bool}}

    /// lastSyncedTimestamp stores the last timestamp where computeTimezoneForWerewolves was called
    pub var lastSyncedTimestamp: Int

    pub struct interface SupplyManager {
        pub var totalSupply: UInt16
        pub fun increment() {
            post {
                self.totalSupply == before(self.totalSupply) + 1:
                    "[SupplyManager](increment): totalSupply should be incremented"
            }
        }
        pub fun decrement() {
            pre {
                self.totalSupply > 0:
                    "[SupplyManager](decrement): totalSupply must be positive in order to decrement"
            }
            post {
                self.totalSupply == before(self.totalSupply) - 1:
                    "[SupplyManager](decrement): totalSupply should be decremented"
            }
        }
    }

    /**********/
    /* TRAITS */
    /**********/

    /// NftTrait is the trait struct meant to be set into NFTs
    pub struct NftTrait {
        pub let id: UInt8
        /// traitType refers to a type of trait (background / clothes / accessory etc...) stored in traitTypes map
        pub let traitType: Int8
        /// value gives the value for the traitType
        pub let value: String

        init(
            id: UInt8,
            traitType: Int8,
            value: String,
        ) {
            self.id = id
            self.traitType = traitType
            self.value = value
        }
    }

    /// PublicTrait is the trait struct meant to be exposed in this contract with totalSupply
    pub struct PublicTrait {
        pub let id: UInt8
        pub let traitType: Int8
        pub let value: String
        pub let totalSupply: UInt16

        init(
            id: UInt8,
            traitType: Int8,
            value: String,
            totalSupply: UInt16
        ) {
            self.id = id
            self.traitType = traitType
            self.value = value
            self.totalSupply = totalSupply
        }
    }

    /// Trait is the trait strcut used inside this contract to manage trait supply and return the different views
    pub struct Trait: SupplyManager {
        pub let id: UInt8
        pub let traitType: Int8
        pub let value: String

        pub var totalSupply: UInt16

        pub fun increment() {
            self.totalSupply = self.totalSupply + 1
        }

        pub fun decrement() {
            self.totalSupply = self.totalSupply - 1
        }

        pub fun toPublic(): BarterYardClubWerewolf.PublicTrait {
            return BarterYardClubWerewolf.PublicTrait(
                id: self.id,
                traitType: self.traitType,
                value: self.value,
                totalSupply: self.totalSupply
            )
        }

        pub fun toNft(): BarterYardClubWerewolf.NftTrait {
            return BarterYardClubWerewolf.NftTrait(
                id: self.id,
                traitType: self.traitType,
                value: self.value,
            )
        }
        init(
            id: UInt8,
            traitType: Int8,
            value: String,
        ) {
            self.id = id
            self.traitType = traitType
            self.value = value

            self.totalSupply = 0
        }
    }

    /*******/
    /* NFT */
    /*******/

    /// ImageType is an enum to manage the different types of image the NFT holds
    pub enum ImageType: UInt8 {
        pub case Day
        pub case Night
        pub case Animated
    }

    /// Werewolf interface
    pub resource interface Werewolf {
        /*******************/
        /* NFT information */
        /*******************/

        pub let id: UInt64 // eg: 1
        pub let name: String // eg: Werewolf #1
        pub let description: String // eg: Barter Yard Club membership

        /// Metadata attributes with public traits view
        access(contract) let attributes: [NftTrait; 8]

        /********************/
        /* Image management */
        /********************/

        /// All images IPFS files
        pub let dayImage: MetadataViews.IPFSFile
        pub let nightImage: MetadataViews.IPFSFile
        pub let animation: MetadataViews.IPFSFile

        /// gmt: can be set by the NFT owner to compute day / night according to his timezone
        pub var utc: Int8
        pub fun setUTC(utc: Int8)
        /// auto: if set to true, let the system compute which NFT to return
        pub var auto: Bool
        pub fun setAuto(auto: Bool)
        /// default: allow the user to set a default image if auto is turned off
        pub var defaultImage: ImageType
        pub fun setDefaultImage(imageType: ImageType)

        /// getThumbnail returns the current image to display depending on user preferences
        pub fun getThumbnail(): MetadataViews.IPFSFile
    }

    pub struct CompleteDisplay {
        pub let name: String
        pub let description: String
        access(contract) let attributes: [NftTrait; 8]
        pub let thumbnail: MetadataViews.IPFSFile
        pub let dayImage: MetadataViews.IPFSFile
        pub let nightImage: MetadataViews.IPFSFile
        pub let animation: MetadataViews.IPFSFile

        pub fun getAttributes(): [NftTrait; 8] {
            return self.attributes
        }

        init(
            name: String,
            description: String,
            attributes: [NftTrait; 8],
            thumbnail: MetadataViews.IPFSFile,
            dayImage: MetadataViews.IPFSFile,
            nightImage: MetadataViews.IPFSFile,
            animation: MetadataViews.IPFSFile,
        ) {
            self.name = name
            self.description = description
            self.attributes = attributes
            self.thumbnail = thumbnail
            self.dayImage = dayImage
            self.nightImage = nightImage
            self.animation = animation
        }
    }

    pub struct NFTConfig {
        pub let utc: Int8
        pub let auto: Bool
        pub let defaultImage: ImageType

        init(
            utc: Int8,
            auto: Bool,
            defaultImage: ImageType,
        ) {
            self.utc = utc
            self.auto = auto
            self.defaultImage = defaultImage
        }
    }

    pub resource NFT: NonFungibleToken.INFT, Werewolf, MetadataViews.Resolver {
        /// Werewolf
        pub let id: UInt64
        pub let name: String
        pub let description: String
        access(contract) let attributes: [NftTrait; 8]

        pub let dayImage: MetadataViews.IPFSFile
        pub let nightImage: MetadataViews.IPFSFile
        pub let animation: MetadataViews.IPFSFile
        pub var utc: Int8
        pub var auto: Bool
        pub var defaultImage: ImageType
        /// Computed thumbnail
        pub var thumbnail: MetadataViews.IPFSFile

        init(
            id: UInt64,
            name: String,
            description: String,
            attributes: [NftTrait; 8],
            dayImage: MetadataViews.IPFSFile,
            nightImage: MetadataViews.IPFSFile,
            animation: MetadataViews.IPFSFile,
        ) {
            self.id = id
            self.name = name.concat(" #").concat(id.toString())
            self.description = description
            self.attributes = attributes
            self.dayImage = dayImage
            self.nightImage = nightImage
            self.animation = animation

            self.utc = 0
            self.auto = true
            self.defaultImage = ImageType.Day

            self.thumbnail = dayImage
        }

        pub fun setUTC(utc: Int8) {
            BarterYardClubWerewolf.timezonedWerewolves.containsKey(utc)
            // Remove werewolf from old UTC
            if BarterYardClubWerewolf.timezonedWerewolves.containsKey(self.utc) {
                BarterYardClubWerewolf.timezonedWerewolves[self.utc]!.remove(key: self.id)
            }
            // Add werewolf to new UTC
            if BarterYardClubWerewolf.timezonedWerewolves.containsKey(utc) {
                BarterYardClubWerewolf.timezonedWerewolves[utc]!.insert(key: self.id, true)
            } else {
                BarterYardClubWerewolf.timezonedWerewolves.insert(key: utc, {
                    self.id: true
                })
            }

            self.utc = utc
            self.computeThumbnail()
        }

        pub fun setAuto(auto: Bool) {
            self.auto = auto
            if !self.auto && BarterYardClubWerewolf.timezonedWerewolves.containsKey(self.utc) {
                BarterYardClubWerewolf.timezonedWerewolves[self.utc]!.remove(key: self.id)
            }  else if self.auto && BarterYardClubWerewolf.timezonedWerewolves.containsKey(self.utc) {
                BarterYardClubWerewolf.timezonedWerewolves[self.utc]!.insert(key: self.id, true)
            }
            self.computeThumbnail()
        }

        pub fun setDefaultImage(imageType: ImageType) {
            self.defaultImage = imageType
            self.computeThumbnail()
        }

        pub fun computeThumbnail() {
            let newThumbnail = self.getThumbnail()
            if newThumbnail.uri() == self.thumbnail.uri() {
                return
            }
            self.thumbnail = newThumbnail
            emit WerewolfTransformed(ids: [self.id], imageType: self.getAutoImageType().rawValue)
        }

        pub fun getAutoImageType(): ImageType {
            if self.auto {
                let hour = BarterYardClubWerewolf.getCurrentHour(self.utc)
                let thumbnail = (hour > 8 && hour < 20 ) ? ImageType.Day : ImageType.Night
                return thumbnail
            }

            return self.defaultImage
        }

        /// getThumbnail implementation from Werewolf interface
        pub fun getThumbnail(): MetadataViews.IPFSFile {
            let imageType = self.getAutoImageType()

            switch imageType {
                case ImageType.Day:
                    return self.dayImage
                case ImageType.Night:
                    return self.nightImage
                case ImageType.Animated:
                    return self.animation
            }

            return self.dayImage
        }

        /// getViews implementation from MetadataViews.Resolver interface
        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(),
                Type<CompleteDisplay>(),
                Type<NFTConfig>()
            ]
        }

        /// resolveView implementation from MetadataViews.Resolver interface
        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name,
                        description: self.description,
                        thumbnail: self.getThumbnail(),
                    )
                case Type<CompleteDisplay>():
                    return CompleteDisplay(
                        name: self.name,
                        description: self.description,
                        attributes: self.attributes,
                        thumbnail: self.getThumbnail(),
                        dayImage: self.dayImage,
                        nightImage: self.nightImage,
                        animation: self.animation,
                    )
                case Type<NFTConfig>():
                    return NFTConfig(
                        utc: self.utc,
                        auto: self.auto,
                        defaultImage: self.defaultImage,
                    )
            }

            return nil
        }

        destroy() {
            BarterYardClubWerewolf.burnNFT()
            emit Burn(id: self.id)
        }
    }

    /**************/
    /* COLLECTION */
    /**************/

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @BarterYardClubWerewolf.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowBarterYardClubWerewolfNFT(id: UInt64): &BarterYardClubWerewolf.NFT? {
            pre {
                self.ownedNFTs[id] != nil : "NFT not in collection"
            }
            // Create an authorized reference to allow downcasting
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &BarterYardClubWerewolf.NFT
        }

        pub fun borrowViewResolver(id: UInt64): &{MetadataViews.Resolver} {
          pre {
            self.ownedNFTs[id] != nil : "NFT not in collection"
          }
          let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
          let BarterYardClubWerewolf = nft as! &BarterYardClubWerewolf.NFT
          return BarterYardClubWerewolf
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // function to burn the NFT
    access(self) fun burnNFT() {
        // Check NFT ownership from caller?
        self.totalSupply = self.totalSupply - 1
    }

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource Admin {

        // mintNFT mints a new NFT with a new Id
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(
            name: String,
            description: String,
            attributes: [NftTrait; 8],
            dayImage: MetadataViews.IPFSFile,
            nightImage: MetadataViews.IPFSFile,
            animation: MetadataViews.IPFSFile,
        ) : @BarterYardClubWerewolf.NFT {
            // create a new NFT
            var newNFT <- create NFT(
                // Start ID at 1
                id: BarterYardClubWerewolf.totalSupply + 1,
                name: name,
                description: description,
                attributes: attributes,
                dayImage: dayImage,
                nightImage: nightImage,
                animation: animation,
            )

            for attribute in attributes {
                let trait = BarterYardClubWerewolf.traits[attribute.id]
                    ?? panic("[Admin](mintNFT) Invalid trait providen")
                trait.increment()
                BarterYardClubWerewolf.traits[attribute.id] = trait
            }

            emit Mint(id: newNFT.id)

            BarterYardClubWerewolf.totalSupply = BarterYardClubWerewolf.totalSupply + 1
            BarterYardClubWerewolf.timezonedWerewolves.insert(key: 0, {
                newNFT.id: true
            })

            return <- newNFT
        }

        pub fun addTraitType(_ traitType: String) {
            BarterYardClubWerewolf.traitTypes.insert(
                key: (Int8)(BarterYardClubWerewolf.traitTypes.length),
                traitType,
            )
        }

        pub fun addTrait(traitType: Int8, value: String) {
            pre {
                BarterYardClubWerewolf.traitTypes[traitType] != nil:
                    "[Admin](addTrait): couldn't add trait because traitTypes doesn't exists"
            }

            let id = (UInt8)(BarterYardClubWerewolf.traits.length)
            BarterYardClubWerewolf.traits.insert(
                key: id,
                Trait(
                    id: id,
                    traitType: traitType,
                    value: value,
                )
            )
        }
    }

    pub fun getTraitsTypes(): {Int8: String} {
        return self.traitTypes
    }

    pub fun getPublicTraits(): {UInt8: PublicTrait} {
        let traits: {UInt8: PublicTrait} = {}
        for trait in self.traits.values {
            traits.insert(key: trait.id, trait.toPublic())
        }
        return traits
    }

    pub fun getNftTraits(): {UInt8: NftTrait} {
        let traits: {UInt8: NftTrait} = {}
        for trait in self.traits.values {
            traits.insert(key: trait.id, trait.toNft())
        }
        return traits
    }

    pub fun getEmptyNftTraits(): [BarterYardClubWerewolf.NftTrait; 8] {
        let emptyTrait = NftTrait(
            id: 0,
            traitType: -1,
            value: "Empty",
        )
        return [
            emptyTrait,
            emptyTrait,
            emptyTrait,
            emptyTrait,
            emptyTrait,
            emptyTrait,
            emptyTrait,
            emptyTrait
        ]
    }

    pub fun getCurrentHour(_ utc: Int8): Int {
        let currentTime = (Int)(getCurrentBlock().timestamp)
        let hour = (currentTime + ((Int)(utc) * 3600)) / 3600 % 24
        return hour
    }

    pub fun getTimezonedWerewolves(): {Int8: {UInt64: Bool}} {
        return self.timezonedWerewolves
    }

    /// computeTimezoneForWerewolves gets werewolves in timezones that needs to be updated and call computeThumbnail
    pub fun computeTimezoneForWerewolves() {
        let utcTimestamp = (Int)(getCurrentBlock().timestamp)
        var elapsedHours = (utcTimestamp - self.lastSyncedTimestamp) / 3600 % 24

        if elapsedHours == 0 {
            return
        }

        while elapsedHours > 0 {
            let hour = (utcTimestamp - 3600 * (elapsedHours - 1)) / 3600 % 24

            let dayStartTimezone = Int8(8 - hour)
            let dayEndTimezone = Int8((hour > 8) ? dayStartTimezone + 12 : dayStartTimezone - 12)

            if self.timezonedWerewolves[dayStartTimezone] == nil && self.timezonedWerewolves[dayEndTimezone] == nil {
                continue
            }

            if self.timezonedWerewolves[dayStartTimezone] != nil {
                emit WerewolfTransformed(
                    ids: self.timezonedWerewolves[dayStartTimezone]!.keys,
                    imageType: ImageType.Day.rawValue,
                )
            }

            if self.timezonedWerewolves[dayEndTimezone] != nil {
                emit WerewolfTransformed(
                    ids: self.timezonedWerewolves[dayEndTimezone]!.keys,
                    imageType: ImageType.Night.rawValue,
                )
            }

            elapsedHours = elapsedHours - 1
        }

        self.lastSyncedTimestamp = utcTimestamp
    }

    init() {
        // Initialize the total supply and max supply
        self.totalSupply = 0
        self.maxSupply = 10000

        self.traits = {}
        self.traitTypes = {}
        self.timezonedWerewolves = {}
        self.lastSyncedTimestamp = (Int)(getCurrentBlock().timestamp)

        // Set the named paths
        self.CollectionStoragePath = /storage/BarterYardClubWerewolfCollection
        self.CollectionPublicPath = /public/BarterYardClubWerewolfCollection
        self.CollectionPrivatePath = /private/BarterYardClubWerewolfCollection
        self.AdminStoragePath = /storage/BarterYardClubWerewolfAdmin
        self.AdminPrivatePath = /private/BarterYardClubWerewolfAdmin

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&BarterYardClubWerewolf.Collection{NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        self.account.link<&BarterYardClubWerewolf.Collection>(BarterYardClubWerewolf.CollectionPrivatePath, target: BarterYardClubWerewolf.CollectionStoragePath)

        // Create an Admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.link<&BarterYardClubWerewolf.Admin>(BarterYardClubWerewolf.AdminPrivatePath, target: BarterYardClubWerewolf.AdminStoragePath)

        emit ContractInitialized()
    }
}

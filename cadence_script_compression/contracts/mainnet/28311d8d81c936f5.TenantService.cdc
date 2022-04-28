
import NonFungibleToken from 0x1d7e57aa55817448

pub contract TenantService: NonFungibleToken {

    // basic data about the tenant
    pub let version: UInt32
    pub let id: String
    pub let name: String
    pub let description: String
    pub var closed: Bool

    // NFT
    pub var totalSupply: UInt64

    // paths
    access(all) let ADMIN_NFT_COLLECTION_PATH: StoragePath
    access(all) let ADMIN_OBJECT_PATH: StoragePath
    access(all) let PRIVATE_NFT_COLLECTION_PATH: StoragePath
    access(all) let PUBLIC_NFT_COLLECTION_PATH: PublicPath

    // archetypes
    access(self) let archetypes: {UInt64: Archetype}
    access(self) let archetypeAdmins: @{UInt64: ArchetypeAdmin}
    access(self) var archetypeSeq: UInt64
    access(self) let artifactsByArchetype: {UInt64: {UInt64: Bool}} // archetypeId -> {artifactId: true}

    // artifacts
    access(self) let artifacts: {UInt64: Artifact}
    access(self) let artifactAdmins: @{UInt64: ArtifactAdmin}
    access(self) var artifactSeq: UInt64
    access(self) var nextNftSerialNumber: {UInt64: UInt64}
    access(self) let setsByArtifact: {UInt64: {UInt64: Bool}} // artifactId -> {setId: true}
    access(self) let faucetsByArtifact: {UInt64: {UInt64: Bool}} // artifactId -> {faucetId: true}

    // sets
    access(self) let sets: {UInt64: Set}
    access(self) let setAdmins: @{UInt64: SetAdmin}
    access(self) var setSeq: UInt64
    access(self) let artifactsBySet: {UInt64: {UInt64: Bool}} // setId -> {artifactId: true}
    access(self) let faucetsBySet: {UInt64: {UInt64: Bool}} // setId -> {faucetId: true}

    // prints
    access(self) let prints: {UInt64: Print}
    access(self) let printAdmins: @{UInt64: PrintAdmin}
    access(self) var printSeq: UInt64

    // faucets
    access(self) let faucets: {UInt64: Faucet}
    access(self) let faucetAdmins: @{UInt64: FaucetAdmin}
    access(self) var faucetSeq: UInt64

    // tenant events
    pub event TenantClosed()

    // NFT events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    init() {
        self.version = 1
        self.id = "eaysGpgi7T"
        self.name = "Cypress Hill"
        self.description = "Cypress Hill NFTs"
        self.closed = false

        self.archetypes = {}
        self.archetypeAdmins <- {}
        self.archetypeSeq = 1
        self.artifactsByArchetype = {}

        self.artifacts = {}
        self.artifactAdmins <- {}
        self.artifactSeq = 1
        self.nextNftSerialNumber = {}
        self.setsByArtifact = {}
        self.faucetsByArtifact = {}

        self.sets = {}
        self.setAdmins <- {}
        self.setSeq = 1
        self.artifactsBySet = {}
        self.faucetsBySet = {}

        self.prints = {}
        self.printAdmins <- {}
        self.printSeq = 1

        self.faucets = {}
        self.faucetAdmins <- {}
        self.faucetSeq = 1

        self.totalSupply = 0

        self.OBJECT_TYPE_MASK = UInt64.max << 55
        self.SEQUENCE_MASK = (UInt64.max << UInt64(9)) >> UInt64(9)

        self.ADMIN_NFT_COLLECTION_PATH = /storage/AdminNFTCollection_eaysGpgi7T
        self.ADMIN_OBJECT_PATH = /storage/TenantAdmin_eaysGpgi7T
        self.PRIVATE_NFT_COLLECTION_PATH = /storage/TenantNFTCollection_eaysGpgi7T
        self.PUBLIC_NFT_COLLECTION_PATH = /public/TenantNFTCollection_eaysGpgi7T

        // create a collection for the admin
        self.account.save<@ShardedCollection>(<- TenantService.createEmptyShardedCollection(numBuckets: 32), to: TenantService.ADMIN_NFT_COLLECTION_PATH)

        // Create a public capability for the Collection
        self.account.link<&{CollectionPublic}>(TenantService.PUBLIC_NFT_COLLECTION_PATH, target: TenantService.ADMIN_NFT_COLLECTION_PATH)

        // put the admin in storage
        self.account.save<@TenantAdmin>(<- create TenantAdmin(), to: TenantService.ADMIN_OBJECT_PATH)

        emit ContractInitialized()
    }

    pub enum ObjectType: UInt8 {
        pub case UNKNOWN

        // An Archetype is a high level organizational unit for a type of NFT. For instance, in the
        // case that the Tenant is a company dealing with professional sports they might have an Archetype
        // for each of the sports that they support, ie: Basketball, Baseball, Football, etc.
        //
        pub case ARCHETYPE

        // An Artifact is the actual object that is minted as an NFT. It contains all of the meta data data
        // and a reference to the Archetype that it belongs to.
        //
        pub case ARTIFACT

        // NFTs can be minted into a Set. A set could be something along the lines of "Greatest Pitchers",
        // "Slam Dunk Artists", or "Running Backs" (continuing with the sports theme from above). NFT do
        // not have to be minted into a set. Also, an NFT could be minted from an Artifact by itself, and
        // in another instance as part of a set - so that the NFT references the same Artifact, but only
        // one of them belongs to the Set.
        //
        pub case SET

        // A Print reserves a block of serial numbers for minting at a later time. It is associated with
        // a single Artifact and when the Print is minted it reserves the next serial number through however
        // many serial numbers are to be reserved. NFTs can then later be minted from the Print and will
        // be given the reserved serial numbers.
        //
        pub case PRINT

        // A Faucet is similar to a Print except that it doesn't reserve a block of serial numbers, it merely
        // mints NFTs from a given Artifact on demand. A Faucet can have a maxMintCount or be unbound and
        // mint infinitely (or however many NFTs are allowed to be minted for the Artifact that it is bound to).
        //
        pub case FAUCET

        // An NFT holds metadata, a reference to it's Artifact (and therefore Archetype), a reference to
        // it's Set (if it belongs to one), a reference to it's Print (if it was minted by one), a reference
        // to it's Faucet (if it was minted by one) and has a unique serial number.
        pub case NFT
    }

    pub let OBJECT_TYPE_MASK: UInt64
    pub let SEQUENCE_MASK: UInt64

    // Generates an ID for the given object type and sequence. We generate IDs this way
    // so that they are unique across the various types of objects supported by this
    // contract.
    //
    pub fun generateId(_ objectType: ObjectType, _ sequence: UInt64): UInt64 {
        if (sequence > 36028797018963967) {
            panic("sequence may only have 55 bits and must be less than 36028797018963967")
        }
        var ret: UInt64 = UInt64(objectType.rawValue)
        ret = ret << UInt64(55)
        ret = ret | ((sequence << UInt64(9)) >> UInt64(9))
        return ret
    }

    // Extracts the ObjectType from an id
    //
    pub fun getObjectType(_ id: UInt64): ObjectType {
        return ObjectType(rawValue: UInt8(id >> UInt64(55)))!
    }

    // Extracts the sequence from an id
    //
    pub fun getSequence(_ id: UInt64): UInt64 {
        return id & TenantService.SEQUENCE_MASK
    }

    // Indicates whether or not the given id is for a given ObjectType.
    //
    pub fun isObjectType(_ id: UInt64, _ objectType: ObjectType): Bool {
        return (TenantService.getObjectType(id) == objectType)
    }

    // Returns the tenant id that was supplied when the contract was created
    //
    pub fun getTenantId(): String {
        return self.id
    }

    // Returns the version of this contract
    //
    pub fun getVersion(): UInt32 {
        return self.version
    }

    // TenantAdmin is used for administering the Tenant
    //
    pub resource TenantAdmin {

        // Closes the Tenant, rendering any write access impossible
        //
        pub fun close() {
            if !TenantService.closed {
                TenantService.closed = true
                emit TenantClosed()
            }
        }

        // Creates a new Archetype returning it's id.
        //
        pub fun createArchetype(
            name: String,
            description: String,
            metadata: {String: TenantService.MetadataField}
        ): UInt64 {
            pre {
                TenantService.closed != true: "The Tenant is closed"
            }
            var archetype = Archetype(name: name, description: description, metadata: metadata)
            TenantService.archetypes[archetype.id] = archetype
            TenantService.archetypeAdmins[archetype.id] <-! create ArchetypeAdmin(archetype.id)
            return archetype.id
        }

        // Grants admin access to the given Archetype
        //
        pub fun borrowArchetypeAdmin(_ id: UInt64): &ArchetypeAdmin {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.archetypeAdmins[id] != nil: "Archetype not found"
                TenantService.isObjectType(id, ObjectType.ARCHETYPE): "ObjectType is not an Archetype"
            }
            return &TenantService.archetypeAdmins[id] as &ArchetypeAdmin
        }

        // Creates a new Artifact returning it's id.
        //
        pub fun createArtifact(
            archetypeId: UInt64,
            name: String,
            description: String,
            maxMintCount: UInt64,
            metadata: {String: TenantService.MetadataField}
        ): UInt64 {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.archetypes[archetypeId] != nil: "The Archetype wasn't found"
                self.borrowArchetypeAdmin(archetypeId).closed != true: "The Archetype is closed"
            }
            var artifact = Artifact(archetypeId: archetypeId, name: name, description: description, maxMintCount: maxMintCount, metadata: metadata)
            TenantService.artifacts[artifact.id] = artifact
            TenantService.artifactAdmins[artifact.id] <-! create ArtifactAdmin(id: artifact.id)
            TenantService.nextNftSerialNumber[artifact.id] = 1
            return artifact.id
        }

        // Grants admin access to the given Artifact
        //
        pub fun borrowArtifactAdmin(_ id: UInt64): &ArtifactAdmin {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.artifactAdmins[id] != nil: "Artifact not found"
                TenantService.isObjectType(id, ObjectType.ARTIFACT): "ObjectType is not an Artifact"
            }
            return &TenantService.artifactAdmins[id] as &ArtifactAdmin
        }

        // Creates a new Set returning it's id.
        //
        pub fun createSet(name: String, description: String, metadata: {String: TenantService.MetadataField}): UInt64 {
            pre {
                TenantService.closed != true: "The Tenant is closed"
            }
            var set = Set(name: name, description: description, metadata: metadata)
            TenantService.sets[set.id] = set
            TenantService.setAdmins[set.id] <-! create SetAdmin(set.id)
            return set.id
        }

        // Grants admin access to the given Set
        //
        pub fun borrowSetAdmin(_ id: UInt64): &SetAdmin {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.setAdmins[id] != nil: "Set not found"
                TenantService.isObjectType(id, ObjectType.SET): "ObjectType is not a Set"
            }
            return &TenantService.setAdmins[id] as &SetAdmin
        }

        // Creates a new Print returning it's id.
        //
        pub fun createPrint(
           artifactId: UInt64,
           setId: UInt64?,
           name: String,
           description: String,
           maxMintCount: UInt64,
           metadata: {String: TenantService.MetadataField}
       ): UInt64 {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.borrowArtifactAdmin(artifactId).closed != true: "The Artifact is closed"
                setId == nil || self.borrowSetAdmin(setId!).closed != true: "The Set is closed"
            }
            var print = Print(artifactId: artifactId, setId: setId, name: name, description: description, maxMintCount: maxMintCount, metadata: metadata)
            TenantService.prints[print.id] = print
            TenantService.printAdmins[print.id] <-! create PrintAdmin(print.id, print.serialNumberStart)
            return print.id
        }

        // Grants admin access to the given Print
        //
        pub fun borrowPrintAdmin(_ id: UInt64): &PrintAdmin {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.printAdmins[id] != nil: "Print not found"
                TenantService.isObjectType(id, ObjectType.PRINT): "ObjectType is not a print"
            }
            return &TenantService.printAdmins[id] as &PrintAdmin
        }

        // Creates a new Faucet returning it's id.
        //
        pub fun createFaucet(
           artifactId: UInt64,
           setId: UInt64?,
           name: String,
           description: String,
           maxMintCount: UInt64,
           metadata: {String: TenantService.MetadataField}
       ): UInt64 {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.borrowArtifactAdmin(artifactId).closed != true: "The Artifact is closed"
                setId == nil || self.borrowSetAdmin(setId!).closed != true: "The Set is closed"
            }
            var faucet = Faucet(artifactId: artifactId, setId: setId, name: name, description: description, maxMintCount: maxMintCount, metadata: metadata)
            TenantService.faucets[faucet.id] = faucet
            TenantService.faucetAdmins[faucet.id] <-! create FaucetAdmin(id: faucet.id)
            return faucet.id
        }

        // Grants admin access to the given Faucet
        //
        pub fun borrowFaucetAdmin(_ id: UInt64): &FaucetAdmin {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                TenantService.faucetAdmins[id] != nil: "Faucet not found"
                TenantService.isObjectType(id, ObjectType.FAUCET): "ObjectType is not a faucet"
            }
            return &TenantService.faucetAdmins[id] as &FaucetAdmin
        }

        // Mints an NFT
        //
        pub fun mintNFT(artifactId: UInt64, printId: UInt64?, faucetId: UInt64?, setId: UInt64?, metadata: {String: TenantService.MetadataField}): @NFT {
            pre {
                TenantService.artifacts[artifactId] != nil: "Cannot mint the NFT: The Artifact wasn't found"
                self.borrowArtifactAdmin(artifactId).closed != true: "The Artifact is closed"

                printId == nil || TenantService.isObjectType(printId!, ObjectType.PRINT): "Id supplied for printId is not an ObjectType of print"
                faucetId == nil || TenantService.isObjectType(faucetId!, ObjectType.FAUCET): "Id supplied for faucetId is not an ObjectType of faucet"
                setId == nil || TenantService.isObjectType(setId!, ObjectType.SET): "Id supplied for setId is not an ObjectType of set"

                printId == nil || TenantService.prints[printId!] != nil: "Cannot mint the NFT: The Print wasn't found"
                faucetId == nil || TenantService.faucets[faucetId!] != nil: "Cannot mint the NFT: The Faucet wasn't found"
                setId == nil || TenantService.sets[setId!] != nil: "Cannot mint the NFT: The Set wasn't found"

                printId == nil || self.borrowPrintAdmin(printId!).closed != true: "The Print is closed"
                faucetId == nil || self.borrowFaucetAdmin(faucetId!).closed != true: "The Faucet is closed"
                setId == nil || self.borrowSetAdmin(setId!).closed != true: "The Set is closed"

                faucetId == nil || TenantService.faucets[faucetId!]!.artifactId == artifactId: "The artifactId doesn't match the Faucet's artifactId"
                printId == nil || TenantService.prints[printId!]!.artifactId == artifactId: "The artifactId doesn't match the Print's artifactId"

                faucetId == nil || TenantService.faucets[faucetId!]!.setId == setId: "The setId doesn't match the Faucet's setId"
                printId == nil || TenantService.prints[printId!]!.setId == setId: "The setId doesn't match the Print's setId"

                !(faucetId != nil && printId != nil): "Can only mint from one of a faucet or print"
            }

            let artifact: Artifact = TenantService.artifacts[artifactId]!
            let artifactAdmin = self.borrowArtifactAdmin(artifactId)
            artifactAdmin.logMint(1)
            if printId != nil {
                artifactAdmin.logPrint(1)
            }

            let archetype: Archetype = TenantService.archetypes[artifact.archetypeId]!
            let archetypeAdmin = self.borrowArchetypeAdmin(artifact.archetypeId)
            if archetypeAdmin != nil {
                archetypeAdmin.logMint(1)
                if printId != nil {
                    archetypeAdmin.logPrint(1)
                }
            }

            if faucetId != nil {
                let faucetAdmin = self.borrowFaucetAdmin(faucetId!)
                faucetAdmin.logMint(1)
            }

            if setId != nil {
                let setAdmin = self.borrowSetAdmin(setId!)
                setAdmin.logMint(1)
                if printId != nil {
                    setAdmin.logPrint(1)
                }
            }

            if printId != nil {
                let printAdmin = self.borrowPrintAdmin(printId!)
                printAdmin.logMint(1)
            }

            let newNFT: @NFT <- create NFT(
                archetypeId: artifact.archetypeId,
                artifactId: artifact.id,
                printId: printId,
                faucetId: faucetId,
                setId: setId,
                metadata: metadata)

            return <- newNFT
        }

        // Mints many NFTs
        //
        pub fun batchMintNFTs(
            count: UInt64,
            artifactId: UInt64,
            printId: UInt64?,
            faucetId: UInt64?,
            setId: UInt64?,
            metadata: {String: TenantService.MetadataField}
        ): @Collection {
            let newCollection <- create Collection()
            var i: UInt64 = 0
            while i < count {
                newCollection.deposit(token: <-self.mintNFT(
                    artifactId: artifactId,
                    printId: printId,
                    faucetId: faucetId,
                    setId: setId,
                    metadata: metadata
                ))
                i = i + (1 as UInt64)
            }
            return <- newCollection
        }

        // Creates a new TenantAdmin that allows for another account
        // to administer the Tenant
        //
        pub fun createNewTenantAdmin(): @TenantAdmin {
            return <- create TenantAdmin()
        }
    }

    // =====================================
    // Archetype
    // =====================================

    pub event ArchetypeCreated(_ id: UInt64)
    pub event ArchetypeDestroyed(_ id: UInt64)
    pub event ArchetypeClosed(_ id: UInt64)

    pub fun getArchetype(_ id: UInt64): Archetype? {
        pre {
            TenantService.isObjectType(id, ObjectType.ARCHETYPE): "Id supplied is not for an archetype"
        }
        return TenantService.archetypes[id]
    }

    pub fun getArchetypeView(_ id: UInt64): ArchetypeView? {
        pre {
            TenantService.isObjectType(id, ObjectType.ARCHETYPE): "Id supplied is not for an archetype"
        }
        if TenantService.archetypes[id] == nil {
            return nil
        }
        let archetype = TenantService.archetypes[id]!
        let archetypeAdmin = &TenantService.archetypeAdmins[id] as &ArchetypeAdmin
        return ArchetypeView(
            id: archetype.id,
            name: archetype.name,
            description: archetype.description,
            metadata: archetype.metadata,
            mintCount: archetypeAdmin.mintCount,
            printCount: archetypeAdmin.printCount,
            closed: archetypeAdmin.closed
        )
    }

    pub fun getArchetypeViews(_ archetypes: [UInt64]): [ArchetypeView] {
        let ret: [ArchetypeView] = []
        for archetype in archetypes {
            let element = self.getArchetypeView(archetype)
            if element != nil {
                ret.append(element!)
            }
        }
        return ret
    }

    pub fun getAllArchetypes(): [Archetype] {
        return TenantService.archetypes.values
    }

    // The immutable data for an Archetype
    //
    pub struct Archetype {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let metadata: {String: TenantService.MetadataField}

        init(name: String, description: String, metadata: {String: TenantService.MetadataField}) {
            self.id = TenantService.generateId(ObjectType.ARCHETYPE, TenantService.archetypeSeq)
            self.name = name
            self.description = description
            self.metadata = metadata
            TenantService.archetypeSeq = TenantService.archetypeSeq + 1 as UInt64
            emit ArchetypeCreated(self.id)
        }
    }

    // The mutable data for an Archetype
    //
    pub resource ArchetypeAdmin {

        pub let id: UInt64
        pub var mintCount: UInt64
        pub var printCount: UInt64
        pub var closed: Bool

        init(_ id: UInt64) {
            self.id = id
            self.mintCount = 0
            self.printCount = 0
            self.closed = false
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                emit ArchetypeClosed(self.id)
            }
        }

        pub fun logMint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Archetype is closed"
            }
            self.mintCount = self.mintCount + count
        }

        pub fun logPrint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Archetype is closed"
            }
            self.printCount = self.printCount + count
        }

        destroy() {
            emit ArchetypeDestroyed(self.id)
        }
    }

    // An immutable view for an Archetype and all of it's data
    //
    pub struct ArchetypeView {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let metadata: {String: TenantService.MetadataField}
        pub let mintCount: UInt64
        pub let printCount: UInt64
        pub let closed: Bool

        init(
            id: UInt64,
            name: String,
            description: String,
            metadata: {String: TenantService.MetadataField},
            mintCount: UInt64,
            printCount: UInt64,
            closed: Bool
        ) {
            self.id = id
            self.name = name
            self.description = description
            self.metadata = metadata
            self.mintCount = mintCount
            self.printCount = printCount
            self.closed = closed
        }
    }

    // =====================================
    // Artifact
    // =====================================

    pub event ArtifactCreated(_ id: UInt64)
    pub event ArtifactMaxMintCountChanged(_ id: UInt64, _ oldMaxMintCount: UInt64, _ newMaxMintCount: UInt64)
    pub event ArtifactDestroyed(_ id: UInt64)
    pub event ArtifactClosed(_ id: UInt64)

    pub fun getArtifact(_ id: UInt64): Artifact? {
        pre {
            TenantService.isObjectType(id, ObjectType.ARTIFACT): "Id supplied is not for an artifact"
        }
        return TenantService.artifacts[id]
    }

    pub fun getArtifactView(_ id: UInt64): ArtifactView? {
        pre {
            TenantService.isObjectType(id, ObjectType.ARTIFACT): "Id supplied is not for an artifact"
        }
        if TenantService.artifacts[id] == nil {
            return nil
        }
        let artifact = TenantService.artifacts[id]!
        let artifactAdmin = &TenantService.artifactAdmins[id] as &ArtifactAdmin
        return ArtifactView(
            id: artifact.id,
            archetypeId: artifact.archetypeId,
            name: artifact.name,
            description: artifact.description,
            metadata: artifact.metadata,
            maxMintCount: artifact.maxMintCount,
            mintCount: artifactAdmin.mintCount,
            printCount: artifactAdmin.printCount,
            closed: artifactAdmin.closed
        )
    }

    pub fun getArtifactViews(_ artifacts: [UInt64]): [ArtifactView] {
        let ret: [ArtifactView] = []
        for artifact in artifacts {
            let element = self.getArtifactView(artifact)
            if element != nil {
                ret.append(element!)
            }
        }
        return ret
    }

    pub fun getAllArtifacts(): [Artifact] {
        return TenantService.artifacts.values
    }

    pub fun getArtifactsBySet(_ setId: UInt64): [UInt64] {
        let map = TenantService.artifactsBySet[setId]
        if map != nil {
            return map!.keys
        }
        return []
    }

    pub fun getFaucetsBySet(_ setId: UInt64): [UInt64] {
        let map = TenantService.faucetsBySet[setId]
        if map != nil {
            return map!.keys
        }
        return []
    }

    pub fun getSetsByArtifact(_ artifactId: UInt64): [UInt64] {
        let map = TenantService.setsByArtifact[artifactId]
        if map != nil {
            return map!.keys
        }
        return []
    }

    pub fun getFaucetsByArtifact(_ artifactId: UInt64): [UInt64] {
        let map = TenantService.faucetsByArtifact[artifactId]
        if map != nil {
            return map!.keys
        }
        return []
    }

    pub fun getArtifactsByArchetype(_ archetypeId: UInt64): [UInt64] {
        let map = TenantService.artifactsByArchetype[archetypeId]
        if map != nil {
            return map!.keys
        }
        return []
    }

    // The immutable data for an Artifact
    //
    pub struct Artifact {
        pub let id: UInt64
        pub let archetypeId: UInt64
        pub let name: String
        pub let description: String
        pub let maxMintCount: UInt64
        pub let metadata: {String: TenantService.MetadataField}

        init(archetypeId: UInt64, name: String, description: String, maxMintCount: UInt64, metadata: {String: TenantService.MetadataField}) {
            self.id = TenantService.generateId(ObjectType.ARTIFACT, TenantService.artifactSeq)
            self.archetypeId = archetypeId
            self.name = name
            self.description = description
            self.maxMintCount = maxMintCount
            self.metadata = metadata
            TenantService.artifactSeq = TenantService.artifactSeq + 1 as UInt64
            emit ArtifactCreated(self.id)
        }
    }

    // The mutable data for an Artifact
    //
    pub resource ArtifactAdmin {

        pub let id: UInt64
        pub var mintCount: UInt64
        pub var printCount: UInt64
        pub var closed: Bool

        init(id: UInt64) {
            self.id = id
            self.mintCount = 0
            self.printCount = 0
            self.closed = false
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                emit ArtifactClosed(self.id)
            }
        }

        pub fun logMint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Artifact is closed"
                ((TenantService.artifacts[self.id]!.maxMintCount == (0 as UInt64))
                    || (TenantService.artifacts[self.id]!.maxMintCount >= (self.mintCount + count))): "The Artifact would exceed it's maxMintCount"
            }
            self.mintCount = self.mintCount + count
        }

        pub fun logPrint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Artifact is closed"
            }
            self.printCount = self.printCount + count
        }

        destroy() {
            emit ArtifactDestroyed(self.id)
        }
    }

    // An immutable view for an Artifact and all of it's data
    //
    pub struct ArtifactView {
        pub let id: UInt64
        pub let archetypeId: UInt64
        pub let name: String
        pub let description: String
        pub let metadata: {String: TenantService.MetadataField}
        pub let maxMintCount: UInt64
        pub let mintCount: UInt64
        pub let printCount: UInt64
        pub let closed: Bool

        init(
            id: UInt64,
            archetypeId: UInt64,
            name: String,
            description: String,
            metadata: {String: TenantService.MetadataField},
            maxMintCount: UInt64,
            mintCount: UInt64,
            printCount: UInt64,
            closed: Bool
        ) {
            self.id = id
            self.archetypeId = archetypeId
            self.name = name
            self.description = description
            self.metadata = metadata
            self.maxMintCount = maxMintCount
            self.mintCount = mintCount
            self.printCount = printCount
            self.closed = closed
        }
    }

    // =====================================
    // Set
    // =====================================

    pub event SetCreated(_ id: UInt64)
    pub event SetDestroyed(_ id: UInt64)
    pub event SetClosed(_ id: UInt64)

    pub fun getSet(_ id: UInt64): Set? {
        pre {
            TenantService.isObjectType(id, ObjectType.SET): "Id supplied is not for an set"
        }
        return TenantService.sets[id]
    }

    pub fun getSetView(_ id: UInt64): SetView? {
        pre {
            TenantService.isObjectType(id, ObjectType.SET): "Id supplied is not for an set"
        }
        if TenantService.sets[id] == nil {
            return nil
        }
        let set = TenantService.sets[id]!
        let setAdmin = &TenantService.setAdmins[id] as &SetAdmin
        return SetView(
            id: set.id,
            name: set.name,
            description: set.description,
            metadata: set.metadata,
            mintCount: setAdmin.mintCount,
            printCount: setAdmin.printCount,
            closed: setAdmin.closed
        )
    }

    pub fun getSetViews(_ sets: [UInt64]): [SetView] {
        let ret: [SetView] = []
        for set in sets {
            let element = self.getSetView(set)
            if element != nil {
                ret.append(element!)
            }
        }
        return ret
    }

    pub fun getAllSets(): [Set] {
        return TenantService.sets.values
    }

    // The immutable data for an Set
    //
    pub struct Set {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let metadata: {String: TenantService.MetadataField}

        init(
            name: String,
            description: String,
            metadata: {String: TenantService.MetadataField}
        ) {
            self.id = TenantService.generateId(ObjectType.SET, TenantService.setSeq)
            self.name = name
            self.description = description
            self.metadata = metadata
            TenantService.setSeq = TenantService.setSeq + 1 as UInt64
            TenantService.faucetsBySet[self.id] = {}
            emit SetCreated(self.id)
        }
    }

    // The mutable data for an Set
    //
    pub resource SetAdmin {
        pub let id: UInt64
        pub var mintCount: UInt64
        pub var printCount: UInt64
        pub var closed: Bool

        init(_ id: UInt64) {
            self.id = id
            self.mintCount = 0
            self.printCount = 0
            self.closed = false
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                emit SetClosed(self.id)
            }
        }

        pub fun logMint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Set is closed"
            }
            self.mintCount = self.mintCount + count
        }

        pub fun logPrint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Set is closed"
            }
            self.printCount = self.printCount + count
        }

        destroy() {
            emit SetDestroyed(self.id)
        }
    }

    // An immutable view for an Set and all of it's data
    //
    pub struct SetView {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let metadata: {String: TenantService.MetadataField}
        pub let mintCount: UInt64
        pub let printCount: UInt64
        pub let closed: Bool

        init(
            id: UInt64,
            name: String,
            description: String,
            metadata: {String: TenantService.MetadataField},
            mintCount: UInt64,
            printCount: UInt64,
            closed: Bool
        ) {
            self.id = id
            self.name = name
            self.description = description
            self.metadata = metadata
            self.mintCount = mintCount
            self.printCount = printCount
            self.closed = closed
        }
    }

    // =====================================
    // Print
    // =====================================

    pub event PrintCreated(_ id: UInt64)
    pub event PrintDestroyed(_ id: UInt64)
    pub event PrintClosed(_ id: UInt64)

    pub fun getPrint(_ id: UInt64): Print? {
        pre {
            TenantService.isObjectType(id, ObjectType.PRINT): "Id supplied is not for a print"
        }
        return TenantService.prints[id]
    }

    pub fun getPrintView(_ id: UInt64): PrintView? {
        pre {
            TenantService.isObjectType(id, ObjectType.PRINT): "Id supplied is not for a print"
        }
        if TenantService.prints[id] == nil {
            return nil
        }
        let print = TenantService.prints[id]!
        let printAdmin = &TenantService.printAdmins[id] as &PrintAdmin
        return PrintView(
            id: print.id,
            artifactId: print.artifactId,
            setId: print.setId,
            name: print.name,
            description: print.description,
            maxMintCount: print.maxMintCount,
            metadata: print.metadata,
            serialNumberStart: print.serialNumberStart,
            nextNftSerialNumber: printAdmin.nextNftSerialNumber,
            mintCount: printAdmin.mintCount,
            closed: printAdmin.closed
        )
    }

    pub fun getPrintViews(_ prints: [UInt64]): [PrintView] {
        let ret: [PrintView] = []
        for print in prints {
            let element = self.getPrintView(print)
            if element != nil {
                ret.append(element!)
            }
        }
        return ret
    }

    pub fun getAllPrints(): [Print] {
        return TenantService.prints.values
    }

    // The immutable data for an Print
    //
    pub struct Print {
        pub let id: UInt64
        pub let artifactId: UInt64
        pub let setId: UInt64?
        pub let name: String
        pub let description: String
        pub let maxMintCount: UInt64
        pub let metadata: {String: TenantService.MetadataField}
        pub let serialNumberStart: UInt64

        init(
            artifactId: UInt64,
            setId: UInt64?,
            name: String,
            description: String,
            maxMintCount: UInt64,
            metadata: {String: TenantService.MetadataField}
        ) {
            pre {
                maxMintCount > 0 : "maxMintCount must be greater than 0"
            }
            self.id = TenantService.generateId(ObjectType.PRINT, TenantService.printSeq)
            self.artifactId = artifactId
            self.setId = setId
            self.name = name
            self.description = description
            self.maxMintCount = maxMintCount
            self.metadata = metadata
            self.serialNumberStart = TenantService.nextNftSerialNumber[artifactId]!
            TenantService.nextNftSerialNumber[artifactId] = self.serialNumberStart + maxMintCount
            TenantService.printSeq = TenantService.printSeq + 1 as UInt64
            emit PrintCreated(self.id)
        }
    }

    // The mutable data for an Print
    //
    pub resource PrintAdmin {

        pub let id: UInt64
        pub var nextNftSerialNumber: UInt64
        pub var mintCount: UInt64
        pub var closed: Bool

        init(_ id: UInt64, _ serialNumberStart: UInt64) {
            self.id = id
            self.mintCount = 0
            self.closed = false
            self.nextNftSerialNumber = serialNumberStart
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                emit PrintClosed(self.id)
            }
        }

        pub fun getAndIncrementSerialNumber(): UInt64 {
            let ret: UInt64 = self.nextNftSerialNumber
            self.nextNftSerialNumber = ret + (1 as UInt64)
            return ret
        }

        pub fun logMint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Print is closed"
                TenantService.prints[self.id]!.maxMintCount >= (self.mintCount + count): "The Print would exceed it's maxMintCount"
            }
            self.mintCount = self.mintCount + count
        }

        destroy() {
            emit PrintDestroyed(self.id)
        }
    }

    // An immutable view for an Print and all of it's data
    //
    pub struct PrintView {
        pub let id: UInt64
        pub let artifactId: UInt64
        pub let setId: UInt64?
        pub let name: String
        pub let description: String
        pub let maxMintCount: UInt64
        pub let metadata: {String: TenantService.MetadataField}
        pub let serialNumberStart: UInt64
        pub let nextNftSerialNumber: UInt64
        pub let mintCount: UInt64
        pub let closed: Bool

        init(
            id: UInt64,
            artifactId: UInt64,
            setId: UInt64?,
            name: String,
            description: String,
            maxMintCount: UInt64,
            metadata: {String: TenantService.MetadataField},
            serialNumberStart: UInt64,
            nextNftSerialNumber: UInt64,
            mintCount: UInt64,
            closed: Bool
        ) {
            self.id = id
            self.artifactId = artifactId
            self.setId = setId
            self.name = name
            self.description = description
            self.maxMintCount = maxMintCount
            self.metadata = metadata
            self.serialNumberStart = serialNumberStart
            self.nextNftSerialNumber = nextNftSerialNumber
            self.mintCount = mintCount
            self.closed = closed
        }
    }

    // =====================================
    // Faucet
    // =====================================

    pub event FaucetCreated(_ id: UInt64)
    pub event FaucetMaxMintCountChanged(_ id: UInt64, _ oldMaxMintCount: UInt64, _ newMaxMintCount: UInt64)
    pub event FaucetDestroyed(_ id: UInt64)
    pub event FaucetClosed(_ id: UInt64)

    pub fun getFaucet(_ id: UInt64): Faucet? {
        pre {
            TenantService.isObjectType(id, ObjectType.FAUCET): "Id supplied is not for a faucet"
        }
        return TenantService.faucets[id]
    }

    pub fun getFaucetView(_ id: UInt64): FaucetView? {
        pre {
            TenantService.isObjectType(id, ObjectType.FAUCET): "Id supplied is not for a faucet"
        }
        if TenantService.faucets[id] == nil {
            return nil
        }
        let faucet = TenantService.faucets[id]!
        let faucetAdmin = &TenantService.faucetAdmins[id] as &FaucetAdmin
        return FaucetView(
            id: faucet.id,
            artifactId: faucet.artifactId,
            setId: faucet.setId,
            name: faucet.name,
            description: faucet.description,
            maxMintCount: faucet.maxMintCount,
            metadata: faucet.metadata,
            mintCount: faucetAdmin.mintCount,
            closed: faucetAdmin.closed
        )
    }

    pub fun getFaucetViews(_ faucets: [UInt64]): [FaucetView] {
        let ret: [FaucetView] = []
        for faucet in faucets {
            let element = self.getFaucetView(faucet)
            if element != nil {
                ret.append(element!)
            }
        }
        return ret
    }

    pub fun getAllFaucets(): [Faucet] {
        return TenantService.faucets.values
    }

    // The immutable data for an Faucet
    //
    pub struct Faucet {
        pub let id: UInt64
        pub let artifactId: UInt64
        pub let setId: UInt64?
        pub let name: String
        pub let description: String
        pub let maxMintCount: UInt64
        pub let metadata: {String: TenantService.MetadataField}

        init(
            artifactId: UInt64,
            setId: UInt64?,
            name: String,
            description: String,
            maxMintCount: UInt64,
            metadata: {String: TenantService.MetadataField}
        ) {
            self.id = TenantService.generateId(ObjectType.FAUCET, TenantService.faucetSeq)
            self.artifactId = artifactId
            self.setId = setId
            self.name = name
            self.description = description
            self.maxMintCount = maxMintCount
            self.metadata = metadata
            TenantService.faucetSeq = TenantService.faucetSeq + 1 as UInt64

            if self.setId != nil {
                let faucetsBySet = TenantService.faucetsBySet[self.setId!]!
                faucetsBySet[self.id] = true
            }

            emit FaucetCreated(self.id)
        }
    }

    // The mutable data for an Faucet
    //
    pub resource FaucetAdmin {

        pub let id: UInt64
        pub var mintCount: UInt64
        pub var closed: Bool

        init(id: UInt64) {
            self.id = id
            self.mintCount = 0
            self.closed = false
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                emit FaucetClosed(self.id)
            }
        }

        pub fun logMint(_ count: UInt64) {
            pre {
                TenantService.closed != true: "The Tenant is closed"
                self.closed != true: "The Faucet is closed"
                ((TenantService.faucets[self.id]!.maxMintCount == (0 as UInt64))
                    || (TenantService.faucets[self.id]!.maxMintCount >= (self.mintCount + count))): "The Faucet would exceed it's maxMintCount"
            }
            self.mintCount = self.mintCount + count
        }

        destroy() {
            emit FaucetDestroyed(self.id)
        }
    }

    // An immutable view for an Faucet and all of it's data
    //
    pub struct FaucetView {
        pub let id: UInt64
        pub let artifactId: UInt64
        pub let setId: UInt64?
        pub let name: String
        pub let description: String
        pub let maxMintCount: UInt64
        pub let metadata: {String: TenantService.MetadataField}
        pub let mintCount: UInt64
        pub let closed: Bool

        init(
            id: UInt64,
            artifactId: UInt64,
            setId: UInt64?,
            name: String,
            description: String,
            maxMintCount: UInt64,
            metadata: {String: TenantService.MetadataField},
            mintCount: UInt64,
            closed: Bool
        ) {
            self.id = id
            self.artifactId = artifactId
            self.setId = setId
            self.name = name
            self.description = description
            self.maxMintCount = maxMintCount
            self.metadata = metadata
            self.mintCount = mintCount
            self.closed = closed
        }
    }

    // =====================================
    // NFT
    // =====================================

    pub event NFTCreated(_ id: UInt64)
    pub event NFTDestroyed(_ id: UInt64)

    pub fun getNFTView(_ nft: &NFT): NFTView {
        let archetype = self.getArchetypeView(nft.archetypeId)!
        let artifact = self.getArtifactView(nft.artifactId)!

        var set: SetView? = nil
        if nft.setId != nil {
            set = self.getSetView(nft.setId!)!
        }

        var print: PrintView? = nil
        if nft.printId != nil {
            print = self.getPrintView(nft.printId!)!
        }

        var faucet: FaucetView? = nil
        if nft.faucetId != nil {
            faucet = self.getFaucetView(nft.faucetId!)!
        }

        return NFTView(
            id: nft.id,
            archetype: archetype,
            artifact: artifact,
            print: print,
            faucet: faucet,
            set: set,
            serialNumber: nft.serialNumber,
            metadata: nft.getMetadata()
        )
    }

    pub fun getNFTViews(_ nfts: [&NFT]): [NFTView] {
        let ret: [NFTView] = []
        for nft in nfts {
            ret.append(self.getNFTView(nft))
        }
        return ret
    }

    // The immutable data for an NFT, this is the actual NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let archetypeId: UInt64
        pub let artifactId: UInt64
        pub let printId: UInt64?
        pub let faucetId: UInt64?
        pub let setId: UInt64?
        pub let serialNumber: UInt64
        access(self) let metadata: {String: TenantService.MetadataField}

        init(
            archetypeId: UInt64,
            artifactId: UInt64,
            printId: UInt64?,
            faucetId: UInt64?,
            setId: UInt64?,
            metadata: {String: TenantService.MetadataField}
        ) {
            self.id = TenantService.generateId(ObjectType.NFT, TenantService.totalSupply)
            self.archetypeId = archetypeId
            self.artifactId = artifactId
            self.printId = printId
            self.faucetId = faucetId
            self.setId = setId
            self.metadata = metadata

            if self.printId != nil {
                let printAdmin = &TenantService.printAdmins[self.printId!!] as &PrintAdmin
                self.serialNumber = printAdmin.getAndIncrementSerialNumber()

            } else {
                self.serialNumber = TenantService.nextNftSerialNumber[self.artifactId]!
                TenantService.nextNftSerialNumber[self.artifactId] = self.serialNumber + (1 as UInt64)

            }

            TenantService.totalSupply = TenantService.totalSupply + (1 as UInt64)

            if self.setId != nil {
                if TenantService.setsByArtifact[self.artifactId] == nil {
                    TenantService.setsByArtifact[self.artifactId] = {}
                }
                let setsByArtifact = TenantService.setsByArtifact[self.artifactId]!
                setsByArtifact[self.setId!] = true

                if TenantService.artifactsBySet[self.setId!] == nil {
                    TenantService.artifactsBySet[self.setId!] = {}
                }
                let artifactsBySet = TenantService.artifactsBySet[self.setId!]!
                artifactsBySet[self.artifactId] = true
            }

            if self.faucetId != nil {
                if TenantService.faucetsByArtifact[self.artifactId] == nil {
                    TenantService.faucetsByArtifact[self.artifactId] = {}
                }
                let faucetsByArtifact = TenantService.faucetsByArtifact[self.artifactId]!
                faucetsByArtifact[self.faucetId!] = true
            }

            if TenantService.artifactsByArchetype[self.archetypeId] == nil {
                TenantService.artifactsByArchetype[self.archetypeId] = {}
            }
            let artifactsByArchetype = TenantService.artifactsByArchetype[self.archetypeId]!
            artifactsByArchetype[self.artifactId] = true

            emit NFTCreated(self.id)
        }

        pub fun getMetadata(): {String: TenantService.MetadataField} {
            return self.metadata
        }

        destroy() {
            emit NFTDestroyed(self.id)
        }
    }

    // An immutable view for an NFT and all of it's data
    //
    pub struct NFTView {
        pub let id: UInt64
        pub let archetype: ArchetypeView
        pub let artifact: ArtifactView
        pub let print: PrintView?
        pub let faucet: FaucetView?
        pub let set: SetView?
        pub let serialNumber: UInt64
        pub let metadata: {String: TenantService.MetadataField}

        init(
            id: UInt64,
            archetype: ArchetypeView,
            artifact: ArtifactView,
            print: PrintView?,
            faucet: FaucetView?,
            set: SetView?,
            serialNumber: UInt64,
            metadata: {String: TenantService.MetadataField}
        ) {
            self.id = id
            self.archetype = archetype
            self.artifact = artifact
            self.print = print
            self.faucet = faucet
            self.set = set
            self.serialNumber = serialNumber
            self.metadata = metadata
        }
    }

    // The public version of the collection that accounts can use
    // to deposit NFTs into other accounts
    //
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNFTData(id: UInt64): &TenantService.NFT?
        pub fun borrowNFTDatas(ids: [UInt64]): [&TenantService.NFT]
    }

    // The collection where NFTs are stored
    //
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: NFT does not exist in the collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @TenantService.NFT
            let id = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }
            destroy oldToken
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            pre {
                TenantService.isObjectType(id, ObjectType.NFT): "Id supplied is not for an nft"
            }
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowNFTData(id: UInt64): &TenantService.NFT? {
            pre {
                TenantService.isObjectType(id, ObjectType.NFT): "Id supplied is not for an nft"
            }
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TenantService.NFT
            } else {
                return nil
            }
        }

        pub fun borrowNFTDatas(ids: [UInt64]): [&TenantService.NFT] {
            let nfts: [&TenantService.NFT] = []
            for id in ids {
                let nft = self.borrowNFTData(id: id)
                if nft != nil {
                    nfts.append(nft!)
                }
            }
            return nfts
        }

        pub fun getNFTView(id: UInt64): NFTView? {
            pre {
                TenantService.isObjectType(id, ObjectType.NFT): "Id supplied is not for an nft"
            }
            let nft = self.borrowNFTData(id: id)
            if nft == nil {
                return nil
            }
            return TenantService.getNFTView(nft!)
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TenantService.Collection()
    }

    // ShardedCollection stores a dictionary of TenantService Collections
    // An NFT is stored in the field that corresponds to its id % numBuckets
    //
    pub resource ShardedCollection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var collections: @{UInt64: Collection}
        pub let numBuckets: UInt64

        init(numBuckets: UInt64) {
            self.collections <- {}
            self.numBuckets = numBuckets
            var i: UInt64 = 0
            while i < numBuckets {
                self.collections[i] <-! TenantService.createEmptyCollection() as! @Collection
                i = i + UInt64(1)
            }
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            post {
                result.id == withdrawID: "The ID of the withdrawn NFT is incorrect"
            }
            let bucket = withdrawID % self.numBuckets
            let token <- self.collections[bucket]?.withdraw(withdrawID: withdrawID)!
            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- TenantService.createEmptyCollection()
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let bucket = token.id % self.numBuckets
            let collection <- self.collections.remove(key: bucket)!
            collection.deposit(token: <-token)
            self.collections[bucket] <-! collection
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            var ids: [UInt64] = []
            for key in self.collections.keys {
                for id in self.collections[key]?.getIDs() ?? [] {
                    ids.append(id)
                }
            }
            return ids
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.borrowNFT(id: id)!
        }

        pub fun borrowNFTData(id: UInt64): &TenantService.NFT? {
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.borrowNFTData(id: id) ?? nil
        }

        pub fun borrowNFTDatas(ids: [UInt64]): [&TenantService.NFT] {
            let nfts: [&TenantService.NFT] = []
            for id in ids {
                let nft = self.borrowNFTData(id: id)
                if nft != nil {
                    nfts.append(nft!)
                }
            }
            return nfts
        }

        pub fun getNFTView(id: UInt64): NFTView? {
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.getNFTView(id: id) ?? nil
        }

        destroy() {
            destroy self.collections
        }
    }

    pub fun createEmptyShardedCollection(numBuckets: UInt64): @ShardedCollection {
        return <-create ShardedCollection(numBuckets: numBuckets)
    }

    // =====================================
    // Metadata
    // =====================================

    // The type of a meta data field
    //
    pub enum MetadataFieldType: UInt8 {
        pub case STRING
        pub case MIME
        pub case NUMBER
        pub case BOOLEAN
        pub case DATE
        pub case DATE_TIME
        pub case URL
        pub case URL_WITH_HASH
        pub case GEO_POINT
    }

    // a meta data field of variable type
    //
    pub struct MetadataField {
        pub let type: MetadataFieldType
        pub let value: AnyStruct

        init(_ type: MetadataFieldType, _ value: AnyStruct) {
            self.type = type
            self.value = value
        }

        pub fun getMimeValue(): Mime? {
            if self.type != MetadataFieldType.MIME {
                return nil
            }
            return self.value as? Mime
        }

        pub fun getStringValue(): String? {
            if self.type != MetadataFieldType.STRING {
                return nil
            }
            return self.value as? String
        }

        pub fun getNumberValue(): String? {
            if self.type != MetadataFieldType.NUMBER {
                return nil
            }
            return self.value as? String
        }

        pub fun getBooleanValue(): Bool? {
            if self.type != MetadataFieldType.BOOLEAN {
                return nil
            }
            return self.value as? Bool
        }

        pub fun getURLValue(): String? {
            if self.type != MetadataFieldType.URL {
                return nil
            }
            return self.value as? String
        }

        pub fun getDateValue(): String? {
            if self.type != MetadataFieldType.DATE {
                return nil
            }
            return self.value as? String
        }

        pub fun getDateTimeValue(): String? {
            if self.type != MetadataFieldType.DATE_TIME {
                return nil
            }
            return self.value as? String
        }

        pub fun getURLWithHashValue(): URLWithHash? {
            if self.type != MetadataFieldType.URL_WITH_HASH {
                return nil
            }
            return self.value as? URLWithHash
        }

        pub fun getGeoPointValue(): GeoPoint? {
            if self.type != MetadataFieldType.GEO_POINT {
                return nil
            }
            return self.value as? GeoPoint
        }
    }

    // A url with a hash of the contents found at the url
    //
    pub struct URLWithHash {
        pub let url: String
        pub let hash: String?
        pub let hashAlgo: String?
        init(_ url: String, _ hash: String, _ hashAlgo: String?) {
            self.url = url
            self.hash = hash
            self.hashAlgo = hashAlgo
        }
    }

    // A geo point without any specific projection
    //
    pub struct GeoPoint {
        pub let lat: UFix64
        pub let lng: UFix64
        init(_ lat: UFix64, _ lng: UFix64) {
            self.lat = lat
            self.lng = lng
        }
    }

    // A piece of Mime content
    //
    pub struct Mime {
        pub let type: String
        pub let bytes: [UInt8]
        init(_ type: String, _ bytes: [UInt8]) {
            self.type = type
            self.bytes = bytes
        }
    }
}

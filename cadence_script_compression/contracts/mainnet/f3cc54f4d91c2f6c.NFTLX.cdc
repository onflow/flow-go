import NonFungibleToken from 0x1d7e57aa55817448

/** NFTLX

    NFTLX is the central smart contract for the NFTLX ecosystem.
    NFTLX is comprised of collections of NFTs which can be found in this
    smart contract. Collections may be created directly in this smart
    contract or through a project smart contract - such as Kicks.
*/
pub contract NFTLX {
    
    // -----------------------------------------------------------------------
    // Contract Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized ()
    pub event SetCreated          (id: UInt32)
    pub event SetRemoved          (id: UInt32)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------

    pub let SetsStoragePath:  StoragePath
    pub let SetsPrivatePath:  PrivatePath
    pub let SetsPublicPath:   PublicPath
    pub let AdminStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // Contract Fields
    // -----------------------------------------------------------------------

    // ---------------  Sets Fields  --------------- \\

    pub var nextSetID: UInt32
    access(contract) var setsCapability: Capability<auth &{UInt32: {ISet}}>

    // ---------------  Sets Getters  --------------- \\

    pub fun getSetsCount(): Int {
        let sets = self.setsCapability.borrow() ?? panic("Unable to load sets from capability")
        return sets.length
    }

    pub fun getSet(withID id: UInt32): &{ISet}? {
        pre {
            self.setsCapability.check() : "Invalid sets capability"
        }
        let sets = self.setsCapability.borrow() ?? panic("Unable to load sets from capability")
        return &sets[id] as &{ISet}
    }

    pub fun getSets(): [&{ISet}] {
        pre {
            self.setsCapability.check() : "Invalid sets capability"
        }
        let setsDict: &{UInt32: {ISet}} = self.setsCapability.borrow() ?? panic("Unable to load sets from capability")
        var sets: [&{ISet}] = []
        for key in setsDict.keys {
            let set = &setsDict[key] as &{ISet} 
            sets.append(set)
        }
        return sets
    }

    // -----------------------------------------------------------------------
    // Core Composite Type Definitions
    // -----------------------------------------------------------------------

    // The NFTLX ecosystem is comprised of 3 different types.
    // 1. Sets
    // 2. Class: stores information from which NFTs are made. 
    // 3. LXNFT
    // TODO: Fill in comments above

    /** ISet

        Sets store groups of Classes and organize NFTs into semantically
        meaningful groups. 
    */ 
    pub resource interface ISet {
        // Set's NFTLX unique identifier, primary datum for indexing and location a set
        pub let id: UInt32
        pub name: String
        // IPFS Content ID (CID) to a directory containing Set's media. CID is not prefixed
        // QUESTION: Should URI require base32 encoding - for web friendliness?
        pub URI: String

        pub fun getIDs(): [UInt32]
        pub fun getClasses(): [&{IClass}]
        pub fun getClass(atIndex index: Int): &{IClass}?
        pub fun getTotalSupply(): UInt32
    }

    /** IClass

        Classes store the characterstics of individual NFTs which cannot be modified
        by the owner of an NFT. It is a useful type to store duplicate information in
        one central location, rather than storing duplicate copies per NFT. For one-of-
        many NFTs, a Class will have multiple NFT instance; for one-of-one NFTs, only
        one copy will ever be instantiated.
    */ 
    pub resource interface IClass {
        pub let id: UInt32
        pub name: String
        pub URI: String
        pub numberMinted: UInt32
        pub let maxSupply: UInt32?

        pub fun getIDs(): [UInt32]
        pub fun getMetadata(): {String: AnyStruct}
    }

    /** ILXNFT

        Luxe NFTs have a conformance requirement of NFT.INFT - which unfortunately
        cannot be codified here until supported by Cadence. LXNFTs extend standard
        NFTs with information specific to NFTLX, such as the set, class, and instance
        identifiers for the NFT. Additionally, LXNFTs can store their metadata
        in the standard metadata field.
    */ 
    pub resource interface ILXNFT {
        // -----  INFT Conformance  -----
        pub let id: UInt64
  
        // -----  ILXNFT Fields  -----

        pub let setID: UInt32
        pub let classID: UInt32
        pub let instanceID: UInt32 // NOTE: To distinguish NFTs within same class. Assigned in incremental order. 

        pub fun getMetadata(): {String: AnyStruct}
    }


    // -----------------------------------------------------------------------
    // Priveleged Type Definitions
    // -----------------------------------------------------------------------

    /** Admin

        Admin is the universal NFTLX Administrator who is privileged with the 
        creation of new collections, deletion of collection, and general 
        maintenance in the ecosystem.
    */
    pub resource Admin {
        
        pub fun addNewSet(set: @{ISet}) {
            var sets <- NFTLX.account.load<@{UInt32: {ISet}}>(from: NFTLX.SetsStoragePath) ?? panic("Unable to load sets from storage.")
            let old <- sets.insert(key: set.id, <-set)
            NFTLX.account.save(<-sets, to: NFTLX.SetsStoragePath)
            destroy old

            emit SetCreated(id: NFTLX.nextSetID)
            NFTLX.nextSetID = NFTLX.nextSetID + 1
        }

        pub fun removeSet(withID id: UInt32) {
            var sets <- NFTLX.account.load<@{UInt32: {ISet}}>(from: NFTLX.SetsStoragePath) ?? panic("Unable to load sets from storage.")
            let old <- sets.remove(key: id)
            NFTLX.account.save(<-sets, to: NFTLX.SetsStoragePath)
            destroy old

            emit SetRemoved(id: id)
        }

        pub fun createNewAdmin(): @Admin {
            return <- create Admin()
        }
    }


    // -----------------------------------------------------------------------
    // Initializer and Setup Functions
    // -----------------------------------------------------------------------

    init() {
        // 1. Set Storage Locations
        self.SetsStoragePath = /storage/NFTLXSetsRegistry
        self.SetsPrivatePath = /private/NFTLXSetsRegistry
        self.SetsPublicPath = /public/NFTLXSetsRegistry
        self.AdminStoragePath = /storage/NFTLXAdmin

        // 2. See if old admin resource exists in storage, if so, delete to start clean.
        if let oldAdmin <- self.account.load<@Admin>(from: self.AdminStoragePath) {
            destroy oldAdmin
        }
        // Init and save new admin resource
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        // 3. If sets do not exist in storage, create and save
        if self.account.borrow<auth &{UInt32: {ISet}}>(from: self.SetsStoragePath) == nil {
            let sets: @{UInt32: {ISet}} <- {}
            self.account.save(<-sets, to: self.SetsStoragePath)
        }

        // Create an authorized capability from storage to private.
        let setsCap = self.account.link<auth &{UInt32: {ISet}}>(
            self.SetsPrivatePath,
            target: self.SetsStoragePath
        ) ?? panic("Unable to links sets from storage to private")
        // QUESTION: Is this ok to have as auth type or will it allow for outside users to cast to restricted type?
        self.account.link<&{UInt32: {ISet}}>(
            self.SetsPublicPath,
            target: self.SetsStoragePath
        ) ?? panic("Unable to links sets from storage to public")

        // Initialize set collection fields
        self.nextSetID = 0
        self.setsCapability = setsCap

        // Notify creation🚀
        emit ContractInitialized()
    }
}
 
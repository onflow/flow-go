import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract MiamiNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // MiamiNFT contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Miami contract is created
    pub event ContractInitialized()

    // Emitted when a new MiamiData struct is created
    pub event MiamiDataCreated(miamiDataID: UInt32, name: String, description: String, mainVideo: String, season: String, creator: Address)

    // Emitted when a Miami is minted
    pub event MiamiMinted(miamiID: UInt64, miamiDataID: UInt32, serialNumber: UInt32)

    // Emitted when the contract's royalty percentage is changed
    pub event RoyaltyPercentageChanged(newRoyaltyPercentage: UFix64)

    pub event MiamiDataIDRetired(miamiDataID: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a Miami is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Emitted when a Miami is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Miami is destroyed
    pub event MiamiDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // contract-level fields.      
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Contains standard storage and public paths of resources
    pub let CollectionStoragePath: StoragePath

    pub let CollectionPublicPath: PublicPath

    pub let AdminStoragePath: StoragePath

    // Variable size dictionary of Miami structs
    access(self) var miamiDatas: {UInt32: MiamiData}

    // Dictionary with MiamiDataID as key and number of NFTs with MiamiDataID are minted
    access(self) var numberMintedPerMiami: {UInt32: UInt32}

    // Dictionary of miamiDataID to  whether they are retired
    access(self) var isMiamiDataRetired: {UInt32: Bool}

    // Keeps track of how many unique MiamiData's are created
    pub var nextMiamiDataID: UInt32

    pub var royaltyPercentage: UFix64

    pub var totalSupply: UInt64

    pub struct MiamiData {

        // The unique ID for the Miami Data
        pub let miamiDataID: UInt32

        pub let name: String

        pub let description: String  

        //stores link to video
        pub let mainVideo: String

        pub let season: String

        pub let creator: Address

        init(
            name: String,
            description: String,
            mainVideo: String,
            season: String,
            creator: Address
        ){
            self.miamiDataID = MiamiNFT.nextMiamiDataID
            self.name = name
            self.description = description
            self.mainVideo = mainVideo
            self.season = season
            self.creator = creator

            MiamiNFT.isMiamiDataRetired[self.miamiDataID] = false

            // Increment the ID so that it isn't used again
            MiamiNFT.nextMiamiDataID = MiamiNFT.nextMiamiDataID + 1 as UInt32

            emit MiamiDataCreated(miamiDataID: self.miamiDataID, name: self.name, description: self.description, mainVideo: self.mainVideo, season: self.season, creator: self.creator)
        }
    }
    
    pub struct Miami {

        // The ID of the MiamiData that the Miami references
        pub let miamiDataID: UInt32

        // The N'th NFT with 'MiamiDataID' minted
        pub let serialNumber: UInt32

        init(miamiDataID: UInt32) {
            self.miamiDataID = miamiDataID

            // Increment the ID so that it isn't used again
            MiamiNFT.numberMintedPerMiami[miamiDataID] = MiamiNFT.numberMintedPerMiami[miamiDataID]! + 1 as UInt32

            self.serialNumber = MiamiNFT.numberMintedPerMiami[miamiDataID]!
        }
    }    



    // The resource that represents the Miami NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Miami ID
        pub let id: UInt64

        // struct of Miami
        pub let miami: Miami

        // Royalty capability which NFT will use
        pub let royaltyVault: Capability<&{FungibleToken.Receiver}> 


        init(serialNumber: UInt32, miamiDataID: UInt32, royaltyVault: Capability<&{FungibleToken.Receiver}> ) {
            MiamiNFT.totalSupply = MiamiNFT.totalSupply + 1 as UInt64
            
            self.id = MiamiNFT.totalSupply

            self.miami = Miami(miamiDataID: miamiDataID)

            self.royaltyVault = royaltyVault          

            // Emitted when a Miami is minted
            emit MiamiMinted(miamiID: self.id, miamiDataID: miamiDataID, serialNumber: serialNumber)
        }

        destroy() {
            emit MiamiDestroyed(id: self.id)
        }

    }

    // Admin is a special authorization resource that
    // allows the owner to perform important functions to modify the 
    // various aspects of the Miami and NFTs
    //
    pub resource Admin {

        pub fun createMiamiData(name: String, description: String, mainVideo: String, season: String, creator: Address): UInt32 {
            // Create the new MiamiData
            var newMiami = MiamiData(
                name: name,
                description: description,
                mainVideo: mainVideo,
                season: season,
                creator: creator
                
            )

            let newID = newMiami.miamiDataID

            // Store it in the contract storage
            MiamiNFT.miamiDatas[newID] = newMiami

            MiamiNFT.numberMintedPerMiami[newID] = 0 as UInt32

            return newID
        }
        
        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // Mint the new Miami
        pub fun mintNFT(miamiDataID: UInt32, royaltyVault: Capability<&{FungibleToken.Receiver}> ): @NFT {
            pre {
                royaltyVault.check():
                    "Royalty capability is invalid!"
            }

            if (MiamiNFT.isMiamiDataRetired[miamiDataID]! == nil) {
                panic("Cannot mint Miami. miamiData not found")
            }

            if (MiamiNFT.isMiamiDataRetired[miamiDataID]!) {
                panic("Cannot mint miami. miamiDataID retired")
            }

            let numInMiami = MiamiNFT.numberMintedPerMiami[miamiDataID]??
                panic("Cannot mint Miami. miamiData not found")

            let newMiami: @NFT <- create NFT(serialNumber: numInMiami + 1, miamiDataID: miamiDataID, royaltyVault: royaltyVault)

            return <-newMiami
        }

        pub fun batchMintNFT(miamiDataID: UInt32, royaltyVault: Capability<&{FungibleToken.Receiver}> , quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(miamiDataID: miamiDataID, royaltyVault: royaltyVault))
                i = i + 1 as UInt64
            }

            return <-newCollection
        }

        // Change the royalty percentage of the contract
        pub fun changeRoyaltyPercentage(newRoyaltyPercentage: UFix64) {
            MiamiNFT.royaltyPercentage = newRoyaltyPercentage
            
            emit RoyaltyPercentageChanged(newRoyaltyPercentage: newRoyaltyPercentage)
        }

        // Retire miamiData so that it cannot be used to mint anymore
        pub fun retireMiamiData(miamiDataID: UInt32) {           
            pre {
                MiamiNFT.isMiamiDataRetired[miamiDataID] != nil: "Cannot retire Miami: Miami doesn't exist!"
            }

            if !MiamiNFT.isMiamiDataRetired[miamiDataID]! {
                MiamiNFT.isMiamiDataRetired[miamiDataID] = true

                emit MiamiDataIDRetired(miamiDataID: miamiDataID)
            }
        }
    }

    // This is the interface users can cast their Miami Collection as
    // to allow others to deposit into their Collection. It also allows for reading
    // the IDs of Miami in the Collection.
    pub resource interface MiamiCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMiami(id: UInt64): &MiamiNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Miami reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: MiamiCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Miami conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Miami from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Miami does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Miami
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a Miami and adds it to the Collections dictionary
        //
        // Parameters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            // Cast the deposited token as NFT to make sure
            // it is the correct type
            let token <- token as! @MiamiNFT.NFT

            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token that was "removed"
            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            // Get an array of the IDs to be deposited
            let keys = tokens.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // getIDs returns an array of the IDs that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to a Miami in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not an specific data. Please use borrowMiami to 
        // read Miami data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowMiami(id: UInt64): &MiamiNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MiamiNFT.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed!
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // Miami contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Miami in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create MiamiNFT.Collection()
    }

    // get dictionary of numberMintedPerMiami
    pub fun getNumberMintedPerMiami(): {UInt32: UInt32} {
        return MiamiNFT.numberMintedPerMiami
    }

    // get how many Miamis with miamiDataID are minted 
    pub fun getMiamiNumberMinted(id: UInt32): UInt32 {
        let numberMinted = MiamiNFT.numberMintedPerMiami[id]??
            panic("miamiDataID not found")
        return numberMinted
    }

    // get the miamiData of a specific id
    pub fun getMiamiData(id: UInt32): MiamiData {
        let miamiData = MiamiNFT.miamiDatas[id]??
            panic("miamiDataID not found")
        return miamiData
    }

    // get all miamiDatas created
    pub fun getMiamiDatas(): {UInt32: MiamiData} {
        return MiamiNFT.miamiDatas
    }

    pub fun getMiamiDatasRetired(): {UInt32: Bool} { 
        return MiamiNFT.isMiamiDataRetired
    }

    pub fun getMiamiDataRetired(miamiDataID: UInt32): Bool { 
        let isMiamiDataRetired = MiamiNFT.isMiamiDataRetired[miamiDataID]??
            panic("miamiDataID not found")
        return isMiamiDataRetired
    }

    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.miamiDatas = {}
        self.numberMintedPerMiami = {}
        self.nextMiamiDataID = 1
        self.royaltyPercentage = 0.10
        self.isMiamiDataRetired = {}
        self.totalSupply = 0
        self.CollectionPublicPath = /public/MiamiCollection001
        self.CollectionStoragePath = /storage/MiamiCollection001
        self.AdminStoragePath = /storage/MiamiAdmin001

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{MiamiCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
 
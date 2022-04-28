import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FBRC from 0xfc91de5e6566cc7c

pub contract GarmentNFT: NonFungibleToken {

    // -----------------------------------------------------------------------
    // GarmentNFT contract Events
    // -----------------------------------------------------------------------

    // Emitted when the Garment contract is created
    pub event ContractInitialized()

    // Emitted when a new GarmentData struct is created
    pub event GarmentDataCreated(garmentDataID: UInt32, mainImage: String, images: [String], name: String, artist: String, description: String)

    // Emitted when a Garment is minted
    pub event GarmentMinted(garmentID: UInt64, garmentDataID: UInt32, serialNumber: UInt32)

    // Emitted when the contract's royalty percentage is changed
    pub event RoyaltyPercentageChanged(newRoyaltyPercentage: UFix64)

    pub event GarmentDataIDRetired(garmentDataID: UInt32)

    // Events for Collection-related actions
    //
    // Emitted when a Garment is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Emitted when a Garment is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Garment is destroyed
    pub event GarmentDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // contract-level fields.      
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Contains standard storage and public paths of resources
    pub let CollectionStoragePath: StoragePath

    pub let CollectionPublicPath: PublicPath

    pub let AdminStoragePath: StoragePath

    // Variable size dictionary of Garment structs
    access(self) var garmentDatas: {UInt32: GarmentData}

    // Dictionary with GarmentDataID as key and number of NFTs with GarmentDataID are minted
    access(self) var numberMintedPerGarment: {UInt32: UInt32}

    // Dictionary of garmentDataID to  whether they are retired
    access(self) var isGarmentDataRetired: {UInt32: Bool}

    // Keeps track of how many unique GarmentData's are created
    pub var nextGarmentDataID: UInt32

    pub var royaltyPercentage: UFix64

    pub var totalSupply: UInt64

    pub struct GarmentData {

        // The unique ID for the Garment Data
        pub let garmentDataID: UInt32

        //stores link to image
        pub let mainImage: String
        //stores link to supporting images
        pub let images: [String]
        pub let name: String
        pub let artist: String
        //description of design
        pub let description: String

        init(
            mainImage: String, 
            images: [String],
            name: String,
            artist: String, 
            description: String,
        ){
            self.garmentDataID = GarmentNFT.nextGarmentDataID
            self.mainImage = mainImage
            self.images = images
            self.name = name
            self.artist = artist
            self.description = description

            GarmentNFT.isGarmentDataRetired[self.garmentDataID] = false

            // Increment the ID so that it isn't used again
            GarmentNFT.nextGarmentDataID = GarmentNFT.nextGarmentDataID + 1 as UInt32

            emit GarmentDataCreated(garmentDataID: self.garmentDataID, mainImage: self.mainImage, images: self.images, name: self.name, artist: self.artist, description: self.description)
        }
    }
    
    pub struct Garment {

        // The ID of the GarmentData that the Garment references
        pub let garmentDataID: UInt32

        // The N'th NFT with 'GarmentDataID' minted
        pub let serialNumber: UInt32

        init(garmentDataID: UInt32) {
            self.garmentDataID = garmentDataID

            // Increment the ID so that it isn't used again
            GarmentNFT.numberMintedPerGarment[garmentDataID] = GarmentNFT.numberMintedPerGarment[garmentDataID]! + 1 as UInt32

            self.serialNumber = GarmentNFT.numberMintedPerGarment[garmentDataID]!
        }
    }    

    // The resource that represents the Garment NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Garment ID
        pub let id: UInt64

        // struct of Garment
        pub let garment: Garment

        // Royalty capability which NFT will use
        pub let royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>

        init(serialNumber: UInt32, garmentDataID: UInt32, royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>) {
            GarmentNFT.totalSupply = GarmentNFT.totalSupply + 1 as UInt64
            
            self.id = GarmentNFT.totalSupply

            self.garment = Garment(garmentDataID: garmentDataID)

            self.royaltyVault = royaltyVault          

            // Emitted when a Garment is minted
            emit GarmentMinted(garmentID: self.id, garmentDataID: garmentDataID, serialNumber: serialNumber)
        }

        destroy() {
            emit GarmentDestroyed(id: self.id)
        }

    }

    // Admin is a special authorization resource that
    // allows the owner to perform important functions to modify the 
    // various aspects of the Garment and NFTs
    //
    pub resource Admin {

        pub fun createGarmentData(
            mainImage: String, 
            images: [String],
            name: String,
            artist: String, 
            description: String,
        ): UInt32 {
            // Create the new GarmentData
            var newGarment = GarmentData(
                mainImage: mainImage, 
                images: images,
                name: name,
                artist: artist, 
                description: description,
            )

            let newID = newGarment.garmentDataID

            // Store it in the contract storage
            GarmentNFT.garmentDatas[newID] = newGarment

            GarmentNFT.numberMintedPerGarment[newID] = 0 as UInt32

            return newID
        }
        
        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        // Mint the new Garment
        pub fun mintNFT(garmentDataID: UInt32, royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>): @NFT {
            pre {
                royaltyVault.check():
                    "Royalty capability is invalid!"
            }

            if (GarmentNFT.isGarmentDataRetired[garmentDataID]! == nil) {
                panic("Cannot mint Garment. garmentData not found")
            }

            if (GarmentNFT.isGarmentDataRetired[garmentDataID]!) {
                panic("Cannot mint garment. garmentDataID retired")
            }

            let numInGarment = GarmentNFT.numberMintedPerGarment[garmentDataID]??
                panic("Cannot mint Garment. garmentData not found")

            let newGarment: @NFT <- create NFT(serialNumber: numInGarment + 1, garmentDataID: garmentDataID, royaltyVault: royaltyVault)

            return <-newGarment
        }

        pub fun batchMintNFT(garmentDataID: UInt32, royaltyVault: Capability<&FBRC.Vault{FungibleToken.Receiver}>, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(garmentDataID: garmentDataID, royaltyVault: royaltyVault))
                i = i + 1 as UInt64
            }

            return <-newCollection
        }

        // Change the royalty percentage of the contract
        pub fun changeRoyaltyPercentage(newRoyaltyPercentage: UFix64) {
            GarmentNFT.royaltyPercentage = newRoyaltyPercentage
            
            emit RoyaltyPercentageChanged(newRoyaltyPercentage: newRoyaltyPercentage)
        }

        // Retire garmentData so that it cannot be used to mint anymore
        pub fun retireGarmentData(garmentDataID: UInt32) {           
            pre {
                GarmentNFT.isGarmentDataRetired[garmentDataID] != nil: "Cannot retire Garment: Garment doesn't exist!"
            }

            if !GarmentNFT.isGarmentDataRetired[garmentDataID]! {
                GarmentNFT.isGarmentDataRetired[garmentDataID] = true

                emit GarmentDataIDRetired(garmentDataID: garmentDataID)
            }
        }
    }

    // This is the interface users can cast their Garment Collection as
    // to allow others to deposit into their Collection. It also allows for reading
    // the IDs of Garment in the Collection.
    pub resource interface GarmentCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowGarment(id: UInt64): &GarmentNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Garment reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs 
    // will store in their account to manage their NFTS
    //
    pub resource Collection: GarmentCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Garment conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Garment from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Garment does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Garment
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

        // deposit takes a Garment and adds it to the Collections dictionary
        //
        // Parameters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            // Cast the deposited token as NFT to make sure
            // it is the correct type
            let token <- token as! @GarmentNFT.NFT

            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token tGarment was "removed"
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

        // borrowNFT Returns a borrowed reference to a Garment in the Collection
        // so tGarment the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        // Note: This only allows the caller to read the ID of the NFT,
        // not an specific data. Please use borrowGarment to 
        // read Garment data.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowGarment(id: UInt64): &GarmentNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &GarmentNFT.NFT
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
    // Garment contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Garment in transactions.
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create GarmentNFT.Collection()
    }

    // get dictionary of numberMintedPerGarment
    pub fun getNumberMintedPerGarment(): {UInt32: UInt32} {
        return GarmentNFT.numberMintedPerGarment
    }

    // get how many Garments with garmentDataID are minted 
    pub fun getGarmentNumberMinted(id: UInt32): UInt32 {
        let numberMinted = GarmentNFT.numberMintedPerGarment[id]??
            panic("garmentDataID not found")
        return numberMinted
    }

    // get the garmentData of a specific id
    pub fun getGarmentData(id: UInt32): GarmentData {
        let garmentData = GarmentNFT.garmentDatas[id]??
            panic("garmentDataID not found")
        return garmentData
    }

    // get all garmentDatas created
    pub fun getGarmentDatas(): {UInt32: GarmentData} {
        return GarmentNFT.garmentDatas
    }

    pub fun getGarmentDatasRetired(): {UInt32: Bool} { 
        return GarmentNFT.isGarmentDataRetired
    }

    pub fun getGarmentDataRetired(garmentDataID: UInt32): Bool { 
        let isGarmentDataRetired = GarmentNFT.isGarmentDataRetired[garmentDataID]??
            panic("garmentDataID not found")
        return isGarmentDataRetired
    }

    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // Initialize contract fields
        self.garmentDatas = {}
        self.numberMintedPerGarment = {}
        self.nextGarmentDataID = 1
        self.royaltyPercentage = 0.10
        self.isGarmentDataRetired = {}
        self.totalSupply = 0
        self.CollectionPublicPath = /public/GarmentCollection20
        self.CollectionStoragePath = /storage/GarmentCollection20
        self.AdminStoragePath = /storage/GarmentAdmin20

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{GarmentCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
 
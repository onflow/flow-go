import NonFungibleToken from 0x1d7e57aa55817448

/*
    Description: Central Smart Contract for the first generation of Zeedle NFTs

    Zeedles are cute little nature-inspired monsters that grow with the real world weather.
    They are the main characters of Zeedz, the first play-for-purpose game where players 
    reduce global carbon emissions by growing Zeedles. 
    
    This smart contract encompasses the main functionality for the first generation
    of Zeedle NFTs. 

    Oriented much on the standard NFT contract, each Zeedle NFT has a certain typeID,
    which is the type of Zeedle - e.g. "Baby Aloe Vera" or "Ginger Biggy". A contract-level
    dictionary takes account of the different quentities that have been minted per Zeedle type.

    Different types also imply different rarities, and these are also hardcoded inside 
    the given Zeedle NFT in order to allow the direct querying of the Zeedle's rarity 
    in external applications and wallets.

    Each batch-minting of Zeedles is resembled by an edition number, with the community pre-sale 
    being the first-ever edition (0). This way, each Zeedle can be traced back to the edition it
    was created in, and the number of minted Zeedles of that type in the specific edition.

    Many of the in-game purchases lead to real-world donations to NGOs focused on climate action. 
    The carbonOffset attribute of a Zeedle proves the impact the in-game purchases related to this Zeedle
    have already made with regards to reducing greenhouse gases. This value is computed by taking the 
    current dollar-value of each purchase at the time of the purchase, and applying the dollar-to-CO2-offset
    formular of the current climate action partner. 
*/
pub contract ZeedzINO: NonFungibleToken {

    //  Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, name: String, description: String, typeID: UInt32, serialNumber: String, edition: UInt32, rarity: String)
    pub event Burned(id: UInt64, from: Address?)
    pub event Offset(id: UInt64, amount: UInt64)

    //  Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let AdminPrivatePath: PrivatePath

    pub var totalSupply: UInt64

    access(contract) var numberMintedPerType: {UInt32: UInt64}

    pub resource NFT: NonFungibleToken.INFT {
        //  The token's ID
        pub let id: UInt64
        //  The memorable short name for the Zeedle, e.g. “Baby Aloe"
        pub let name: String 
        //  A short description of the Zeedle's type
        pub let description: String 
        //  Number id of the Zeedle type -> e.g "1 = Ginger Biggy, 2 = Baby Aloe, etc”
        pub let typeID: UInt32
        //  A Zeedle's unique serial number from the Zeedle's edition 
        pub let serialNumber: String
        //  Number id of the Zeedle's edition -> e.g "1 = first edition, 2 = second edition, etc"  
        pub let edition: UInt32 
        //  The total number of Zeedle's minted in this edition
        pub let editionCap: UInt32 
        //  The Zeedle's evolutionary stage 
        pub let evolutionStage: UInt32
        //  The Zeedle's rarity -> e.g "RARE, COMMON, LEGENDARY, etc" 
        pub let rarity: String
        //  URI to the image of the Zeedle 
        pub let imageURI: String
        //  The total amount this Zeedle has contributed to offsetting CO2 emissions
        pub var carbonOffset: UInt64

        init(initID: UInt64, initName: String, initDescription: String, initTypeID: UInt32, initSerialNumber: String, initEdition: UInt32, initEditionCap: UInt32, initEvolutionStage: UInt32, initRarity: String, initImageURI: String) {
            self.id = initID
            self.name = initName
            self.description = initDescription
            self.typeID = initTypeID
            self.serialNumber = initSerialNumber
            self.edition = initEdition
            self.editionCap = initEditionCap
            self.evolutionStage = initEvolutionStage
            self.rarity = initRarity
            self.imageURI = initImageURI
            self.carbonOffset = 0
        }

        pub fun getMetadata(): {String: AnyStruct} {
            return {"name": self.name, "description": self.description, "typeID": self.typeID, "serialNumber": self.serialNumber, "edition": self.edition, "editionCap": self.editionCap, "evolutionStage": self.evolutionStage, "rarity": self.rarity, "imageURI": self.imageURI, "carbonOffset": self.carbonOffset}
        }

        access(contract) fun increaseOffset(amount: UInt64) {
            self.carbonOffset = self.carbonOffset + amount
        }
    }

    // 
    //  This is the interface that users can cast their Zeedz Collection as
    //  to allow others to deposit Zeedles into their Collection. It also allows for reading
    //  the details of Zeedles in the Collection.
    // 
    pub resource interface ZeedzCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowZeedle(id: UInt64): &ZeedzINO.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Zeedle reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // 
    //  This is the interface that users can cast their Zeedz Collection as
    //  to allow themselves to call the burn function on their own collection.
    // 
    pub resource interface ZeedzCollectionPrivate {
        pub fun burn(burnID: UInt64)
    }

    //
    //  A collection of Zeedz NFTs owned by an account.
    //   
    pub resource Collection: ZeedzCollectionPublic, ZeedzCollectionPrivate, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Not able to find specified NFT within the owner's collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun burn(burnID: UInt64){
            let token <- self.ownedNFTs.remove(key: burnID) ?? panic("Not able to find specified NFT within the owner's collection")
            let zeedle <- token as! @ZeedzINO.NFT

            //  reduce numberOfMinterPerType
            ZeedzINO.numberMintedPerType[zeedle.typeID] = ZeedzINO.numberMintedPerType[zeedle.typeID]! - (1 as UInt64)

            destroy zeedle
            emit Burned(id: burnID, from: self.owner?.address)
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @ZeedzINO.NFT
            let id: UInt64 = token.id
            //  add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        //
        //  Returns an array of the IDs that are in the collection.
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        //
        //  Gets a reference to an NFT in the collection
        //  so that the caller can read its metadata and call its methods.
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        //
        //  Gets a reference to an NFT in the collection as a Zeed,
        //  exposing all of its fields
        //  this is safe as there are no functions that can be called on the Zeed.
        //
        pub fun borrowZeedle(id: UInt64): &ZeedzINO.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ZeedzINO.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    //
    //  Public function that anyone can call to create a new empty collection.
    // 
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    //
    //  The Administrator resource that an Administrator or something similar 
    //  would own to be able to mint & level-up NFT's.
    //
    pub resource Administrator {

        //
        //  Mints a new NFT with a new ID
        //  and deposit it in the recipients collection using their collection reference.
        //
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, name: String, description: String, typeID: UInt32, serialNumber: String, edition: UInt32, editionCap: UInt32, evolutionStage: UInt32, rarity: String, imageURI: String) {
            recipient.deposit(token: <-create ZeedzINO.NFT(initID: ZeedzINO.totalSupply, initName: name, initDescription: description, initTypeID: typeID, initSerialNumber: serialNumber, initEdition: edition, initEdition: editionCap, initEvolutionStage: evolutionStage, initRarity: rarity, initImageURI: imageURI))
            emit Minted(id: ZeedzINO.totalSupply, name: name, description: description, typeID: typeID, serialNumber: serialNumber, edition: edition, rarity: rarity)

            // increase numberOfMinterPerType and totalSupply
            ZeedzINO.totalSupply = ZeedzINO.totalSupply + (1 as UInt64)
            if ZeedzINO.numberMintedPerType[typeID] == nil {
                ZeedzINO.numberMintedPerType[typeID] = 1 
            } else {
                ZeedzINO.numberMintedPerType[typeID] = ZeedzINO.numberMintedPerType[typeID]! + (1 as UInt64)
            }
        }

        //
        //  Increase the Zeedle's total carbon offset by the given amount
        //
        pub fun increaseOffset(zeedleRef: &ZeedzINO.NFT, amount: UInt64) {
            zeedleRef.increaseOffset(amount: amount)
            emit Offset(id: zeedleRef.id, amount: amount)
        }
    }

    //
    //  Get a reference to a Zeedle from an account's Collection, if available.
    //  If an account does not have a Zeedz.Collection, panic.
    //  If it has a collection but does not contain the zeedleId, return nil.
    //  If it has a collection and that collection contains the zeedleId, return a reference to that.
    //
    pub fun fetch(_ from: Address, zeedleID: UInt64): &ZeedzINO.NFT? {
        let collection = getAccount(from)
            .getCapability(ZeedzINO.CollectionPublicPath)!
            .borrow<&ZeedzINO.Collection{ZeedzINO.ZeedzCollectionPublic}>()
            ?? panic("Couldn't get collection")
        return collection.borrowZeedle(id: zeedleID)
    }

    // 
    //  Returns the number of minted Zeedles for each Zeedle type.
    //
    pub fun getMintedPerType(): {UInt32: UInt64} {
        return self.numberMintedPerType
    }


    init() {
        self.CollectionStoragePath = /storage/ZeedzINOCollection
        self.CollectionPublicPath = /public/ZeedzINOCollection
        self.AdminStoragePath = /storage/ZeedzINOMinter
        self.AdminPrivatePath= /private/ZeedzINOAdminPrivate

        self.totalSupply = 0
        self.numberMintedPerType = {}

        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
        self.account.link<&Administrator>(self.AdminPrivatePath, target: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
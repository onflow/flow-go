// Blockletes.cdc

import NonFungibleToken from 0x1d7e57aa55817448

pub contract Blockletes_NFT_V2: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)
    pub event NewSeasonStarted(newCurrentSeason: UInt16)
    pub event TokenBaseURISet(newBaseURI: String)


    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let AdminStoragePath: StoragePath
    pub let AdminPrivatePath: PrivatePath

    // totalSupply
    // The total number of Blockletes that have been minted
    //
    pub var totalSupply: UInt64

    // currentSeason
    // The current season of Blockletes
    //
    pub var currentSeason: UInt16

    // baseURI
    //
    pub var baseURI: String

    // NFT
    // A Blocklete as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        pub let season: UInt16

        // The Blocklete token's attributes - a Dictionary
        // of key-value pairs describing all intrinsic attributes
        // which make each Blocklete NFT unique
        // 
        // This is the initial attribute mapping for the Blockletes:
        //[0] = dnaversion
        //[1] = power
        //[2] = accuracy
        //[3] = composure
        //[4] = stamina
        //[5] = powerpeak
        //[6] = accuracypeak
        //[7] = composurepeak
        //[8] = staminapeak
        //[9] = reserved (Species and Sex)
        //[10] = skin_tone
        //[11] = head
        //[12] = hair
        //[13] = hairline
        //[14] = hair_color
        //[15] = face
        //[16] = ears
        //[17] = body
        //[18] = body_texture
        //[19] = bottom
        //[20] = bottom_texture
        //[21] = shoes
        //[22] = shoes_texture
        //[23] = hat
        //[24] = hat_texture
        //[25] = gloves
        //[26] = glasses
        //[27] = glasses_texture
        //[28] = accessories
        //[29] = pose
        //

        // New attibutes, skills, or attire may be added 
        // dynamically to this list as future Blocklete features
        // and games are added to the ecosystem. Blockletes can
        // gain additional attributes, skills or attire. Their
        // abilities can be upgraded through gameplay or training
        // For example, a Blocklete's power value can be increased
        // through training

        pub let attributes: {UInt16: UInt16}

        // initializer
        //
        init(initID: UInt64, attributes: {UInt16: UInt16}) {
            self.id = initID
            self.attributes = attributes
            self.season = Blockletes_NFT_V2.currentSeason
        }

        // Upgrade the Blocklete NFT's abilities by amounts specified
        // in the passed in by the abilities upgrade resource passed
        // in. The upgrade resource can only be created by authorized accounts,
        // such as an administrator or blocklete trainers in the training 
        // marketplace.
        pub fun upgrade(abilitiesUpgrade: @AbilitiesUpgradeBundle) {
            pre {
                abilitiesUpgrade != nil: 
                    "Cannot upgrade Blocklete until the upgrade bundle is created"
            }
            
            for key in abilitiesUpgrade.abilityUpgrades.keys {
                self.attributes[key] = self.attributes[key]! + abilitiesUpgrade.abilityUpgrades[key]!;
            }

            destroy abilitiesUpgrade
        }
    }

    pub resource AbilitiesUpgradeBundle {
        pub let abilityUpgrades: {UInt16: UInt16}

        // initializer
        //
        init (abilityUpgrades: {UInt16: UInt16}) {
            self.abilityUpgrades = abilityUpgrades
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important Blocklete 
    // functions for upgrades from the game, adding 
    // new Blocklete skills, increment to new season
    //
    pub resource Admin {

        // incrementSeason ends the current season by incrementing
        // the season number. Blockletes minted after this
        // will use the new season number
        //
        // Returns: The new season number
        //
        pub fun incrementSeason(): UInt16 {
            // End the current season and start a new one
            // by incrementing the TopShot season number
            Blockletes_NFT_V2.currentSeason = Blockletes_NFT_V2.currentSeason + (1 as UInt16)

            emit NewSeasonStarted(newCurrentSeason: Blockletes_NFT_V2.currentSeason)

            return Blockletes_NFT_V2.currentSeason
        }

        pub fun setBaseURI(newBaseURI: String) {
            Blockletes_NFT_V2.baseURI = newBaseURI
            emit TokenBaseURISet(newBaseURI: newBaseURI)
        }

        pub fun createAbilitiesUpgradeBundle(abilityUpgradeValues: {UInt16: UInt16}): @AbilitiesUpgradeBundle {
            return <-create AbilitiesUpgradeBundle(abilityUpgrades: abilityUpgradeValues)
        }
    }

    // This is the interface that users can cast their Blocklete Collection as
    // to allow others to deposit Blockletes into their Collection. It also allows for reading
    // the details of Blockletes in the Collection.
    pub resource interface BlockletesCollectionPublic {
        pub fun upgradeBlocklete(tokenId: UInt64, abilitiesUpgrade: @AbilitiesUpgradeBundle)
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowBlocklete(id: UInt64): &Blockletes_NFT_V2.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Blocklete reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Blocklete NFTs owned by an account
    //
    pub resource Collection: BlockletesCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // upgrade
        // increase one or more abilities of a blocklete according to the passed in
        // abilities upgrade resource. The abilities upgrade resource can only be created
        // by authorized roles, such as admins or trainers
        //
        pub fun upgradeBlocklete(tokenId: UInt64, abilitiesUpgrade: @AbilitiesUpgradeBundle) {
            let ref = &self.ownedNFTs[tokenId] as auth &NonFungibleToken.NFT
            let blockleteToken = ref as! &Blockletes_NFT_V2.NFT
            blockleteToken.upgrade(abilitiesUpgrade: <-abilitiesUpgrade)
        }

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // batchWithdraw withdraws multiple NFTs and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: The collection of withdrawn tokens
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

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Blockletes_NFT_V2.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

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

        // borrowBlocklete
        // Gets a reference to an NFT in the collection as a Blocklete,
        // exposing all of its fields (including the Blocklete attributes).
        // This is safe as there are no functions that can be called on the Blocklete.
        //
        pub fun borrowBlocklete(id: UInt64): &Blockletes_NFT_V2.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Blockletes_NFT_V2.NFT
            } else {
                return nil
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

    // NFTMinter
    // Resource that allows an admin to mint new NFTs
    //
    pub resource NFTMinter {

        // mintBlocklete
        // Mints a new Blocklete NFT with a new ID
        // and deposits it in the recipients collection using their collection reference
        //
        pub fun mintBlocklete(recipient: &{NonFungibleToken.CollectionPublic}, newAttributes: {UInt16: UInt16}) {
            emit Minted(id: Blockletes_NFT_V2.totalSupply)

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create Blockletes_NFT_V2.NFT(
                initID: Blockletes_NFT_V2.totalSupply,
                attributes: newAttributes
            ))

            Blockletes_NFT_V2.totalSupply = Blockletes_NFT_V2.totalSupply + (1 as UInt64)
        }
 
        // batchMintBlockletes
        // Mints multiple new Blocklete NFTs given a list of Blocklete attributes
        // and deposits the NFTs into the recipients collection using their collection reference
        //
        pub fun batchMintBlockletes(recipient: &{NonFungibleToken.CollectionPublic}, attributesList: [{UInt16: UInt16}]) {
            for attributes in attributesList {
                // deposit it in the recipient's account using their reference
                self.mintBlocklete(recipient: recipient, newAttributes: attributes);
            }
        }
    }

    // fetch
    // Get a reference to a Blocklete from an account's Collection, if available.
    // If an account does not have a Blockletes.Collection, panic.
    // If it has a collection but does not contain the blockleteId, return nil.
    // If it has a collection and that collection contains the blockleteId, return a reference to that.
    //
    pub fun fetch(_ from: Address, blockleteId: UInt64): &Blockletes_NFT_V2.NFT? {
        let collection = getAccount(from)
            .getCapability(Blockletes_NFT_V2.CollectionPublicPath)
            .borrow<&Blockletes_NFT_V2.Collection{Blockletes_NFT_V2.BlockletesCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Blockletes.Collection.borowBlocklete to get the correct blockleteId
        // (it checks it before returning it).
        return collection.borrowBlocklete(id: blockleteId)
    }

    // initializer
    //
    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/BlockletesCollection_NFT_V2
        self.CollectionPublicPath = /public/BlockletesCollection_NFT_V2
        self.MinterStoragePath = /storage/BlockletesMinter_NFT_V2
        self.AdminStoragePath = /storage/BlockletesAdmin_NFT_V2
        self.AdminPrivatePath = /private/BlockletesAdminUpgrade_NFT_V2

        // Initialize the total supply
        self.totalSupply = 0

        // Initialize the season number to 1. Season 1 was completed
        // on Ethereum. Once all tokens are migrated from Ethereum to Flow,
        // the contract can be incremented to Season 2
        self.currentSeason = 1

        self.baseURI = "https://www.blockletegames.com/golf/"

        // Create a Minter resource and save it to admin storage
        self.account.save(<-create NFTMinter(), to: self.MinterStoragePath)

        self.account.save(<-create Admin(), to: self.AdminStoragePath)

        self.account.link<&Blockletes_NFT_V2.Admin>(
            self.AdminPrivatePath,
            target: self.AdminStoragePath
        ) ?? panic("Could not get a capability to the admin")

        emit ContractInitialized()  
    }
}
import NonFungibleToken from 0x1d7e57aa55817448

// Gaia
// NFT an open NFT standard!
//
pub contract Gaia: NonFungibleToken {

    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event TemplateCreated(id: UInt64, metadata: {String:String})
    pub event SetCreated(setID: UInt64, name: String, description: String, website: String, imageURI: String, creator: Address, marketFee: UFix64)
    pub event SetAddedAllowedAccount(setID: UInt64, allowedAccount: Address)
    pub event SetRemovedAllowedAccount(setID: UInt64, allowedAccount: Address)
    pub event TemplateAddedToSet(setID: UInt64, templateID: UInt64)
    pub event TemplateLockedFromSet(setID: UInt64, templateID: UInt64, numNFTs: UInt64)
    pub event SetLocked(setID: UInt64)
    pub event Minted(assetID: UInt64, templateID: UInt64, setID: UInt64, mintNumber: UInt64)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // Variable size dictionary of Play structs
    access(self) var templateDatas: {UInt64: Template}

    // Variable size dictionary of SetData structs
    access(self) var setDatas: {UInt64: SetData}

    // Variable size dictionary of Set resources
    access(self) var sets: @{UInt64: Set}

    // totalSupply
    // The total number of Gaia that have been minted
    //
    pub var totalSupply: UInt64


    // The ID that is used to create Templates. 
    // Every time a Template is created, templateID is assigned 
    // to the new Template's ID and then is incremented by 1.
    pub var nextTemplateID: UInt64


    // The ID that is used to create Sets. Every time a Set is created
    // setID is assigned to the new set's ID and then is incremented by 1.
    pub var nextSetID: UInt64

    // -----------------------------------------------------------------------
    // Gaia contract-level Composite Type definitions
    // -----------------------------------------------------------------------
    // These are just *definitions* for Types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these Types
    // can be created by this contract that contains stored values.
    // -----------------------------------------------------------------------

    // Template is a Struct that holds metadata associated 
    // with a specific NFT template
    // NFTs will all reference a single template as the owner of
    // its metadata. The templates are publicly accessible, so anyone can
    // read the metadata associated with a specific template ID
    //
    pub struct Template {

        // The unique ID for the template
        pub let templateID: UInt64

        // Stores all the metadata about the template as a string mapping
        // This is not the long term way NFT metadata will be stored.
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Template metadata cannot be empty"
            }
            self.templateID = Gaia.nextTemplateID
            self.metadata = metadata

            // Increment the ID so that it isn't used again
            Gaia.nextTemplateID = Gaia.nextTemplateID + 1 as UInt64

            emit TemplateCreated(id: self.templateID, metadata: metadata)
        }
    }

    // A Set is a grouping of Templates that have occured in the real world
    // that make up a related group of collectibles, like sets of Magic cards.
    // A Template can exist in multiple different sets. 
    // SetData is a struct that is stored in a field of the contract.
    // Anyone can query the constant information
    // about a set by calling various getters located 
    // at the end of the contract. Only the admin has the ability 
    // to modify any data in the private Set resource.
    //
    pub struct SetData {

        // Unique ID for the Set
        pub let setID: UInt64

        // Name of the Set
        pub let name: String

        // Brief description of the Set
        pub let description: String

        // Set cover image
        pub let imageURI: String

        // Set website url
        pub let website: String

        // Set creator account address
        pub let creator: Address
        
        // Accounts allowed to mint
        access(self) let allowedAccounts: [Address]

        pub fun returnAllowedAccounts(): [Address] {
            return self.allowedAccounts
        }

        // Set marketplace fee
        pub let marketFee: UFix64

        init(name: String, description: String, website: String, imageURI: String, creator: Address, marketFee: UFix64) {
            pre {
                name.length > 0: "New set name cannot be empty"
                description.length > 0: "New set description cannot be empty"
                imageURI.length > 0: "New set imageURI cannot be empty"
                creator != nil: "Creator must not be nil"
                marketFee >= 0.0 && marketFee <= 0.15: "Market fee must be a number between 0.00 and 0.15"
            }
            
            self.setID = Gaia.nextSetID
            self.name = name
            self.description = description
            self.website = website
            self.imageURI = imageURI
            self.creator = creator
            self.allowedAccounts = [creator, Gaia.account.address]
            self.marketFee = marketFee

            // Increment the setID so that it isn't used again
            Gaia.nextSetID = Gaia.nextSetID + 1 as UInt64
            emit SetCreated(setID: self.setID, name: name, description: description, website: website, imageURI: imageURI, creator: creator, marketFee: marketFee)
        }

        pub fun addAllowedAccount(account: Address) {
            pre {
                !self.allowedAccounts.contains(account): "Account already allowed"
            }

            self.allowedAccounts.append(account)

            emit SetAddedAllowedAccount(setID: self.setID, allowedAccount: account)
        }

        pub fun removeAllowedAccount(account: Address) {
            pre {
                self.creator != account: "Cannot remove set creator"
                self.allowedAccounts.contains(account): "Not in allowed accounts"
            }

            var index = 0
            for acc in self.allowedAccounts {
                if (acc == account) {
                    self.allowedAccounts.remove(at: index)
                    break
                }
                index = index + 1
            }

            emit SetRemovedAllowedAccount(setID: self.setID, allowedAccount: account)
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // Templates from a set and mint NFTs.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods.
    //
    // The admin can add Templates to a Set so that the set can mint NFTs
    // that reference that template data.
    // The NFTs that are minted by a Set will be listed as belonging to
    // the Set that minted it, as well as the Template it references.
    // 
    // Admin can also lock Templates from the Set, meaning that the lockd
    // Template can no longer have NFTs minted from it.
    //
    // If the admin locks the Set, no more Templates can be added to it, but 
    // NFTs can still be minted.
    //
    // If lockAll() and lock() are called back-to-back, 
    // the Set is closed off forever and nothing more can be done with it.
    pub resource Set {

        // Unique ID for the set
        pub let setID: UInt64

        // Array of templates that are a part of this set.
        // When a template is added to the set, its ID gets appended here.
        // The ID does not get removed from this array when a templates is locked.
        pub var templates: [UInt64]

        // Map of template IDs that Indicates if a template in this Set can be minted.
        // When a templates is added to a Set, it is mapped to false (not locked).
        // When a templates is locked, this is set to true and cannot be changed.
        pub var lockedTemplates: {UInt64: Bool}

        // Indicates if the Set is currently locked.
        // When a Set is created, it is unlocked 
        // and templates are allowed to be added to it.
        // When a set is locked, templates cannot be added.
        // A Set can never be changed from locked to unlocked,
        // the decision to lock a Set it is final.
        // If a Set is locked, templates cannot be added, but
        // NFTs can still be minted from templates
        // that exist in the Set.
        pub var locked: Bool

        // Mapping of Template IDs that indicates the number of NFTs 
        // that have been minted for specific Templates in this Set.
        // When a NFT is minted, this value is stored in the NFT to
        // show its place in the Set, eg. 13 of 60.
        pub var numberMintedPerTemplate: {UInt64: UInt64}

        init(name: String, description: String, website: String, imageURI: String, creator: Address, marketFee: UFix64)
         {
            self.setID = Gaia.nextSetID
            self.templates = []
            self.lockedTemplates = {}
            self.locked = false
            self.numberMintedPerTemplate = {}
            // Create a new SetData for this Set and store it in contract storage
            Gaia.setDatas[self.setID] = SetData(name: name, description: description, website: website, imageURI: imageURI, creator: creator, marketFee: marketFee)
        }

        // addTemplate adds a template to the set
        //
        // Parameters: templateID: The ID of the template that is being added
        //
        // Pre-Conditions:
        // The template needs to be an existing template
        // The Set needs to be not locked
        // The template can't have already been added to the Set
        //
        pub fun addTemplate(templateID: UInt64) {
            pre {
                Gaia.templateDatas[templateID] != nil: "Cannot add the Template to Set: Template doesn't exist."
                !self.locked: "Cannot add the template to the Set after the set has been locked."
                self.numberMintedPerTemplate[templateID] == nil: "The template has already beed added to the set."
            }

            // Add the Play to the array of Plays
            self.templates.append(templateID)

            // Open the Play up for minting
            self.lockedTemplates[templateID] = false

            // Initialize the Moment count to zero
            self.numberMintedPerTemplate[templateID] = 0

            emit TemplateAddedToSet(setID: self.setID, templateID: templateID)
        }

        // addTemplates adds multiple templates to the Set
        //
        // Parameters: templateIDs: The IDs of the templates that are being added
        //
        pub fun addTemplates(templateIDs: [UInt64]) {
            for template in templateIDs {
                self.addTemplate(templateID: template)
            }
        }

        // retirePlay retires a Play from the Set so that it can't mint new Moments
        //
        // Parameters: playID: The ID of the Play that is being retired
        //
        // Pre-Conditions:
        // The Play is part of the Set and not retired (available for minting).
        // 
        pub fun lockTemplate(templateID: UInt64) {
            pre {
                self.lockedTemplates[templateID] != nil: "Cannot lock the template: Template doesn't exist in this set!"
            }

            if !self.lockedTemplates[templateID]! {
                self.lockedTemplates[templateID] = true

                emit TemplateLockedFromSet(setID: self.setID, templateID: templateID, numNFTs: self.numberMintedPerTemplate[templateID]!)
            }
        }

        // lockAll lock all the templates in the Set
        // Afterwards, none of the locked templates will be able to mint new NFTs
        //
        pub fun lockAll() {
            for template in self.templates {
                self.lockTemplate(templateID: template)
            }
        }

        // lock() locks the Set so that no more Templates can be added to it
        //
        // Pre-Conditions:
        // The Set should not be locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        // mintNFT mints a new NFT and returns the newly minted NFT
        // 
        // Parameters: templateID: The ID of the Template that the NFT references
        //
        // Pre-Conditions:
        // The Template must exist in the Set and be allowed to mint new NFTs
        //
        // Returns: The NFT that was minted
        // 
        pub fun mintNFT(templateID: UInt64): @NFT {
            pre {
                self.lockedTemplates[templateID] != nil: "Cannot mint the NFT: This template doesn't exist."
                !self.lockedTemplates[templateID]!: "Cannot mint the NFT from this template: This template has been locked."
            }

            // Gets the number of NFTs that have been minted for this Template
            // to use as this NFT's serial number
            let numInTemplate = self.numberMintedPerTemplate[templateID]!

            // Mint the new moment
            let newNFT: @NFT <- create NFT(mintNumber: numInTemplate + 1 as UInt64,
                                              templateID: templateID,
                                              setID: self.setID)

            // Increment the count of Moments minted for this Play
            self.numberMintedPerTemplate[templateID] = numInTemplate + 1 as UInt64

            return <-newNFT
        }

        // batchMintNFT mints an arbitrary quantity of NFTs 
        // and returns them as a Collection
        //
        // Parameters: templateID: the ID of the Template that the NFTs are minted for
        //             quantity: The quantity of NFTs to be minted
        //
        // Returns: Collection object that contains all the NFTs that were minted
        //
        pub fun batchMintNFT(templateID: UInt64, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintNFT(templateID: templateID))
                i = i + 1 as UInt64
            }

            return <-newCollection
        }
    }


    pub struct NFTData {

        // The ID of the Set that the Moment comes from
        pub let setID: UInt64

        // The ID of the Play that the Moment references
        pub let templateID: UInt64

        // The place in the edition that this Moment was minted
        // Otherwise know as the serial number
        pub let mintNumber: UInt64

        init(setID: UInt64, templateID: UInt64, mintNumber: UInt64) {
            self.setID = setID
            self.templateID = templateID
            self.mintNumber = mintNumber
        }

    }

    // NFT
    // A Flow Asset as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64
        // Struct of NFT metadata
        pub let data: NFTData

        // initializer
        //
        init(mintNumber: UInt64, templateID: UInt64, setID: UInt64) {
            // Increment the global Moment IDs
            Gaia.totalSupply = Gaia.totalSupply + 1 as UInt64

            self.id = Gaia.totalSupply

            // Set the metadata struct
            self.data = NFTData(setID: setID, templateID: templateID, mintNumber: mintNumber)

            emit Minted(assetID: self.id, templateID: templateID, setID: self.data.setID, mintNumber: self.data.mintNumber)
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the 
    // various aspects of the Templates, Sets, and NFTs
    //
    pub resource Admin {

        // createTemplate creates a new Template struct 
        // and stores it in the Templates dictionary in the TopShot smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Name": "John Doe", "DoB": "4/14/1990"}
        //
        // Returns: the ID of the new Template object
        //
        pub fun createTemplate(metadata: {String: String}): UInt64 {
            // Create the new Template
            var newTemplate = Template(metadata: metadata)
            let newID = newTemplate.templateID

            // Store it in the contract storage
            Gaia.templateDatas[newID] = newTemplate

            return newID
        }

         pub fun createTemplates(templates: [{String: String}], setID: UInt64, authorizedAccount: Address){
             
              var templateIDs: [UInt64] = []
            for metadata in templates {
                var ID = self.createTemplate(metadata: metadata)
                templateIDs.append(ID)
            }
            self.borrowSet(setID: setID, authorizedAccount: authorizedAccount).addTemplates(templateIDs: templateIDs)
        }

        // createSet creates a new Set resource and stores it
        // in the sets mapping in the contract
        //
        // Parameters: name: The name of the Set
        //
        pub fun createSet(name: String, description: String, website: String, imageURI: String, creator: Address, marketFee: UFix64) {
            // Create the new Set
            var newSet <- create Set(name: name, description: description, website: website, imageURI: imageURI, creator: creator, marketFee: marketFee)
            // Store it in the sets mapping field
            Gaia.sets[newSet.setID] <-! newSet
        }

        // borrowSet returns a reference to a set in the contract
        // so that the admin can call methods on it
        //
        // Parameters: setID: The ID of the Set that you want to
        // get a reference to
        //
        // Returns: A reference to the Set with all of the fields
        // and methods exposed
        //
        pub fun borrowSet(setID: UInt64, authorizedAccount: Address): &Set {
            pre {
                Gaia.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
                Gaia.setDatas[setID]!.returnAllowedAccounts().contains(authorizedAccount): "Account not authorized"
            }
            
            // Get a reference to the Set and return it
            // use `&` to indicate the reference to the object and type
            return &Gaia.sets[setID] as &Set
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their Gaia Collection as
    // to allow others to deposit Gaia into their Collection. It also allows for reading
    // the details of Gaia in the Collection.
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowGaiaNFT(id: UInt64): &Gaia.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow GaiaAsset reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of GaiaAsset NFTs owned by an account
    //
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn NFTs
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
            let token <- token as! @Gaia.NFT

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

        // borrowGaiaNFT
        // Gets a reference to an NFT in the collection as a GaiaAsset,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the GaiaAsset.
        //
        pub fun borrowGaiaNFT(id: UInt64): &Gaia.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Gaia.NFT
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

    // getAllTemplates returns all the plays in topshot
    //
    // Returns: An array of all the plays that have been created
    pub fun getAllTemplates(): [Gaia.Template] {
        return Gaia.templateDatas.values
    }

    // getTemplateMetaData returns all the metadata associated with a specific Template
    // 
    // Parameters: templateID: The id of the Template that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getTemplateMetaData(templateID: UInt64): {String: String}? {
        return self.templateDatas[templateID]?.metadata
    }

    // getTemplateMetaDataByField returns the metadata associated with a 
    //                        specific field of the metadata
    //                        Ex: field: "Name" will return something
    //                        like "John Doe"
    // 
    // Parameters: templateID: The id of the Template that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getTemplateMetaDataByField(templateID: UInt64, field: String): String? {
        // Don't force a revert if the playID or field is invalid
        if let template = Gaia.templateDatas[templateID] {
            return template.metadata[field]
        } else {
            return nil
        }
    }

    // getSetName returns the name that the specified Set
    //            is associated with.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: The name of the Set
    pub fun getSetName(setID: UInt64): String? {
        // Don't force a revert if the setID is invalid
        return Gaia.setDatas[setID]?.name
    }

    pub fun getSetMarketFee(setID: UInt64): UFix64? {
        // Don't force a revert if the setID is invalid
        return Gaia.setDatas[setID]?.marketFee
    }
    
    pub fun getSetImage(setID: UInt64): String? {
        // Don't force a revert if the setID is invalid
        return Gaia.setDatas[setID]?.imageURI
    } 

    pub fun getSetInfo(setID: UInt64): SetData? {
        // Don't force a revert if the setID is invalid
        return Gaia.setDatas[setID]
    } 

    // getSetIDsByName returns the IDs that the specified Set name
    //                 is associated with.
    // 
    // Parameters: setName: The name of the Set that is being searched
    //
    // Returns: An array of the IDs of the Set if it exists, or nil if doesn't
    pub fun getSetIDsByName(setName: String): [UInt64]? {
        var setIDs: [UInt64] = []

        // Iterate through all the setDatas and search for the name
        for setData in Gaia.setDatas.values {
            if setName == setData.name {
                // If the name is found, return the ID
                setIDs.append(setData.setID)
            }
        }

        // If the name isn't found, return nil
        // Don't force a revert if the setName is invalid
        if setIDs.length == 0 {
            return nil
        } else {
            return setIDs
        }
    }

    // getTemplatesInSet returns the list of Template IDs that are in the Set
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: An array of Template IDs
    pub fun getTemplatesInSet(setID: UInt64): [UInt64]? {
        // Don't force a revert if the setID is invalid
        return Gaia.sets[setID]?.templates
    }

    // isSetTemplateLocked returns a boolean that indicates if a Set/Template combo
    //                  is locked.
    //                  If an template is locked, it still remains in the Set,
    //                  but NFTs can no longer be minted from it.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //             playID: The id of the Play that is being searched
    //
    // Returns: Boolean indicating if the template is locked or not
    pub fun isSetTemplateLocked(setID: UInt64, templateID: UInt64): Bool? {
        // Don't force a revert if the set or play ID is invalid
        // Remove the set from the dictionary to get its field
        if let setToRead <- Gaia.sets.remove(key: setID) {

            // See if the Play is retired from this Set
            let locked = setToRead.lockedTemplates[templateID]

            // Put the Set back in the contract storage
            Gaia.sets[setID] <-! setToRead

            // Return the retired status
            return locked
        } else {

            // If the Set wasn't found, return nil
            return nil
        }
    }

    // isSetLocked returns a boolean that indicates if a Set
    //             is locked. If it's locked, 
    //             new Plays can no longer be added to it,
    //             but NFTs can still be minted from Templates the set contains.
    // 
    // Parameters: setID: The id of the Set that is being searched
    //
    // Returns: Boolean indicating if the Set is locked or not
    pub fun isSetLocked(setID: UInt64): Bool? {
        // Don't force a revert if the setID is invalid
        return Gaia.sets[setID]?.locked
    }

    // getTotalMinted return the number of NFTS that have been 
    //                        minted from a certain set and template.
    //
    // Parameters: setID: The id of the Set that is being searched
    //             templateID: The id of the Template that is being searched
    //
    // Returns: The total number of NFTs 
    //          that have been minted from an set and template
    pub fun getTotalMinted(setID: UInt64, templateID: UInt64): UInt64? {
        // Don't force a revert if the Set or play ID is invalid
        // Remove the Set from the dictionary to get its field
        if let setToRead <- Gaia.sets.remove(key: setID) {

            // Read the numMintedPerPlay
            let amount = setToRead.numberMintedPerTemplate[templateID]

            // Put the Set back into the Sets dictionary
            Gaia.sets[setID] <-! setToRead

            return amount
        } else {
            // If the set wasn't found return nil
            return nil
        }
    }

    // fetch
    // Get a reference to a GaiaAsset from an account's Collection, if available.
    // If an account does not have a Gaia.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &Gaia.NFT? {
        let collection = getAccount(from)
            .getCapability(Gaia.CollectionPublicPath)
            .borrow<&Gaia.Collection{Gaia.CollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Gaia.Collection.borowGaiaAsset to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowGaiaNFT(id: itemID)
    }
    
    // checkSetup
    // Get a reference to a GaiaAsset from an account's Collection, if available.
    // If an account does not have a Gaia.Collection, returns false.
    // If it has a collection, return true.
    //
    pub fun checkSetup(_ address: Address): Bool {
        return getAccount(address)
        .getCapability<&{Gaia.CollectionPublic}>(Gaia.CollectionPublicPath)
        .check()
    }

    // initializer
    //
    init() {
        // Set our named paths
        //FIXME: REMOVE SUFFIX BEFORE RELEASE
        self.CollectionStoragePath = /storage/GaiaCollection001
        self.CollectionPublicPath = /public/GaiaCollection001

        // Initialize contract fields
        self.templateDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextTemplateID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{CollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/GaiaAdmin)
    }
}

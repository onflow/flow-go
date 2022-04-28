import NonFungibleToken from 0x1d7e57aa55817448

pub contract KlktnNFT2: NonFungibleToken {

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Events
  // -----------------------------------------------------------------------
  pub event ContractInitialized() // Emitted when KlktnNFT contract is created
  pub event NFTTemplateCreated(typeID: UInt64, name: String, mintLimit: UInt64, priceUSD: UFix64, priceFlow: UFix64, metadata: {String: String}, isPack: Bool)
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, typeID: UInt64, serialNumber: UInt64, metadata: KlktnNFTTemplatePublic)
  pub event ExternalMinted(id: UInt64, typeID: UInt64, serialNumber: UInt64, metadata: KlktnNFTTemplatePublic)
  pub event PackOpened(id: UInt64, typeID: UInt64, name: String, address: Address?) // Emitted when a pack is opened
  
  // -----------------------------------------------------------------------
  // KlktnNFT Contract Named Paths
  // -----------------------------------------------------------------------
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let AdminStoragePath: StoragePath

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Properties
  // -----------------------------------------------------------------------
  // totalSupply: total number of KlktnNFTs minted
  pub var totalSupply: UInt64
  // KlktnNFTTypeSet: dictionary for metadata and administrative parameters per typeID
  access(self) var KlktnNFTTypeSet: {UInt64: KlktnNFTTemplate}

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Resource Interfaces
  // -----------------------------------------------------------------------
  // KlktnNFTCollectionPublic:
  // - This is the interface that users can cast their KlktnNFT Collection as
  // - to allow others to deposit KlktnNFT into their Collection
  // - It also allows for reading the details of KlktnNFT in the Collection
  pub resource interface KlktnNFTCollectionPublic {
    pub fun deposit(token: @NonFungibleToken.NFT)
    pub fun getIDs(): [UInt64]
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
    pub fun borrowKlktnNFT(id: UInt64): &KlktnNFT2.NFT? {
      // If the result isn't nil, the id of the returned reference
      // should be the same as the argument to the function
      post {
        (result == nil) || (result?.id == id):
          "Cannot borrow KlktnNFT reference: The ID of the returned reference is incorrect"
      }
    }
    pub fun openPack(packID: UInt64)
  }

  // AdminPrivate: admin private interface that Admin implements
  pub resource interface AdminPrivate {
    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, serialNumber: UInt64, metadata: {String: String})
    pub fun mintNextAvailableNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, metadata: {String: String})
    pub fun updateTemplateMetadata(typeID: UInt64, metadataToUpdate: {String: String}): KlktnNFT2.KlktnNFTTemplate
    pub fun createNFTTemplate(typeID: UInt64, isPack: Bool, name: String, mintLimit: UInt64, priceUSD: UFix64, priceFlow: UFix64, isProtected: Bool, metadata: {String: String})
    pub fun protectNFTTemplate(typeID: UInt64)
    pub fun unprotectNFTTemplate(typeID: UInt64)
    pub fun createNewAdmin(): @Admin
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Structs
  // -----------------------------------------------------------------------
  // KlktnNFTTemplate: metadata and properties for token per typeID
  pub struct KlktnNFTTemplate {
    pub let isPack: Bool
    pub let typeID: UInt64
    pub let name: String
    pub var isProtected: Bool
    pub var mintLimit: UInt64
    pub var priceUSD: UFix64
    pub var priceFlow: UFix64
    pub var tokenMinted: UInt64
    pub var maxSerialNumberMinted: UInt64
    pub var isExpired: Bool
    access(self) var serialNumberMinted: {UInt64: Bool}
    access(self) var metadata: {String: String}

    pub fun getPublic (): KlktnNFTTemplatePublic {
      return KlktnNFTTemplatePublic(initTypeID: self.typeID, initIsPack: self.isPack, initName: self.name, initMintLimit: self.mintLimit, initMetadata: self.metadata)
    }

    pub fun getSerialNumberMinted(): {UInt64: Bool} {
      return self.serialNumberMinted
    }

    pub fun addSerialNumberToMintedDict (serialNumber: UInt64) {
      self.serialNumberMinted[serialNumber] = true
      self.tokenMinted  = self.tokenMinted + 1
    }

    pub fun updateMaxSerialNumberMinted (mintedSerialNumber: UInt64) {
      if (mintedSerialNumber > self.maxSerialNumberMinted) {
        self.maxSerialNumberMinted = mintedSerialNumber
      }
    }

    pub fun updatePriceUSD(newPriceUSD: UFix64) {
      self.priceUSD = newPriceUSD
    }

    pub fun updatePriceFlow(newPriceFlow: UFix64) {
      self.priceFlow = newPriceFlow
    }

    pub fun updateMintLimit(newMintLimit: UInt64) {
      self.mintLimit = newMintLimit
    }

    pub fun updateMetadata(newMetadata: {String: String}) {
      self.metadata = newMetadata
    }

    pub fun protect () {      
      self.isProtected = true
    }

    pub fun unprotect () {      
      self.isProtected = false
    }

    // expireNFTTemplate resets serialNumberMinted dictionary & mark NFT template as expired
    pub fun expireNFTTemplate() {
      self.serialNumberMinted = {}
      self.isExpired = true
    }

    init(initTypeID: UInt64, initIsPack: Bool, initName: String, initMintLimit: UInt64, initPriceUSD: UFix64, initPriceFlow: UFix64, initIsProtected: Bool, initMetadata: {String: String}){
      self.isProtected = initIsProtected
      self.isPack = initIsPack
      self.typeID = initTypeID
      self.name = initName
      self.mintLimit = initMintLimit
      self.metadata = initMetadata
      self.priceUSD = initPriceUSD
      self.priceFlow = initPriceFlow
      self.serialNumberMinted = {}
      self.tokenMinted = 0
      self.maxSerialNumberMinted = 0
      self.isExpired = false
      emit NFTTemplateCreated(typeID: initTypeID, name: initName, mintLimit: initMintLimit, priceUSD: initPriceUSD, priceFlow: initPriceFlow, metadata: initMetadata, isPack: self.isPack)
    }
  }

  // KlktnNFTTemplatePublic: publically available metadata and properties for token per typeID
  // -- we use this in emitted events & user-level transactions/scripts to display NFT template info without exposing the full object
  pub struct KlktnNFTTemplatePublic {
    pub let isPack: Bool
    pub let typeID: UInt64
    pub let name: String
    pub var mintLimit: UInt64
    pub var metadata: {String: String}

    init(initTypeID: UInt64, initIsPack: Bool, initName: String, initMintLimit: UInt64, initMetadata: {String: String}) {
      self.isPack = initIsPack
      self.typeID = initTypeID
      self.name = initName
      self.mintLimit = initMintLimit
      self.metadata = initMetadata
    }
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Resources
  // -----------------------------------------------------------------------

  // NFT: The resource that represents the NFTs
  pub resource NFT: NonFungibleToken.INFT {
    pub let id: UInt64
    pub let typeID: UInt64
    pub let serialNumber: UInt64
    access(self) let metadata: {String: String}
    
    // getNFTTemplateMetadata gets template metadata for the NFT template
    pub fun getNFTTemplate(): KlktnNFTTemplatePublic? {
      return KlktnNFT2.KlktnNFTTypeSet[self.typeID]!.getPublic()
    }

    // getNFTMetadata gets NFT's own immutable metadata
    pub fun getNFTMetadata(): {String: String} {
      return self.metadata
    }

    // getNFTMetadata gets a KlktnNFTTemplatePublic struct with combined metadata
    pub fun getFullMetadata(): KlktnNFTTemplatePublic {
      let template = KlktnNFT2.KlktnNFTTypeSet[self.typeID]!.getPublic()
      for key in self.metadata.keys {
        template.metadata[key] = self.metadata[key]
      }
      return template
    }

    init(initID: UInt64, initTypeID: UInt64, initSerialNumber: UInt64, initMetadata: {String: String}) {
      self.id = initID
      self.typeID = initTypeID
      self.serialNumber = initSerialNumber
      self.metadata = initMetadata
    }
  }

  // Collection: a resource that every user who owns NFTs
  // - will store in their account to manage their NFTs & packs
  pub resource Collection: KlktnNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    // withdraw: removes an NFT from the collection and moves it to the caller
    // - parameter: withdrawID: the ID of the owned NFT that is to be removed from the Collection
    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
      emit Withdraw(id: token.id, from: self.owner?.address)
      return <-token
    }

    // deposit: takes an NFT and adds it to the Collection dictionary
    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @KlktnNFT2.NFT
      let id: UInt64 = token.id
      // add the new token to the dictionary which removes the old one
      let oldToken <- self.ownedNFTs[id] <- token
      emit Deposit(id: id, to: self.owner?.address)
      destroy oldToken
    }

    // batchDeposit: batch deposit a collection to current collection
    pub fun batchDeposit(collection: @Collection) {
      let keys = collection.getIDs()
      for key in keys {
        self.deposit(token: <-collection.withdraw(withdrawID: key))
      }
      destroy collection
    }

    // getIDs:
    // - Returns an array of the IDs that are in the collection
    pub fun getIDs(): [UInt64] {
      return self.ownedNFTs.keys
    }

    // borrowNFT: 
    // - Gets a reference to an NFT in the collection
    // so that the caller can read its metadata and call its methods
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
      return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    // borrowKlktnNFT: 
    // - Gets a reference to an NFT in the collection as a KlktnNFT,
    // - exposing all of its fields (including the typeID)
    // - This is safe as there are no administrative functions that can be called on the KlktnNFT
    pub fun borrowKlktnNFT(id: UInt64): &KlktnNFT2.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &KlktnNFT2.NFT
      } else {
        return nil
      }
    }

    // openPack: open an NFT as a pack by destroying it and emitting a PackOpened event
    pub fun openPack(packID: UInt64) {
      pre {
        self.ownedNFTs[packID] != nil:
          "invalid packID."
      }
      let packRef = (&self.ownedNFTs[packID] as auth &NonFungibleToken.NFT) as! &KlktnNFT2.NFT
      let packTemplateInfo = packRef.getNFTTemplate()!
      if (!packTemplateInfo.isPack) {
        panic("NFT is not a pack.")
      }
      let pack <- self.ownedNFTs.remove(key: packID)
      emit PackOpened(id: packID, typeID: packTemplateInfo.typeID, name: packTemplateInfo.name, address: self.owner?.address)
      destroy pack
    }

    // destructor
    destroy() {
      destroy self.ownedNFTs
    }

    // initializer
    init () {
      self.ownedNFTs <- {}
    }
}

  // createEmptyCollection:
  // - Public function that anyone can call to create a new empty Collection
  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <- create Collection()
  }

  // Admin
  // - Administrative resource that only the contract deployer has access to
  // - to mint token and create NFT templates
  pub resource Admin: AdminPrivate {

    // mintNFT: Mints a new NFT with a new ID and a specified serialNumber, deposit it in the recipient's collection using their collection reference
    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, serialNumber: UInt64, metadata: {String: String}) {
      pre {
        KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID): // template with typeID exists
          "template for typeID does not exist."
        !KlktnNFT2.KlktnNFTTypeSet[typeID]!.isExpired: // template not expired
          "token of this typeID is no longer being offered."
        serialNumber > 0: // valid serialNumber
          "invalid serialNumber for token of this typeID."
        !KlktnNFT2.KlktnNFTTypeSet[typeID]!.getSerialNumberMinted().containsKey(serialNumber):
          "invalid serialNumber for token of this typeID."
      }
      // Get the final serialNumber to mint
      let targetTokenMetadata = KlktnNFT2.KlktnNFTTypeSet[typeID]!.getPublic()
      // emit Minted event
      emit Minted(id: KlktnNFT2.totalSupply, typeID: typeID, serialNumber: serialNumber, metadata: targetTokenMetadata)
      // mint and deposit NFT in the recipient's account using their receiver reference
      recipient.deposit(token: <-create KlktnNFT2.NFT(initID: KlktnNFT2.totalSupply, initTypeID: typeID, initSerialNumber: serialNumber, initMetadata: metadata))
      // increase KlktnNFT total supply
      KlktnNFT2.totalSupply = KlktnNFT2.totalSupply + (1 as UInt64)
      // add the minted serialNumber to the serialNumberMinted hashmap
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.addSerialNumberToMintedDict(serialNumber: serialNumber)
      // update maxSerialNumberMinted
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updateMaxSerialNumberMinted(mintedSerialNumber: serialNumber)
      // mark NFT template as expired when mintLimit is reached
      if (KlktnNFT2.KlktnNFTTypeSet[typeID]!.tokenMinted == KlktnNFT2.KlktnNFTTypeSet[typeID]!.mintLimit) {
        KlktnNFT2.KlktnNFTTypeSet[typeID]!.expireNFTTemplate()
      }
    }

    // mintNextAvailableNFT: Mints a new NFT with a new ID and the next available serialNumber, deposit it in the recipients collection using their collection reference
    pub fun mintNextAvailableNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, metadata: {String: String}) {
      pre {
        KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID): // template with typeID exists
          "template for typeID does not exist."
        !KlktnNFT2.KlktnNFTTypeSet[typeID]!.isProtected: // template is protected (not on sale publically)
          "template for typeID is not on sale."
        !KlktnNFT2.KlktnNFTTypeSet[typeID]!.isExpired: // template not expired
          "token of this typeID is no longer being offered."
        KlktnNFT2.KlktnNFTTypeSet[typeID]!.maxSerialNumberMinted <= KlktnNFT2.KlktnNFTTypeSet[typeID]!.mintLimit:
          "token of this typeID is no longer being offered."
      }
      // Get the final serialNumber to mint
      let finalSerialNumber = KlktnNFT2.KlktnNFTTypeSet[typeID]!.maxSerialNumberMinted + (1 as UInt64)
      let targetTokenMetadata = KlktnNFT2.KlktnNFTTypeSet[typeID]!.getPublic()
      // emit Minted event
      emit ExternalMinted(id: KlktnNFT2.totalSupply, typeID: typeID, serialNumber: finalSerialNumber, metadata: targetTokenMetadata)
      // mint and deposit NFT in the recipient's account using their receiver reference
      recipient.deposit(token: <-create KlktnNFT2.NFT(initID: KlktnNFT2.totalSupply, initTypeID: typeID, initSerialNumber: finalSerialNumber, initMetadata: metadata))
      // increase KlktnNFT total supply
      KlktnNFT2.totalSupply = KlktnNFT2.totalSupply + (1 as UInt64)
      // add the minted serialNumber to the serialNumberMinted hashmap
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.addSerialNumberToMintedDict(serialNumber: finalSerialNumber)
      // update maxSerialNumberMinted
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updateMaxSerialNumberMinted(mintedSerialNumber: finalSerialNumber)
      // mark NFT template as expired when mintLimit is reached
      if (KlktnNFT2.KlktnNFTTypeSet[typeID]!.tokenMinted == KlktnNFT2.KlktnNFTTypeSet[typeID]!.mintLimit) {
        KlktnNFT2.KlktnNFTTypeSet[typeID]!.expireNFTTemplate()
      }
    }
    
    // updateTemplateMetadata updates an NFT template metadata
    pub fun updateTemplateMetadata(typeID: UInt64, metadataToUpdate: {String: String}): KlktnNFT2.KlktnNFTTemplate {
      pre {
        KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID) != nil:
          "Token with the typeID does not exist."
      }
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updateMetadata(newMetadata: metadataToUpdate)
      // return a copy of the updated object
      return KlktnNFT2.KlktnNFTTypeSet[typeID]!
    }

    // createNFTTemplate creates an NFT template for token of typeID
    pub fun createNFTTemplate(typeID: UInt64, isPack: Bool, name: String, mintLimit: UInt64, priceUSD: UFix64, priceFlow: UFix64, isProtected: Bool, metadata: {String: String}){
      pre {
        !KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID):
          "NFT template with the same typeID already exists."
      }
      // create a new KlktnNFTTemplate resource for the typeID
      let newNFTTemplate = KlktnNFTTemplate(
        initTypeID: typeID,
        initIsPack: isPack,
        initName: name,
        initMintLimit: mintLimit,
        initPriceUSD: priceUSD,
        initPriceFlow: priceFlow,
        initIsProtected: isProtected,
        initMetadata: metadata
      )
      // store it in the KlktnNFTTypeSet mapping field
      KlktnNFT2.KlktnNFTTypeSet[newNFTTemplate.typeID] = newNFTTemplate
    }

    // protectNFTTemplate: protects an NFT template from public purchase
    pub fun protectNFTTemplate(typeID: UInt64) {
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.protect()
    }

    // unprotectNFTTemplate: unprotects an NFT template to make it available for public purchase
    pub fun unprotectNFTTemplate(typeID: UInt64) {
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.unprotect()
    }

    // updateNFTTemplatePriceUSD: updates priceFUSD for an NFT template
    pub fun updateNFTTemplatePriceUSD(typeID: UInt64, newPriceUSD: UFix64) {
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updatePriceUSD(newPriceUSD: newPriceUSD)
    }

    // updateNFTTemplatePriceFlow: updates priceFlow for an NFT template
    pub fun updateNFTTemplatePriceFlow(typeID: UInt64, newPriceFlow: UFix64) {
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updatePriceFlow(newPriceFlow: newPriceFlow)
    }

    // updateNFTTemplateMintLimit: updates mintLimit for an NFT template
    pub fun updateNFTTemplateMintLimit(typeID: UInt64, newMintLimit: UInt64) {
      pre {
        KlktnNFT2.KlktnNFTTypeSet[typeID]!.maxSerialNumberMinted <= newMintLimit:
          "invalid mintLimit."
      }
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.updateMintLimit(newMintLimit: newMintLimit)
    }

    // expireNFTTemplate expires an NFT template permanently
    pub fun expireNFTTemplate(typeID: UInt64) {
      KlktnNFT2.KlktnNFTTypeSet[typeID]!.expireNFTTemplate()
    }

    // createNewAdmin creates a new Admin resource
    pub fun createNewAdmin(): @Admin {
      return <- create Admin()
    }
  }

  // -----------------------------------------------------------------------
  // KlktnNFT contract-level functions
  // -----------------------------------------------------------------------

  // peekTokenLimit returns enforced mint limit for a token of typeID
  pub fun peekTokenLimit(typeID: UInt64): UInt64? {
    if let token = KlktnNFT2.KlktnNFTTypeSet[typeID] {
      return token.mintLimit
    } else {
      return nil
    }
  }

  // peekNFTTemplates returns all NFT templates
  // note: this is safe as KlktnNFTTemplate does not have pub(set) access, so the retriever cannot alter the data inside
  pub fun peekNFTTemplates(): [KlktnNFTTemplate]{
    return KlktnNFT2.KlktnNFTTypeSet.values
  }

  // peekNFTTemplates returns a list of typeID of all NFT templates
  pub fun peekNFTTemplatesTypeID(): [UInt64] {
    return KlktnNFT2.KlktnNFTTypeSet.keys
  }

  // isValidSerialNumber returns boolean indicating serialNumber to mint for token of typeId is valid
  pub fun isValidSerialNumber(typeID: UInt64, serialNumber: UInt64): Bool {
    var NFTTemplateObj = KlktnNFT2.KlktnNFTTypeSet[typeID]!
    return !NFTTemplateObj.getSerialNumberMinted().containsKey(serialNumber)
  }

  // isNFTTemplateExpired returns boolean indicating token of typeID is expired
  // - We also return true representing token of typeID is expired for tokens without valid templates
  pub fun isNFTTemplateExpired(typeID: UInt64): Bool {
    if (!KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID)) {
      return true
    }
    return KlktnNFT2.KlktnNFTTypeSet[typeID]!.isExpired
  }

  // isNFTTemplateExist returns boolean indicating if template exists
  pub fun isNFTTemplateExist(typeID: UInt64): Bool {
    if KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID) {
      return true
    }
    return false
  }

  // getNFTTemplateMetadata
  // - returns the metadata of an NFT given a typeID
  pub fun getNFTTemplateMetadata(typeID: UInt64): {String: String} {
    if KlktnNFT2.KlktnNFTTypeSet.containsKey(typeID) {
      return KlktnNFT2.KlktnNFTTypeSet[typeID]!.getPublic().metadata
    }
    panic("invalid token typeID.")
  }

  // getFlowPriceByTypeID gets the NFT template information by typeID
  pub fun getNFTTemplateInfo(typeID: UInt64): KlktnNFTTemplate {
    return KlktnNFT2.KlktnNFTTypeSet[typeID]!
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Initializer
  // -----------------------------------------------------------------------
  init() {
    // Set our named paths
    self.CollectionStoragePath = /storage/KlktnNFT2Collection
    self.CollectionPublicPath = /public/KlktnNFT2Collection
    self.AdminStoragePath = /storage/KlktnNFT2Admin

    // Initialize the total supply
    self.totalSupply = 0

    // Initialize the type mappings
    self.KlktnNFTTypeSet = {}

    // Create a Minter resource and save it to storage
    let admin <- create Admin()
    self.account.save(<-admin, to: self.AdminStoragePath)
    emit ContractInitialized()
  }
}
// KlktnNFT implements NonFungibleToken contract interface
import NonFungibleToken from 0x1d7e57aa55817448

pub contract KlktnNFT: NonFungibleToken {

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Events
  // -----------------------------------------------------------------------

  // Emitted when KlktnNFT contract is created
  pub event ContractInitialized()
  // Emitted when Collection events below are created
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, typeID: UInt64, serialNumber: UInt64, metadata: {String: String})
  // Emitted when an nft template is created
  pub event NFTTemplateCreated(typeID: UInt64, tokenName: String, mintLimit: UInt64, metadata: {String: String})
  
  // -----------------------------------------------------------------------
  // KlktnNFT Contract Named Paths
  // -----------------------------------------------------------------------
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let AdminStoragePath: StoragePath

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Properties
  // -----------------------------------------------------------------------
  // totalSupply:
  // - Total number of KlktnNFTs that have been minted
  pub var totalSupply: UInt64
  // klktnNFTTypeSet:
  // - Dictionary for metadata and administrative parameters per typeID
  access(self) var klktnNFTTypeSet: {UInt64: KlktnNFTMetadata}
  // tokenMintedPerType:
  // - Dictionary to track minted tokens per typeID
  access(self) var tokenMintedPerType: {UInt64: UInt64}

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
    pub fun borrowKlktnNFT(id: UInt64): &KlktnNFT.NFT? {
      // If the result isn't nil, the id of the returned reference
      // should be the same as the argument to the function
      post {
        (result == nil) || (result?.id == id):
          "Cannot borrow KlktnNFT reference: The ID of the returned reference is incorrect"
      }
    }
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Structs
  // -----------------------------------------------------------------------
  // KlktnNFTMetadata:
  // - metadata and properties for token per typeID
  pub struct KlktnNFTMetadata {
    pub let typeID: UInt64
    pub let tokenName: String
    pub var mintLimit: UInt64
    pub let metadata: {String: String}

    init(initTypeID: UInt64, initTokenName: String, initMintLimit: UInt64, initMetadata: {String: String}){
      self.typeID = initTypeID
      self.tokenName = initTokenName
      self.mintLimit = initMintLimit
      self.metadata = initMetadata
      emit NFTTemplateCreated(typeID: initTypeID, tokenName: initTokenName, mintLimit: initMintLimit, metadata: initMetadata)
    }
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Resources
  // -----------------------------------------------------------------------
  // NFT:
  // - The resource that represents the artist-released NFTs
  pub resource NFT: NonFungibleToken.INFT {
    // unique id for the NFT
    pub let id: UInt64
    // token's type, e.g. 1 == Heart
    pub let typeID: UInt64
    // serial number of token, this is unique and auto-increment per typeID
    pub let serialNumber: UInt64

    // fetch metadata from the contract
    pub fun getNFTMetadata(): {String: String} {
      return KlktnNFT.getNFTMetadata(typeID: self.typeID)
    }

    init(initID: UInt64, initTypeID: UInt64, initSerialNumber: UInt64) {
      self.id = initID
      self.typeID = initTypeID
      self.serialNumber = initSerialNumber
    }
  }

  // Collection
  // - A resource that every user who owns NFTs
  // - will srore in their account to manage their NFTs
  pub resource Collection: KlktnNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    // withdraw:
    // - Removes an NFT from the collection and moves it to the caller
    // - parameter: withdrawID: the ID of the owned NFT that is to be removed from the Collection
    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
      emit Withdraw(id: token.id, from: self.owner?.address)
      return <-token
    }

    // deposit:
    // - Takes an NFT and adds it to the Collection dictionary
    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @KlktnNFT.NFT
      let id: UInt64 = token.id
      // add the new token to the dictionary which removes the old one
      let oldToken <- self.ownedNFTs[id] <- token
      emit Deposit(id: id, to: self.owner?.address)
      destroy oldToken
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
    pub fun borrowKlktnNFT(id: UInt64): &KlktnNFT.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &KlktnNFT.NFT
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

  // createEmptyCollection:
  // - Public function that anyone can call to create a new empty Collection
  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <- create Collection()
  }

  // Admin
  // - Administrative resource that only the contract deployer has access to
  // - to mint token and create NFT templates
  pub resource Admin {

    // mintNFT: Mints a new NFT with a new ID
    // - and deposit it in the recipients collection using their collection reference
    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64) {
      // check if template of typeID exists
      if !KlktnNFT.klktnNFTTypeSet.containsKey(typeID) {
        panic("template for typeID does not exist.")
      }
      // check if token template is expired
      if KlktnNFT.checkTokenExpiration(typeID: typeID) {
        panic("token of this typeID is no longer being offered.")
      }      
      let targetTokenMetadata = KlktnNFT.klktnNFTTypeSet[typeID]!
      // check serial number existence, initialize it if serial number does not exist
      if !KlktnNFT.tokenMintedPerType.containsKey(typeID) {
        KlktnNFT.tokenMintedPerType[typeID] = (0 as UInt64)
      }
      let serialNumber = KlktnNFT.tokenMintedPerType[typeID]! + (1 as UInt64)
      // emit Minted event
      emit Minted(id: KlktnNFT.totalSupply, typeID: typeID, serialNumber: serialNumber, metadata: targetTokenMetadata.metadata)

      // deposit it in the recipient's account using their reference
      recipient.deposit(token: <-create KlktnNFT.NFT(
        initID: KlktnNFT.totalSupply,
        initTypeID: typeID,
        initSerialNumber: serialNumber
        )
      )

      KlktnNFT.totalSupply = KlktnNFT.totalSupply + (1 as UInt64)
      // increse the serial number for the minted token type
      KlktnNFT.tokenMintedPerType[typeID] = serialNumber
    }
    
    pub fun updateTemplateMetadata(typeID: UInt64, metadataToUpdate: {String: String}): KlktnNFT.KlktnNFTMetadata {
      if !KlktnNFT.klktnNFTTypeSet.containsKey(typeID) {
        panic("Token with the typeID does not exist.")
      }
      // typeID cannot change
      var NFTTemplateObj = KlktnNFT.klktnNFTTypeSet[typeID]!
      let typeID = NFTTemplateObj.typeID
      let tokenName = NFTTemplateObj.tokenName
      let mintLimit = NFTTemplateObj.mintLimit
      let newNFTTemplateObj = KlktnNFTMetadata(initTypeID: typeID, initTokenName: tokenName, initMintLimit: mintLimit, initMetadata: metadataToUpdate)
      // update
      KlktnNFT.klktnNFTTypeSet[typeID] = newNFTTemplateObj
      // return updated object
      return KlktnNFT.klktnNFTTypeSet[typeID]!
    }

    // mintNFT: createTemplate: creates a template for token of typeID
    pub fun createTemplate(typeID: UInt64, tokenName: String, mintLimit: UInt64, metadata: {String: String}): UInt64 {
      // check if template with the same id exists
      if KlktnNFT.klktnNFTTypeSet.containsKey(typeID) {
        panic("Token with the same typeID already exists.")
      }
      // create a new KlktnNFTMetaData resource for the typeID
      var newNFTTemplate = KlktnNFTMetadata(initTypeID: typeID, initTokenName: tokenName, initMintLimit: mintLimit, initMetadata: metadata)
      // store it in the klktnNFTTypeSet mapping field
      KlktnNFT.klktnNFTTypeSet[newNFTTemplate.typeID] = newNFTTemplate
      return newNFTTemplate.typeID
    }
  }

  // -----------------------------------------------------------------------
  // KlktnNFT contract-level function definitions
  // -----------------------------------------------------------------------
  // fetch:
  // - Get a reference to a KlktnNFT from an account's Collection, if available.
  // - If an account does not have a KlktnNFT.Collection, panic.
  // - If it has a collection but does not contain the itemID, return nil.
  // - If it has a collection and that collection contains the itemID, return a reference to that.
  pub fun fetch(_ from: Address, itemID: UInt64): &KlktnNFT.NFT? {
    let collection = getAccount(from)
      .getCapability(KlktnNFT.CollectionPublicPath)
      .borrow<&KlktnNFT.Collection{KlktnNFT.KlktnNFTCollectionPublic}>()
      ?? panic("Couldn't get collection")
    // We trust KlktnNFT.Collection.borrowKlktnNFT to get the correct itemID
    // (it checks it before returning it).
    return collection.borrowKlktnNFT(id: itemID)
  }

  // peekTokenLimit:
  // - Returns: enforced mint limit for a token of typeID
  pub fun peekTokenLimit(typeID: UInt64): UInt64? {
    if let token = KlktnNFT.klktnNFTTypeSet[typeID] {
      return token.mintLimit
    } else {
      return nil
    }
  }

  // checkTokenExpiration
  // - Returns: boolean indicating token of typeID is expired
  // - We also return true representing token of typeID is expired for tokens without valid templates
  pub fun checkTokenExpiration(typeID: UInt64): Bool {
    // get enforced token mint limit
    var tokenMintLimit = (0 as UInt64)
    if let tokenMetadata = KlktnNFT.klktnNFTTypeSet[typeID] {
      tokenMintLimit = tokenMetadata.mintLimit
    }
    // Get number of minted tokens
    var tokenMinted = (0 as UInt64)
    if let tokenMintedFromContractVar = KlktnNFT.tokenMintedPerType[typeID] {
      tokenMinted = KlktnNFT.tokenMintedPerType[typeID]!
    }
    return tokenMinted >= tokenMintLimit
  }

  // checkTemplate
  // - Returns: boolean indicating if template exists
  pub fun checkTemplate(typeID: UInt64): Bool {
    if KlktnNFT.klktnNFTTypeSet.containsKey(typeID) {
      return true
    }
    return false
  }

  // getNFTMetadata
  // - returns the metadata of an NFT given a typeID
  pub fun getNFTMetadata(typeID: UInt64): {String: String} {
    if KlktnNFT.klktnNFTTypeSet.containsKey(typeID) {
      return KlktnNFT.klktnNFTTypeSet[typeID]!.metadata
    }
    panic("invalid tokentypeID.")
  }

  // -----------------------------------------------------------------------
  // KlktnNFT Contract Initializer
  // -----------------------------------------------------------------------
  init() {
    // Set our named paths
    self.CollectionStoragePath = /storage/KlktnNFTCollection
    self.CollectionPublicPath = /public/KlktnNFTCollection
    self.AdminStoragePath = /storage/KlktnNFTAdmin

    // Initialize the total supply
    self.totalSupply = 0

    // Initialize the type mappings
    self.klktnNFTTypeSet = {}
    self.tokenMintedPerType = {}

    // Create a Minter resource and save it to storage
    let admin <- create Admin()
    self.account.save(<-admin, to: self.AdminStoragePath)
    emit ContractInitialized()
  }
}

// SPDX-License-Identifier: Unlicense

import NonFungibleToken from 0x1d7e57aa55817448

pub contract StarvingChildren: NonFungibleToken {

  // -----------------------------------------------------------------------
  // StarvingChildren contract-level fields.
  // These contain actual values that are stored in the smart contract.
  // -----------------------------------------------------------------------
  
  // The total supply that is used to create FNT. 
  // Every time a NFT is created,  
  // totalSupply is incremented by 1 and then is assigned to NFT's ID.
  pub var totalSupply: UInt64

  // The next template ID that is used to create Template. 
  // Every time a Template is created, nextTemplateId is assigned 
  // to the new Template's ID and then is incremented by 1.
  pub var nextTemplateId: UInt64
  
  // The next NFT ID that is used to create NFT. 
  // Every time a NFT is created, nextNFTId is assigned 
  // to the new NFT's ID and then is incremented by 1.
  pub var nextNFTId: UInt64

  access(self) var adminAddress: Address?

  // Variable size dictionary of Template structs
  access(self) var templateDatas: {UInt64: Template}
  
  // Variable size dictionary of minted templates structs
  access(self) var numberMintedByTemplate: {UInt64: UInt64}
  
  /// Path where the public capability for the `Collection` is available
  pub let collectionPublicPath: PublicPath

  /// Path where the `Collection` is stored
  pub let collectionStoragePath: StoragePath

  /// Path where the `Admin` is stored
  pub let adminStoragePath: StoragePath
  
  /// Path where the public capability for the `Admin` is available
  pub let adminPublicPath: PublicPath

  /// Path where the private capability for the `Admin` is available
  pub let adminPrivatePath: PrivatePath

  /// Path where the `Profile` is stored
  pub let profileStoragePath: StoragePath  
  
  /// Path where the public capability for the `Profile` is available
  pub let profilePublicPath: PublicPath

  /// Event used on create template
  pub event TemplateCreated(template: Template)

  /// Event used on destroy NFT from collection
  pub event NFTDestroyed(nftId: UInt64)

  /// Event used on withdraw NFT from collection
  pub event Withdraw(id: UInt64, from: Address?)

  /// Event used on deposit NFT to collection
  pub event Deposit(id: UInt64, to: Address?)
  
  /// Event used on mint NFT
  pub event NFTMinted(nftId: UInt64, nftData: NFTData)

  /// Event used on contract initiation
  pub event ContractInitialized()
    
  // -----------------------------------------------------------------------
  // StarvingChildren contract-level Composite Type definitions
  // -----------------------------------------------------------------------
  // These are just *definitions* for Types that this contract
  // and other accounts can use. These definitions do not contain
  // actual stored values, but an instance (or object) of one of these Types
  // can be created by this contract that contains stored values.
  // ----------------------------------------------------------------------- 
  
  // Template is a Struct that holds metadata associated with a specific 
  // nft
  //
  // NFT resource will all reference a single template as the owner of
  // its metadata. The templates are publicly accessible, so anyone can
  // read the metadata associated with a specific NFT ID
  //
  pub struct Template {
    pub let templateId: UInt64
    pub let price: UFix64
    pub let maxEditions: UInt64
    pub let creationDate: UInt64    
    pub let metadataBuyer: {String: String}
    pub let metadataCharity: {String: String}

    init(metadataBuyer: {String: String}, metadataCharity: {String: String}, price: UFix64, maxEditions: UInt64, creationDate: UInt64) {
      pre {
        metadataBuyer.length != 0: "metadataBuyer cannot be empty"
        metadataCharity.length != 0: "metadataCharity cannot be empty"
        price > 0.0: "price must be more than zero "
      }

      self.templateId = StarvingChildren.nextTemplateId
      self.metadataBuyer = metadataBuyer
      self.metadataCharity = metadataCharity
      self.price = price
      self.maxEditions = maxEditions
      self.creationDate = creationDate

      StarvingChildren.nextTemplateId = StarvingChildren.nextTemplateId + UInt64(1)
    }
  }

  // NFTData is a Struct that holds template's ID and a metadata
  //
  pub struct NFTData {
    pub let templateId: UInt64
    pub let edition: UInt64
    pub let metadata: {String: String}

    init(metadata: {String: String}, templateId: UInt64, edition: UInt64) {
      self.templateId = templateId
      self.metadata = metadata
      self.edition = edition
    }
  }

  // The resource that represents the NFT
  //
  pub resource NFT: NonFungibleToken.INFT {
    pub let id: UInt64
    pub let data: NFTData

    init(edition: UInt64, metadata: {String: String}, templateId: UInt64) {
      self.id = StarvingChildren.nextNFTId

      self.data = NFTData(metadata:metadata, templateId:templateId, edition: edition)

      emit NFTMinted(nftId: self.id, nftData: self.data)
      StarvingChildren.nextNFTId = StarvingChildren.nextNFTId + UInt64(1)
    }

    destroy() {
      emit NFTDestroyed(nftId: self.id)
    }
  }

  // Blank interface to flag admin account
  pub resource interface AdminPublic {

  }

  // Admin is a resource that deployer user has
  // to mint NFT and create templates
  //
  pub resource Admin : AdminPublic{

    // mintNFT create a new NFT using a template ID
    //
    // Parameters: templateId: The ID of the Template
    // Parameters: metadata: The metadata to save inside Template 
    //
    // returns: @NFT the token that was created
    pub fun mintNFT(metadata: {String: String}, templateId: UInt64, buyer: Bool): @NFT {
      if(buyer == true){
        StarvingChildren.numberMintedByTemplate[templateId] = StarvingChildren.numberMintedByTemplate[templateId]! + 1
      }

      let edition = StarvingChildren.numberMintedByTemplate[templateId]!
      let newNFT: @NFT <- create NFT(edition: edition, metadata: metadata, templateId: templateId)

      StarvingChildren.totalSupply = StarvingChildren.totalSupply + 1
       
      return <- newNFT
    }

    // createTemplate create a template using a metadata 
    //
    // Parameters: metadataBuyer: The buyer's metadata to save inside Template 
    // Parameters: metadataCharity: The charity's metadata to save inside Template 
    // Parameters: price: The amount of FUSD to buy the template
    // Parameters: maxEditions: The max amount of editions
    // Parameters: creationDate: The creation date of the template
    //
    // returns: UInt64 the new template ID 
    pub fun createTemplate(metadataBuyer: {String: String}, metadataCharity: {String: String}, price: UFix64, maxEditions: UInt64, creationDate: UInt64): UInt64 {
      var newTemplate = Template(metadataBuyer: metadataBuyer, metadataCharity: metadataCharity, price: price, maxEditions: maxEditions, creationDate: creationDate)
      
      StarvingChildren.numberMintedByTemplate[newTemplate.templateId] = 0
      StarvingChildren.templateDatas[newTemplate.templateId] = newTemplate

      emit TemplateCreated(template: newTemplate)

      return newTemplate.templateId
    }
  }

  // This is the interface that users can cast their NFT Collection as
  // to allow others to deposit NFT into their Collection, allows for reading
  // the NFT IDs and borrow NFT in the Collection. 
  pub resource interface StarvingChildrenCollectionPublic {
    pub fun deposit(token: @NonFungibleToken.NFT)
    pub fun getIDs(): [UInt64]
    pub fun borrow(id: UInt64): &StarvingChildren.NFT?
  }

  // Collection is a resource that every user who owns NFTs 
  // will store in their account to manage their NFTS
  //
  pub resource Collection: StarvingChildrenCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
    // Dictionary of StarvingChildren conforming tokens
    // NFT is a resource type with a UInt64 ID field
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    init() {
      self.ownedNFTs <- {}
    }

    // withdraw removes an StarvingChildren from the Collection and moves it to the caller
    //
    // Parameters: withdrawID: The ID of the NFT 
    // that is to be removed from the Collection
    //
    // returns: @NonFungibleToken.NFT the token that was withdrawn
    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      // Remove the nft from the Collection
      let token <- self.ownedNFTs.remove(key: withdrawID) 
          ?? panic("Cannot withdraw: StarvingChildren does not exist in the collection")

      emit Withdraw(id: token.id, from: self.owner?.address)
      
      // Return the withdrawn token
      return <-token
    }

    // deposit takes a StarvingChildren and adds it to the Collections dictionary
    //
    // Paramters: token: the NFT to be deposited in the collection
    //
    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @StarvingChildren.NFT

      let id = token.id

      let oldToken <- self.ownedNFTs[id] <- token

      if self.owner?.address != nil {
        emit Deposit(id: id, to: self.owner?.address)
      }

      destroy oldToken
    }

    // getIDs returns an array of the IDs that are in the Collection
    pub fun getIDs(): [UInt64] {
      return self.ownedNFTs.keys
    }

    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
      return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    // borrow Returns a borrowed reference to a StarvingChildren in the Collection
    // so that the caller can read its ID
    //
    // Parameters: id: The ID of the NFT to get the reference for
    //
    // Returns: A reference to the NFT
    //
    pub fun borrow(id: UInt64): &StarvingChildren.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &StarvingChildren.NFT
      } else {
        return nil
      }
    }
    
    // If a transaction destroys the Collection object,
    // All the NFTs contained within are also destroyed!
    // Much like when Damian Lillard destroys the hopes and
    // dreams of the entire city of Houston.
    //
    destroy() {
      destroy self.ownedNFTs
    }
  }

  // This is the interface that users can cast their Profile as
  // to allow others to store name 
  pub resource interface PublicProfile {
    pub let name: String
  }

  // Profile is a resource that every user will store in 
  // their account to store name and termsAcceptedAt
  //
  pub resource Profile: PublicProfile {
    pub let name: String
    pub let termsAcceptedAt: UInt64

    init(name: String, termsAcceptedAt: UInt64) {
        self.name = name
        self.termsAcceptedAt = termsAcceptedAt
    }
  }

  // -----------------------------------------------------------------------
  // StarvingChildren contract-level function definitions
  // -----------------------------------------------------------------------

  // createProfile creates a new profile a user can store it in their 
  // account storage.
  //
  pub fun createProfile(name: String, termsAcceptedAt: UInt64): @Profile {
    let profile <- create Profile(name: name, termsAcceptedAt: termsAcceptedAt)
    return <- profile
  }

  // createEmptyCollection creates a new Collection a user can store 
  // it in their account storage.
  //
  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <-create StarvingChildren.Collection()
  }

  // createAdmin save admin address on contract
  //
  pub fun createAdmin(acc: AuthAccount){
    pre {
        StarvingChildren.adminAddress == nil : "Administrator already registered"
    }

    if(StarvingChildren.adminAddress == nil){
      acc.save<@Admin>(<- create Admin(), to: StarvingChildren.adminStoragePath)
      acc.link<&Admin{AdminPublic}>(StarvingChildren.adminPublicPath, target: StarvingChildren.adminStoragePath)
      StarvingChildren.adminAddress = acc.address;
    }
  }

  // getMetadatas get all metadatas stored in the contract
  //
  pub fun getMetadatas(): {UInt64: Template}{
    return StarvingChildren.templateDatas
  }

  // getAllTemplates get all templates stored in the contract
  //
  pub fun getAllTemplates(): [Template]{
    return StarvingChildren.templateDatas.values
  }

  // getAdminAddress get admin wallet address stored in the contract
  //
  pub fun getAdminAddress(): Address? {
    return StarvingChildren.adminAddress;
  }
  
  // getNumberMintedByTemplate get number nft minted by template
  //
  pub fun getNumberMintedByTemplate(templateId: UInt64): UInt64?{
    return StarvingChildren.numberMintedByTemplate[templateId];
  }

  init() {
    // Paths
    self.collectionPublicPath = /public/StarvingChildrenCollection
    self.collectionStoragePath = /storage/StarvingChildrenCollection
    self.adminStoragePath = /storage/StarvingChildrenAdmin
    self.adminPrivatePath = /private/StarvingChildrenAdmin
    self.adminPublicPath = /public/StarvingChildrenAdmin
    self.profileStoragePath = /storage/StarvingChildrenProfile
    self.profilePublicPath = /public/StarvingChildrenProfile

    self.nextTemplateId = 1
    self.nextNFTId = 1
    self.totalSupply = 0
    self.templateDatas = {}
    self.numberMintedByTemplate = {}
    self.adminAddress = nil
    
    emit ContractInitialized()
  }
}

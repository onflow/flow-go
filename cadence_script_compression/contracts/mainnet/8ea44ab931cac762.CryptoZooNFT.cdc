// SPDX-License-Identifier: UNLICENSED

import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x8ea44ab931cac762

pub contract CryptoZooNFT: NonFungibleToken {

  pub event ContractInitialized()
  pub event NFTTemplateCreated(typeID: UInt64, name: String, mintLimit: UInt64, priceUSD: UFix64, priceFlow: UFix64, metadata: {String: String}, isPack: Bool)
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, typeID: UInt64, serialNumber: UInt64, metadata: {String: String})
  pub event LandMinted(id: UInt64, typeID: UInt64, serialNumber: UInt64, metadata: {String: String}, coord: [UInt64])
  pub event PackOpened(id: UInt64, typeID: UInt64, name: String, address: Address?)
  
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let AdminStoragePath: StoragePath

  pub var totalSupply: UInt64
  access(self) var CryptoZooNFTTypeDict: {UInt64: CryptoZooNFTTemplate}
  access(self) var tokenMintedPerTypeID: {UInt64: UInt64}

  pub resource interface CryptoZooNFTCollectionPublic {
    pub fun deposit(token: @NonFungibleToken.NFT)
    pub fun getIDs(): [UInt64]
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
    pub fun borrowCryptoZooNFT(id: UInt64): &CryptoZooNFT.NFT? {
      post {
        (result == nil) || (result?.id == id):
          "Cannot borrow CryptoZooNFT NFT reference: The ID of the returned reference is incorrect"
      }
    }
  }

  pub struct CryptoZooNFTTemplate {
    pub let isPack: Bool
    pub let typeID: UInt64
    pub let name: String
    pub let description: String
    pub var mintLimit: UInt64
    pub var priceUSD: UFix64
    pub var priceFlow: UFix64
    pub var tokenMinted: UInt64
    pub var isExpired: Bool
    pub var isLand: Bool
    access(self) var metadata: {String: String}
    access(self) var timestamps: {String: UFix64}
    access(self) var coordMinted: {UInt64: [UInt64]}

    pub fun getMetadata(): {String: String} {
      return self.metadata
    }

    pub fun getTimestamps(): {String: UFix64} {
      return self.timestamps
    }

    pub fun checkIsCoordMinted(coord: [UInt64]): Bool {
      if (!self.isLand) {
        return false
      }
      if (self.coordMinted.containsKey(coord[0]) && self.coordMinted[coord[0]]!.contains(coord[1])) {
        return true
      }
      return false
    }

    pub fun addCoordMinted(coord: [UInt64]) {
      pre {
        !self.checkIsCoordMinted(coord: coord):
          "coord already exists"
        coord.length == 2:
          "invalid coord length"
      }
      if (!self.coordMinted.containsKey(coord[0])) {
        self.coordMinted[coord[0]] = [coord[1]] 
      } else {
        self.coordMinted[coord[0]]!.append(coord[1])
      }
    }

    pub fun getCoordMinted(): {UInt64: [UInt64]} {
      return self.coordMinted
    }

    access(contract) fun updatePriceUSD(newPriceUSD: UFix64) {
      self.priceUSD = newPriceUSD
    }

    access(contract) fun updatePriceFlow(newPriceFlow: UFix64) {
      self.priceFlow = newPriceFlow
    }

    access(contract) fun updateMintLimit(newMintLimit: UInt64) {
      self.mintLimit = newMintLimit
      self.unExpireNFTTemplate()
    }

    access(contract) fun updateMetadata(newMetadata: {String: String}) {
      self.metadata = newMetadata
    }

    access(contract) fun updateTimestamps(newTimestamps: {String: UFix64}) {
      self.timestamps = newTimestamps
    }

    access(contract) fun expireNFTTemplate() {
      self.isExpired = true
    }

    access(contract) fun unExpireNFTTemplate() {
      self.isExpired = false
    }

    init(initTypeID: UInt64, initIsPack: Bool, initName: String, initDescription: String, initMintLimit: UInt64, initPriceUSD: UFix64, initPriceFlow: UFix64, initMetadata: {String: String}, initTimestamps: {String: UFix64}, initIsLand: Bool){
      self.isPack = initIsPack
      self.typeID = initTypeID
      self.name = initName
      self.description = initDescription
      self.mintLimit = initMintLimit
      self.metadata = initMetadata
      self.timestamps = initTimestamps
      self.priceUSD = initPriceUSD
      self.priceFlow = initPriceFlow
      self.tokenMinted = 0
      self.isExpired = false
      self.isLand = initIsLand
      self.coordMinted = {}
      emit NFTTemplateCreated(typeID: initTypeID, name: initName, mintLimit: initMintLimit, priceUSD: initPriceUSD, priceFlow: initPriceFlow, metadata: initMetadata, isPack: self.isPack)
    }
  }

  pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
    pub let name: String
    pub let id: UInt64
    pub let typeID: UInt64
    pub let serialNumber: UInt64
    access(self) let coord: [UInt64]
    
    pub fun getViews(): [Type] {
      return [
        Type<MetadataViews.Display>(),
        Type<MetadataViews.HTTPThumbnail>(),
        Type<MetadataViews.IPFSThumbnail>()
      ]
    }

    pub fun resolveView(_ view: Type): AnyStruct? {
      switch view {
        case Type<MetadataViews.Display>():
          return MetadataViews.Display(
            name: self.name,
            description: self.getNFTTemplate()!.description,
          )
        case Type<MetadataViews.HTTPThumbnail>():
          return MetadataViews.HTTPThumbnail(
            uri: self.getNFTTemplate()!.getMetadata()["uri"]!,
            mimetype: self.getNFTTemplate()!.getMetadata()["mimetype"]!
          )
        case Type<MetadataViews.IPFSThumbnail>():
          return MetadataViews.IPFSThumbnail(
            cid: self.getNFTTemplate()!.getMetadata()["cid"]!,
            mimetype: self.getNFTTemplate()!.getMetadata()["mimetype"]!
          )
      }
      return nil
    }

    pub fun getNFTTemplate(): CryptoZooNFTTemplate? {
      return CryptoZooNFT.CryptoZooNFTTypeDict[self.typeID]
    }

    pub fun getCoord(): [UInt64] {
      return self.coord
    }

    init(initID: UInt64, initTypeID: UInt64, initName: String, initSerialNumber: UInt64, initCoord: [UInt64]) {
      self.id = initID
      self.name = initName
      self.typeID = initTypeID
      self.serialNumber = initSerialNumber
      self.coord = initCoord
    }
  }

  pub resource Collection: CryptoZooNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
      emit Withdraw(id: token.id, from: self.owner?.address)
      return <-token
    }

    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @CryptoZooNFT.NFT
      let id: UInt64 = token.id
      let oldToken <- self.ownedNFTs[id] <- token
      emit Deposit(id: id, to: self.owner?.address)
      destroy oldToken
    }

    pub fun batchDeposit(collection: @Collection) {
      let keys = collection.getIDs()
      for key in keys {
        self.deposit(token: <-collection.withdraw(withdrawID: key))
      }
      destroy collection
    }

    pub fun getIDs(): [UInt64] {
      return self.ownedNFTs.keys
    }

    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
      return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    pub fun borrowCryptoZooNFT(id: UInt64): &CryptoZooNFT.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &CryptoZooNFT.NFT
      } else {
        return nil
      }
    }

    pub fun openPack(packID: UInt64) {
      pre {
        self.ownedNFTs[packID] != nil:
          "invalid packID."
      }
      let packRef = (&self.ownedNFTs[packID] as auth &NonFungibleToken.NFT) as! &CryptoZooNFT.NFT
      let packTemplateInfo = packRef.getNFTTemplate()!
      if (!packTemplateInfo.isPack) {
        panic("NFT is not a pack.")
      }
      let pack <- self.ownedNFTs.remove(key: packID)
      emit PackOpened(id: packID, typeID: packTemplateInfo.typeID, name: packTemplateInfo.name, address: self.owner?.address)
      destroy pack
    }

    destroy() {
      destroy self.ownedNFTs
    }

    init () {
      self.ownedNFTs <- {}
    }
  }

  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <- create Collection()
  }

  pub resource Admin {

    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64) {
      pre {
        CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID):
          "invalid typeID"
        !CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.isExpired:
          "sold out"
      }
      let metadata = CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.getMetadata()
      if !CryptoZooNFT.tokenMintedPerTypeID.containsKey(typeID) {
        CryptoZooNFT.tokenMintedPerTypeID[typeID] = (0 as UInt64)
      }
      let serialNumber = CryptoZooNFT.tokenMintedPerTypeID[typeID]! + (1 as UInt64)
      emit Minted(id: CryptoZooNFT.totalSupply, typeID: typeID, serialNumber: serialNumber, metadata: metadata)
      
      recipient.deposit(token: <-create CryptoZooNFT.NFT(initID: CryptoZooNFT.totalSupply, initTypeID: typeID, initName: CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.name, initSerialNumber: serialNumber, initCoord: []))
      CryptoZooNFT.totalSupply = CryptoZooNFT.totalSupply + (1 as UInt64)

      if (serialNumber >= CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.mintLimit) {
        CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.expireNFTTemplate()
      }

      CryptoZooNFT.tokenMintedPerTypeID[typeID] = CryptoZooNFT.tokenMintedPerTypeID[typeID]! + (1 as UInt64)
    }

    pub fun mintLandNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, coord: [UInt64], nftName: String) {
      pre {
        CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID):
          "invalid typeID"
        !CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.isExpired:
          "sold out"
        coord.length == 2:
          "invalid coord length"
        !CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.checkIsCoordMinted(coord: coord):
          "invalid coord"
      }
      let metadata = CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.getMetadata()
      if !CryptoZooNFT.tokenMintedPerTypeID.containsKey(typeID) {
        CryptoZooNFT.tokenMintedPerTypeID[typeID] = (0 as UInt64)
      }
      let serialNumber = CryptoZooNFT.tokenMintedPerTypeID[typeID]! + (1 as UInt64)
      emit LandMinted(id: CryptoZooNFT.totalSupply, typeID: typeID, serialNumber: serialNumber, metadata: metadata, coord: coord)
      recipient.deposit(token: <-create CryptoZooNFT.NFT(initID: CryptoZooNFT.totalSupply, initTypeID: typeID, initName: nftName, initSerialNumber: serialNumber, initCoord: coord))
      
      CryptoZooNFT.totalSupply = CryptoZooNFT.totalSupply + (1 as UInt64)
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.addCoordMinted(coord: coord)

      if (serialNumber >= CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.mintLimit) {
        CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.expireNFTTemplate()
      }

      CryptoZooNFT.tokenMintedPerTypeID[typeID] = CryptoZooNFT.tokenMintedPerTypeID[typeID]! + (1 as UInt64)
    }
    
    pub fun updateTemplateMetadata(typeID: UInt64, newMetadata: {String: String}): CryptoZooNFT.CryptoZooNFTTemplate {
      pre {
        CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID) != nil:
          "Token with the typeID does not exist."
      }
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.updateMetadata(newMetadata: newMetadata)
      return CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!
    }

    pub fun createNFTTemplate(typeID: UInt64, isPack: Bool, name: String, description: String, mintLimit: UInt64, priceUSD: UFix64, priceFlow: UFix64, metadata: {String: String}, timestamps: {String: UFix64}, isLand: Bool){
      pre {
        !CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID):
          "NFT template with the same typeID already exists."
      }
      let newNFTTemplate = CryptoZooNFTTemplate(
        initTypeID: typeID,
        initIsPack: isPack,
        initName: name,
        initDescription: description,
        initMintLimit: mintLimit,
        initPriceUSD: priceUSD,
        initPriceFlow: priceFlow,
        initMetadata: metadata,
        initTimestamps: timestamps,
        initIsLand: isLand,
      )
      CryptoZooNFT.CryptoZooNFTTypeDict[newNFTTemplate.typeID] = newNFTTemplate
    }

    pub fun updateNFTTemplatePriceUSD(typeID: UInt64, newPriceUSD: UFix64) {
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.updatePriceUSD(newPriceUSD: newPriceUSD)
    }

    pub fun updateNFTTemplatePriceFlow(typeID: UInt64, newPriceFlow: UFix64) {
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.updatePriceFlow(newPriceFlow: newPriceFlow)
    }

    pub fun updateNFTTemplateMintLimit(typeID: UInt64, newMintLimit: UInt64) {
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.updateMintLimit(newMintLimit: newMintLimit)
    }

    pub fun expireNFTTemplate(typeID: UInt64) {
      CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.expireNFTTemplate()
    }

    pub fun createNewAdmin(): @Admin {
      return <- create Admin()
    }
  }

  pub fun checkMintLimit(typeID: UInt64): UInt64? {
    if let token = CryptoZooNFT.CryptoZooNFTTypeDict[typeID] {
      return token.mintLimit
    } else {
      return nil
    }
  }

  pub fun checkNFTTemplates(): [CryptoZooNFTTemplate]{
    return CryptoZooNFT.CryptoZooNFTTypeDict.values
  }

  pub fun checkNFTTemplatesTypeIDs(): [UInt64] {
    return CryptoZooNFT.CryptoZooNFTTypeDict.keys
  }

  pub fun isNFTTemplateExpired(typeID: UInt64): Bool {
    if (!CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID)) {
      return true
    }
    return CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.isExpired
  }

  pub fun isNFTTemplateExist(typeID: UInt64): Bool {
    if CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID) {
      return true
    }
    return false
  }

  pub fun getNFTTemplateMetadata(typeID: UInt64): {String: String} {
    if !CryptoZooNFT.CryptoZooNFTTypeDict.containsKey(typeID) {
      panic("invalid typeID")
    }
    return CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.getMetadata()
  }

  pub fun getNFTTemplateByTypeID(typeID: UInt64): CryptoZooNFTTemplate {
    return CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!
  }

  pub fun checkIsCoordMinted(typeID: UInt64, coord: [UInt64]): Bool {
    return CryptoZooNFT.CryptoZooNFTTypeDict[typeID]!.checkIsCoordMinted(coord: coord)
  }

  init() {
    self.CollectionStoragePath = /storage/CryptoZooCollection
    self.CollectionPublicPath = /public/CryptoZooCollection
    self.AdminStoragePath = /storage/CryptoZooAdmin

    self.totalSupply = 0

    self.tokenMintedPerTypeID = {}
    self.CryptoZooNFTTypeDict = {}

    let admin <- create Admin()
    self.account.save(<-admin, to: self.AdminStoragePath)
    emit ContractInitialized()
  }
}

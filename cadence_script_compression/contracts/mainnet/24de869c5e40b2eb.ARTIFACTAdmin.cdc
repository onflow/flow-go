// SPDX-License-Identifier: Unlicense

import NonFungibleToken from 0x1d7e57aa55817448
import ARTIFACTPack, ARTIFACT, Interfaces from 0x24de869c5e40b2eb

pub contract ARTIFACTAdmin: Interfaces {

  // -----------------------------------------------------------------------
  // ARTIFACTAdmin contract-level fields.
  // These contain actual values that are stored in the smart contract.
  // -----------------------------------------------------------------------

  /// Path where the `Admin` is stored
  pub let ARTIFACTAdminStoragePath: StoragePath

  /// Path where the private capability for the `Admin` is available
  pub let ARTIFACTAdminPrivatePath: PrivatePath

  /// Path where the private capability for the `Admin` is available
  pub let ARTIFACTAdminOpenerPrivatePath: PrivatePath
  
  /// Path where the `AdminTokenReceiver` is stored
  pub let ARTIFACTAdminTokenReceiverStoragePath: StoragePath

  /// Path where the public capability for the `AdminTokenReceiver` is available
  pub let ARTIFACTAdminTokenReceiverPublicPath: PublicPath

  /// Path where the private capability for the `AdminTokenReceiver` is available
  pub let ARTIFACTAdminTokenReceiverPrivatePath: PrivatePath
    
  // -----------------------------------------------------------------------
  // ARTIFACTAdmin contract-level Composite Type definitions
  // -----------------------------------------------------------------------
  // These are just *definitions* for Types that this contract
  // and other accounts can use. These definitions do not contain
  // actual stored values, but an instance (or object) of one of these Types
  // can be created by this contract that contains stored values.
  // ----------------------------------------------------------------------- 

  // ARTIFACTAdmin is a resource that ARTIFACTAdmin creator has
  // to mint NFT, create templates, open pack, create pack and create pack template
  // 
  pub resource Admin : Interfaces.ARTIFACTAdminOpener {
    
    // openPack create new NFTs randomly
    //
    // Parameters: userPack: The userPack reference with all pack informations
    // Parameters: packID: The pack ID
    // Parameters: owner: The pack owner
    //
    // returns: @[NonFungibleToken.NFT] the NFT created by the pack
    pub fun openPack(userPack: &{Interfaces.IPack}, packID: UInt64, owner: Address): @[NonFungibleToken.NFT] {
      pre {
          !userPack.isOpen : "User Pack must be closed"    
          !ARTIFACTPack.checkPackTemplateLockStatus(packTemplateId: userPack.templateId): "pack template is locked"
      }

      var nfts: @[NonFungibleToken.NFT] <- []
      let packTemplate = ARTIFACTPack.getPackTemplate(templateId: userPack.templateId)! 
      var templateIDs: [UInt64] = self.getTemplateIdsFromPacksAvailable(packTemplate: &packTemplate as &ARTIFACTPack.PackTemplate)

      if templateIDs.length > 19 {
        panic("Max number of template IDs inside a pack is 19")
      }

      var i: Int = 0
      while i < templateIDs.length {
        let token <- self.mintNFT(templateId: templateIDs[i], packID: packID, owner: owner)
        nfts.append(<- token)
        i = i + 1
      } 

      ARTIFACTPack.updatePackTemplate(packTemplate: packTemplate)

      return <- nfts
    }

    access(self) fun getTemplateIdsFromPacksAvailable(packTemplate: &ARTIFACTPack.PackTemplate) : [UInt64] {
        pre {
          packTemplate.packsAvailable.length > 0 : "No pack available"
        }

        let indexPackAvailable = unsafeRandom() % UInt64(packTemplate.packsAvailable.length)
        let templateIDs = packTemplate.packsAvailable[indexPackAvailable]!
        packTemplate.packsAvailable.remove(at: indexPackAvailable)

        return templateIDs
    }

    // updateLockStatus to update lock status of packs
    //
    // Parameters: packTemplateId: The pack template ID
    // Parameters: lockStatus: The lock status of pack template
    //
    pub fun updateLockStatus(packTemplateId: UInt64, lockStatus: Bool) {
      ARTIFACTPack.updateLockStatus(packTemplateId: packTemplateId, lockStatus: lockStatus)
    }

    // createPack create a new Pack NFT 
    //
    // Parameters: packTemplate: The pack template with all information
    // Parameters: adminRef: Admin capability to open Pack 
    // Parameters: owner: The Pack owner
    // Parameters: listingID: The sale offer ID
    //
    // returns: @NonFungibleToken.NFT the pack that was created
    pub fun createPack(packTemplate: {Interfaces.IPackTemplate}, adminRef: Capability<&{Interfaces.ARTIFACTAdminOpener}>, owner: Address, listingID: UInt64) : @NonFungibleToken.NFT {
      return <- ARTIFACTPack.createPack(packTemplate: packTemplate, adminRef: adminRef, owner: owner, listingID: listingID)
    }

    // createPackTemplate create a new PackTemplate 
    //
    // Parameters: metadata: The flexible field 
    // Parameters: totalSupply: The max quantity of pack to sell
    // Parameters: maxQuantityPerTransaction: The max quantity of pack to sell into one transaction
    // Parameters: packsAvailable: The available pack options 
    //
    // returns: UInt64 the new pack template ID 
    pub fun createPackTemplate(metadata: {String: String}, totalSupply: UInt64, maxQuantityPerTransaction: UInt64, packsAvailable: [[UInt64]]) : UInt64 {
      var newPackTemplate = ARTIFACTPack.createPackTemplate(metadata: metadata, totalSupply: totalSupply, maxQuantityPerTransaction: maxQuantityPerTransaction, packsAvailable: packsAvailable)

      return newPackTemplate.templateId
    }

    // mintNFT create a new NFT using a template ID
    //
    // Parameters: templateId: The Template ID
    // Parameters: packID: The pack ID
    // Parameters: owner: The pack owner
    //
    // returns: @NFT the token that was created
    pub fun mintNFT(templateId: UInt64, packID: UInt64, owner: Address): @ARTIFACT.NFT {
      pre {
          ARTIFACT.templateDatas.containsKey(templateId) : "templateId not found"       
          ARTIFACT.templateDatas[templateId]!.maxEditions > ARTIFACT.numberMintedByTemplate[templateId]! : "template not available"
      }

      return <- ARTIFACT.createNFT(templateId: templateId, packID: packID, owner: owner)
    }

    // createTemplate create a template using a metadata 
    //
    // Parameters: metadata: The buyer's metadata to save inside Template 
    // Parameters: maxEditions: The max amount of editions
    // Parameters: creationDate: The creation date of the template
    // Parameters: rarity: The rairty of the template
    // Parameters: databaseID: The database ID off-chain information 
    //
    // returns: UInt64 the new template ID 
    pub fun createTemplate(metadata: {String: String}, maxEditions: UInt64, creationDate: UInt64, rarity: UInt64, databaseID: String): UInt64 {
      var newTemplate = ARTIFACT.createTemplate(metadata: metadata, maxEditions: maxEditions, creationDate: creationDate, rarity: rarity, databaseID: databaseID)

      self.mintFirstNFT(templateId: newTemplate.templateId)

      return newTemplate.templateId
    }

    // mintFirstNFT create first NFT of the template and save it inside admin wallet
    //
    // Parameters: templateId: The ID of the Template 
    //
    access(self) fun mintFirstNFT(templateId: UInt64) {
      let nft : @ARTIFACT.NFT <- self.mintNFT(templateId: templateId, packID: 0, owner: self.owner?.address!)

      let collectionRef = getAccount(self.owner?.address!).getCapability(ARTIFACT.collectionPublicPath)
        .borrow<&{ARTIFACT.CollectionPublic}>()
        ?? panic("Could not get public collection reference")

      collectionRef.deposit(token: <- nft)
    }
  }

  pub resource SuperAdmin {
    access(self) var admins: [Address]

    init() {
      self.admins = []
    }

    pub fun givePermission(address: Address) {
      pre {
        self.admins.length <= 310 : "Max limit is 310"
      }
      self.admins.append(address)
    }

    pub fun revokePermission(address: Address) {
      var i: Int = 0
      while i < self.admins.length {
        if self.admins[i] == address {
          self.admins.remove(at: i)
        }
        i = i + 1
      }
    }
  }

  pub resource interface AdminTokenReceiverPublic {
      pub fun receiveAdmin(adminRef: Capability<&Admin> )
      pub fun receiveAdminCapabilityOpener(adminRef: Capability<&{Interfaces.ARTIFACTAdminOpener}> )
      pub fun receiveSuperAdmin(superAdminRef: @SuperAdmin)
  }

  pub resource AdminTokenReceiver: AdminTokenReceiverPublic {

    access(self) var adminRef: Capability<&Admin>?
    access(self) var adminCapabilityOpener: Capability<&{Interfaces.ARTIFACTAdminOpener}>?
    access(self) var superAdminRef: @[SuperAdmin]

    init() {
      self.adminRef = nil
      self.adminCapabilityOpener = nil
      self.superAdminRef <- []
    }

    destroy() {
      destroy self.superAdminRef
    }

    pub fun receiveAdmin(adminRef: Capability<&Admin> ) {
      self.adminRef = adminRef
    }
    
    pub fun receiveAdminCapabilityOpener(adminRef: Capability<&{Interfaces.ARTIFACTAdminOpener}> ) {
      self.adminCapabilityOpener = adminRef
    }

    pub fun receiveSuperAdmin(superAdminRef: @SuperAdmin) {
      self.superAdminRef.append(<- superAdminRef) 
    }

    pub fun getAdminRef(): &Admin? {
      return self.adminRef!.borrow()
    }

    pub fun getAdminOpenerRef(): Capability<&{Interfaces.ARTIFACTAdminOpener}> {
      return self.adminCapabilityOpener!
    }

    pub fun getSuperAdminRef(): &SuperAdmin {
      if( self.superAdminRef.length == 0) {
        panic("Can't access super admin permission")
      }
      return &self.superAdminRef[0] as auth &SuperAdmin
    }
  }

  // -----------------------------------------------------------------------
  // ARTIFACTAdmin contract-level function definitions
  // -----------------------------------------------------------------------

  // createAdminTokenReceiver create a admin token receiver
  //
  pub fun createAdminTokenReceiver(): @AdminTokenReceiver {
    return <- create AdminTokenReceiver()
  }
  
  init() {
    // Paths
    self.ARTIFACTAdminStoragePath = /storage/ARTIFACTAdmin
    self.ARTIFACTAdminPrivatePath = /private/ARTIFACTAdmin
    self.ARTIFACTAdminOpenerPrivatePath = /private/ARTIFACTAdminOpener
    self.ARTIFACTAdminTokenReceiverStoragePath = /storage/ARTIFACTAdminTokenReceiver
    self.ARTIFACTAdminTokenReceiverPublicPath = /public/ARTIFACTAdminTokenReceiver
    self.ARTIFACTAdminTokenReceiverPrivatePath = /private/ARTIFACTAdminTokenReceiver
    
    if(self.account.borrow<&{ARTIFACTAdmin.AdminTokenReceiverPublic}>(from: self.ARTIFACTAdminTokenReceiverStoragePath) == nil) {
        self.account.save<@ARTIFACTAdmin.AdminTokenReceiver>(<- create AdminTokenReceiver(), to: self.ARTIFACTAdminTokenReceiverStoragePath)
        self.account.link<&{ARTIFACTAdmin.AdminTokenReceiverPublic}>(self.ARTIFACTAdminTokenReceiverPublicPath, target: self.ARTIFACTAdminTokenReceiverStoragePath)
    }

    let adminTokenReceiver = self.account.borrow<&ARTIFACTAdmin.AdminTokenReceiver>(from: self.ARTIFACTAdminTokenReceiverStoragePath)
          ?? panic("Could not borrow user ARTIFACTAdmin admin token reference")
    adminTokenReceiver.receiveSuperAdmin(superAdminRef: <- create SuperAdmin())

    if self.account.borrow<&ARTIFACTAdmin.Admin>(from: ARTIFACTAdmin.ARTIFACTAdminStoragePath) == nil {
      self.account.save<@ARTIFACTAdmin.Admin>(<- create ARTIFACTAdmin.Admin(), to: ARTIFACTAdmin.ARTIFACTAdminStoragePath)
    }

    self.account.link<&ARTIFACTAdmin.Admin>(ARTIFACTAdmin.ARTIFACTAdminPrivatePath, target: ARTIFACTAdmin.ARTIFACTAdminStoragePath)!
    self.account.link<&{Interfaces.ARTIFACTAdminOpener}>(ARTIFACTAdmin.ARTIFACTAdminOpenerPrivatePath, target: ARTIFACTAdmin.ARTIFACTAdminStoragePath)!
  }
}
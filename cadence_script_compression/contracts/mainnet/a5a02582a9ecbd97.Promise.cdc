pub contract Promise {

  access(self) var templates: {UInt32: Template}
  access(self) var lastTemplateId: UInt32
  access(self) let reward: UInt64
  pub let collectionStoragePath: StoragePath 
  pub let collectionPublicPath: PublicPath
  pub let vaultStoragePath: StoragePath 
  pub let vaultPublicPath: PublicPath
  pub let profileStoragePath: StoragePath 
  pub let profilePublicPath: PublicPath

  init() {
    self.templates = {}
    self.lastTemplateId = 0
    self.reward = 3
    self.collectionStoragePath = /storage/PromiseCollection
    self.collectionPublicPath = /public/PromiseCollection
    self.vaultStoragePath = /storage/PromiseVault
    self.vaultPublicPath = /public/PromiseVault
    self.profileStoragePath = /storage/PromiseProfile
    self.profilePublicPath = /public/PromiseProfile
  }

  pub struct Template {
    pub let author: Address
    pub let authorName: String
    pub let content: String
    pub let templateId: UInt32
    pub(set) var edition: UInt32
    
    init(author: Address, authorName: String, content: String, edition: UInt32, templateId: UInt32) {
      self.edition = edition
      self.author = author
      self.content = content
      self.templateId = templateId
      self.authorName = authorName
    }

    pub fun incrementEdition() {
      self.edition = self.edition + 1
    }
  }

   pub struct NftInfo {
    pub let data: Template
    pub let createdAt: UFix64
    
    init(data: Template, createdAt: UFix64) {
      self.data = data
      self.createdAt = createdAt
    }
  }

  pub resource NFT {
    pub let data: Template
    pub let edition: UInt32
    pub let createdAt: UFix64

    init(templateId: UInt32, author: Address, content: String) {
      if(Promise.templates[templateId] == nil) {
        panic("Template ID does not exist")
      }
      Promise.templates[templateId]!.incrementEdition()
      let template = Promise.templates[templateId]!
      self.data = Template(
        author: template.author,
        authorName: template.authorName,
        content: template.content,
        edition: template.edition,
        templateId: template.templateId)
      self.edition = Promise.templates[templateId]!.edition
      self.createdAt = getCurrentBlock().timestamp
    }
  }

  pub resource interface PublicCollection {
    pub fun list(): [NftInfo]
  }

  pub resource Collection: PublicCollection {
    pub var nfts: @{UInt32: NFT}

    init() {
      self.nfts <- {}
    }

    destroy() {
      destroy self.nfts
    }

    pub fun deposit(nft: @NFT) {
      if(self.nfts[nft.data.templateId] != nil) {
        panic("You have already holded accountable for this promise")
      }
      self.nfts[nft.data.templateId] <-! nft
    }

    pub fun list(): [NftInfo] {
      var results: [NftInfo] = []
      for key in self.nfts.keys {
        let nft = &self.nfts[key] as &NFT
        let nftInfo = NftInfo(data: nft.data, createdAt: nft.createdAt)
        results.append(nftInfo)
      }
      return results
    }
  }

  pub resource interface PublicVault {
    pub fun deposit(temporaryVault: @Vault)
    pub var balance: UInt64
  }

  pub resource Vault: PublicVault {
    pub var balance: UInt64

    init(balance: UInt64) {
      self.balance = balance
    }

    pub fun withdraw(amount: UInt64): @Vault {
      if(self.balance < amount) {
        let temporaryVault <- create Vault(balance: 0)
        return <- temporaryVault
      }
      self.balance = self.balance - amount
      let temporaryVault <- create Vault(balance: amount)
      return <- temporaryVault
    }

    pub fun deposit(temporaryVault: @Vault) {
      self.balance = self.balance + temporaryVault.balance
      destroy temporaryVault
    }
  }

  pub resource interface PublicProfile {
    pub let name: String
  }

  pub resource Profile: PublicProfile {
    pub let name: String
    pub let termsAcceptedAt: UInt64

    init(name: String, termsAcceptedAt: UInt64) {
        self.name = name
        self.termsAcceptedAt = termsAcceptedAt
    }
  }

  pub fun createProfile(name: String, termsAcceptedAt: UInt64): @Profile {
    let profile <- create Profile(name: name, termsAcceptedAt: termsAcceptedAt)
    return <- profile
  }

  pub fun createVault(): @Vault {
    let vault <- create Vault(balance: 0)
    return <- vault
  }

  pub fun createCollection(): @Collection {
    let collection <- create Collection()
    return <- collection
  }

  pub fun createTemplate(author: Address, content: String): @NFT {
    let account = getAccount(author)
    let vault = account.getCapability<&{PublicVault}>(self.vaultPublicPath).borrow()
      ?? panic ("Could not borrow Vault")

    let userProfile = account.getCapability<&{Promise.PublicProfile}>(Promise.profilePublicPath).borrow()
      ?? panic ("Could not borrow user profile")

    let reward <- create Vault(balance: self.reward)
    vault.deposit(temporaryVault: <- reward)
    self.lastTemplateId = self.lastTemplateId + 1
    let template = Template(
      author: author,
      authorName: userProfile.name,
      content: content,
      edition: 0,
      templateId: self.lastTemplateId)
    self.templates[self.lastTemplateId] = template    
    let nft <- create NFT(templateId: self.lastTemplateId, author: author, content: content)
    return <- nft
  }

  pub fun createNextEdition(author: Address, templateId: UInt32, payment: @Vault): @NFT? {
    if(payment.balance < 1) {
      panic("Not enough balance")
    }
    let template = self.templates[templateId]!
    if(template.author == author) {
      panic("Cannot hold accountable for own promises")
    }

    let nft <-! create NFT(templateId: templateId, author: template.author, content: template.content)
    destroy payment
    return <- nft
  }

}

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import Domains from 0xaac12f8ba7b6bacd


// Flowns is the core contract of FNS, Flowns define Root domain and admin resource
pub contract Flowns {
  // paths
  pub let FlownsAdminPrivatePath: PrivatePath
  pub let FlownsAdminStoragePath: StoragePath
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let CollectionPrivatePath: PrivatePath

  // variables
  pub var totalRootDomains: UInt64
  // status that set register pause or not
  priv var isPause: Bool
  // for domain name on-chain validator 
  priv var forbidChars: String
  // events


  pub event RootDomainDestroyed(id: UInt64)

  pub event RootDomainCreated(name: String, nameHash: String, id: UInt64)

  pub event RenewDomain(name: String, nameHash: String, duration: UFix64, price: UFix64)
  
  pub event RootDomainPriceChanged(name: String, key: Int, price: UFix64)

  pub event RootDomainVaultWithdrawn(name: String, amount: UFix64)

  pub event RootDomainServerAdded()

  pub event FlownsAdminCreated()

  pub event RootDomainVaultChanged()

  pub event FlownsPaused()

  pub event FlownsActivated()

  pub event FlownsForbidCharsUpdated(before: String, after: String)

  pub event RootDomainMaxLengthUpdated(domainId: UInt64, before: Int, after: Int)

  pub event RootDomainCommissionRateUpdated(domainId: UInt64, before: UFix64, after: UFix64)

  pub event RootDomainMintDurationUpdated(domainId: UInt64, before: UFix64, after: UFix64)

  pub event DomainRegisterCommissionAllocated(domainId: UInt64, nammeHash: String, amount: UFix64, commissionAmount: UFix64, refer: Address)


  // structs 
  pub struct RootDomainInfo {
    pub let id: UInt64
    pub let name: String
    pub let nameHash: String
    pub let domainCount: UInt64
    pub let minRentDuration: UFix64
    pub let maxDomainLength: Int
    pub let prices: {Int: UFix64}
    pub let commissionRate: UFix64


    init(
      id: UInt64,
      name: String,
      nameHash: String,
      domainCount: UInt64,
      minRentDuration: UFix64,
      maxDomainLength: Int,
      prices: {Int: UFix64},
      commissionRate: UFix64
    ) {
      self.id = id
      self.name = name
      self.nameHash = nameHash
      self.domainCount = domainCount
      self.minRentDuration = minRentDuration
      self.maxDomainLength = maxDomainLength
      self.prices = prices
      self.commissionRate = commissionRate
    }
  }

  // resources
  // Rootdomain is the root of domain name
  // ex. domain 'fns.flow' 'flow' is the root domain name, and save as a resource by RootDomain
  pub resource RootDomain {
    pub let id: UInt64

    pub let name: String
    
    // namehash is calc by eth-ens-namehash
    pub let nameHash: String

    pub var domainCount: UInt64

    // Here is the vault to receive domain rent fee, every root domain has his own vault
    // you can call Flowns.getRootVaultBalance to get balance
    priv var domainVault: @FungibleToken.Vault

    // Here is the prices store for domain rent fee
    // When user register or renew a domain ,the rent price is get from here, and price store by {domains length: flow per second}
    // If cannot get price, then register will not open
    priv var prices: {Int: UFix64}

    priv var minRentDuration: UFix64

    priv var maxDomainLength: Int

    priv var commissionRate: UFix64

    // Server store the collection private resource to manage the domains
    // Server need to init before open register
    access(self) var server: Capability<&Domains.Collection>?

    init(id: UInt64, name: String, nameHash: String, vault: @FungibleToken.Vault){
      self.id = id
      self.name = name
      self.nameHash = nameHash
      self.domainCount = 0
      self.domainVault <- vault
      self.prices = {}
      self.server = nil
      self.minRentDuration = 3153600.00
      self.maxDomainLength = 30
      self.commissionRate = 0.0
    }

    // Set CollectionPrivate to RootDomain resource
    pub fun addCapability(_ cap: Capability<&Domains.Collection>) {
      pre {
        cap.check() : "Invalid server capablity"
        self.server == nil : "Server already set"
      }
      self.server = cap
      
      emit RootDomainServerAdded()

    }

    // Query root domain info
    pub fun getRootDomainInfo() : RootDomainInfo {
      return RootDomainInfo(
        id: self.id,
        name: self.name,
        nameHash: self.nameHash,
        domainCount: self.domainCount,
        minRentDuration: self.minRentDuration,
        maxDomainLength: self.maxDomainLength,
        prices: self.prices,
        commissionRate: self.commissionRate

      )
    }

    // Query root domain vault balance
    pub fun getVaultBalance() : UFix64 {
      pre {
        self.domainVault != nil : "Vault not init yet..."
      }
      return self.domainVault.balance
    }

    pub fun getPrices(): {Int: UFix64} {
      return self.prices
    }

    // Mint domain
    access(account) fun mintDomain(name: String, duration: UFix64, receiver: Capability<&{NonFungibleToken.Receiver}>){
      pre {
        self.server != nil : "Domains collection has not been linked to the server"
      }
      let nameHash = Flowns.getDomainNameHash(name: name, parentNameHash: self.nameHash)
    
      let expiredTime = getCurrentBlock().timestamp + duration

      self.server!.borrow()!.mintDomain(name: name, nameHash: nameHash, parentName: self.name, expiredAt: expiredTime, receiver: receiver)
      self.domainCount = self.domainCount + (1 as UInt64)

    }
    // Set domain rent fee
    pub fun setPrices(key: Int, price: UFix64) {
      self.prices[key]= price
      
      emit RootDomainPriceChanged(name: self.name, key: key, price: price)
    }

    access(account) fun setMinRentDuration(_ duration: UFix64) {
      let oldDuration = self.minRentDuration
      self.minRentDuration = duration

      emit RootDomainMintDurationUpdated(domainId: self.id, before: oldDuration, after: duration)
    }

    access(account) fun setMaxDomainLength(_ length: Int) {
      let oldLength = self.maxDomainLength
      self.maxDomainLength = length

      emit RootDomainMaxLengthUpdated(domainId: self.id, before: oldLength, after: length)
    }

    access(account) fun setCommissionRate(_ rate: UFix64) {
      let oldRate = self.commissionRate
      self.commissionRate = rate

      emit RootDomainCommissionRateUpdated(domainId: self.id, before: oldRate, after: rate)
    }

    // Renew domain
    pub fun renewDomain(domain: &Domains.NFT, duration: UFix64, feeTokens: @FungibleToken.Vault) {
      pre {
        !Domains.isDeprecated(nameHash: domain.nameHash, domainId: domain.id) : "Domain already deprecated ..."
      }
      // When domain name longer than 10, the price will set by 10 price
      var len = domain.name.length
      if len > 10 {
        len = 10
      }
      let price = self.getPrices()[len]

      if domain.parent != self.name {
        panic("domain not root domain's sub domain")
      }
      if duration < self.minRentDuration {
        panic("Duration must greater than min rent duration ".concat(self.minRentDuration.toString()))
      }
     
      if price == 0.0 || price == nil {
        panic("Can not renew domain, rent price not set yet")
      }
      
      // Calc rent price
      let rentPrice = price! * duration 
      
      let rentFee = feeTokens.balance
      
      // check the rent fee
      if rentFee < rentPrice {
        panic("Not enough fee to renew your domain.")
      }

      // Receive rent fee
      self.domainVault.deposit(from: <- feeTokens)

      let expiredAt = Domains.getExpiredTime(domain.nameHash)! + UFix64(duration)
      // Update domain's expire time with Domains expired mapping
      Domains.updateExpired(nameHash: domain.nameHash, time: expiredAt )

      emit RenewDomain(name: domain.name, nameHash: domain.nameHash, duration: duration, price: rentFee )

    }

    // Register domain
    pub fun registerDomain(name: String, duration: UFix64, feeTokens: @FungibleToken.Vault, receiver: Capability<&{NonFungibleToken.Receiver}>, refer: Address? ){
      pre {
        self.server != nil : "Your client has not been linked to the server"
        name.length <= self.maxDomainLength : "Domain name can not exceed max length: ".concat(self.maxDomainLength.toString())
      }

      let nameHash = Flowns.getDomainNameHash(name: name, parentNameHash: self.nameHash)
    
      if Flowns.available(nameHash: nameHash) == false {
        panic("Domain not available")
      }

      // same as renew domain
      var len = name.length
      if len > 10 {
        len = 10
      }

      let price = self.getPrices()[len]
      // limit the register and renew time longer than one year

      if duration < self.minRentDuration {
        panic("Duration must geater than min rent duration, expect: ".concat(self.minRentDuration.toString()))
      }
      if price == 0.0 || price == nil {
        panic("Can not register domain, rent price not set yet")
      }

      let rentPrice = price! * duration 
      
      let rentFee = feeTokens.balance

      if rentFee < rentPrice {
         panic("Not enough fee to rent your domain, expect: ".concat(rentPrice.toString()))
      }

      let expiredTime = getCurrentBlock().timestamp + UFix64(duration)

      // distribution of commission
      if self.commissionRate > 0.0 && refer != nil {
        let commissionFee  = rentFee * self.commissionRate
        
        let referAcc = getAccount(refer!)

        let collectionCap = referAcc.getCapability<&{Domains.CollectionPublic}>(Domains.CollectionPublicPath) 
        let collection = collectionCap.borrow()
        if collection != nil {
          let ids = collection!.getIDs()
          if ids.length > 0 {
            // default domains as a receiver
            let id = ids[0]
            let domain: &{Domains.DomainPublic} = collection!.borrowDomain(id: id)
            if domain.receivable == true {
              domain.depositVault(from: <- feeTokens.withdraw(amount: commissionFee))
              emit DomainRegisterCommissionAllocated(domainId: self.id, nammeHash: nameHash, amount: rentFee, commissionAmount: commissionFee, refer: refer!)
            }
          }
        }
      }

      self.domainVault.deposit(from: <- feeTokens)

      self.server!.borrow()!.mintDomain(name: name, nameHash: nameHash, parentName: self.name, expiredAt: expiredTime, receiver: receiver)

      self.domainCount = self.domainCount + (1 as UInt64)

    }

    // Withdraw vault fee 
    access(account) fun withdrawVault(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
      let vault = receiver.borrow()!
      vault.deposit(from: <- self.domainVault.withdraw(amount: amount))
      
      emit RootDomainVaultWithdrawn(name: self.name, amount: amount)
    }

    access(account) fun changeRootDomainVault(vault: @FungibleToken.Vault) {

      let balance = self.getVaultBalance()

      if balance > 0.0 {
        panic("Please withdraw the balance of the previous vault first ")
      }

      let preVault <- self.domainVault <- vault
      
      // clean the price
      self.prices = {}
      emit RootDomainVaultChanged()
      destroy preVault
    }

    destroy(){
        log("Destroy Root domains")
        destroy self.domainVault
        emit RootDomainDestroyed(id: self.id)
    }

  }


  // Root domain public interface for fns user
  pub resource interface RootDomainCollectionPublic {

    pub fun getDomainInfo(domainId: UInt64) : RootDomainInfo

    pub fun getAllDomains(): {UInt64: RootDomainInfo}

    pub fun renewDomain(domainId: UInt64, domain: &Domains.NFT, duration: UFix64, feeTokens: @FungibleToken.Vault)

    pub fun registerDomain(domainId: UInt64, name: String, duration: UFix64, feeTokens: @FungibleToken.Vault, receiver: Capability<&{NonFungibleToken.Receiver}>,  refer: Address? )

    pub fun getPrices(domainId: UInt64): {Int: UFix64}

    pub fun getVaultBalance(domainId: UInt64): UFix64

  }

  // Manager resource
  pub resource interface RootDomainCollectionAdmin {

    access(account) fun createRootDomain(
      name: String, 
      vault: @FungibleToken.Vault
    )

    access(account) fun withdrawVault(domainId: UInt64, receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64)

    access(account) fun changeRootDomainVault(domainId: UInt64, vault: @FungibleToken.Vault)

    access(account) fun setPrices(domainId: UInt64, len: Int, price: UFix64)

    access(account) fun setMinRentDuration(domainId: UInt64, duration: UFix64)

    access(account) fun setMaxDomainLength(domainId: UInt64, length: Int)

    access(account) fun setCommissionRate(domainId: UInt64, rate: UFix64)
    
    access(account) fun mintDomain(domainId: UInt64, name: String, duration: UFix64, receiver: Capability<&{NonFungibleToken.Receiver}>)
  }

  // Root domain Collection 
  pub resource RootDomainCollection: RootDomainCollectionPublic, RootDomainCollectionAdmin {
    // Root domains
    access(account) var domains: @{UInt64: RootDomain}

    init(
    ) {
      self.domains <- {}
    }

    // Create root domain
    access(account) fun createRootDomain(
      name: String, 
      vault: @FungibleToken.Vault
    ) {
      
      let nameHash = Flowns.hash(node:"", lable: name)
      let prefix = "0x"

      let rootDomain  <- create RootDomain(
        id: Flowns.totalRootDomains,
        name: name,
        nameHash: prefix.concat(nameHash),
        vault: <- vault
      )

      Flowns.totalRootDomains = Flowns.totalRootDomains + 1 as UInt64
      emit RootDomainCreated(name: name, nameHash: nameHash,  id: rootDomain.id)

      let oldDomain <- self.domains[rootDomain.id] <- rootDomain
      destroy oldDomain
    }

    pub fun renewDomain(domainId: UInt64, domain: &Domains.NFT, duration: UFix64, feeTokens: @FungibleToken.Vault){
      pre {
          self.domains[domainId] != nil : "Root domain not exist..."
        }
      let root = self.getRootDomain(domainId)
      root.renewDomain(domain: domain, duration: duration, feeTokens: <- feeTokens)
    }

    pub fun registerDomain(domainId: UInt64, name: String, duration: UFix64, feeTokens: @FungibleToken.Vault, receiver: Capability<&{NonFungibleToken.Receiver}>, refer: Address?){
      pre {
        self.domains[domainId] != nil : "Root domain not exist..."
      }
      let root = self.getRootDomain(domainId)
      root.registerDomain(name: name, duration: duration, feeTokens: <-feeTokens, receiver: receiver, refer: refer )
    }

    pub fun getVaultBalance(domainId: UInt64): UFix64 {
        pre {
        self.domains[domainId] != nil : "Root domain not exist..."
      }
      let rootRef = &self.domains[domainId] as? &RootDomain

      return rootRef.getVaultBalance()
    }

    access(account) fun withdrawVault(domainId: UInt64, receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
      pre {
        self.domains[domainId] != nil : "Root domain not exist..."
      }
      self.getRootDomain(domainId).withdrawVault(receiver: receiver, amount: amount)
    }

    access(account) fun changeRootDomainVault(domainId: UInt64, vault: @FungibleToken.Vault) {
      pre {
        self.domains[domainId] != nil : "Root domain not exist..."
      }
      self.getRootDomain(domainId).changeRootDomainVault(vault: <- vault)
    }

    access(account) fun mintDomain(domainId: UInt64, name: String, duration: UFix64, receiver: Capability<&{NonFungibleToken.Receiver}>) {
        pre {
        self.domains[domainId] != nil : "Root domain not exist..."
      }
      let root = self.getRootDomain(domainId)
      root.mintDomain(name: name, duration: duration, receiver: receiver)
    }

    // Get all root domains
    pub fun getAllDomains(): {UInt64: RootDomainInfo} {
      var domainInfos: {UInt64: RootDomainInfo }= {}
      for id in self.domains.keys {
        let itemRef = &self.domains[id] as? &RootDomain
        domainInfos[id] = itemRef.getRootDomainInfo()
      }
      return domainInfos
    }
    
    access(account) fun setPrices(domainId: UInt64, len: Int, price: UFix64){
      self.getRootDomain(domainId).setPrices(key: len, price: price)
    }

    access(account) fun setMinRentDuration(domainId: UInt64, duration: UFix64){
      pre {
        duration >= 604800.00 : "Duration must be greater than one week"
      }
      self.getRootDomain(domainId).setMinRentDuration(duration)
    }

    access(account) fun setMaxDomainLength(domainId: UInt64, length: Int){
      pre {
        length > 0 && length < 50 : "Domain length must greater than 0 and smaller than 50"
      }
      self.getRootDomain(domainId).setMaxDomainLength(length)
    }

    access(account) fun setCommissionRate(domainId: UInt64, rate: UFix64){
      pre {
        rate >= 0.0 && rate <= 1.0 : "Commission rate not valid"
      }
      self.getRootDomain(domainId).setCommissionRate(rate)
    }

    // get domain reference
    access(contract) fun getRootDomain(_ domainId: UInt64) : &RootDomain {
      pre {
        self.domains[domainId] != nil: "domain doesn't exist"
      }
      return &self.domains[domainId] as &RootDomain
    }

    // get Root domain info
    pub fun getDomainInfo(domainId: UInt64): RootDomainInfo {
      return self.getRootDomain(domainId).getRootDomainInfo()
    }

    // Query root domain's rent price
    pub fun getPrices(domainId: UInt64): {Int: UFix64} {
      return self.getRootDomain(domainId).getPrices()
    }


    destroy() {            
        destroy self.domains
    }
  }

  // Admin interface resource
  pub resource interface AdminPrivate {

    pub fun addCapability(_ cap: Capability<&Flowns.RootDomainCollection>)

    pub fun addRootDomainCapability(domainId: UInt64, cap: Capability<&Domains.Collection>)

    pub fun createRootDomain(name: String, vault: @FungibleToken.Vault)

    pub fun setRentPrice(domainId: UInt64, len: Int, price: UFix64)

    pub fun withdrawVault(domainId: UInt64, receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64)

    pub fun changeRootDomainVault(domainId: UInt64, vault: @FungibleToken.Vault)
    
    pub fun mintDomain(domainId: UInt64, name: String, duration: UFix64, receiver: Capability<&{NonFungibleToken.Receiver}>)

    pub fun setMinRentDuration(domainId: UInt64, duration: UFix64)

    pub fun setMaxDomainLength(domainId: UInt64, length: Int)

    pub fun setCommissionRate(domainId: UInt64, rate: UFix64)

    pub fun setDomainForbidChars(_ chars: String)

    pub fun setPause(_ flag: Bool)

  }


  pub resource Admin: AdminPrivate {

    access(self) var server: Capability<&Flowns.RootDomainCollection>?

    init() {
      // Server is the root collection for manager to create and store root domain
      self.server = nil
    }

    // init RootDomainCollection for admin
    pub fun addCapability(_ cap: Capability<&Flowns.RootDomainCollection>) {
      pre {
        cap.check() : "Invalid server capablity"
        self.server == nil : "Server already set"
      }
        self.server = cap
    }

    // init Root domain's Domains collection to create collection for domain register 
    pub fun addRootDomainCapability(domainId: UInt64, cap: Capability<&Domains.Collection>) {
      pre {
          cap.check() : "Invalid server capablity"
      }
      self.server!.borrow()!.getRootDomain(domainId).addCapability(cap)
    }

    // Create root domain with admin
    pub fun createRootDomain(name: String, vault: @FungibleToken.Vault) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.createRootDomain(name: name, vault: <- vault)
    }

    // Set rent price
    pub fun setRentPrice(domainId: UInt64, len: Int, price: UFix64) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.setPrices(domainId: domainId, len: len, price: price)
    }

    pub fun setMinRentDuration(domainId: UInt64, duration: UFix64) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.setMinRentDuration(domainId: domainId, duration: duration)
    }

    pub fun setMaxDomainLength(domainId: UInt64, length: Int) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.setMaxDomainLength(domainId: domainId, length: length)
    }

    
    pub fun setCommissionRate(domainId: UInt64, rate: UFix64) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.setCommissionRate(domainId: domainId, rate: rate)
    }
    
    

    // Withdraw vault 
    pub fun withdrawVault(domainId: UInt64, receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.withdrawVault(domainId: domainId, receiver: receiver, amount: amount)
    }

    // Withdraw vault 
    pub fun changeRootDomainVault(domainId: UInt64, vault: @FungibleToken.Vault) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.changeRootDomainVault(domainId: domainId, vault: <- vault)
    }

    // Mint domain with root domain
    pub fun mintDomain(domainId: UInt64, name: String, duration: UFix64,receiver: Capability<&{NonFungibleToken.Receiver}>) {
      pre {
        self.server != nil : "Your client has not been linked to the server"
      }

      self.server!.borrow()!.mintDomain(domainId: domainId, name: name, duration: duration, receiver: receiver)
    }

    pub fun setPause(_ flag: Bool) {
      pre {
        Flowns.isPause != flag : "Already done!"
      }
      Flowns.isPause = flag
      if flag == true {
        emit FlownsPaused()
      } else {
        emit FlownsActivated()
      }
    }

    pub fun setDomainForbidChars(_ chars: String) {
      let oldChars = Flowns.forbidChars
      Flowns.forbidChars = chars
      
      emit FlownsForbidCharsUpdated(before: oldChars, after: chars)
    }


  }

  // Create admin resource
  priv fun createAdminClient(): @Admin {
    emit FlownsAdminCreated()
    return <- create Admin()
  }

  pub fun getDomainNameHash(name: String, parentNameHash: String): String {
    
    let prefix = "0x"
    let forbidenChars: [UInt8] = Flowns.forbidChars.utf8
    let nameASCII = name.utf8

    for char in forbidenChars {
      if nameASCII.contains(char) {
        panic("Domain name illegal ...")
      }
    }

    let domainNameHash = Flowns.hash(node: parentNameHash.slice(from: 2, upTo: 66), lable:name )
    return prefix.concat(domainNameHash)
  }

  // calc hash with node and lable
  pub fun hash(node: String, lable: String): String {
    var prefixNode = node
    if node.length == 0 {
      prefixNode = "0000000000000000000000000000000000000000000000000000000000000000"
    }
    let lableHash = String.encodeHex(HashAlgorithm.SHA3_256.hash(lable.utf8))
    let hash = String.encodeHex(HashAlgorithm.SHA3_256.hash(prefixNode.concat(lableHash).utf8))
    return hash
  }


  // Query root domain
  pub fun getRootDomainInfo(domainId: UInt64): RootDomainInfo? {
    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    if let collection = rootCollectionCap.borrow()  {
      return collection.getDomainInfo(domainId: domainId)
    }
    return nil
  }
  // Query all root domain
  pub fun getAllRootDomains(): {UInt64: RootDomainInfo}? {

    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    if let collection = rootCollectionCap.borrow()  {
        return collection.getAllDomains()
    }
    return nil
  }
  
  // Check domain available 
  pub fun available(nameHash: String): Bool {

    if Domains.getRecords(nameHash) == nil {
      return true
    }
    return Domains.isExpired(nameHash)
  }

  pub fun getRentPrices(domainId: UInt64): {Int: UFix64} {

    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    if let collection = rootCollectionCap.borrow()  {
      return collection.getPrices(domainId: domainId)
    }
    return {}
  }

  pub fun getRootVaultBalance(domainId: UInt64): UFix64 {

    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    let collection = rootCollectionCap.borrow()?? panic("Could not borrow collection ")
    let balance = collection.getVaultBalance(domainId: domainId)
    return balance
  }

  pub fun registerDomain(domainId: UInt64, name: String, duration: UFix64, feeTokens: @FungibleToken.Vault, receiver: Capability<&{NonFungibleToken.Receiver}>, refer: Address? ){
    pre {
      Flowns.isPause == false : "Register pause"
    }
    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    let collection = rootCollectionCap.borrow() ?? panic("Could not borrow collection ")
    collection.registerDomain(domainId: domainId, name: name, duration: duration, feeTokens: <-feeTokens, receiver: receiver, refer: refer)
  }
  
  pub fun renewDomain(domainId: UInt64, domain: &Domains.NFT, duration: UFix64, feeTokens: @FungibleToken.Vault) {
    pre {
      Flowns.isPause == false : "Renewer pause"
    }
    let account = Flowns.account
    let rootCollectionCap = account.getCapability<&{Flowns.RootDomainCollectionPublic}>(self.CollectionPublicPath)
    let collection = rootCollectionCap.borrow() ?? panic("Could not borrow collection ")
    collection.renewDomain(domainId: domainId, domain: domain, duration: duration, feeTokens: <-feeTokens)
  }
  
  init() {

    self.CollectionPublicPath = /public/flownsCollection
    self.CollectionPrivatePath = /private/flownsCollection
    self.CollectionStoragePath = /storage/flownsCollection
    self.FlownsAdminPrivatePath = /private/flownsAdmin
    self.FlownsAdminStoragePath =/storage/flownsAdmin

    let account = self.account
    let admin <- Flowns.createAdminClient()

    account.save<@Flowns.Admin>(<-admin, to: Flowns.FlownsAdminStoragePath)
    self.totalRootDomains = 0
    self.isPause = true
    self.forbidChars = "!@#$%^&*()<>? ./"

    let collection <- create RootDomainCollection()
    account.save(<-collection, to: Flowns.CollectionStoragePath)
    account.link<&{Flowns.RootDomainCollectionPublic}>(Flowns.CollectionPublicPath, target: Flowns.CollectionStoragePath)
    account.link<&Flowns.RootDomainCollection>(Flowns.CollectionPrivatePath, target: Flowns.CollectionStoragePath)
    account.link<&Flowns.Admin{Flowns.AdminPrivate}>(Flowns.FlownsAdminPrivatePath, target: Flowns.FlownsAdminStoragePath)
  }
}
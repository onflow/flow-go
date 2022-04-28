
// Simplified version from https://flow-view-source.com/testnet/account/0xba1132bc08f82fe2/contract/Profile
// It allows to somone to update a stauts on the blockchain

pub contract NameTag {
  pub let publicPath: PublicPath
  pub let storagePath: StoragePath

  pub resource interface Public {
    pub fun readTag(): String
  }
  
  pub resource interface Owner {
    pub fun readTag(): String
    
    pub fun changeTag(_ tag: String) {
      pre {
        tag.length <= 15:
          "Tags must be under 15 characters long."
      }
    }
  }
  
  pub resource Base: Owner, Public {
    access(self) var tag: String
    
    init() {
      self.tag = ""
    }
    
    pub fun readTag(): String { return self.tag }
    pub fun changeTag(_ tag: String) { self.tag = tag }
  }
  
  pub fun new(): @NameTag.Base {
    return <- create Base()
  }
  
  pub fun hasTag(_ address: Address): Bool {
    return getAccount(address)
      .getCapability<&{NameTag.Public}>(NameTag.publicPath)
      .check()
  }
  
  pub fun fetch(_ address: Address): &{NameTag.Public} {
    return getAccount(address)
      .getCapability<&{NameTag.Public}>(NameTag.publicPath)
      .borrow()!
  }
    
  init() {
    self.publicPath = /public/boulangeriev1PublicNameTag
    self.storagePath = /storage/boulangeriev1StorageNameTag
    
    self.account.save(<- self.new(), to: self.storagePath)
    self.account.link<&Base{Public}>(self.publicPath, target: self.storagePath)
  }
}
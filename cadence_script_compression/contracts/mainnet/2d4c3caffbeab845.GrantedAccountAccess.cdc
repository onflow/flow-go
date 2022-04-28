// MADE BY: Emerald City, Jacob Tucker

// This is a very simple contract that lets users add addresses
// to an "Info" resource signifying they want them to share their account.  

// This is specifically used by the
// `pub fun borrowSharedRef(fromHost: Address): &FLOATEvents`
// function inside FLOAT.cdc to give users access to someone elses
// FLOATEvents if they are on this shared list.

// This contract is my way of saying I hate private capabilities, so I
// implemented an alternative solution to private access.

pub contract GrantedAccountAccess {

  pub let InfoStoragePath: StoragePath
  pub let InfoPublicPath: PublicPath

  pub resource interface InfoPublic {
    pub fun getAllowed(): [Address]
    pub fun isAllowed(account: Address): Bool
  }

  // A list of people you allow to share your
  // account.
  pub resource Info: InfoPublic {
    access(account) var allowed: {Address: Bool}

    // Allow someone to share your account
    pub fun addAccount(account: Address) {
      self.allowed[account] = true
    }

    pub fun removeAccount(account: Address) {
      self.allowed.remove(key: account)
    }

    pub fun getAllowed(): [Address] {
      return self.allowed.keys
    }

    pub fun isAllowed(account: Address): Bool {
      return self.allowed.containsKey(account)
    }

    init() {
      self.allowed = {}
    }
  }

  pub fun createInfo(): @Info {
    return <- create Info()
  }

  init() {
    self.InfoStoragePath = /storage/GrantedAccountAccessInfo
    self.InfoPublicPath = /public/GrantedAccountAccessInfo
  }

}
import FungibleToken from 0xf233dcee88fe0abe

pub contract TeleportedSportiumToken: FungibleToken {
  // Frozen flag controlled by Admin
  pub var isFrozen: Bool

  // Total supply of TeleportedSportiumTokens in existence
  pub var totalSupply: UFix64

  // Record teleported Ethereum hashes
  pub var teleported: {String: Bool}

  // Defines token vault storage path
  pub let TokenStoragePath: StoragePath

  // Defines token vault public balance path
  pub let TokenPublicBalancePath: PublicPath

  // Defines token vault public receiver path
  pub let TokenPublicReceiverPath: PublicPath

  // Event that is emitted when the contract is created
  pub event TokensInitialized(initialSupply: UFix64)

  // Event that is emitted when tokens are withdrawn from a Vault
  pub event TokensWithdrawn(amount: UFix64, from: Address?)

  // Event that is emitted when tokens are deposited to a Vault
  pub event TokensDeposited(amount: UFix64, to: Address?)

  // Event that is emitted when new tokens are teleported in from Ethereum (from: Ethereum Address, 20 bytes)
  pub event TokensTeleportedIn(amount: UFix64, from: [UInt8], hash: String)

  // Event that is emitted when tokens are destroyed and teleported to Ethereum (to: Ethereum Address, 20 bytes)
  pub event TokensTeleportedOut(amount: UFix64, to: [UInt8])

  // Event that is emitted when teleport fee is collected (type 0: out, 1: in)
  pub event FeeCollected(amount: UFix64, type: UInt8)

  // Event that is emitted when a new burner resource is created
  pub event TeleportAdminCreated(allowedAmount: UFix64)

  // Vault
  //
  // Each user stores an instance of only the Vault in their storage
  // The functions in the Vault and governed by the pre and post conditions
  // in FungibleToken when they are called.
  // The checks happen at runtime whenever a function is called.
  //
  // Resources can only be created in the context of the contract that they
  // are defined in, so there is no way for a malicious user to create Vaults
  // out of thin air. A special Minter resource needs to be defined to mint
  // new tokens.
  //
  pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {
    
    // holds the balance of a users tokens
    pub var balance: UFix64

    // initialize the balance at resource creation time
    init(balance: UFix64) {
      self.balance = balance
    }

    // withdraw
    //
    // Function that takes an integer amount as an argument
    // and withdraws that amount from the Vault.
    // It creates a new temporary Vault that is used to hold
    // the money that is being transferred. It returns the newly
    // created Vault to the context that called so it can be deposited
    // elsewhere.
    //
    pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
      self.balance = self.balance - amount
      emit TokensWithdrawn(amount: amount, from: self.owner?.address)
      return <- create Vault(balance: amount)
    }

    // deposit
    //
    // Function that takes a Vault object as an argument and adds
    // its balance to the balance of the owners Vault.
    // It is allowed to destroy the sent Vault because the Vault
    // was a temporary holder of the tokens. The Vault's balance has
    // been consumed and therefore can be destroyed.
    pub fun deposit(from: @FungibleToken.Vault) {
      let vault <- from as! @TeleportedSportiumToken.Vault
      self.balance = self.balance + vault.balance
      emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
      vault.balance = 0.0
      destroy vault
    }

    destroy() {
      TeleportedSportiumToken.totalSupply = TeleportedSportiumToken.totalSupply - self.balance
    }
  }

  // createEmptyVault
  //
  // Function that creates a new Vault with a balance of zero
  // and returns it to the calling context. A user must call this function
  // and store the returned Vault in their storage in order to allow their
  // account to be able to receive deposits of this token type.
  //
  pub fun createEmptyVault(): @FungibleToken.Vault {
    return <- create Vault(balance: 0.0)
  }

  pub resource Allowance {
    pub var balance: UFix64

    // initialize the balance at resource creation time
    init(balance: UFix64) {
      self.balance = balance
    }
  }

  pub resource Administrator {

    // createNewTeleportAdmin
    //
    // Function that creates and returns a new teleport admin resource
    //
    pub fun createNewTeleportAdmin(allowedAmount: UFix64): @TeleportAdmin {
      emit TeleportAdminCreated(allowedAmount: allowedAmount)
      return <- create TeleportAdmin(allowedAmount: allowedAmount)
    }

    pub fun freeze() {
      TeleportedSportiumToken.isFrozen = true
    }

    pub fun unfreeze() {
      TeleportedSportiumToken.isFrozen = false
    }

    pub fun createAllowance(allowedAmount: UFix64): @Allowance {
      return <- create Allowance(balance: allowedAmount)
    }
  }

  pub resource interface TeleportUser {
    // fee collected when token is teleported from Ethereum to Flow
    pub var inwardFee: UFix64

    // fee collected when token is teleported from Flow to Ethereum
    pub var outwardFee: UFix64
    
    // the amount of tokens that the minter is allowed to mint
    pub var allowedAmount: UFix64

    pub fun teleportOut(from: @FungibleToken.Vault, to: [UInt8])

    pub fun depositAllowance(from: @Allowance)

    pub fun getEthereumAdminAccount(): [UInt8]
  }

  pub resource interface TeleportControl {
    pub fun teleportIn(amount: UFix64, from: [UInt8], hash: String): @TeleportedSportiumToken.Vault

    pub fun withdrawFee(amount: UFix64): @FungibleToken.Vault
    
    pub fun updateInwardFee(fee: UFix64)

    pub fun updateOutwardFee(fee: UFix64)

    pub fun updateEthereumAdminAccount(account: [UInt8])
  }

  // TeleportAdmin resource
  //
  //  Resource object that has the capability to mint teleported tokens
  //  upon receiving teleport request from Ethereum side
  //
  pub resource TeleportAdmin: TeleportUser, TeleportControl {
    
    // the amount of tokens that the minter is allowed to mint
    pub var allowedAmount: UFix64

    // receiver reference to collect teleport fee
    pub let feeCollector: @TeleportedSportiumToken.Vault

    // fee collected when token is teleported from Ethereum to Flow
    pub var inwardFee: UFix64

    // fee collected when token is teleported from Flow to Ethereum
    pub var outwardFee: UFix64

    // corresponding controller account on Ethereum
    access(self) var ethereumAdminAccount: [UInt8]

    // teleportIn
    //
    // Function that mints new tokens, adds them to the total supply,
    // and returns them to the calling context.
    //
    pub fun teleportIn(amount: UFix64, from: [UInt8], hash: String): @TeleportedSportiumToken.Vault {
      pre {
        !TeleportedSportiumToken.isFrozen: "Teleport service is frozen"
        amount <= self.allowedAmount: "Amount minted must be less than the allowed amount"
        amount > self.inwardFee: "Amount minted must be greater than inward teleport fee"
        from.length == 20: "Ethereum address should be 20 bytes"
        hash.length == 64: "Ethereum tx hash should be 32 bytes"
        !(TeleportedSportiumToken.teleported[hash] ?? false): "Same hash already teleported"
      }
      TeleportedSportiumToken.totalSupply = TeleportedSportiumToken.totalSupply + amount
      self.allowedAmount = self.allowedAmount - amount

      TeleportedSportiumToken.teleported[hash] = true
      emit TokensTeleportedIn(amount: amount, from: from, hash: hash)

      let vault <- create Vault(balance: amount)
      let fee <- vault.withdraw(amount: self.inwardFee)

      self.feeCollector.deposit(from: <-fee)
      emit FeeCollected(amount: self.inwardFee, type: 1)

      return <- vault
    }

    // teleportOut
    //
    // Function that destroys a Vault instance, effectively burning the tokens.
    //
    // Note: the burned tokens are automatically subtracted from the 
    // total supply in the Vault destructor.
    //
    pub fun teleportOut(from: @FungibleToken.Vault, to: [UInt8]) {
      pre {
        !TeleportedSportiumToken.isFrozen: "Teleport service is frozen"
        to.length == 20: "Ethereum address should be 20 bytes"
      }

      let vault <- from as! @TeleportedSportiumToken.Vault
      let fee <- vault.withdraw(amount: self.outwardFee)

      self.feeCollector.deposit(from: <-fee)
      emit FeeCollected(amount: self.outwardFee, type: 0)

      let amount = vault.balance
      destroy vault
      emit TokensTeleportedOut(amount: amount, to: to)
    }

    pub fun withdrawFee(amount: UFix64): @FungibleToken.Vault {
      return <- self.feeCollector.withdraw(amount: amount)
    }

    pub fun updateInwardFee(fee: UFix64) {
      self.inwardFee = fee
    }

    pub fun updateOutwardFee(fee: UFix64) {
      self.outwardFee = fee
    }

    pub fun updateEthereumAdminAccount(account: [UInt8]) {
      pre {
        account.length == 20: "Ethereum address should be 20 bytes"
      }

      self.ethereumAdminAccount = account
    }

    pub fun getFeeAmount(): UFix64 {
      return self.feeCollector.balance
    }

    pub fun depositAllowance(from: @Allowance) {
      self.allowedAmount = self.allowedAmount + from.balance

      destroy from
    }

    pub fun getEthereumAdminAccount(): [UInt8] {
      return self.ethereumAdminAccount
    }

    init(allowedAmount: UFix64) {
      self.allowedAmount = allowedAmount

      self.feeCollector <- TeleportedSportiumToken.createEmptyVault() as! @TeleportedSportiumToken.Vault
      self.inwardFee = 0.01
      self.outwardFee = 3.0

      self.ethereumAdminAccount = []
    }

    destroy() {
      destroy self.feeCollector
    }
  }

  init() {
    self.isFrozen = false
    self.totalSupply = 0.0
    self.teleported = {}
    self.TokenStoragePath = /storage/TeleportedSportiumTokenVault
    self.TokenPublicBalancePath = /public/TeleportedSportiumTokenBalance
    self.TokenPublicReceiverPath = /public/TeleportedSportiumTokenReceiver

    let admin <- create Administrator()
    self.account.save(<-admin, to: /storage/TeleportedSportiumTokenAdmin)

    // Emit an event that shows that the contract was initialized
    emit TokensInitialized(initialSupply: self.totalSupply)
  }
}

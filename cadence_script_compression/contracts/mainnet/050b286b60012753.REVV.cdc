import FungibleToken from 0xf233dcee88fe0abe

pub contract REVV: FungibleToken {

  // Max REVV supply
  pub let MAX_SUPPLY:UFix64

  // Total supply of REVV tokens in existence
  pub var totalSupply: UFix64

  // Event that is emitted when the contract is created
  pub event TokensInitialized(initialSupply: UFix64)

  // Event that is emitted when tokens are withdrawn from a Vault
  pub event TokensWithdrawn(amount: UFix64, from: Address?)

  // Event that is emitted when tokens are deposited to a Vault
  pub event TokensDeposited(amount: UFix64, to: Address?)

  // Event that is emitted when new tokens are minted
  pub event TokensMinted(amount: UFix64)

  // The storage path for the Admin token
  pub let RevvAdminStoragePath: StoragePath

  // The public path for the token balance
  pub let RevvBalancePublicPath: PublicPath

  // The public path for the token receiver
  pub let RevvReceiverPublicPath: PublicPath

  // The storage path for the token vault
  pub let RevvVaultStoragePath: StoragePath

  // The private path for the token vault
  pub let RevvVaultPrivatePath: PrivatePath


  // The escrow vault for REVV from REVV vaults that were destroyed
  access(contract) let escrowVault: @REVV.Vault

  // Admin resource
  //
  pub resource Admin {}

  // Vault
  //
  // Each user stores an instance of only the Vault in their storage
  // The functions in the Vault are governed by the pre and post conditions
  // in FungibleToken when they are called.
  // The checks happen at runtime whenever a function is called.
  //
  // Resources can only be created in the context of the contract that they
  // are defined in, so there is no way for a malicious user to create Vaults
  // out of thin air.
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
    // Function that takes an amount as an argument
    // and withdraws that amount from the Vault.
    // It creates a new temporary Vault that is used to hold
    // the money that is being transferred. It returns the newly
    // created Vault to the context that called so it can be deposited
    // elsewhere.
    //
    pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
      self.balance = self.balance - amount
      emit TokensWithdrawn(amount: amount, from: self.owner?.address)
      return <-create Vault(balance: amount)
    }

    // deposit
    //
    // Function that takes a Vault object as an argument and adds
    // its balance to the balance of the owners Vault.
    // It is allowed to destroy the sent Vault because the Vault
    // was a temporary holder of the tokens. The Vault's balance has
    // been consumed and therefore the vault can be destroyed.
    //
    pub fun deposit(from: @FungibleToken.Vault) {
      let vault <- from as! @REVV.Vault
      self.balance = self.balance + vault.balance
      emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
      vault.balance = 0.0
      destroy vault
    }

    // destroy
    //
    // Burning in the sense of reducing total supply is prevented by overriding the Vault's destroy method 
    // and transferring the balance to the REVV contract's escrow vault
    //
    destroy() {
      if self.balance > 0.0 {
        REVV.depositToEscrow(from: <- create Vault(balance: self.balance))
      }
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
    return <-create Vault(balance: 0.0)
  }

  // depositToEscrow
  //
  // Function accessible from contract only, which deposits REVV into the escrow vault
  //
  access(contract) fun depositToEscrow(from: @FungibleToken.Vault) {
    let vault <- from as! @REVV.Vault
    self.escrowVault.deposit(from: <- vault)
  }

  // withdraw from escrowVault
  // Public method which requires as argument an admin object reference, which only the account owner has access to
  //
  pub fun withdrawFromEscrow(adminRef: &Admin, amount: UFix64): @FungibleToken.Vault {
    pre {
      adminRef != nil: "adminRef is nil"
    }
    return <- self.escrowVault.withdraw(amount: amount)
  }

  // getEscrowVaultBalance
  //
  // returns the balance for the contract's escrow vault
  //
  pub fun getEscrowVaultBalance(): UFix64 {
    return self.escrowVault.balance
  }

  // mint 
  // 
  // Can only be called by contract.
  // Total minted amount can never exceed MAX_SUPPLY
  //
  access(contract) fun mint(amount: UFix64) {
    pre {
      amount > 0.0 : "Mint amount must be larger than 0.0"
      self.totalSupply + amount <= self.MAX_SUPPLY : "totalSupply + mint amount can't exceed max supply"
    }
    let revvVaultRef = self.account.borrow<&FungibleToken.Vault>(from: self.RevvVaultStoragePath)!
    let mintVault <- create REVV.Vault(balance: amount) as! @FungibleToken.Vault
    revvVaultRef.deposit(from: <- mintVault)
    self.totalSupply = self.totalSupply + amount

    emit TokensMinted(amount: amount)
  }

  init() {
    // Init supply fields
    //
    self.totalSupply = UFix64(0)
    self.MAX_SUPPLY = UFix64(3_000_000_000)

    //Initialize the path fields
    //
    self.RevvAdminStoragePath = /storage/revvAdmin

    self.RevvBalancePublicPath = /public/revvBalance

    self.RevvReceiverPublicPath = /public/revvReceiver

    self.RevvVaultStoragePath = /storage/revvVault

    self.RevvVaultPrivatePath = /private/revvVault

    // create and store Admin resource 
    // this resource is currently not used by the contract, added in case
    // needed in future
    //
    self.account.save(<- create Admin(), to: self.RevvAdminStoragePath)

    // create an escrow vault
    //
    self.escrowVault <- self.createEmptyVault() as! @REVV.Vault;

    // Create an REVV vault and save it in storage
    //
    let vault <- self.createEmptyVault()
    self.account.save(<-vault, to: self.RevvVaultStoragePath)

    // Create a public capability to the stored Vault that only exposes
    // the `deposit` method through the `Receiver` interface
    //
    self.account.link<&REVV.Vault{FungibleToken.Receiver}>(
        self.RevvReceiverPublicPath,
        target: self.RevvVaultStoragePath
    )

    // Create a public capability to the stored Vault that only exposes
    // the `balance` field through the `Balance` interface
    //
    self.account.link<&REVV.Vault{FungibleToken.Balance}>(
        self.RevvBalancePublicPath,
        target: self.RevvVaultStoragePath
    )

    self.account.link<&REVV.Vault{FungibleToken.Provider}>(
      self.RevvVaultPrivatePath,
      target: self.RevvVaultStoragePath
    )

    // Mint total supply
    //
    self.mint(amount: self.MAX_SUPPLY)

    // Emit an event that shows that the contract was initialized
    //
    emit TokensInitialized(initialSupply: self.totalSupply)
  }
}
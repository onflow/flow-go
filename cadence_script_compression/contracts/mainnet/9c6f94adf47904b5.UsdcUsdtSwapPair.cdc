import FungibleToken from 0xf233dcee88fe0abe
import FiatToken from 0xb19436aae4d94622
import TeleportedTetherToken from 0xcfdd90d4a00f7b5b

// Exchange pair between FiatToken and TeleportedTetherToken
// Token1: FiatToken (USDC)
// Token2: TeleportedTetherToken (USDT)
pub contract UsdcUsdtSwapPair: FungibleToken {
  // Frozen flag controlled by Admin
  pub var isFrozen: Bool
  
  // Total supply of UsdcUsdtSwapPair liquidity token in existence
  pub var totalSupply: UFix64

  // Controls FiatToken vault
  access(contract) let token1Vault: @FiatToken.Vault

  // Controls TeleportedTetherToken vault
  access(contract) let token2Vault: @TeleportedTetherToken.Vault

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

  // Event that is emitted when new tokens are minted
  pub event TokensMinted(amount: UFix64)

  // Event that is emitted when tokens are destroyed
  pub event TokensBurned(amount: UFix64)

  // Event that is emitted when a swap happens
  // Side 1: from token1 to token2
  // Side 2: from token2 to token1
  pub event Trade(token1Amount: UFix64, token2Amount: UFix64, side: UInt8)

  // Vault
  //
  // Each user stores an instance of only the Vault in their storage
  // The functions in the Vault and governed by the pre and post conditions
  // in UsdcUsdtSwapPair when they are called.
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
      return <-create Vault(balance: amount)
    }

    // deposit
    //
    // Function that takes a Vault object as an argument and adds
    // its balance to the balance of the owners Vault.
    // It is allowed to destroy the sent Vault because the Vault
    // was a temporary holder of the tokens. The Vault's balance has
    // been consumed and therefore can be destroyed.
    pub fun deposit(from: @FungibleToken.Vault) {
      let vault <- from as! @UsdcUsdtSwapPair.Vault
      self.balance = self.balance + vault.balance
      emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
      vault.balance = 0.0
      destroy vault
    }

    destroy() {
      UsdcUsdtSwapPair.totalSupply = UsdcUsdtSwapPair.totalSupply - self.balance
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

  pub resource TokenBundle {
    pub var token1: @FiatToken.Vault
    pub var token2: @TeleportedTetherToken.Vault

    // initialize the vault bundle
    init(fromToken1: @FiatToken.Vault, fromToken2: @TeleportedTetherToken.Vault) {
      self.token1 <- fromToken1
      self.token2 <- fromToken2
    }

    pub fun depositToken1(from: @FiatToken.Vault) {
      self.token1.deposit(from: <- from)
    }

    pub fun depositToken2(from: @TeleportedTetherToken.Vault) {
      self.token2.deposit(from: <- from)
    }

    pub fun withdrawToken1(): @FiatToken.Vault {
      var vault <- FiatToken.createEmptyVault() as! @FiatToken.Vault
      vault <-> self.token1
      return <- vault
    }

    pub fun withdrawToken2(): @TeleportedTetherToken.Vault {
      var vault <- TeleportedTetherToken.createEmptyVault() as! @TeleportedTetherToken.Vault
      vault <-> self.token2
      return <- vault
    }

    destroy() {
      destroy self.token1
      destroy self.token2
    }
  }

  // createEmptyBundle
  //
  pub fun createEmptyTokenBundle(): @UsdcUsdtSwapPair.TokenBundle {
    return <- create TokenBundle(
      fromToken1: <- (FiatToken.createEmptyVault() as! @FiatToken.Vault),
      fromToken2: <- (TeleportedTetherToken.createEmptyVault() as! @TeleportedTetherToken.Vault)
    )
  }

  // createTokenBundle
  //
  pub fun createTokenBundle(fromToken1: @FiatToken.Vault, fromToken2: @TeleportedTetherToken.Vault): @UsdcUsdtSwapPair.TokenBundle {
    return <- create TokenBundle(fromToken1: <- fromToken1, fromToken2: <- fromToken2)
  }

  // mintTokens
  //
  // Function that mints new tokens, adds them to the total supply,
  // and returns them to the calling context.
  //
  access(contract) fun mintTokens(amount: UFix64): @UsdcUsdtSwapPair.Vault {
    pre {
      amount > 0.0: "Amount minted must be greater than zero"
    }
    UsdcUsdtSwapPair.totalSupply = UsdcUsdtSwapPair.totalSupply + amount
    emit TokensMinted(amount: amount)
    return <-create Vault(balance: amount)
  }

  // burnTokens
  //
  // Function that destroys a Vault instance, effectively burning the tokens.
  //
  // Note: the burned tokens are automatically subtracted from the 
  // total supply in the Vault destructor.
  //
  access(contract) fun burnTokens(from: @UsdcUsdtSwapPair.Vault) {
    let vault <- from as! @UsdcUsdtSwapPair.Vault
    let amount = vault.balance
    destroy vault
    emit TokensBurned(amount: amount)
  }

  pub resource Admin {
    pub fun freeze() {
      UsdcUsdtSwapPair.isFrozen = true
    }

    pub fun unfreeze() {
      UsdcUsdtSwapPair.isFrozen = false
    }
  }

  pub struct PoolAmounts {
    pub let token1Amount: UFix64
    pub let token2Amount: UFix64

    init(token1Amount: UFix64, token2Amount: UFix64) {
      self.token1Amount = token1Amount
      self.token2Amount = token2Amount
    }
  }

  pub fun getFeePercentage(): UFix64 {
    return 0.0
  }

  // Check current pool amounts
  pub fun getPoolAmounts(): PoolAmounts {
    return PoolAmounts(token1Amount: UsdcUsdtSwapPair.token1Vault.balance, token2Amount: UsdcUsdtSwapPair.token2Vault.balance)
  }

  // Get quote for Token1 (given) -> Token2
  pub fun quoteSwapExactToken1ForToken2(amount: UFix64): UFix64 {
    pre {
      self.token2Vault.balance >= amount: "Not enough Token2 in the pool"
    }

    // Fixed price 1:1
    return amount
  }

  // Get quote for Token1 -> Token2 (given)
  pub fun quoteSwapToken1ForExactToken2(amount: UFix64): UFix64 {
    pre {
      self.token2Vault.balance >= amount: "Not enough Token2 in the pool"
    }

    // Fixed price 1:1
    return amount
  }

  // Get quote for Token2 (given) -> Token1
  pub fun quoteSwapExactToken2ForToken1(amount: UFix64): UFix64 {
    pre {
      self.token1Vault.balance >= amount: "Not enough Token1 in the pool"
    }

    // Fixed price 1:1
    return amount
  }

  // Get quote for Token2 -> Token1 (given)
  pub fun quoteSwapToken2ForExactToken1(amount: UFix64): UFix64 {
    pre {
      self.token1Vault.balance >= amount: "Not enough Token1 in the pool"
    }

    // Fixed price 1:1
    return amount
  }

  // Swaps Token1 (FiatToken) -> Token2 (tUSDT)
  pub fun swapToken1ForToken2(from: @FiatToken.Vault): @TeleportedTetherToken.Vault {
    pre {
      !UsdcUsdtSwapPair.isFrozen: "UsdcUsdtSwapPair is frozen"
      from.balance > 0.0: "Empty token vault"
    }

    // Calculate amount from pricing curve
    // A fee portion is taken from the final amount
    let token1Amount = from.balance
    let token2Amount = self.quoteSwapExactToken1ForToken2(amount: token1Amount)

    assert(token2Amount > 0.0, message: "Exchanged amount too small")

    self.token1Vault.deposit(from: <- (from as! @FungibleToken.Vault))
    emit Trade(token1Amount: token1Amount, token2Amount: token2Amount, side: 1)

    return <- (self.token2Vault.withdraw(amount: token2Amount) as! @TeleportedTetherToken.Vault)
  }

  // Swap Token2 (tUSDT) -> Token1 (FiatToken)
  pub fun swapToken2ForToken1(from: @TeleportedTetherToken.Vault): @FiatToken.Vault {
    pre {
      !UsdcUsdtSwapPair.isFrozen: "UsdcUsdtSwapPair is frozen"
      from.balance > 0.0: "Empty token vault"
    }

    // Calculate amount from pricing curve
    // A fee portion is taken from the final amount
    let token2Amount = from.balance
    let token1Amount = self.quoteSwapExactToken2ForToken1(amount: token2Amount)

    assert(token1Amount > 0.0, message: "Exchanged amount too small")

    self.token2Vault.deposit(from: <- (from as! @FungibleToken.Vault))
    emit Trade(token1Amount: token1Amount, token2Amount: token2Amount, side: 2)

    return <- (self.token1Vault.withdraw(amount: token1Amount) as! @FiatToken.Vault)
  }

  // Used to add liquidity without minting new liquidity token
  pub fun donateLiquidity(from: @UsdcUsdtSwapPair.TokenBundle) {
    let token1Vault <- from.withdrawToken1()
    let token2Vault <- from.withdrawToken2()

    UsdcUsdtSwapPair.token1Vault.deposit(from: <- token1Vault)
    UsdcUsdtSwapPair.token2Vault.deposit(from: <- token2Vault)

    destroy from
  }

  pub fun addLiquidity(from: @UsdcUsdtSwapPair.TokenBundle): @UsdcUsdtSwapPair.Vault {
    let token1Vault <- from.withdrawToken1()
    let token2Vault <- from.withdrawToken2()

    let totalLiquidityAmount = token1Vault.balance + token2Vault.balance

    assert(totalLiquidityAmount > 0.0, message: "Empty liquidity vaults")

    self.token1Vault.deposit(from: <- token1Vault)
    self.token2Vault.deposit(from: <- token2Vault)

    destroy from

    return <- UsdcUsdtSwapPair.mintTokens(amount: totalLiquidityAmount)
  }

  pub fun removeLiquidity(from: @UsdcUsdtSwapPair.Vault, token1Amount: UFix64, token2Amount: UFix64): @UsdcUsdtSwapPair.TokenBundle {
    pre {
      from.balance > 0.0: "Empty liquidity token vault"
      from.balance < UsdcUsdtSwapPair.totalSupply: "Cannot remove all liquidity"
      from.balance == token1Amount + token2Amount: "Incorrect withdrawal amounts"
    }

    // Burn liquidity tokens and withdraw
    UsdcUsdtSwapPair.burnTokens(from: <- from)

    let token1Vault <- UsdcUsdtSwapPair.token1Vault.withdraw(amount: token1Amount) as! @FiatToken.Vault
    let token2Vault <- UsdcUsdtSwapPair.token2Vault.withdraw(amount: token2Amount) as! @TeleportedTetherToken.Vault

    let tokenBundle <- UsdcUsdtSwapPair.createTokenBundle(fromToken1: <- token1Vault, fromToken2: <- token2Vault)
    return <- tokenBundle
  }

  init() {
    self.isFrozen = true // frozen until admin unfreezes
    self.totalSupply = 0.0

    self.TokenStoragePath = /storage/usdcUsdtFspLpVault
    self.TokenPublicBalancePath = /public/usdcUsdtFspLpBalance
    self.TokenPublicReceiverPath = /public/usdcUsdtFspLpReceiver

    // Setup internal FiatToken vault
    self.token1Vault <- FiatToken.createEmptyVault() as! @FiatToken.Vault

    // Setup internal TeleportedTetherToken vault
    self.token2Vault <- TeleportedTetherToken.createEmptyVault() as! @TeleportedTetherToken.Vault

    let admin <- create Admin()
    self.account.save(<-admin, to: /storage/usdcUsdtPairAdmin)

    // Emit an event that shows that the contract was initialized
    emit TokensInitialized(initialSupply: self.totalSupply)
  }
}

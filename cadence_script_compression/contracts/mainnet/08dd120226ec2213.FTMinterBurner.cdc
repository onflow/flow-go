/// Support FT minter/burner, minimal interfaces
/// do we want mintToAccount?
import FungibleToken from 0xf233dcee88fe0abe

pub contract interface FTMinterBurner {
  pub resource interface IMinter {
    // only define func for PegBridge to call, allowedAmount isn't strictly required
    pub fun mintTokens(amount: UFix64): @FungibleToken.Vault
  }
  pub resource interface IBurner {
    pub fun burnTokens(from: @FungibleToken.Vault)
  }
  /// token contract must also define same name resource and impl mintTokens/burnTokens
  pub resource Minter: IMinter {
  // we could add pre/post to mintTokens fun here
  }
  pub resource Burner: IBurner {
  // we could add pre/post to burnTokens fun here
  }
}
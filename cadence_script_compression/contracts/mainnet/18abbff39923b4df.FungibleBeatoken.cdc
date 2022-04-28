// SPDX-License-Identifier: UNLICENSED
import FungibleToken from 0xf233dcee88fe0abe

pub contract FungibleBeatoken: FungibleToken {

    // Total Supply
    pub var totalSupply: UFix64

    // Events
    pub event TokensInitialized(initialSupply: UFix64)
    pub event TokensWithdrawn(amount: UFix64, from: Address?)
    pub event TokensDeposited(amount: UFix64, to: Address?)

    // Paths
    pub let minterPath: PrivatePath
    pub let minterStoragePath: StoragePath

    pub let publicReceiverPath: PublicPath
    pub let vaultStoragePath: StoragePath

    pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {
        pub var balance: UFix64

        init(balance: UFix64) {
            self.balance = balance
        }

        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            self.balance = self.balance - amount
            emit TokensWithdrawn(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        pub fun deposit(from: @FungibleToken.Vault) {
            let vault <- from as! @FungibleBeatoken.Vault
            self.balance = self.balance + vault.balance
            emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
            destroy vault
        }
    }

    pub resource VaultMinter {
        pub fun mintTokens(amount: UFix64): @FungibleBeatoken.Vault {
            FungibleBeatoken.totalSupply = FungibleBeatoken.totalSupply + amount
            return <-create Vault(balance: amount)
        }
    }

    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balance: 0.0)
    }

    init() {
        self.totalSupply = 0.0
        
        // minter paths
        self.minterPath = /private/beatokenMainMinter
        self.minterStoragePath = /storage/beatokenMainMinter

        // Vault paths
        self.publicReceiverPath = /public/beatokenReceiver
        self.vaultStoragePath = /storage/beatokenMainVault

        let vault <- create Vault(balance: self.totalSupply)
        self.account.save(<-vault, to: self.vaultStoragePath)
        self.account.link<&Vault{FungibleToken.Receiver,FungibleToken.Balance}>(self.publicReceiverPath, target: self.vaultStoragePath)

        let minter <-create VaultMinter()
        self.account.save(<- minter, to: self.minterStoragePath)
        self.account.link<&VaultMinter>(self.minterPath, target: self.minterStoragePath)

        emit TokensInitialized(initialSupply: self.totalSupply)
    }
}
 
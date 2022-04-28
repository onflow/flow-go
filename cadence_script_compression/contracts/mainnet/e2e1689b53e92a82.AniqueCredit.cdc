//  SPDX-License-Identifier: UNLICENSED
import FungibleToken from 0xf233dcee88fe0abe

pub contract AniqueCredit: FungibleToken {

    // Total supply of all tokens in existence.
    pub var totalSupply: UFix64

    /// TokensInitialized
    ///
    /// The event that is emitted when the contract is created
    ///
    pub event TokensInitialized(initialSupply: UFix64)

    /// TokensWithdrawn
    ///
    /// The event that is emitted when tokens are withdrawn from a Vault
    ///
    pub event TokensWithdrawn(amount: UFix64, from: Address?)

    /// TokensDeposited
    ///
    /// The event that is emitted when tokens are deposited into a Vault
    ///
    pub event TokensDeposited(amount: UFix64, to: Address?)

    /// TokensMinted
    ///
    /// The event that is emitted when new tokens are minted
    pub event TokensMinted(amount: UFix64, to: Address?)

    /// TokensBurned
    ///
    /// The event that is emitted when tokens are destroyed
    pub event TokensBurned(amount: UFix64, from: Address?)

    pub let vaultStoragePath: StoragePath
    pub let vaultPublicPath: PublicPath
    pub let minterStoragePath: StoragePath
    pub let minterPrivatePath: PrivatePath
    pub let burnerStoragePath: StoragePath
    pub let burnerPrivatePath: PrivatePath
    pub let adminStoragePath: StoragePath

    pub resource interface Provider {
        pub fun withdrawByAdmin(amount: UFix64, admin: &AniqueCredit.Admin): @AniqueCredit.Vault {
            post {
                // `result` refers to the return value of the function
                result.balance == UFix64(amount):
                    "Withdrawal amount must be the same as the balance of the withdrawn Vault"
                admin != nil:
                    "admin must be set"
            }
        }
    }
    
    pub resource interface Receiver {

        /// deposit takes a Vault and deposits it into the implementing resource type
        ///
        pub fun deposit(from: @FungibleToken.Vault)
    }

    pub resource interface Balance {
        pub var balance: UFix64
    }

    pub resource Vault: Provider, Receiver, Balance, FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {

        pub var balance: UFix64

        // initialize the balance at resource creation time
        init(balance: UFix64) {
            self.balance = balance
        }

        // withdraw
        pub fun withdrawByAdmin(amount: UFix64, admin: &AniqueCredit.Admin): @AniqueCredit.Vault {
            self.balance = self.balance - amount
            emit TokensWithdrawn(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            pre {
                false: "Use withdrawByAdmin"
            }
            return <- create Vault(balance: 0.0)
        }

        // deposit
        pub fun deposit(from: @FungibleToken.Vault) {
            let vault <- from as! @AniqueCredit.Vault
            self.balance = self.balance + vault.balance
            emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            AniqueCredit.totalSupply = AniqueCredit.totalSupply - self.balance
        }
    }

    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balance: 0.0)
    }

    // VaultMinter
    //
    // Resource object that an admin can control to mint new tokens
    pub resource VaultMinter {

        // Function that mints new tokens and deposits into an account's vault
        // using their `Receiver` reference.
        // We say `&AnyResource{Receiver}` to say that the recipient can be any resource
        // as long as it implements the Receiver interface
        pub fun mintTokens(amount: UFix64, recipient: &AnyResource{Receiver}) {
            AniqueCredit.totalSupply = AniqueCredit.totalSupply + amount
            recipient.deposit(from: <-create Vault(balance: amount))
            emit TokensMinted(amount: amount, to: recipient.owner?.address)
        }
    }

    // VaultBurner
    //
    // Resource object that an admin can control to burn minted tokens
    pub resource VaultBurner {

        // Function that burns minted tokens and withdwaw from an account's vault
        // using their `Provider` reference.
        // We say `&AnyResource{Provider}` to say that the sender can be any resource
        // as long as it implements the Provider interface
        pub fun burnTokens(amount: UFix64, account: &AnyResource{Provider}, admin: &AniqueCredit.Admin) {
            let vault <- account.withdrawByAdmin(amount: amount, admin: admin)
            destroy vault
            emit TokensBurned(amount: amount, from: account.owner?.address)
        }
    }

    pub resource Admin {
        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

        pub fun createNewVaultMinter(): @VaultMinter {
            return <-create VaultMinter()
        }

        pub fun createNewVaultBurner(): @VaultBurner {
            return <-create VaultBurner()
        }
    }

    // The init function for the contract. All fields in the contract must
    // be initialized at deployment. This is just an example of what
    // an implementation could do in the init function. The numbers are arbitrary.
    init() {
        self.totalSupply = 0.0

        self.vaultStoragePath  = /storage/AniqueCreditVault
        self.vaultPublicPath   =  /public/AniqueCreditReceiver
        self.minterStoragePath = /storage/AniqueCreditMinter
        self.minterPrivatePath = /private/AniqueCreditMinter
        self.burnerStoragePath = /storage/AniqueCreditBurner
        self.burnerPrivatePath = /private/AniqueCreditBurner
        self.adminStoragePath  = /storage/AniqueCreditAdmin

        let vault <- create Vault(balance: self.totalSupply)

        self.account.save(<-vault, to: self.vaultStoragePath)
        self.account.link<&Vault{Receiver, Balance}>(self.vaultPublicPath, target: self.vaultStoragePath)

        self.account.save(<-create VaultMinter(), to: self.minterStoragePath)
        self.account.link<&VaultMinter>(self.minterPrivatePath, target: self.minterStoragePath)

        self.account.save(<-create VaultBurner(), to: self.burnerStoragePath)
        self.account.link<&VaultBurner>(self.burnerPrivatePath, target: self.burnerStoragePath)

        self.account.save<@Admin>(<- create Admin(), to: self.adminStoragePath)

        emit TokensInitialized(initialSupply: self.totalSupply)
    }
}

/**

# FlowToken example contract

This is an example implementation of the Flow Fungible Token standard.
Is not part of the standard, but just shows how most tokens
should implement the standard, including the Flow network token itself.

The FlowToken contract only needs to be deployed in one account.
The only part of the contract that would be stored in each user's account
is the Vault object, below

The implementation does not need to redefine the interfaces that are
already defined in the Fungible Token interface

*/

import FungibleToken from 0x%s

pub contract FlowToken: FungibleToken {

    // Total supply of flow tokens in existence
    pub var totalSupply: UFix64

    // Event that is emitted when the contract is created
    pub event FungibleTokenInitialized(initialSupply: UFix64)

    // Event that is emitted when tokens are withdrawn from a Vault
    pub event Withdraw(amount: UFix64, from: Address?)

    // Event that is emitted when tokens are deposited to a Vault
    pub event Deposit(amount: UFix64, to: Address?)

    // Event that is emitted when new tokens are minted
    pub event Mint(amount: UFix64)

    // Event that is emitted when tokens are destroyed
    pub event Burn(amount: UFix64)

    // Event that is emitted when a mew minter resource is created
    pub event MinterCreated(allowedAmount: UFix64)

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
        pub fun withdraw(amount: UFix64): @FlowToken.Vault {
            self.balance = self.balance - amount
            emit Withdraw(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        // deposit
        //
        // Function that takes a Vault object as an argument and adds
        // its balance to the balance of the owners Vault.
        // It is allowed to destroy the sent Vault because the Vault
        // was a temporary holder of the tokens. The Vault's balance has
        // been consumed and therefore can be destroyed.
        pub fun deposit(from: @Vault) {
            let vault <- from as! @FlowToken.Vault
            self.balance = self.balance + vault.balance
            emit Deposit(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            FlowToken.totalSupply = FlowToken.totalSupply - self.balance
        }
    }

    // createEmptyVault
    //
    // Function that creates a new Vault with a balance of zero
    // and returns it to the calling context. A user must call this function
    // and store the returned Vault in their storage in order to allow their
    // account to be able to receive deposits of this token type.
    //
    pub fun createEmptyVault(): @FlowToken.Vault {
        return <-create Vault(balance: 0.0)
    }

    // MintAndBurn
    //
    // Resource object that token admin accounts could hold
    // to mint and burn new tokens.
    //
    pub resource MintAndBurn {

        // the amount of tokens that the minter is allowed to mint
        pub var allowedAmount: UFix64

        // mintTokens
        //
        // Function that mints new tokens, adds them to the total Supply,
        // and returns them to the calling context
        //
        pub fun mintTokens(amount: UFix64): @FlowToken.Vault {
            pre {
                amount > UFix64(0): "Amount minted must be greater than zero"
                amount <= self.allowedAmount: "Amount minted must be less than the allowed amount"
            }
            FlowToken.totalSupply = FlowToken.totalSupply + amount
            self.allowedAmount = self.allowedAmount - amount
            emit Mint(amount: amount)
            return <-create Vault(balance: amount)
        }

        // burnTokens
        //
        // Function that takes a Vault as an argument, subtracts its balance
        // from the total supply, then destroys the Vault,
        // thereby removing the tokens from existence.
        //
        // Returns the amount that was burnt.
        //
        pub fun burnTokens(from: @Vault) {
            let vault <- from as! @FlowToken.Vault
            let amount = vault.balance
            destroy vault
            emit Burn(amount: amount)
        }

        // createNewMinter
        //
        // Function that creates and returns a new minter resource
        //
        pub fun createNewMinter(allowedAmount: UFix64): @MintAndBurn {
            emit MinterCreated(allowedAmount: allowedAmount)
            return <-create MintAndBurn(allowedAmount: allowedAmount)
        }

        init(allowedAmount: UFix64) {
            self.allowedAmount = allowedAmount
        }
    }

    // The initializer for the contract. All fields in the contract must
    // be initialized at deployment. This is just an example of what
    // an implementation could do in the initializer.
    //
    // The numbers are arbitrary.
    //
    init() {
        // Initialize the totalSupply field to the initial balance
        self.totalSupply = 1000.0

        // Create the Vault with the total supply of tokens and save it in storage
        //
        let vault <- create Vault(balance: self.totalSupply)
        self.account.save(<-vault, to: /storage/flowTokenVault)

        // Create a public capability to the stored Vault that only exposes
        // the `deposit` method through the `Receiver` interface
        //
        self.account.link<&{FungibleToken.Receiver}>(
            /public/flowTokenReceiver,
            target: /storage/flowTokenVault
        )

        // Create a public capability to the stored Vault that only exposes
        // the `balance` field through the `Balance` interface
        //
        self.account.link<&{FungibleToken.Balance}>(
            /public/flowTokenBalance,
            target: /storage/flowTokenVault
        )

        // Create a new MintAndBurn resource and store it in account storage
        let mintAndBurn <- create MintAndBurn(allowedAmount: 100.0)
        self.account.save(<-mintAndBurn, to: /storage/flowTokenMintAndBurn)

        // Emit an event that shows that the contract was initialized
        emit FungibleTokenInitialized(initialSupply: self.totalSupply)
    }
}

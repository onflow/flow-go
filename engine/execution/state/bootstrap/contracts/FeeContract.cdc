/**

Fee Contract

This contract provides a vault where tokens are collected as a fee, at the end
of epoch they are burned by the admin account.

*/

import FungibleToken from 0x%s
import FlowToken from 0x%s

pub contract FeeContract {

    // Event that is emitted when the contract has initialized
    pub event FeeContractInitialized()

    // Event that is emitted when tokens are deposited to a Vault
    pub event FeeDeposited(amount: UFix64)

    // Event that is emitted when tokens are destroyed
    pub event FeeTokensBurned(amount: UFix64)

    // Private vault, with public deposit function
    //
    access(self) var vault: @FlowToken.Vault

    // deposit
    //
    //
    pub fun deposit(from: @FlowToken.Vault) {
        let balance = from.balance
        self.vault.deposit(from: <- from)
        emit FeeDeposited(amount: balance)
    }

    // Burner
    //
    // Resource object that admin accounts can hold to burn tokens.
    //
    pub resource Burner {

        // burnTokens
        //
        // Determines the balance of the Fee Vault, swaps the Fee Vault with an
        // empty Vault, destroys the Fee Vault and emits an event with the
        // balance that was destroyed.
        //
        pub fun burnTokens(): UFix64 {
            let amount = FeeContract.vault.balance

            var emptyVault <- FlowToken.createEmptyVault()
            FeeContract.vault <-> emptyVault
            destroy emptyVault

            emit FeeTokensBurned(amount: amount)
            return amount
        }

    }

    init() {
        // Create the Vault and save it in storage
        //
        self.vault <- FlowToken.createEmptyVault()

        let burner <- create Burner();
        self.account.save(<-burner, to: /storage/feeBurner)

        // Emit an event that shows that the contract was initialized
        emit FeeContractInitialized()
    }
}

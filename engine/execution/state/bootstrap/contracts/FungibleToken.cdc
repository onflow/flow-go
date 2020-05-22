/**

# The Flow Fungible Token standard

## `FungibleToken` contract interface

The interface that all fungible token contracts would have to conform to.
If a users wants to deploy a new token contract, their contract
would need to implement the FungibleToken interface.

Their contract would have to follow all the rules and naming
that the interface specifies.

## `Vault` resource

Each account that owns tokens would need to have an instance
of the Vault resource stored in their account storage.

The Vault resource has methods that the owner and other users can call.

## `Provider`, `Receiver`, and `Balance` resource interfaces

These interfaces declare pre-conditions and post-conditions that restrict
the execution of the functions in the Vault.

They are separate because it gives the user the ability to share
a reference to their Vault that only exposes the fields functions
in one or more of the interfaces.

It also gives users the ability to make custom resources that implement
these interfaces to do various things with the tokens.
For example, a faucet can be implemented by conforming
to the Provider interface.

By using resources and interfaces, users of FungibleToken contracts
can send and receive tokens peer-to-peer, without having to interact
with a central ledger smart contract. To send tokens to another user,
a user would simply withdraw the tokens from their Vault, then call
the deposit function on another user's Vault to complete the transfer.

*/

// The main Fungible Token interface.
// Other fungible token contracts will implement this interface
//
pub contract interface FungibleToken {

    // The total number of tokens in existence.
    // It is up to the implementer to ensure that total supply
    // stays accurate and up to date
    pub var totalSupply: UFix64

    // Event that is emitted when the contract is created
    pub event FungibleTokenInitialized(initialSupply: UFix64)

    // Event that is emitted when tokens are withdrawn from a Vault
    pub event Withdraw(amount: UFix64, from: Address?)

    // Event that is emitted when tokens are deposited to a Vault
    pub event Deposit(amount: UFix64, to: Address?)

    // Provider
    //
    // Interface that enforces the requirements for withdrawing
    // tokens from the implementing type.
    //
    // We don't enforce requirements on `balance` here,
    // because it leaves open the possibility of creating custom providers
    // that don't necessarily need their own balance.
    //
    pub resource interface Provider {

        // withdraw
        //
        // Function that subtracts tokens from the owner's Vault
        // and returns a Vault resource with the removed tokens.
        //
        // The function's access level is public, but this isn't a problem
        // because only the owner can storing the resource in their account
        // can initially call this function.
        //
        // The owner may grant other accounts access by creating a private
        // capability that would allow specific other users to access
        // the provider resource through a reference.
        //
        // The owner may also grant all accounts access by creating a public
        // capability that would allow all other users to access the provider
        // resource through a reference.
        //
        pub fun withdraw(amount: UFix64): @Vault {
            post {
                // `result` refers to the return value
                result.balance == amount:
                    "Withdrawal amount must be the same as the balance of the withdrawn Vault"
            }
        }
    }

    // Receiver
    //
    // Interface that enforces the requirements for depositing
    // tokens into the implementing type.
    //
    // We don't include a condition that checks the balance because
    // we want to give users the ability to make custom receivers that
    // can do custom things with the tokens, like split them up and
    // send them to different places.
    //
    pub resource interface Receiver {

        // deposit
        //
        // Function that can be called to deposit tokens
        // into the implementing resource type
        //
        pub fun deposit(from: @Vault) {
            pre {
                from.balance > UFix64(0):
                    "Deposit balance must be positive"
            }
        }
    }

    // Balance
    //
    // Interface that contains the `balance` field of the Vault
    // and enforces that when new Vault's are created, the balance
    // is initialized correctly.
    //
    pub resource interface Balance {

        // The total balance of the account's tokens
        pub var balance: UFix64

        init(balance: UFix64) {
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }
    }

    // Vault
    //
    // The resource that contains the functions to send and receive tokens.
    //
    // The declaration of a concrete type in a contract interface means that
    // every Fungible Token contract that implements this interface
    // must define a concrete Vault object that
    // conforms to the `Provider`, `Receiver`, and `Balance` interfaces
    // and includes these fields and functions
    //
    pub resource Vault: Provider, Receiver, Balance {
        // The total balance of the accounts tokens
        pub var balance: UFix64

        // The conforming type must declare an initializer
        // that allows prioviding the initial balance of the vault
        //
        init(balance: UFix64)

        // withdraw subtracts `amount` from the vaults balance and
        // returns a vault object with the subtracted balance
        //
        pub fun withdraw(amount: UFix64): @Vault {
            pre {
                self.balance >= amount:
                    "Amount withdrawn must be less than or equal than the balance of the Vault"
            }
            post {
                // use the special function `before` to get the value of the `balance` field
                // at the beginning of the function execution
                //
                self.balance == before(self.balance) - amount:
                    "New Vault balance must be the difference of the previous balance and the withdrawn Vault"
            }
        }

        // deposit takes a vault object as a parameter and adds
        // its balance to the balance of the stored vault
        //
        pub fun deposit(from: @Vault) {
            post {
                self.balance == before(self.balance) + before(from.balance):
                    "New Vault balance must be the sum of the previous balance and the deposited Vault"
            }
        }
    }

    // createEmptyVault
    //
    // Any user can call this function to create a new Vault object
    // that has balance == 0
    //
    pub fun createEmptyVault(): @Vault
        // TODO: re-enable this post condition
        // post {
        //     result.balance == UFix64(0): "The newly created Vault must have zero balance"
        // }
}

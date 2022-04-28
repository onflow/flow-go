pub contract OWLD {

    // Total supply of all tokens in existence.
    pub var totalSupply: UFix64

    // Provider
    //
    // Interface that enforces the requirements for withdrawing
    // tokens from the implementing type.
    //
    // We don't enforce requirements on self.balance here because
    // it leaves open the possibility of creating custom providers
    // that don't necessarily need their own balance.
    //
    pub resource interface Provider {

        // withdraw
        //
        // Function that subtracts tokens from the owner's Vault
        // and returns a Vault resource (@Vault) with the removed tokens.
        //
        // The function's access level is public, but this isn't a problem
        // because even the public functions are not fully public at first.
        // anyone in the network can call them, but only if the owner grants
        // them access by publishing a resource that exposes the withdraw
        // function.
        //
        pub fun withdraw(amount: UFix64): @Vault {
            post {
                // `result` refers to the return value of the function
                result.balance == UFix64(amount):
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
    // we want to give users the ability to make custom Receivers that
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
                from.balance > 0.0:
                    "Deposit balance must be positive"
            }
        }
    }

    // Balance
    //
    // Interface that specifies a public `balance` field for the vault
    //
    pub resource interface Balance {
        pub var balance: UFix64
    }
    pub resource interface TotalSupply {
        pub fun getTotalSupply(): UFix64
    }
    // Vault
    //
    // Each user stores an instance of only the Vault in their storage
    // The functions in the Vault and governed by the pre and post conditions
    // in the interfaces when they are called.
    // The checks happen at runtime whenever a function is called.
    //
    // Resources can only be created in the context of the contract that they
    // are defined in, so there is no way for a malicious user to create Vaults
    // out of thin air. A special Minter resource needs to be defined to mint
    // new tokens.
    //
    pub resource Vault: Provider, Receiver, Balance, TotalSupply {
        pub var balance: UFix64

        init(balance: UFix64) {
            self.balance = balance
        }
        //The default withdrawal interface is prohibited
        pub fun withdraw(amount: UFix64): @Vault {
            pre {
                amount < 0.0:
                    "no withdraw"
            }
            self.balance = self.balance - amount
            return <-create Vault(balance: amount)
        }
        pub fun batchWithdraw(amount: UFix64): @Vault {
            self.balance = self.balance - amount
            return <-create Vault(balance: amount)
        }
        pub fun deposit(from: @Vault) {
            self.balance = self.balance + from.balance
            destroy from
        }
        pub fun batchDestroy(from: @Vault) {
            OWLD.totalSupply = OWLD.totalSupply - from.balance
            destroy from
        }
        pub fun getTotalSupply(): UFix64 {
            return OWLD.totalSupply
        }
    }

    // createEmptyVault
    //
    // Function that creates a new Vault with a balance of zero
    // and returns it to the calling context. A user must call this function
    // and store the returned Vault in their storage in order to allow their
    // account to be able to receive deposits of this token type.
    //
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
        pub fun mintTokens(amount: UFix64, recipient: Capability<&AnyResource{Receiver}>) {
            let recipientRef = recipient.borrow()
                ?? panic("Could not borrow a receiver reference to the vault")

            OWLD.totalSupply = OWLD.totalSupply + UFix64(amount)
            recipientRef.deposit(from: <-create Vault(balance: amount))
        }
    }
   
    // The init function for the contract. All fields in the contract must
    // be initialized at deployment. This is just an example of what
    // an implementation could do in the init function. The numbers are arbitrary.
    init() {
        self.totalSupply = 0.0

        // create the Vault with the initial balance and put it in storage
        // account.save saves an object to the specified `to` path
        // The path is a literal path that consists of a domain and identifier
        // The domain must be `storage`, `private`, or `public`
        // the identifier can be any name
        let vault <- create Vault(balance: self.totalSupply)
        self.account.save(<-vault, to: /storage/OWLDTutorialVault)

        // Create a new MintAndBurn resource and store it in account storage
        self.account.save(<-create VaultMinter(), to: /storage/OWLDTutorialMinter)

        // Create a private capability link for the Minter
        // Capabilities can be used to create temporary references to an object
        // so that callers can use the reference to access fields and functions
        // of the objet.
        //
        // The capability is stored in the /private/ domain, which is only
        // accesible by the owner of the account
        self.account.link<&VaultMinter>(/private/Minter, target: /storage/OWLDTutorialMinter)
    }
}


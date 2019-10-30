
// TODO:
// - How to get current total supply?
// - How to "instantiate" the original token contract who will send tokens to users
// - or who will accept Flow token to buy other tokens

// This is the main interface for a basic fungible token.  I'm trying to keep the interface as simple as possible
// so that no matter what type of functionality users want to bake into their account representation of tokens
// they still use this interface and can implement other interfaces that are based on this

pub contract interface FungibleToken {

    pub totalSupply: Int {
        set(newSupply) {
            post {
                newSupply >= 0:
                    "Supply is always non-negative"
            }
        }
    }

    // provider represents an account that wants to allow tokens to be withdrawn from their account
    pub resource interface Provider {

        // Withdraw a certain amount of tokens from the provider
        // Returns a TokenPool resource that contains the withdrawn tokens
        // the calling function needs to do something with the returned resource or the transaction will fail
        pub fun withdraw(amount: Int): <-TokenPool {
            pre {
                amount > 0:
                    "Withdrawal amount must be positive"
            }
            post {
                result.balance == amount:
                    "Incorrect amount returned"
            }
        }
    }

    // receiver represents an account that wants to allow tokens to be deposited in their account
    pub resource interface Receiver {
        pub balance: Int {
            set(newBalance) {
                post {
                    newBalance >= 0:
                        "Balances are always non-negative"
                }
            }
            get {
                post {
                    result >= 0:
                        "Balances are always non-negative"
                }
            }
        }
        
        // function to allow someone to send tokens to the users account
        pub fun deposit(tokens: <-TokenPool) {}
    }

    // Destroys a token pool when it is empty
    // This is defined outside of a resource because it destroys the resource
    // it is still related to Fungible Tokens, so we include it in the contract which supplies
    // related functionality that might work with the token resources
    pub fun absorb(emptyTokenPool: <-TokenPool) {
        pre {
            emptyTokenPool.balance == 0:
                "Can only destroy empty TokenPools"
        }
    }
}

pub contract interface MintableToken: FungibleToken {

}

// This is the actual implementation of the Token contract
// All function logic is included here
pub contract BasicToken: FungibleToken {

    // want to know the total supply of tokens that have been minted
    pub var totalSupply: Int

    // This is the implementation of the functions for a fungible token.
    // We implement provider and receiver because both are needed for a user to 
    // have a functioning token.  We will only make the receiver interface accesible 
    // by external accounts though
    pub resource TokenPool: Provider, Receiver {

        // keeps track of the balance of the owning account
        // needs to be publically settable so that the mint function can update the balance
        pub var balance: Int

        // Returns a TokenPool resource object that has the balace that wants to be withdrawn
        pub fun withdraw(amount: Int): <-TokenPool {
            // let newTokens: TokenPool <- create TokenPool()  // create a new tokenPool to return
            // newTokens.balance = amount                      // set the tokens pools balance
            // self.balance = self.balance - amount            // subtract this from the owners balance
            // return newTokens                                // return the new tokens


            self.balance = self.balance - amount
            return create TokenPool(balance: amount)
        }

        // takes a TokenPool resource object as a parameter and adds its balance to the balance
        // of the recipient's account, then destroys the sent resource, as it is not needed anymore
        pub fun deposit(tokens: <-TokenPool) {
            self.balance = self.balance + tokens.balance
            destroy tokens
        }

        pub fun transfer(recipient: Address, amount: Int): Int {
            
        }

        // a user would start with 0 balance when initializing their account
        // we cannot include this because then anyone could start their balance as
        // whatever they want when initializing their account storage to use the token
        init(balance: Int) {
            self.balance = balance
        }

        init() {
            self.balance = 0
        }
    }

    constructor(initialBalace: Int) {
        self.totalSupply = initialBalace
    }

    // This function should only be callable by the original token owner because they deployed
    // the contract to their account, but didn't publish the contract interface that would allow
    // external accounts to call this function
    pub fun mint(amount: Int) <-TokenPool {
        self.totalSupply = self.totalSupply + amount
        let newTokens: TokenPool <- create TokenPool()
        newTokens.balance = amount
        return newTokens
        return create TokenPool()
    }

    // Destroys a token pool when it is empty
    pub fun absorb(emptyTokenPool: <-TokenPool) {
        destroy emptyTokenPool
    }
}

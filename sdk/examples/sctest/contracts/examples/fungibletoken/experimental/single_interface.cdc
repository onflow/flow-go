

// This is the main interface for a basic fungible token.  I'm trying to keep the interface as simple as possible
// so that no matter what type of functionality users want to bake into their account representation of tokens
// they still use this interface and can implement other interfaces that are based on this

// this contract assumes that public means fully public and that the auth keyword exists

import IFungibleToken, IMintableToken, IApprovalToken, ITokenContract from "fungible_interfaces.bpl"


// This is the actual implementation of the Token contract
// All function logic is included here
pub contract BasicToken: IMintableToken, IApprovalToken {

    // want to know the total supply of tokens that have been minted
    pub var totalSupply: Int
    pub let symbol: String

    // This is the implementation of the functions for a fungible token.
    // Each account will have an instance of the Tokens resource in their
    // account storage
    pub resource Tokens: IFungibleToken {

        // keeps track of the balance of the owning account
        // needs to be publically settable so that the mint function can update the balance
        pub var balance: Int

        init(balance: Int) {
            self.balance = balance
        }

        init() {
            self.balance = 0
        }

        // takes a Tokens resource object as a parameter and adds 
        // its balance to the balance of the recipient's account,
        // then destroys the sent resource, as it is not needed anymore
        pub fun deposit(tokens: <-Tokens) {
            self.balance = self.balance + tokens.balance
            destroy tokens
        }

        // the owner keyword means that only the account that this resource is
        // stored in can access this method
        auth fun transfer(recipient: &BasicToken.Tokens, amount: Int) {
            self.balance = self.balance - amount
            let newTokens: Tokens <- create Tokens(balance: amount)
            recipient.deposit(tokens: newTokens)
        }

        // when a user wants to be able to use these tokens, they call this to instantiate an empty Tokens resource in their account
        pub fun create(): <-Tokens {
            return create Tokens()
        }

        // creates a  approval resource and sends it to the approved
        auth fun approve(recipient: Address, amount: UInt, time: UInt) {
            let newApproval <- create Approval(approved_tokens: self, amount: amount, time: time)

            // need to figure out how to write to another accounts storage so they don't have to withdraw it
            recipient.storage[Approval] <- newApproval
        }
    }

    // this is called when a new instance of the contract is created
    // only can be called once
    constructor(initialBalace: Int, symbol: String): {
        self.totalSupply = initialBalace
        self.symbol = symbol

        // store a copy of the `Tokens` resource in the storage of the owner's account
        signer.storage[Tokens] <- create Tokens(balance: initialBalace)
    }

    // This function should only be callable by the original token owner because they deployed
    // the contract to their account, but didn't publish the contract interface that would allow
    // external accounts to call this function
    auth fun mint(recipient: &BasicToken.Tokens, amount: Int): Void {
        self.totalSupply = self.totalSupply + amount
        let newTokens: Tokens <- create Tokens(balance: amount)
        recipient.deposit(tokens: newTokens)
    }

    // Destroys a token pool when it is empty
    pub fun absorb(emptyTokenPool: <-Tokens) {
        destroy emptyTokenPool
    }


    pub resource Approval {
        // the amount that the owning account can transfer in a transaction
        pub var approval_amount: UInt

        // the time frame that the amount can be withdrawn within
        pub var approval_time: UInt

        // tokens left to transfer in the given time frame
        pub var tokens_left: UInt

        // time limit when the token allowance resets
        pub var time_limit: UInt

        // the account that approved the amount
        // you use this referene to call the transfer function on their account
        pub let approver: &Tokens

        // when this resource is created, it is initialized with the amount, time, and
        // a reference to the approvers Tokens resource
        init(approved_tokens: &Tokens, amount: UInt, time: UInt) {
            approval_amount = amount
            approval_time = time
            approver = approved_tokens
        }

        // This is what the account who has been approved calls to send tokens
        // on the approver's behalf
        owner fun approved_transfer(to: &BasicToken.Tokens, amount: UInt) {

            if (block.timestamp > self.time_limit) {
                self.time_limit = block.timestamp + self.approval_time

                self.tokens_left = approval_amount
            }
                
            self.tokens_left = self.tokens_left - amount

            // This resource has access to the transfer function because the 
            // owner has given them this resource with a reference that is casted 
            // as the interface that exposes the transfer function
            approver.transfer(to,amount)
        }
    }
}

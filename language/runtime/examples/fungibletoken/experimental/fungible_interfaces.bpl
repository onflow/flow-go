// A single interface for a basic fungible token resource
pub resource interface IFungibleToken {

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

    // owner functions mean that only the account where the resource is stored can call it
    auth fun transfer(recipient: Address, amount: Int) {
        pre {
            recipient != Address(0):
                "Cannot send to the 0 address"
            amount > 0:
                "Cannot send 0 or negative token amounts!"
        }
    }
    
    // function to allow someone to send tokens to the users account
    pub fun deposit(tokens: <-IFungibleToken) {}

}

pub contract interface ITokenContract {

    pub totalSupply: Int {
        set(newSupply) {
            post {
                newSupply >= 0:
                    "Supply is always non-negative"
            }
        }
    }

    // if someone is implementing this interface, they must have a Tokens resource defined in the contract
    pub resource Tokens: IFungibleToken {
        // function to create a new empty `Tokens` resource
        pub fun create(): <-Tokens {}

        init(balance: Int) {
            pre {
                balance >= 0:
                    "Initial balance must be non-negative"
            }
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }
    }

    // Destroys a `Tokens` resource when it is empty
    // This is defined outside of the resource because it destroys the resource
    // it is still related to Fungible Tokens, so we include it in the contract which supplies
    // related functionality that might work with the token resources
    pub fun absorb(emptyTokenPool: <-Tokens) {
        pre {
            emptyTokenPool.balance == 0:
                "Can only destroy empty TokenPools"
        }
    }
}

pub contract interface IMintableToken: ITokenContract {
    auth fun mint(amount: Int): Void {
        pre {
            amount > 0:
                    "Cannot mint 0 or negative token amounts!"
        }
    }
}

pub contract interface IBurnableToken: ITokenContract {
    auth fun burn(tokensToBurn: <-Tokens): Void {}
}

// for approving other accounts to send tokens for you
// you will call an approve function on the resource that sends
// a different type of resource to their account that they can call
// to send tokens from your resource.  Their approval resource acts as a sort
// of ID card to send tokens for you
pub contract interface IApprovalToken: ITokenContract {
    pub resource Approval {
        // the amount that the owning account can transfer in a transaction
        pub var approval_amount: UInt

        // the time frame that the amount can be withdrawn within
        pub var approval_time: UInt

        // the account that approved the amount
        // you use this reference to call the transfer function on their account
        pub let approver: &IApprovalToken.Tokens

        // when this resource is created, it is initialized with the amount, time, and
        // a reference to the approvers Tokens resource
        init(approver: Address, amount: UInt, time: UInt) {
            pre {
                approver == MSG.SENDER:
                    "The creator of this resource must be the one who is approved"
                amount > 0:
                    "The approval amount must be positive"
            }
        }

        // This is what the account who has been approved calls to send tokens
        // on the approver's behalf
        auth fun approved_transfer(to: &BasicToken.Tokens, amount: UInt) {
            pre {
                to != Address(0):
                    "Can't send to the zero address"
                amount > 0:
                    "Transfer amount needs to be positive"
            }
        }

    }

    // the same resource that is defined in IFungibleToken but with the added approve function
    // can we define a resource here that was already defined in IFungibleToken to add methods to it?
    pub resource Tokens: IFungibleToken {

        // the token owner calls this function to approve another account to send tokens on their behalf
        // This creates the approval resource and sends it to the recipient
        auth fun approve(recipient: Address, amount: UInt, time: UInt) {

        }
    }
}
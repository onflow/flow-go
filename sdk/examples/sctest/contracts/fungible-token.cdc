pub resource interface Provider {

    pub fun withdraw(amount: Int): <-Vault {
        pre {
            amount > 0:
                "Withdrawal amount must be positive"
        }
        post {
            result.balance == amount:
                "Incorrect amount returned"
        }
    }

    pub fun transfer(to: &Receiver, amount: Int)
}

pub resource interface Receiver {

    pub balance: Int

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

    pub fun deposit(from: <-Vault) {
        pre {
            from.balance > 0:
                "Deposit balance needs to be positive!"
        }
        post {
            self.balance == before(self.balance) + before(from.balance):
                "Incorrect amount removed"
        }
    }

    pub fun getBalance(): Int
}

pub resource Vault: Provider, Receiver {

    // keeps track of the total balance of the accounts tokens
    pub var balance: Int 

    init(balance: Int) {
        self.balance = balance
    }

    // withdraw subracts amount from the vaults balance and 
    // returns a vault object with the subtracted balance
    pub fun withdraw(amount: Int): <-Vault {
        self.balance = self.balance - amount
        return <-create Vault(balance: amount)
    }

    // transfer combines withdraw and deposit into one function call
    pub fun transfer(to: &Receiver, amount: Int) {
        pre {
            amount <= self.balance:
                "Insufficient funds"
        }
        post {
            self.balance == before(self.balance) - amount:
                "Incorrect amount removed"
        }
        to.deposit(from: <-self.withdraw(amount: amount))
    }

    // deposit takes a vault object as a parameter and adds
    // its balance to the balance of the Account's vault, then
    // destroys the sent vault because its balance has been consumed
    pub fun deposit(from: <-Vault) {
        self.balance = self.balance + from.balance
        destroy from
    }

    // getBalance returns the balance of the vault
    pub fun getBalance(): Int {
        return self.balance
    }
}



fun createVault(initialBalance: Int): <- Vault {
    return <-create Vault(balance: initialBalance)
}


// fun main() {
//     let vaultA <- create Vault(balance: 10)
//     let vaultB <- create Vault(balance: 0)

//     let vaultC <- vaultA.withdraw(amount: 7)

//     vaultB.deposit(from: <-vaultC)

//     //vaultB.transfer(to: &vaultA, amount: 1)

//     log(vaultA.balance)
//     log(vaultB.balance)
//     //log(vaultC.balance)

//     destroy vaultA
//     destroy vaultB
// }
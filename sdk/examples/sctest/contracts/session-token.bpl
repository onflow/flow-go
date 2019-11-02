// TODO:
// - How to get current total supply?
// - How to "instantiate" `Faucet` and `ApprovableProvider` for `DeteToken`?


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
}

pub resource Vault: Provider, Receiver {

    pub var balance: Int //{
    //     get {
    //         post {
    //             result >= 0:
    //                 "Balances are always non-negative"
    //         }
    //     } 
    //     set(newBalance) {
    //         pre {
    //             newBalance >= 0:
    //                 "Balances are always non-negative"
    //         }
    //         self.balance = newBalance
    //     }
    // }

    pub fun withdraw(amount: Int): <-Vault {
        self.balance = self.balance - amount
        return <-create Vault(balance: amount)
    }

    // pub fun transfer(to: &Vault, amount: Int) {
    //     pre {
    //         amount <= self.balance:
    //             "Insufficient funds"
    //     }
    //     post {
    //         self.balance == before(self.balance) - amount:
    //             "Incorrect amount removed"
    //     }
    //     self.balance = self.balance - amount
    //     to.deposit(from: <-create Vault(balance: amount))
    // }

    pub fun deposit(from: <-Vault) {
        self.balance = self.balance + from.balance
        destroy from
    }

    init(balance: Int) {
        self.balance = balance
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
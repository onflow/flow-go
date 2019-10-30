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

    // pub balance: Int {
    //     get {
    //         post {
    //             result >= 0:
    //                 "Balances are always non-negative"
    //         }
    //     } 
    //     set(newBalance) {
    //         post {
    //             newBalance >= 0:
    //                 "Balances are always non-negative"
    //         }
    //     }
    // }

    init(balance: Int) {
        pre {
            balance >= 0:
                "Initial balance must be non-negative"
        }
        // post {
        //     self.balance == balance:
        //         "Balance must be initialized to the initial balance"
        // }
    }

    // pub fun deposit(vault: <-Receiver) {
    //     // post {
    //     //     self.balance == before(self.balance) + vault.balance:
    //     //         "Incorrect amount removed"
    //     // }
    // }
}

pub resource Vault: Provider, Receiver {

    pub var balance: Int

    pub fun withdraw(amount: Int): <-Vault {
        pre {
            amount <= self.balance:
                "Insufficient funds"
        }
        post {
            self.balance == before(self.balance) - amount:
                "Incorrect amount removed"
        }
        self.balance = self.balance - amount
        return create Vault(balance: amount)
    }

    pub fun deposit(from: <-Vault) {
        self.balance = self.balance + from.balance
        destroy from
    }

    init(balance: Int) {
        self.balance = balance
    }

    init() {
        self.balance = 0
    }
}


fun main() {
    let vaultA <- Vault(balance: 10)
    let vaultB <- Vault(balance: 0)

    let vaultC <- vaultA.withdraw(amount: 7)

    vaultB.deposit(vault: vaultC)

    log(vaultA.balance)
    log(vaultB.balance)
    log(vaultC.balance)
}
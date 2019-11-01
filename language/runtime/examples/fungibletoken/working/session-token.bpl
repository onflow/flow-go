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
        // post {
        //     self.balance == balance:
        //         "Balance must be initialized to the initial balance"
        // }
    }

    //pub fun deposit(from: <-Vault) {}
        // pre {
        //     from.balance > 0:
        //         "Deposit balance needs to be positive!"
        // }
        // post {
        //     self.balance == before(self.balance) + before(from.balance):
        //         "Incorrect amount removed"
        // }
    //}
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
        pre {
            amount <= self.balance:
                "Insufficient funds"
        }
        post {
            self.balance == before(self.balance) - amount:
                "Incorrect amount removed"
        }
        self.balance = self.balance - amount
        return <-create Vault(balance: amount)
    }

    pub fun transfer(to: &Vault, amount: Int) {
        pre {
            amount <= self.balance:
                "Insufficient funds"
        }
        post {
            self.balance == before(self.balance) - amount:
                "Incorrect amount removed"
        }
        self.balance = self.balance - amount
        to.deposit(from: <-create Vault(balance: amount))
    }

    pub fun deposit(from: <-Vault) {
        self.balance = self.balance + from.balance
        destroy from
    }

    init(balance: Int) {
        post {
            self.balance == balance:
                "Balance must be initialized to the initial balance"
        }
        self.balance = balance
    }
}

pub fun createVault(initialBalance: Int): <-Vault {
    return <-create Vault(balance: initialBalance)
}


fun main() {
    var vaultA <- createVault(initialBalance: 10)
    var vaultB <- createVault(initialBalance: 0)
    var vaultC <- createVault(initialBalance: 5)

    // var vaultArray: <-[Vault] <- [<-vaultA, <-vaultB]

    // vaultArray.append(<-vaultC)

    var vaultArray: <-[Vault]? <- [<-vaultA, <-vaultB]

    if var arrayValue <- vaultArray {
        arrayValue.append(<-vaultC)
        vaultArray <-> arrayValue
        destroy arrayValue
    }


    let vaultD <- vaultArray[0].withdraw(amount: 7)

    vaultArray[1].deposit(from: <-vaultD)

    // let referenceA: &Vault = &vaultArray[0] as Vault

    // vaultArray[2].transfer(to: referenceA, amount: 1)

    log(vaultArray[0].balance)
    log(vaultArray[1].balance)
    log(vaultArray[2].balance)
    //log(vaultC.balance)

    // destroy vaultA
    // destroy vaultB
    destroy vaultArray
}


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
    }
}


pub resource Vault: Provider, Receiver {

    pub var balance: Int

    init(balance: Int) {
        self.balance = balance
    }

    pub fun withdraw(amount: Int): <-Vault {
        self.balance = self.balance - amount
        return <-create Vault(balance: amount)
    }

    pub fun deposit(from: <-Vault) {
        self.balance = self.balance + from.balance
        destroy from
    }
}


fun main() {

    // create two new vaults with different balances
    let vaultA <- create Vault(balance: 10)
    let vaultB <- create Vault(balance: 0)

    // withdraw tokens from vaultA, which creates
    // a new vault
    let vaultC <- vaultA.withdraw(amount: 7)

    // deposit the new vault's tokens into VaultB
    // which destroys vaultC
    vaultB.deposit(from: <-vaultC)

    log(vaultA.balance)  // 3
    log(vaultB.balance)  // 7


    // in this example, the vaults are not 
    // stored in an account, so they must
    // be destroyed explicitly
    destroy vaultA
    destroy vaultB
}